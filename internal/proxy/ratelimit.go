package proxy

import (
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"gateway-api/helper/response"
	configcache "gateway-api/internal/config/cache"

	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"
)

type RateLimiter struct {
	redis  *redis.Client
	logger *slog.Logger
}

func NewRateLimiter(redisClient *redis.Client, logger *slog.Logger) *RateLimiter {
	if redisClient == nil {
		return nil
	}
	return &RateLimiter{redis: redisClient, logger: logger}
}

func (r *RateLimiter) Allow(c *fiber.Ctx, route *UpstreamRoute) (bool, error) {
	if r == nil || r.redis == nil || route == nil || route.RateLimit == nil {
		return true, nil
	}
	if c.Method() == fiber.MethodOptions {
		return true, nil
	}

	policy := route.RateLimit
	if policy.LimitType != "ip" {
		r.logWarn("rate limit policy type is not enforced yet", "limit_type", policy.LimitType, "policy_id", policy.ID, "route_id", route.RouteID)
		return true, nil
	}
	if policy.MaxRequests <= 0 || policy.WindowSeconds <= 0 {
		r.logWarn("invalid rate limit policy config", "policy_id", policy.ID, "max_requests", policy.MaxRequests, "window_seconds", policy.WindowSeconds)
		return true, nil
	}

	now := time.Now().Unix()
	windowStart := now / int64(policy.WindowSeconds) * int64(policy.WindowSeconds)
	resetAt := windowStart + int64(policy.WindowSeconds)
	key := rateLimitKey(policy, c.IP(), windowStart)

	count, err := r.redis.Incr(c.Context(), key).Result()
	if err != nil {
		r.logError("redis rate limit increment failed; allowing request", "error", err, "key", key, "route_id", route.RouteID)
		return true, nil
	}
	if count == 1 {
		if err := r.redis.Expire(c.Context(), key, time.Duration(policy.WindowSeconds)*time.Second).Err(); err != nil {
			r.logWarn("redis rate limit expire failed", "error", err, "key", key, "route_id", route.RouteID)
		}
	}

	remaining := int64(policy.MaxRequests) - count
	if remaining < 0 {
		remaining = 0
	}
	setRateLimitHeaders(c, policy.MaxRequests, remaining, resetAt)

	if count > int64(policy.MaxRequests) {
		retryAfter := resetAt - now
		if retryAfter < 0 {
			retryAfter = 0
		}
		c.Set(fiber.HeaderRetryAfter, strconv.FormatInt(retryAfter, 10))
		return false, response.Error(c, fiber.StatusTooManyRequests, "rate_limit_exceeded", "Too many requests")
	}

	return true, nil
}

func rateLimitKey(policy *configcache.RateLimitPolicyValue, clientIP string, windowStart int64) string {
	return fmt.Sprintf("rl:ip:%s:%s:%d", policy.ID, clientIP, windowStart)
}

func setRateLimitHeaders(c *fiber.Ctx, limit int, remaining int64, resetAt int64) {
	c.Set("X-RateLimit-Limit", strconv.Itoa(limit))
	c.Set("X-RateLimit-Remaining", strconv.FormatInt(remaining, 10))
	c.Set("X-RateLimit-Reset", strconv.FormatInt(resetAt, 10))
}

func (r *RateLimiter) logWarn(message string, args ...any) {
	if r.logger != nil {
		r.logger.Warn(message, args...)
	}
}

func (r *RateLimiter) logError(message string, args ...any) {
	if r.logger != nil {
		r.logger.Error(message, args...)
	}
}
