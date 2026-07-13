package corsconfigs

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrCORSConfigNotFound = errors.New("CORS config not found")

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

func (r *Repository) RouteExists(ctx context.Context, routeID uuid.UUID) (bool, error) {
	var exists bool
	err := r.db.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM routes WHERE id = $1 AND deleted_at IS NULL
		)
	`, routeID).Scan(&exists)
	return exists, err
}

func (r *Repository) FindByRouteID(ctx context.Context, routeID uuid.UUID) (*CORSConfig, error) {
	var config CORSConfig
	err := r.db.QueryRow(ctx, `
		SELECT id, route_id, allowed_origins, allowed_methods, allowed_headers,
		       allow_credentials, max_age, is_active, created_at, updated_at, deleted_at
		FROM cors_configs
		WHERE route_id = $1 AND deleted_at IS NULL
	`, routeID).Scan(
		&config.ID,
		&config.RouteID,
		&config.AllowedOrigins,
		&config.AllowedMethods,
		&config.AllowedHeaders,
		&config.AllowCredentials,
		&config.MaxAge,
		&config.IsActive,
		&config.CreatedAt,
		&config.UpdatedAt,
		&config.DeletedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrCORSConfigNotFound
	}
	if err != nil {
		return nil, err
	}
	return &config, nil
}

func (r *Repository) Upsert(ctx context.Context, config *CORSConfig) error {
	return r.db.QueryRow(ctx, `
		INSERT INTO cors_configs (
			id, route_id, allowed_origins, allowed_methods, allowed_headers,
			allow_credentials, max_age, is_active
		)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		ON CONFLICT (route_id) WHERE deleted_at IS NULL DO UPDATE SET
			allowed_origins = EXCLUDED.allowed_origins,
			allowed_methods = EXCLUDED.allowed_methods,
			allowed_headers = EXCLUDED.allowed_headers,
			allow_credentials = EXCLUDED.allow_credentials,
			max_age = EXCLUDED.max_age,
			is_active = EXCLUDED.is_active
		RETURNING id, created_at, updated_at
	`, config.ID, config.RouteID, config.AllowedOrigins, config.AllowedMethods,
		config.AllowedHeaders, config.AllowCredentials, config.MaxAge, config.IsActive,
	).Scan(&config.ID, &config.CreatedAt, &config.UpdatedAt)
}

func (r *Repository) DeleteByRouteID(ctx context.Context, routeID uuid.UUID) error {
	result, err := r.db.Exec(ctx, `
		UPDATE cors_configs
		SET is_active = FALSE, deleted_at = now()
		WHERE route_id = $1 AND deleted_at IS NULL
	`, routeID)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return ErrCORSConfigNotFound
	}
	return nil
}
