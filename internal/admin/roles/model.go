package roles

import (
	"time"

	"github.com/google/uuid"
)

type Role struct {
	ID          uuid.UUID
	Name        string
	Description *string
	IsActive    bool
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type Permission struct {
	ID       uuid.UUID
	Resource string
	Action   string
}
