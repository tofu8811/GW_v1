package proxy

import (
	configcache "gateway-api/internal/config/cache"
	"gateway-api/internal/proxy/loadbalancer"
)

type UpstreamRoute struct {
	RouteID       string
	RoutePath     string
	RouteMethod   string
	StripPrefix   bool
	RewriteTarget *string
	RateLimit     *configcache.RateLimitPolicyValue

	ServiceID   string
	ServiceName string
	Protocol    string
	LBStrategy  string
	InstanceID  string
	Host        string
	Port        int
	Weight      int
	TimeoutMS   int
	RetryCount  int

	CircuitBreakerEnabled bool
	AvailableInstances    []loadbalancer.Instance

	MatchedInstances int
}
