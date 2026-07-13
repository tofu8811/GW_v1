package permissions

import (
	"gateway-api/internal/middleware"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"
)

func RegisterPermissionRoutes(router fiber.Router, db *pgxpool.Pool) {
	handler := NewHandler(NewRepository(db))
	router.Get("/", middleware.RequirePermission("permissions:read"), handler.FindAll)
	router.Get("/:id", middleware.RequirePermission("permissions:read"), handler.FindByID)
}
