package roles

import (
	"gateway-api/internal/middleware"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"
)

func RegisterRoleRoutes(router fiber.Router, db *pgxpool.Pool) {
	handler := NewHandler(NewRepository(db))
	router.Get("/", middleware.RequirePermission("roles:read"), handler.FindAll)
	router.Get("/:id/permissions", middleware.RequirePermission("roles:read"), handler.FindPermissions)
	router.Get("/:id", middleware.RequirePermission("roles:read"), handler.FindByID)
}
