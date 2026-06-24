package proxy

import "github.com/google/uuid"

type RouteConfig struct {
	ID            uuid.UUID
	Path          string
	Method        string
	ServiceID     uuid.UUID
	StripPrefix   bool
	RewriteTarget *string
	AuthRequired  bool
	RateLimitID   *uuid.UUID
	Priority      int
}

type ServiceConfig struct {
	ID                    uuid.UUID
	Name                  string
	Protocol              string
	LBStrategy            string
	TimeoutMS             int
	RetryCount            int16
	CircuitBreakerEnabled bool
}

type ServiceInstance struct {
	ID        uuid.UUID
	ServiceID uuid.UUID
	Host      string
	Port      int
	Weight    int16
}
