package config

import (
	"os"
	"path/filepath"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	AppEnv      string
	AppPort     string
	DatabaseURL string
	RedisAddr   string
	RedisPass   string
	RedisDB     int
}

func Load() Config {
	loadDotEnv()

	redisDB, _ := strconv.Atoi(getEnv("REDIS_DB", "0"))
	databaseURL := getEnv("DATABASE_URL", "")
	if databaseURL == "" {
		databaseURL = buildDatabaseURL()
	}

	return Config{
		AppEnv:      getEnv("APP_ENV", "development"),
		AppPort:     getEnv("APP_PORT", "8080"),
		DatabaseURL: databaseURL,
		RedisAddr:   getEnv("REDIS_ADDR", "localhost:6379"),
		RedisPass:   getEnv("REDIS_PASSWORD", ""),
		RedisDB:     redisDB,
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
