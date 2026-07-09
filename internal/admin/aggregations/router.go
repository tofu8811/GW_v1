package aggregations

import (
	"gateway-api/internal/middleware"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"
)

func RegisterAggregationRoutes(router fiber.Router, db *pgxpool.Pool, notifier ConfigNotifier) {
	handler := NewHandler(NewRepository(db), notifier)

	router.Post("/aggregations", middleware.RequirePermission("aggregations:write"), handler.Create)
	router.Get("/aggregations", middleware.RequirePermission("aggregations:read"), handler.FindAll)
	router.Get("/aggregations/:id", middleware.RequirePermission("aggregations:read"), handler.FindByID)
	router.Put("/aggregations/:id", middleware.RequirePermission("aggregations:write"), handler.Update)
	router.Delete("/aggregations/:id", middleware.RequirePermission("aggregations:write"), handler.Delete)
	router.Post("/aggregations/:id/steps", middleware.RequirePermission("aggregations:write"), handler.CreateStep)
	router.Get("/aggregations/:id/steps", middleware.RequirePermission("aggregations:read"), handler.FindSteps)
	router.Put("/aggregation-steps/:id", middleware.RequirePermission("aggregations:write"), handler.UpdateStep)
	router.Delete("/aggregation-steps/:id", middleware.RequirePermission("aggregations:write"), handler.DeleteStep)
}
