package routes

import (
	"time"

	"github.com/google/uuid"
)

type Route struct {
	ID              uuid.UUID
	Path            string
	Method          string
	ServiceID       uuid.UUID
	StripPrefix     bool
	RewriteTarget   *string
	AuthRequired    bool
	RequiredScopeID *uuid.UUID
	RateLimitID     *uuid.UUID
	CORSPolicyID    *uuid.UUID
	CORSPolicy      *CORSPolicySummary
	Priority        int
	IsActive        bool
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type CORSPolicySummary struct {
	ID               uuid.UUID
	Name             string
	AllowedOrigins   []string
	AllowedMethods   []string
	AllowedHeaders   []string
	ExposedHeaders   []string
	AllowCredentials bool
	MaxAge           int
}
