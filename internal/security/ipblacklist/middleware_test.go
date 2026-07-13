package ipblacklist

import (
	"net/http/httptest"
	"net/netip"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func TestMiddlewareBlocksMatchedIP(t *testing.T) {
	checker := &Checker{
		exact: map[string]entry{},
		cidrs: []cidrEntry{
			{Prefix: netip.MustParsePrefix("0.0.0.0/0"), Rule: "0.0.0.0/0", Reason: "test block"},
		},
	}
	app := fiber.New()
	app.Use(Middleware(checker, nil))
	app.Get("/api/products", func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	req := httptest.NewRequest(fiber.MethodGet, "/api/products", nil)

	res, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test() error = %v", err)
	}
	if res.StatusCode != fiber.StatusForbidden {
		t.Fatalf("status = %d, want %d", res.StatusCode, fiber.StatusForbidden)
	}
}

func TestMiddlewareSkipsAdminRoutes(t *testing.T) {
	checker := &Checker{
		exact: map[string]entry{},
		cidrs: []cidrEntry{
			{Prefix: netip.MustParsePrefix("0.0.0.0/0"), Rule: "0.0.0.0/0", Reason: "test block"},
		},
	}
	app := fiber.New()
	app.Use(Middleware(checker, nil))
	app.Get("/admin/ip-blacklist", func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	req := httptest.NewRequest(fiber.MethodGet, "/admin/ip-blacklist", nil)

	res, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test() error = %v", err)
	}
	if res.StatusCode != fiber.StatusOK {
		t.Fatalf("status = %d, want %d", res.StatusCode, fiber.StatusOK)
	}
}
