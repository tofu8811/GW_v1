package corsconfigs

import (
	"context"
	"errors"
	"strings"

	"gateway-api/helper/idgen"
	"gateway-api/helper/response"
	"gateway-api/helper/validation"

	"github.com/gofiber/fiber/v2"
)

const defaultMaxAge = 3600

type ConfigNotifier interface {
	NotifyChange(ctx context.Context, group string) error
}

type Handler struct {
	repository *Repository
	notifier   ConfigNotifier
}

func NewHandler(repository *Repository, notifier ConfigNotifier) *Handler {
	return &Handler{repository: repository, notifier: notifier}
}

func (h *Handler) FindByRouteID(c *fiber.Ctx) error {
	routeID, err := validation.ParseRequiredUUID("id", c.Params("id"))
	if err != nil {
		return response.BadRequest(c, err.Error())
	}

	config, err := h.repository.FindByRouteID(c.Context(), routeID)
	if errors.Is(err, ErrCORSConfigNotFound) {
		return response.NotFound(c, "CORS config not found")
	}
	if err != nil {
		return response.InternalServerError(c)
	}
	return response.OK(c, toResponse(*config))
}

func (h *Handler) Upsert(c *fiber.Ctx) error {
	routeID, err := validation.ParseRequiredUUID("id", c.Params("id"))
	if err != nil {
		return response.BadRequest(c, err.Error())
	}
	exists, err := h.repository.RouteExists(c.Context(), routeID)
	if err != nil {
		return response.InternalServerError(c)
	}
	if !exists {
		return response.NotFound(c, "route not found")
	}

	var req UpsertCORSConfigRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "invalid request body")
	}

	config, err := normalizeConfig(req)
	if err != nil {
		return response.BadRequest(c, err.Error())
	}
	id, err := idgen.NewUUID()
	if err != nil {
		return response.InternalServerError(c)
	}
	config.ID = id
	config.RouteID = routeID

	if err := h.repository.Upsert(c.Context(), &config); err != nil {
		return response.InternalServerError(c)
	}
	if err := h.notifyChange(c.Context()); err != nil {
		return response.InternalServerError(c)
	}
	return response.OK(c, toResponse(config))
}

func (h *Handler) Delete(c *fiber.Ctx) error {
	routeID, err := validation.ParseRequiredUUID("id", c.Params("id"))
	if err != nil {
		return response.BadRequest(c, err.Error())
	}
	if err := h.repository.DeleteByRouteID(c.Context(), routeID); errors.Is(err, ErrCORSConfigNotFound) {
		return response.NotFound(c, "CORS config not found")
	} else if err != nil {
		return response.InternalServerError(c)
	}
	if err := h.notifyChange(c.Context()); err != nil {
		return response.InternalServerError(c)
	}
	return response.NoContent(c)
}

func (h *Handler) notifyChange(ctx context.Context) error {
	if h.notifier == nil {
		return nil
	}
	return h.notifier.NotifyChange(ctx, "routes")
}

func normalizeConfig(req UpsertCORSConfigRequest) (CORSConfig, error) {
	origins := normalizeUnique(req.AllowedOrigins, false)
	if len(origins) == 0 {
		return CORSConfig{}, validation.FieldError{Field: "allowed_origins", Message: "at least one origin is required"}
	}

	methods := normalizeUnique(req.AllowedMethods, true)
	if len(methods) == 0 {
		return CORSConfig{}, validation.FieldError{Field: "allowed_methods", Message: "at least one method is required"}
	}
	for _, method := range methods {
		if _, err := validation.NormalizeRouteMethod(method, false); err != nil {
			return CORSConfig{}, validation.FieldError{Field: "allowed_methods", Message: "contains an invalid method"}
		}
	}

	if req.AllowCredentials && contains(origins, "*") {
		return CORSConfig{}, validation.FieldError{Field: "allowed_origins", Message: "wildcard origin cannot be used with credentials"}
	}

	maxAge := defaultMaxAge
	if req.MaxAge != nil {
		maxAge = *req.MaxAge
	}
	if err := validation.ValidateIntBetween("max_age", maxAge, 0, 86400); err != nil {
		return CORSConfig{}, err
	}

	return CORSConfig{
		AllowedOrigins:   origins,
		AllowedMethods:   methods,
		AllowedHeaders:   normalizeUnique(req.AllowedHeaders, false),
		AllowCredentials: req.AllowCredentials,
		MaxAge:           maxAge,
	}, nil
}

func normalizeUnique(values []string, upper bool) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if upper {
			value = strings.ToUpper(value)
		}
		if value == "" {
			continue
		}
		key := strings.ToLower(value)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, value)
	}
	return result
}

func contains(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}

func toResponse(config CORSConfig) CORSConfigResponse {
	return CORSConfigResponse{
		ID:               config.ID.String(),
		RouteID:          config.RouteID.String(),
		AllowedOrigins:   config.AllowedOrigins,
		AllowedMethods:   config.AllowedMethods,
		AllowedHeaders:   config.AllowedHeaders,
		AllowCredentials: config.AllowCredentials,
		MaxAge:           config.MaxAge,
	}
}
