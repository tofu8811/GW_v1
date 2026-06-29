package main

import (
	"context"
	"log"
	"os/signal"
	"syscall"

	"gateway-api/config"
	"gateway-api/infrastructure/logger"
	"gateway-api/infrastructure/postgres"
	redisclient "gateway-api/infrastructure/redis"
	"gateway-api/internal/admin"
	"gateway-api/internal/auth"
	configcache "gateway-api/internal/config/cache"
	"gateway-api/internal/health"
	"gateway-api/internal/proxy"
	"gateway-api/internal/server"
	"gateway-api/internal/upstream/breaker"
	upstreamhealth "gateway-api/internal/upstream/health"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

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

	cacheStore := configcache.NewStore(db, rdb, logg, configcache.Config{
		PollInterval:    cfg.ConfigPollInterval,
		ConfigTTL:       cfg.ConfigTTL,
		RebuildLockTTL:  cfg.ConfigRebuildLockTTL,
		RebuildLockWait: cfg.ConfigLockWait,
		SchemaVersion:   cfg.ConfigSchemaVersion,
	})

	if err := cacheStore.WarmAll(ctx); err != nil {
		logg.Error("failed to warm config cache", "error", err)
		log.Fatal(err)
	}
	go cacheStore.SubscribeReload(ctx)
	go cacheStore.PollVersion(ctx)

	breakers := breaker.NewRegistry(breaker.Config{
		FailureThreshold: cfg.BreakerFailureThreshold,
		OpenTimeout:      cfg.BreakerOpenTimeout,
		HalfOpenMax:      cfg.BreakerHalfOpenMax,
	})
	upstreamHealthStore := upstreamhealth.NewStore(rdb, cfg.HealthKeyTTL)
	upstreamChecker := upstreamhealth.NewChecker(upstreamHealthStore, cacheStore, breakers, upstreamhealth.Config{
		Interval:           cfg.HealthCheckInterval,
		ProbeTimeout:       cfg.HealthProbeTimeout,
		UnhealthyThreshold: cfg.HealthUnhealthyThreshold,
		HealthyThreshold:   cfg.HealthHealthyThreshold,
	}, logg)
	go upstreamChecker.Start(ctx)
	upstreamHealthFilter := upstreamhealth.NewHealthFilter(upstreamHealthStore, breakers)

	healthHandler := health.NewHandler(db, rdb, cacheStore.Ready)
	srv := server.New(logg, healthHandler)

	auth.RegisterAuthRoutes(srv.App, db, cfg.JWTSecret, cfg.JWTAccessTTL, cfg.AppEnv)
	admin.RegisterAdminRoutes(srv.App, db, cacheStore, upstreamHealthStore, upstreamChecker)
	proxy.RegisterGatewayRoutes(srv.App, cacheStore, logg, upstreamHealthFilter, breakers)

	if err := srv.Run(cfg.AppPort); err != nil {
		logg.Error("server stopped", "error", err)
		log.Fatal(err)
	}
}
