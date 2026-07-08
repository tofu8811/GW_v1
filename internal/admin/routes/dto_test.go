package routes

import (
	"encoding/json"
	"testing"
)

func TestUpdateRouteRequestRequiredScopeIDCanBeCleared(t *testing.T) {
	var req UpdateRouteRequest
	if err := json.Unmarshal([]byte(`{"required_scope_id":null}`), &req); err != nil {
		t.Fatalf("unmarshal request: %v", err)
	}

	if !req.RequiredScopeID.Set {
		t.Fatal("expected required_scope_id to be marked as provided")
	}
	if req.RequiredScopeID.Value != nil {
		t.Fatalf("expected nil required_scope_id, got %q", *req.RequiredScopeID.Value)
	}
}

func TestUpdateRouteRequestRequiredScopeIDCanBeOmitted(t *testing.T) {
	var req UpdateRouteRequest
	if err := json.Unmarshal([]byte(`{}`), &req); err != nil {
		t.Fatalf("unmarshal request: %v", err)
	}

	if req.RequiredScopeID.Set {
		t.Fatal("expected omitted required_scope_id to remain unset")
	}
}

func TestUpdateRouteRequestRequiredScopeIDCanBeSet(t *testing.T) {
	var req UpdateRouteRequest
	if err := json.Unmarshal([]byte(`{"required_scope_id":"c0000000-0000-0000-0000-000000000201"}`), &req); err != nil {
		t.Fatalf("unmarshal request: %v", err)
	}

	if !req.RequiredScopeID.Set {
		t.Fatal("expected required_scope_id to be marked as provided")
	}
	if req.RequiredScopeID.Value == nil || *req.RequiredScopeID.Value != "c0000000-0000-0000-0000-000000000201" {
		t.Fatalf("unexpected required_scope_id value: %#v", req.RequiredScopeID.Value)
	}
}
