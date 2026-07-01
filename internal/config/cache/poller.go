package cache

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"
)

type Poller struct {
	redis    *redis.Client
	logger   *slog.Logger
	interval time.Duration
}

func NewPoller(redisClient *redis.Client, logger *slog.Logger, interval time.Duration) *Poller {
	if logger == nil {
		logger = slog.Default()
	}
	if interval <= 0 {
		interval = 20 * time.Second
	}
	return &Poller{redis: redisClient, logger: logger, interval: interval}
}

func (p *Poller) PollVersion(ctx context.Context, currentVersion func() int64, rebuild func(context.Context) error) {
	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			version, err := p.redis.Get(ctx, KeyVersion).Int64()
			if errors.Is(err, redis.Nil) {
				version = 0
			} else if err != nil {
				p.logger.Warn("config version poll failed", "error", err)
				continue
			}

			if version != currentVersion() {
				if err := rebuild(ctx); err != nil {
					p.logger.Error("config version rebuild failed", "redis_version", version, "error", err)
				}
			}
		}
	}
}
