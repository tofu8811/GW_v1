package cache

import (
	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"
)

func RegisterCacheRoutes(router fiber.Router, store ConfigStore, notifier ConfigNotifier, redisClient *redis.Client) {
	handler := NewHandler(store, notifier, redisClient)

	router.Post("/reload", handler.Reload)
	router.Get("/version", handler.Version)
}
