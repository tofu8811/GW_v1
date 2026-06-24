package health

import (
	"context"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"gateway-api/helper/response"
)

type Handler struct {
	DB    *pgxpool.Pool
	Redis *redis.Client
}

func NewHandler(db *pgxpool.Pool, redisClient *redis.Client) *Handler {
	return &Handler{
		DB:    db,
		Redis: redisClient,
	}
}

func (h *Handler) Health(c *fiber.Ctx) error {
	return response.OK(c, fiber.Map{
		"status": "ok",
	})
}

func (h *Handler) Ready(c *fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	postgresStatus := "ok"
	redisStatus := "ok"

	if err := h.DB.Ping(ctx); err != nil {
		postgresStatus = "error"
	}

	if err := h.Redis.Ping(ctx).Err(); err != nil {
		redisStatus = "error"
	}

	data := fiber.Map{
		"status":   "ok",
		"postgres": postgresStatus,
		"redis":    redisStatus,
	}

	if postgresStatus != "ok" || redisStatus != "ok" {
		data["status"] = "error"

		return response.ErrorWithDetails(
			c,
			fiber.StatusServiceUnavailable,
			"service_unavailable",
			"One or more dependencies are unavailable",
			data,
		)
	}
	return response.OK(c, data)
}
