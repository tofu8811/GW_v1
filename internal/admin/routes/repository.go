package routes

import (
	"context"
	"errors"

	"gateway-api/helper/pagination"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrRouteNotFound = errors.New("route not found")

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Create(ctx context.Context, route *Route) error {
	query := `
		INSERT INTO routes (
			id, path, method, service_id, strip_prefix, rewrite_target,
			auth_required, rate_limit_id, priority, is_active
		)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
		RETURNING created_at, updated_at
	`

	return r.db.QueryRow(
		ctx,
		query,
		route.ID,
		route.Path,
		route.Method,
		route.ServiceID,
		route.StripPrefix,
		route.RewriteTarget,
		route.AuthRequired,
		route.RateLimitID,
		route.Priority,
		route.IsActive,
	).Scan(&route.CreatedAt, &route.UpdatedAt)
}

func (r *Repository) FindAll(ctx context.Context, p pagination.Pagination) ([]Route, error) {
	query := `
		SELECT id, path, method, service_id, strip_prefix, rewrite_target,
		       auth_required, rate_limit_id, priority, is_active, created_at, updated_at
		FROM routes
		ORDER BY priority DESC, created_at DESC
		LIMIT $1 OFFSET $2
	`

	rows, err := r.db.Query(ctx, query, p.Limit, p.Offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var routes []Route

	for rows.Next() {
		var route Route

		err := rows.Scan(
			&route.ID,
			&route.Path,
			&route.Method,
			&route.ServiceID,
			&route.StripPrefix,
			&route.RewriteTarget,
			&route.AuthRequired,
			&route.RateLimitID,
			&route.Priority,
			&route.IsActive,
			&route.CreatedAt,
			&route.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}

		routes = append(routes, route)
	}

	return routes, rows.Err()
}

func (r *Repository) Count(ctx context.Context) (int64, error) {
	var total int64
	err := r.db.QueryRow(ctx, `SELECT count(*) FROM routes`).Scan(&total)
	return total, err
}

func (r *Repository) FindByID(ctx context.Context, id uuid.UUID) (*Route, error) {
	query := `
		SELECT id, path, method, service_id, strip_prefix, rewrite_target,
		       auth_required, rate_limit_id, priority, is_active, created_at, updated_at
		FROM routes
		WHERE id = $1
	`

	var route Route

	err := r.db.QueryRow(ctx, query, id).Scan(
		&route.ID,
		&route.Path,
		&route.Method,
		&route.ServiceID,
		&route.StripPrefix,
		&route.RewriteTarget,
		&route.AuthRequired,
		&route.RateLimitID,
		&route.Priority,
		&route.IsActive,
		&route.CreatedAt,
		&route.UpdatedAt,
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrRouteNotFound
	}

	if err != nil {
		return nil, err
	}

	return &route, nil
}

func (r *Repository) Update(ctx context.Context, route *Route) error {
	query := `
		UPDATE routes
		SET path = $2,
		    method = $3,
		    service_id = $4,
		    strip_prefix = $5,
		    rewrite_target = $6,
		    auth_required = $7,
		    rate_limit_id = $8,
		    priority = $9,
		    is_active = $10
		WHERE id = $1
		RETURNING updated_at
	`

	err := r.db.QueryRow(
		ctx,
		query,
		route.ID,
		route.Path,
		route.Method,
		route.ServiceID,
		route.StripPrefix,
		route.RewriteTarget,
		route.AuthRequired,
		route.RateLimitID,
		route.Priority,
		route.IsActive,
	).Scan(&route.UpdatedAt)

	if errors.Is(err, pgx.ErrNoRows) {
		return ErrRouteNotFound
	}

	return err
}

func (r *Repository) Delete(ctx context.Context, id uuid.UUID) error {
	result, err := r.db.Exec(ctx, `DELETE FROM routes WHERE id = $1`, id)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return ErrRouteNotFound
	}

	return nil
}
