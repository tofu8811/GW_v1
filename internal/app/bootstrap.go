package app

import (
	"context"

	"gateway-api/config"
	"gateway-api/infrastructure/logger"
	"gateway-api/infrastructure/postgres"
	"gateway-api/infrastructure/rabbitmq"
	redisclient "gateway-api/infrastructure/redis"
	"gateway-api/internal/admin"
	"gateway-api/internal/auth"
	configcache "gateway-api/internal/config/cache"
	"gateway-api/internal/health"
	"gateway-api/internal/middleware"
	"gateway-api/internal/proxy"
	"gateway-api/internal/security/ipblacklist"
	"gateway-api/internal/server"
	"gateway-api/internal/upstream/breaker"
	upstreamhealth "gateway-api/internal/upstream/health"
)

func RunGateway(ctx context.Context) error {
	deps, err := initGatewayDependencies()
	if err != nil {
		return err
	}
	defer deps.close()

	cfg := deps.config
	logg := deps.logger

	db, err := postgres.NewPool(cfg.DatabaseURL)
	if err != nil {
		logg.Error("failed to connect postgres", "error", err)
		return err
	}
	defer db.Close()

	rdb, err := redisclient.NewClient(cfg.RedisAddr, cfg.RedisPass, cfg.RedisDB)
	if err != nil {
		logg.Error("failed to connect redis", "error", err)
		return err
	}
	defer rdb.Close()

	cacheStore := configcache.NewStore(db, rdb, logg, configcache.Config{
		PollInterval:    cfg.ConfigPollInterval,
		ConfigTTL:       cfg.ConfigTTL,
		RebuildLockTTL:  cfg.ConfigRebuildLockTTL,
		RebuildLockWait: cfg.ConfigLockWait,
		SchemaVersion:   cfg.ConfigSchemaVersion,
		CORSSource:      cfg.CORSSource,
	})

	if err := cacheStore.WarmAll(ctx); err != nil {
		logg.Error("failed to warm config cache", "error", err)
		return err
	}
	configNotifier := configcache.NewNotifier(rdb)
	configSubscriber := configcache.NewSubscriber(rdb, logg)
	configPoller := configcache.NewPoller(rdb, logg, cfg.ConfigPollInterval)
	go configSubscriber.SubscribeReload(ctx, cacheStore.RebuildAll)
	go configPoller.PollVersion(ctx, cacheStore.CurrentVersion, cacheStore.RebuildAll)

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
	srv := server.New(logg, healthHandler, deps.requestLogSink, cfg.AppEnv, cfg.GatewayNode)
	ipBlacklistChecker := ipblacklist.NewChecker(db, rdb, logg)
	if err := ipBlacklistChecker.Reload(ctx); err != nil {
		logg.Error("failed to warm ip blacklist cache", "error", err)
		return err
	}

	auth.RegisterAuthRoutes(srv.App, db, rdb, cfg.JWTSecret, cfg.JWTAccessTTL, cfg.JWTRefreshTTL, cfg.AppEnv)
	jwtAuth := middleware.JWTAuth(cfg.JWTSecret, rdb, db)
	admin.RegisterAdminRoutes(srv.App, db, rdb, cacheStore, configNotifier, upstreamHealthStore, upstreamChecker, ipBlacklistChecker, jwtAuth)

	apiKeyLastUsedBatcher := middleware.NewAPIKeyLastUsedBatcher(db, logg, 0)
	defer apiKeyLastUsedBatcher.Flush(context.Background())
	go apiKeyLastUsedBatcher.Start(ctx)
	gatewayAuth := middleware.NewGatewayAuth(db, rdb, cacheStore, apiKeyLastUsedBatcher, cfg.JWTSecret)
	proxy.RegisterGatewayRoutes(srv.App, cacheStore, rdb, logg, upstreamHealthFilter, breakers, gatewayAuth, ipBlacklistChecker)

	if err := srv.Run(cfg.AppPort); err != nil {
		logg.Error("server stopped", "error", err)
		return err
	}

	return nil
}

func initGatewayDependencies() (*gatewayDependencies, error) {
	cfg := config.Load()
	logg := logger.New(cfg.AppEnv)
	if err := cfg.Validate(); err != nil {
		logg.Error("invalid configuration", "error", err)
		return nil, err
	}

	requestLogFile, err := logger.OpenJSONLogFile(cfg.LogFilePath)
	if err != nil {
		logg.Error("failed to open request log file", "error", err, "path", cfg.LogFilePath)
		return nil, err
	}

	deps := &gatewayDependencies{
		config:         cfg,
		logger:         logg,
		requestLogSink: middleware.NewJSONLineSink(requestLogFile),
		cleanup: []func(){
			func() {
				if err := requestLogFile.Close(); err != nil {
					logg.Warn("failed to close request log file", "error", err)
				}
			},
		},
	}

	if cfg.RabbitMQURL != "" {
		logPublisher, err := rabbitmq.NewLogPublisher(rabbitmq.LogPublisherConfig{
			URL:        cfg.RabbitMQURL,
			Exchange:   cfg.RabbitMQLogExchange,
			RoutingKey: cfg.RabbitMQLogRoutingKey,
			Timeout:    cfg.RabbitMQPublishTimeout,
		}, logg)
		if err != nil {
			logg.Warn("failed to create rabbitmq log publisher, falling back to file logs", "error", err)
		} else {
			deps.cleanup = append(deps.cleanup, func() {
				if err := logPublisher.Close(); err != nil {
					logg.Warn("failed to close rabbitmq log publisher", "error", err)
				}
			})
			deps.requestLogSink = logPublisher
		}
	}

	return deps, nil
}
