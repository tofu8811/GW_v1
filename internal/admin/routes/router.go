package routes

import (
	"gateway-api/internal/middleware"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"
)

func RegisterRouteRoutes(router fiber.Router, db *pgxpool.Pool, notifier ConfigNotifier) {
	repository := NewRepository(db)
	handler := NewHandler(repository, notifier)

	router.Post("/", middleware.RequirePermission("routes:write"), handler.Create)
	router.Get("/", middleware.RequirePermission("routes:read"), handler.FindAll)
	router.Get("/:id", middleware.RequirePermission("routes:read"), handler.FindByID)
	router.Put("/:id", middleware.RequirePermission("routes:write"), handler.Update)
	router.Delete("/:id", middleware.RequirePermission("routes:write"), handler.Delete)
}
