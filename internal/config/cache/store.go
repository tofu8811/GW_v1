package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strconv"
	"strings"
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

	mu           sync.RWMutex
	routes       []RouteValue
	routesByKey  map[string]RouteValue
	pipelines    map[string][]PipelineValue
	pluginMeta   map[string]PluginMetaValue
	localVersion int64
	ready        bool
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
		db:          db,
		redis:       redisClient,
		logger:      logger,
		config:      config,
		routesByKey: map[string]RouteValue{},
		pipelines:   map[string][]PipelineValue{},
		pluginMeta:  map[string]PluginMetaValue{},
	}
}

func (s *Store) Ready() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.ready
}

func (s *Store) WarmAll(ctx context.Context) error {
	if err := s.rebuildAll(ctx); err != nil {
		return err
	}

	s.mu.Lock()
	s.ready = true
	s.mu.Unlock()

	return nil
}

func (s *Store) SubscribeReload(ctx context.Context) {
	pubsub := s.redis.Subscribe(ctx, KeyReload)
	defer pubsub.Close()

	ch := pubsub.Channel()
	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}
			// Fast path for admin changes; missed messages are covered by PollVersion.
			if err := s.rebuildAll(ctx); err != nil {
				s.logger.Error("config reload failed", "group", msg.Payload, "error", err)
			}
		}
	}
}

func (s *Store) PollVersion(ctx context.Context) {
	ticker := time.NewTicker(s.config.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			version, err := s.redis.Get(ctx, KeyVersion).Int64()
			if errors.Is(err, redis.Nil) {
				version = 0
			} else if err != nil {
				s.logger.Warn("config version poll failed", "error", err)
				continue
			}

			if version != s.currentVersion() {
				if err := s.rebuildAll(ctx); err != nil {
					s.logger.Error("config version rebuild failed", "redis_version", version, "error", err)
				}
			}
		}
	}
}

func (s *Store) RebuildRoute(ctx context.Context, routeID string) error {
	return s.rebuildAll(ctx)
}

func (s *Store) NotifyChange(ctx context.Context, group string) error {
	if _, err := s.redis.Incr(ctx, KeyVersion).Result(); err != nil {
		return err
	}
	return s.redis.Publish(ctx, KeyReload, group).Err()
}

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
		_ = s.rebuildAll(ctx)
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

func (s *Store) Pipeline(routeID string) []PipelineValue {
	s.mu.RLock()
	defer s.mu.RUnlock()

	pipeline := s.pipelines[routeID]
	out := make([]PipelineValue, len(pipeline))
	copy(out, pipeline)
	return out
}

