package rabbitmq

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"time"

	"gateway-api/internal/middleware"

	amqp "github.com/rabbitmq/amqp091-go"
)

type LogPublisherConfig struct {
	URL        string
	Exchange   string
	RoutingKey string
	Timeout    time.Duration
}

type LogPublisher struct {
	conn       *amqp.Connection
	channel    *amqp.Channel
	exchange   string
	routingKey string
	timeout    time.Duration
	logger     *slog.Logger
	mu         sync.Mutex
}

func NewLogPublisher(cfg LogPublisherConfig, logger *slog.Logger) (*LogPublisher, error) {
	if strings.TrimSpace(cfg.URL) == "" {
		return nil, errors.New("rabbitmq url is required")
	}
	if strings.TrimSpace(cfg.Exchange) == "" {
		cfg.Exchange = "gateway.logs.exchange"
	}
	if strings.TrimSpace(cfg.RoutingKey) == "" {
		cfg.RoutingKey = "gateway.request.completed"
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 500 * time.Millisecond
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
	if err := ch.ExchangeDeclare(cfg.Exchange, "topic", true, false, false, false, nil); err != nil {
		_ = ch.Close()
		_ = conn.Close()
		return nil, err
	}
	if err := ch.Confirm(false); err != nil {
		_ = ch.Close()
		_ = conn.Close()
		return nil, err
	}

	return &LogPublisher{
		conn:       conn,
		channel:    ch,
		exchange:   cfg.Exchange,
		routingKey: cfg.RoutingKey,
		timeout:    cfg.Timeout,
		logger:     logger,
	}, nil
}

func (p *LogPublisher) WriteLog(ctx context.Context, entry middleware.RequestLogEntry) error {
	if p == nil || p.channel == nil {
		return nil
	}

	payload, err := json.Marshal(entry)
	if err != nil {
		return err
	}

	publishCtx, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()

	routingKey := p.routingKey
	if entry.IsError {
		routingKey = "gateway.request.error"
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	confirmation, err := p.channel.PublishWithDeferredConfirmWithContext(
		publishCtx,
		p.exchange,
		routingKey,
		false,
		false,
		amqp.Publishing{
			ContentType:  "application/json",
			DeliveryMode: amqp.Persistent,
			Timestamp:    entry.Timestamp,
			MessageId:    entry.TraceID,
			Body:         payload,
		},
	)
	if err != nil {
		return err
	}
	if confirmation == nil {
		return nil
	}
	confirmed, err := confirmation.WaitContext(publishCtx)
	if err != nil {
		return err
	}
	if !confirmed {
		return context.DeadlineExceeded
	}
	return nil
}

func (p *LogPublisher) Close() error {
	if p == nil {
		return nil
	}
	var firstErr error
	if p.channel != nil {
		if err := p.channel.Close(); err != nil {
			firstErr = err
		}
	}
	if p.conn != nil {
		if err := p.conn.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	if firstErr != nil && p.logger != nil {
		p.logger.Warn("failed to close rabbitmq log publisher", "error", firstErr)
	}
	return firstErr
}
