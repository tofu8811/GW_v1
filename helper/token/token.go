package token

import (
	"errors"
	"time"

	cryptoutil "gateway-api/helper/crypto"

	"github.com/golang-jwt/jwt/v5"
)

const DefaultRefreshTokenBytes = 32

var (
	ErrEmptySecret      = errors.New("jwt secret is required")
	ErrEmptyToken       = errors.New("token is required")
	ErrEmptySubject     = errors.New("subject is required")
	ErrInvalidToken     = errors.New("token is invalid")
	ErrInvalidTTL       = errors.New("ttl must be greater than zero")
	ErrUnexpectedMethod = errors.New("unexpected jwt signing method")
)

type Claims struct {
	UserID      string   `json:"user_id"`
	Role        string   `json:"role,omitempty"`
	Permissions []string `json:"permissions,omitempty"`
	jwt.RegisteredClaims
}

type AccessTokenInput struct {
	UserID      string
	Role        string
	Permissions []string
	TTL         time.Duration
	Secret      string
	Issuer      string
}

func GenerateAccessToken(input AccessTokenInput) (string, error) {
	if input.UserID == "" {
		return "", ErrEmptySubject
	}
	if input.Secret == "" {
		return "", ErrEmptySecret
	}
	if input.TTL <= 0 {
		return "", ErrInvalidTTL
	}

	tokenID, err := cryptoutil.GenerateRandomTokenWithBytes(16)
	if err != nil {
		return "", err
	}

	now := time.Now().UTC()
	claims := Claims{
		UserID:      input.UserID,
		Role:        input.Role,
		Permissions: input.Permissions,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        tokenID,
			Subject:   input.UserID,
			Issuer:    input.Issuer,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(input.TTL)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(input.Secret))
}

func ParseAccessToken(tokenString string, secret string) (*Claims, error) {
	if tokenString == "" {
		return nil, ErrEmptyToken
	}
	if secret == "" {
		return nil, ErrEmptySecret
	}

	claims := &Claims{}
	parsedToken, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (any, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, ErrUnexpectedMethod
		}

		return []byte(secret), nil
	})
	if err != nil {
		return nil, err
	}
	if !parsedToken.Valid {
		return nil, ErrInvalidToken
	}

	return claims, nil
}

func GenerateRefreshToken() (string, error) {
	return cryptoutil.GenerateRandomTokenWithBytes(DefaultRefreshTokenBytes)
}

func HashRefreshToken(refreshToken string) (string, error) {
	return cryptoutil.HashRefreshToken(refreshToken)
}

func VerifyRefreshToken(refreshToken string, hashedRefreshToken string) bool {
	if refreshToken == "" || hashedRefreshToken == "" {
		return false
	}

	hashedInput, err := HashRefreshToken(refreshToken)
	if err != nil {
		return false
	}

	return hashedInput == hashedRefreshToken
}
