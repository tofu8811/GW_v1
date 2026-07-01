package ratelimit

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

var (
	ErrNilRedis          = errors.New("rate limiter redis client is nil")
	ErrNilPolicy         = errors.New("rate limit policy is required")
	ErrUnsupportedPolicy = errors.New("unsupported rate limit policy type")
	ErrInvalidPolicy     = errors.New("invalid rate limit policy config")
	ErrMissingIdentifier = errors.New("rate limit identifier is required")
)

type Limiter struct {
	redis *redis.Client
}

func NewLimiter(redisClient *redis.Client) *Limiter {
	return &Limiter{redis: redisClient}
}

func (l *Limiter) Allow(ctx context.Context, req Request) (Result, error) {
	if l == nil || l.redis == nil {
		return Result{}, ErrNilRedis
	}

	policy := req.Policy
	if strings.TrimSpace(policy.ID) == "" {
		return Result{}, ErrNilPolicy
	}
	if policy.LimitType != LimitTypeIP {
		return Result{}, ErrUnsupportedPolicy
	}
	if policy.MaxRequests <= 0 || policy.WindowSeconds <= 0 {
		return Result{}, ErrInvalidPolicy
	}
	if strings.TrimSpace(req.Identifier) == "" {
		return Result{}, ErrMissingIdentifier
	}

	now := req.Now
	if now.IsZero() {
		now = time.Now()
	}

	windowSeconds := int64(policy.WindowSeconds)
	nowUnix := now.Unix()
	windowStart := nowUnix / windowSeconds * windowSeconds
	resetAt := windowStart + windowSeconds
	key := Key(policy, req.Identifier, windowStart)

	count, err := l.redis.Incr(ctx, key).Result()
	if err != nil {
		return Result{}, err
	}
	if count == 1 {
		if err := l.redis.Expire(ctx, key, time.Duration(policy.WindowSeconds)*time.Second).Err(); err != nil {
			return Result{}, err
		}
	}

	remaining := int64(policy.MaxRequests) - count
	if remaining < 0 {
		remaining = 0
	}

	retryAfter := int64(0)
	allowed := count <= int64(policy.MaxRequests)
	if !allowed {
		retryAfter = resetAt - nowUnix
		if retryAfter < 0 {
			retryAfter = 0
		}
	}

	return Result{
		Allowed:    allowed,
		Limit:      policy.MaxRequests,
		Remaining:  remaining,
		ResetAt:    resetAt,
		RetryAfter: retryAfter,
		Key:        key,
		Count:      count,
	}, nil
}

func Key(policy Policy, identifier string, windowStart int64) string {
	return fmt.Sprintf("rl:%s:%s:%s:%d", policy.LimitType, policy.ID, identifier, windowStart)
}
