package admin

import (
	"context"

	adminAPIKeys "gateway-api/internal/admin/apikeys"
	adminCache "gateway-api/internal/admin/cache"
	adminInstances "gateway-api/internal/admin/instances"
	adminRateLimits "gateway-api/internal/admin/ratelimits"
	adminRoutes "gateway-api/internal/admin/routes"
	adminServices "gateway-api/internal/admin/services"
	configcache "gateway-api/internal/config/cache"
	upstreamhealth "gateway-api/internal/upstream/health"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

type ConfigNotifier interface {
	BumpVersion(ctx context.Context) (int64, error)
	PublishReload(ctx context.Context, group string) error
	NotifyChange(ctx context.Context, group string) error
}

func RegisterAdminRoutes(app *fiber.App, db *pgxpool.Pool, redisClient *redis.Client, cacheStore *configcache.Store, notifier ConfigNotifier, healthStore *upstreamhealth.Store, healthChecker adminInstances.HealthChecker, middlewares ...fiber.Handler) {
	admin := app.Group("/admin")

	for _, middleware := range middlewares {
		if middleware != nil {
			admin.Use(middleware)
		}
	}

	adminCache.RegisterCacheRoutes(admin.Group("/cache"), cacheStore, notifier, redisClient)
	adminAPIKeys.RegisterAPIKeyRoutes(admin.Group("/api-keys"), db)
	adminServices.RegisterServiceRoutes(admin.Group("/services"), db, notifier, cacheStore, healthStore)
	adminInstances.RegisterServiceInstanceRoutes(admin.Group("/services"), db, notifier)
	adminInstances.RegisterInstanceRoutes(admin.Group("/instances"), db, notifier, healthStore, healthChecker)
	adminRoutes.RegisterRouteRoutes(admin.Group("/routes"), db, notifier)
	adminRateLimits.RegisterRateLimitPolicyRoutes(admin.Group("/rate-limit-policies"), db, notifier)
}
