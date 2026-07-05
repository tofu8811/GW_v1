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

func (a *APIKeyAuth) Authenticate(c *fiber.Ctx, routeID string, method string, routePath string) error {
	if strings.TrimSpace(c.Get(authorizationHeader)) != "" {
		return authenticateJWT(c, a.jwtSecret, a.rdb, a.db)
	}

	rawKey := strings.TrimSpace(c.Get(apiKeyHeader))
	if rawKey == "" {
		return response.Unauthorized(c, "JWT bearer token or API key is required")
	}

	keyHash, err := cryptoutil.HashAPIKey(rawKey)
	if err != nil {
		return response.Unauthorized(c, "invalid API key")
	}

	var (
		apiKeyID     string
		ownerUserID  *string
		scopes       []string
		isActive     bool
		expiresAt    *time.Time
		revokedAt    *time.Time
		clientActive bool
	)
	err = a.db.QueryRow(c.Context(), `
		SELECT ak.id::text,
		       c.owner_user_id::text,
		       COALESCE(array_agg(p.action || ':' || p.resource) FILTER (WHERE p.id IS NOT NULL), '{}'::text[]) AS scopes,
		       ak.is_active,
		       ak.expires_at,
		       ak.revoked_at,
		       c.is_active
		FROM api_keys ak
		JOIN clients c ON c.id = ak.client_id
		LEFT JOIN api_key_scopes aks ON aks.api_key_id = ak.id
		LEFT JOIN permissions p ON p.id = aks.permission_id AND p.deleted_at IS NULL AND p.is_active
		WHERE ak.key_hash = $1
		  AND ak.deleted_at IS NULL
		  AND c.deleted_at IS NULL
		GROUP BY ak.id, c.owner_user_id, c.is_active
	`, keyHash).Scan(&apiKeyID, &ownerUserID, &scopes, &isActive, &expiresAt, &revokedAt, &clientActive)
	if errors.Is(err, pgx.ErrNoRows) {
		return response.Unauthorized(c, "invalid API key")
	}
	if err != nil {
		return response.InternalServerError(c)
	}
	if !isActive || !clientActive || revokedAt != nil || (expiresAt != nil && !expiresAt.After(time.Now())) {
		return response.Unauthorized(c, "API key is inactive, revoked, or expired")
	}
	if !ScopeAllowsRoute(scopes, routeID, method, routePath) {
		return response.Forbidden(c, "API key does not have access to this route")
	}

	if _, err := a.db.Exec(c.Context(), `UPDATE api_keys SET last_used_at = now() WHERE id = $1`, apiKeyID); err != nil {
		return response.InternalServerError(c)
	}

	c.Locals(LocalsAPIKeyID, apiKeyID)
	c.Locals(LocalsAPIKeyScopes, scopes)
	SetAPIKeyLogContext(c, apiKeyID)
	if ownerUserID != nil {
		c.Locals(LocalsUserID, *ownerUserID)
	}
	c.Request().Header.Del(apiKeyHeader)

	return nil
}

func ScopeAllowsRoute(scopes []string, routeID string, method string, routePath string) bool {
	requiredRoute := "route:" + strings.ToLower(strings.TrimSpace(routeID))
	requiredMethodPath := strings.ToUpper(strings.TrimSpace(method)) + ":" + strings.TrimSpace(routePath)

	for _, scope := range scopes {
		normalized := strings.TrimSpace(scope)
		if normalized == "*" || strings.EqualFold(normalized, requiredRoute) || strings.EqualFold(normalized, requiredMethodPath) {
			return true
		}
	}

	return false
}
