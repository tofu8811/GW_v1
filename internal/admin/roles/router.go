package roles

import (
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"
)

func RegisterRoleRoutes(router fiber.Router, db *pgxpool.Pool) {
	handler := NewHandler(NewRepository(db))
	router.Get("/", handler.FindAll)
	router.Get("/:id/permissions", handler.FindPermissions)
	router.Get("/:id", handler.FindByID)
}
