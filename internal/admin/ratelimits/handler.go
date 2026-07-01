package ratelimits

import (
	"context"
	"errors"
	"strings"

	"gateway-api/helper/dberror"
	"gateway-api/helper/idgen"
	"gateway-api/helper/pagination"
	"gateway-api/helper/response"
	"gateway-api/helper/validation"

	"github.com/gofiber/fiber/v2"
)

type Handler struct {
	repository *Repository
	notifier   ConfigNotifier
}

type ConfigNotifier interface {
	NotifyChange(ctx context.Context, group string) error
}

func NewHandler(repository *Repository, notifier ConfigNotifier) *Handler {
	return &Handler{repository: repository, notifier: notifier}
}

func (h *Handler) Create(c *fiber.Ctx) error {
	var req CreateRateLimitPolicyRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "invalid request body")
	}

	name, err := normalizeName(req.Name)
	if err != nil {
		return response.BadRequest(c, err.Error())
	}
	limitType, err := normalizeLimitType(req.LimitType)
	if err != nil {
		return response.BadRequest(c, err.Error())
	}
	if err := validation.ValidateIntGreaterThan("max_requests", req.MaxRequests, 0); err != nil {
		return response.BadRequest(c, err.Error())
	}
	if err := validation.ValidateIntGreaterThan("window_seconds", req.WindowSeconds, 0); err != nil {
		return response.BadRequest(c, err.Error())
	}

	id, err := idgen.NewUUID()
	if err != nil {
		return response.InternalServerError(c)
	}

	policy := RateLimitPolicy{
		ID:            id,
		Name:          name,
		LimitType:     limitType,
		MaxRequests:   req.MaxRequests,
		WindowSeconds: req.WindowSeconds,
		IsActive:      boolValue(req.IsActive, true),
	}

	if err := h.repository.Create(c.Context(), &policy); err != nil {
		return handleDBError(c, err)
	}
	if err := h.notifyChange(c); err != nil {
		return response.InternalServerError(c)
	}

	return response.Created(c, toResponse(policy))
}

func (h *Handler) FindAll(c *fiber.Ctx) error {
	p := pagination.FromQuery(c)

	policies, err := h.repository.FindAll(c.Context(), p)
	if err != nil {
		return response.InternalServerError(c)
	}

	responses := make([]RateLimitPolicyResponse, 0, len(policies))
	for _, policy := range policies {
		responses = append(responses, toResponse(policy))
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

	policy, err := h.repository.FindByID(c.Context(), id)
	if errors.Is(err, ErrRateLimitPolicyNotFound) {
		return response.NotFound(c, "rate limit policy not found")
	}
	if err != nil {
		return response.InternalServerError(c)
	}

	return response.OK(c, toResponse(*policy))
}

func (h *Handler) Update(c *fiber.Ctx) error {
	id, err := validation.ParseRequiredUUID("id", c.Params("id"))
	if err != nil {
		return response.BadRequest(c, err.Error())
	}

	policy, err := h.repository.FindByID(c.Context(), id)
	if errors.Is(err, ErrRateLimitPolicyNotFound) {
		return response.NotFound(c, "rate limit policy not found")
	}
	if err != nil {
		return response.InternalServerError(c)
	}

	var req UpdateRateLimitPolicyRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "invalid request body")
	}

	if req.Name != nil {
		name, err := normalizeName(*req.Name)
		if err != nil {
			return response.BadRequest(c, err.Error())
		}
		policy.Name = name
	}
	if req.LimitType != nil {
		limitType, err := normalizeLimitType(*req.LimitType)
		if err != nil {
			return response.BadRequest(c, err.Error())
		}
		policy.LimitType = limitType
	}
	if req.MaxRequests != nil {
		if err := validation.ValidateIntGreaterThan("max_requests", *req.MaxRequests, 0); err != nil {
			return response.BadRequest(c, err.Error())
		}
		policy.MaxRequests = *req.MaxRequests
	}
	if req.WindowSeconds != nil {
		if err := validation.ValidateIntGreaterThan("window_seconds", *req.WindowSeconds, 0); err != nil {
			return response.BadRequest(c, err.Error())
		}
		policy.WindowSeconds = *req.WindowSeconds
	}
	if req.IsActive != nil {
		policy.IsActive = *req.IsActive
	}

	if err := h.repository.Update(c.Context(), policy); err != nil {
		if errors.Is(err, ErrRateLimitPolicyNotFound) {
			return response.NotFound(c, "rate limit policy not found")
		}
		return handleDBError(c, err)
	}
	if err := h.notifyChange(c); err != nil {
		return response.InternalServerError(c)
	}

	return response.OK(c, toResponse(*policy))
}

func (h *Handler) Delete(c *fiber.Ctx) error {
	id, err := validation.ParseRequiredUUID("id", c.Params("id"))
	if err != nil {
		return response.BadRequest(c, err.Error())
	}

	err = h.repository.Delete(c.Context(), id)
	if errors.Is(err, ErrRateLimitPolicyNotFound) {
		return response.NotFound(c, "rate limit policy not found")
	}
	if err != nil {
		return handleDBError(c, err)
	}
	if err := h.notifyChange(c); err != nil {
		return response.InternalServerError(c)
	}

	return response.NoContent(c)
}

func (h *Handler) notifyChange(c *fiber.Ctx) error {
	if h.notifier == nil {
		return nil
	}
	return h.notifier.NotifyChange(c.Context(), "rate_limits")
}

func normalizeName(name string) (string, error) {
	normalized := strings.TrimSpace(name)
	if normalized == "" {
		return "", validation.FieldError{Field: "name", Message: "name is required"}
	}
	return normalized, nil
}

func normalizeLimitType(limitType string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(limitType))
	if err := validation.ValidateEnum("limit_type", normalized, "ip", "user", "api_key"); err != nil {
		return "", err
	}
	return normalized, nil
}

func boolValue(value *bool, defaultValue bool) bool {
	if value == nil {
		return defaultValue
	}
	return *value
}

func toResponse(policy RateLimitPolicy) RateLimitPolicyResponse {
	return RateLimitPolicyResponse{
		ID:            policy.ID.String(),
		Name:          policy.Name,
		LimitType:     policy.LimitType,
		MaxRequests:   policy.MaxRequests,
		WindowSeconds: policy.WindowSeconds,
		IsActive:      policy.IsActive,
		CreatedAt:     policy.CreatedAt,
		UpdatedAt:     policy.UpdatedAt,
	}
}

func handleDBError(c *fiber.Ctx, err error) error {
	if apiErr, ok := dberror.MapDBError(err); ok {
		return response.Error(c, apiErr.Status, apiErr.Code, apiErr.Message)
	}

	return response.InternalServerError(c)
}
