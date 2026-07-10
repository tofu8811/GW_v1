package consumer

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"time"

	"gateway-api/internal/logservice/model"

	amqp "github.com/rabbitmq/amqp091-go"
)

type Indexer interface {
	BulkIndex(ctx context.Context, logs []model.RequestLog) error
}

type LogPublisher interface {
	PublishLog(entry model.RequestLog)
}

type Config struct {
	URL           string
	Exchange      string
	Queue         string
	RoutingKey    string
	DLX           string
	DLQ           string
	BatchSize     int
	FlushInterval time.Duration
	Prefetch      int
}

type Consumer struct {
	cfg      Config
	indexer  Indexer
	realtime LogPublisher
	logger   *slog.Logger
	conn     *amqp.Connection
	channel  *amqp.Channel
}

func New(cfg Config, indexer Indexer, logger *slog.Logger, realtime ...LogPublisher) (*Consumer, error) {
	if strings.TrimSpace(cfg.URL) == "" {
		return nil, errors.New("rabbitmq url is required")
	}
	if cfg.Exchange == "" {
		cfg.Exchange = "gateway.logs.exchange"
	}
	if cfg.Queue == "" {
		cfg.Queue = "gateway.logs.queue"
	}
	if cfg.RoutingKey == "" {
		cfg.RoutingKey = "gateway.request.*"
	}
	if cfg.DLX == "" {
		cfg.DLX = "gateway.logs.dlx"
	}
	if cfg.DLQ == "" {
		cfg.DLQ = "gateway.logs.dlq"
	}
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = 500
	}
	if cfg.FlushInterval <= 0 {
		cfg.FlushInterval = time.Second
	}
	if cfg.Prefetch <= 0 {
		cfg.Prefetch = 100
	}

	conn, err := amqp.Dial(cfg.URL)
	if err != nil {
		return nil, err
	}
	ch, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		return nil, err
	}

	if err := declareTopology(ch, cfg); err != nil {
		_ = ch.Close()
		_ = conn.Close()
		return nil, err
	}
	if err := ch.Qos(cfg.Prefetch, 0, false); err != nil {
		_ = ch.Close()
		_ = conn.Close()
		return nil, err
	}

	var publisher LogPublisher
	if len(realtime) > 0 {
		publisher = realtime[0]
	}
	return &Consumer{cfg: cfg, indexer: indexer, realtime: publisher, logger: logger, conn: conn, channel: ch}, nil
}

func declareTopology(ch *amqp.Channel, cfg Config) error {
	if err := ch.ExchangeDeclare(cfg.Exchange, "topic", true, false, false, false, nil); err != nil {
		return err
	}
	if err := ch.ExchangeDeclare(cfg.DLX, "topic", true, false, false, false, nil); err != nil {
		return err
	}
	if _, err := ch.QueueDeclare(cfg.DLQ, true, false, false, false, nil); err != nil {
		return err
	}
	if err := ch.QueueBind(cfg.DLQ, "#", cfg.DLX, false, nil); err != nil {
		return err
	}
	args := amqp.Table{"x-dead-letter-exchange": cfg.DLX}
	if _, err := ch.QueueDeclare(cfg.Queue, true, false, false, false, args); err != nil {
		return err
	}
	return ch.QueueBind(cfg.Queue, cfg.RoutingKey, cfg.Exchange, false, nil)
}

func (c *Consumer) Start(ctx context.Context) error {
	deliveries, err := c.channel.Consume(c.cfg.Queue, "gateway-log-service", false, false, false, false, nil)
	if err != nil {
		return err
	}

	ticker := time.NewTicker(c.cfg.FlushInterval)
	defer ticker.Stop()

	batch := make([]model.RequestLog, 0, c.cfg.BatchSize)
	acks := make([]amqp.Delivery, 0, c.cfg.BatchSize)

	flush := func() {
		if len(batch) == 0 {
			return
		}
		if err := c.indexer.BulkIndex(ctx, batch); err != nil {
			if c.logger != nil {
				c.logger.Error("failed to bulk index request logs", "error", err, "count", len(batch))
			}
			for _, delivery := range acks {
				_ = delivery.Nack(false, true)
			}
		} else {
			for _, delivery := range acks {
				_ = delivery.Ack(false)
			}
		}
		batch = batch[:0]
		acks = acks[:0]
	}

	for {
		select {
		case <-ctx.Done():
			flush()
			return ctx.Err()
		case <-ticker.C:
			flush()
		case delivery, ok := <-deliveries:
			if !ok {
				flush()
				return nil
			}
			var entry model.RequestLog
			if err := json.Unmarshal(delivery.Body, &entry); err != nil {
				if c.logger != nil {
					c.logger.Warn("invalid request log message sent to dlq", "error", err)
				}
				_ = delivery.Nack(false, false)
				continue
			}
			if entry.Timestamp.IsZero() {
				entry.Timestamp = time.Now().UTC()
			}
			if c.realtime != nil {
				c.realtime.PublishLog(entry)
			}
			batch = append(batch, entry)
			acks = append(acks, delivery)
			if len(batch) >= c.cfg.BatchSize {
				flush()
			}
		}
	}
}

func (c *Consumer) Close() error {
	if c == nil {
		return nil
	}
	var firstErr error
	if c.channel != nil {
		if err := c.channel.Close(); err != nil {
			firstErr = err
		}
	}
	if c.conn != nil {
		if err := c.conn.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}
