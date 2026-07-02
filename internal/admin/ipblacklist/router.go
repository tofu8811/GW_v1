package ipblacklist

import (
	runtimeipblacklist "gateway-api/internal/security/ipblacklist"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"
)

func RegisterIPBlacklistRoutes(router fiber.Router, db *pgxpool.Pool, checker *runtimeipblacklist.Checker) {
	repository := NewRepository(db)
	handler := NewHandler(repository, checker)

	router.Post("/", handler.Create)
	router.Get("/", handler.FindAll)
	router.Get("/:id", handler.FindByID)
	router.Put("/:id", handler.Update)
	router.Delete("/:id", handler.Delete)
}
// GET /admin/ip-blacklist?deleted_only=true
// GET /admin/ip-blacklist?include_deleted=true