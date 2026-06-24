package proxy

import (
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"
)

func RegisterProxyRoutes(app *fiber.App, db *pgxpool.Pool) {
	repository := NewRepository(db)
	handler := NewHandler(repository)

	app.All("/*", handler.Proxy)
}
