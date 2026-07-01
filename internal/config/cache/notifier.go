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

func (n *Notifier) BumpVersion(ctx context.Context) (int64, error) {
	return n.redis.Incr(ctx, KeyVersion).Result()
}

func (n *Notifier) PublishReload(ctx context.Context, group string) error {
	return n.redis.Publish(ctx, KeyReload, group).Err()
}

func (n *Notifier) NotifyChange(ctx context.Context, group string) error {
	if _, err := n.BumpVersion(ctx); err != nil {
		return err
	}
	return n.PublishReload(ctx, group)
}
