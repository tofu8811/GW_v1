package aggregations

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type Aggregation struct {
	ID        uuid.UUID
	Name      string
	Path      string
	Method    string
	IsActive  bool
	CreatedAt time.Time
	UpdatedAt time.Time
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
