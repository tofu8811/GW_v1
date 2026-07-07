package apikeys

import (
	"time"

	"github.com/google/uuid"
)

type APIKey struct {
	ID            uuid.UUID
	KeyHash       string
	KeyPrefix     string
	Label         *string
	ClientID      uuid.UUID
	PermissionIDs []uuid.UUID
	Permissions   []string
	RateLimitID   *uuid.UUID
	ExpiresAt     *time.Time
	IsActive      bool
	RevokedAt     *time.Time
	LastUsedAt    *time.Time
	CreatedBy     *uuid.UUID
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type APIKeyOption struct {
	ID   uuid.UUID
	Name string
}

type APIKeyOptions struct {
	Clients     []APIKeyOption
	Permissions []APIKeyOption
	RateLimits  []APIKeyOption
}
