package aggregations

import (
	"encoding/json"
	"testing"
)

func TestNormalizeRequestTemplateRequiresMethodAndPath(t *testing.T) {
	if _, err := normalizeRequestTemplate(json.RawMessage(`{"method":"GET"}`)); err == nil {
		t.Fatal("expected missing path error")
	}
	if _, err := normalizeRequestTemplate(json.RawMessage(`{"method":"NOPE","path":"/products"}`)); err == nil {
		t.Fatal("expected invalid method error")
	}
}

func TestNormalizeRequestTemplateNormalizesMethodAndPath(t *testing.T) {
	payload, err := normalizeRequestTemplate(json.RawMessage(`{"method":"get","path":"products"}`))
	if err != nil {
		t.Fatal(err)
	}
	if string(payload) != `{"method":"GET","path":"/products"}` {
		t.Fatalf("unexpected template: %s", string(payload))
	}
}

func TestNormalizeResponseMappingRequiresTarget(t *testing.T) {
	if _, err := normalizeResponseMapping(json.RawMessage(`{"target":" "}`)); err == nil {
		t.Fatal("expected target error")
	}
}

func TestNullableUUIDCanBeCleared(t *testing.T) {
	var req UpdateAggregationStepRequest
	if err := json.Unmarshal([]byte(`{"depends_on":null}`), &req); err != nil {
		t.Fatal(err)
	}
	if !req.DependsOn.Set || req.DependsOn.Value != nil {
		t.Fatalf("expected depends_on set to nil: %#v", req.DependsOn)
	}
}
