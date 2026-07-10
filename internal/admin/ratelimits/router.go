package ratelimits

import (
	"gateway-api/internal/middleware"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"
)

func RegisterRateLimitPolicyRoutes(router fiber.Router, db *pgxpool.Pool, notifier ConfigNotifier) {
	repository := NewRepository(db)
	handler := NewHandler(repository, notifier)

	router.Post("/", middleware.RequirePermission("rate_limits:write"), handler.Create)
	router.Get("/", middleware.RequirePermission("rate_limits:read"), handler.FindAll)
	router.Get("/:id", middleware.RequirePermission("rate_limits:read"), handler.FindByID)
	router.Put("/:id", middleware.RequirePermission("rate_limits:write"), handler.Update)
	router.Delete("/:id", middleware.RequirePermission("rate_limits:write"), handler.Delete)
}
