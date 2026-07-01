package apikeys

import "testing"

func TestNormalizeScopes(t *testing.T) {
	scopes, err := normalizeScopes([]string{" GET:/api/orders ", "GET:/api/orders", "*"})
	if err != nil {
		t.Fatal(err)
	}
	if len(scopes) != 2 || scopes[0] != "GET:/api/orders" || scopes[1] != "*" {
		t.Fatalf("unexpected scopes: %#v", scopes)
	}
}

func TestNormalizeScopesRequiresValue(t *testing.T) {
	if _, err := normalizeScopes([]string{" "}); err == nil {
		t.Fatal("expected an error")
	}
}
