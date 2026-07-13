package apikeys

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"gateway-api/helper/dberror"
	"gateway-api/helper/pagination"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrAPIKeyNotFound = errors.New("API key not found")
var ErrAPIKeyHashExists = errors.New("API key hash already exists")

var (
	ErrClientUnavailable    = errors.New("client does not exist or is inactive")
	ErrScopeUnavailable     = errors.New("API scope does not exist or is inactive")
	ErrRateLimitUnavailable = errors.New("rate limit policy does not exist or is inactive")
)

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
	if err := validateReferences(ctx, tx, apiKey); err != nil {
		return err
	}
	if exists, err := apiKeyHashExists(ctx, tx, apiKey.KeyHash); err != nil {
		return err
	} else if exists {
		return ErrAPIKeyHashExists
	}
	apiKey.Scopes, err = findScopeDetails(ctx, tx, apiKey.ScopeIDs)
	if err != nil {
		return err
	}

	err = tx.QueryRow(ctx, `
		INSERT INTO api_keys (
			id, key_hash, key_prefix, label, client_id,
			rate_limit_id, expires_at, is_active, created_by
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		RETURNING created_at, updated_at
	`, apiKey.ID, apiKey.KeyHash, apiKey.KeyPrefix, apiKey.Label, apiKey.ClientID,
		apiKey.RateLimitID, apiKey.ExpiresAt, apiKey.IsActive, apiKey.CreatedBy,
	).Scan(&apiKey.CreatedAt, &apiKey.UpdatedAt)
	if isAPIKeyHashConflict(err) {
		return ErrAPIKeyHashExists
	}
	if err != nil {
		return err
	}
	if err := replaceScopes(ctx, tx, apiKey.ID, apiKey.ScopeIDs); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *Repository) FindAll(ctx context.Context, p pagination.Pagination, filters APIKeyListFilters) ([]APIKey, error) {
	whereClause, args := buildAPIKeyListWhere(filters)
	limitPlaceholder := len(args) + 1
	offsetPlaceholder := len(args) + 2
	args = append(args, p.Limit, p.Offset)
	rows, err := r.db.Query(ctx, selectAPIKeysQuery(whereClause)+fmt.Sprintf(`
		ORDER BY ak.created_at DESC
		LIMIT $%d OFFSET $%d
	`, limitPlaceholder, offsetPlaceholder), args...)
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

func (r *Repository) Count(ctx context.Context, filters APIKeyListFilters) (int64, error) {
	var total int64
	whereClause, args := buildAPIKeyListWhere(filters)
	err := r.db.QueryRow(ctx, `SELECT count(*) FROM api_keys ak `+whereClause, args...).Scan(&total)
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
	if err := validateReferences(ctx, tx, apiKey); err != nil {
		return err
	}
	apiKey.Scopes, err = findScopeDetails(ctx, tx, apiKey.ScopeIDs)
	if err != nil {
		return err
	}

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
	if err := replaceScopes(ctx, tx, apiKey.ID, apiKey.ScopeIDs); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *Repository) Revoke(ctx context.Context, id uuid.UUID) (*APIKey, error) {
	result, err := r.db.Exec(ctx, `
		UPDATE api_keys
		SET is_active = FALSE, revoked_at = COALESCE(revoked_at, now())
		WHERE id = $1 AND deleted_at IS NULL
	`, id)
	if err != nil {
		return nil, err
	}
	if result.RowsAffected() == 0 {
		return nil, ErrAPIKeyNotFound
	}
	return r.FindByID(ctx, id)
}

func (r *Repository) Rotate(ctx context.Context, id uuid.UUID, newID uuid.UUID, keyHash string, keyPrefix string, createdBy *uuid.UUID) (*APIKey, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	var oldKey APIKey
	err = tx.QueryRow(ctx, `
		SELECT id, label, client_id, rate_limit_id, expires_at, created_by
		FROM api_keys
		WHERE id = $1 AND deleted_at IS NULL
		FOR UPDATE
	`, id).Scan(&oldKey.ID, &oldKey.Label, &oldKey.ClientID, &oldKey.RateLimitID, &oldKey.ExpiresAt, &oldKey.CreatedBy)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrAPIKeyNotFound
	}
	if err != nil {
		return nil, err
	}
	if createdBy == nil {
		createdBy = oldKey.CreatedBy
	}
	if exists, err := apiKeyHashExists(ctx, tx, keyHash); err != nil {
		return nil, err
	} else if exists {
		return nil, ErrAPIKeyHashExists
	}

	scopeIDs, scopes, err := findAPIKeyScopes(ctx, tx, id)
	if err != nil {
		return nil, err
	}
	result, err := tx.Exec(ctx, `
		UPDATE api_keys
		SET is_active = FALSE, revoked_at = COALESCE(revoked_at, now())
		WHERE id = $1 AND deleted_at IS NULL
	`, id)
	if err != nil {
		return nil, err
	}
	if result.RowsAffected() == 0 {
		return nil, ErrAPIKeyNotFound
	}

	newKey := APIKey{
		ID: newID, KeyHash: keyHash, KeyPrefix: keyPrefix, Label: oldKey.Label,
		ClientID: oldKey.ClientID, ScopeIDs: scopeIDs, Scopes: scopes,
		RateLimitID: oldKey.RateLimitID, ExpiresAt: oldKey.ExpiresAt,
		IsActive: true, CreatedBy: createdBy,
	}

	err = tx.QueryRow(ctx, `
		INSERT INTO api_keys (
			id, key_hash, key_prefix, label, client_id,
			rate_limit_id, expires_at, is_active, created_by
		) VALUES ($1,$2,$3,$4,$5,$6,$7,TRUE,$8)
		RETURNING created_at, updated_at
	`, newKey.ID, newKey.KeyHash, newKey.KeyPrefix, newKey.Label, newKey.ClientID,
		newKey.RateLimitID, newKey.ExpiresAt, newKey.CreatedBy,
	).Scan(&newKey.CreatedAt, &newKey.UpdatedAt)
	if isAPIKeyHashConflict(err) {
		return nil, ErrAPIKeyHashExists
	}
	if err != nil {
		return nil, err
	}
	if err := replaceScopes(ctx, tx, newKey.ID, newKey.ScopeIDs); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &newKey, nil
}

