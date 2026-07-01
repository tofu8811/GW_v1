package password

import (
	"errors"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func TestHashPasswordReturnsBcryptHash(t *testing.T) {
	hash, err := HashPasswordWithCost("secret123", bcrypt.MinCost)
	if err != nil {
		t.Fatalf("expected password to be hashed: %v", err)
	}

	if hash == "" {
		t.Fatal("expected hash to be returned")
	}

	if hash == "secret123" {
		t.Fatal("expected hash to be different from plain password")
	}

	if !VerifyPassword("secret123", hash) {
		t.Fatal("expected password to match hash")
	}
}

func TestVerifyPasswordRejectsWrongPassword(t *testing.T) {
	hash, err := HashPasswordWithCost("secret123", bcrypt.MinCost)
	if err != nil {
		t.Fatalf("expected password to be hashed: %v", err)
	}

	if VerifyPassword("wrong-password", hash) {
		t.Fatal("expected wrong password to be rejected")
	}
}

func TestHashPasswordRejectsEmptyPassword(t *testing.T) {
	_, err := HashPassword("")
	if !errors.Is(err, ErrEmptyPassword) {
		t.Fatalf("expected ErrEmptyPassword, got %v", err)
	}
}

func TestVerifyPasswordRejectsEmptyInput(t *testing.T) {
	if VerifyPassword("", "hash") {
		t.Fatal("expected empty password to be rejected")
	}

	if VerifyPassword("secret123", "") {
		t.Fatal("expected empty hash to be rejected")
	}
}
