package proxy

import (
	"errors"
	"log/slog"
	"strconv"

	"gateway-api/helper/response"
	configcache "gateway-api/internal/config/cache"
	"gateway-api/internal/ratelimit"

	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"
)

type RateLimiter struct {
	limiter *ratelimit.Limiter
	logger  *slog.Logger
}

func NewRateLimiter(redisClient *redis.Client, logger *slog.Logger) *RateLimiter {
	if redisClient == nil {
		return nil
	}
	return &RateLimiter{limiter: ratelimit.NewLimiter(redisClient), logger: logger}
}

func (r *RateLimiter) Allow(c *fiber.Ctx, route *UpstreamRoute) (bool, error) {
	if r == nil || r.limiter == nil || route == nil || route.RateLimit == nil {
		return true, nil
	}
	if c.Method() == fiber.MethodOptions {
		return true, nil
	}

	policy := route.RateLimit
	result, err := r.limiter.Allow(c.Context(), ratelimit.Request{
		Policy:     policyFromCache(policy),
		Identifier: c.IP(),
	})
	if err != nil {
		if errors.Is(err, ratelimit.ErrUnsupportedPolicy) {
			r.logWarn("rate limit policy type is not enforced yet", "limit_type", policy.LimitType, "policy_id", policy.ID, "route_id", route.RouteID)
			return true, nil
		}
		if errors.Is(err, ratelimit.ErrInvalidPolicy) || errors.Is(err, ratelimit.ErrNilPolicy) || errors.Is(err, ratelimit.ErrMissingIdentifier) {
			r.logWarn("invalid rate limit policy config", "error", err, "policy_id", policy.ID, "max_requests", policy.MaxRequests, "window_seconds", policy.WindowSeconds, "route_id", route.RouteID)
			return true, nil
		}

		r.logError("redis rate limit check failed; allowing request", "error", err, "route_id", route.RouteID)
		return true, nil
	}

	setRateLimitHeaders(c, result)
	if !result.Allowed {
		c.Set(fiber.HeaderRetryAfter, strconv.FormatInt(result.RetryAfter, 10))
		return false, response.Error(c, fiber.StatusTooManyRequests, "rate_limit_exceeded", "Too many requests")
	}

	return true, nil
}

func policyFromCache(policy *configcache.RateLimitPolicyValue) ratelimit.Policy {
	if policy == nil {
		return ratelimit.Policy{}
	}
	return ratelimit.Policy{
		ID:            policy.ID,
		LimitType:     policy.LimitType,
		MaxRequests:   policy.MaxRequests,
		WindowSeconds: policy.WindowSeconds,
	}
}

func setRateLimitHeaders(c *fiber.Ctx, result ratelimit.Result) {
	c.Set("X-RateLimit-Limit", strconv.Itoa(result.Limit))
	c.Set("X-RateLimit-Remaining", strconv.FormatInt(result.Remaining, 10))
	c.Set("X-RateLimit-Reset", strconv.FormatInt(result.ResetAt, 10))
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
