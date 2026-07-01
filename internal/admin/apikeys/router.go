package apikeys

import (
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"
)

func RegisterAPIKeyRoutes(router fiber.Router, db *pgxpool.Pool) {
	handler := NewHandler(NewRepository(db))

	router.Post("/", handler.Create)
	router.Get("/", handler.FindAll)
	router.Get("/:id", handler.FindByID)
	router.Put("/:id", handler.Update)
	router.Post("/:id/revoke", handler.Revoke)
	router.Post("/:id/rotate", handler.Rotate)
}
