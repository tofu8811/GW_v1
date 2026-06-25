package gateway

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"gateway-api/helper/response"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/proxy"
)

var ErrRouteNotFound = errors.New("gateway route not found")

type Handler struct {
	repository *Repository
	logger     *slog.Logger
}

func NewHandler(repository *Repository, logger *slog.Logger) *Handler {
	return &Handler{repository: repository, logger: logger}
}

func (h *Handler) Proxy(c *fiber.Ctx) error {
	requestPath := c.Path()
	method := c.Method()

	route, params, err := h.findRoute(c.Context(), requestPath, method)
	if errors.Is(err, ErrRouteNotFound) {
		return response.NotFound(c, "gateway route not found")
	}
	if err != nil {
		h.logger.Error("failed to find gateway route", "error", err, "path", requestPath, "method", method)
		return response.InternalServerError(c)
	}

	targetURL := buildTargetURL(route, requestPath, params, string(c.Request().URI().QueryString()))
	h.logger.Info("proxying request", "method", method, "path", requestPath, "service", route.ServiceName, "upstream", targetURL)

	timeout := time.Duration(route.TimeoutMS) * time.Millisecond
	if timeout <= 0 {
		timeout = 5 * time.Second
	}

	prepareForwardedRequest(c)

	if err := proxy.DoTimeout(c, targetURL, timeout); err != nil {
		h.logger.Error("failed to proxy request", "error", err, "upstream", targetURL)
		return response.Error(c, fiber.StatusBadGateway, "bad_gateway", "upstream service unavailable")
	}

	return nil
}

func (h *Handler) findRoute(ctx context.Context, path string, method string) (*UpstreamRoute, map[string]string, error) {
	candidates, err := h.repository.FindCandidates(ctx, method)
	if err != nil {
		return nil, nil, err
	}

	for i := range candidates {
		params, ok := matchPath(candidates[i].RoutePath, path)
		if ok {
			return &candidates[i], params, nil
		}
	}

	return nil, nil, ErrRouteNotFound
}

func buildTargetURL(route *UpstreamRoute, requestPath string, params map[string]string, rawQuery string) string {
	targetPath := rewritePath(route, requestPath, params)
	targetURL := fmt.Sprintf("%s://%s:%d%s", route.Protocol, route.Host, route.Port, targetPath)
	if rawQuery != "" {
		targetURL += "?" + rawQuery
	}

	return targetURL
}

func rewritePath(route *UpstreamRoute, requestPath string, params map[string]string) string {
	if route.RewriteTarget != nil && *route.RewriteTarget != "" {
		return fillPathParams(*route.RewriteTarget, params)
	}

	if route.StripPrefix {
		if tail, ok := params["*"]; ok {
			return "/" + tail
		}

		return requestPath
	}

	return requestPath
}

func matchPath(pattern string, path string) (map[string]string, bool) {
	patternParts := splitPath(pattern)
	pathParts := splitPath(path)
	params := map[string]string{}

	for i := range patternParts {
		part := patternParts[i]

		if part == "*" || strings.HasSuffix(part, "...}") {
			if i != len(patternParts)-1 {
				return nil, false
			}

			name := "*"
			if part != "*" {
				name = strings.TrimSuffix(strings.TrimPrefix(part, "{"), "...}")
			}
			tail := strings.Join(pathParts[i:], "/")
			params[name] = tail
			params["*"] = tail
			return params, true
		}

		if i >= len(pathParts) {
			return nil, false
		}

		if strings.HasPrefix(part, "{") && strings.HasSuffix(part, "}") {
			name := strings.TrimSuffix(strings.TrimPrefix(part, "{"), "}")
			params[name] = pathParts[i]
			continue
		}

		if part != pathParts[i] {
			return nil, false
		}
	}

	if len(patternParts) != len(pathParts) {
		return nil, false
	}

	return params, true
}

func splitPath(path string) []string {
	path = strings.Trim(path, "/")
	if path == "" {
		return []string{}
	}

	return strings.Split(path, "/")
}

func fillPathParams(path string, params map[string]string) string {
	for name, value := range params {
		path = strings.ReplaceAll(path, "{"+name+"}", value)
	}

	if strings.HasPrefix(path, "/") {
		return path
	}

	return "/" + path
}