type apiKeyScanner interface {
	Scan(dest ...any) error
}

func scanAPIKey(row apiKeyScanner) (APIKey, error) {
	var key APIKey
	var scopeServiceIDs []uuid.UUID
	var scopeCodes []string
	var scopeResources []string
	var scopeActions []string
	err := row.Scan(&key.ID, &key.KeyPrefix, &key.Label, &key.ClientID, &key.ScopeIDs,
		&scopeServiceIDs, &scopeCodes, &scopeResources, &scopeActions,
		&key.RateLimitID, &key.ExpiresAt, &key.IsActive, &key.RevokedAt, &key.LastUsedAt,
		&key.CreatedBy, &key.CreatedAt, &key.UpdatedAt)
	if err != nil {
		return key, err
	}
	key.Scopes = make([]APIScope, 0, len(key.ScopeIDs))
	for index, scopeID := range key.ScopeIDs {
		scope := APIScope{ID: scopeID}
		if index < len(scopeServiceIDs) {
			scope.ServiceID = scopeServiceIDs[index]
		}
		if index < len(scopeCodes) {
			scope.Code = scopeCodes[index]
		}
		if index < len(scopeResources) {
			scope.Resource = scopeResources[index]
		}
		if index < len(scopeActions) {
			scope.Action = scopeActions[index]
		}
		key.Scopes = append(key.Scopes, scope)
	}
	return key, nil
}

