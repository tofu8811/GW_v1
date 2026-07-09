package proxy

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"unsafe"

	configcache "gateway-api/internal/config/cache"
	"gateway-api/internal/proxy/loadbalancer"

	"github.com/gofiber/fiber/v2"
)

func TestHandleAggregationMergesStepResponses(t *testing.T) {
	products := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[{"id":1}]`))
	}))
	defer products.Close()
	users := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[{"id":2}]`))
	}))
	defer users.Close()

	agg := testAggregation(t, products, users)
	status, body := runAggregation(t, agg, testServices(), testInstances(t, products, users))
	if status != fiber.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", status, body)
	}
	var payload struct {
		Success bool              `json:"success"`
		Data    map[string][]any  `json:"data"`
		Meta    map[string]string `json:"meta"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatal(err)
	}
	if !payload.Success || len(payload.Data["products"]) != 1 || len(payload.Data["users"]) != 1 || payload.Meta["aggregation"] != "demo-dashboard" {
		t.Fatalf("unexpected payload: %s", body)
	}
}

func TestHandleAggregationReturnsPartialForOptionalStepFailure(t *testing.T) {
	products := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[{"id":1}]`))
	}))
	defer products.Close()
	users := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "down", http.StatusBadGateway)
	}))
	defer users.Close()

	agg := testAggregation(t, products, users)
	status, body := runAggregation(t, agg, testServices(), testInstances(t, products, users))
	if status != fiber.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", status, body)
	}
	var payload struct {
		Data map[string]any `json:"data"`
		Meta struct {
			Errors []aggregationError `json:"errors"`
		} `json:"meta"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatal(err)
	}
	if _, ok := payload.Data["products"]; !ok || payload.Data["users"] != nil || len(payload.Meta.Errors) != 1 {
		t.Fatalf("expected partial response with one error: %s", body)
	}
}

func TestHandleAggregationFailsForRequiredStepFailure(t *testing.T) {
	products := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "down", http.StatusBadGateway)
	}))
	defer products.Close()
	users := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[]`))
	}))
	defer users.Close()

	agg := testAggregation(t, products, users)
	status, _ := runAggregation(t, agg, testServices(), testInstances(t, products, users))
	if status != fiber.StatusBadGateway {
		t.Fatalf("expected 502, got %d", status)
	}
}

func TestHandleAggregationOptionalInvalidJSONReturnsErrorMeta(t *testing.T) {
	products := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[]`))
	}))
	defer products.Close()
	users := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`not-json`))
	}))
	defer users.Close()

	agg := testAggregation(t, products, users)
	status, body := runAggregation(t, agg, testServices(), testInstances(t, products, users))
	if status != fiber.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", status, body)
	}
	var payload struct {
		Meta struct {
			Errors []aggregationError `json:"errors"`
		} `json:"meta"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Meta.Errors) != 1 {
		t.Fatalf("expected optional error meta, got %s", body)
	}
}

func runAggregation(t *testing.T, agg configcache.AggregationValue, services map[string]configcache.ServiceValue, instances map[string][]configcache.InstanceValue) (int, []byte) {
	t.Helper()
	app := fiber.New()
	handler := &Handler{
		configCache: testConfigStore(t, services, instances),
		logger:      slog.New(slog.NewTextHandler(io.Discard, nil)),
		roundRobin:  loadbalancer.NewRoundRobin(),
		weighted:    loadbalancer.NewWeightedRoundRobin(),
	}
	app.Get("/api/dashboard", func(c *fiber.Ctx) error {
		return handler.handleAggregation(c, &agg)
	})
	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/api/dashboard", nil), 2_000)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, body
}

func testAggregation(t *testing.T, products *httptest.Server, users *httptest.Server) configcache.AggregationValue {
	t.Helper()
	return configcache.AggregationValue{
		ID: "agg-1", Name: "demo-dashboard", Path: "/api/dashboard", Method: "GET",
		Steps: []configcache.AggregationStepValue{
			{ID: "products-step", Sequence: 1, IsRequired: true, RequestTemplate: []byte(`{"method":"GET","path":"/products"}`), ResponseMapping: []byte(`{"target":"products"}`), ServiceID: "product-service"},
			{ID: "users-step", Sequence: 2, IsRequired: false, RequestTemplate: []byte(`{"method":"GET","path":"/users"}`), ResponseMapping: []byte(`{"target":"users"}`), ServiceID: "user-service"},
		},
	}
}

func testServices() map[string]configcache.ServiceValue {
	return map[string]configcache.ServiceValue{
		"product-service": {ID: "product-service", Name: "product-service", Protocol: "http", TimeoutMS: 500},
		"user-service":    {ID: "user-service", Name: "user-service", Protocol: "http", TimeoutMS: 500},
	}
}

func testInstances(t *testing.T, products *httptest.Server, users *httptest.Server) map[string][]configcache.InstanceValue {
	t.Helper()
	productHost, productPort := serverHostPort(t, products)
	userHost, userPort := serverHostPort(t, users)
	return map[string][]configcache.InstanceValue{
		"product-service": {{ID: "p1", ServiceID: "product-service", Host: productHost, Port: productPort}},
		"user-service":    {{ID: "u1", ServiceID: "user-service", Host: userHost, Port: userPort}},
	}
}

func testConfigStore(t *testing.T, services map[string]configcache.ServiceValue, instances map[string][]configcache.InstanceValue) *configcache.Store {
	t.Helper()
	store := configcache.NewStore(nil, nil, slog.New(slog.NewTextHandler(io.Discard, nil)), configcache.DefaultConfig())
	setPrivateField(t, store, "servicesByID", services)
	setPrivateField(t, store, "instancesByServiceID", instances)
	return store
}

func setPrivateField(t *testing.T, store *configcache.Store, name string, value any) {
	t.Helper()
	field := reflect.ValueOf(store).Elem().FieldByName(name)
	reflect.NewAt(field.Type(), unsafe.Pointer(field.UnsafeAddr())).Elem().Set(reflect.ValueOf(value))
}
