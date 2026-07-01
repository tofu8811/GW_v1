package apikeys

import (
	"time"

	"github.com/google/uuid"
)

type APIKey struct {
	ID          uuid.UUID
	KeyHash     string
	KeyPrefix   string
	Label       *string
	UserID      *uuid.UUID
	Scopes      []string
	RateLimitID *uuid.UUID
	ExpiresAt   *time.Time
	IsActive    bool
	LastUsedAt  *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}
