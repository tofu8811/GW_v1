package clients

import (
	"gateway-api/internal/middleware"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"
)

func RegisterClientRoutes(router fiber.Router, db *pgxpool.Pool, notifier ConfigNotifier) {
	repository := NewRepository(db)
	handler := NewHandler(repository, notifier)

	router.Post("/", middleware.RequirePermission("clients:write"), handler.Create)
	router.Get("/", middleware.RequirePermission("clients:read"), handler.FindAll)
	router.Get("/:id", middleware.RequirePermission("clients:read"), handler.FindByID)
	router.Put("/:id", middleware.RequirePermission("clients:write"), handler.Update)
	router.Delete("/:id", middleware.RequirePermission("clients:write"), handler.Delete)
}
