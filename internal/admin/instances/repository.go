package instances

import (
	"context"
	"errors"

	"gateway-api/helper/pagination"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrInstanceNotFound = errors.New("service instance not found")

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Create(ctx context.Context, instance *ServiceInstance) error {
	query := `
		INSERT INTO service_instances (id, service_id, host, port, weight, is_active)
		VALUES ($1,$2,$3,$4,$5,$6)
		RETURNING created_at
	`

	return r.db.QueryRow(
		ctx,
		query,
		instance.ID,
		instance.ServiceID,
		instance.Host,
		instance.Port,
		instance.Weight,
		instance.IsActive,
	).Scan(&instance.CreatedAt)
}

func (r *Repository) FindAll(ctx context.Context, p pagination.Pagination) ([]ServiceInstance, error) {
	query := `
		SELECT id, service_id, host, port, weight, is_active, created_at
		FROM service_instances
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2
	`

	rows, err := r.db.Query(ctx, query, p.Limit, p.Offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanInstances(rows)
}

func (r *Repository) Count(ctx context.Context) (int64, error) {
	var total int64
	err := r.db.QueryRow(ctx, `SELECT count(*) FROM service_instances`).Scan(&total)
	return total, err
}

func (r *Repository) FindByServiceID(ctx context.Context, serviceID uuid.UUID, p pagination.Pagination) ([]ServiceInstance, error) {
	query := `
		SELECT id, service_id, host, port, weight, is_active, created_at
		FROM service_instances
		WHERE service_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`

	rows, err := r.db.Query(ctx, query, serviceID, p.Limit, p.Offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanInstances(rows)
}

func (r *Repository) CountByServiceID(ctx context.Context, serviceID uuid.UUID) (int64, error) {
	var total int64
	err := r.db.QueryRow(ctx, `SELECT count(*) FROM service_instances WHERE service_id = $1`, serviceID).Scan(&total)
	return total, err
}

func (r *Repository) FindByID(ctx context.Context, id uuid.UUID) (*ServiceInstance, error) {
	query := `
		SELECT id, service_id, host, port, weight, is_active, created_at
		FROM service_instances
		WHERE id = $1
	`

	var instance ServiceInstance

	err := r.db.QueryRow(ctx, query, id).Scan(
		&instance.ID,
		&instance.ServiceID,
		&instance.Host,
		&instance.Port,
		&instance.Weight,
		&instance.IsActive,
		&instance.CreatedAt,
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrInstanceNotFound
	}

	if err != nil {
		return nil, err
	}

	return &instance, nil
}

func (r *Repository) Update(ctx context.Context, instance *ServiceInstance) error {
	query := `
		UPDATE service_instances
		SET service_id = $2,
		    host = $3,
		    port = $4,
		    weight = $5,
		    is_active = $6
		WHERE id = $1
	`

	result, err := r.db.Exec(
		ctx,
		query,
		instance.ID,
		instance.ServiceID,
		instance.Host,
		instance.Port,
		instance.Weight,
		instance.IsActive,
	)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return ErrInstanceNotFound
	}

	return nil
}

func (r *Repository) Delete(ctx context.Context, id uuid.UUID) error {
	result, err := r.db.Exec(ctx, `DELETE FROM service_instances WHERE id = $1`, id)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return ErrInstanceNotFound
	}

	return nil
}

func scanInstances(rows pgx.Rows) ([]ServiceInstance, error) {
	var instances []ServiceInstance

	for rows.Next() {
		var instance ServiceInstance

		err := rows.Scan(
			&instance.ID,
			&instance.ServiceID,
			&instance.Host,
			&instance.Port,
			&instance.Weight,
			&instance.IsActive,
			&instance.CreatedAt,
		)
		if err != nil {
			return nil, err
		}

		instances = append(instances, instance)
	}

	return instances, rows.Err()
}
