package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/redis/go-redis/v9"
)

func (s *Store) writeSnapshot(ctx context.Context, snap snapshot) error {
	// MULTI/EXEC keeps each route JSON, plugins, and pipeline visible as one config generation.
	pipe := s.redis.TxPipeline()

	staleKeys, err := s.scanConfigKeys(ctx, "cfg:route:*", "cfg:service:*", "cfg:service-instances:*", "cfg:apikey:*", "cfg:aggregation:*", "cfg:pipeline:*", "cfg:plugins:*", "cfg:plugin:*")
	if err != nil {
		return err
	}
	if len(staleKeys) > 0 {
		pipe.Del(ctx, staleKeys...)
	}

	for _, service := range snap.Services {
		if err := setJSON(ctx, pipe, serviceKey(service.ID), service, s.config.ConfigTTL); err != nil {
			return err
		}
	}

	for serviceID, instances := range snap.InstancesByServiceID {
		value := ServiceInstancesValue{SchemaVersion: s.config.SchemaVersion, ServiceID: serviceID, Items: instances}
		if err := setJSON(ctx, pipe, serviceInstancesKey(serviceID), value, s.config.ConfigTTL); err != nil {
			return err
		}
	}

	for _, route := range snap.Routes {
		if err := setJSON(ctx, pipe, routeKey(route.Method, route.Path), route, s.config.ConfigTTL); err != nil {
			return err
		}

		routePlugins := RoutePluginsValue{
			SchemaVersion: s.config.SchemaVersion,
			Items:         snap.Pipelines[route.RouteID],
		}
		if err := setJSON(ctx, pipe, fmt.Sprintf("cfg:plugins:%s", route.RouteID), routePlugins, s.config.ConfigTTL); err != nil {
			return err
		}

		pipeline := PipelineCacheValue{
			SchemaVersion: s.config.SchemaVersion,
			Items:         snap.Pipelines[route.RouteID],
		}
		if err := setJSON(ctx, pipe, fmt.Sprintf("cfg:pipeline:%s", route.RouteID), pipeline, s.config.ConfigTTL); err != nil {
			return err
		}
	}

	for _, aggregation := range snap.Aggregations {
		if err := setJSON(ctx, pipe, aggregationKey(aggregation.Method, aggregation.Path), aggregation, s.config.ConfigTTL); err != nil {
			return err
		}
	}
	for _, apiKey := range snap.APIKeys {
		if err := setJSON(ctx, pipe, fmt.Sprintf("cfg:apikey:%s", apiKey.KeyHash), apiKey, s.config.ConfigTTL); err != nil {
			return err
		}
	}
	for _, meta := range snap.PluginMeta {
		if err := setJSON(ctx, pipe, fmt.Sprintf("cfg:plugin:%s", meta.Code), meta, s.config.ConfigTTL); err != nil {
			return err
		}
	}

	_, err = pipe.Exec(ctx)
	return err
}

