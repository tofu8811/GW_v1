package routes

import (
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"
)

func RegisterRouteRoutes(router fiber.Router, db *pgxpool.Pool, notifier ConfigNotifier) {
	repository := NewRepository(db)
	handler := NewHandler(repository, notifier)

	router.Post("/", handler.Create)
	router.Get("/", handler.FindAll)
	router.Get("/:id", handler.FindByID)
	router.Put("/:id", handler.Update)
	router.Delete("/:id", handler.Delete)
}
