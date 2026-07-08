package middleware

import "testing"

func TestHasScope(t *testing.T) {
	required := "01972f6a-0002-7000-8000-000000000001"
	tests := []struct {
		name     string
		scopeIDs []string
		want     bool
	}{
		{name: "required scope", scopeIDs: []string{required}, want: true},
		{name: "among many", scopeIDs: []string{"01972f6a-0002-7000-8000-000000000002", required}, want: true},
		{name: "missing", scopeIDs: []string{"01972f6a-0002-7000-8000-000000000002"}, want: false},
		{name: "wildcard is not supported", scopeIDs: []string{"*"}, want: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := hasScope(test.scopeIDs, required)
			if got != test.want {
				t.Fatalf("expected %v, got %v", test.want, got)
			}
		})
	}
}