func (s *Store) loadFromRedis(ctx context.Context) error {
	snap := snapshot{
		InstancesByServiceID: map[string][]InstanceValue{},
		Pipelines:            map[string][]PipelineValue{},
		PluginMeta:           map[string]PluginMetaValue{},
	}

	serviceKeys, err := s.scanConfigKeys(ctx, "cfg:service:*")
	if err != nil {
		return err
	}
	for _, key := range serviceKeys {
		value, err := s.redis.Get(ctx, key).Bytes()
		if err != nil {
			return err
		}

		var service ServiceValue
		if err := json.Unmarshal(value, &service); err != nil {
			return err
		}
		if service.SchemaVersion != s.config.SchemaVersion {
			return s.RebuildAll(ctx)
		}
		snap.Services = append(snap.Services, service)
	}

	instanceKeys, err := s.scanConfigKeys(ctx, "cfg:service-instances:*")
	if err != nil {
		return err
	}
	for _, key := range instanceKeys {
		value, err := s.redis.Get(ctx, key).Bytes()
		if err != nil {
			return err
		}

		var instances ServiceInstancesValue
		if err := json.Unmarshal(value, &instances); err != nil {
			return err
		}
		if instances.SchemaVersion != s.config.SchemaVersion {
			return s.RebuildAll(ctx)
		}
		snap.InstancesByServiceID[instances.ServiceID] = cloneInstances(instances.Items)
	}

	keys, err := s.scanConfigKeys(ctx, "cfg:route:*")
	if err != nil {
		return err
	}
	for _, key := range keys {
		value, err := s.redis.Get(ctx, key).Bytes()
		if err != nil {
			return err
		}

		var route RouteValue
		if err := json.Unmarshal(value, &route); err != nil {
			return err
		}
		if route.SchemaVersion != s.config.SchemaVersion {
			return s.RebuildAll(ctx)
		}

		pipelineValue, err := s.readPipeline(ctx, route.RouteID)
		if err != nil {
			return err
		}

		snap.Routes = append(snap.Routes, route)
		snap.Pipelines[route.RouteID] = pipelineValue.Items
	}

	apiKeyKeys, err := s.scanConfigKeys(ctx, "cfg:apikey:*")
	if err != nil {
		return err
	}
	for _, key := range apiKeyKeys {
		value, err := s.redis.Get(ctx, key).Bytes()
		if err != nil {
			return err
		}

		var apiKey APIKeyValue
		if err := json.Unmarshal(value, &apiKey); err != nil {
			return err
		}
		if apiKey.SchemaVersion != s.config.SchemaVersion {
			return s.RebuildAll(ctx)
		}

		snap.APIKeys = append(snap.APIKeys, apiKey)
	}

	aggregationKeys, err := s.scanConfigKeys(ctx, "cfg:aggregation:*")
	if err != nil {
		return err
	}
	for _, key := range aggregationKeys {
		value, err := s.redis.Get(ctx, key).Bytes()
		if err != nil {
			return err
		}

		var aggregation AggregationValue
		if err := json.Unmarshal(value, &aggregation); err != nil {
			return err
		}
		if aggregation.SchemaVersion != s.config.SchemaVersion {
			return s.RebuildAll(ctx)
		}

		snap.Aggregations = append(snap.Aggregations, aggregation)
	}
	sortRoutes(snap.Routes)

	version, err := s.redis.Get(ctx, KeyVersion).Int64()
	if errors.Is(err, redis.Nil) {
		version = 0
	} else if err != nil {
		return err
	}
	snap.Version = version

	s.applySnapshot(snap)
	return nil
}
func (s *Store) readPipeline(ctx context.Context, routeID string) (PipelineCacheValue, error) {
	value, err := s.redis.Get(ctx, fmt.Sprintf("cfg:pipeline:%s", routeID)).Bytes()
	if errors.Is(err, redis.Nil) {
		return PipelineCacheValue{SchemaVersion: s.config.SchemaVersion}, nil
	}
	if err != nil {
		return PipelineCacheValue{}, err
	}

	var pipeline PipelineCacheValue
	if err := json.Unmarshal(value, &pipeline); err != nil {
		return PipelineCacheValue{}, err
	}
	if pipeline.SchemaVersion != s.config.SchemaVersion {
		return PipelineCacheValue{}, fmt.Errorf("unsupported pipeline schema_version %d", pipeline.SchemaVersion)
	}

	return pipeline, nil
}

func (s *Store) scanConfigKeys(ctx context.Context, patterns ...string) ([]string, error) {
	var keys []string
	for _, pattern := range patterns {
		var cursor uint64
		for {
			found, next, err := s.redis.Scan(ctx, cursor, pattern, 100).Result()
			if err != nil {
				return nil, err
			}
			keys = append(keys, found...)
			cursor = next
			if cursor == 0 {
				break
			}
		}
	}

	return keys, nil
}
