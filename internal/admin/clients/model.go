package clients

import (
	"time"

	"github.com/google/uuid"
)

type Client struct {
	ID          uuid.UUID
	Name        string
	ClientType  string
	OwnerUserID *uuid.UUID
	IsActive    bool
	CreatedAt   time.Time
	UpdatedAt   time.Time
}
