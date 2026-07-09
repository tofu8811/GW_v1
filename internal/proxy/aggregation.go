package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"gateway-api/helper/response"
	configcache "gateway-api/internal/config/cache"
	appmiddleware "gateway-api/internal/middleware"
	"gateway-api/internal/proxy/loadbalancer"

	"github.com/gofiber/fiber/v2"
)

const (
	defaultAggregationTotalTimeout = 30 * time.Second
	defaultStepTimeout             = 5 * time.Second
	maxAggregationStepBodyBytes    = 1 << 20
	maxAggregationTotalBodyBytes   = 4 << 20
)

var internalIdentityHeaders = map[string]struct{}{
	"x-user-id":   {},
	"x-client-id": {},
	"x-scopes":    {},
}

type aggregationRequestTemplate struct {
	Method         string            `json:"method"`
	Path           string            `json:"path"`
	ForwardQuery   bool              `json:"forward_query"`
	ForwardBody    bool              `json:"forward_body"`
	Headers        map[string]string `json:"headers"`
	ForwardHeaders []string          `json:"forward_headers"`
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
	ctx, cancel := context.WithTimeout(context.Background(), defaultAggregationTotalTimeout)
	defer cancel()

	data := map[string]any{}
	meta := aggregationMeta{Aggregation: aggregation.Name}
	completedSteps := map[string]bool{}

	for _, step := range aggregation.Steps {
		if err := ctx.Err(); err != nil {
			return response.Error(c, fiber.StatusGatewayTimeout, "gateway_timeout", "aggregation timed out")
		}
		if step.DependsOn != nil && !completedSteps[*step.DependsOn] {
			err := aggregationErrorForStep(step, "dependency step did not complete")
			if step.IsRequired {
				return response.Error(c, fiber.StatusBadGateway, "bad_gateway", "required aggregation step failed")
			}
			meta.Errors = append(meta.Errors, err)
			continue
		}

		value, err := h.executeAggregationStep(ctx, c, step)
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
				if errors.Is(err, context.DeadlineExceeded) {
					return response.Error(c, fiber.StatusGatewayTimeout, "gateway_timeout", "required aggregation step timed out")
				}
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
		if exceedsJSONSize(data, maxAggregationTotalBodyBytes) {
			return response.Error(c, fiber.StatusBadGateway, "bad_gateway", "aggregation response is too large")
		}
		completedSteps[step.ID] = true
	}

	return response.WithMeta(c, data, meta)
}

func (h *Handler) executeAggregationStep(parent context.Context, c *fiber.Ctx, step configcache.AggregationStepValue) (any, error) {
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
		instances = h.healthFilter.KeepAlive(parent, service.ID, instances, service.CircuitBreakerEnabled)
		if len(instances) == 0 {
			return nil, fmt.Errorf("upstream service unavailable")
		}
	}

	selected, err := h.pickInstance(UpstreamRoute{ServiceID: service.ID, LBStrategy: service.LBStrategy}, instances)
	if err != nil {
		return nil, err
	}
	targetPath := fillPathParams(requestTemplate.Path, pathParamsFromLocals(c))
	targetURL := fmt.Sprintf("%s://%s:%d%s", service.Protocol, selected.Host, selected.Port, targetPath)
	if requestTemplate.ForwardQuery {
		if rawQuery := string(c.Request().URI().QueryString()); rawQuery != "" {
			targetURL += "?" + rawQuery
		}
	}
	timeout := time.Duration(service.TimeoutMS) * time.Millisecond
	if timeout <= 0 {
		timeout = defaultStepTimeout
	}
	if deadline, ok := parent.Deadline(); ok {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return nil, context.DeadlineExceeded
		}
		if remaining < timeout {
			timeout = remaining
		}
	}

	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()

	var body io.Reader
	if requestTemplate.ForwardBody && methodAllowsBody(requestTemplate.Method) {
		body = bytes.NewReader(c.BodyRaw())
	}
	req, err := http.NewRequestWithContext(ctx, requestTemplate.Method, targetURL, body)
	if err != nil {
		return nil, err
	}
	applyAggregationHeaders(c, req, requestTemplate)

	started := time.Now()
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) || errors.Is(parent.Err(), context.DeadlineExceeded) {
			return nil, context.DeadlineExceeded
		}
		return nil, fmt.Errorf("upstream service unavailable")
	}
	defer resp.Body.Close()
	if resp.StatusCode >= http.StatusBadRequest {
		return nil, fmt.Errorf("upstream returned status %d", resp.StatusCode)
	}

	bodyBytes, err := readLimited(resp.Body, maxAggregationStepBodyBytes)
	if err != nil {
		return nil, err
	}
	var value any
	if err := json.Unmarshal(bodyBytes, &value); err != nil {
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

func readLimited(reader io.Reader, maxBytes int64) ([]byte, error) {
	limited := io.LimitReader(reader, maxBytes+1)
	body, err := io.ReadAll(limited)
	if err != nil {
		return nil, err
	}
	if int64(len(body)) > maxBytes {
		return nil, fmt.Errorf("upstream response is too large")
	}
	return body, nil
}

func exceedsJSONSize(value any, maxBytes int64) bool {
	body, err := json.Marshal(value)
	return err == nil && int64(len(body)) > maxBytes
}

func methodAllowsBody(method string) bool {
	switch strings.ToUpper(method) {
	case http.MethodPost, http.MethodPut, http.MethodPatch:
		return true
	default:
		return false
	}
}

func applyAggregationHeaders(c *fiber.Ctx, req *http.Request, template aggregationRequestTemplate) {
	for name, value := range template.Headers {
		if isInternalIdentityHeader(name) || isHopByHopHeader(name) {
			continue
		}
		req.Header.Set(name, value)
	}
	for _, name := range template.ForwardHeaders {
		name = strings.TrimSpace(name)
		if name == "" || isInternalIdentityHeader(name) || isHopByHopHeader(name) {
			continue
		}
		if value := c.Get(name); value != "" {
			req.Header.Set(name, value)
		}
	}
	if requestID := c.GetRespHeader(fiber.HeaderXRequestID); requestID != "" {
		req.Header.Set(fiber.HeaderXRequestID, requestID)
	}
	setInternalIdentityHeaders(c, req)
}

func setInternalIdentityHeaders(c *fiber.Ctx, req *http.Request) {
	if userID, ok := c.Locals(appmiddleware.LocalsUserID).(string); ok && strings.TrimSpace(userID) != "" {
		req.Header.Set("X-User-ID", userID)
	}
	if apiKeyID, ok := c.Locals(appmiddleware.LocalsAPIKeyID).(string); ok && strings.TrimSpace(apiKeyID) != "" {
		req.Header.Set("X-Client-ID", apiKeyID)
	}
	if scopes, ok := c.Locals(appmiddleware.LocalsAPIKeyScopes).([]string); ok && len(scopes) > 0 {
		req.Header.Set("X-Scopes", strings.Join(scopes, ","))
	}
}

func isInternalIdentityHeader(name string) bool {
	_, ok := internalIdentityHeaders[strings.ToLower(strings.TrimSpace(name))]
	return ok
}

func isHopByHopHeader(name string) bool {
	for _, header := range hopByHopHeaders {
		if strings.EqualFold(header, strings.TrimSpace(name)) {
			return true
		}
	}
	return false
}

func pathParamsFromLocals(c *fiber.Ctx) map[string]string {
	if params, ok := c.Locals("aggregation_path_params").(map[string]string); ok {
		return params
	}
	return nil
}
