package server

import (
	"log/slog"
	"strings"

	"gateway-api/internal/health"
	appmiddleware "gateway-api/internal/middleware"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/gofiber/fiber/v2/middleware/requestid"
)

type Server struct {
	App    *fiber.App
	Logger *slog.Logger
}

func New(logger *slog.Logger, healthHandler *health.Handler, requestLogSink appmiddleware.RequestLogSink, appEnv string, gatewayNode string) *Server {
	app := fiber.New(fiber.Config{
		AppName: "API Gateway",
	})

	app.Use(requestid.New())
	app.Use(appmiddleware.LoggerWithSink(requestLogSink, logger, appEnv, gatewayNode))
	app.Use(recover.New())
	controlPlaneCORS := cors.New()
	app.Use(func(c *fiber.Ctx) error {
		path := c.Path()
		if path == "/health" || path == "/ready" || strings.HasPrefix(path, "/auth/") || strings.HasPrefix(path, "/admin/") {
			return controlPlaneCORS(c)
		}
		return c.Next()
	})

	app.Get("/health", healthHandler.Health)
	app.Get("/ready", healthHandler.Ready)

	return &Server{
		App:    app,
		Logger: logger,
	}
}

func (s *Server) Run(port string) error {
	s.Logger.Info("starting server", "port", port)
	return s.App.Listen(":" + port)
}
