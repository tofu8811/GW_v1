package permissions

import (
	"time"

	"github.com/google/uuid"
)

type Permission struct {
	ID        uuid.UUID
	Resource  string
	Action    string
	IsActive  bool
	CreatedAt time.Time
	UpdatedAt time.Time
}
