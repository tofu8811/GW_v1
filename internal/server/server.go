package server

import (
	"log/slog"

	"gateway-api/internal/health"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/gofiber/fiber/v2/middleware/requestid"
)

type Server struct {
	App    *fiber.App
	Logger *slog.Logger
}

func New(logger *slog.Logger, healthHandler *health.Handler) *Server {
	app := fiber.New(fiber.Config{
		AppName: "API Gateway",
	})

	app.Use(recover.New())
	app.Use(requestid.New())
	app.Use(cors.New())

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
