package auth

import (
	"context"
	"errors"
	"time"

	tokenhelper "gateway-api/helper/token"
	"gateway-api/internal/middleware"

	"github.com/redis/go-redis/v9"
)

const refreshTokenKeyPrefix = "refresh:"

var ErrRefreshTokenNotFound = errors.New("refresh token not found")

var revokeSessionScript = redis.NewScript(`
local owner = redis.call("GET", KEYS[1])
if not owner or owner ~= ARGV[1] then
    return 0
end

redis.call("DEL", KEYS[1])
redis.call("SET", KEYS[2], "1", "PX", ARGV[2])
return 1
`)

type RefreshStore struct {
	client *redis.Client
}

func NewRefreshStore(client *redis.Client) *RefreshStore {
	return &RefreshStore{client: client}
}

func (s *RefreshStore) Save(ctx context.Context, refreshToken string, userID string, ttl time.Duration) error {
	key, err := refreshTokenKey(refreshToken)
	if err != nil {
		return err
	}

	return s.client.Set(ctx, key, userID, ttl).Err()
}

func (s *RefreshStore) Consume(ctx context.Context, refreshToken string) (string, error) {
	key, err := refreshTokenKey(refreshToken)
	if err != nil {
		return "", ErrRefreshTokenNotFound
	}

	userID, err := s.client.GetDel(ctx, key).Result()
	if errors.Is(err, redis.Nil) {
		return "", ErrRefreshTokenNotFound
	}
	if err != nil {
		return "", err
	}

	return userID, nil
}

func (s *RefreshStore) RevokeSession(ctx context.Context, refreshToken string, userID string, tokenID string, accessTTL time.Duration) (bool, error) {
	refreshKey, err := refreshTokenKey(refreshToken)
	if err != nil || userID == "" || tokenID == "" || accessTTL <= 0 {
		return false, nil
	}

	result, err := revokeSessionScript.Run(
		ctx,
		s.client,
		[]string{refreshKey, middleware.JWTBlacklistKey(tokenID)},
		userID,
		accessTTL.Milliseconds(),
	).Int()
	if err != nil {
		return false, err
	}

	return result == 1, nil
}

func refreshTokenKey(refreshToken string) (string, error) {
	hash, err := tokenhelper.HashRefreshToken(refreshToken)
	if err != nil {
		return "", err
	}

	return refreshTokenKeyPrefix + hash, nil
}
