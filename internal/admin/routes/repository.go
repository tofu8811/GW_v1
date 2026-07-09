package routes

import (
	"context"
	"database/sql"
	"errors"

	"gateway-api/helper/pagination"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrRouteNotFound = errors.New("route not found")

var (
	ErrRequiredScopeUnavailable     = errors.New("required_scope_id does not exist or is inactive")
	ErrRequiredScopeServiceMismatch = errors.New("required_scope_id must belong to the same service as the route")
	ErrCORSPolicyUnavailable        = errors.New("cors_policy_id does not exist or is inactive")
)

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Create(ctx context.Context, route *Route) error {
	if err := r.validateRequiredScope(ctx, route.ServiceID, route.RequiredScopeID); err != nil {
		return err
	}
	if err := r.validateCORSPolicy(ctx, route.CORSPolicyID); err != nil {
		return err
	}
	return r.db.QueryRow(ctx, `
		INSERT INTO routes (
			id, path, method, service_id, strip_prefix, rewrite_target,
			auth_required, required_scope_id, rate_limit_id, cors_policy_id, priority, is_active
		)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
		RETURNING created_at, updated_at
	`, route.ID, route.Path, route.Method, route.ServiceID, route.StripPrefix, route.RewriteTarget, route.AuthRequired, route.RequiredScopeID, route.RateLimitID, route.CORSPolicyID, route.Priority, route.IsActive).Scan(&route.CreatedAt, &route.UpdatedAt)
}

func (r *Repository) FindAll(ctx context.Context, p pagination.Pagination) ([]Route, error) {
	rows, err := r.db.Query(ctx, routeSelectSQL()+`
		WHERE r.deleted_at IS NULL
		ORDER BY r.priority DESC, r.created_at DESC
		LIMIT $1 OFFSET $2
	`, p.Limit, p.Offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	routes := []Route{}
	for rows.Next() {
		route, err := scanRoute(rows)
		if err != nil {
			return nil, err
		}
		routes = append(routes, route)
	}
	return routes, rows.Err()
}

func (r *Repository) Count(ctx context.Context) (int64, error) {
	var total int64
	err := r.db.QueryRow(ctx, `SELECT count(*) FROM routes WHERE deleted_at IS NULL`).Scan(&total)
	return total, err
}

func (r *Repository) FindByID(ctx context.Context, id uuid.UUID) (*Route, error) {
	row := r.db.QueryRow(ctx, routeSelectSQL()+`WHERE r.id = $1 AND r.deleted_at IS NULL`, id)
	route, err := scanRoute(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrRouteNotFound
	}
	if err != nil {
		return nil, err
	}
	return &route, nil
}

func (r *Repository) Update(ctx context.Context, route *Route) error {
	if err := r.validateRequiredScope(ctx, route.ServiceID, route.RequiredScopeID); err != nil {
		return err
	}
	if err := r.validateCORSPolicy(ctx, route.CORSPolicyID); err != nil {
		return err
	}
	err := r.db.QueryRow(ctx, `
		UPDATE routes
		SET path = $2,
		    method = $3,
		    service_id = $4,
		    strip_prefix = $5,
		    rewrite_target = $6,
		    auth_required = $7,
		    required_scope_id = $8,
		    rate_limit_id = $9,
		    cors_policy_id = $10,
		    priority = $11,
		    is_active = $12
		WHERE id = $1 AND deleted_at IS NULL
		RETURNING updated_at
	`, route.ID, route.Path, route.Method, route.ServiceID, route.StripPrefix, route.RewriteTarget, route.AuthRequired, route.RequiredScopeID, route.RateLimitID, route.CORSPolicyID, route.Priority, route.IsActive).Scan(&route.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrRouteNotFound
	}
	return err
}

func (r *Repository) Delete(ctx context.Context, id uuid.UUID) error {
	result, err := r.db.Exec(ctx, `UPDATE routes SET is_active = FALSE, deleted_at = now() WHERE id = $1 AND deleted_at IS NULL`, id)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return ErrRouteNotFound
	}
	return nil
}

func (r *Repository) validateRequiredScope(ctx context.Context, serviceID uuid.UUID, requiredScopeID *uuid.UUID) error {
	if requiredScopeID == nil {
		return nil
	}
	var scopeServiceID uuid.UUID
	err := r.db.QueryRow(ctx, `
		SELECT service_id
		FROM api_scopes
		WHERE id = $1 AND is_active AND deleted_at IS NULL
	`, *requiredScopeID).Scan(&scopeServiceID)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrRequiredScopeUnavailable
	}
	if err != nil {
		return err
	}
	if scopeServiceID != serviceID {
		return ErrRequiredScopeServiceMismatch
	}
	return nil
}

func (r *Repository) validateCORSPolicy(ctx context.Context, corsPolicyID *uuid.UUID) error {
	if corsPolicyID == nil {
		return nil
	}
	var exists bool
	err := r.db.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM cors_policies WHERE id = $1 AND is_active = TRUE AND deleted_at IS NULL)`, *corsPolicyID).Scan(&exists)
	if err != nil {
		return err
	}
	if !exists {
		return ErrCORSPolicyUnavailable
	}
	return nil
}

func routeSelectSQL() string {
	return `
		SELECT r.id, r.path, r.method, r.service_id, r.strip_prefix, r.rewrite_target,
		       r.auth_required, r.required_scope_id, r.rate_limit_id, r.cors_policy_id,
		       cp.id::text, COALESCE(cp.name, ''),
		       COALESCE(cp.allowed_origins, '{}'::text[]),
		       COALESCE(cp.allowed_methods, '{}'::text[]),
		       COALESCE(cp.allowed_headers, '{}'::text[]),
		       COALESCE(cp.exposed_headers, '{}'::text[]),
		       COALESCE(cp.allow_credentials, FALSE), COALESCE(cp.max_age, 0),
		       r.priority, r.is_active, r.created_at, r.updated_at
		FROM routes r
		LEFT JOIN cors_policies cp ON cp.id = r.cors_policy_id AND cp.is_active = TRUE AND cp.deleted_at IS NULL
	`
}

type routeScanner interface {
	Scan(dest ...any) error
}

func scanRoute(row routeScanner) (Route, error) {
	var route Route
	var corsPolicyID *string
	var policyID sql.NullString
	var policy CORSPolicySummary
	err := row.Scan(
		&route.ID,
		&route.Path,
		&route.Method,
		&route.ServiceID,
		&route.StripPrefix,
		&route.RewriteTarget,
		&route.AuthRequired,
		&route.RequiredScopeID,
		&route.RateLimitID,
		&corsPolicyID,
		&policyID,
		&policy.Name,
		&policy.AllowedOrigins,
		&policy.AllowedMethods,
		&policy.AllowedHeaders,
		&policy.ExposedHeaders,
		&policy.AllowCredentials,
		&policy.MaxAge,
		&route.Priority,
		&route.IsActive,
		&route.CreatedAt,
		&route.UpdatedAt,
	)
	if err != nil {
		return Route{}, err
	}
	if corsPolicyID != nil {
		parsed, err := uuid.Parse(*corsPolicyID)
		if err != nil {
			return Route{}, err
		}
		route.CORSPolicyID = &parsed
	}
	if policyID.Valid {
		parsed, err := uuid.Parse(policyID.String)
		if err != nil {
			return Route{}, err
		}
		policy.ID = parsed
		route.CORSPolicy = &policy
	}
	return route, nil
}
