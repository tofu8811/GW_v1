package health

import (
	"context"

	"gateway-api/internal/proxy/loadbalancer"
	"gateway-api/internal/upstream/breaker"
)

type AliveSetReader interface {
	AliveSet(ctx context.Context, serviceID string) (map[string]struct{}, error)
}

type HealthFilter struct {
	store    AliveSetReader
	breakers *breaker.Registry
}

func NewHealthFilter(store AliveSetReader, breakers *breaker.Registry) *HealthFilter {
	return &HealthFilter{store: store, breakers: breakers}
}

func (f *HealthFilter) KeepAlive(ctx context.Context, serviceID string, in []loadbalancer.Instance, breakerEnabled bool) []loadbalancer.Instance {
	if f == nil || f.store == nil {
		return cloneInstances(in)
	}

	alive, err := f.store.AliveSet(ctx, serviceID)
	failOpen := err != nil || len(alive) == 0

	out := make([]loadbalancer.Instance, 0, len(in))
	for _, instance := range in {
		if !failOpen {
			if _, ok := alive[instance.ID]; !ok {
				continue
			}
		}
		if breakerEnabled && f.breakers != nil && f.breakers.Get(instance.ID).State() == breaker.Open {
			continue
		}
		out = append(out, instance)
	}

	return out
}

func cloneInstances(in []loadbalancer.Instance) []loadbalancer.Instance {
	out := make([]loadbalancer.Instance, len(in))
	copy(out, in)
	return out
}
