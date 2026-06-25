package gateway

import (
	"log/slog"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"
)

func RegisterGatewayRoutes(app *fiber.App, db *pgxpool.Pool, logger *slog.Logger) {
	repository := NewRepository(db)
	handler := NewHandler(repository, logger)

	app.Use(handler.Proxy)
}
