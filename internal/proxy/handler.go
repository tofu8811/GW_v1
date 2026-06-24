package proxy

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"gateway-api/helper/response"

	"github.com/gofiber/fiber/v2"
)

type Handler struct {
	repository *Repository
	client     *http.Client
}

func NewHandler(repository *Repository) *Handler {
	return &Handler{
		repository: repository,
		client:     &http.Client{},
	}
}

func (h *Handler) Proxy(c *fiber.Ctx) error {
	route, err := h.repository.FindRoute(c.Context(), c.Method(), c.Path())
	if errors.Is(err, ErrRouteNotFound) {
		return response.NotFound(c, "route not found")
	}
	if err != nil {
		return response.InternalServerError(c)
	}

	service, err := h.repository.FindService(c.Context(), route.ServiceID)
	if errors.Is(err, ErrServiceNotFound) {
		return response.Error(c, fiber.StatusBadGateway, "bad_gateway", "service is not active")
	}
	if err != nil {
		return response.InternalServerError(c)
	}

	instances, err := h.repository.FindActiveInstances(c.Context(), service.ID)
	if errors.Is(err, ErrInstancesNotFound) {
		return response.Error(c, fiber.StatusBadGateway, "bad_gateway", "no active service instance")
	}
	if err != nil {
		return response.InternalServerError(c)
	}

	targetURL, err := buildTargetURL(c, *route, *service, instances[0])
	if err != nil {
		return response.Error(c, fiber.StatusBadGateway, "bad_gateway", "invalid upstream target")
	}

	upstream, cancel, err := h.forward(c, *service, targetURL)
	if errors.Is(err, context.DeadlineExceeded) {
		return response.Error(c, fiber.StatusGatewayTimeout, "gateway_timeout", "upstream request timeout")
	}
	if err != nil {
		return response.Error(c, fiber.StatusBadGateway, "bad_gateway", "upstream request failed")
	}
	defer cancel()
	defer upstream.Body.Close()

	copyResponseHeaders(c, upstream.Header)

	body, err := io.ReadAll(upstream.Body)
	if err != nil {
		return response.Error(c, fiber.StatusBadGateway, "bad_gateway", "cannot read upstream response")
	}

	return c.Status(upstream.StatusCode).Send(body)
}

func (h *Handler) forward(c *fiber.Ctx, service ServiceConfig, targetURL string) (*http.Response, context.CancelFunc, error) {
	timeout := time.Duration(service.TimeoutMS) * time.Millisecond
	if timeout <= 0 {
		timeout = 5 * time.Second
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)

	req, err := http.NewRequestWithContext(ctx, c.Method(), targetURL, bytes.NewReader(c.Body()))
	if err != nil {
		cancel()
		return nil, nil, err
	}

	copyRequestHeaders(c, req)
	addForwardedHeaders(c, req)

	upstream, err := h.client.Do(req)
	if err != nil {
		cancel()
		return nil, nil, err
	}

	return upstream, cancel, nil
}

func buildTargetURL(c *fiber.Ctx, route RouteConfig, service ServiceConfig, instance ServiceInstance) (string, error) {
	targetPath := c.Path()

	if route.RewriteTarget != nil && strings.TrimSpace(*route.RewriteTarget) != "" {
		targetPath = strings.TrimSpace(*route.RewriteTarget)
	} else if route.StripPrefix {
		targetPath = strings.TrimPrefix(targetPath, route.Path)
		if targetPath == "" {
			targetPath = "/"
		}
		if !strings.HasPrefix(targetPath, "/") {
			targetPath = "/" + targetPath
		}
	}

	target := url.URL{
		Scheme: service.Protocol,
		Host:   fmt.Sprintf("%s:%d", instance.Host, instance.Port),
		Path:   targetPath,
	}

	query := string(c.Request().URI().QueryString())
	if query != "" {
		target.RawQuery = query
	}

	return target.String(), nil
}

func copyRequestHeaders(c *fiber.Ctx, req *http.Request) {
	c.Request().Header.VisitAll(func(key []byte, value []byte) {
		name := string(key)
		if isHopByHopHeader(name) || strings.EqualFold(name, fiber.HeaderHost) {
			return
		}

		req.Header.Add(name, string(value))
	})
}

func addForwardedHeaders(c *fiber.Ctx, req *http.Request) {
	req.Header.Set("X-Forwarded-For", c.IP())
	req.Header.Set("X-Forwarded-Host", c.Hostname())
	req.Header.Set("X-Forwarded-Proto", c.Protocol())

	requestID := c.Get(fiber.HeaderXRequestID)
	if requestID == "" {
		requestID = c.GetRespHeader(fiber.HeaderXRequestID)
	}
	if requestID != "" {
		req.Header.Set(fiber.HeaderXRequestID, requestID)
	}
}

func copyResponseHeaders(c *fiber.Ctx, headers http.Header) {
	for name, values := range headers {
		if isHopByHopHeader(name) || strings.EqualFold(name, fiber.HeaderContentLength) {
			continue
		}

		for i, value := range values {
			if i == 0 {
				c.Set(name, value)
				continue
			}
			c.Append(name, value)
		}
	}
}

func isHopByHopHeader(name string) bool {
	switch strings.ToLower(name) {
	case "connection",
		"keep-alive",
		"proxy-authenticate",
		"proxy-authorization",
		"te",
		"trailer",
		"transfer-encoding",
		"upgrade":
		return true
	default:
		return false
	}
}
