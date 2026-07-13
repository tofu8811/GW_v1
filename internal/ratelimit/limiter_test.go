package ratelimit

import "testing"

func TestKeyWithScopeUsesAggregationNamespaceAndSubject(t *testing.T) {
	policy := Policy{ID: "policy-1", LimitType: LimitTypeIP}
	got := KeyWithScope(policy, "127.0.0.1", "aggregation", "agg-1", 100)
	want := "rl:aggregation:ip:policy-1:agg-1:127.0.0.1:100"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}
