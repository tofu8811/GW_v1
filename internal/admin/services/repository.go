package services

import (
	"context"
	"errors"
	"fmt"

	"gateway-api/helper/pagination"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrServiceNotFound = errors.New("service not found")

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Create(ctx context.Context, service *Service, scopeResource *string) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	query := `
		INSERT INTO services (
			id, name, description, protocol, lb_strategy, health_path,
			timeout_ms, retry_count, circuit_breaker_enabled, is_active
		)
		VALUES ($1,$2,$3,$4,$5,NULLIF($6, ''),$7,$8,$9,$10)
		RETURNING created_at, updated_at
	`

	err = tx.QueryRow(
		ctx,
		query,
		service.ID,
		service.Name,
		service.Description,
		service.Protocol,
		service.LBStrategy,
		service.HealthPath,
		service.TimeoutMS,
		service.RetryCount,
		service.CircuitBreakerEnabled,
		service.IsActive,
	).Scan(&service.CreatedAt, &service.UpdatedAt)
	if err != nil {
		return err
	}
	if scopeResource != nil {
		if err := createDefaultAPIScopes(ctx, tx, service.ID, *scopeResource); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (r *Repository) FindAll(ctx context.Context, p pagination.Pagination) ([]Service, error) {
	query := `
		SELECT id, name, description, protocol, lb_strategy, COALESCE(health_path, ''), timeout_ms,
		       retry_count, circuit_breaker_enabled, is_active, created_at, updated_at
		FROM services
		WHERE deleted_at IS NULL
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2
	`

	rows, err := r.db.Query(ctx, query, p.Limit, p.Offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var services []Service

	for rows.Next() {
		var service Service

		err := rows.Scan(
			&service.ID,
			&service.Name,
			&service.Description,
			&service.Protocol,
			&service.LBStrategy,
			&service.HealthPath,
			&service.TimeoutMS,
			&service.RetryCount,
			&service.CircuitBreakerEnabled,
			&service.IsActive,
			&service.CreatedAt,
			&service.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}

		services = append(services, service)
	}

	return services, rows.Err()
}

func (r *Repository) Count(ctx context.Context) (int64, error) {
	var total int64
	err := r.db.QueryRow(ctx, `SELECT count(*) FROM services WHERE deleted_at IS NULL`).Scan(&total)
	return total, err
}

func (r *Repository) FindByID(ctx context.Context, id uuid.UUID) (*Service, error) {
	query := `
		SELECT id, name, description, protocol, lb_strategy, COALESCE(health_path, ''), timeout_ms,
		       retry_count, circuit_breaker_enabled, is_active, created_at, updated_at
		FROM services
		WHERE id = $1 AND deleted_at IS NULL
	`

	var service Service

	err := r.db.QueryRow(ctx, query, id).Scan(
		&service.ID,
		&service.Name,
		&service.Description,
		&service.Protocol,
		&service.LBStrategy,
		&service.HealthPath,
		&service.TimeoutMS,
		&service.RetryCount,
		&service.CircuitBreakerEnabled,
		&service.IsActive,
		&service.CreatedAt,
		&service.UpdatedAt,
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrServiceNotFound
	}

	if err != nil {
		return nil, err
	}

	return &service, nil
}

func (r *Repository) Update(ctx context.Context, service *Service) error {
	query := `
		UPDATE services
		SET name = $2,
		    description = $3,
		    protocol = $4,
		    lb_strategy = $5,
		    health_path = NULLIF($6, ''),
		    timeout_ms = $7,
		    retry_count = $8,
		    circuit_breaker_enabled = $9,
		    is_active = $10
		WHERE id = $1 AND deleted_at IS NULL
		RETURNING updated_at
	`

	err := r.db.QueryRow(
		ctx,
		query,
		service.ID,
		service.Name,
		service.Description,
		service.Protocol,
		service.LBStrategy,
		service.HealthPath,
		service.TimeoutMS,
		service.RetryCount,
		service.CircuitBreakerEnabled,
		service.IsActive,
	).Scan(&service.UpdatedAt)

	if errors.Is(err, pgx.ErrNoRows) {
		return ErrServiceNotFound
	}

	return err
}

func (r *Repository) Delete(ctx context.Context, id uuid.UUID) error {
	result, err := r.db.Exec(ctx, `UPDATE services SET is_active = FALSE, deleted_at = now() WHERE id = $1 AND deleted_at IS NULL`, id)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return ErrServiceNotFound
	}

	return nil
}

func createDefaultAPIScopes(ctx context.Context, tx pgx.Tx, serviceID uuid.UUID, resource string) error {
	for _, action := range []string{"read", "write"} {
		code := fmt.Sprintf("%s:%s", resource, action)
		description := fmt.Sprintf("%s access for %s", action, resource)
		if _, err := tx.Exec(ctx, `
			INSERT INTO api_scopes (service_id, code, resource, action, description)
			SELECT $1, $2, $3, $4, $5
			WHERE NOT EXISTS (
				SELECT 1
				FROM api_scopes
				WHERE service_id = $1
				  AND resource = $3
				  AND action = $4
				  AND deleted_at IS NULL
			)
		`, serviceID, code, resource, action, description); err != nil {
			return err
		}
	}
	return nil
}
