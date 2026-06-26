package auth

import (
	"time"

	"gateway-api/internal/middleware"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"
)

func RegisterAuthRoutes(app *fiber.App, db *pgxpool.Pool, jwtSecret string, accessTTL time.Duration, issuer string) {
	repository := NewRepository(db)
	handler := NewHandler(repository, jwtSecret, accessTTL, issuer)

	auth := app.Group("/auth")
	auth.Post("/login", handler.Login)
	auth.Get("/me", middleware.JWTAuth(jwtSecret), handler.Me)
}
