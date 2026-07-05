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
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	err = tx.QueryRow(ctx, `
		INSERT INTO api_keys (
			id, key_hash, key_prefix, label, client_id,
			rate_limit_id, expires_at, is_active, created_by
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		RETURNING created_at, updated_at
	`, apiKey.ID, apiKey.KeyHash, apiKey.KeyPrefix, apiKey.Label, apiKey.ClientID,
		apiKey.RateLimitID, apiKey.ExpiresAt, apiKey.IsActive, apiKey.CreatedBy,
	).Scan(&apiKey.CreatedAt, &apiKey.UpdatedAt)
	if err != nil {
		return err
	}
	if err := replacePermissions(ctx, tx, apiKey.ID, apiKey.PermissionIDs); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *Repository) FindAll(ctx context.Context, p pagination.Pagination) ([]APIKey, error) {
	rows, err := r.db.Query(ctx, selectAPIKeysQuery(`WHERE ak.deleted_at IS NULL`)+`
		ORDER BY ak.created_at DESC
		LIMIT $1 OFFSET $2
	`, p.Limit, p.Offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	keys := make([]APIKey, 0)
	for rows.Next() {
		key, err := scanAPIKey(rows)
		if err != nil {
			return nil, err
		}
		keys = append(keys, key)
	}
	return keys, rows.Err()
}

func (r *Repository) Count(ctx context.Context) (int64, error) {
	var total int64
	err := r.db.QueryRow(ctx, `SELECT count(*) FROM api_keys WHERE deleted_at IS NULL`).Scan(&total)
	return total, err
}

func (r *Repository) FindByID(ctx context.Context, id uuid.UUID) (*APIKey, error) {
	rows, err := r.db.Query(ctx, selectAPIKeysQuery(`WHERE ak.id = $1 AND ak.deleted_at IS NULL`), id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if !rows.Next() {
		if rows.Err() != nil {
			return nil, rows.Err()
		}
		return nil, ErrAPIKeyNotFound
	}
	key, err := scanAPIKey(rows)
	if err != nil {
		return nil, err
	}
	return &key, rows.Err()
}

func (r *Repository) Update(ctx context.Context, apiKey *APIKey) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	err = tx.QueryRow(ctx, `
		UPDATE api_keys SET
			label = $2, client_id = $3, rate_limit_id = $4,
			expires_at = $5, is_active = $6
		WHERE id = $1 AND deleted_at IS NULL
		RETURNING updated_at
	`, apiKey.ID, apiKey.Label, apiKey.ClientID, apiKey.RateLimitID,
		apiKey.ExpiresAt, apiKey.IsActive).Scan(&apiKey.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrAPIKeyNotFound
	}
	if err != nil {
		return err
	}
	if err := replacePermissions(ctx, tx, apiKey.ID, apiKey.PermissionIDs); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *Repository) Revoke(ctx context.Context, id uuid.UUID) (*APIKey, error) {
	_, err := r.db.Exec(ctx, `
		UPDATE api_keys
		SET is_active = FALSE, revoked_at = COALESCE(revoked_at, now())
		WHERE id = $1 AND deleted_at IS NULL
	`, id)
	if err != nil {
		return nil, err
	}
	return r.FindByID(ctx, id)
}

func (r *Repository) Rotate(ctx context.Context, id uuid.UUID, keyHash string, keyPrefix string) (*APIKey, error) {
	_, err := r.db.Exec(ctx, `
		UPDATE api_keys
		SET key_hash = $2, key_prefix = $3, is_active = TRUE, revoked_at = NULL, last_used_at = NULL
		WHERE id = $1 AND deleted_at IS NULL
	`, id, keyHash, keyPrefix)
	if err != nil {
		return nil, err
	}
	return r.FindByID(ctx, id)
}

type apiKeyScanner interface {
	Scan(dest ...any) error
}

func scanAPIKey(row apiKeyScanner) (APIKey, error) {
	var key APIKey
	err := row.Scan(&key.ID, &key.KeyPrefix, &key.Label, &key.ClientID, &key.PermissionIDs,
		&key.RateLimitID, &key.ExpiresAt, &key.IsActive, &key.RevokedAt, &key.LastUsedAt,
		&key.CreatedBy, &key.CreatedAt, &key.UpdatedAt)
	return key, err
}

func selectAPIKeysQuery(whereClause string) string {
	return `
		SELECT ak.id, ak.key_prefix, ak.label, ak.client_id,
		       COALESCE(array_agg(aks.permission_id) FILTER (WHERE aks.permission_id IS NOT NULL), '{}'::uuid[]) AS permission_ids,
		       ak.rate_limit_id, ak.expires_at, ak.is_active, ak.revoked_at, ak.last_used_at,
		       ak.created_by, ak.created_at, ak.updated_at
		FROM api_keys ak
		LEFT JOIN api_key_scopes aks ON aks.api_key_id = ak.id
		` + whereClause + `
		GROUP BY ak.id
	`
}

func replacePermissions(ctx context.Context, tx pgx.Tx, apiKeyID uuid.UUID, permissionIDs []uuid.UUID) error {
	if _, err := tx.Exec(ctx, `DELETE FROM api_key_scopes WHERE api_key_id = $1`, apiKeyID); err != nil {
		return err
	}
	for _, permissionID := range permissionIDs {
		if _, err := tx.Exec(ctx, `
			INSERT INTO api_key_scopes (api_key_id, permission_id)
			VALUES ($1, $2)
			ON CONFLICT DO NOTHING
		`, apiKeyID, permissionID); err != nil {
			return err
		}
	}
	return nil
}
