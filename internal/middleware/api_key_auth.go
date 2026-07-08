package middleware

import (
	"context"
	"strings"
	"time"

	cryptoutil "gateway-api/helper/crypto"
	"gateway-api/helper/response"
	configcache "gateway-api/internal/config/cache"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgconn"
)

const (
	apiKeyHeader       = "X-API-Key"
	LocalsAPIKeyID     = "api_key_id"
	LocalsAPIKeyScopes = "api_key_scopes"
)

type APIKeyCache interface {
	FindAPIKeyByHash(hash string) (configcache.APIKeyValue, bool)
}

type APIKeyLastUsedUpdater interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
}

type APIKeyAuth struct {
	db    APIKeyLastUsedUpdater
	cache APIKeyCache
}

func NewAPIKeyAuth(db APIKeyLastUsedUpdater, cache APIKeyCache) *APIKeyAuth {
	return &APIKeyAuth{db: db, cache: cache}
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

	if a.cache == nil {
		return response.InternalServerError(c)
	}
	apiKey, ok := a.cache.FindAPIKeyByHash(keyHash)
	if !ok {
		return response.Unauthorized(c, "invalid API key")
	}
	if !apiKey.IsActive || !apiKey.ClientActive || apiKey.RevokedAt != nil || (apiKey.ExpiresAt != nil && !apiKey.ExpiresAt.After(time.Now())) {
		return response.Unauthorized(c, "API key is inactive, revoked, or expired")
	}
	if requiredScopeID != nil && !hasScope(apiKey.ScopeIDs, *requiredScopeID) {
		return response.Forbidden(c, "API key does not have required scope for this route")
	}

	if a.db != nil {
		if _, err := a.db.Exec(c.Context(), `UPDATE api_keys SET last_used_at = now() WHERE id = $1`, apiKey.ID); err != nil {
			return response.InternalServerError(c)
		}
	}

	c.Locals(LocalsAPIKeyID, apiKey.ID)
	c.Locals(LocalsAPIKeyScopes, apiKey.ScopeIDs)
	SetAPIKeyLogContext(c, apiKey.ID)
	if apiKey.OwnerUserID != nil {
		c.Locals(LocalsUserID, *apiKey.OwnerUserID)
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
