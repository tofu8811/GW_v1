package services

import (
	"context"
	"errors"
	"strings"

	"gateway-api/helper/dberror"
	"gateway-api/helper/idgen"
	"gateway-api/helper/pagination"
	"gateway-api/helper/response"
	"gateway-api/helper/validation"
	configcache "gateway-api/internal/config/cache"
	upstreamhealth "gateway-api/internal/upstream/health"

	"github.com/gofiber/fiber/v2"
)

const (
	defaultProtocol   = "http"
	defaultLBStrategy = "round_robin"
	defaultTimeoutMS  = 5000
)

type Handler struct {
	repository  *Repository
	notifier    ConfigNotifier
	healthStore *upstreamhealth.Store
	configCache ServiceInstanceCache
}

type ConfigNotifier interface {
	NotifyChange(ctx context.Context, group string) error
}

type ServiceInstanceCache interface {
	ActiveInstancesByService(serviceID string) []configcache.ActiveInstanceValue
}

func NewHandler(repository *Repository, notifier ConfigNotifier, healthStore *upstreamhealth.Store, configCache ServiceInstanceCache) *Handler {
	return &Handler{repository: repository, notifier: notifier, healthStore: healthStore, configCache: configCache}
}

func (h *Handler) Create(c *fiber.Ctx) error {
	var req CreateServiceRequest

	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "invalid request body")
	}

	name, err := normalizeServiceName(req.Name)
	if err != nil {
		return response.BadRequest(c, err.Error())
	}

	protocol, err := normalizeProtocolOrDefault(req.Protocol)
	if err != nil {
		return response.BadRequest(c, err.Error())
	}

	lbStrategy, err := normalizeLBStrategyOrDefault(req.LBStrategy)
	if err != nil {
		return response.BadRequest(c, err.Error())
	}

	healthPath := normalizeHealthPath(req.HealthPath)

	timeoutMS := intValue(req.TimeoutMS, defaultTimeoutMS)
	if err := validation.ValidateIntGreaterThan("timeout_ms", timeoutMS, 0); err != nil {
		return response.BadRequest(c, err.Error())
	}

	retryCount := int16Value(req.RetryCount, 0)
	if err := validation.ValidateIntMin("retry_count", int(retryCount), 0); err != nil {
		return response.BadRequest(c, err.Error())
	}

	id, err := idgen.NewUUID()
	if err != nil {
		return response.InternalServerError(c)
	}

	service := Service{
		ID:                    id,
		Name:                  name,
		Description:           stringPtr(req.Description),
		Protocol:              protocol,
		LBStrategy:            lbStrategy,
		HealthPath:            healthPath,
		TimeoutMS:             timeoutMS,
		RetryCount:            retryCount,
		CircuitBreakerEnabled: boolValue(req.CircuitBreakerEnabled, false),
		IsActive:              boolValue(req.IsActive, true),
	}

	if err := h.repository.Create(c.Context(), &service); err != nil {
		return handleDBError(c, err)
	}
	if err := h.notifyChange(c, "services"); err != nil {
		return response.InternalServerError(c)
	}

	return response.Created(c, toResponse(service))
}

func (h *Handler) FindAll(c *fiber.Ctx) error {
	p := pagination.FromQuery(c)

	services, err := h.repository.FindAll(c.Context(), p)
	if err != nil {
		return response.InternalServerError(c)
	}

	responses := make([]ServiceResponse, 0, len(services))
	for _, service := range services {
		responses = append(responses, toResponse(service))
	}

	total, err := h.repository.Count(c.Context())
	if err != nil {
		return response.InternalServerError(c)
	}

	return response.WithMeta(c, responses, pagination.NewMeta(p, total))
}

func (h *Handler) FindByID(c *fiber.Ctx) error {
	id, err := validation.ParseRequiredUUID("id", c.Params("id"))
	if err != nil {
		return response.BadRequest(c, err.Error())
	}

	service, err := h.repository.FindByID(c.Context(), id)
	if errors.Is(err, ErrServiceNotFound) {
		return response.NotFound(c, "service not found")
	}

	if err != nil {
		return response.InternalServerError(c)
	}

	return response.OK(c, toResponse(*service))
}

