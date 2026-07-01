package proxy

import (
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"gateway-api/internal/proxy/loadbalancer"

	"github.com/gofiber/fiber/v2"
)

func TestForwardWithRetryFailoversToNextInstance(t *testing.T) {
	good := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
	defer good.Close()

	host, port := serverHostPort(t, good)
	badPort := closedPort(t)
	app := fiber.New()
	handler := testForwardHandler()
	route := &UpstreamRoute{
		RouteID:          "route-1",
		RoutePath:        "/api",
		RouteMethod:      "GET",
		ServiceID:        "svc",
		ServiceName:      "svc",
		Protocol:         "http",
		LBStrategy:       "round_robin",
		TimeoutMS:        200,
		RetryCount:       1,
		StripPrefix:      true,
		MatchedInstances: 2,
		AvailableInstances: []loadbalancer.Instance{
			{ID: "a", ServiceID: "svc", Host: "127.0.0.1", Port: badPort, Weight: 1},
			{ID: "b", ServiceID: "svc", Host: host, Port: port, Weight: 1},
		},
	}
	app.Get("/api/test", func(c *fiber.Ctx) error {
		prepareForwardedRequest(c)
		return handler.forwardWithRetry(c, route, c.Path(), map[string]string{"*": "test"}, time.Now())
	})

	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/api/test", nil), 2_000)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK || string(body) != "ok" {
		t.Fatalf("expected failover success, status=%d body=%s", resp.StatusCode, string(body))
	}
}

func TestForwardWithRetryReturnsUnavailableWhenAllInstancesFail(t *testing.T) {
	app := fiber.New()
	handler := testForwardHandler()
	route := &UpstreamRoute{
		RouteID:          "route-1",
		RoutePath:        "/api",
		RouteMethod:      "GET",
		ServiceID:        "svc",
		ServiceName:      "svc",
		Protocol:         "http",
		LBStrategy:       "round_robin",
		TimeoutMS:        50,
		RetryCount:       1,
		StripPrefix:      true,
		MatchedInstances: 1,
		AvailableInstances: []loadbalancer.Instance{
			{ID: "a", ServiceID: "svc", Host: "127.0.0.1", Port: closedPort(t), Weight: 1},
		},
	}
	app.Get("/api/test", func(c *fiber.Ctx) error {
		prepareForwardedRequest(c)
		return handler.forwardWithRetry(c, route, c.Path(), map[string]string{"*": "test"}, time.Now())
	})

	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/api/test", nil), 2_000)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", resp.StatusCode)
	}
}

func testForwardHandler() *Handler {
	return &Handler{
		logger:     slog.New(slog.NewTextHandler(io.Discard, nil)),
		roundRobin: loadbalancer.NewRoundRobin(),
		weighted:   loadbalancer.NewWeightedRoundRobin(),
	}
}

func serverHostPort(t *testing.T, server *httptest.Server) (string, int) {
	t.Helper()
	host, portRaw, err := net.SplitHostPort(server.Listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	port, err := strconv.Atoi(portRaw)
	if err != nil {
		t.Fatal(err)
	}
	return host, port
}

func closedPort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	_ = listener.Close()
	return port
}
