package proxy

import (
	"log/slog"

	configcache "gateway-api/internal/config/cache"
	"gateway-api/internal/upstream/breaker"
	upstreamhealth "gateway-api/internal/upstream/health"

	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"
)

func RegisterGatewayRoutes(app *fiber.App, configCache *configcache.Store, redisClient *redis.Client, logger *slog.Logger, healthFilter *upstreamhealth.HealthFilter, breakers *breaker.Registry, authenticator RouteAuthenticator) {
	rateLimiter := NewRateLimiter(redisClient, logger)
	handler := NewHandler(configCache, logger, healthFilter, breakers, rateLimiter, authenticator)

	app.Use(handler.Proxy)
}
