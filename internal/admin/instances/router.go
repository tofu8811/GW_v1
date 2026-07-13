package instances

import (
	"gateway-api/internal/middleware"
	upstreamhealth "gateway-api/internal/upstream/health"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"
)

func RegisterServiceInstanceRoutes(router fiber.Router, db *pgxpool.Pool, notifier ConfigNotifier) {
	repository := NewRepository(db)
	handler := NewHandler(repository, notifier, nil, nil)

	router.Post("/:id/instances", middleware.RequirePermission("services:write"), handler.CreateForService)
	router.Get("/:id/instances", middleware.RequirePermission("services:read"), handler.FindByServiceID)
}

func RegisterInstanceRoutes(router fiber.Router, db *pgxpool.Pool, notifier ConfigNotifier, healthStore *upstreamhealth.Store, healthChecker HealthChecker) {
	repository := NewRepository(db)
	handler := NewHandler(repository, notifier, healthStore, healthChecker)

	router.Get("/", middleware.RequirePermission("services:read"), handler.FindAll)
	router.Get("/:id/health", middleware.RequirePermission("health:read"), handler.GetHealth)
	router.Post("/:id/health-check", middleware.RequirePermission("health:write"), handler.CheckHealth)
	router.Get("/:id", middleware.RequirePermission("services:read"), handler.FindByID)
	router.Put("/:id", middleware.RequirePermission("services:write"), handler.Update)
	router.Delete("/:id", middleware.RequirePermission("services:write"), handler.Delete)
}
