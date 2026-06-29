package services

import (
	configcache "gateway-api/internal/config/cache"
	upstreamhealth "gateway-api/internal/upstream/health"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"
)

func RegisterServiceRoutes(router fiber.Router, db *pgxpool.Pool, notifier *configcache.Store, healthStore *upstreamhealth.Store) {
	repository := NewRepository(db)
	handler := NewHandler(repository, notifier, healthStore, notifier)

	router.Post("/", handler.Create)
	router.Get("/", handler.FindAll)
	router.Get("/:id/health", handler.GetHealth)
	router.Get("/:id", handler.FindByID)
	router.Put("/:id", handler.Update)
	router.Delete("/:id", handler.Delete)
}
