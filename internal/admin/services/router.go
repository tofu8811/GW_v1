package services

import (
	"gateway-api/internal/middleware"
	upstreamhealth "gateway-api/internal/upstream/health"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"
)

func RegisterServiceRoutes(router fiber.Router, db *pgxpool.Pool, notifier ConfigNotifier, configCache ServiceInstanceCache, healthStore *upstreamhealth.Store) {
	repository := NewRepository(db)
	handler := NewHandler(repository, notifier, healthStore, configCache)

	router.Post("/", middleware.RequirePermission("services:write"), handler.Create)
	router.Get("/", middleware.RequirePermission("services:read"), handler.FindAll)
	router.Get("/:id/health", middleware.RequirePermission("health:read"), handler.GetHealth)
	router.Get("/:id", middleware.RequirePermission("services:read"), handler.FindByID)
	router.Put("/:id", middleware.RequirePermission("services:write"), handler.Update)
	router.Delete("/:id", middleware.RequirePermission("services:write"), handler.Delete)
}
