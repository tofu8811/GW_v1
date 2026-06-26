package main

import (
	"log"

	"gateway-api/config"
	"gateway-api/infrastructure/logger"
	"gateway-api/infrastructure/postgres"
	redisclient "gateway-api/infrastructure/redis"
	"gateway-api/internal/admin"
	"gateway-api/internal/auth"
	"gateway-api/internal/gateway"
	"gateway-api/internal/health"
	"gateway-api/internal/server"
)

func main() {
	cfg := config.Load()
	logg := logger.New(cfg.AppEnv)

	db, err := postgres.NewPool(cfg.DatabaseURL)
	if err != nil {
		logg.Error("failed to connect postgres", "error", err)
		log.Fatal(err)
	}
	defer db.Close()

	rdb, err := redisclient.NewClient(cfg.RedisAddr, cfg.RedisPass, cfg.RedisDB)
	if err != nil {
		logg.Error("failed to connect redis", "error", err)
		log.Fatal(err)
	}
	defer rdb.Close()

	healthHandler := health.NewHandler(db, rdb)
	srv := server.New(logg, healthHandler)

	auth.RegisterAuthRoutes(srv.App, db, cfg.JWTSecret, cfg.JWTAccessTTL, cfg.AppEnv)
	admin.RegisterAdminRoutes(srv.App, db)
	gateway.RegisterGatewayRoutes(srv.App, db, logg)

	if err := srv.Run(cfg.AppPort); err != nil {
		logg.Error("server stopped", "error", err)
		log.Fatal(err)
	}
}
