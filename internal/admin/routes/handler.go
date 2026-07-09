package routes

import (
	"context"
	"errors"

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
	var req CreateRouteRequest

	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "invalid request body")
	}

	path, err := validation.NormalizePath(req.Path)
	if err != nil {
		return response.BadRequest(c, err.Error())
	}

	serviceID, err := validation.ParseRequiredUUID("service_id", req.ServiceID)
	if err != nil {
		return response.BadRequest(c, err.Error())
	}

	id, err := idgen.NewUUID()
	if err != nil {
		return response.InternalServerError(c)
	}

	method, err := validation.NormalizeRouteMethodOrDefault(req.Method, validation.DefaultRouteMethod, true)
	if err != nil {
		return response.BadRequest(c, err.Error())
	}

	requiredScopeID, err := validation.ParseOptionalUUID("required_scope_id", req.RequiredScopeID)
	if err != nil {
		return response.BadRequest(c, err.Error())
	}
	authRequired := boolValue(req.AuthRequired, true)
	if requiredScopeID != nil && !authRequired {
		return response.BadRequest(c, "auth_required must be true when required_scope_id is set")
	}

	rateLimitID, err := validation.ParseOptionalUUID("rate_limit_id", req.RateLimitID)
	if err != nil {
		return response.BadRequest(c, err.Error())
	}

	corsPolicyID, err := validation.ParseOptionalUUID("cors_policy_id", req.CORSPolicyID)
	if err != nil {
		return response.BadRequest(c, err.Error())
	}

	route := Route{
		ID:              id,
		Path:            path,
		Method:          method,
		ServiceID:       serviceID,
		StripPrefix:     boolValue(req.StripPrefix, false),
		RewriteTarget:   stringPtr(req.RewriteTarget),
		AuthRequired:    authRequired,
		RequiredScopeID: requiredScopeID,
		RateLimitID:     rateLimitID,
		CORSPolicyID:    corsPolicyID,
		Priority:        intValue(req.Priority, 0),
		IsActive:        boolValue(req.IsActive, true),
	}

	if err := h.repository.Create(c.Context(), &route); err != nil {
		return handleDBError(c, err)
	}
	if err := h.notifyChange(c, "routes"); err != nil {
		return response.InternalServerError(c)
	}

	return response.Created(c, toResponse(route))
}

func (h *Handler) FindAll(c *fiber.Ctx) error {
	p := pagination.FromQuery(c)

	routes, err := h.repository.FindAll(c.Context(), p)
	if err != nil {
		return response.InternalServerError(c)
	}

	responses := make([]RouteResponse, 0, len(routes))
	for _, route := range routes {
		responses = append(responses, toResponse(route))
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

	route, err := h.repository.FindByID(c.Context(), id)
	if errors.Is(err, ErrRouteNotFound) {
		return response.NotFound(c, "route not found")
	}

	if err != nil {
		return response.InternalServerError(c)
	}

	return response.OK(c, toResponse(*route))
}

func (h *Handler) Update(c *fiber.Ctx) error {
	id, err := validation.ParseRequiredUUID("id", c.Params("id"))
	if err != nil {
		return response.BadRequest(c, err.Error())
	}

	route, err := h.repository.FindByID(c.Context(), id)
	if errors.Is(err, ErrRouteNotFound) {
		return response.NotFound(c, "route not found")
	}

	if err != nil {
		return response.InternalServerError(c)
	}

	var req UpdateRouteRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "invalid request body")
	}

	if req.Path != nil {
		path, err := validation.NormalizePath(*req.Path)
		if err != nil {
			return response.BadRequest(c, err.Error())
		}
		route.Path = path
	}

	if req.Method != nil {
		method, err := validation.NormalizeRouteMethod(*req.Method, true)
		if err != nil {
			return response.BadRequest(c, err.Error())
		}
		route.Method = method
	}

	if req.ServiceID != nil {
		serviceID, err := validation.ParseRequiredUUID("service_id", *req.ServiceID)
		if err != nil {
			return response.BadRequest(c, err.Error())
		}
		route.ServiceID = serviceID
	}

	if req.StripPrefix != nil {
		route.StripPrefix = *req.StripPrefix
	}

	if req.RewriteTarget != nil {
		route.RewriteTarget = stringPtr(req.RewriteTarget)
	}

	if req.AuthRequired != nil {
		route.AuthRequired = *req.AuthRequired
	}

	if req.RequiredScopeID.Set {
		requiredScopeID, err := validation.ParseOptionalUUID("required_scope_id", req.RequiredScopeID.Value)
		if err != nil {
			return response.BadRequest(c, err.Error())
		}
		route.RequiredScopeID = requiredScopeID
	}
	if route.RequiredScopeID != nil && !route.AuthRequired {
		return response.BadRequest(c, "auth_required must be true when required_scope_id is set")
	}

	if req.RateLimitID != nil {
		rateLimitID, err := validation.ParseOptionalUUID("rate_limit_id", req.RateLimitID)
		if err != nil {
			return response.BadRequest(c, err.Error())
		}
		route.RateLimitID = rateLimitID
	}

	if req.CORSPolicyID.Set {
		corsPolicyID, err := validation.ParseOptionalUUID("cors_policy_id", req.CORSPolicyID.Value)
		if err != nil {
			return response.BadRequest(c, err.Error())
		}
		route.CORSPolicyID = corsPolicyID
	}

	if req.Priority != nil {
		route.Priority = *req.Priority
	}

	if req.IsActive != nil {
		route.IsActive = *req.IsActive
	}

	if err := h.repository.Update(c.Context(), route); err != nil {
		if errors.Is(err, ErrRouteNotFound) {
			return response.NotFound(c, "route not found")
		}
		return handleDBError(c, err)
	}
	if err := h.notifyChange(c, "routes"); err != nil {
		return response.InternalServerError(c)
	}

	return response.OK(c, toResponse(*route))
}

