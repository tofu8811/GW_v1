package middleware

import (
	"errors"
	"strings"
	"time"

	cryptoutil "gateway-api/helper/crypto"
	"gateway-api/helper/response"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

const (
	apiKeyHeader       = "X-API-Key"
	LocalsAPIKeyID     = "api_key_id"
	LocalsAPIKeyScopes = "api_key_scopes"
)

type APIKeyAuth struct {
	db        *pgxpool.Pool
	rdb       *redis.Client
	jwtSecret string
}

func NewAPIKeyAuth(db *pgxpool.Pool, rdb *redis.Client, jwtSecret string) *APIKeyAuth {
	return &APIKeyAuth{db: db, rdb: rdb, jwtSecret: jwtSecret}
}

func (a *APIKeyAuth) Authenticate(c *fiber.Ctx, requiredScopeID *string) error {
	rawKey := strings.TrimSpace(c.Get(apiKeyHeader))
	if rawKey == "" {
		return response.Unauthorized(c, "API key is required")
	}

	keyHash, err := cryptoutil.HashAPIKey(rawKey)
	if err != nil {
		return response.Unauthorized(c, "invalid API key")
	}

	var (
		apiKeyID     string
		ownerUserID  *string
		scopeIDs     []string
		isActive     bool
		expiresAt    *time.Time
		revokedAt    *time.Time
		clientActive bool
	)
	err = a.db.QueryRow(c.Context(), `
		SELECT ak.id::text,
		       c.owner_user_id::text,
		       COALESCE(array_agg(asc_.id::text) FILTER (WHERE asc_.id IS NOT NULL), '{}'::text[]) AS scope_ids,
		       ak.is_active,
		       ak.expires_at,
		       ak.revoked_at,
		       c.is_active
		FROM api_keys ak
		JOIN clients c ON c.id = ak.client_id
		LEFT JOIN api_key_scopes aks ON aks.api_key_id = ak.id
		LEFT JOIN api_scopes asc_ ON asc_.id = aks.scope_id AND asc_.deleted_at IS NULL AND asc_.is_active
		WHERE ak.key_hash = $1
		  AND ak.deleted_at IS NULL
		  AND c.deleted_at IS NULL
		GROUP BY ak.id, c.owner_user_id, c.is_active
	`, keyHash).Scan(&apiKeyID, &ownerUserID, &scopeIDs, &isActive, &expiresAt, &revokedAt, &clientActive)
	if errors.Is(err, pgx.ErrNoRows) {
		return response.Unauthorized(c, "invalid API key")
	}
	if err != nil {
		return response.InternalServerError(c)
	}
	if !isActive || !clientActive || revokedAt != nil || (expiresAt != nil && !expiresAt.After(time.Now())) {
		return response.Unauthorized(c, "API key is inactive, revoked, or expired")
	}
	if requiredScopeID != nil && !hasScope(scopeIDs, *requiredScopeID) {
		return response.Forbidden(c, "API key does not have required scope for this route")
	}

	if _, err := a.db.Exec(c.Context(), `UPDATE api_keys SET last_used_at = now() WHERE id = $1`, apiKeyID); err != nil {
		return response.InternalServerError(c)
	}

	c.Locals(LocalsAPIKeyID, apiKeyID)
	c.Locals(LocalsAPIKeyScopes, scopeIDs)
	SetAPIKeyLogContext(c, apiKeyID)
	if ownerUserID != nil {
		c.Locals(LocalsUserID, *ownerUserID)
	}
	c.Request().Header.Del(apiKeyHeader)

	return nil
}

func hasScope(scopeIDs []string, required string) bool {
	for _, scopeID := range scopeIDs {
		if scopeID == required {
			return true
		}
	}
	return false
}
