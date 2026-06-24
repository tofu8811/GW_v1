package admin

import (
	adminInstances "gateway-api/internal/admin/instances"
	adminRoutes "gateway-api/internal/admin/routes"
	adminServices "gateway-api/internal/admin/services"

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

	adminServices.RegisterServiceRoutes(admin.Group("/services"), db)
	adminInstances.RegisterServiceInstanceRoutes(admin.Group("/services"), db)
	adminInstances.RegisterInstanceRoutes(admin.Group("/instances"), db)
	adminRoutes.RegisterRouteRoutes(admin.Group("/routes"), db)
}
