package token

import (
	"errors"
	"testing"
	"time"
)

func TestGenerateAndParseAccessToken(t *testing.T) {
	tokenString, err := GenerateAccessToken(AccessTokenInput{
		UserID:      "user-1",
		Role:        "admin",
		Permissions: []string{"services:read", "routes:write"},
		TTL:         time.Minute,
		Secret:      "test-secret",
		Issuer:      "gateway-api",
	})
	if err != nil {
		t.Fatalf("expected token to be generated: %v", err)
	}

	claims, err := ParseAccessToken(tokenString, "test-secret")
	if err != nil {
		t.Fatalf("expected token to be parsed: %v", err)
	}

	if claims.UserID != "user-1" {
		t.Fatalf("expected user-1, got %q", claims.UserID)
	}
	if claims.Subject != "user-1" {
		t.Fatalf("expected subject user-1, got %q", claims.Subject)
	}
	if claims.ID == "" {
		t.Fatal("expected token ID to be generated")
	}
	if claims.Role != "admin" {
		t.Fatalf("expected role admin, got %q", claims.Role)
	}
	if len(claims.Permissions) != 2 {
		t.Fatalf("expected 2 permissions, got %d", len(claims.Permissions))
	}
}

func TestGenerateAccessTokenUsesUniqueTokenIDs(t *testing.T) {
	firstToken, err := GenerateAccessToken(AccessTokenInput{
		UserID: "user-1",
		TTL:    time.Minute,
		Secret: "test-secret",
	})
	if err != nil {
		t.Fatalf("expected first token to be generated: %v", err)
	}

	secondToken, err := GenerateAccessToken(AccessTokenInput{
		UserID: "user-1",
		TTL:    time.Minute,
		Secret: "test-secret",
	})
	if err != nil {
		t.Fatalf("expected second token to be generated: %v", err)
	}

	firstClaims, err := ParseAccessToken(firstToken, "test-secret")
	if err != nil {
		t.Fatalf("expected first token to be parsed: %v", err)
	}
	secondClaims, err := ParseAccessToken(secondToken, "test-secret")
	if err != nil {
		t.Fatalf("expected second token to be parsed: %v", err)
	}

	if firstClaims.ID == secondClaims.ID {
		t.Fatalf("expected unique token IDs, got %q", firstClaims.ID)
	}
}

func TestParseAccessTokenRejectsWrongSecret(t *testing.T) {
	tokenString, err := GenerateAccessToken(AccessTokenInput{
		UserID: "user-1",
		TTL:    time.Minute,
		Secret: "test-secret",
	})
	if err != nil {
		t.Fatalf("expected token to be generated: %v", err)
	}

	if _, err := ParseAccessToken(tokenString, "wrong-secret"); err == nil {
		t.Fatal("expected wrong secret to be rejected")
	}
}

func TestParseAccessTokenRejectsExpiredToken(t *testing.T) {
	tokenString, err := GenerateAccessToken(AccessTokenInput{
		UserID: "user-1",
		TTL:    time.Nanosecond,
		Secret: "test-secret",
	})
	if err != nil {
		t.Fatalf("expected token to be generated: %v", err)
	}

	time.Sleep(time.Millisecond)

	if _, err := ParseAccessToken(tokenString, "test-secret"); err == nil {
		t.Fatal("expected expired token to be rejected")
	}
}

func TestGenerateAccessTokenRejectsInvalidInput(t *testing.T) {
	_, err := GenerateAccessToken(AccessTokenInput{TTL: time.Minute, Secret: "secret"})
	if !errors.Is(err, ErrEmptySubject) {
		t.Fatalf("expected ErrEmptySubject, got %v", err)
	}

	_, err = GenerateAccessToken(AccessTokenInput{UserID: "user-1", TTL: time.Minute})
	if !errors.Is(err, ErrEmptySecret) {
		t.Fatalf("expected ErrEmptySecret, got %v", err)
	}

	_, err = GenerateAccessToken(AccessTokenInput{UserID: "user-1", Secret: "secret"})
	if !errors.Is(err, ErrInvalidTTL) {
		t.Fatalf("expected ErrInvalidTTL, got %v", err)
	}
}

func TestGenerateAndVerifyRefreshToken(t *testing.T) {
	refreshToken, err := GenerateRefreshToken()
	if err != nil {
		t.Fatalf("expected refresh token to be generated: %v", err)
	}

	hashedRefreshToken, err := HashRefreshToken(refreshToken)
	if err != nil {
		t.Fatalf("expected refresh token to be hashed: %v", err)
	}

	if !VerifyRefreshToken(refreshToken, hashedRefreshToken) {
		t.Fatal("expected refresh token to match hash")
	}

	if VerifyRefreshToken("wrong-refresh-token", hashedRefreshToken) {
		t.Fatal("expected wrong refresh token to be rejected")
	}
}
