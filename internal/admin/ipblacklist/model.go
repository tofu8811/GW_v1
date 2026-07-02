package ipblacklist

import (
	"time"

	"github.com/google/uuid"
)

type IPBlacklistEntry struct {
	ID        uuid.UUID
	IPOrCIDR  string
	Reason    *string
	CreatedBy *uuid.UUID
	ExpiresAt *time.Time
	IsActive  bool
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt *time.Time
}
