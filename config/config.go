package config

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

const defaultJWTSecret = "change_me_in_local_env"

type Config struct {
	AppEnv      string
	AppPort     string
	LogFilePath string
	GatewayNode string
	DatabaseURL string
	RedisAddr   string
	RedisPass   string
	RedisDB     int

	TrustedProxies []string

	JWTSecret     string
	JWTAccessTTL  time.Duration
	JWTRefreshTTL time.Duration

	ConfigPollInterval   time.Duration
	ConfigTTL            time.Duration
	ConfigRebuildLockTTL time.Duration
	ConfigLockWait       time.Duration
	ConfigSchemaVersion  int
	CORSSource           string

	HealthCheckInterval      time.Duration
	HealthProbeTimeout       time.Duration
	HealthUnhealthyThreshold int
	HealthHealthyThreshold   int
	HealthKeyTTL             time.Duration

	BreakerFailureThreshold int
	BreakerOpenTimeout      time.Duration
	BreakerHalfOpenMax      int

	RabbitMQURL              string
	RabbitMQLogExchange      string
	RabbitMQLogQueue         string
	RabbitMQLogRoutingKey    string
	RabbitMQLogDLX           string
	RabbitMQLogDLQ           string
	RabbitMQPublishTimeout   time.Duration
	ElasticsearchURL         string
	ElasticsearchIndexPrefix string
	LogConsumerBatchSize     int
	LogConsumerFlushInterval time.Duration
	LogConsumerPrefetch      int
	LogServicePort           string
}

func Load() Config {
	loadDotEnv()

	redisDB, _ := strconv.Atoi(getEnv("REDIS_DB", "0"))
	jwtAccessTTL := getDurationEnv("JWT_ACCESS_TOKEN_TTL", 15*time.Minute)
	jwtRefreshTTL := getDurationEnv("JWT_REFRESH_TOKEN_TTL", 7*24*time.Hour)
	schemaVersion, _ := strconv.Atoi(getEnv("CONFIG_SCHEMA_VERSION", "2"))
	databaseURL := getEnv("DATABASE_URL", "")
	if databaseURL == "" {
		databaseURL = buildDatabaseURL()
	}

	healthInterval := durationEnv("HEALTH_CHECK_INTERVAL", 10*time.Second)
	healthTTL := durationEnv("HEALTH_KEY_TTL", 30*time.Second)
	if healthInterval >= healthTTL {
		healthInterval = healthTTL / 2
	}

	return Config{
		AppEnv:      getEnv("APP_ENV", "development"),
		AppPort:     getEnv("APP_PORT", "8080"),
		LogFilePath: getEnv("LOG_FILE", "logs/gateway.jsonl"),
		GatewayNode: getEnv("GATEWAY_NODE", "gateway-api-1"),
		DatabaseURL: databaseURL,
		RedisAddr:   getEnv("REDIS_ADDR", "localhost:6379"),
		RedisPass:   getEnv("REDIS_PASSWORD", ""),
		RedisDB:     redisDB,

		TrustedProxies: splitCSV(getEnv("TRUSTED_PROXIES", "")),

		JWTSecret:     getEnv("JWT_SECRET", defaultJWTSecret),
		JWTAccessTTL:  jwtAccessTTL,
		JWTRefreshTTL: jwtRefreshTTL,

		ConfigPollInterval:   durationSeconds("CONFIG_POLL_INTERVAL_SECONDS", 20*time.Second),
		ConfigTTL:            durationSeconds("CONFIG_TTL_SECONDS", 0),
		ConfigRebuildLockTTL: durationSeconds("CONFIG_REBUILD_LOCK_TTL_SECONDS", 10*time.Second),
		ConfigLockWait:       durationSeconds("CONFIG_REBUILD_LOCK_WAIT_SECONDS", 2*time.Second),
		ConfigSchemaVersion:  schemaVersion,
		CORSSource:           strings.ToLower(strings.TrimSpace(getEnv("CORS_SOURCE", "policy"))),

		HealthCheckInterval:      healthInterval,
		HealthProbeTimeout:       durationEnv("HEALTH_PROBE_TIMEOUT", 2*time.Second),
		HealthUnhealthyThreshold: intEnv("HEALTH_UNHEALTHY_THRESHOLD", 3),
		HealthHealthyThreshold:   intEnv("HEALTH_HEALTHY_THRESHOLD", 2),
		HealthKeyTTL:             healthTTL,

		BreakerFailureThreshold: intEnv("BREAKER_FAILURE_THRESHOLD", 5),
		BreakerOpenTimeout:      durationEnv("BREAKER_OPEN_TIMEOUT", 15*time.Second),
		BreakerHalfOpenMax:      intEnv("BREAKER_HALFOPEN_MAX", 1),

		RabbitMQURL:              getEnv("RABBITMQ_URL", ""),
		RabbitMQLogExchange:      getEnv("RABBITMQ_LOG_EXCHANGE", "gateway.logs.exchange"),
		RabbitMQLogQueue:         getEnv("RABBITMQ_LOG_QUEUE", "gateway.logs.queue"),
		RabbitMQLogRoutingKey:    getEnv("RABBITMQ_LOG_ROUTING_KEY", "gateway.request.completed"),
		RabbitMQLogDLX:           getEnv("RABBITMQ_LOG_DLX", "gateway.logs.dlx"),
		RabbitMQLogDLQ:           getEnv("RABBITMQ_LOG_DLQ", "gateway.logs.dlq"),
		RabbitMQPublishTimeout:   durationEnv("RABBITMQ_PUBLISH_TIMEOUT", 500*time.Millisecond),
		ElasticsearchURL:         getEnv("ELASTICSEARCH_URL", "http://localhost:9200"),
		ElasticsearchIndexPrefix: getEnv("ELASTICSEARCH_LOG_INDEX_PREFIX", "gateway-logs"),
		LogConsumerBatchSize:     intEnv("LOG_CONSUMER_BATCH_SIZE", 500),
		LogConsumerFlushInterval: durationEnv("LOG_CONSUMER_FLUSH_INTERVAL", time.Second),
		LogConsumerPrefetch:      intEnv("LOG_CONSUMER_PREFETCH", 100),
		LogServicePort:           getEnv("LOG_SERVICE_PORT", "8081"),
	}
}

