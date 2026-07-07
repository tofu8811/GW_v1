package middleware

import (
	"strings"

	"gateway-api/helper/response"

	"github.com/gofiber/fiber/v2"
)

func RequirePermission(required string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if strings.TrimSpace(required) == "" {
			return c.Next()
		}
		if strings.EqualFold(GetUserRole(c), "admin") {
			return c.Next()
		}
		for _, permission := range GetPermissions(c) {
			if permission == "*" || strings.EqualFold(permission, required) {
				return c.Next()
			}
		}
		return response.Forbidden(c, "missing permission: "+required)
	}
}
// bắt buộc gắn sau jwtauth vì dựa trên thông tin trong jwt
