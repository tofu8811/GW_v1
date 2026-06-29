package config

import (
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	AppEnv      string
	AppPort     string
	DatabaseURL string
	RedisAddr   string
	RedisPass   string
	RedisDB     int

	ConfigPollInterval   time.Duration
	ConfigTTL            time.Duration
	ConfigRebuildLockTTL time.Duration
	ConfigLockWait       time.Duration
	ConfigSchemaVersion  int

	HealthCheckInterval      time.Duration
	HealthProbeTimeout       time.Duration
	HealthUnhealthyThreshold int
	HealthHealthyThreshold   int
	HealthKeyTTL             time.Duration

	BreakerFailureThreshold int
	BreakerOpenTimeout      time.Duration
	BreakerHalfOpenMax      int
}

func Load() Config {
	loadDotEnv()

	redisDB, _ := strconv.Atoi(getEnv("REDIS_DB", "0"))
	schemaVersion, _ := strconv.Atoi(getEnv("CONFIG_SCHEMA_VERSION", "1"))
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
		DatabaseURL: databaseURL,
		RedisAddr:   getEnv("REDIS_ADDR", "localhost:6379"),
		RedisPass:   getEnv("REDIS_PASSWORD", ""),
		RedisDB:     redisDB,

		ConfigPollInterval:   durationSeconds("CONFIG_POLL_INTERVAL_SECONDS", 20*time.Second),
		ConfigTTL:            durationSeconds("CONFIG_TTL_SECONDS", 0),
		ConfigRebuildLockTTL: durationSeconds("CONFIG_REBUILD_LOCK_TTL_SECONDS", 10*time.Second),
		ConfigLockWait:       durationSeconds("CONFIG_REBUILD_LOCK_WAIT_SECONDS", 2*time.Second),
		ConfigSchemaVersion:  schemaVersion,

		HealthCheckInterval:      healthInterval,
		HealthProbeTimeout:       durationEnv("HEALTH_PROBE_TIMEOUT", 2*time.Second),
		HealthUnhealthyThreshold: intEnv("HEALTH_UNHEALTHY_THRESHOLD", 3),
		HealthHealthyThreshold:   intEnv("HEALTH_HEALTHY_THRESHOLD", 2),
		HealthKeyTTL:             healthTTL,

		BreakerFailureThreshold: intEnv("BREAKER_FAILURE_THRESHOLD", 5),
		BreakerOpenTimeout:      durationEnv("BREAKER_OPEN_TIMEOUT", 15*time.Second),
		BreakerHalfOpenMax:      intEnv("BREAKER_HALFOPEN_MAX", 1),
	}
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
