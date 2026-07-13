package apikeys

import (
	"gateway-api/internal/middleware"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"
)

func RegisterAPIKeyRoutes(router fiber.Router, db *pgxpool.Pool, notifier ConfigNotifier) {
	handler := NewHandler(NewRepository(db), notifier)

	router.Post("/", middleware.RequirePermission("api_keys:write"), handler.Create)
	router.Get("/", middleware.RequirePermission("api_keys:read"), handler.FindAll)
	router.Get("/options", middleware.RequirePermission("api_keys:read"), handler.Options)
	router.Get("/:id", middleware.RequirePermission("api_keys:read"), handler.FindByID)
	router.Put("/:id", middleware.RequirePermission("api_keys:write"), handler.Update)
	router.Post("/:id/revoke", middleware.RequirePermission("api_keys:write"), handler.Revoke)
	router.Post("/:id/rotate", middleware.RequirePermission("api_keys:write"), handler.Rotate)
}
