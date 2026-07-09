package corspolicies

import (
	"time"

	"github.com/google/uuid"
)

type CORSPolicy struct {
	ID               uuid.UUID
	Name             string
	AllowedOrigins   []string
	AllowedMethods   []string
	AllowedHeaders   []string
	ExposedHeaders   []string
	AllowCredentials bool
	MaxAge           int
	IsActive         bool
	CreatedAt        time.Time
	UpdatedAt        time.Time
}
