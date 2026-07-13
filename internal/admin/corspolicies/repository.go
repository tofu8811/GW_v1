package corspolicies

import (
	"context"
	"errors"

	"gateway-api/helper/pagination"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrCORSPolicyNotFound = errors.New("CORS policy not found")
var ErrCORSPolicyInUse = errors.New("cors policy is still used by active routes or aggregations")

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Create(ctx context.Context, policy *CORSPolicy) error {
	return r.db.QueryRow(ctx, `
		INSERT INTO cors_policies (
			id, name, allowed_origins, allowed_methods, allowed_headers,
			exposed_headers, allow_credentials, max_age, is_active
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		RETURNING created_at, updated_at
	`, policy.ID, policy.Name, policy.AllowedOrigins, policy.AllowedMethods, policy.AllowedHeaders, policy.ExposedHeaders, policy.AllowCredentials, policy.MaxAge, policy.IsActive).Scan(&policy.CreatedAt, &policy.UpdatedAt)
}

func (r *Repository) FindAll(ctx context.Context, p pagination.Pagination) ([]CORSPolicy, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, name, allowed_origins, allowed_methods, allowed_headers,
		       exposed_headers, allow_credentials, max_age, is_active, created_at, updated_at
		FROM cors_policies
		WHERE deleted_at IS NULL
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2
	`, p.Limit, p.Offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []CORSPolicy{}
	for rows.Next() {
		policy, err := scanPolicy(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, policy)
	}
	return items, rows.Err()
}

func (r *Repository) Count(ctx context.Context) (int64, error) {
	var total int64
	err := r.db.QueryRow(ctx, `SELECT count(*) FROM cors_policies WHERE deleted_at IS NULL`).Scan(&total)
	return total, err
}

func (r *Repository) FindByID(ctx context.Context, id uuid.UUID) (*CORSPolicy, error) {
	row := r.db.QueryRow(ctx, `
		SELECT id, name, allowed_origins, allowed_methods, allowed_headers,
		       exposed_headers, allow_credentials, max_age, is_active, created_at, updated_at
		FROM cors_policies
		WHERE id = $1 AND deleted_at IS NULL
	`, id)
	policy, err := scanPolicy(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrCORSPolicyNotFound
	}
	if err != nil {
		return nil, err
	}
	return &policy, nil
}

func (r *Repository) Update(ctx context.Context, policy *CORSPolicy) error {
	err := r.db.QueryRow(ctx, `
		UPDATE cors_policies
		SET name = $2,
		    allowed_origins = $3,
		    allowed_methods = $4,
		    allowed_headers = $5,
		    exposed_headers = $6,
		    allow_credentials = $7,
		    max_age = $8,
		    is_active = $9
		WHERE id = $1 AND deleted_at IS NULL
		RETURNING updated_at
	`, policy.ID, policy.Name, policy.AllowedOrigins, policy.AllowedMethods, policy.AllowedHeaders, policy.ExposedHeaders, policy.AllowCredentials, policy.MaxAge, policy.IsActive).Scan(&policy.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrCORSPolicyNotFound
	}
	return err
}

func (r *Repository) Delete(ctx context.Context, id uuid.UUID) error {
	inUse, err := r.isReferencedByActiveConfig(ctx, id)
	if err != nil {
		return err
	}
	if inUse {
		return ErrCORSPolicyInUse
	}
	result, err := r.db.Exec(ctx, `UPDATE cors_policies SET is_active = FALSE, deleted_at = now() WHERE id = $1 AND deleted_at IS NULL`, id)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return ErrCORSPolicyNotFound
	}
	return nil
}

func (r *Repository) isReferencedByActiveConfig(ctx context.Context, id uuid.UUID) (bool, error) {
	var exists bool
	err := r.db.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM routes WHERE cors_policy_id = $1 AND is_active = TRUE AND deleted_at IS NULL
			UNION ALL
			SELECT 1 FROM aggregation_configs WHERE cors_policy_id = $1 AND is_active = TRUE AND deleted_at IS NULL
		)
	`, id).Scan(&exists)
	return exists, err
}

type policyScanner interface {
	Scan(dest ...any) error
}

func scanPolicy(row policyScanner) (CORSPolicy, error) {
	var policy CORSPolicy
	err := row.Scan(&policy.ID, &policy.Name, &policy.AllowedOrigins, &policy.AllowedMethods, &policy.AllowedHeaders, &policy.ExposedHeaders, &policy.AllowCredentials, &policy.MaxAge, &policy.IsActive, &policy.CreatedAt, &policy.UpdatedAt)
	return policy, err
}
