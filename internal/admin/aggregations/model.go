package aggregations

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type Aggregation struct {
	ID              uuid.UUID
	Name            string
	Path            string
	Method          string
	AuthRequired    bool
	RequiredScopeID *uuid.UUID
	RateLimitID     *uuid.UUID
	CORSPolicyID    *uuid.UUID
	CORSPolicy      *CORSPolicySummary
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

type AggregationStep struct {
	ID              uuid.UUID
	AggregationID   uuid.UUID
	ServiceID       uuid.UUID
	Sequence        int16
	DependsOn       *uuid.UUID
	IsRequired      bool
	RequestTemplate json.RawMessage
	ResponseMapping json.RawMessage
	IsActive        bool
	CreatedAt       time.Time
	UpdatedAt       time.Time
}
