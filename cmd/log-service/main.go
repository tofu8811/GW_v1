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
	"gateway-api/internal/logservice/api"
	logconsumer "gateway-api/internal/logservice/consumer"
	loges "gateway-api/internal/logservice/elasticsearch"
	"gateway-api/internal/middleware"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/gofiber/fiber/v2/middleware/requestid"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cfg := config.Load()
	logg := logger.New(cfg.AppEnv)
	if err := cfg.Validate(); err != nil {
		logg.Error("invalid configuration", "error", err)
		log.Fatal(err)
	}

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

	esClient := loges.NewClient(cfg.ElasticsearchURL, cfg.ElasticsearchIndexPrefix)
	realtimeHub := api.NewRealtimeHub()
	consumer, err := logconsumer.New(logconsumer.Config{
		URL:           cfg.RabbitMQURL,
		Exchange:      cfg.RabbitMQLogExchange,
		Queue:         cfg.RabbitMQLogQueue,
		RoutingKey:    "gateway.request.*",
		DLX:           cfg.RabbitMQLogDLX,
		DLQ:           cfg.RabbitMQLogDLQ,
		BatchSize:     cfg.LogConsumerBatchSize,
		FlushInterval: cfg.LogConsumerFlushInterval,
		Prefetch:      cfg.LogConsumerPrefetch,
	}, esClient, logg, realtimeHub)
	if err != nil {
		logg.Error("failed to create log consumer", "error", err)
		log.Fatal(err)
	}
	defer consumer.Close()

	go func() {
		if err := consumer.Start(ctx); err != nil && ctx.Err() == nil {
			logg.Error("log consumer stopped", "error", err)
			stop()
		}
	}()

	app := fiber.New(fiber.Config{AppName: "Gateway Log Service"})
	app.Use(requestid.New())
	app.Use(recover.New())
	app.Use(cors.New(cors.Config{
		AllowOrigins:  "http://localhost:5173,http://127.0.0.1:5173",
		AllowMethods:  fiber.MethodGet + "," + fiber.MethodPost + "," + fiber.MethodPut + "," + fiber.MethodPatch + "," + fiber.MethodDelete + "," + fiber.MethodOptions,
		AllowHeaders:  "Origin,Content-Type,Accept,Authorization,Cache-Control",
		ExposeHeaders: "Content-Type,Cache-Control,Connection",
	}))
	app.Options("/*", func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusNoContent)
	})

	api.RegisterRoutes(app, api.NewHandler(esClient, api.NewPostgresRouteScopeStore(db), realtimeHub), middleware.JWTAuth(cfg.JWTSecret, rdb, db))

	go func() {
		<-ctx.Done()
		if err := app.Shutdown(); err != nil {
			logg.Warn("failed to shutdown log service", "error", err)
		}
	}()

	logg.Info("starting log service", "port", cfg.LogServicePort)
	if err := app.Listen(":" + cfg.LogServicePort); err != nil {
		logg.Error("log service stopped", "error", err)
		log.Fatal(err)
	}
}
