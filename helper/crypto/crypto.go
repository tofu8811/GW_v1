package crypto

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
)

const DefaultRandomTokenBytes = 32

var (
	ErrEmptyValue       = errors.New("value is required")
	ErrEmptySecret      = errors.New("secret is required")
	ErrInvalidByteSize  = errors.New("byte size must be greater than zero")
	ErrInvalidSignature = errors.New("signature is required")
)

func SHA256Hex(value string) (string, error) {
	if value == "" {
		return "", ErrEmptyValue
	}

	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:]), nil
}

func HashAPIKey(apiKey string) (string, error) {
	return SHA256Hex(apiKey)
}

func HashRefreshToken(refreshToken string) (string, error) {
	return SHA256Hex(refreshToken)
}

func HMACSHA256Hex(value string, secret string) (string, error) {
	if value == "" {
		return "", ErrEmptyValue
	}
	if secret == "" {
		return "", ErrEmptySecret
	}

	mac := hmac.New(sha256.New, []byte(secret))
	if _, err := mac.Write([]byte(value)); err != nil {
		return "", err
	}

	return hex.EncodeToString(mac.Sum(nil)), nil
}

func VerifyHMACSHA256Hex(value string, signature string, secret string) bool {
	if value == "" || signature == "" || secret == "" {
		return false
	}

	expectedSignature, err := HMACSHA256Hex(value, secret)
	if err != nil {
		return false
	}

	expectedBytes, err := hex.DecodeString(expectedSignature)
	if err != nil {
		return false
	}

	signatureBytes, err := hex.DecodeString(signature)
	if err != nil {
		return false
	}

	return hmac.Equal(signatureBytes, expectedBytes)
}

func GenerateRandomToken() (string, error) {
	return GenerateRandomTokenWithBytes(DefaultRandomTokenBytes)
}

func GenerateRandomTokenWithBytes(byteSize int) (string, error) {
	if byteSize <= 0 {
		return "", ErrInvalidByteSize
	}

	buffer := make([]byte, byteSize)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}

	return base64.RawURLEncoding.EncodeToString(buffer), nil
}
