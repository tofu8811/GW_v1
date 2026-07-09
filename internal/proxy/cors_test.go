package proxy

import (
	"net/http/httptest"
	"testing"

	configcache "gateway-api/internal/config/cache"

	"github.com/gofiber/fiber/v2"
)

func TestValidateCORSRequest(t *testing.T) {
	config := &configcache.CORSValue{
		AllowedOrigins: []string{"http://localhost:5173"},
		AllowedMethods: []string{"GET", "POST"},
	}
	if err := validateCORSRequest(config, "http://localhost:5173", "GET"); err != nil {
		t.Fatalf("expected allowed request, got %v", err)
	}
	if err := validateCORSRequest(config, "http://evil.example", "GET"); err == nil {
		t.Fatal("expected origin rejection")
	}
	if err := validateCORSRequest(config, "http://localhost:5173", "DELETE"); err == nil {
		t.Fatal("expected method rejection")
	}
}

func TestHeadersAllowedIsCaseInsensitive(t *testing.T) {
	config := &configcache.CORSValue{AllowedHeaders: []string{"Content-Type", "Authorization"}}
	if !headersAllowed(config, "content-type, AUTHORIZATION") {
		t.Fatal("expected headers to be allowed")
	}
	if headersAllowed(config, "X-Not-Allowed") {
		t.Fatal("expected header rejection")
	}
}

func TestActualCORSHeadersAreAppliedAfterResponse(t *testing.T) {
	config := &configcache.CORSValue{
		AllowedOrigins:   []string{"http://localhost:5173"},
		AllowCredentials: true,
	}
	app := fiber.New()
	app.Get("/", func(c *fiber.Ctx) error {
		defer setActualCORSHeaders(c, config, "http://localhost:5173")
		return c.SendString("ok")
	})

	request := httptest.NewRequest("GET", "/", nil)
	request.Host = "gateway.test"
	response, err := app.Test(request)
	if err != nil {
		t.Fatalf("unexpected app error: %v", err)
	}
	if got := response.Header.Get(fiber.HeaderAccessControlAllowOrigin); got != "http://localhost:5173" {
		t.Fatalf("unexpected allow origin: %q", got)
	}
	if got := response.Header.Get(fiber.HeaderAccessControlAllowCredentials); got != "true" {
		t.Fatalf("unexpected allow credentials: %q", got)
	}
}

func TestSpecificOriginSetsVaryAndExposedHeaders(t *testing.T) {
	config := &configcache.CORSValue{
		AllowedOrigins: []string{"http://localhost:5173"},
		ExposedHeaders: []string{"X-Request-ID"},
	}
	app := fiber.New()
	app.Get("/", func(c *fiber.Ctx) error {
		defer setActualCORSHeaders(c, config, "http://localhost:5173")
		return c.SendString("ok")
	})

	response, err := app.Test(httptest.NewRequest("GET", "/", nil))
	if err != nil {
		t.Fatalf("unexpected app error: %v", err)
	}
	if got := response.Header.Get(fiber.HeaderVary); got != fiber.HeaderOrigin {
		t.Fatalf("unexpected vary header: %q", got)
	}
	if got := response.Header.Get(fiber.HeaderAccessControlExposeHeaders); got != "X-Request-ID" {
		t.Fatalf("unexpected exposed headers: %q", got)
	}
}

func TestWildcardOriginDoesNotSetVary(t *testing.T) {
	config := &configcache.CORSValue{AllowedOrigins: []string{"*"}}
	app := fiber.New()
	app.Get("/", func(c *fiber.Ctx) error {
		defer setActualCORSHeaders(c, config, "http://localhost:5173")
		return c.SendString("ok")
	})

	response, err := app.Test(httptest.NewRequest("GET", "/", nil))
	if err != nil {
		t.Fatalf("unexpected app error: %v", err)
	}
	if got := response.Header.Get(fiber.HeaderAccessControlAllowOrigin); got != "*" {
		t.Fatalf("unexpected allow origin: %q", got)
	}
	if got := response.Header.Get(fiber.HeaderVary); got != "" {
		t.Fatalf("expected no vary header for wildcard, got %q", got)
	}
}
