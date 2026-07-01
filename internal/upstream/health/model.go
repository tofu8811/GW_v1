package health

import (
	"errors"
	"time"
)

var ErrInstanceNotFound = errors.New("upstream instance not found")

type Status string

const (
	StatusAlive   Status = "alive"
	StatusDown    Status = "down"
	StatusUnknown Status = "unknown"
)

type InstanceHealth struct {
	InstanceID string    `json:"instance_id"`
	ServiceID  string    `json:"service_id"`
	Status     Status    `json:"status"`
	LatencyMS  float64   `json:"latency_ms"`
	FailCount  int       `json:"fail_count"`
	LastCheck  time.Time `json:"last_check"`
}
