package proxy

import (
	"testing"

	configcache "gateway-api/internal/config/cache"
	"gateway-api/internal/proxy/loadbalancer"
)

func TestUpstreamRoutesPreservesAuthRequired(t *testing.T) {
	routes := upstreamRoutesFromCache(configcache.RouteValue{
		RouteID:      "route-1",
		AuthRequired: true,
		Service:      configcache.ServiceValue{ID: "service-1"},
		Instances:    []configcache.InstanceValue{{ID: "instance-1"}},
	})
	if len(routes) != 1 || !routes[0].AuthRequired {
		t.Fatalf("expected auth_required to be preserved: %#v", routes)
	}
}

func TestMatchPathSupportsCatchAllWildcard(t *testing.T) {
	params, ok := matchPath("/api/*", "/api/users/42")
	if !ok {
		t.Fatal("expected wildcard route to match")
	}

	if params["*"] != "users/42" {
		t.Fatalf("expected wildcard tail users/42, got %q", params["*"])
	}
}

func TestMatchPathSupportsNamedCatchAll(t *testing.T) {
	params, ok := matchPath("/api/{tail...}", "/api/users/42")
	if !ok {
		t.Fatal("expected named catch-all route to match")
	}

	if params["tail"] != "users/42" {
		t.Fatalf("expected named tail users/42, got %q", params["tail"])
	}
	if params["*"] != "users/42" {
		t.Fatalf("expected wildcard tail users/42, got %q", params["*"])
	}
}

func TestMatchPathRejectsExtraSegmentsWithoutCatchAll(t *testing.T) {
	if _, ok := matchPath("/api/users", "/api/users/42"); ok {
		t.Fatal("expected exact route to reject extra path segment")
	}
}

func TestRewritePathStripPrefixUsesCatchAllTail(t *testing.T) {
	route := &UpstreamRoute{StripPrefix: true}
	got := rewritePath(route, "/api/users/42", map[string]string{"*": "users/42"})
	if got != "/users/42" {
		t.Fatalf("expected /users/42, got %q", got)
	}
}

func TestRewritePathRewriteTargetTakesPriority(t *testing.T) {
	target := "/v2/{id}"
	route := &UpstreamRoute{StripPrefix: true, RewriteTarget: &target}
	got := rewritePath(route, "/api/users/42", map[string]string{"id": "42", "*": "users/42"})
	if got != "/v2/42" {
		t.Fatalf("expected /v2/42, got %q", got)
	}
}

func TestPickInstanceUsesConfiguredLBStrategy(t *testing.T) {
	handler := &Handler{
		roundRobin: loadbalancer.NewRoundRobin(),
		weighted:   loadbalancer.NewWeightedRoundRobin(),
	}
	instances := []loadbalancer.Instance{
		{ID: "a", ServiceID: "service-1", Weight: 2},
		{ID: "b", ServiceID: "service-1", Weight: 1},
	}

	roundRobinRoute := UpstreamRoute{ServiceID: "service-1", LBStrategy: "round_robin"}
	first, err := handler.pickInstance(roundRobinRoute, instances)
	if err != nil {
		t.Fatalf("unexpected round robin error: %v", err)
	}
	second, err := handler.pickInstance(roundRobinRoute, instances)
	if err != nil {
		t.Fatalf("unexpected round robin error: %v", err)
	}
	if first.ID != "a" || second.ID != "b" {
		t.Fatalf("expected round robin sequence a, b, got %s, %s", first.ID, second.ID)
	}

	weightedRoute := UpstreamRoute{ServiceID: "service-1", LBStrategy: "weighted"}
	weightedFirst, err := handler.pickInstance(weightedRoute, instances)
	if err != nil {
		t.Fatalf("unexpected weighted error: %v", err)
	}
	weightedSecond, err := handler.pickInstance(weightedRoute, instances)
	if err != nil {
		t.Fatalf("unexpected weighted error: %v", err)
	}
	if weightedFirst.ID != "a" || weightedSecond.ID != "a" {
		t.Fatalf("expected weighted sequence to start a, a, got %s, %s", weightedFirst.ID, weightedSecond.ID)
	}
}
