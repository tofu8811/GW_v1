package loadbalancer

import (
	"errors"
	"sort"
	"sync"
)

var ErrNoInstance = errors.New("no available instance")

type RoundRobin struct {
	mutex       sync.Mutex
	counters map[string]uint64
}

func NewRoundRobin() *RoundRobin {
	return &RoundRobin{counters: make(map[string]uint64)}
}

// Pick expects instances to contain only currently healthy instances.
func (r *RoundRobin) Pick(serviceID string, instances []Instance) (Instance, error) {
	if len(instances) == 0 {
		return Instance{}, ErrNoInstance
	}

	sorted := make([]Instance, len(instances))
	copy(sorted, instances)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].ID < sorted[j].ID
	})

	r.mutex.Lock()
	index := r.counters[serviceID] % uint64(len(sorted))
	r.counters[serviceID]++
	r.mutex.Unlock()

	return sorted[int(index)], nil
}