func (c Config) Validate() error {
	secret := strings.TrimSpace(c.JWTSecret)
	if secret == "" {
		return errors.New("JWT_SECRET is required")
	}
	if strings.EqualFold(strings.TrimSpace(c.AppEnv), "production") && secret == defaultJWTSecret {
		return errors.New("JWT_SECRET must be changed in production")
	}
	return nil
}

func loadDotEnv() {
	candidates := []string{}

	if wd, err := os.Getwd(); err == nil {
		candidates = append(candidates, envCandidatesFrom(wd)...)
	}

	if exe, err := os.Executable(); err == nil {
		candidates = append(candidates, envCandidatesFrom(filepath.Dir(exe))...)
	}

	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			_ = godotenv.Overload(candidate)
			return
		}
	}
}

func envCandidatesFrom(start string) []string {
	var candidates []string

	dir, err := filepath.Abs(start)
	if err != nil {
		return candidates
	}

	for {
		candidates = append(candidates, filepath.Join(dir, ".env"))

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return candidates
}

func buildDatabaseURL() string {
	host := getEnv("POSTGRES_HOST", "127.0.0.1")
	port := getEnv("POSTGRES_PORT", "5432")
	user := getEnv("POSTGRES_USER", "gateway_user")
	password := getEnv("POSTGRES_PASSWORD", "gateway_password")
	db := getEnv("POSTGRES_DB", "gateway_db")

	return "postgres://" + user + ":" + password + "@" + host + ":" + port + "/" + db + "?sslmode=disable"
}

func getEnv(key string, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}

func splitCSV(value string) []string {
	parts := strings.Split(value, ",")
	items := make([]string, 0, len(parts))

	for _, part := range parts {
		item := strings.TrimSpace(part)
		if item != "" {
			items = append(items, item)
		}
	}

	return items
}
func getDurationEnv(key string, fallback time.Duration) time.Duration {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	duration, err := parseDuration(value)
	if err != nil || duration <= 0 {
		return fallback
	}

	return duration
}

func parseDuration(value string) (time.Duration, error) {
	if strings.HasSuffix(value, "d") {
		days, err := strconv.Atoi(strings.TrimSuffix(value, "d"))
		if err != nil || days <= 0 {
			return 0, strconv.ErrSyntax
		}

		return time.Duration(days) * 24 * time.Hour, nil
	}

	return time.ParseDuration(value)
}

func durationSeconds(key string, fallback time.Duration) time.Duration {
	value, err := strconv.Atoi(getEnv(key, ""))
	if err != nil || value < 0 {
		return fallback
	}
	return time.Duration(value) * time.Second
}

func durationEnv(key string, fallback time.Duration) time.Duration {
	value := getEnv(key, "")
	if value == "" {
		return fallback
	}
	duration, err := time.ParseDuration(value)
	if err == nil && duration >= 0 {
		return duration
	}
	seconds, err := strconv.Atoi(value)
	if err != nil || seconds < 0 {
		return fallback
	}
	return time.Duration(seconds) * time.Second
}

func intEnv(key string, fallback int) int {
	value, err := strconv.Atoi(getEnv(key, ""))
	if err != nil || value < 0 {
		return fallback
	}
	return value
}
