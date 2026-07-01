package ratelimits

import (
	"time"

	"github.com/google/uuid"
)

type RateLimitPolicy struct {
	ID            uuid.UUID
	Name          string
	LimitType     string
	MaxRequests   int
	WindowSeconds int
	IsActive      bool
	CreatedAt     time.Time
	UpdatedAt     time.Time
}
