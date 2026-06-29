package breaker

import "sync"

type Registry struct {
	mu       sync.RWMutex
	breakers map[string]*Breaker
	cfg      Config
}

func NewRegistry(cfg Config) *Registry {
	return &Registry{
		breakers: map[string]*Breaker{},
		cfg:      normalizeConfig(cfg),
	}
}

func (r *Registry) Get(instanceID string) *Breaker {
	r.mu.RLock()
	br := r.breakers[instanceID]
	r.mu.RUnlock()
	if br != nil {
		return br
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if br = r.breakers[instanceID]; br != nil {
		return br
	}
	br = New(r.cfg)
	r.breakers[instanceID] = br
	return br
}

func (r *Registry) Prune(keep map[string]struct{}) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for instanceID := range r.breakers {
		if _, ok := keep[instanceID]; !ok {
			delete(r.breakers, instanceID)
		}
	}
}
