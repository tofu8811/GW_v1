package cache

import (
	"context"
	"errors"

	"gateway-api/helper/response"
	configcache "gateway-api/internal/config/cache"

	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"
)

type ConfigStore interface {
	RebuildAll(ctx context.Context) error
	CurrentVersion() int64
}

type ConfigNotifier interface {
	BumpVersion(ctx context.Context) (int64, error)
	PublishReload(ctx context.Context, group string) error
	NotifyChange(ctx context.Context, group string) error
}

type Handler struct {
	store    ConfigStore
	notifier ConfigNotifier
	redis    *redis.Client
}

func NewHandler(store ConfigStore, notifier ConfigNotifier, redisClient *redis.Client) *Handler {
	return &Handler{store: store, notifier: notifier, redis: redisClient}
}

func (h *Handler) Reload(c *fiber.Ctx) error {
	if h.store == nil || h.notifier == nil {
		return response.InternalServerError(c)
	}

	bumpedVersion, err := h.notifier.BumpVersion(c.Context())
	if err != nil {
		return response.InternalServerError(c)
	}
	if err := h.store.RebuildAll(c.Context()); err != nil {
		return response.InternalServerError(c)
	}
	if err := h.notifier.PublishReload(c.Context(), "cache"); err != nil {
		return response.InternalServerError(c)
	}

	redisVersion, err := h.redisVersion(c.Context())
	if err != nil {
		return response.InternalServerError(c)
	}
	localVersion := h.store.CurrentVersion()

	return response.OK(c, fiber.Map{
		"reloaded":       true,
		"published":      true,
		"bumped_version": bumpedVersion,
		"redis_version":  redisVersion,
		"local_version":  localVersion,
		"synced":         redisVersion == localVersion,
		"sync_pending":   redisVersion != localVersion,
	})
}

func (h *Handler) Version(c *fiber.Ctx) error {
	if h.store == nil {
		return response.InternalServerError(c)
	}

	redisVersion, err := h.redisVersion(c.Context())
	if err != nil {
		return response.InternalServerError(c)
	}
	localVersion := h.store.CurrentVersion()

	return response.OK(c, fiber.Map{
		"redis_version": redisVersion,
		"local_version": localVersion,
		"synced":        redisVersion == localVersion,
		"sync_pending":  redisVersion != localVersion,
	})
}

func (h *Handler) redisVersion(ctx context.Context) (int64, error) {
	if h.redis == nil {
		return 0, nil
	}
	version, err := h.redis.Get(ctx, configcache.KeyVersion).Int64()
	if errors.Is(err, redis.Nil) {
		return 0, nil
	}
	return version, err
}
