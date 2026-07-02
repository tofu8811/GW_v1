package ipblacklist

import (
	"context"
	"errors"
	"log/slog"
	"net/netip"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

const (
	KeyExactIP = "blacklist:ip"
	KeyCIDR    = "blacklist:cidr"
)

var ErrInvalidClientIP = errors.New("invalid client IP")

type Match struct {
	Blocked bool
	Rule    string
	Reason  string
}

type Checker struct {
	db     *pgxpool.Pool
	redis  *redis.Client
	logger *slog.Logger

	mu    sync.RWMutex
	exact map[string]entry
	cidrs []cidrEntry
}

type entry struct {
	Rule      string
	Reason    string
	ExpiresAt *time.Time
}

type cidrEntry struct {
	Prefix    netip.Prefix
	Rule      string
	Reason    string
	ExpiresAt *time.Time
}

func NewChecker(db *pgxpool.Pool, redisClient *redis.Client, logger *slog.Logger) *Checker {
	return &Checker{
		db:     db,
		redis:  redisClient,
		logger: logger,
		exact:  map[string]entry{},
	}
}

func (c *Checker) Reload(ctx context.Context) error {
	if c == nil || c.db == nil {
		return nil
	}

	rows, err := c.db.Query(ctx, `
		SELECT ip_or_cidr::text, reason, expires_at
		FROM ip_blacklist
		WHERE is_active = TRUE
		  AND deleted_at IS NULL
		  AND (expires_at IS NULL OR expires_at > now())
		ORDER BY created_at DESC
	`)
	if err != nil {
		return err
	}
	defer rows.Close()

	exact := map[string]entry{}
	var cidrs []cidrEntry
	var exactRedis []interface{}
	var cidrRedis []interface{}

	for rows.Next() {
		var raw string
		var reason *string
		var expiresAt *time.Time
		if err := rows.Scan(&raw, &reason, &expiresAt); err != nil {
			return err
		}

		prefix, err := netip.ParsePrefix(raw)
		if err != nil {
			c.logWarn("skipping invalid ip blacklist rule from database", "rule", raw, "error", err)
			continue
		}

		item := entry{Rule: prefix.String(), Reason: reasonValue(reason), ExpiresAt: expiresAt}
		if isExact(prefix) {
			ip := prefix.Addr().Unmap().String()
			exact[ip] = item
			exactRedis = append(exactRedis, ip)
			continue
		}

		cidrs = append(cidrs, cidrEntry{
			Prefix:    prefix,
			Rule:      prefix.String(),
			Reason:    item.Reason,
			ExpiresAt: expiresAt,
		})
		cidrRedis = append(cidrRedis, prefix.String())
	}
	if err := rows.Err(); err != nil {
		return err
	}

	c.mu.Lock()
	c.exact = exact
	c.cidrs = cidrs
	c.mu.Unlock()

	if c.redis == nil {
		return nil
	}

	pipe := c.redis.TxPipeline()
	pipe.Del(ctx, KeyExactIP, KeyCIDR)
	if len(exactRedis) > 0 {
		pipe.SAdd(ctx, KeyExactIP, exactRedis...)
	}
	if len(cidrRedis) > 0 {
		pipe.SAdd(ctx, KeyCIDR, cidrRedis...)
	}
	_, err = pipe.Exec(ctx)
	return err
}

func (c *Checker) IsBlocked(ctx context.Context, clientIP string) (Match, error) {
	if c == nil {
		return Match{}, nil
	}

	addr, err := netip.ParseAddr(clientIP)
	if err != nil {
		return Match{}, ErrInvalidClientIP
	}
	addr = addr.Unmap()
	ip := addr.String()

	if c.redis != nil {
		blocked, err := c.redis.SIsMember(ctx, KeyExactIP, ip).Result()
		if err != nil {
			return Match{}, err
		}
		if blocked {
			c.mu.RLock()
			item := c.exact[ip]
			c.mu.RUnlock()
			if NowExpired(item.ExpiresAt) {
				return Match{}, nil
			}
			if item.Rule == "" {
				item = entry{Rule: ip, Reason: defaultReason("")}
			}
			return Match{Blocked: true, Rule: item.Rule, Reason: defaultReason(item.Reason)}, nil
		}
	}

	c.mu.RLock()
	if item, ok := c.exact[ip]; ok {
		c.mu.RUnlock()
		if NowExpired(item.ExpiresAt) {
			return Match{}, nil
		}
		return Match{Blocked: true, Rule: item.Rule, Reason: defaultReason(item.Reason)}, nil
	}
	for _, item := range c.cidrs {
		if !NowExpired(item.ExpiresAt) && item.Prefix.Contains(addr) {
			c.mu.RUnlock()
			return Match{Blocked: true, Rule: item.Rule, Reason: defaultReason(item.Reason)}, nil
		}
	}
	c.mu.RUnlock()

	return Match{}, nil
}

func isExact(prefix netip.Prefix) bool {
	return (prefix.Addr().Is4() && prefix.Bits() == 32) || (prefix.Addr().Is6() && prefix.Bits() == 128)
}

func reasonValue(reason *string) string {
	if reason == nil {
		return ""
	}
	return *reason
}

func defaultReason(reason string) string {
	if reason == "" {
		return "IP address is blocked"
	}
	return reason
}

func (c *Checker) logWarn(message string, args ...any) {
	if c != nil && c.logger != nil {
		c.logger.Warn(message, args...)
	}
}

func NowExpired(expiresAt *time.Time) bool {
	return expiresAt != nil && !expiresAt.After(time.Now())
}
