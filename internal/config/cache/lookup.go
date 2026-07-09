package cache

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/redis/go-redis/v9"
)

func (s *Store) GetRouteFromCache(ctx context.Context, method string, path string) (*RouteValue, error) {
	key := routeKey(method, path)
	value, err := s.redis.Get(ctx, key).Bytes()
	if err == nil {
		var route RouteValue
		if jsonErr := json.Unmarshal(value, &route); jsonErr == nil && route.SchemaVersion == s.config.SchemaVersion {
			s.rememberRoute(route)
			return &route, nil
		}
		// Unknown schemas are ignored so the control plane can rebuild a compatible value.
		_ = s.RebuildAll(ctx)
	}
	if err != nil && !errors.Is(err, redis.Nil) {
		s.logger.Warn("redis route cache read failed; using local config", "key", key, "error", err)
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	route, ok := s.routesByKey[key]
	if !ok {
		return nil, ErrRouteNotFound
	}

	return cloneRoute(route), nil
}

func (s *Store) FindCandidates(method string) []RouteValue {
	s.mu.RLock()
	defer s.mu.RUnlock()

	routes := make([]RouteValue, 0, len(s.routes))
	for _, route := range s.routes {
		if route.Method == method || route.Method == "ANY" {
			routes = append(routes, *cloneRoute(route))
		}
	}

	return routes
}

func (s *Store) FindAggregation(method string, path string) (*AggregationValue, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	aggregation, ok := s.aggregationsByKey[aggregationKey(method, path)]
	if !ok {
		return nil, false
	}

	return cloneAggregation(aggregation), true
}
func (s *Store) FindAPIKeyByHash(hash string) (APIKeyValue, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	apiKey, ok := s.apiKeysByHash[hash]
	if !ok {
		return APIKeyValue{}, false
	}

	return cloneAPIKey(apiKey), true
}
func (s *Store) Pipeline(routeID string) []PipelineValue {
	s.mu.RLock()
	defer s.mu.RUnlock()

	pipeline := s.pipelines[routeID]
	out := make([]PipelineValue, len(pipeline))
	copy(out, pipeline)
	return out
}

func (s *Store) FindService(serviceID string) (ServiceValue, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	service, ok := s.servicesByID[serviceID]
	return service, ok
}

func (s *Store) FindInstancesByServiceID(serviceID string) []InstanceValue {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return cloneInstances(s.instancesByServiceID[serviceID])
}

func (s *Store) ActiveInstances() []ActiveInstanceValue {
	s.mu.RLock()
	defer s.mu.RUnlock()

	instances := []ActiveInstanceValue{}
	for serviceID, serviceInstances := range s.instancesByServiceID {
		service, ok := s.servicesByID[serviceID]
		if !ok {
			continue
		}
		for _, instance := range serviceInstances {
			instances = append(instances, ActiveInstanceValue{
				ServiceID:  serviceID,
				InstanceID: instance.ID,
				Host:       instance.Host,
				Port:       instance.Port,
				HealthPath: service.HealthPath,
			})
		}
	}

	return instances
}

func (s *Store) FindActiveInstance(instanceID string) (ActiveInstanceValue, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for serviceID, serviceInstances := range s.instancesByServiceID {
		service, ok := s.servicesByID[serviceID]
		if !ok {
			continue
		}
		for _, instance := range serviceInstances {
			if instance.ID == instanceID {
				return ActiveInstanceValue{
					ServiceID:  serviceID,
					InstanceID: instance.ID,
					Host:       instance.Host,
					Port:       instance.Port,
					HealthPath: service.HealthPath,
				}, true
			}
		}
	}

	return ActiveInstanceValue{}, false
}

func (s *Store) ActiveInstancesByService(serviceID string) []ActiveInstanceValue {
	s.mu.RLock()
	defer s.mu.RUnlock()

	service, ok := s.servicesByID[serviceID]
	if !ok {
		return nil
	}

	instances := []ActiveInstanceValue{}
	for _, instance := range s.instancesByServiceID[serviceID] {
		instances = append(instances, ActiveInstanceValue{
			ServiceID:  serviceID,
			InstanceID: instance.ID,
			Host:       instance.Host,
			Port:       instance.Port,
			HealthPath: service.HealthPath,
		})
	}

	return instances
}

func (s *Store) rememberRoute(route RouteValue) {

	s.mu.Lock()
	defer s.mu.Unlock()

	s.routesByKey[routeKey(route.Method, route.Path)] = route
}
