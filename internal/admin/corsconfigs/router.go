package corsconfigs

import (
	"gateway-api/internal/middleware"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"
)

func RegisterCORSConfigRoutes(router fiber.Router, db *pgxpool.Pool, notifier ConfigNotifier) {
	handler := NewHandler(NewRepository(db), notifier)

	router.Get("/:id/cors", middleware.RequirePermission("routes:read"), handler.FindByRouteID)
	router.Put("/:id/cors", middleware.RequirePermission("routes:write"), handler.Upsert)
	router.Delete("/:id/cors", middleware.RequirePermission("routes:write"), handler.Delete)
}