func (h *Handler) Update(c *fiber.Ctx) error {
	id, err := validation.ParseRequiredUUID("id", c.Params("id"))
	if err != nil {
		return response.BadRequest(c, err.Error())
	}

	service, err := h.repository.FindByID(c.Context(), id)
	if errors.Is(err, ErrServiceNotFound) {
		return response.NotFound(c, "service not found")
	}

	if err != nil {
		return response.InternalServerError(c)
	}

	var req UpdateServiceRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "invalid request body")
	}

	if req.Name != nil {
		name, err := normalizeServiceName(*req.Name)
		if err != nil {
			return response.BadRequest(c, err.Error())
		}
		service.Name = name
	}

	if req.Description != nil {
		service.Description = stringPtr(req.Description)
	}

	if req.Protocol != nil {
		protocol, err := normalizeProtocol(*req.Protocol)
		if err != nil {
			return response.BadRequest(c, err.Error())
		}
		service.Protocol = protocol
	}

	if req.LBStrategy != nil {
		lbStrategy, err := normalizeLBStrategy(*req.LBStrategy)
		if err != nil {
			return response.BadRequest(c, err.Error())
		}
		service.LBStrategy = lbStrategy
	}

	if req.HealthPath != nil {
		service.HealthPath = normalizeHealthPath(*req.HealthPath)
	}

	if req.TimeoutMS != nil {
		if err := validation.ValidateIntGreaterThan("timeout_ms", *req.TimeoutMS, 0); err != nil {
			return response.BadRequest(c, err.Error())
		}
		service.TimeoutMS = *req.TimeoutMS
	}

	if req.RetryCount != nil {
		if err := validation.ValidateIntMin("retry_count", int(*req.RetryCount), 0); err != nil {
			return response.BadRequest(c, err.Error())
		}
		service.RetryCount = *req.RetryCount
	}

	if req.CircuitBreakerEnabled != nil {
		service.CircuitBreakerEnabled = *req.CircuitBreakerEnabled
	}

	if req.IsActive != nil {
		service.IsActive = *req.IsActive
	}

	if err := h.repository.Update(c.Context(), service); err != nil {
		if errors.Is(err, ErrServiceNotFound) {
			return response.NotFound(c, "service not found")
		}
		return handleDBError(c, err)
	}
	if err := h.notifyChange(c, "services"); err != nil {
		return response.InternalServerError(c)
	}

	return response.OK(c, toResponse(*service))
}

func (h *Handler) Delete(c *fiber.Ctx) error {
	id, err := validation.ParseRequiredUUID("id", c.Params("id"))
	if err != nil {
		return response.BadRequest(c, err.Error())
	}

	err = h.repository.Delete(c.Context(), id)
	if errors.Is(err, ErrServiceNotFound) {
		return response.NotFound(c, "service not found")
	}

	if err != nil {
		return handleDBError(c, err)
	}
	if err := h.notifyChange(c, "services"); err != nil {
		return response.InternalServerError(c)
	}

	return response.NoContent(c)
}

func (h *Handler) notifyChange(c *fiber.Ctx, group string) error {
	if h.notifier == nil {
		return nil
	}
	return h.notifier.NotifyChange(c.Context(), group)
}

func normalizeServiceName(name string) (string, error) {
	normalized := strings.TrimSpace(name)
	if normalized == "" {
		return "", validation.FieldError{Field: "name", Message: "name is required"}
	}

	return normalized, nil
}

func normalizeProtocolOrDefault(protocol string) (string, error) {
	if strings.TrimSpace(protocol) == "" {
		protocol = defaultProtocol
	}

	return normalizeProtocol(protocol)
}

func normalizeProtocol(protocol string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(protocol))
	if err := validation.ValidateEnum("protocol", normalized, "http", "grpc"); err != nil {
		return "", err
	}

	return normalized, nil
}

func normalizeLBStrategyOrDefault(strategy string) (string, error) {
	if strings.TrimSpace(strategy) == "" {
		strategy = defaultLBStrategy
	}

	return normalizeLBStrategy(strategy)
}

func normalizeLBStrategy(strategy string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(strategy))
	if err := validation.ValidateEnum("lb_strategy", normalized, "round_robin", "weighted"); err != nil {
		return "", err
	}

	return normalized, nil
}

func normalizeHealthPath(path string) string {
	normalized := strings.TrimSpace(path)
	if normalized == "" {
		return ""
	}
	if strings.HasPrefix(normalized, "/") {
		return normalized
	}

	return "/" + normalized
}

func boolValue(value *bool, defaultValue bool) bool {
	if value == nil {
		return defaultValue
	}
	return *value
}

func intValue(value *int, defaultValue int) int {
	if value == nil {
		return defaultValue
	}
	return *value
}

func int16Value(value *int16, defaultValue int16) int16 {
	if value == nil {
		return defaultValue
	}
	return *value
}

func stringPtr(value *string) *string {
	if value == nil || strings.TrimSpace(*value) == "" {
		return nil
	}

	trimmed := strings.TrimSpace(*value)
	return &trimmed
}

func toResponse(service Service) ServiceResponse {
	return ServiceResponse{
		ID:                    service.ID.String(),
		Name:                  service.Name,
		Description:           service.Description,
		Protocol:              service.Protocol,
		LBStrategy:            service.LBStrategy,
		HealthPath:            service.HealthPath,
		TimeoutMS:             service.TimeoutMS,
		RetryCount:            service.RetryCount,
		CircuitBreakerEnabled: service.CircuitBreakerEnabled,
		IsActive:              service.IsActive,
		CreatedAt:             service.CreatedAt,
		UpdatedAt:             service.UpdatedAt,
	}
}

func handleDBError(c *fiber.Ctx, err error) error {
	if apiErr, ok := dberror.MapDBError(err); ok {
		return response.Error(c, apiErr.Status, apiErr.Code, apiErr.Message)
	}

	return response.InternalServerError(c)
}
