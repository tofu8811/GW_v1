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

type GatewayAuth struct {
	db        *pgxpool.Pool
	rdb       *redis.Client
	jwtSecret string
}

func NewGatewayAuth(db *pgxpool.Pool, rdb *redis.Client, jwtSecret string) *GatewayAuth {
	return &GatewayAuth{db: db, rdb: rdb, jwtSecret: jwtSecret}
}

func (a *GatewayAuth) Authenticate(c *fiber.Ctx, routeID string, method string, routePath string) error {
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
		apiKeyID   string
		userID     *string
		scopes     []string
		isActive   bool
		expiresAt  *time.Time
		userActive *bool
	)
	err = a.db.QueryRow(c.Context(), `
		SELECT ak.id::text, ak.user_id::text, ak.scopes, ak.is_active, ak.expires_at, u.is_active
		FROM api_keys ak
		LEFT JOIN users u ON u.id = ak.user_id
		WHERE ak.key_hash = $1
	`, keyHash).Scan(&apiKeyID, &userID, &scopes, &isActive, &expiresAt, &userActive)
	if errors.Is(err, pgx.ErrNoRows) {
		return response.Unauthorized(c, "invalid API key")
	}
	if err != nil {
		return response.InternalServerError(c)
	}
	if !isActive || (expiresAt != nil && !expiresAt.After(time.Now())) {
		return response.Unauthorized(c, "API key is inactive or expired")
	}
	if userActive != nil && !*userActive {
		return response.Unauthorized(c, "API key owner is inactive")
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
	if userID != nil {
		c.Locals(LocalsUserID, *userID)
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
