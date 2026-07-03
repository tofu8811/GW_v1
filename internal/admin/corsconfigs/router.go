package corsconfigs

import (
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"
)

func RegisterCORSConfigRoutes(router fiber.Router, db *pgxpool.Pool, notifier ConfigNotifier) {
	handler := NewHandler(NewRepository(db), notifier)

	router.Get("/:id/cors", handler.FindByRouteID)
	router.Put("/:id/cors", handler.Upsert)
	router.Delete("/:id/cors", handler.Delete)
}