func selectAPIKeysQuery(whereClause string) string {
	return `
		SELECT ak.id, ak.key_prefix, ak.label, ak.client_id,
		       COALESCE(array_agg(asc_.id ORDER BY asc_.resource, asc_.action) FILTER (WHERE asc_.id IS NOT NULL), '{}'::uuid[]) AS scope_ids,
		       COALESCE(array_agg(asc_.service_id ORDER BY asc_.resource, asc_.action) FILTER (WHERE asc_.id IS NOT NULL), '{}'::uuid[]) AS scope_service_ids,
		       COALESCE(array_agg(asc_.code ORDER BY asc_.resource, asc_.action) FILTER (WHERE asc_.id IS NOT NULL), '{}'::text[]) AS scope_codes,
		       COALESCE(array_agg(asc_.resource ORDER BY asc_.resource, asc_.action) FILTER (WHERE asc_.id IS NOT NULL), '{}'::text[]) AS scope_resources,
		       COALESCE(array_agg(asc_.action ORDER BY asc_.resource, asc_.action) FILTER (WHERE asc_.id IS NOT NULL), '{}'::text[]) AS scope_actions,
		       ak.rate_limit_id, ak.expires_at, ak.is_active, ak.revoked_at, ak.last_used_at,
		       ak.created_by, ak.created_at, ak.updated_at
		FROM api_keys ak
		LEFT JOIN api_key_scopes aks ON aks.api_key_id = ak.id
		LEFT JOIN api_scopes asc_ ON asc_.id = aks.scope_id AND asc_.deleted_at IS NULL AND asc_.is_active
		` + whereClause + `
		GROUP BY ak.id
	`
}

func buildAPIKeyListWhere(filters APIKeyListFilters) (string, []any) {
	conditions := make([]string, 0, 5)
	args := make([]any, 0, 2)

	if filters.DeletedOnly {
		conditions = append(conditions, "ak.deleted_at IS NOT NULL")
	} else if !filters.IncludeDeleted {
		conditions = append(conditions, "ak.deleted_at IS NULL")
	}
	if !filters.IncludeRevoked {
		conditions = append(conditions, "ak.revoked_at IS NULL")
	}
	if filters.ClientID != nil {
		args = append(args, *filters.ClientID)
		conditions = append(conditions, fmt.Sprintf("ak.client_id = $%d", len(args)))
	}
	if filters.IsActive != nil {
		args = append(args, *filters.IsActive)
		conditions = append(conditions, fmt.Sprintf("ak.is_active = $%d", len(args)))
	}
	if len(conditions) == 0 {
		return "", args
	}
	return "WHERE " + strings.Join(conditions, " AND "), args
}

func replaceScopes(ctx context.Context, tx pgx.Tx, apiKeyID uuid.UUID, scopeIDs []uuid.UUID) error {
	if _, err := tx.Exec(ctx, `DELETE FROM api_key_scopes WHERE api_key_id = $1`, apiKeyID); err != nil {
		return err
	}
	for _, scopeID := range scopeIDs {
		if _, err := tx.Exec(ctx, `
			INSERT INTO api_key_scopes (api_key_id, scope_id)
			VALUES ($1, $2)
			ON CONFLICT DO NOTHING
		`, apiKeyID, scopeID); err != nil {
			return err
		}
	}
	return nil
}

func findAPIKeyScopes(ctx context.Context, tx pgx.Tx, apiKeyID uuid.UUID) ([]uuid.UUID, []APIScope, error) {
	rows, err := tx.Query(ctx, `
		SELECT asc_.id, asc_.service_id, asc_.code, asc_.resource, asc_.action
		FROM api_key_scopes aks
		JOIN api_scopes asc_ ON asc_.id = aks.scope_id
		WHERE aks.api_key_id = $1 AND asc_.is_active AND asc_.deleted_at IS NULL
		ORDER BY asc_.resource, asc_.action
	`, apiKeyID)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	scopeIDs := make([]uuid.UUID, 0)
	scopes := make([]APIScope, 0)
	for rows.Next() {
		var scope APIScope
		if err := rows.Scan(&scope.ID, &scope.ServiceID, &scope.Code, &scope.Resource, &scope.Action); err != nil {
			return nil, nil, err
		}
		scopeIDs = append(scopeIDs, scope.ID)
		scopes = append(scopes, scope)
	}
	return scopeIDs, scopes, rows.Err()
}

