package auth

import "github.com/google/uuid"

type User struct {
	ID           uuid.UUID
	Username     string
	Email        string
	PasswordHash string
	Role         string
	Permissions  []string
	IsActive     bool
}
