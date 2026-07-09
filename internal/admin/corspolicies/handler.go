package corspolicies

import (
	"context"
	"errors"
	"net"
	"net/url"
	"strings"

	"gateway-api/helper/dberror"
	"gateway-api/helper/idgen"
	"gateway-api/helper/pagination"
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

func (h *Handler) Create(c *fiber.Ctx) error {
	var req CreateCORSPolicyRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "invalid request body")
	}
	policy, err := policyFromCreate(req)
	if err != nil {
		return response.BadRequest(c, err.Error())
	}
	if err := h.repository.Create(c.Context(), policy); err != nil {
		return handleDBError(c, err)
	}
	if err := h.notifyChange(c); err != nil {
		return response.InternalServerError(c)
	}
	return response.Created(c, toResponse(*policy))
}

func (h *Handler) FindAll(c *fiber.Ctx) error {
	p := pagination.FromQuery(c)
	items, err := h.repository.FindAll(c.Context(), p)
	if err != nil {
		return response.InternalServerError(c)
	}
	responses := make([]CORSPolicyResponse, 0, len(items))
	for _, item := range items {
		responses = append(responses, toResponse(item))
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
	if errors.Is(err, ErrCORSPolicyNotFound) {
		return response.NotFound(c, "CORS policy not found")
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
	if errors.Is(err, ErrCORSPolicyNotFound) {
		return response.NotFound(c, "CORS policy not found")
	}
	if err != nil {
		return response.InternalServerError(c)
	}
	var req UpdateCORSPolicyRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "invalid request body")
	}
	if err := applyPolicyUpdate(policy, req); err != nil {
		return response.BadRequest(c, err.Error())
	}
	if err := h.repository.Update(c.Context(), policy); err != nil {
		if errors.Is(err, ErrCORSPolicyNotFound) {
			return response.NotFound(c, "CORS policy not found")
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
	if err := h.repository.Delete(c.Context(), id); err != nil {
		if errors.Is(err, ErrCORSPolicyNotFound) {
			return response.NotFound(c, "CORS policy not found")
		}
		if errors.Is(err, ErrCORSPolicyInUse) {
			return response.Conflict(c, err.Error())
		}
		return handleDBError(c, err)
	}
	if err := h.notifyChange(c); err != nil {
		return response.InternalServerError(c)
	}
	return response.NoContent(c)
}

func policyFromCreate(req CreateCORSPolicyRequest) (*CORSPolicy, error) {
	id, err := idgen.NewUUID()
	if err != nil {
		return nil, err
	}
	name, err := normalizeName(req.Name)
	if err != nil {
		return nil, err
	}
	origins, methods, headers, exposed, maxAge, err := normalizePolicyValues(req.AllowedOrigins, req.AllowedMethods, req.AllowedHeaders, req.ExposedHeaders, req.AllowCredentials, req.MaxAge)
	if err != nil {
		return nil, err
	}
	return &CORSPolicy{ID: id, Name: name, AllowedOrigins: origins, AllowedMethods: methods, AllowedHeaders: headers, ExposedHeaders: exposed, AllowCredentials: req.AllowCredentials, MaxAge: maxAge, IsActive: boolValue(req.IsActive, true)}, nil
}

func applyPolicyUpdate(policy *CORSPolicy, req UpdateCORSPolicyRequest) error {
	if req.Name != nil {
		name, err := normalizeName(*req.Name)
		if err != nil {
			return err
		}
		policy.Name = name
	}
	allowCredentials := policy.AllowCredentials
	if req.AllowCredentials != nil {
		allowCredentials = *req.AllowCredentials
	}
	origins := policy.AllowedOrigins
	if req.AllowedOrigins != nil {
		origins = req.AllowedOrigins
	}
	methods := policy.AllowedMethods
	if req.AllowedMethods != nil {
		methods = req.AllowedMethods
	}
	headers := policy.AllowedHeaders
	if req.AllowedHeaders != nil {
		headers = req.AllowedHeaders
	}
	exposed := policy.ExposedHeaders
	if req.ExposedHeaders != nil {
		exposed = req.ExposedHeaders
	}
	maxAgePtr := req.MaxAge
	if maxAgePtr == nil {
		current := policy.MaxAge
		maxAgePtr = &current
	}
	normalizedOrigins, normalizedMethods, normalizedHeaders, normalizedExposed, maxAge, err := normalizePolicyValues(origins, methods, headers, exposed, allowCredentials, maxAgePtr)
	if err != nil {
		return err
	}
	policy.AllowedOrigins = normalizedOrigins
	policy.AllowedMethods = normalizedMethods
	policy.AllowedHeaders = normalizedHeaders
	policy.ExposedHeaders = normalizedExposed
	policy.AllowCredentials = allowCredentials
	policy.MaxAge = maxAge
	if req.IsActive != nil {
		policy.IsActive = *req.IsActive
	}
	return nil
}

func normalizeName(value string) (string, error) {
	name := strings.TrimSpace(value)
	if name == "" {
		return "", validation.FieldError{Field: "name", Message: "name is required"}
	}
	return name, nil
}

func normalizePolicyValues(origins []string, methods []string, headers []string, exposed []string, allowCredentials bool, maxAgePtr *int) ([]string, []string, []string, []string, int, error) {
	normalizedOrigins, err := normalizeOrigins(origins)
	if err != nil {
		return nil, nil, nil, nil, 0, err
	}
	if len(normalizedOrigins) == 0 {
		return nil, nil, nil, nil, 0, validation.FieldError{Field: "allowed_origins", Message: "at least one origin is required"}
	}
	normalizedMethods, err := normalizeMethods(methods)
	if err != nil {
		return nil, nil, nil, nil, 0, err
	}
	if len(normalizedMethods) == 0 {
		return nil, nil, nil, nil, 0, validation.FieldError{Field: "allowed_methods", Message: "at least one method is required"}
	}
	normalizedHeaders, err := normalizeHeaderList("allowed_headers", headers)
	if err != nil {
		return nil, nil, nil, nil, 0, err
	}
	normalizedExposed, err := normalizeHeaderList("exposed_headers", exposed)
	if err != nil {
		return nil, nil, nil, nil, 0, err
	}
	if allowCredentials {
		if containsExact(normalizedOrigins, "*") {
			return nil, nil, nil, nil, 0, validation.FieldError{Field: "allowed_origins", Message: "wildcard origin cannot be used with credentials"}
		}
		if containsExact(normalizedMethods, "*") {
			return nil, nil, nil, nil, 0, validation.FieldError{Field: "allowed_methods", Message: "wildcard method cannot be used with credentials"}
		}
		if containsExact(normalizedHeaders, "*") {
			return nil, nil, nil, nil, 0, validation.FieldError{Field: "allowed_headers", Message: "wildcard header cannot be used with credentials"}
		}
	}
	if containsExact(normalizedMethods, "*") {
		return nil, nil, nil, nil, 0, validation.FieldError{Field: "allowed_methods", Message: "wildcard method is not supported"}
	}
	if containsExact(normalizedHeaders, "*") {
		return nil, nil, nil, nil, 0, validation.FieldError{Field: "allowed_headers", Message: "wildcard header is not supported"}
	}
	maxAge := defaultMaxAge
	if maxAgePtr != nil {
		maxAge = *maxAgePtr
	}
	if err := validation.ValidateIntBetween("max_age", maxAge, 0, 86400); err != nil {
		return nil, nil, nil, nil, 0, err
	}
	return normalizedOrigins, normalizedMethods, normalizedHeaders, normalizedExposed, maxAge, nil
}

func normalizeOrigins(values []string) ([]string, error) {
	seen := map[string]struct{}{}
	result := []string{}
	for _, value := range values {
		origin := strings.TrimSpace(value)
		if origin == "" {
			continue
		}
		if origin == "*" {
			if _, ok := seen[origin]; !ok {
				seen[origin] = struct{}{}
				result = append(result, origin)
			}
			continue
		}
		normalized, err := normalizeOrigin(origin)
		if err != nil {
			return nil, validation.FieldError{Field: "allowed_origins", Message: err.Error()}
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		result = append(result, normalized)
	}
	return result, nil
}

func normalizeOrigin(value string) (string, error) {
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", errors.New("contains an invalid origin")
	}
	if parsed.RawQuery != "" || parsed.Fragment != "" || (parsed.Path != "" && parsed.Path != "/") {
		return "", errors.New("origin must not include path, query, or fragment")
	}
	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return "", errors.New("origin scheme must be http or https")
	}
	host := strings.ToLower(parsed.Host)
	if hostname, port, err := net.SplitHostPort(host); err == nil {
		host = strings.ToLower(hostname) + ":" + port
	}
	return scheme + "://" + host, nil
}

func normalizeMethods(values []string) ([]string, error) {
	methods := normalizeUnique(values, true)
	for _, method := range methods {
		if _, err := validation.NormalizeRouteMethod(method, false); err != nil {
			return nil, validation.FieldError{Field: "allowed_methods", Message: "contains an invalid method"}
		}
	}
	if !containsExact(methods, fiber.MethodOptions) {
		methods = append(methods, fiber.MethodOptions)
	}
	return methods, nil
}

func normalizeHeaderList(field string, values []string) ([]string, error) {
	items := normalizeUnique(values, false)
	for _, item := range items {
		if item == "*" {
			continue
		}
		if strings.ContainsAny(item, " \t\r\n,") {
			return nil, validation.FieldError{Field: field, Message: "contains an invalid header"}
		}
	}
	return items, nil
}

func normalizeUnique(values []string, upper bool) []string {
	seen := map[string]struct{}{}
	result := []string{}
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

func containsExact(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}

func boolValue(value *bool, fallback bool) bool {
	if value == nil {
		return fallback
	}
	return *value
}

func (h *Handler) notifyChange(c *fiber.Ctx) error {
	if h.notifier == nil {
		return nil
	}
	return h.notifier.NotifyChange(c.Context(), "cors_policies")
}

func toResponse(policy CORSPolicy) CORSPolicyResponse {
	return CORSPolicyResponse{ID: policy.ID.String(), Name: policy.Name, AllowedOrigins: policy.AllowedOrigins, AllowedMethods: policy.AllowedMethods, AllowedHeaders: policy.AllowedHeaders, ExposedHeaders: policy.ExposedHeaders, AllowCredentials: policy.AllowCredentials, MaxAge: policy.MaxAge, IsActive: policy.IsActive, CreatedAt: policy.CreatedAt, UpdatedAt: policy.UpdatedAt}
}

func handleDBError(c *fiber.Ctx, err error) error {
	if apiErr, ok := dberror.MapDBError(err); ok {
		return response.Error(c, apiErr.Status, apiErr.Code, apiErr.Message)
	}
	return response.InternalServerError(c)
}
