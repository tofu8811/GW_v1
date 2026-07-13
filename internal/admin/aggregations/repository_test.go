package aggregations

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
)

func TestHasStepCycleDetectsCycle(t *testing.T) {
	a := uuid.New()
	b := uuid.New()
	steps := map[uuid.UUID]AggregationStep{
		a: {ID: a, DependsOn: &b, IsActive: true},
		b: {ID: b, DependsOn: &a, IsActive: true},
	}
	if !hasStepCycle(steps) {
		t.Fatal("expected dependency cycle")
	}
}

func TestValidateUniqueResponseTargetsRejectsDuplicate(t *testing.T) {
	a := uuid.New()
	b := uuid.New()
	steps := map[uuid.UUID]AggregationStep{
		a: {ID: a, IsActive: true, ResponseMapping: json.RawMessage(`{"target":"items"}`)},
		b: {ID: b, IsActive: true, ResponseMapping: json.RawMessage(`{"target":"items"}`)},
	}
	if err := validateUniqueResponseTargets(steps); err != ErrResponseTargetDuplicate {
		t.Fatalf("expected duplicate target error, got %v", err)
	}
}
