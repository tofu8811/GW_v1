package proxy

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"gateway-api/helper/response"
	configcache "gateway-api/internal/config/cache"
	"gateway-api/internal/proxy/loadbalancer"

	"github.com/gofiber/fiber/v2"
)

type aggregationRequestTemplate struct {
	Method string `json:"method"`
	Path   string `json:"path"`
}

type aggregationResponseMapping struct {
	Target string `json:"target"`
}

type aggregationError struct {
	Step    string `json:"step"`
	Service string `json:"service"`
	Message string `json:"message"`
}

type aggregationMeta struct {
	Aggregation string             `json:"aggregation"`
	Errors      []aggregationError `json:"errors,omitempty"`
}

func (h *Handler) handleAggregation(c *fiber.Ctx, aggregation *configcache.AggregationValue) error {
	data := map[string]any{}
	meta := aggregationMeta{Aggregation: aggregation.Name}
	completedSteps := map[string]bool{}

	for _, step := range aggregation.Steps {
		if step.DependsOn != nil && !completedSteps[*step.DependsOn] {
			err := aggregationErrorForStep(step, "dependency step did not complete")
			if step.IsRequired {
				return response.Error(c, fiber.StatusBadGateway, "bad_gateway", "required aggregation step failed")
			}
			meta.Errors = append(meta.Errors, err)
			continue
		}

		value, err := h.executeAggregationStep(c, step)
		if err != nil {
			if step.IsRequired {
				h.logger.Warn("required aggregation step failed",
					"request_id", c.GetRespHeader(fiber.HeaderXRequestID),
					"aggregation_id", aggregation.ID,
					"aggregation", aggregation.Name,
					"step_id", step.ID,
					"service_id", step.ServiceID,
					"error", err,
				)
				return response.Error(c, fiber.StatusBadGateway, "bad_gateway", "required aggregation step failed")
			}
			meta.Errors = append(meta.Errors, aggregationErrorForStep(step, err.Error()))
			continue
		}

		target, err := aggregationStepTarget(step)
		if err != nil {
			if step.IsRequired {
				return response.Error(c, fiber.StatusBadGateway, "bad_gateway", "required aggregation step failed")
			}
			meta.Errors = append(meta.Errors, aggregationErrorForStep(step, err.Error()))
			continue
		}

		data[target] = value
		completedSteps[step.ID] = true
	}

	return response.WithMeta(c, data, meta)
}

func (h *Handler) executeAggregationStep(c *fiber.Ctx, step configcache.AggregationStepValue) (any, error) {
	requestTemplate, err := parseAggregationRequestTemplate(step.RequestTemplate)
	if err != nil {
		return nil, err
	}
	service, ok := h.configCache.FindService(step.ServiceID)
	if !ok {
		return nil, fmt.Errorf("upstream service unavailable")
	}
	serviceInstances := h.configCache.FindInstancesByServiceID(step.ServiceID)
	if len(serviceInstances) == 0 {
		return nil, fmt.Errorf("upstream service unavailable")
	}

	instances := make([]loadbalancer.Instance, 0, len(serviceInstances))
	for _, instance := range serviceInstances {
		instances = append(instances, loadbalancer.Instance{ID: instance.ID, ServiceID: instance.ServiceID, Host: instance.Host, Port: instance.Port, Weight: instance.Weight})
	}
	if h.healthFilter != nil {
		instances = h.healthFilter.KeepAlive(c.Context(), service.ID, instances, service.CircuitBreakerEnabled)
		if len(instances) == 0 {
			return nil, fmt.Errorf("upstream service unavailable")
		}
	}

	selected, err := h.pickInstance(UpstreamRoute{ServiceID: service.ID, LBStrategy: service.LBStrategy}, instances)
	if err != nil {
		return nil, err
	}
	targetURL := fmt.Sprintf("%s://%s:%d%s", service.Protocol, selected.Host, selected.Port, requestTemplate.Path)
	timeout := time.Duration(service.TimeoutMS) * time.Millisecond
	if timeout <= 0 {
		timeout = 5 * time.Second
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, requestTemplate.Method, targetURL, nil)
	if err != nil {
		return nil, err
	}
	if requestID := c.GetRespHeader(fiber.HeaderXRequestID); requestID != "" {
		req.Header.Set(fiber.HeaderXRequestID, requestID)
	}

	started := time.Now()
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("upstream service unavailable")
	}
	defer resp.Body.Close()
	if resp.StatusCode >= http.StatusBadRequest {
		return nil, fmt.Errorf("upstream returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var value any
	if err := json.Unmarshal(body, &value); err != nil {
		return nil, fmt.Errorf("upstream response is not valid JSON")
	}

	h.logger.Info("aggregation step completed",
		"request_id", c.GetRespHeader(fiber.HeaderXRequestID),
		"step_id", step.ID,
		"service_id", step.ServiceID,
		"target_url", targetURL,
		"duration_ms", time.Since(started).Milliseconds(),
	)
	return value, nil
}

func parseAggregationRequestTemplate(raw json.RawMessage) (aggregationRequestTemplate, error) {
	var template aggregationRequestTemplate
	if len(raw) == 0 || string(raw) == "null" {
		return template, fmt.Errorf("request_template is required")
	}
	if err := json.Unmarshal(raw, &template); err != nil {
		return template, fmt.Errorf("request_template is invalid")
	}
	template.Method = strings.ToUpper(strings.TrimSpace(template.Method))
	template.Path = strings.TrimSpace(template.Path)
	if template.Method == "" || template.Path == "" {
		return template, fmt.Errorf("request_template must include method and path")
	}
	if !strings.HasPrefix(template.Path, "/") {
		return template, fmt.Errorf("request_template.path must start with /")
	}
	return template, nil
}

func aggregationStepTarget(step configcache.AggregationStepValue) (string, error) {
	var mapping aggregationResponseMapping
	if len(step.ResponseMapping) == 0 || string(step.ResponseMapping) == "null" {
		return "", fmt.Errorf("response_mapping is required")
	}
	if err := json.Unmarshal(step.ResponseMapping, &mapping); err != nil {
		return "", fmt.Errorf("response_mapping is invalid")
	}
	target := strings.TrimSpace(mapping.Target)
	if target == "" {
		return "", fmt.Errorf("response_mapping.target is required")
	}
	return target, nil
}

func aggregationErrorForStep(step configcache.AggregationStepValue, message string) aggregationError {
	return aggregationError{Step: step.ID, Service: step.ServiceID, Message: message}
}
