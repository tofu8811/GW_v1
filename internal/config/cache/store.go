package cache

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

var ErrRouteNotFound = errors.New("route config not found")

type Store struct {
	db     *pgxpool.Pool
	redis  *redis.Client
	logger *slog.Logger
	config Config

	mu                   sync.RWMutex
	routes               []RouteValue
	routesByKey          map[string]RouteValue
	servicesByID         map[string]ServiceValue
	instancesByServiceID map[string][]InstanceValue
	apiKeysByHash        map[string]APIKeyValue
	aggregationsByKey    map[string]AggregationValue
	pipelines            map[string][]PipelineValue
	pluginMeta           map[string]PluginMetaValue
	localVersion         int64
	ready                bool
}

func NewStore(db *pgxpool.Pool, redisClient *redis.Client, logger *slog.Logger, config Config) *Store {
	if config.SchemaVersion == 0 {
		config.SchemaVersion = CurrentSchemaVersion
	}
	if config.PollInterval <= 0 {
		config.PollInterval = 20 * time.Second
	}
	if config.RebuildLockTTL <= 0 {
		config.RebuildLockTTL = 10 * time.Second
	}
	if config.RebuildLockWait <= 0 {
		config.RebuildLockWait = 2 * time.Second
	}

	return &Store{
		db:                   db,
		redis:                redisClient,
		logger:               logger,
		config:               config,
		routesByKey:          map[string]RouteValue{},
		servicesByID:         map[string]ServiceValue{},
		instancesByServiceID: map[string][]InstanceValue{},
		apiKeysByHash:        map[string]APIKeyValue{},
		aggregationsByKey:    map[string]AggregationValue{},
		pipelines:            map[string][]PipelineValue{},
		pluginMeta:           map[string]PluginMetaValue{},
	}
}

func (s *Store) Ready() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.ready
}

func (s *Store) WarmAll(ctx context.Context) error {
	if err := s.RebuildAll(ctx); err != nil {
		return err
	}

	s.mu.Lock()
	s.ready = true
	s.mu.Unlock()

	return nil
}

func (s *Store) RebuildRoute(ctx context.Context, routeID string) error {
	return s.RebuildAll(ctx)
}

func (s *Store) RebuildAll(ctx context.Context) error {
	locked, err := s.redis.SetNX(ctx, KeyRebuildLock, "1", s.config.RebuildLockTTL).Result()
	if err != nil {
		s.logger.Warn("config rebuild lock unavailable; using local config if present", "error", err)
		return err
	}

	if !locked {
		return s.waitForRebuildThenLoad(ctx)
	}

	defer func() {
		if err := s.redis.Del(context.Background(), KeyRebuildLock).Err(); err != nil {
			s.logger.Warn("failed to release config rebuild lock", "error", err)
		}
	}()

	snap, err := s.readSnapshot(ctx)
	if err != nil {
		return err
	}

	if err := s.writeSnapshot(ctx, snap); err != nil {
		return err
	}

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

func (s *Store) waitForRebuildThenLoad(ctx context.Context) error {
	// Nodes that lose the rebuild lock wait for the writer, so only one node hits PostgreSQL.
	deadline := time.NewTimer(s.config.RebuildLockTTL + s.config.RebuildLockWait)
	defer deadline.Stop()

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-deadline.C:
			return s.loadFromRedis(ctx)
		case <-ticker.C:
			exists, err := s.redis.Exists(ctx, KeyRebuildLock).Result()
			if err != nil {
				return err
			}
			if exists == 0 {
				return s.loadFromRedis(ctx)
			}
		}
	}
}

func (s *Store) readSnapshot(ctx context.Context) (snapshot, error) {
	tx, err := s.db.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead})
	if err != nil {
		return snapshot{}, err
	}
	defer tx.Rollback(ctx)

	services, err := readServices(ctx, tx, s.config.SchemaVersion)
	if err != nil {
		return snapshot{}, err
	}

	instancesByServiceID, err := readServiceInstances(ctx, tx)
	if err != nil {
		return snapshot{}, err
	}

	routes, err := readRoutes(ctx, tx, s.config.SchemaVersion, s.config.CORSSource)
	if err != nil {
		return snapshot{}, err
	}

	pipelines, pluginMeta, err := readPlugins(ctx, tx, s.config.SchemaVersion)
	if err != nil {
		return snapshot{}, err
	}

	apiKeys, err := readAPIKeys(ctx, tx, s.config.SchemaVersion)
	if err != nil {
		return snapshot{}, err
	}

	aggregations, err := readAggregations(ctx, tx, s.config.SchemaVersion, s.config.CORSSource)
	if err != nil {
		return snapshot{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return snapshot{}, err
	}

	return snapshot{
		Services:             services,
		InstancesByServiceID: instancesByServiceID,
		Routes:               routes,
		APIKeys:              apiKeys,
		Aggregations:         aggregations,
		Pipelines:            pipelines,
		PluginMeta:           pluginMeta,
	}, nil
}

func (s *Store) applySnapshot(snap snapshot) {
	servicesByID := map[string]ServiceValue{}
	for _, service := range snap.Services {
		servicesByID[service.ID] = service
	}

	instancesByServiceID := map[string][]InstanceValue{}
	for serviceID, instances := range snap.InstancesByServiceID {
		instancesByServiceID[serviceID] = cloneInstances(instances)
	}

	routesByKey := map[string]RouteValue{}
	for _, route := range snap.Routes {
		routesByKey[routeKey(route.Method, route.Path)] = route
	}

	apiKeysByHash := map[string]APIKeyValue{}
	for _, apiKey := range snap.APIKeys {
		apiKeysByHash[apiKey.KeyHash] = apiKey
	}

	aggregationsByKey := map[string]AggregationValue{}
	for _, aggregation := range snap.Aggregations {
		aggregationsByKey[aggregationKey(aggregation.Method, aggregation.Path)] = aggregation
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.routes = snap.Routes
	s.routesByKey = routesByKey
	s.servicesByID = servicesByID
	s.instancesByServiceID = instancesByServiceID
	s.apiKeysByHash = apiKeysByHash
	s.aggregationsByKey = aggregationsByKey
	s.pipelines = snap.Pipelines
	s.pluginMeta = snap.PluginMeta
	s.localVersion = snap.Version
}

func (s *Store) CurrentVersion() int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.localVersion
}
