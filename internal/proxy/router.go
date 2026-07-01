package proxy

import (
	"log/slog"

	configcache "gateway-api/internal/config/cache"
	"gateway-api/internal/upstream/breaker"
	upstreamhealth "gateway-api/internal/upstream/health"

	"github.com/gofiber/fiber/v2"
)

func RegisterGatewayRoutes(app *fiber.App, configCache *configcache.Store, logger *slog.Logger, healthFilter *upstreamhealth.HealthFilter, breakers *breaker.Registry, authenticator RouteAuthenticator) {
	handler := NewHandler(configCache, logger, healthFilter, breakers, authenticator)

	app.Use(handler.Proxy)
}
