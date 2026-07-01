package health

import (
	"context"
	"testing"
	"time"

	"gateway-api/internal/proxy/loadbalancer"
	"gateway-api/internal/upstream/breaker"
)

type fakeAliveStore struct {
	alive map[string]struct{}
	err   error
}

func (f fakeAliveStore) AliveSet(context.Context, string) (map[string]struct{}, error) {
	return f.alive, f.err
}

func TestHealthFilterFailOpenWhenAliveSetEmpty(t *testing.T) {
	instances := testInstances()
	filter := NewHealthFilter(fakeAliveStore{alive: map[string]struct{}{}}, nil)

	got := filter.KeepAlive(context.Background(), "svc", instances, false)
	if len(got) != len(instances) {
		t.Fatalf("expected fail-open to keep all instances, got %d", len(got))
	}
}

func TestHealthFilterKeepsOnlyAliveInstances(t *testing.T) {
	filter := NewHealthFilter(fakeAliveStore{alive: map[string]struct{}{"b": {}}}, nil)

	got := filter.KeepAlive(context.Background(), "svc", testInstances(), false)
	if len(got) != 1 || got[0].ID != "b" {
		t.Fatalf("expected only b, got %#v", got)
	}
}

func TestHealthFilterRemovesOpenBreaker(t *testing.T) {
	registry := breaker.NewRegistry(breaker.Config{FailureThreshold: 1, OpenTimeout: time.Minute, HalfOpenMax: 1})
	registry.Get("a").OnFailure()
	filter := NewHealthFilter(fakeAliveStore{alive: map[string]struct{}{}}, registry)

	got := filter.KeepAlive(context.Background(), "svc", testInstances(), true)
	if len(got) != 1 || got[0].ID != "b" {
		t.Fatalf("expected breaker-open instance a to be removed, got %#v", got)
	}
}

func testInstances() []loadbalancer.Instance {
	return []loadbalancer.Instance{
		{ID: "a", ServiceID: "svc", Host: "127.0.0.1", Port: 1, Weight: 1},
		{ID: "b", ServiceID: "svc", Host: "127.0.0.1", Port: 2, Weight: 1},
	}
}
