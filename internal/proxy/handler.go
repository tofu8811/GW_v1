package proxy

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"gateway-api/helper/response"
	configcache "gateway-api/internal/config/cache"
	appmiddleware "gateway-api/internal/middleware"
	"gateway-api/internal/proxy/loadbalancer"
	"gateway-api/internal/upstream/breaker"
	upstreamhealth "gateway-api/internal/upstream/health"

	"github.com/gofiber/fiber/v2"
)

var ErrRouteNotFound = errors.New("gateway route not found")

type Handler struct {
	configCache   *configcache.Store
	logger        *slog.Logger
	roundRobin    *loadbalancer.RoundRobin
	weighted      *loadbalancer.WeightedRoundRobin
	healthFilter  *upstreamhealth.HealthFilter
	breakers      *breaker.Registry
	authenticator RouteAuthenticator
	rateLimiter   *RateLimiter
}

type RouteAuthenticator interface {
	Authenticate(c *fiber.Ctx, requiredScopeID *string) error
}

func NewHandler(configCache *configcache.Store, logger *slog.Logger, healthFilter *upstreamhealth.HealthFilter, breakers *breaker.Registry, rateLimiter *RateLimiter, authenticator RouteAuthenticator) *Handler {
	return &Handler{
		configCache:   configCache,
		logger:        logger,
		roundRobin:    loadbalancer.NewRoundRobin(),
		weighted:      loadbalancer.NewWeightedRoundRobin(),
		healthFilter:  healthFilter,
		breakers:      breakers,
		authenticator: authenticator,
		rateLimiter:   rateLimiter,
	}
}

func (h *Handler) Proxy(c *fiber.Ctx) error {
	startedAt := time.Now()
	requestPath := c.Path()
	method := c.Method()
	if method == fiber.MethodOptions && strings.TrimSpace(c.Get(fiber.HeaderAccessControlRequestMethod)) != "" {
		return h.handleCORSPreflight(c, requestPath)
	}

	if aggregation, ok := h.configCache.FindAggregation(method, requestPath); ok {
		origin := strings.TrimSpace(c.Get(fiber.HeaderOrigin))
		if origin != "" {
			if err := validateCORSRequest(aggregation.CORS, origin, method); err != nil {
				return response.Forbidden(c, err.Error())
			}
			defer setActualCORSHeaders(c, aggregation.CORS, origin)
		}
		return h.handleAggregation(c, aggregation)
	}

	route, params, err := h.findRoute(c.Context(), requestPath, method)
	if errors.Is(err, ErrRouteNotFound) {
		h.logger.Warn("gateway route not found",
			"request_id", c.GetRespHeader(fiber.HeaderXRequestID),
			"method", method,
			"path", requestPath,
			"client_ip", c.IP(),
		)
		return response.NotFound(c, "gateway route not found")
	}
	if err != nil {
		h.logger.Error("failed to find gateway route",
			"request_id", c.GetRespHeader(fiber.HeaderXRequestID),
			"error", err,
			"path", requestPath,
			"method", method,
			"client_ip", c.IP(),
		)
		return response.InternalServerError(c)
	}

	origin := strings.TrimSpace(c.Get(fiber.HeaderOrigin))
	if origin != "" {
		if err := validateCORSRequest(route.CORS, origin, method); err != nil {
			return response.Forbidden(c, err.Error())
		}
		defer setActualCORSHeaders(c, route.CORS, origin)
	}

	appmiddleware.SetRouteLogContext(c, route.RouteID, route.ServiceID, route.ServiceName, route.RoutePath)
	if route.AuthRequired {
		if h.authenticator == nil {
			h.logger.Error("route requires authentication but no authenticator is configured", "route_id", route.RouteID)
			return response.InternalServerError(c)
		}
		if err := h.authenticator.Authenticate(c, route.RequiredScopeID); err != nil {
			return err
		}
		if c.Response().StatusCode() >= fiber.StatusBadRequest {
			return nil
		}
	}

	if h.rateLimiter != nil {
		allowed, err := h.rateLimiter.Allow(c, route)
		if err != nil {
			return err
		}
		if !allowed {
			return nil
		}
	}

	prepareForwardedRequest(c)
	return h.forwardWithRetry(c, route, requestPath, params, startedAt)
}

