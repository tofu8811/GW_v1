package routes

import (
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
}

func NewHandler(repository *Repository) *Handler {
	return &Handler{repository: repository}
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

	rateLimitID, err := validation.ParseOptionalUUID("rate_limit_id", req.RateLimitID)
	if err != nil {
		return response.BadRequest(c, err.Error())
	}

	route := Route{
		ID:            id,
		Path:          path,
		Method:        method,
		ServiceID:     serviceID,
		StripPrefix:   boolValue(req.StripPrefix, false),
		RewriteTarget: stringPtr(req.RewriteTarget),
		AuthRequired:  boolValue(req.AuthRequired, true),
		RateLimitID:   rateLimitID,
		Priority:      intValue(req.Priority, 0),
		IsActive:      boolValue(req.IsActive, true),
	}

	if err := h.repository.Create(c.Context(), &route); err != nil {
		return handleDBError(c, err)
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

	if req.RateLimitID != nil {
		rateLimitID, err := validation.ParseOptionalUUID("rate_limit_id", req.RateLimitID)
		if err != nil {
			return response.BadRequest(c, err.Error())
		}
		route.RateLimitID = rateLimitID
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

	return response.NoContent(c)
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
	var rateLimitID *string
	if route.RateLimitID != nil {
		value := route.RateLimitID.String()
		rateLimitID = &value
	}

	return RouteResponse{
		ID:            route.ID.String(),
		Path:          route.Path,
		Method:        route.Method,
		ServiceID:     route.ServiceID.String(),
		StripPrefix:   route.StripPrefix,
		RewriteTarget: route.RewriteTarget,
		AuthRequired:  route.AuthRequired,
		RateLimitID:   rateLimitID,
		Priority:      route.Priority,
		IsActive:      route.IsActive,
		CreatedAt:     route.CreatedAt,
		UpdatedAt:     route.UpdatedAt,
	}
}

func handleDBError(c *fiber.Ctx, err error) error {
	if apiErr, ok := dberror.MapDBError(err); ok {
		return response.Error(c, apiErr.Status, apiErr.Code, apiErr.Message)
	}

	return response.InternalServerError(c)
}
