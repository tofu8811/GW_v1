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
