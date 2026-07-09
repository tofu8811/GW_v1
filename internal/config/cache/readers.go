package cache

import (
	"context"
	"database/sql"
	"encoding/json"
	"sort"

	"github.com/jackc/pgx/v5"
)

func readServices(ctx context.Context, tx pgx.Tx, schemaVersion int) ([]ServiceValue, error) {
	rows, err := tx.Query(ctx, `
		SELECT id::text, name, protocol, lb_strategy, COALESCE(health_path, ''), timeout_ms, retry_count, circuit_breaker_enabled
		FROM services
		WHERE is_active = TRUE
		  AND deleted_at IS NULL
		  AND protocol = 'http'
		ORDER BY created_at ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	services := []ServiceValue{}
	for rows.Next() {
		var service ServiceValue
		if err := rows.Scan(&service.ID, &service.Name, &service.Protocol, &service.LBStrategy, &service.HealthPath, &service.TimeoutMS, &service.RetryCount, &service.CircuitBreakerEnabled); err != nil {
			return nil, err
		}
		service.SchemaVersion = schemaVersion
		services = append(services, service)
	}

	return services, rows.Err()
}

func readServiceInstances(ctx context.Context, tx pgx.Tx) (map[string][]InstanceValue, error) {
	rows, err := tx.Query(ctx, `
		SELECT si.id::text, si.service_id::text, si.host, si.port, si.weight
		FROM service_instances si
		JOIN services s ON s.id = si.service_id
		WHERE si.is_active = TRUE
		  AND si.deleted_at IS NULL
		  AND s.is_active = TRUE
		  AND s.deleted_at IS NULL
		  AND s.protocol = 'http'
		ORDER BY si.service_id, si.created_at ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	instancesByServiceID := map[string][]InstanceValue{}
	for rows.Next() {
		var instance InstanceValue
		if err := rows.Scan(&instance.ID, &instance.ServiceID, &instance.Host, &instance.Port, &instance.Weight); err != nil {
			return nil, err
		}
		instancesByServiceID[instance.ServiceID] = append(instancesByServiceID[instance.ServiceID], instance)
	}

	return instancesByServiceID, rows.Err()
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
			r.required_scope_id::text,
			r.rate_limit_id::text,
			rlp.id::text,
			COALESCE(rlp.name, ''),
			COALESCE(rlp.limit_type, ''),
			COALESCE(rlp.max_requests, 0),
			COALESCE(rlp.window_seconds, 0),
			cc.id::text,
			COALESCE(cc.allowed_origins, '{}'::text[]),
			COALESCE(cc.allowed_methods, '{}'::text[]),
			COALESCE(cc.allowed_headers, '{}'::text[]),
			COALESCE(cc.allow_credentials, FALSE),
			COALESCE(cc.max_age, 0),
			r.priority,
			r.service_id::text
		FROM routes r
		JOIN services s ON s.id = r.service_id
		LEFT JOIN rate_limit_policies rlp ON rlp.id = r.rate_limit_id AND rlp.is_active = TRUE AND rlp.deleted_at IS NULL
		LEFT JOIN cors_configs cc ON cc.route_id = r.id AND cc.is_active = TRUE AND cc.deleted_at IS NULL
		WHERE r.is_active = TRUE
		  AND r.deleted_at IS NULL
		  AND s.is_active = TRUE
		  AND s.deleted_at IS NULL
		  AND s.protocol = 'http'
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
		var rateLimitPolicyID sql.NullString
		var rateLimitPolicy RateLimitPolicyValue
		var corsID sql.NullString
		var corsValue CORSValue

		err := rows.Scan(
			&route.RouteID,
			&route.Path,
			&route.Method,
			&route.StripPrefix,
			&route.RewriteTarget,
			&route.AuthRequired,
			&route.RequiredScopeID,
			&rateLimitID,
			&rateLimitPolicyID,
			&rateLimitPolicy.Name,
			&rateLimitPolicy.LimitType,
			&rateLimitPolicy.MaxRequests,
			&rateLimitPolicy.WindowSeconds,
			&corsID,
			&corsValue.AllowedOrigins,
			&corsValue.AllowedMethods,
			&corsValue.AllowedHeaders,
			&corsValue.AllowCredentials,
			&corsValue.MaxAge,
			&route.Priority,
			&route.ServiceID,
		)
		if err != nil {
			return nil, err
		}

		route.SchemaVersion = schemaVersion
		route.RateLimitID = rateLimitID
		if rateLimitPolicyID.Valid {
			rateLimitPolicy.ID = rateLimitPolicyID.String
			route.RateLimit = &rateLimitPolicy
		}
		if corsID.Valid {
			route.CORS = &corsValue
		}

		routes = append(routes, route)
	}

	return routes, rows.Err()
}
func readAPIKeys(ctx context.Context, tx pgx.Tx, schemaVersion int) ([]APIKeyValue, error) {
	rows, err := tx.Query(ctx, `
		SELECT
			ak.id::text,
			ak.key_hash,
			ak.key_prefix,
			ak.client_id::text,
			c.owner_user_id::text,
			COALESCE(array_agg(asc_.id::text ORDER BY asc_.resource, asc_.action) FILTER (WHERE asc_.id IS NOT NULL), '{}'::text[]) AS scope_ids,
			ak.rate_limit_id::text,
			ak.expires_at,
			ak.is_active,
			ak.revoked_at,
			c.is_active
		FROM api_keys ak
		JOIN clients c ON c.id = ak.client_id
		LEFT JOIN api_key_scopes aks ON aks.api_key_id = ak.id
		LEFT JOIN api_scopes asc_ ON asc_.id = aks.scope_id AND asc_.deleted_at IS NULL AND asc_.is_active
		WHERE ak.deleted_at IS NULL
		  AND c.deleted_at IS NULL
		GROUP BY ak.id, c.id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	apiKeys := []APIKeyValue{}
	for rows.Next() {
		var apiKey APIKeyValue
		err := rows.Scan(
			&apiKey.ID,
			&apiKey.KeyHash,
			&apiKey.KeyPrefix,
			&apiKey.ClientID,
			&apiKey.OwnerUserID,
			&apiKey.ScopeIDs,
			&apiKey.RateLimitID,
			&apiKey.ExpiresAt,
			&apiKey.IsActive,
			&apiKey.RevokedAt,
			&apiKey.ClientActive,
		)
		if err != nil {
			return nil, err
		}

		apiKey.SchemaVersion = schemaVersion
		apiKeys = append(apiKeys, apiKey)
	}

	return apiKeys, rows.Err()
}

func readAggregations(ctx context.Context, tx pgx.Tx, schemaVersion int) ([]AggregationValue, error) {
	rows, err := tx.Query(ctx, `
		SELECT id::text, name, path, method
		FROM aggregation_configs
		WHERE is_active = TRUE
		  AND deleted_at IS NULL
		ORDER BY length(path) DESC, created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	aggregations := []AggregationValue{}
	byID := map[string]int{}
	for rows.Next() {
		var aggregation AggregationValue
		if err := rows.Scan(&aggregation.ID, &aggregation.Name, &aggregation.Path, &aggregation.Method); err != nil {
			return nil, err
		}
		aggregation.SchemaVersion = schemaVersion
		byID[aggregation.ID] = len(aggregations)
		aggregations = append(aggregations, aggregation)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(aggregations) == 0 {
		return aggregations, nil
	}

	stepRows, err := tx.Query(ctx, `
		SELECT
			agg.id::text,
			st.id::text,
			st.sequence,
			st.depends_on::text,
			st.is_required,
			COALESCE(st.request_template, 'null'::jsonb),
			COALESCE(st.response_mapping, 'null'::jsonb),
			st.service_id::text
		FROM aggregation_configs agg
		JOIN aggregation_steps st ON st.aggregation_id = agg.id
		JOIN services s ON s.id = st.service_id
		WHERE agg.is_active = TRUE
		  AND agg.deleted_at IS NULL
		  AND st.is_active = TRUE
		  AND st.deleted_at IS NULL
		  AND s.is_active = TRUE
		  AND s.deleted_at IS NULL
		  AND s.protocol = 'http'
		ORDER BY agg.id, st.sequence ASC
	`)
	if err != nil {
		return nil, err
	}
	defer stepRows.Close()

	for stepRows.Next() {
		var aggregationID string
		var step AggregationStepValue
		var requestTemplate []byte
		var responseMapping []byte

		err := stepRows.Scan(
			&aggregationID,
			&step.ID,
			&step.Sequence,
			&step.DependsOn,
			&step.IsRequired,
			&requestTemplate,
			&responseMapping,
			&step.ServiceID,
		)
		if err != nil {
			return nil, err
		}
		step.RequestTemplate = append(json.RawMessage(nil), requestTemplate...)
		step.ResponseMapping = append(json.RawMessage(nil), responseMapping...)
		if index, ok := byID[aggregationID]; ok {
			aggregations[index].Steps = append(aggregations[index].Steps, step)
		}
	}

	return aggregations, stepRows.Err()
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
		  AND rp.deleted_at IS NULL
		  AND gp.is_active = TRUE
		  AND gp.deleted_at IS NULL
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
