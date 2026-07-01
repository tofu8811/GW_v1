package middleware

import (
	"errors"
	"strings"

	"gateway-api/helper/response"
	tokenhelper "gateway-api/helper/token"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

const (
	LocalsJWTClaims   = "jwt_claims"
	LocalsUserID      = "user_id"
	LocalsUserRole    = "user_role"
	LocalsPermissions = "permissions"

	authorizationHeader = "Authorization"
	bearerPrefix        = "Bearer "
	jwtBlacklistPrefix  = "jwt:blacklist:"
)

func JWTBlacklistKey(tokenID string) string {
	return jwtBlacklistPrefix + tokenID
}

func JWTAuth(secret string, rdb *redis.Client, databases ...*pgxpool.Pool) fiber.Handler {
	var db *pgxpool.Pool
	if len(databases) > 0 {
		db = databases[0]
	}

	return func(c *fiber.Ctx) error {
		if err := authenticateJWT(c, secret, rdb, db); err != nil {
			return err
		}
		return c.Next()
	}
}

func authenticateJWT(c *fiber.Ctx, secret string, rdb *redis.Client, db *pgxpool.Pool) error {
	authHeader := strings.TrimSpace(c.Get(authorizationHeader))
	if authHeader == "" {
		return response.Unauthorized(c, "missing authorization header")
	}

	if !strings.HasPrefix(authHeader, bearerPrefix) {
		return response.Unauthorized(c, "invalid authorization header")
	}

	rawToken := strings.TrimSpace(strings.TrimPrefix(authHeader, bearerPrefix))
	if rawToken == "" {
		return response.Unauthorized(c, "missing access token")
	}

	claims, err := tokenhelper.ParseAccessToken(rawToken, secret)
	if err != nil {
		return response.Unauthorized(c, "invalid or expired token")
	}
	if claims.ID == "" {
		return response.Unauthorized(c, "access token is missing jti")
	}

	blacklisted, err := rdb.Exists(c.Context(), JWTBlacklistKey(claims.ID)).Result()
	if err != nil {
		return response.InternalServerError(c)
	}
	if blacklisted > 0 {
		return response.Unauthorized(c, "access token has been revoked")
	}
	if db != nil {
		var isActive bool
		err := db.QueryRow(c.Context(), `SELECT is_active FROM users WHERE id = $1`, claims.UserID).Scan(&isActive)
		if errors.Is(err, pgx.ErrNoRows) || (err == nil && !isActive) {
			return response.Unauthorized(c, "user is inactive or no longer exists")
		}
		if err != nil {
			return response.InternalServerError(c)
		}
	}

	c.Locals(LocalsJWTClaims, claims)
	c.Locals(LocalsUserID, claims.UserID)
	c.Locals(LocalsUserRole, claims.Role)
	c.Locals(LocalsPermissions, claims.Permissions)

	return nil
}

func GetJWTClaims(c *fiber.Ctx) (*tokenhelper.Claims, bool) {
	claims, ok := c.Locals(LocalsJWTClaims).(*tokenhelper.Claims)
	return claims, ok
}

func GetUserID(c *fiber.Ctx) string {
	userID, _ := c.Locals(LocalsUserID).(string)
	return userID
}

func GetUserRole(c *fiber.Ctx) string {
	role, _ := c.Locals(LocalsUserRole).(string)
	return role
}

func GetPermissions(c *fiber.Ctx) []string {
	permissions, _ := c.Locals(LocalsPermissions).([]string)
	return permissions
}
