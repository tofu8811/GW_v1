package proxy

import (
	"log/slog"

	configcache "gateway-api/internal/config/cache"

	"github.com/gofiber/fiber/v2"
)

func RegisterGatewayRoutes(app *fiber.App, configCache *configcache.Store, logger *slog.Logger) {
	handler := NewHandler(configCache, logger)

	app.Use(handler.Proxy)
}
