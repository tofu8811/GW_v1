package instances

import (
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"
)

func RegisterServiceInstanceRoutes(router fiber.Router, db *pgxpool.Pool) {
	repository := NewRepository(db)
	handler := NewHandler(repository)

	router.Post("/:id/instances", handler.CreateForService)
	router.Get("/:id/instances", handler.FindByServiceID)
}

func RegisterInstanceRoutes(router fiber.Router, db *pgxpool.Pool) {
	repository := NewRepository(db)
	handler := NewHandler(repository)

	router.Get("/", handler.FindAll)
	router.Get("/:id", handler.FindByID)
	router.Put("/:id", handler.Update)
	router.Delete("/:id", handler.Delete)
}