type apiKeyHashQuerier interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

func apiKeyHashExists(ctx context.Context, db apiKeyHashQuerier, keyHash string) (bool, error) {
	var exists bool
	err := db.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM api_keys WHERE key_hash = $1)`, keyHash).Scan(&exists)
	return exists, err
}

func isAPIKeyHashConflict(err error) bool {
	_, constraint, ok := dberror.ClassifyPgError(err)
	return ok && strings.HasPrefix(constraint, "api_keys_key_hash")
}

func (r *Repository) FindOptions(ctx context.Context) (*APIKeyOptions, error) {
	clients, err := queryOptions(ctx, r.db, `
		SELECT id, name
		FROM clients
		WHERE is_active AND deleted_at IS NULL
		ORDER BY name
	`)
	if err != nil {
		return nil, err
	}
	scopes, err := queryScopeOptions(ctx, r.db)
	if err != nil {
		return nil, err
	}
	rateLimits, err := queryOptions(ctx, r.db, `
		SELECT id, name
		FROM rate_limit_policies
		WHERE is_active AND deleted_at IS NULL
		ORDER BY name
	`)
	if err != nil {
		return nil, err
	}
	return &APIKeyOptions{Clients: clients, Scopes: scopes, RateLimits: rateLimits}, nil
}

type optionQuerier interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
}

func queryOptions(ctx context.Context, db optionQuerier, query string) ([]APIKeyOption, error) {
	rows, err := db.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	options := make([]APIKeyOption, 0)
	for rows.Next() {
		var option APIKeyOption
		if err := rows.Scan(&option.ID, &option.Name); err != nil {
			return nil, err
		}
		options = append(options, option)
	}
	return options, rows.Err()
}

func queryScopeOptions(ctx context.Context, db optionQuerier) ([]APIScope, error) {
	rows, err := db.Query(ctx, `
		SELECT id, service_id, code, resource, action
		FROM api_scopes
		WHERE is_active AND deleted_at IS NULL
		ORDER BY resource, action
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	scopes := make([]APIScope, 0)
	for rows.Next() {
		var scope APIScope
		if err := rows.Scan(&scope.ID, &scope.ServiceID, &scope.Code, &scope.Resource, &scope.Action); err != nil {
			return nil, err
		}
		scopes = append(scopes, scope)
	}
	return scopes, rows.Err()
}

func validateReferences(ctx context.Context, tx pgx.Tx, apiKey *APIKey) error {
	var exists bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS (
		SELECT 1 FROM clients WHERE id = $1 AND is_active AND deleted_at IS NULL
	)`, apiKey.ClientID).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return ErrClientUnavailable
	}
	if apiKey.RateLimitID != nil {
		if err := tx.QueryRow(ctx, `SELECT EXISTS (
			SELECT 1 FROM rate_limit_policies WHERE id = $1 AND is_active AND deleted_at IS NULL
		)`, *apiKey.RateLimitID).Scan(&exists); err != nil {
			return err
		}
		if !exists {
			return ErrRateLimitUnavailable
		}
	}
	return nil
}

func findScopeDetails(ctx context.Context, tx pgx.Tx, scopeIDs []uuid.UUID) ([]APIScope, error) {
	rows, err := tx.Query(ctx, `
		SELECT id, service_id, code, resource, action
		FROM api_scopes
		WHERE id = ANY($1) AND is_active AND deleted_at IS NULL
	`, scopeIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	scopesByID := make(map[uuid.UUID]APIScope, len(scopeIDs))
	for rows.Next() {
		var scope APIScope
		if err := rows.Scan(&scope.ID, &scope.ServiceID, &scope.Code, &scope.Resource, &scope.Action); err != nil {
			return nil, err
		}
		scopesByID[scope.ID] = scope
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	scopes := make([]APIScope, 0, len(scopeIDs))
	for _, id := range scopeIDs {
		scope, ok := scopesByID[id]
		if !ok {
			return nil, ErrScopeUnavailable
		}
		scopes = append(scopes, scope)
	}
	return scopes, nil
}
