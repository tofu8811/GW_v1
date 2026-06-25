package gateway

import "testing"

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
