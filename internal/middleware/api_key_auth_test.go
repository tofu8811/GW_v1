package middleware

import "testing"

func TestScopeAllowsRoute(t *testing.T) {
	tests := []struct {
		name   string
		scopes []string
		want   bool
	}{
		{name: "wildcard", scopes: []string{"*"}, want: true},
		{name: "route id", scopes: []string{"route:80000000-0000-0000-0000-000000000105"}, want: true},
		{name: "method and path", scopes: []string{"GET:/api/orders"}, want: true},
		{name: "wrong method", scopes: []string{"POST:/api/orders"}, want: false},
		{name: "unrelated scope", scopes: []string{"services:read"}, want: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := ScopeAllowsRoute(test.scopes, "80000000-0000-0000-0000-000000000105", "GET", "/api/orders")
			if got != test.want {
				t.Fatalf("expected %v, got %v", test.want, got)
			}
		})
	}
}
