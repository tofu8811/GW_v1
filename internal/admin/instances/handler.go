package instances

import (
	"context"
	"errors"
	"strings"

	"gateway-api/helper/dberror"
	"gateway-api/helper/idgen"
	"gateway-api/helper/pagination"
	"gateway-api/helper/response"
	"gateway-api/helper/validation"
	upstreamhealth "gateway-api/internal/upstream/health"

	"github.com/gofiber/fiber/v2"
)

const defaultWeight int16 = 1

type Handler struct {
	repository    *Repository
	notifier      ConfigNotifier
	healthStore   *upstreamhealth.Store
	healthChecker HealthChecker
}

type ConfigNotifier interface {
	NotifyChange(ctx context.Context, group string) error
}

type HealthChecker interface {
	CheckInstance(ctx context.Context, instanceID string) (upstreamhealth.InstanceHealth, error)
}

func NewHandler(repository *Repository, notifier ConfigNotifier, healthStore *upstreamhealth.Store, healthChecker HealthChecker) *Handler {
	return &Handler{repository: repository, notifier: notifier, healthStore: healthStore, healthChecker: healthChecker}
}

func (h *Handler) CreateForService(c *fiber.Ctx) error {
	serviceID, err := validation.ParseRequiredUUID("service_id", c.Params("id"))
	if err != nil {
		return response.BadRequest(c, err.Error())
	}

	var req CreateInstanceRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "invalid request body")
	}

	host, err := normalizeHost(req.Host)
	if err != nil {
		return response.BadRequest(c, err.Error())
	}

	if err := validation.ValidateIntBetween("port", req.Port, 1, 65535); err != nil {
		return response.BadRequest(c, err.Error())
	}

	weight := int16Value(req.Weight, defaultWeight)
	if err := validation.ValidateIntMin("weight", int(weight), 1); err != nil {
		return response.BadRequest(c, err.Error())
	}

	id, err := idgen.NewUUID()
	if err != nil {
		return response.InternalServerError(c)
	}

	instance := ServiceInstance{
		ID:        id,
		ServiceID: serviceID,
		Host:      host,
		Port:      req.Port,
		Weight:    weight,
		IsActive:  boolValue(req.IsActive, true),
	}

	if err := h.repository.Create(c.Context(), &instance); err != nil {
		return handleDBError(c, err)
	}
	if err := h.notifyChange(c, "services"); err != nil {
		return response.InternalServerError(c)
	}

	return response.Created(c, toResponse(instance))
}

func (h *Handler) FindByServiceID(c *fiber.Ctx) error {
	serviceID, err := validation.ParseRequiredUUID("service_id", c.Params("id"))
	if err != nil {
		return response.BadRequest(c, err.Error())
	}

	p := pagination.FromQuery(c)

	instances, err := h.repository.FindByServiceID(c.Context(), serviceID, p)
	if err != nil {
		return response.InternalServerError(c)
	}

	responses := make([]InstanceResponse, 0, len(instances))
	for _, instance := range instances {
		responses = append(responses, toResponse(instance))
	}

	total, err := h.repository.CountByServiceID(c.Context(), serviceID)
	if err != nil {
		return response.InternalServerError(c)
	}

	return response.WithMeta(c, responses, pagination.NewMeta(p, total))
}

func (h *Handler) FindAll(c *fiber.Ctx) error {
	p := pagination.FromQuery(c)

	instances, err := h.repository.FindAll(c.Context(), p)
	if err != nil {
		return response.InternalServerError(c)
	}

	responses := make([]InstanceResponse, 0, len(instances))
	for _, instance := range instances {
		responses = append(responses, toResponse(instance))
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

	instance, err := h.repository.FindByID(c.Context(), id)
	if errors.Is(err, ErrInstanceNotFound) {
		return response.NotFound(c, "service instance not found")
	}

	if err != nil {
		return response.InternalServerError(c)
	}

	return response.OK(c, toResponse(*instance))
}

func (h *Handler) Update(c *fiber.Ctx) error {
	id, err := validation.ParseRequiredUUID("id", c.Params("id"))
	if err != nil {
		return response.BadRequest(c, err.Error())
	}

	instance, err := h.repository.FindByID(c.Context(), id)
	if errors.Is(err, ErrInstanceNotFound) {
		return response.NotFound(c, "service instance not found")
	}

	if err != nil {
		return response.InternalServerError(c)
	}

	var req UpdateInstanceRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "invalid request body")
	}

	if req.ServiceID != nil {
		serviceID, err := validation.ParseRequiredUUID("service_id", *req.ServiceID)
		if err != nil {
			return response.BadRequest(c, err.Error())
		}
		instance.ServiceID = serviceID
	}

	if req.Host != nil {
		host, err := normalizeHost(*req.Host)
		if err != nil {
			return response.BadRequest(c, err.Error())
		}
		instance.Host = host
	}

	if req.Port != nil {
		if err := validation.ValidateIntBetween("port", *req.Port, 1, 65535); err != nil {
			return response.BadRequest(c, err.Error())
		}
		instance.Port = *req.Port
	}

	if req.Weight != nil {
		if err := validation.ValidateIntMin("weight", int(*req.Weight), 1); err != nil {
			return response.BadRequest(c, err.Error())
		}
		instance.Weight = *req.Weight
	}

	if req.IsActive != nil {
		instance.IsActive = *req.IsActive
	}

	if err := h.repository.Update(c.Context(), instance); err != nil {
		if errors.Is(err, ErrInstanceNotFound) {
			return response.NotFound(c, "service instance not found")
		}
		return handleDBError(c, err)
	}
	if err := h.notifyChange(c, "services"); err != nil {
		return response.InternalServerError(c)
	}

	return response.OK(c, toResponse(*instance))
}

func (h *Handler) Delete(c *fiber.Ctx) error {
	id, err := validation.ParseRequiredUUID("id", c.Params("id"))
	if err != nil {
		return response.BadRequest(c, err.Error())
	}

	err = h.repository.Delete(c.Context(), id)
	if errors.Is(err, ErrInstanceNotFound) {
		return response.NotFound(c, "service instance not found")
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

func normalizeHost(host string) (string, error) {
	normalized := strings.TrimSpace(host)
	if normalized == "" {
		return "", validation.FieldError{Field: "host", Message: "host is required"}
	}

	return normalized, nil
}

func boolValue(value *bool, defaultValue bool) bool {
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

func toResponse(instance ServiceInstance) InstanceResponse {
	return InstanceResponse{
		ID:        instance.ID.String(),
		ServiceID: instance.ServiceID.String(),
		Host:      instance.Host,
		Port:      instance.Port,
		Weight:    instance.Weight,
		IsActive:  instance.IsActive,
		CreatedAt: instance.CreatedAt,
	}
}

func handleDBError(c *fiber.Ctx, err error) error {
	if apiErr, ok := dberror.MapDBError(err); ok {
		return response.Error(c, apiErr.Status, apiErr.Code, apiErr.Message)
	}

	return response.InternalServerError(c)
}
