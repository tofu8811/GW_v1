package ratelimits

import (
	"context"
	"errors"

	"gateway-api/helper/pagination"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrRateLimitPolicyNotFound = errors.New("rate limit policy not found")

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Create(ctx context.Context, policy *RateLimitPolicy) error {
	query := `
		INSERT INTO rate_limit_policies (
			id, name, limit_type, max_requests, window_seconds, is_active
		)
		VALUES ($1,$2,$3,$4,$5,$6)
		RETURNING created_at, updated_at
	`

	return r.db.QueryRow(
		ctx,
		query,
		policy.ID,
		policy.Name,
		policy.LimitType,
		policy.MaxRequests,
		policy.WindowSeconds,
		policy.IsActive,
	).Scan(&policy.CreatedAt, &policy.UpdatedAt)
}

func (r *Repository) FindAll(ctx context.Context, p pagination.Pagination) ([]RateLimitPolicy, error) {
	query := `
		SELECT id, name, limit_type, max_requests, window_seconds, is_active, created_at, updated_at
		FROM rate_limit_policies
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2
	`

	rows, err := r.db.Query(ctx, query, p.Limit, p.Offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var policies []RateLimitPolicy
	for rows.Next() {
		var policy RateLimitPolicy
		err := rows.Scan(
			&policy.ID,
			&policy.Name,
			&policy.LimitType,
			&policy.MaxRequests,
			&policy.WindowSeconds,
			&policy.IsActive,
			&policy.CreatedAt,
			&policy.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		policies = append(policies, policy)
	}

	return policies, rows.Err()
}

func (r *Repository) Count(ctx context.Context) (int64, error) {
	var total int64
	err := r.db.QueryRow(ctx, `SELECT count(*) FROM rate_limit_policies`).Scan(&total)
	return total, err
}

func (r *Repository) FindByID(ctx context.Context, id uuid.UUID) (*RateLimitPolicy, error) {
	query := `
		SELECT id, name, limit_type, max_requests, window_seconds, is_active, created_at, updated_at
		FROM rate_limit_policies
		WHERE id = $1
	`

	var policy RateLimitPolicy
	err := r.db.QueryRow(ctx, query, id).Scan(
		&policy.ID,
		&policy.Name,
		&policy.LimitType,
		&policy.MaxRequests,
		&policy.WindowSeconds,
		&policy.IsActive,
		&policy.CreatedAt,
		&policy.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrRateLimitPolicyNotFound
	}
	if err != nil {
		return nil, err
	}

	return &policy, nil
}

func (r *Repository) Update(ctx context.Context, policy *RateLimitPolicy) error {
	query := `
		UPDATE rate_limit_policies
		SET name = $2,
		    limit_type = $3,
		    max_requests = $4,
		    window_seconds = $5,
		    is_active = $6
		WHERE id = $1
		RETURNING updated_at
	`

	err := r.db.QueryRow(
		ctx,
		query,
		policy.ID,
		policy.Name,
		policy.LimitType,
		policy.MaxRequests,
		policy.WindowSeconds,
		policy.IsActive,
	).Scan(&policy.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrRateLimitPolicyNotFound
	}

	return err
}

func (r *Repository) Delete(ctx context.Context, id uuid.UUID) error {
	result, err := r.db.Exec(ctx, `DELETE FROM rate_limit_policies WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return ErrRateLimitPolicyNotFound
	}

	return nil
}
