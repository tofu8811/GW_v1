package services

import (
	"time"

	"github.com/google/uuid"
)

type Service struct {
	ID                    uuid.UUID
	Name                  string
	Description           *string
	Protocol              string
	LBStrategy            string
	HealthPath            string
	TimeoutMS             int
	RetryCount            int16
	CircuitBreakerEnabled bool
	IsActive              bool
	CreatedAt             time.Time
	UpdatedAt             time.Time
}
