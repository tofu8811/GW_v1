package validation

import "testing"

func TestValidateRouteRewriteTargetAllowsRouteParams(t *testing.T) {
	rewriteTarget := "/v2/products/{id}"
	if err := ValidateRouteRewriteTarget("/api/product/{id}", &rewriteTarget); err != nil {
		t.Fatalf("expected rewrite target to be valid, got %v", err)
	}
}

func TestValidateRouteRewriteTargetRejectsUnknownParams(t *testing.T) {
	rewriteTarget := "/v2/products/{id}"
	if err := ValidateRouteRewriteTarget("/api/products", &rewriteTarget); err == nil {
		t.Fatal("expected rewrite target with unknown param to fail")
	}
}

func TestValidateRouteRewriteTargetAllowsCatchAllParams(t *testing.T) {
	rewriteTarget := "/{*}"
	if err := ValidateRouteRewriteTarget("/api/*", &rewriteTarget); err != nil {
		t.Fatalf("expected wildcard rewrite target to be valid, got %v", err)
	}
}

func TestValidateStripPrefixRequiresCatchAll(t *testing.T) {
	if err := ValidateStripPrefix("/api/products", true); err == nil {
		t.Fatal("expected strip_prefix without catch-all to fail")
	}
}

func TestValidateStripPrefixAllowsCatchAll(t *testing.T) {
	if err := ValidateStripPrefix("/api/{tail...}", true); err != nil {
		t.Fatalf("expected strip_prefix with catch-all to be valid, got %v", err)
	}
}
