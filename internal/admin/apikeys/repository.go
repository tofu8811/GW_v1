package apikeys

import (
	"context"
	"errors"

	"gateway-api/helper/pagination"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrAPIKeyNotFound = errors.New("API key not found")

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Create(ctx context.Context, apiKey *APIKey) error {
	return r.db.QueryRow(ctx, `
		INSERT INTO api_keys (
			id, key_hash, key_prefix, label, user_id, scopes,
			rate_limit_id, expires_at, is_active
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		RETURNING created_at, updated_at
	`, apiKey.ID, apiKey.KeyHash, apiKey.KeyPrefix, apiKey.Label, apiKey.UserID,
		apiKey.Scopes, apiKey.RateLimitID, apiKey.ExpiresAt, apiKey.IsActive,
	).Scan(&apiKey.CreatedAt, &apiKey.UpdatedAt)
}

func (r *Repository) FindAll(ctx context.Context, p pagination.Pagination) ([]APIKey, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, key_prefix, label, user_id, scopes, rate_limit_id,
		       expires_at, is_active, last_used_at, created_at, updated_at
		FROM api_keys
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2
	`, p.Limit, p.Offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	keys := make([]APIKey, 0)
	for rows.Next() {
		var key APIKey
		if err := rows.Scan(&key.ID, &key.KeyPrefix, &key.Label, &key.UserID, &key.Scopes,
			&key.RateLimitID, &key.ExpiresAt, &key.IsActive, &key.LastUsedAt,
			&key.CreatedAt, &key.UpdatedAt); err != nil {
			return nil, err
		}
		keys = append(keys, key)
	}
	return keys, rows.Err()
}

func (r *Repository) Count(ctx context.Context) (int64, error) {
	var total int64
	err := r.db.QueryRow(ctx, `SELECT count(*) FROM api_keys`).Scan(&total)
	return total, err
}

func (r *Repository) FindByID(ctx context.Context, id uuid.UUID) (*APIKey, error) {
	var key APIKey
	err := r.db.QueryRow(ctx, `
		SELECT id, key_prefix, label, user_id, scopes, rate_limit_id,
		       expires_at, is_active, last_used_at, created_at, updated_at
		FROM api_keys WHERE id = $1
	`, id).Scan(&key.ID, &key.KeyPrefix, &key.Label, &key.UserID, &key.Scopes,
		&key.RateLimitID, &key.ExpiresAt, &key.IsActive, &key.LastUsedAt,
		&key.CreatedAt, &key.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrAPIKeyNotFound
	}
	if err != nil {
		return nil, err
	}
	return &key, nil
}

func (r *Repository) Update(ctx context.Context, apiKey *APIKey) error {
	err := r.db.QueryRow(ctx, `
		UPDATE api_keys SET
			label = $2, user_id = $3, scopes = $4, rate_limit_id = $5,
			expires_at = $6, is_active = $7
		WHERE id = $1
		RETURNING updated_at
	`, apiKey.ID, apiKey.Label, apiKey.UserID, apiKey.Scopes, apiKey.RateLimitID,
		apiKey.ExpiresAt, apiKey.IsActive).Scan(&apiKey.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrAPIKeyNotFound
	}
	return err
}

func (r *Repository) Revoke(ctx context.Context, id uuid.UUID) (*APIKey, error) {
	var key APIKey
	err := r.db.QueryRow(ctx, `
		UPDATE api_keys
		SET is_active = FALSE
		WHERE id = $1
		RETURNING id, key_prefix, label, user_id, scopes, rate_limit_id,
		          expires_at, is_active, last_used_at, created_at, updated_at
	`, id).Scan(&key.ID, &key.KeyPrefix, &key.Label, &key.UserID, &key.Scopes,
		&key.RateLimitID, &key.ExpiresAt, &key.IsActive, &key.LastUsedAt,
		&key.CreatedAt, &key.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrAPIKeyNotFound
	}
	if err != nil {
		return nil, err
	}
	return &key, nil
}

func (r *Repository) Rotate(ctx context.Context, id uuid.UUID, keyHash string, keyPrefix string) (*APIKey, error) {
	var key APIKey
	err := r.db.QueryRow(ctx, `
		UPDATE api_keys
		SET key_hash = $2, key_prefix = $3, is_active = TRUE, last_used_at = NULL
		WHERE id = $1
		RETURNING id, key_prefix, label, user_id, scopes, rate_limit_id,
		          expires_at, is_active, last_used_at, created_at, updated_at
	`, id, keyHash, keyPrefix).Scan(&key.ID, &key.KeyPrefix, &key.Label, &key.UserID,
		&key.Scopes, &key.RateLimitID, &key.ExpiresAt, &key.IsActive,
		&key.LastUsedAt, &key.CreatedAt, &key.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrAPIKeyNotFound
	}
	if err != nil {
		return nil, err
	}
	return &key, nil
}
