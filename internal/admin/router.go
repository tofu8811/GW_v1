package admin

import (
	adminInstances "gateway-api/internal/admin/instances"
	adminRoutes "gateway-api/internal/admin/routes"
	adminServices "gateway-api/internal/admin/services"
	configcache "gateway-api/internal/config/cache"
	upstreamhealth "gateway-api/internal/upstream/health"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"
)

func RegisterAdminRoutes(app *fiber.App, db *pgxpool.Pool, notifier *configcache.Store, healthStore *upstreamhealth.Store, healthChecker adminInstances.HealthChecker, middlewares ...fiber.Handler) {
	admin := app.Group("/admin")

	for _, middleware := range middlewares {
		if middleware != nil {
			admin.Use(middleware)
		}
	}

	adminServices.RegisterServiceRoutes(admin.Group("/services"), db, notifier, healthStore)
	adminInstances.RegisterServiceInstanceRoutes(admin.Group("/services"), db, notifier)
	adminInstances.RegisterInstanceRoutes(admin.Group("/instances"), db, notifier, healthStore, healthChecker)
	adminRoutes.RegisterRouteRoutes(admin.Group("/routes"), db, notifier)
}
