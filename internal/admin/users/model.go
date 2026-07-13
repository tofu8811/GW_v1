package users

import (
	"time"

	"github.com/google/uuid"
)

type User struct {
	ID           uuid.UUID
	Username     string
	Email        string
	RoleID       uuid.UUID
	RoleName     string
	IsActive     bool
	PasswordHash string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}
