package proxy

import (
	"errors"
	"strconv"
	"strings"

	"gateway-api/helper/response"
	configcache "gateway-api/internal/config/cache"

	"github.com/gofiber/fiber/v2"
)

var (
	errCORSNotConfigured = errors.New("CORS is not configured for this route")
	errCORSOriginDenied  = errors.New("origin is not allowed")
	errCORSMethodDenied  = errors.New("method is not allowed by CORS policy")
	errCORSHeaderDenied  = errors.New("request header is not allowed by CORS policy")
)

func (h *Handler) handleCORSPreflight(c *fiber.Ctx, path string) error {
	requestedMethod := strings.ToUpper(strings.TrimSpace(c.Get(fiber.HeaderAccessControlRequestMethod)))
	origin := strings.TrimSpace(c.Get(fiber.HeaderOrigin))
	if requestedMethod == "" || origin == "" {
		return response.BadRequest(c, "invalid CORS preflight request")
	}

	route, err := h.findRouteForCORS(path, requestedMethod)
	if errors.Is(err, ErrRouteNotFound) {
		return response.NotFound(c, "gateway route not found")
	}
	if err != nil {
		return response.InternalServerError(c)
	}
	if err := validateCORSRequest(route.CORS, origin, requestedMethod); err != nil {
		return response.Forbidden(c, err.Error())
	}
	if !headersAllowed(route.CORS, c.Get(fiber.HeaderAccessControlRequestHeaders)) {
		return response.Forbidden(c, errCORSHeaderDenied.Error())
	}

	setPreflightCORSHeaders(c, route.CORS, origin)
	return response.NoContent(c)
}

func (h *Handler) findRouteForCORS(path string, method string) (*configcache.RouteValue, error) {
	candidates := h.configCache.FindCandidates(method)
	for i := range candidates {
		if _, ok := matchPath(candidates[i].Path, path); ok {
			return &candidates[i], nil
		}
	}
	return nil, ErrRouteNotFound
}

func validateCORSRequest(config *configcache.CORSValue, origin string, method string) error {
	if config == nil {
		return errCORSNotConfigured
	}
	if !originAllowed(config.AllowedOrigins, origin) {
		return errCORSOriginDenied
	}
	if !containsFold(config.AllowedMethods, method) {
		return errCORSMethodDenied
	}
	return nil
}

func originAllowed(origins []string, origin string) bool {
	return containsFold(origins, "*") || containsFold(origins, origin)
}

func headersAllowed(config *configcache.CORSValue, requested string) bool {
	requestedHeaders := splitHeaderValues(requested)
	if len(requestedHeaders) == 0 {
		return true
	}
	if config == nil {
		return false
	}
	if containsFold(config.AllowedHeaders, "*") {
		return true
	}
	for _, header := range requestedHeaders {
		if !containsFold(config.AllowedHeaders, header) {
			return false
		}
	}
	return true
}

func setActualCORSHeaders(c *fiber.Ctx, config *configcache.CORSValue, origin string) {
	setCORSOriginHeaders(c, config, origin)
}

func setPreflightCORSHeaders(c *fiber.Ctx, config *configcache.CORSValue, origin string) {
	setCORSOriginHeaders(c, config, origin)
	c.Set(fiber.HeaderAccessControlAllowMethods, strings.Join(config.AllowedMethods, ", "))
	if len(config.AllowedHeaders) > 0 {
		c.Set(fiber.HeaderAccessControlAllowHeaders, strings.Join(config.AllowedHeaders, ", "))
	}
	c.Set(fiber.HeaderAccessControlMaxAge, strconv.Itoa(config.MaxAge))
}

func setCORSOriginHeaders(c *fiber.Ctx, config *configcache.CORSValue, origin string) {
	allowedOrigin := origin
	if containsFold(config.AllowedOrigins, "*") && !config.AllowCredentials {
		allowedOrigin = "*"
	}
	c.Set(fiber.HeaderAccessControlAllowOrigin, allowedOrigin)
	appendVary(c, fiber.HeaderOrigin)
	if config.AllowCredentials {
		c.Set(fiber.HeaderAccessControlAllowCredentials, "true")
	} else {
		c.Response().Header.Del(fiber.HeaderAccessControlAllowCredentials)
	}
}

func appendVary(c *fiber.Ctx, value string) {
	current := c.GetRespHeader(fiber.HeaderVary)
	if current == "" {
		c.Set(fiber.HeaderVary, value)
		return
	}
	if !containsFold(splitHeaderValues(current), value) {
		c.Set(fiber.HeaderVary, current+", "+value)
	}
}

func containsFold(values []string, expected string) bool {
	for _, value := range values {
		if strings.EqualFold(strings.TrimSpace(value), strings.TrimSpace(expected)) {
			return true
		}
	}
	return false
}

func splitHeaderValues(value string) []string {
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if normalized := strings.TrimSpace(part); normalized != "" {
			result = append(result, normalized)
		}
	}
	return result
}
