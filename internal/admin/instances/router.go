package instances

import (
	configcache "gateway-api/internal/config/cache"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"
)

func RegisterServiceInstanceRoutes(router fiber.Router, db *pgxpool.Pool, notifier *configcache.Store) {
	repository := NewRepository(db)
	handler := NewHandler(repository, notifier)

	router.Post("/:id/instances", handler.CreateForService)
	router.Get("/:id/instances", handler.FindByServiceID)
}

func RegisterInstanceRoutes(router fiber.Router, db *pgxpool.Pool, notifier *configcache.Store) {
	repository := NewRepository(db)
	handler := NewHandler(repository, notifier)

	router.Get("/", handler.FindAll)
	router.Get("/:id", handler.FindByID)
	router.Put("/:id", handler.Update)
	router.Delete("/:id", handler.Delete)
}
