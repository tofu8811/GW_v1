package api

import (
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func TestParseRealtimeQueryDefaults(t *testing.T) {
	app := fiber.New()
	var parsedWindow string
	var parsedInterval string
	var parsedTopLimit int
	var parseErr error
	app.Get("/", func(c *fiber.Ctx) error {
		query, err := parseRealtimeQuery(c)
		parseErr = err
		parsedWindow = query.Window
		parsedInterval = query.Interval
		parsedTopLimit = query.TopLimit
		return nil
	})
	if _, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil)); err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if parseErr != nil {
		t.Fatalf("unexpected parse error: %v", parseErr)
	}
	if parsedWindow != "60s" || parsedInterval != "1s" || parsedTopLimit != 10 {
		t.Fatalf("unexpected defaults: window=%s interval=%s top_limit=%d", parsedWindow, parsedInterval, parsedTopLimit)
	}
}

func TestParseRealtimeQueryClampsValuesAndNormalizesMethod(t *testing.T) {
	app := fiber.New()
	var got string
	var parseErr error
	app.Get("/", func(c *fiber.Ctx) error {
		query, err := parseRealtimeQuery(c)
		parseErr = err
		got = query.Window + "|" + query.Interval + "|" + strconv.Itoa(query.TopLimit) + "|" + query.Method
		return nil
	})
	req := httptest.NewRequest(fiber.MethodGet, "/?window=2h&interval=100ms&top_limit=500&method=post", nil)
	if _, err := app.Test(req); err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if parseErr != nil {
		t.Fatalf("unexpected parse error: %v", parseErr)
	}
	if got != "15m|1s|50|POST" {
		t.Fatalf("unexpected clamped query: %s", got)
	}
}

func TestParseRealtimeQueryRejectsInvalidStatusCode(t *testing.T) {
	app := fiber.New()
	var parseErr error
	app.Get("/", func(c *fiber.Ctx) error {
		_, parseErr = parseRealtimeQuery(c)
		return nil
	})
	if _, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/?status_code=abc", nil)); err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if parseErr == nil {
		t.Fatal("expected invalid status code error")
	}
}
