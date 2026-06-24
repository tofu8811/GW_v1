package proxy

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrRouteNotFound     = errors.New("route not found")
	ErrServiceNotFound   = errors.New("service not found")
	ErrInstancesNotFound = errors.New("active service instances not found")
)

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

func (r *Repository) FindRoute(ctx context.Context, method string, path string) (*RouteConfig, error) {
	query := `
		SELECT id, path, method, service_id, strip_prefix, rewrite_target,
		       auth_required, rate_limit_id, priority
		FROM routes
		WHERE is_active = TRUE
		  AND method IN ($1, 'ANY')
		  AND ($2 = path OR $2 LIKE path || '/%')
		ORDER BY
		  CASE WHEN method = $1 THEN 0 ELSE 1 END,
		  length(path) DESC,
		  priority DESC
		LIMIT 1
	`

	var route RouteConfig
	err := r.db.QueryRow(ctx, query, method, path).Scan(
		&route.ID,
		&route.Path,
		&route.Method,
		&route.ServiceID,
		&route.StripPrefix,
		&route.RewriteTarget,
		&route.AuthRequired,
		&route.RateLimitID,
		&route.Priority,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrRouteNotFound
	}
	if err != nil {
		return nil, err
	}

	return &route, nil
}

func (r *Repository) FindService(ctx context.Context, id uuid.UUID) (*ServiceConfig, error) {
	query := `
		SELECT id, name, protocol, lb_strategy, timeout_ms,
		       retry_count, circuit_breaker_enabled
		FROM services
		WHERE id = $1
		  AND is_active = TRUE
	`

	var service ServiceConfig
	err := r.db.QueryRow(ctx, query, id).Scan(
		&service.ID,
		&service.Name,
		&service.Protocol,
		&service.LBStrategy,
		&service.TimeoutMS,
		&service.RetryCount,
		&service.CircuitBreakerEnabled,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrServiceNotFound
	}
	if err != nil {
		return nil, err
	}

	return &service, nil
}

func (r *Repository) FindActiveInstances(ctx context.Context, serviceID uuid.UUID) ([]ServiceInstance, error) {
	query := `
		SELECT id, service_id, host, port, weight
		FROM service_instances
		WHERE service_id = $1
		  AND is_active = TRUE
		ORDER BY created_at ASC
	`

	rows, err := r.db.Query(ctx, query, serviceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var instances []ServiceInstance
	for rows.Next() {
		var instance ServiceInstance
		err := rows.Scan(
			&instance.ID,
			&instance.ServiceID,
			&instance.Host,
			&instance.Port,
			&instance.Weight,
		)
		if err != nil {
			return nil, err
		}

		instances = append(instances, instance)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	if len(instances) == 0 {
		return nil, ErrInstancesNotFound
	}

	return instances, nil
}
