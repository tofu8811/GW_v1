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
