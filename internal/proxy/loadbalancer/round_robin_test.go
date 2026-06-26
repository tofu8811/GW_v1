package loadbalancer

import "testing"

func TestRoundRobinPickRotatesInstancesInStableOrder(t *testing.T) {
	lb := NewRoundRobin()
	instances := []Instance{
		{ID: "b", ServiceID: "service-1"},
		{ID: "a", ServiceID: "service-1"},
	}

	first, err := lb.Pick("service-1", instances)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	second, err := lb.Pick("service-1", instances)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	third, err := lb.Pick("service-1", instances)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if first.ID != "a" || second.ID != "b" || third.ID != "a" {
		t.Fatalf("expected a, b, a rotation, got %s, %s, %s", first.ID, second.ID, third.ID)
	}
}
