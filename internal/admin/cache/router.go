package cache

import (
	"gateway-api/internal/middleware"

	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"
)

func RegisterCacheRoutes(router fiber.Router, store ConfigStore, notifier ConfigNotifier, redisClient *redis.Client) {
	handler := NewHandler(store, notifier, redisClient)

	router.Post("/reload", middleware.RequirePermission("cache:reload"), handler.Reload)
	router.Get("/version", middleware.RequirePermission("cache:reload"), handler.Version)
}
