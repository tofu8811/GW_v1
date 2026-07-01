package cache

import (
	"context"
	"log/slog"

	"github.com/redis/go-redis/v9"
)

type Subscriber struct {
	redis  *redis.Client
	logger *slog.Logger
}

func NewSubscriber(redisClient *redis.Client, logger *slog.Logger) *Subscriber {
	if logger == nil {
		logger = slog.Default()
	}
	return &Subscriber{redis: redisClient, logger: logger}
}

func (s *Subscriber) SubscribeReload(ctx context.Context, rebuild func(context.Context) error) {
	pubsub := s.redis.Subscribe(ctx, KeyReload)
	defer pubsub.Close()

	ch := pubsub.Channel()
	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}
			// Fast path for admin changes; missed messages are covered by Poller.
			if err := rebuild(ctx); err != nil {
				s.logger.Error("config reload failed", "group", msg.Payload, "error", err)
			}
		}
	}
}
