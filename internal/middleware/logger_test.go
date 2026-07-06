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
		SetRouteLogContext(c, "route-1", "service-1", "test-service", "/api/test")
		SetInstanceLogContext(c, "instance-1")
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
	if entry.ServiceID != "service-1" || entry.InstanceID != "instance-1" || entry.NormalizedPath != "/api/test" {
		t.Fatalf("unexpected service context: %+v", entry)
	}
	if entry.UserID != "user-1" || entry.APIKeyID != "key-1" {
		t.Fatalf("unexpected auth context: %+v", entry)
	}
	if entry.StatusCode != fiber.StatusBadGateway || entry.UpstreamLatencyMS < 25 {
		t.Fatalf("unexpected response metrics: %+v", entry)
	}
	if entry.StatusClass != "5xx" || !entry.IsError || entry.ErrorType != "server_error" {
		t.Fatalf("unexpected status classification: %+v", entry)
	}
	if entry.GatewayLatencyMS < 0 {
		t.Fatalf("expected non-negative gateway latency: %+v", entry)
	}
	if entry.ErrorMessage != "upstream failed" {
		t.Fatalf("unexpected error message: %q", entry.ErrorMessage)
	}
}

func TestLoggerUsesFiberRequestIDAsTraceID(t *testing.T) {
	var output bytes.Buffer
	app := fiber.New()
	app.Use(requestid.New())
	app.Use(Logger(&output, nil))
	app.Get("/api/test", func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusNoContent)
	})

	request := httptest.NewRequest(fiber.MethodGet, "/api/test", nil)
	request.Header.Set(fiber.HeaderXRequestID, "trace-from-requestid")
	if _, err := app.Test(request); err != nil {
		t.Fatalf("request failed: %v", err)
	}

	var entry requestLogEntry
	if err := json.Unmarshal(bytes.TrimSpace(output.Bytes()), &entry); err != nil {
		t.Fatalf("invalid JSON log: %v", err)
	}
	if entry.TraceID != "trace-from-requestid" {
		t.Fatalf("expected Fiber request id trace, got %q", entry.TraceID)
	}
}
