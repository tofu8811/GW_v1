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
	err := r.db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM routes WHERE id = $1)`, routeID).Scan(&exists)
	return exists, err
}

func (r *Repository) FindByRouteID(ctx context.Context, routeID uuid.UUID) (*CORSConfig, error) {
	var config CORSConfig
	err := r.db.QueryRow(ctx, `
		SELECT id, route_id, allowed_origins, allowed_methods, allowed_headers,
		       allow_credentials, max_age
		FROM cors_configs
		WHERE route_id = $1
	`, routeID).Scan(
		&config.ID,
		&config.RouteID,
		&config.AllowedOrigins,
		&config.AllowedMethods,
		&config.AllowedHeaders,
		&config.AllowCredentials,
		&config.MaxAge,
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
			allow_credentials, max_age
		)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
		ON CONFLICT (route_id) DO UPDATE SET
			allowed_origins = EXCLUDED.allowed_origins,
			allowed_methods = EXCLUDED.allowed_methods,
			allowed_headers = EXCLUDED.allowed_headers,
			allow_credentials = EXCLUDED.allow_credentials,
			max_age = EXCLUDED.max_age
		RETURNING id
	`, config.ID, config.RouteID, config.AllowedOrigins, config.AllowedMethods,
		config.AllowedHeaders, config.AllowCredentials, config.MaxAge,
	).Scan(&config.ID)
}

func (r *Repository) DeleteByRouteID(ctx context.Context, routeID uuid.UUID) error {
	result, err := r.db.Exec(ctx, `DELETE FROM cors_configs WHERE route_id = $1`, routeID)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return ErrCORSConfigNotFound
	}
	return nil
}
