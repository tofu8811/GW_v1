package auth

import (
	"time"

	"gateway-api/internal/middleware"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

func RegisterAuthRoutes(app *fiber.App, db *pgxpool.Pool, rdb *redis.Client, jwtSecret string, accessTTL time.Duration, refreshTTL time.Duration, issuer string) {
	repository := NewRepository(db)
	refreshStore := NewRefreshStore(rdb)
	handler := NewHandler(repository, refreshStore, jwtSecret, accessTTL, refreshTTL, issuer)

	auth := app.Group("/auth")
	auth.Post("/login", handler.Login)
	auth.Post("/refresh", handler.Refresh)

	jwtAuth := middleware.JWTAuth(jwtSecret, rdb)
	auth.Get("/me", jwtAuth, handler.Me)
	auth.Post("/logout", jwtAuth, handler.Logout)
}
