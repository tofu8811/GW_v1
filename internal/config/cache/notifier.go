package cache

import (
	"context"

	"github.com/redis/go-redis/v9"
)

type Notifier struct {
	redis *redis.Client
}

func NewNotifier(redisClient *redis.Client) *Notifier {
	return &Notifier{redis: redisClient}
}

func (n *Notifier) NotifyChange(ctx context.Context, group string) error {
	if _, err := n.redis.Incr(ctx, KeyVersion).Result(); err != nil {
		return err
	}
	return n.redis.Publish(ctx, KeyReload, group).Err()
}
