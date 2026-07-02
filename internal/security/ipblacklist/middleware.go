package ipblacklist

import (
	"errors"
	"log/slog"
	"strings"

	"gateway-api/helper/response"

	"github.com/gofiber/fiber/v2"
)

func Middleware(checker *Checker, logger *slog.Logger) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if checker == nil || strings.HasPrefix(c.Path(), "/admin") {
			return c.Next()
		}

		clientIP := ClientIP(c)
		match, err := checker.IsBlocked(c.Context(), clientIP)
		if err != nil {
			if logger != nil {
				message := "ip blacklist check failed; allowing request"
				if errors.Is(err, ErrInvalidClientIP) {
					message = "invalid client IP for blacklist check; allowing request"
				}
				logger.Warn(message,
					"error", err,
					"client_ip", clientIP,
					"path", c.Path(),
					"method", c.Method(),
				)
			}
			return c.Next()
		}

		if !match.Blocked {
			return c.Next()
		}

		if logger != nil {
			logger.Warn("blocked request from blacklisted IP",
				"client_ip", clientIP,
				"matched_rule", match.Rule,
				"path", c.Path(),
				"method", c.Method(),
			)
		}

		return response.Error(c, fiber.StatusForbidden, "ip_blocked", match.Reason)
	}
}

func ClientIP(c *fiber.Ctx) string {
	return c.IP()
}
