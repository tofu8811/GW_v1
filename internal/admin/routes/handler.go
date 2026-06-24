package routes

import (
	"errors"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
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
		return c.Status(400).JSON(fiber.Map{"message": "invalid request body"})
	}

	if req.Path == "" || req.ServiceID == "" {
		return c.Status(400).JSON(fiber.Map{"message": "path and service_id are required"})
	}

	serviceID, err := uuid.Parse(req.ServiceID)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"message": "invalid service_id"})
	}

	id, err := uuid.NewV7()
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"message": "cannot generate route id"})
	}

	method := strings.ToUpper(req.Method)
	if method == "" {
		method = "GET"
	}

	if !isValidMethod(method) {
		return c.Status(400).JSON(fiber.Map{"message": "invalid method"})
	}

	rateLimitID, err := uuidPtr(req.RateLimitID)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"message": "invalid rate_limit_id"})
	}

	route := Route{
		ID:            id,
		Path:          req.Path,
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
		return c.Status(500).JSON(fiber.Map{"message": "cannot create route"})
	}

	return c.Status(201).JSON(toResponse(route))
}

func (h *Handler) FindAll(c *fiber.Ctx) error {
	routes, err := h.repository.FindAll(c.Context())
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"message": "cannot get routes"})
	}

	responses := make([]RouteResponse, 0, len(routes))
	for _, route := range routes {
		responses = append(responses, toResponse(route))
	}

	return c.JSON(responses)
}

func (h *Handler) FindByID(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"message": "invalid route id"})
	}

	route, err := h.repository.FindByID(c.Context(), id)
	if errors.Is(err, ErrRouteNotFound) {
		return c.Status(404).JSON(fiber.Map{"message": "route not found"})
	}

	if err != nil {
		return c.Status(500).JSON(fiber.Map{"message": "cannot get route"})
	}

	return c.JSON(toResponse(*route))
}

func (h *Handler) Update(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"message": "invalid route id"})
	}

	route, err := h.repository.FindByID(c.Context(), id)
	if errors.Is(err, ErrRouteNotFound) {
		return c.Status(404).JSON(fiber.Map{"message": "route not found"})
	}

	if err != nil {
		return c.Status(500).JSON(fiber.Map{"message": "cannot get route"})
	}

	var req UpdateRouteRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"message": "invalid request body"})
	}

	if req.Path != nil {
		route.Path = *req.Path
	}

	if req.Method != nil {
		method := strings.ToUpper(*req.Method)
		if !isValidMethod(method) {
			return c.Status(400).JSON(fiber.Map{"message": "invalid method"})
		}
		route.Method = method
	}

	if req.ServiceID != nil {
		serviceID, err := uuid.Parse(*req.ServiceID)
		if err != nil {
			return c.Status(400).JSON(fiber.Map{"message": "invalid service_id"})
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
		rateLimitID, err := uuidPtr(req.RateLimitID)
		if err != nil {
			return c.Status(400).JSON(fiber.Map{"message": "invalid rate_limit_id"})
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
		return c.Status(500).JSON(fiber.Map{"message": "cannot update route"})
	}

	return c.JSON(toResponse(*route))
}

func (h *Handler) Delete(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"message": "invalid route id"})
	}

	err = h.repository.Delete(c.Context(), id)
	if errors.Is(err, ErrRouteNotFound) {
		return c.Status(404).JSON(fiber.Map{"message": "route not found"})
	}

	if err != nil {
		return c.Status(500).JSON(fiber.Map{"message": "cannot delete route"})
	}

	return c.JSON(fiber.Map{"message": "delete route successfully"})
}

func isValidMethod(method string) bool {
	switch method {
	case "GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS", "ANY":
		return true
	default:
		return false
	}
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

func uuidPtr(value *string) (*uuid.UUID, error) {
	if value == nil || *value == "" {
		return nil, nil
	}

	id, err := uuid.Parse(*value)
	if err != nil {
		return nil, err
	}

	return &id, nil
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
