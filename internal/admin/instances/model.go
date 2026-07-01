package instances

import (
	"time"

	"github.com/google/uuid"
)

type ServiceInstance struct {
	ID        uuid.UUID
	ServiceID uuid.UUID
	Host      string
	Port      int
	Weight    int16
	IsActive  bool
	CreatedAt time.Time
}