func (h *Handler) Delete(c *fiber.Ctx) error {
	id, err := validation.ParseRequiredUUID("id", c.Params("id"))
	if err != nil {
		return response.BadRequest(c, err.Error())
	}

	err = h.repository.Delete(c.Context(), id)
	if errors.Is(err, ErrRouteNotFound) {
		return response.NotFound(c, "route not found")
	}

	if err != nil {
		return handleDBError(c, err)
	}
	if err := h.notifyChange(c, "routes"); err != nil {
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

func stringPtr(value *string) *string {
	if value == nil || *value == "" {
		return nil
	}

	return value
}

func toResponse(route Route) RouteResponse {
	var requiredScopeID *string
	if route.RequiredScopeID != nil {
		value := route.RequiredScopeID.String()
		requiredScopeID = &value
	}
	var rateLimitID *string
	if route.RateLimitID != nil {
		value := route.RateLimitID.String()
		rateLimitID = &value
	}
	var corsPolicyID *string
	if route.CORSPolicyID != nil {
		value := route.CORSPolicyID.String()
		corsPolicyID = &value
	}
	var corsPolicy *CORSPolicySummaryResponse
	if route.CORSPolicy != nil {
		corsPolicy = &CORSPolicySummaryResponse{
			ID:               route.CORSPolicy.ID.String(),
			Name:             route.CORSPolicy.Name,
			AllowedOrigins:   route.CORSPolicy.AllowedOrigins,
			AllowedMethods:   route.CORSPolicy.AllowedMethods,
			AllowedHeaders:   route.CORSPolicy.AllowedHeaders,
			ExposedHeaders:   route.CORSPolicy.ExposedHeaders,
			AllowCredentials: route.CORSPolicy.AllowCredentials,
			MaxAge:           route.CORSPolicy.MaxAge,
		}
	}

	return RouteResponse{
		ID:              route.ID.String(),
		Path:            route.Path,
		Method:          route.Method,
		ServiceID:       route.ServiceID.String(),
		StripPrefix:     route.StripPrefix,
		RewriteTarget:   route.RewriteTarget,
		AuthRequired:    route.AuthRequired,
		RequiredScopeID: requiredScopeID,
		RateLimitID:     rateLimitID,
		CORSPolicyID:    corsPolicyID,
		CORSPolicy:      corsPolicy,
		Priority:        route.Priority,
		IsActive:        route.IsActive,
		CreatedAt:       route.CreatedAt,
		UpdatedAt:       route.UpdatedAt,
	}
}
func handleDBError(c *fiber.Ctx, err error) error {
	if errors.Is(err, ErrRequiredScopeUnavailable) || errors.Is(err, ErrRequiredScopeServiceMismatch) || errors.Is(err, ErrCORSPolicyUnavailable) {
		return response.Error(c, fiber.StatusUnprocessableEntity, "invalid_reference", err.Error())
	}
	if apiErr, ok := dberror.MapDBError(err); ok {
		return response.Error(c, apiErr.Status, apiErr.Code, apiErr.Message)
	}

	return response.InternalServerError(c)
}
