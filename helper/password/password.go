package password

import (
	"errors"

	"golang.org/x/crypto/bcrypt"
)

const DefaultCost = bcrypt.DefaultCost

var (
	ErrEmptyPassword = errors.New("password is required")
	ErrEmptyHash     = errors.New("password hash is required")
)

func HashPassword(plainPassword string) (string, error) {
	return HashPasswordWithCost(plainPassword, DefaultCost)
}

func HashPasswordWithCost(plainPassword string, cost int) (string, error) {
	if plainPassword == "" {
		return "", ErrEmptyPassword
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(plainPassword), cost)
	if err != nil {
		return "", err
	}

	return string(hashedPassword), nil
}

func VerifyPassword(plainPassword string, hashedPassword string) bool {
	if plainPassword == "" || hashedPassword == "" {
		return false
	}

	return bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(plainPassword)) == nil
}
