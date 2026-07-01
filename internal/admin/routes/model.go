package routes

import (
	"time"

	"github.com/google/uuid"
)

type Route struct {
	ID            uuid.UUID
	Path          string
	Method        string
	ServiceID     uuid.UUID
	StripPrefix   bool
	RewriteTarget *string
	AuthRequired  bool
	RateLimitID   *uuid.UUID
	Priority      int
	IsActive      bool
	CreatedAt     time.Time
	UpdatedAt     time.Time
}