package users

import (
	"gateway-api/internal/middleware"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"
)

func RegisterUserRoutes(router fiber.Router, db *pgxpool.Pool) {
	handler := NewHandler(NewRepository(db))

	router.Get("/", middleware.RequirePermission("users:read"), handler.FindAll)
	router.Post("/", middleware.RequirePermission("users:write"), handler.Create)
	router.Get("/:id", middleware.RequirePermission("users:read"), handler.FindByID)
	router.Put("/:id", middleware.RequirePermission("users:write"), handler.Update)
	router.Delete("/:id", middleware.RequirePermission("users:write"), handler.Delete)
}
