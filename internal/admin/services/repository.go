package services

import (
	"context"
	"errors"

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

func (r *Repository) Create(ctx context.Context, service *Service) error {
	query := `
		INSERT INTO services (
			id, name, description, protocol, lb_strategy, timeout_ms,
			retry_count, circuit_breaker_enabled, is_active
		)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		RETURNING created_at, updated_at
	`

	return r.db.QueryRow(
		ctx,
		query,
		service.ID,
		service.Name,
		service.Description,
		service.Protocol,
		service.LBStrategy,
		service.TimeoutMS,
		service.RetryCount,
		service.CircuitBreakerEnabled,
		service.IsActive,
	).Scan(&service.CreatedAt, &service.UpdatedAt)
}

func (r *Repository) FindAll(ctx context.Context, p pagination.Pagination) ([]Service, error) {
	query := `
		SELECT id, name, description, protocol, lb_strategy, timeout_ms,
		       retry_count, circuit_breaker_enabled, is_active, created_at, updated_at
		FROM services
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
	err := r.db.QueryRow(ctx, `SELECT count(*) FROM services`).Scan(&total)
	return total, err
}

func (r *Repository) FindByID(ctx context.Context, id uuid.UUID) (*Service, error) {
	query := `
		SELECT id, name, description, protocol, lb_strategy, timeout_ms,
		       retry_count, circuit_breaker_enabled, is_active, created_at, updated_at
		FROM services
		WHERE id = $1
	`

	var service Service

	err := r.db.QueryRow(ctx, query, id).Scan(
		&service.ID,
		&service.Name,
		&service.Description,
		&service.Protocol,
		&service.LBStrategy,
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
		    timeout_ms = $6,
		    retry_count = $7,
		    circuit_breaker_enabled = $8,
		    is_active = $9
		WHERE id = $1
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
	result, err := r.db.Exec(ctx, `DELETE FROM services WHERE id = $1`, id)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return ErrServiceNotFound
	}

	return nil
}
