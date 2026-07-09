package cache

import "testing"

func TestFindAggregationReturnsClone(t *testing.T) {
	store := &Store{aggregationsByKey: map[string]AggregationValue{
		aggregationKey("GET", "/api/dashboard"): {
			ID: "agg-id", Name: "dashboard", Path: "/api/dashboard", Method: "GET",
			Steps: []AggregationStepValue{{ID: "step-1", RequestTemplate: []byte(`{"method":"GET","path":"/products"}`), ResponseMapping: []byte(`{"target":"products"}`)}},
		},
	}}

	agg, ok := store.FindAggregation("GET", "/api/dashboard")
	if !ok {
		t.Fatal("expected aggregation to be found")
	}
	agg.Steps[0].RequestTemplate[0] = '['

	again, ok := store.FindAggregation("GET", "/api/dashboard")
	if !ok {
		t.Fatal("expected aggregation to be found")
	}

	if string(again.Steps[0].RequestTemplate) != `{"method":"GET","path":"/products"}` {
		t.Fatalf("expected cloned request template, got %s", string(again.Steps[0].RequestTemplate))
	}
}

func TestFindAggregationMiss(t *testing.T) {
	store := &Store{aggregationsByKey: map[string]AggregationValue{}}
	if _, ok := store.FindAggregation("GET", "/missing"); ok {
		t.Fatal("expected cache miss")
	}
}
