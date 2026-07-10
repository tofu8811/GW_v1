package elasticsearch

import (
	"testing"

	"gateway-api/internal/logservice/model"
)

func TestBuildBoolQueryIncludesFiltersAndSearch(t *testing.T) {
	query := buildBoolQuery(model.LogQuery{
		From:        "2026-07-06T00:00:00Z",
		To:          "2026-07-06T01:00:00Z",
		ServiceName: "product-service",
		Method:      "get",
		StatusClass: "5xx",
		Query:       "timeout",
	})

	boolQuery, ok := query["bool"].(map[string]any)
	if !ok {
		t.Fatalf("expected bool query, got %#v", query)
	}
	filters, ok := boolQuery["filter"].([]map[string]any)
	if !ok || len(filters) != 4 {
		t.Fatalf("expected 4 filters, got %#v", boolQuery["filter"])
	}
	must, ok := boolQuery["must"].([]map[string]any)
	if !ok || len(must) != 1 {
		t.Fatalf("expected multi_match must query, got %#v", boolQuery["must"])
	}
}

func TestBuildBoolQueryUsesExactLogFilters(t *testing.T) {
	query := buildBoolQuery(model.LogQuery{
		TraceID:        "trace-1",
		Path:           "/api/dashboard",
		NormalizedPath: "/api/dashboard",
		APIKeyID:       "key-1",
		ErrorMessage:   "timeout",
	})

	boolQuery, ok := query["bool"].(map[string]any)
	if !ok {
		t.Fatalf("expected bool query, got %#v", query)
	}
	filters, ok := boolQuery["filter"].([]map[string]any)
	if !ok || len(filters) != 4 {
		t.Fatalf("expected 4 exact filters, got %#v", boolQuery["filter"])
	}
	must, ok := boolQuery["must"].([]map[string]any)
	if !ok || len(must) != 1 {
		t.Fatalf("expected one fuzzy error_message must query, got %#v", boolQuery["must"])
	}
}

func TestBuildSortDefaultsToTimestampDesc(t *testing.T) {
	sort := buildSort("")
	field, ok := sort[0]["@timestamp"].(map[string]any)
	if !ok {
		t.Fatalf("expected @timestamp sort, got %#v", sort)
	}
	if field["order"] != "desc" {
		t.Fatalf("expected desc sort, got %#v", field["order"])
	}
}

func TestBuildSortAllowsKnownFields(t *testing.T) {
	sort := buildSort("response_time_ms:asc")
	field, ok := sort[0]["response_time_ms"].(map[string]any)
	if !ok {
		t.Fatalf("expected response_time_ms sort, got %#v", sort)
	}
	if field["order"] != "asc" {
		t.Fatalf("expected asc sort, got %#v", field["order"])
	}
}

func TestBuildSortRejectsUnknownFields(t *testing.T) {
	sort := buildSort("unknown_field:asc")
	field, ok := sort[0]["@timestamp"].(map[string]any)
	if !ok {
		t.Fatalf("expected fallback @timestamp sort, got %#v", sort)
	}
	if field["order"] != "desc" {
		t.Fatalf("expected fallback desc sort, got %#v", field["order"])
	}
}

func TestBuildSortRejectsUnknownDirections(t *testing.T) {
	sort := buildSort("status_code:sideways")
	field, ok := sort[0]["status_code"].(map[string]any)
	if !ok {
		t.Fatalf("expected status_code sort, got %#v", sort)
	}
	if field["order"] != "desc" {
		t.Fatalf("expected desc sort for invalid direction, got %#v", field["order"])
	}
}

func TestBuildBoolQueryExcludesControlPlaneWhenEnabled(t *testing.T) {
	query := buildBoolQuery(model.LogQuery{ExcludeControlPlane: true})
	boolQuery, ok := query["bool"].(map[string]any)
	if !ok {
		t.Fatalf("expected bool query, got %#v", query)
	}
	mustNot, ok := boolQuery["must_not"].([]map[string]any)
	if !ok || len(mustNot) != 4 {
		t.Fatalf("expected control-plane exclusions, got %#v", boolQuery["must_not"])
	}
}

func TestBuildBoolQueryKeepsLogsUnfilteredByDefault(t *testing.T) {
	query := buildBoolQuery(model.LogQuery{})
	boolQuery, ok := query["bool"].(map[string]any)
	if !ok {
		t.Fatalf("expected bool query, got %#v", query)
	}
	if _, exists := boolQuery["must_not"]; exists {
		t.Fatalf("expected logs query to keep control-plane logs, got %#v", boolQuery)
	}
}
