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
	ClientID    uuid.UUID
	ScopeIDs    []uuid.UUID
	Scopes      []APIScope
	RateLimitID *uuid.UUID
	ExpiresAt   *time.Time
	IsActive    bool
	RevokedAt   *time.Time
	LastUsedAt  *time.Time
	CreatedBy   *uuid.UUID
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type APIScope struct {
	ID       uuid.UUID
	Code     string
	Resource string
	Action   string
}

type APIKeyOption struct {
	ID   uuid.UUID
	Name string
}

type APIKeyOptions struct {
	Clients    []APIKeyOption
	Scopes     []APIScope
	RateLimits []APIKeyOption
}

type APIKeyListFilters struct {
	ClientID       *uuid.UUID
	IsActive       *bool
	IncludeRevoked bool
	IncludeDeleted bool
	DeletedOnly    bool
}
