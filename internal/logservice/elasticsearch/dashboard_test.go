package elasticsearch

import (
	"testing"

	"gateway-api/internal/logservice/model"
)

func TestBuildDashboardAggregationsIncludesRealtimeAggs(t *testing.T) {
	aggs := buildDashboardAggregations(model.RealtimeQuery{Interval: "1s", TopLimit: 10})
	for _, key := range []string{
		"rps",
		"errors",
		"latency_avg",
		"latency_percentiles",
		"status_class",
		"status_code",
		"top_routes",
	} {
		if _, ok := aggs[key]; !ok {
			t.Fatalf("expected aggregation %q in %#v", key, aggs)
		}
	}
}

func TestErrorRateBucketsAvoidsDivideByZero(t *testing.T) {
	raw := map[string]any{
		"buckets": []any{
			map[string]any{
				"key_as_string": "2026-07-07T10:00:00Z",
				"doc_count":     float64(0),
				"errors":        map[string]any{"doc_count": float64(0)},
			},
		},
	}
	got := errorRateBuckets(raw)
	if len(got) != 1 {
		t.Fatalf("expected one bucket, got %#v", got)
	}
	if got[0].ErrorRate != 0 {
		t.Fatalf("expected zero error rate, got %#v", got[0])
	}
}

func TestBuildRealtimeBoolQueryUsesWindowAndFilters(t *testing.T) {
	query := buildRealtimeBoolQuery(model.RealtimeQuery{
		Window:      "60s",
		ServiceName: "product-service",
		Method:      "get",
		StatusCode:  "500",
	})
	boolQuery, ok := query["bool"].(map[string]any)
	if !ok {
		t.Fatalf("expected bool query, got %#v", query)
	}
	filters, ok := boolQuery["filter"].([]map[string]any)
	if !ok || len(filters) != 4 {
		t.Fatalf("expected range plus 3 filters, got %#v", boolQuery["filter"])
	}
	mustNot, ok := boolQuery["must_not"].([]map[string]any)
	if !ok || len(mustNot) != 4 {
		t.Fatalf("expected realtime control-plane exclusions, got %#v", boolQuery["must_not"])
	}
}
