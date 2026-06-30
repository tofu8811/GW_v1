package instances

import (
	upstreamhealth "gateway-api/internal/upstream/health"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"
)

func RegisterServiceInstanceRoutes(router fiber.Router, db *pgxpool.Pool, notifier ConfigNotifier) {
	repository := NewRepository(db)
	handler := NewHandler(repository, notifier, nil, nil)

	router.Post("/:id/instances", handler.CreateForService)
	router.Get("/:id/instances", handler.FindByServiceID)
}

func RegisterInstanceRoutes(router fiber.Router, db *pgxpool.Pool, notifier ConfigNotifier, healthStore *upstreamhealth.Store, healthChecker HealthChecker) {
	repository := NewRepository(db)
	handler := NewHandler(repository, notifier, healthStore, healthChecker)

	router.Get("/", handler.FindAll)
	router.Get("/:id/health", handler.GetHealth)
	router.Post("/:id/health-check", handler.CheckHealth)
	router.Get("/:id", handler.FindByID)
	router.Put("/:id", handler.Update)
	router.Delete("/:id", handler.Delete)
}