func (s *Store) rebuildAll(ctx context.Context) error {
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

	routes, err := readRoutes(ctx, tx, s.config.SchemaVersion)
	if err != nil {
		return snapshot{}, err
	}

	pipelines, pluginMeta, err := readPlugins(ctx, tx, s.config.SchemaVersion)
	if err != nil {
		return snapshot{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return snapshot{}, err
	}

	return snapshot{
		Routes:     routes,
		Pipelines:  pipelines,
		PluginMeta: pluginMeta,
	}, nil
}

func readRoutes(ctx context.Context, tx pgx.Tx, schemaVersion int) ([]RouteValue, error) {
	rows, err := tx.Query(ctx, `
		SELECT
			r.id::text,
			r.path,
			r.method,
			r.strip_prefix,
			r.rewrite_target,
			r.auth_required,
			r.rate_limit_id::text,
			r.priority,
			s.id::text,
			s.name,
			s.protocol,
			s.lb_strategy,
			s.timeout_ms,
			s.retry_count,
			COALESCE(
				jsonb_agg(
					jsonb_build_object(
						'id', si.id::text,
						'host', si.host,
						'port', si.port,
						'weight', si.weight
					)
					ORDER BY si.created_at ASC
				) FILTER (WHERE si.id IS NOT NULL),
				'[]'::jsonb
			)
		FROM routes r
		JOIN services s ON s.id = r.service_id
		LEFT JOIN service_instances si ON si.service_id = s.id AND si.is_active = TRUE
		WHERE r.is_active = TRUE
		  AND s.is_active = TRUE
		  AND s.protocol = 'http'
		GROUP BY r.id, s.id
		ORDER BY r.priority DESC, length(r.path) DESC, r.created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var routes []RouteValue
	for rows.Next() {
		var route RouteValue
		var rateLimitID *string
		var instances []byte

		err := rows.Scan(
			&route.RouteID,
			&route.Path,
			&route.Method,
			&route.StripPrefix,
			&route.RewriteTarget,
			&route.AuthRequired,
			&rateLimitID,
			&route.Priority,
			&route.Service.ID,
			&route.Service.Name,
			&route.Service.Protocol,
			&route.Service.LBStrategy,
			&route.Service.TimeoutMS,
			&route.Service.RetryCount,
			&instances,
		)
		if err != nil {
			return nil, err
		}

		route.SchemaVersion = schemaVersion
		route.RateLimitID = rateLimitID
		if err := json.Unmarshal(instances, &route.Instances); err != nil {
			return nil, err
		}

		routes = append(routes, route)
	}

	return routes, rows.Err()
}

func readPlugins(ctx context.Context, tx pgx.Tx, schemaVersion int) (map[string][]PipelineValue, map[string]PluginMetaValue, error) {
	rows, err := tx.Query(ctx, `
		SELECT
			rp.route_id::text,
			gp.code,
			gp.name,
			gp.description,
			gp.phase,
			gp.default_priority,
			gp.config_schema,
			rp.priority,
			rp.config,
			rp.is_required
		FROM route_plugins rp
		JOIN gateway_plugins gp ON gp.id = rp.plugin_id
		WHERE rp.is_active = TRUE
		  AND gp.is_active = TRUE
		ORDER BY rp.route_id, gp.phase, rp.priority
	`)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	pipelines := map[string][]PipelineValue{}
	pluginMeta := map[string]PluginMetaValue{}

	for rows.Next() {
		var routeID string
		var meta PluginMetaValue
		var step PipelineValue

		err := rows.Scan(
			&routeID,
			&meta.Code,
			&meta.Name,
			&meta.Description,
			&meta.Phase,
			&meta.DefaultPriority,
			&meta.ConfigSchema,
			&step.Priority,
			&step.Config,
			&step.IsRequired,
		)
		if err != nil {
			return nil, nil, err
		}

		meta.SchemaVersion = schemaVersion
		step.Code = meta.Code
		step.Phase = meta.Phase
		pipelines[routeID] = append(pipelines[routeID], step)
		pluginMeta[meta.Code] = meta
	}

	for routeID := range pipelines {
		sort.SliceStable(pipelines[routeID], func(i, j int) bool {
			if pipelines[routeID][i].Phase == pipelines[routeID][j].Phase {
				return pipelines[routeID][i].Priority < pipelines[routeID][j].Priority
			}
			return phaseOrder(pipelines[routeID][i].Phase) < phaseOrder(pipelines[routeID][j].Phase)
		})
	}

	return pipelines, pluginMeta, rows.Err()
}

func (s *Store) writeSnapshot(ctx context.Context, snap snapshot) error {
	// MULTI/EXEC keeps each route JSON, plugins, and pipeline visible as one config generation.
	pipe := s.redis.TxPipeline()

	staleKeys, err := s.scanConfigKeys(ctx, "cfg:route:*", "cfg:pipeline:*", "cfg:plugins:*", "cfg:plugin:*")
	if err != nil {
		return err
	}
	if len(staleKeys) > 0 {
		pipe.Del(ctx, staleKeys...)
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

	for _, meta := range snap.PluginMeta {
		if err := setJSON(ctx, pipe, fmt.Sprintf("cfg:plugin:%s", meta.Code), meta, s.config.ConfigTTL); err != nil {
			return err
		}
	}

	_, err = pipe.Exec(ctx)
	return err
}

func (s *Store) loadFromRedis(ctx context.Context) error {
	keys, err := s.scanConfigKeys(ctx, "cfg:route:*")
	if err != nil {
		return err
	}

	snap := snapshot{
		Pipelines:  map[string][]PipelineValue{},
		PluginMeta: map[string]PluginMetaValue{},
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
			return s.rebuildAll(ctx)
		}

		pipelineValue, err := s.readPipeline(ctx, route.RouteID)
		if err != nil {
			return err
		}

		snap.Routes = append(snap.Routes, route)
		snap.Pipelines[route.RouteID] = pipelineValue.Items
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

func (s *Store) applySnapshot(snap snapshot) {
	routesByKey := map[string]RouteValue{}
	for _, route := range snap.Routes {
		routesByKey[routeKey(route.Method, route.Path)] = route
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.routes = snap.Routes
	s.routesByKey = routesByKey
	s.pipelines = snap.Pipelines
	s.pluginMeta = snap.PluginMeta
	s.localVersion = snap.Version
}

func (s *Store) rememberRoute(route RouteValue) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.routesByKey[routeKey(route.Method, route.Path)] = route
}

func (s *Store) currentVersion() int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.localVersion
}

func setJSON(ctx context.Context, pipe redis.Pipeliner, key string, value any, ttl time.Duration) error {
	payload, err := json.Marshal(value)
	if err != nil {
		return err
	}

	if ttl > 0 {
		pipe.Set(ctx, key, payload, ttl)
		return nil
	}

	pipe.Set(ctx, key, payload, 0)
	return nil
}

func routeKey(method string, path string) string {
	return fmt.Sprintf("cfg:route:%s:%s", strings.ToUpper(method), path)
}

func phaseOrder(phase string) int {
	switch phase {
	case "before_request":
		return 10
	case "proxy":
		return 20
	case "after_response":
		return 30
	case "on_error":
		return 40
	default:
		return 100
	}
}

func sortRoutes(routes []RouteValue) {
	sort.SliceStable(routes, func(i, j int) bool {
		if routes[i].Priority != routes[j].Priority {
			return routes[i].Priority > routes[j].Priority
		}
		if len(routes[i].Path) != len(routes[j].Path) {
			return len(routes[i].Path) > len(routes[j].Path)
		}
		return routes[i].RouteID < routes[j].RouteID
	})
}

func cloneRoute(route RouteValue) *RouteValue {
	cloned := route
	if route.Instances != nil {
		cloned.Instances = make([]InstanceValue, len(route.Instances))
		copy(cloned.Instances, route.Instances)
	}
	return &cloned
}

func ParseDurationSeconds(value string, fallback time.Duration) time.Duration {
	seconds, err := strconv.Atoi(value)
	if err != nil || seconds < 0 {
		return fallback
	}
	return time.Duration(seconds) * time.Second
}
