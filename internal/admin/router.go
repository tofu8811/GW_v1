package admin

import (
	adminRoutes "gateway-api/internal/admin/routes"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"
)

func RegisterAdminRoutes(app *fiber.App, db *pgxpool.Pool, middlewares ...fiber.Handler) {
	admin := app.Group("/admin")

	for _, middleware := range middlewares {
		if middleware != nil {
			admin.Use(middleware)
		}
	}

	adminRoutes.RegisterRouteRoutes(admin.Group("/routes"), db)
}