func (h *Handler) findRoute(ctx context.Context, path string, method string) (*UpstreamRoute, map[string]string, error) {
	candidates := h.configCache.FindCandidates(method)

	for i := range candidates {
		params, ok := matchPath(candidates[i].Path, path)
		if !ok {
			continue
		}

		service, ok := h.configCache.FindService(candidates[i].ServiceID)
		if !ok {
			continue
		}
		serviceInstances := h.configCache.FindInstancesByServiceID(candidates[i].ServiceID)
		if len(serviceInstances) == 0 {
			continue
		}

		matched := upstreamRoutesFromCache(candidates[i], service, serviceInstances)
		instances := make([]loadbalancer.Instance, 0, len(matched))
		for _, route := range matched {
			instances = append(instances, loadbalancer.Instance{
				ID:        route.InstanceID,
				ServiceID: route.ServiceID,
				Host:      route.Host,
				Port:      route.Port,
				Weight:    route.Weight,
			})
		}

		if h.healthFilter != nil {
			instances = h.healthFilter.KeepAlive(ctx, matched[0].ServiceID, instances, matched[0].CircuitBreakerEnabled)
			if len(instances) == 0 {
				continue
			}
		}

		selectedRoute := matched[0]
		selectedRoute.MatchedInstances = len(instances)
		selectedRoute.AvailableInstances = instances

		return &selectedRoute, params, nil
	}

	return nil, nil, ErrRouteNotFound
}

func upstreamRoutesFromCache(route configcache.RouteValue, service configcache.ServiceValue, instances []configcache.InstanceValue) []UpstreamRoute {
	routes := make([]UpstreamRoute, 0, len(instances))
	for _, instance := range instances {
		routes = append(routes, UpstreamRoute{
			RouteID:               route.RouteID,
			RoutePath:             route.Path,
			RouteMethod:           route.Method,
			AuthRequired:          route.AuthRequired,
			RequiredScopeID:       route.RequiredScopeID,
			StripPrefix:           route.StripPrefix,
			RewriteTarget:         route.RewriteTarget,
			RateLimit:             route.RateLimit,
			CORS:                  route.CORS,
			ServiceID:             route.ServiceID,
			ServiceName:           service.Name,
			Protocol:              service.Protocol,
			LBStrategy:            service.LBStrategy,
			InstanceID:            instance.ID,
			Host:                  instance.Host,
			Port:                  instance.Port,
			Weight:                instance.Weight,
			TimeoutMS:             service.TimeoutMS,
			RetryCount:            service.RetryCount,
			CircuitBreakerEnabled: service.CircuitBreakerEnabled,
		})
	}
	return routes
}

func (h *Handler) pickInstance(route UpstreamRoute, instances []loadbalancer.Instance) (loadbalancer.Instance, error) {
	switch route.LBStrategy {
	case "", "round_robin":
		return h.roundRobin.Pick(route.ServiceID, instances)
	case "weighted":
		return h.weighted.Pick(route.ServiceID, instances)
	default:
		return loadbalancer.Instance{}, fmt.Errorf("unsupported load balancing strategy: %s", route.LBStrategy)
	}
}

func normalizedLBStrategy(strategy string) string {
	if strategy == "" {
		return "round_robin"
	}

	return strategy
}

func buildTargetURL(route *UpstreamRoute, requestPath string, params map[string]string, rawQuery string) string {
	targetPath := rewritePath(route, requestPath, params)
	targetURL := fmt.Sprintf("%s://%s:%d%s", route.Protocol, route.Host, route.Port, targetPath)
	if rawQuery != "" {
		targetURL += "?" + rawQuery
	}

	return targetURL
}

func rewritePath(route *UpstreamRoute, requestPath string, params map[string]string) string {
	if route.RewriteTarget != nil && *route.RewriteTarget != "" {
		return fillPathParams(*route.RewriteTarget, params)
	}

	if route.StripPrefix {
		if tail, ok := params["*"]; ok {
			return "/" + tail
		}

		return requestPath
	}

	return requestPath
}

func matchPath(pattern string, path string) (map[string]string, bool) {
	patternParts := splitPath(pattern)
	pathParts := splitPath(path)
	params := map[string]string{}

	for i := range patternParts {
		part := patternParts[i]

		if part == "*" || strings.HasSuffix(part, "...}") {
			if i != len(patternParts)-1 {
				return nil, false
			}

			name := "*"
			if part != "*" {
				name = strings.TrimSuffix(strings.TrimPrefix(part, "{"), "...}")
			}
			tail := strings.Join(pathParts[i:], "/")
			params[name] = tail
			params["*"] = tail
			return params, true
		}

		if i >= len(pathParts) {
			return nil, false
		}

		if strings.HasPrefix(part, "{") && strings.HasSuffix(part, "}") {
			name := strings.TrimSuffix(strings.TrimPrefix(part, "{"), "}")
			params[name] = pathParts[i]
			continue
		}

		if part != pathParts[i] {
			return nil, false
		}
	}

	if len(patternParts) != len(pathParts) {
		return nil, false
	}

	return params, true
}

func splitPath(path string) []string {
	path = strings.Trim(path, "/")
	if path == "" {
		return []string{}
	}

	return strings.Split(path, "/")
}

func fillPathParams(path string, params map[string]string) string {
	for name, value := range params {
		path = strings.ReplaceAll(path, "{"+name+"}", value)
	}

	if strings.HasPrefix(path, "/") {
		return path
	}

	return "/" + path
}
