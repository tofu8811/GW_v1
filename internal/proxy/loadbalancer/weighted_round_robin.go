package loadbalancer

import (
	"errors"
	"sort"
	"sync"
)

var ErrNoWeightedInstance = errors.New("no available weighted instance")

type WeightedRoundRobin struct {
	mu       sync.Mutex
	counters map[string]uint64
}

func NewWeightedRoundRobin() *WeightedRoundRobin {
	return &WeightedRoundRobin{counters: make(map[string]uint64)}
}

// Pick expects instances to contain only currently healthy instances.
func (w *WeightedRoundRobin) Pick(serviceID string, instances []Instance) (Instance, error) {
	if len(instances) == 0 {
		return Instance{}, ErrNoInstance
	}

	sorted := make([]Instance, 0, len(instances))
	for _, instance := range instances {
		if instance.Weight > 0 {
			sorted = append(sorted, instance)
		}
	}
	if len(sorted) == 0 {
		return Instance{}, ErrNoWeightedInstance
	}

	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].ID < sorted[j].ID
	})

	totalWeight := uint64(0)
	for _, instance := range sorted {
		totalWeight += uint64(instance.Weight)
	}

	w.mu.Lock()
	index := w.counters[serviceID] % totalWeight
	w.counters[serviceID]++
	w.mu.Unlock()

	for _, instance := range sorted {
		weight := uint64(instance.Weight)
		if index < weight {
			return instance, nil
		}
		index -= weight
	}

	return sorted[len(sorted)-1], nil
}
