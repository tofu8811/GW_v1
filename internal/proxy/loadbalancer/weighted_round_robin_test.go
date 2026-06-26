package loadbalancer

import "testing"

func TestWeightedRoundRobinPickHonorsInstanceWeight(t *testing.T) {
	lb := NewWeightedRoundRobin()
	instances := []Instance{
		{ID: "b", ServiceID: "service-1", Weight: 1},
		{ID: "a", ServiceID: "service-1", Weight: 2},
	}

	got := make([]string, 0, 4)
	for range 4 {
		instance, err := lb.Pick("service-1", instances)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		got = append(got, instance.ID)
	}

	want := []string{"a", "a", "b", "a"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("expected sequence %v, got %v", want, got)
		}
	}
}

func TestWeightedRoundRobinPickSkipsZeroWeightInstances(t *testing.T) {
	lb := NewWeightedRoundRobin()
	instances := []Instance{
		{ID: "a", ServiceID: "service-1", Weight: 0},
		{ID: "b", ServiceID: "service-1", Weight: 1},
	}

	for range 3 {
		instance, err := lb.Pick("service-1", instances)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if instance.ID != "b" {
			t.Fatalf("expected only positive-weight instance b, got %s", instance.ID)
		}
	}
}
