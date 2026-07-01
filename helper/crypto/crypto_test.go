package crypto

import (
	"encoding/hex"
	"errors"
	"testing"
)

func TestSHA256Hex(t *testing.T) {
	got, err := SHA256Hex("abc")
	if err != nil {
		t.Fatalf("expected value to be hashed: %v", err)
	}

	const expected = "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad"
	if got != expected {
		t.Fatalf("expected %q, got %q", expected, got)
	}
}

func TestHashAPIKeyUsesSHA256(t *testing.T) {
	got, err := HashAPIKey("gw_live_secret")
	if err != nil {
		t.Fatalf("expected api key to be hashed: %v", err)
	}

	if len(got) != hex.EncodedLen(32) {
		t.Fatalf("expected sha256 hex length 64, got %d", len(got))
	}
}

func TestHashRefreshTokenRejectsEmptyValue(t *testing.T) {
	_, err := HashRefreshToken("")
	if !errors.Is(err, ErrEmptyValue) {
		t.Fatalf("expected ErrEmptyValue, got %v", err)
	}
}

func TestHMACSHA256HexAndVerify(t *testing.T) {
	signature, err := HMACSHA256Hex("payload", "secret")
	if err != nil {
		t.Fatalf("expected signature to be generated: %v", err)
	}

	if !VerifyHMACSHA256Hex("payload", signature, "secret") {
		t.Fatal("expected valid signature to pass")
	}

	if VerifyHMACSHA256Hex("payload", signature, "wrong-secret") {
		t.Fatal("expected wrong secret to be rejected")
	}
}

func TestHMACSHA256HexRejectsEmptySecret(t *testing.T) {
	_, err := HMACSHA256Hex("payload", "")
	if !errors.Is(err, ErrEmptySecret) {
		t.Fatalf("expected ErrEmptySecret, got %v", err)
	}
}

func TestGenerateRandomToken(t *testing.T) {
	token, err := GenerateRandomTokenWithBytes(16)
	if err != nil {
		t.Fatalf("expected random token to be generated: %v", err)
	}

	if token == "" {
		t.Fatal("expected random token")
	}

	secondToken, err := GenerateRandomTokenWithBytes(16)
	if err != nil {
		t.Fatalf("expected second random token to be generated: %v", err)
	}

	if token == secondToken {
		t.Fatal("expected generated tokens to be different")
	}
}

func TestGenerateRandomTokenRejectsInvalidByteSize(t *testing.T) {
	_, err := GenerateRandomTokenWithBytes(0)
	if !errors.Is(err, ErrInvalidByteSize) {
		t.Fatalf("expected ErrInvalidByteSize, got %v", err)
	}
}
