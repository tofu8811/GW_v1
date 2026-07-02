package middleware

import (
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func TestJWTAuthStopsHandlerWhenAuthorizationIsMissing(t *testing.T) {
	app := fiber.New()
	downstreamCalled := false
	app.Get("/protected", JWTAuth("test-secret", nil), func(c *fiber.Ctx) error {
		downstreamCalled = true
		return c.SendStatus(fiber.StatusOK)
	})

	response, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/protected", nil))
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != fiber.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", response.StatusCode)
	}
	if downstreamCalled {
		t.Fatal("protected handler must not run after authentication failure")
	}
}
