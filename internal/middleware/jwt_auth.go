package middleware

import (
	"strings"

	"gateway-api/helper/response"
	tokenhelper "gateway-api/helper/token"

	"github.com/gofiber/fiber/v2"
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

func JWTAuth(secret string, rdb *redis.Client) fiber.Handler {
	return func(c *fiber.Ctx) error {
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

		c.Locals(LocalsJWTClaims, claims)
		c.Locals(LocalsUserID, claims.UserID)
		c.Locals(LocalsUserRole, claims.Role)
		c.Locals(LocalsPermissions, claims.Permissions)

		return c.Next()
	}
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
