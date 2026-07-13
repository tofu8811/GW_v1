package corspolicies

import (
	"gateway-api/internal/middleware"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"
)

func RegisterCORSPolicyRoutes(router fiber.Router, db *pgxpool.Pool, notifier ConfigNotifier) {
	handler := NewHandler(NewRepository(db), notifier)

	router.Post("/", middleware.RequirePermission("cors_policies:write"), handler.Create)
	router.Get("/", middleware.RequirePermission("cors_policies:read"), handler.FindAll)
	router.Get("/:id", middleware.RequirePermission("cors_policies:read"), handler.FindByID)
	router.Put("/:id", middleware.RequirePermission("cors_policies:write"), handler.Update)
	router.Delete("/:id", middleware.RequirePermission("cors_policies:write"), handler.Delete)
}
