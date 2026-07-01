package config

import (
	"testing"
	"time"
)

func TestGetDurationEnvSupportsDays(t *testing.T) {
	t.Setenv("TEST_DURATION", "7d")

	got := getDurationEnv("TEST_DURATION", time.Hour)
	if got != 7*24*time.Hour {
		t.Fatalf("expected 168h, got %s", got)
	}
}

func TestValidateRejectsDefaultJWTSecretInProduction(t *testing.T) {
	cfg := Config{AppEnv: "production", JWTSecret: defaultJWTSecret}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected production JWT secret validation error")
	}
}

func TestValidateAllowsConfiguredJWTSecret(t *testing.T) {
	cfg := Config{AppEnv: "production", JWTSecret: "a-long-random-production-secret"}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}
}
