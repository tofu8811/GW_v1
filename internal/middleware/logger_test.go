package middleware

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/requestid"
)

func TestLoggerWritesElasticCompatibleJSON(t *testing.T) {
	var output bytes.Buffer
	app := fiber.New()
	app.Use(requestid.New())
	app.Use(Logger(&output, nil))
	app.Get("/api/test", func(c *fiber.Ctx) error {
		SetRouteLogContext(c, "route-1", "test-service")
		SetAPIKeyLogContext(c, "key-1")
		c.Locals(LocalsUserID, "user-1")
		AddUpstreamLatency(c, 25*time.Millisecond)
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{
			"error": fiber.Map{"message": "upstream failed"},
		})
	})

	response, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/api/test", nil))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if response.StatusCode != fiber.StatusBadGateway {
		t.Fatalf("expected status %d, got %d", fiber.StatusBadGateway, response.StatusCode)
	}

	var entry requestLogEntry
	if err := json.Unmarshal(bytes.TrimSpace(output.Bytes()), &entry); err != nil {
		t.Fatalf("invalid JSON log: %v", err)
	}
	if entry.TraceID == "" {
		t.Fatal("expected trace_id")
	}
	if entry.RouteID != "route-1" || entry.ServiceName != "test-service" {
		t.Fatalf("unexpected route context: %+v", entry)
	}
	if entry.UserID != "user-1" || entry.APIKeyID != "key-1" {
		t.Fatalf("unexpected auth context: %+v", entry)
	}
	if entry.StatusCode != fiber.StatusBadGateway || entry.UpstreamLatencyMS < 25 {
		t.Fatalf("unexpected response metrics: %+v", entry)
	}
	if entry.ErrorMessage != "upstream failed" {
		t.Fatalf("unexpected error message: %q", entry.ErrorMessage)
	}
}
