package proxy

import (
	"errors"
	"time"

	"gateway-api/helper/response"
	appmiddleware "gateway-api/internal/middleware"
	"gateway-api/internal/proxy/loadbalancer"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/proxy"
)

var ErrAllInstancesFailed = errors.New("all upstream instances failed")

func (h *Handler) forwardWithRetry(c *fiber.Ctx, route *UpstreamRoute, requestPath string, params map[string]string, startedAt time.Time) error {
	instances := cloneLBInstances(route.AvailableInstances)
	if len(instances) == 0 {
		return response.Error(c, fiber.StatusServiceUnavailable, "service_unavailable", "no healthy upstream instances available")
	}

	timeout := time.Duration(route.TimeoutMS) * time.Millisecond
	if timeout <= 0 {
		timeout = 5 * time.Second
	}

	maxAttempts := route.RetryCount + 1
	if maxAttempts <= 0 {
		maxAttempts = 1
	}

	var lastErr error
	for attempt := 0; attempt < maxAttempts && len(instances) > 0; attempt++ {
		inst, err := h.pickInstance(*route, instances)
		if err != nil {
			lastErr = err
			break
		}

		br := h.breakerFor(route, inst)
		if br != nil && !br.Allow() {
			instances = excludeInstance(instances, inst.ID)
			attempt--
			continue
		}

		selected := routeForInstance(route, inst)
		targetURL := buildTargetURL(&selected, requestPath, params, string(c.Request().URI().QueryString()))

		h.logger.Info("proxying request to upstream instance",
			"request_id", c.GetRespHeader(fiber.HeaderXRequestID),
			"method", c.Method(),
			"path", requestPath,
			"target_url", targetURL,
			"route_id", route.RouteID,
			"service_id", route.ServiceID,
			"service", route.ServiceName,
			"lb_strategy", normalizedLBStrategy(route.LBStrategy),
			"matched_instances", route.MatchedInstances,
			"instance_id", inst.ID,
			"instance_host", inst.Host,
			"instance_port", inst.Port,
			"instance_weight", inst.Weight,
			"attempt", attempt+1,
			"timeout_ms", timeout.Milliseconds(),
			"client_ip", c.IP(),
		)

		upstreamStartedAt := time.Now()
		err = proxy.DoTimeout(c, targetURL, timeout)
		appmiddleware.AddUpstreamLatency(c, time.Since(upstreamStartedAt))
		if err == nil {
			if br != nil {
				br.OnSuccess()
			}
			h.logger.Info("proxied request completed",
				"request_id", c.GetRespHeader(fiber.HeaderXRequestID),
				"method", c.Method(),
				"path", requestPath,
				"target_url", targetURL,
				"route_id", route.RouteID,
				"service_id", route.ServiceID,
				"service", route.ServiceName,
				"lb_strategy", normalizedLBStrategy(route.LBStrategy),
				"instance_id", inst.ID,
				"instance_host", inst.Host,
				"instance_port", inst.Port,
				"status", c.Response().StatusCode(),
				"duration_ms", time.Since(startedAt).Milliseconds(),
			)
			return nil
		}

		lastErr = err
		if br != nil {
			br.OnFailure()
		}
		h.logger.Warn("failed to proxy request to upstream instance",
			"request_id", c.GetRespHeader(fiber.HeaderXRequestID),
			"error", err,
			"method", c.Method(),
			"path", requestPath,
			"target_url", targetURL,
			"route_id", route.RouteID,
			"service_id", route.ServiceID,
			"instance_id", inst.ID,
			"duration_ms", time.Since(startedAt).Milliseconds(),
		)
		if !isRetryable(c.Method(), err) {
			return response.Error(c, fiber.StatusBadGateway, "bad_gateway", "upstream service unavailable")
		}
		instances = excludeInstance(instances, inst.ID)
	}

	if lastErr != nil {
		h.logger.Error("all upstream attempts failed",
			"request_id", c.GetRespHeader(fiber.HeaderXRequestID),
			"error", lastErr,
			"method", c.Method(),
			"path", requestPath,
			"route_id", route.RouteID,
			"service_id", route.ServiceID,
			"duration_ms", time.Since(startedAt).Milliseconds(),
		)
	}

	return response.Error(c, fiber.StatusServiceUnavailable, "service_unavailable", ErrAllInstancesFailed.Error())
}

func (h *Handler) breakerFor(route *UpstreamRoute, inst loadbalancer.Instance) interface {
	Allow() bool
	OnSuccess()
	OnFailure()
} {
	if !route.CircuitBreakerEnabled || h.breakers == nil {
		return nil
	}
	return h.breakers.Get(inst.ID)
}

func routeForInstance(route *UpstreamRoute, inst loadbalancer.Instance) UpstreamRoute {
	selected := *route
	selected.InstanceID = inst.ID
	selected.Host = inst.Host
	selected.Port = inst.Port
	selected.Weight = inst.Weight
	return selected
}

func excludeInstance(instances []loadbalancer.Instance, instanceID string) []loadbalancer.Instance {
	out := make([]loadbalancer.Instance, 0, len(instances))
	for _, instance := range instances {
		if instance.ID != instanceID {
			out = append(out, instance)
		}
	}
	return out
}

func cloneLBInstances(in []loadbalancer.Instance) []loadbalancer.Instance {
	out := make([]loadbalancer.Instance, len(in))
	copy(out, in)
	return out
}
