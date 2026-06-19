package health

import (
	"context"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
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
	return c.JSON(fiber.Map{
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

	statusCode := fiber.StatusOK
	overall := "ok"

	if postgresStatus != "ok" || redisStatus != "ok" {
		statusCode = fiber.StatusServiceUnavailable
		overall = "error"
	}

	return c.Status(statusCode).JSON(fiber.Map{
		"status":   overall,
		"postgres": postgresStatus,
		"redis":    redisStatus,
	})
}
