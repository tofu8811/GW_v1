package middleware

import (
	"errors"
	"strings"
	"time"

	"gateway-api/helper/response"
	tokenhelper "gateway-api/helper/token"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

type GatewayAuth struct {
	apiKeyAuth *APIKeyAuth
	db         *pgxpool.Pool
	rdb        *redis.Client
	jwtSecret  string
}

func NewGatewayAuth(db *pgxpool.Pool, rdb *redis.Client, cache APIKeyCache, jwtSecret string) *GatewayAuth {
	return &GatewayAuth{
		apiKeyAuth: NewAPIKeyAuth(db, cache),
		db:         db,
		rdb:        rdb,
		jwtSecret:  jwtSecret,
	}
}

func (a *GatewayAuth) Authenticate(c *fiber.Ctx, requiredScopeID *string) error {
	authHeader := strings.TrimSpace(c.Get(authorizationHeader))
	if authHeader != "" {
		if !strings.HasPrefix(authHeader, bearerPrefix) {
			return response.Unauthorized(c, "invalid authorization header")
		}
		return a.authenticateJWT(c, requiredScopeID, strings.TrimSpace(strings.TrimPrefix(authHeader, bearerPrefix)))
	}
	return a.apiKeyAuth.Authenticate(c, requiredScopeID)
}

func (a *GatewayAuth) authenticateJWT(c *fiber.Ctx, requiredScopeID *string, rawToken string) error {
	if rawToken == "" {
		return response.Unauthorized(c, "missing access token")
	}
	claims, err := tokenhelper.ParseAccessToken(rawToken, a.jwtSecret)
	if err != nil {
		return response.Unauthorized(c, "invalid or expired token")
	}
	if claims.ID == "" {
		return response.Unauthorized(c, "access token is missing jti")
	}
	if a.rdb != nil {
		blacklisted, err := a.rdb.Exists(c.Context(), JWTBlacklistKey(claims.ID)).Result()
		if err != nil {
			return response.InternalServerError(c)
		}
		if blacklisted > 0 {
			return response.Unauthorized(c, "access token has been revoked")
		}
	}
	if a.db != nil {
		if err := a.ensureUserActive(c, claims.UserID); err != nil {
			return err
		}
	}
	if requiredScopeID != nil && !strings.EqualFold(claims.Role, "admin") {
		if a.db == nil {
			return response.InternalServerError(c)
		}
		allowed, err := a.userHasScope(c, claims.UserID, *requiredScopeID)
		if err != nil {
			return response.InternalServerError(c)
		}
		if !allowed {
			return response.Forbidden(c, "JWT does not have required scope for this route")
		}
		c.Locals(LocalsAPIKeyScopes, []string{*requiredScopeID})
	}

	c.Locals(LocalsJWTClaims, claims)
	c.Locals(LocalsUserID, claims.UserID)
	c.Locals(LocalsUserRole, claims.Role)
	c.Locals(LocalsPermissions, claims.Permissions)

	return nil
}

func (a *GatewayAuth) ensureUserActive(c *fiber.Ctx, userID string) error {
	var isActive bool
	err := a.db.QueryRow(c.Context(), `SELECT is_active FROM users WHERE id = $1`, userID).Scan(&isActive)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && !isActive) {
		return response.Unauthorized(c, "user is inactive or no longer exists")
	}
	if err != nil {
		return response.InternalServerError(c)
	}
	return nil
}

func (a *GatewayAuth) userHasScope(c *fiber.Ctx, userID string, scopeID string) (bool, error) {
	var allowed bool
	err := a.db.QueryRow(c.Context(), `
		SELECT EXISTS (
			SELECT 1
			FROM clients c
			JOIN api_keys ak
			  ON ak.client_id = c.id
			 AND ak.is_active = TRUE
			 AND ak.deleted_at IS NULL
			 AND ak.revoked_at IS NULL
			 AND (ak.expires_at IS NULL OR ak.expires_at > $3)
			JOIN api_key_scopes aks
			  ON aks.api_key_id = ak.id
			WHERE c.owner_user_id = $1::uuid
			  AND c.is_active = TRUE
			  AND c.deleted_at IS NULL
			  AND aks.scope_id = $2::uuid
		)
	`, userID, scopeID, time.Now()).Scan(&allowed)
	return allowed, err
}
