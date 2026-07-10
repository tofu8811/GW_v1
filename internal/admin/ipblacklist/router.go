package ipblacklist

import (
	"gateway-api/internal/middleware"
	runtimeipblacklist "gateway-api/internal/security/ipblacklist"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"
)

func RegisterIPBlacklistRoutes(router fiber.Router, db *pgxpool.Pool, checker *runtimeipblacklist.Checker) {
	repository := NewRepository(db)
	handler := NewHandler(repository, checker)

	router.Post("/", middleware.RequirePermission("ip_blacklist:write"), handler.Create)
	router.Get("/", middleware.RequirePermission("ip_blacklist:read"), handler.FindAll)
	router.Get("/:id", middleware.RequirePermission("ip_blacklist:read"), handler.FindByID)
	router.Put("/:id", middleware.RequirePermission("ip_blacklist:write"), handler.Update)
	router.Delete("/:id", middleware.RequirePermission("ip_blacklist:write"), handler.Delete)
}

// GET /admin/ip-blacklist?deleted_only=true
// GET /admin/ip-blacklist?include_deleted=true
