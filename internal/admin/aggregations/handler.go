package aggregations

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"gateway-api/helper/idgen"
	"gateway-api/helper/pagination"
	"gateway-api/helper/response"
	"gateway-api/helper/validation"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
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
	var req CreateAggregationRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "invalid request body")
	}

	aggregation, err := h.aggregationFromCreate(req)
	if err != nil {
		return response.BadRequest(c, err.Error())
	}
	if err := h.repository.Create(c.Context(), aggregation); err != nil {
		return handleDBError(c, err)
	}
	if err := h.notifyChange(c); err != nil {
		return response.InternalServerError(c)
	}

	return response.Created(c, toAggregationResponse(*aggregation))
}

func (h *Handler) FindAll(c *fiber.Ctx) error {
	p := pagination.FromQuery(c)
	items, err := h.repository.FindAll(c.Context(), p)
	if err != nil {
		return response.InternalServerError(c)
	}
	responses := make([]AggregationResponse, 0, len(items))
	for _, item := range items {
		responses = append(responses, toAggregationResponse(item))
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
	aggregation, err := h.repository.FindByID(c.Context(), id)
	if errors.Is(err, ErrAggregationNotFound) {
		return response.NotFound(c, "aggregation not found")
	}
	if err != nil {
		return response.InternalServerError(c)
	}
	return response.OK(c, toAggregationResponse(*aggregation))
}

func (h *Handler) Update(c *fiber.Ctx) error {
	id, err := validation.ParseRequiredUUID("id", c.Params("id"))
	if err != nil {
		return response.BadRequest(c, err.Error())
	}
	aggregation, err := h.repository.FindByID(c.Context(), id)
	if errors.Is(err, ErrAggregationNotFound) {
		return response.NotFound(c, "aggregation not found")
	}
	if err != nil {
		return response.InternalServerError(c)
	}

	var req UpdateAggregationRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "invalid request body")
	}
	if err := applyAggregationUpdate(aggregation, req); err != nil {
		return response.BadRequest(c, err.Error())
	}
	if err := h.repository.Update(c.Context(), aggregation); err != nil {
		return handleDBError(c, err)
	}
	if err := h.notifyChange(c); err != nil {
		return response.InternalServerError(c)
	}
	return response.OK(c, toAggregationResponse(*aggregation))
}

func (h *Handler) Delete(c *fiber.Ctx) error {
	id, err := validation.ParseRequiredUUID("id", c.Params("id"))
	if err != nil {
		return response.BadRequest(c, err.Error())
	}
	if err := h.repository.Delete(c.Context(), id); err != nil {
		if errors.Is(err, ErrAggregationNotFound) {
			return response.NotFound(c, "aggregation not found")
		}
		return response.InternalServerError(c)
	}
	if err := h.notifyChange(c); err != nil {
		return response.InternalServerError(c)
	}
	return response.NoContent(c)
}

func (h *Handler) CreateStep(c *fiber.Ctx) error {
	aggregationID, err := validation.ParseRequiredUUID("id", c.Params("id"))
	if err != nil {
		return response.BadRequest(c, err.Error())
	}
	var req CreateAggregationStepRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "invalid request body")
	}
	step, err := h.stepFromCreate(aggregationID, req)
	if err != nil {
		return response.BadRequest(c, err.Error())
	}
	if err := h.repository.CreateStep(c.Context(), step); err != nil {
		return handleDBError(c, err)
	}
	if err := h.notifyChange(c); err != nil {
		return response.InternalServerError(c)
	}
	return response.Created(c, toStepResponse(*step))
}

func (h *Handler) FindSteps(c *fiber.Ctx) error {
	aggregationID, err := validation.ParseRequiredUUID("id", c.Params("id"))
	if err != nil {
		return response.BadRequest(c, err.Error())
	}
	steps, err := h.repository.FindSteps(c.Context(), aggregationID)
	if errors.Is(err, ErrAggregationNotFound) {
		return response.NotFound(c, "aggregation not found")
	}
	if err != nil {
		return response.InternalServerError(c)
	}
	responses := make([]AggregationStepResponse, 0, len(steps))
	for _, step := range steps {
		responses = append(responses, toStepResponse(step))
	}
	return response.OK(c, responses)
}

func (h *Handler) UpdateStep(c *fiber.Ctx) error {
	id, err := validation.ParseRequiredUUID("id", c.Params("id"))
	if err != nil {
		return response.BadRequest(c, err.Error())
	}
	step, err := h.repository.FindStepByID(c.Context(), id)
	if errors.Is(err, ErrAggregationStepNotFound) {
		return response.NotFound(c, "aggregation step not found")
	}
	if err != nil {
		return response.InternalServerError(c)
	}
	var req UpdateAggregationStepRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "invalid request body")
	}
	if err := applyStepUpdate(step, req); err != nil {
		return response.BadRequest(c, err.Error())
	}
	if err := h.repository.UpdateStep(c.Context(), step); err != nil {
		return handleDBError(c, err)
	}
	if err := h.notifyChange(c); err != nil {
		return response.InternalServerError(c)
	}
	return response.OK(c, toStepResponse(*step))
}

func (h *Handler) DeleteStep(c *fiber.Ctx) error {
	id, err := validation.ParseRequiredUUID("id", c.Params("id"))
	if err != nil {
		return response.BadRequest(c, err.Error())
	}
	if err := h.repository.DeleteStep(c.Context(), id); err != nil {
		if errors.Is(err, ErrAggregationStepNotFound) {
			return response.NotFound(c, "aggregation step not found")
		}
		return response.InternalServerError(c)
	}
	if err := h.notifyChange(c); err != nil {
		return response.InternalServerError(c)
	}
	return response.NoContent(c)
}

func (h *Handler) aggregationFromCreate(req CreateAggregationRequest) (*Aggregation, error) {
	id, err := idgen.NewUUID()
	if err != nil {
		return nil, err
	}
	name, err := normalizeName(req.Name)
	if err != nil {
		return nil, err
	}
	path, err := validation.NormalizePath(req.Path)
	if err != nil {
		return nil, err
	}
	method, err := validation.NormalizeRouteMethodOrDefault(req.Method, validation.DefaultRouteMethod, true)
	if err != nil {
		return nil, err
	}
	authRequired := boolValue(req.AuthRequired, true)
	requiredScopeID, err := validation.ParseOptionalUUID("required_scope_id", req.RequiredScopeID)
	if err != nil {
		return nil, err
	}
	if requiredScopeID != nil && !authRequired {
		return nil, validation.FieldError{Field: "auth_required", Message: "must be true when required_scope_id is set"}
	}
	rateLimitID, err := validation.ParseOptionalUUID("rate_limit_id", req.RateLimitID)
	if err != nil {
		return nil, err
	}
	corsPolicyID, err := validation.ParseOptionalUUID("cors_policy_id", req.CORSPolicyID)
	if err != nil {
		return nil, err
	}
	return &Aggregation{ID: id, Name: name, Path: path, Method: method, AuthRequired: authRequired, RequiredScopeID: requiredScopeID, RateLimitID: rateLimitID, CORSPolicyID: corsPolicyID, IsActive: boolValue(req.IsActive, false)}, nil
}

func applyAggregationUpdate(aggregation *Aggregation, req UpdateAggregationRequest) error {
	if req.Name != nil {
		name, err := normalizeName(*req.Name)
		if err != nil {
			return err
		}
		aggregation.Name = name
	}
	if req.Path != nil {
		path, err := validation.NormalizePath(*req.Path)
		if err != nil {
			return err
		}
		aggregation.Path = path
	}
	if req.Method != nil {
		method, err := validation.NormalizeRouteMethod(*req.Method, true)
		if err != nil {
			return err
		}
		aggregation.Method = method
	}
	if req.AuthRequired != nil {
		aggregation.AuthRequired = *req.AuthRequired
	}
	if req.RequiredScopeID.Set {
		requiredScopeID, err := validation.ParseOptionalUUID("required_scope_id", req.RequiredScopeID.Value)
		if err != nil {
			return err
		}
		aggregation.RequiredScopeID = requiredScopeID
	}
	if aggregation.RequiredScopeID != nil && !aggregation.AuthRequired {
		return validation.FieldError{Field: "auth_required", Message: "must be true when required_scope_id is set"}
	}
	if req.RateLimitID.Set {
		rateLimitID, err := validation.ParseOptionalUUID("rate_limit_id", req.RateLimitID.Value)
		if err != nil {
			return err
		}
		aggregation.RateLimitID = rateLimitID
	}
	if req.CORSPolicyID.Set {
		corsPolicyID, err := validation.ParseOptionalUUID("cors_policy_id", req.CORSPolicyID.Value)
		if err != nil {
			return err
		}
		aggregation.CORSPolicyID = corsPolicyID
	}
	if req.IsActive != nil {
		aggregation.IsActive = *req.IsActive
	}
	return nil
}

func (h *Handler) stepFromCreate(aggregationID uuid.UUID, req CreateAggregationStepRequest) (*AggregationStep, error) {
	id, err := idgen.NewUUID()
	if err != nil {
		return nil, err
	}
	serviceID, err := validation.ParseRequiredUUID("service_id", req.ServiceID)
	if err != nil {
		return nil, err
	}
	dependsOn, err := validation.ParseOptionalUUID("depends_on", req.DependsOn)
	if err != nil {
		return nil, err
	}
	if err := validateSequence(req.Sequence); err != nil {
		return nil, err
	}
	requestTemplate, err := normalizeRequestTemplate(req.RequestTemplate)
	if err != nil {
		return nil, err
	}
	responseMapping, err := normalizeResponseMapping(req.ResponseMapping)
	if err != nil {
		return nil, err
	}
	return &AggregationStep{ID: id, AggregationID: aggregationID, ServiceID: serviceID, Sequence: req.Sequence, DependsOn: dependsOn, IsRequired: boolValue(req.IsRequired, true), RequestTemplate: requestTemplate, ResponseMapping: responseMapping, IsActive: boolValue(req.IsActive, true)}, nil
}

func applyStepUpdate(step *AggregationStep, req UpdateAggregationStepRequest) error {
	if req.ServiceID != nil {
		serviceID, err := validation.ParseRequiredUUID("service_id", *req.ServiceID)
		if err != nil {
			return err
		}
		step.ServiceID = serviceID
	}
	if req.Sequence != nil {
		if err := validateSequence(*req.Sequence); err != nil {
			return err
		}
		step.Sequence = *req.Sequence
	}
	if req.DependsOn.Set {
		dependsOn, err := validation.ParseOptionalUUID("depends_on", req.DependsOn.Value)
		if err != nil {
			return err
		}
		step.DependsOn = dependsOn
	}
	if req.IsRequired != nil {
		step.IsRequired = *req.IsRequired
	}
	if len(req.RequestTemplate) > 0 {
		requestTemplate, err := normalizeRequestTemplate(req.RequestTemplate)
		if err != nil {
			return err
		}
		step.RequestTemplate = requestTemplate
	}
	if len(req.ResponseMapping) > 0 {
		responseMapping, err := normalizeResponseMapping(req.ResponseMapping)
		if err != nil {
			return err
		}
		step.ResponseMapping = responseMapping
	}
	if req.IsActive != nil {
		step.IsActive = *req.IsActive
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

func validateSequence(sequence int16) error {
	if sequence <= 0 {
		return validation.FieldError{Field: "sequence", Message: "must be greater than 0"}
	}
	return nil
}

func normalizeRequestTemplate(raw json.RawMessage) (json.RawMessage, error) {
	var payload struct {
		Method string `json:"method"`
		Path   string `json:"path"`
	}
	if len(raw) == 0 || string(raw) == "null" {
		return nil, validation.FieldError{Field: "request_template", Message: "is required"}
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, validation.FieldError{Field: "request_template", Message: "must be a JSON object"}
	}
	method, err := validation.NormalizeRouteMethod(payload.Method, true)
	if err != nil {
		return nil, err
	}
	requestPath := strings.TrimSpace(payload.Path)
	if requestPath != "" && !strings.HasPrefix(requestPath, "/") {
		requestPath = "/" + requestPath
	}
	path, err := validation.NormalizePath(requestPath)
	if err != nil {
		return nil, err
	}
	payload.Method = method
	payload.Path = path
	return json.Marshal(payload)
}

func normalizeResponseMapping(raw json.RawMessage) (json.RawMessage, error) {
	var payload struct {
		Target string `json:"target"`
	}
	if len(raw) == 0 || string(raw) == "null" {
		return nil, validation.FieldError{Field: "response_mapping", Message: "is required"}
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, validation.FieldError{Field: "response_mapping", Message: "must be a JSON object"}
	}
	payload.Target = strings.TrimSpace(payload.Target)
	if payload.Target == "" {
		return nil, validation.FieldError{Field: "response_mapping.target", Message: "is required"}
	}
	return json.Marshal(payload)
}

func (h *Handler) notifyChange(c *fiber.Ctx) error {
	if h.notifier == nil {
		return nil
	}
	return h.notifier.NotifyChange(c.Context(), "aggregations")
}

func boolValue(value *bool, fallback bool) bool {
	if value == nil {
		return fallback
	}
	return *value
}

func toAggregationResponse(aggregation Aggregation) AggregationResponse {
	var requiredScopeID *string
	if aggregation.RequiredScopeID != nil {
		value := aggregation.RequiredScopeID.String()
		requiredScopeID = &value
	}
	var rateLimitID *string
	if aggregation.RateLimitID != nil {
		value := aggregation.RateLimitID.String()
		rateLimitID = &value
	}
	var corsPolicyID *string
	if aggregation.CORSPolicyID != nil {
		value := aggregation.CORSPolicyID.String()
		corsPolicyID = &value
	}
	var corsPolicy *CORSPolicySummaryResponse
	if aggregation.CORSPolicy != nil {
		corsPolicy = &CORSPolicySummaryResponse{ID: aggregation.CORSPolicy.ID.String(), Name: aggregation.CORSPolicy.Name, AllowedOrigins: aggregation.CORSPolicy.AllowedOrigins, AllowedMethods: aggregation.CORSPolicy.AllowedMethods, AllowedHeaders: aggregation.CORSPolicy.AllowedHeaders, ExposedHeaders: aggregation.CORSPolicy.ExposedHeaders, AllowCredentials: aggregation.CORSPolicy.AllowCredentials, MaxAge: aggregation.CORSPolicy.MaxAge}
	}
	return AggregationResponse{ID: aggregation.ID.String(), Name: aggregation.Name, Path: aggregation.Path, Method: aggregation.Method, AuthRequired: aggregation.AuthRequired, RequiredScopeID: requiredScopeID, RateLimitID: rateLimitID, CORSPolicyID: corsPolicyID, CORSPolicy: corsPolicy, IsActive: aggregation.IsActive, CreatedAt: aggregation.CreatedAt, UpdatedAt: aggregation.UpdatedAt}
}
func toStepResponse(step AggregationStep) AggregationStepResponse {
	var dependsOn *string
	if step.DependsOn != nil {
		value := step.DependsOn.String()
		dependsOn = &value
	}
	return AggregationStepResponse{ID: step.ID.String(), AggregationID: step.AggregationID.String(), ServiceID: step.ServiceID.String(), Sequence: step.Sequence, DependsOn: dependsOn, IsRequired: step.IsRequired, RequestTemplate: step.RequestTemplate, ResponseMapping: step.ResponseMapping, IsActive: step.IsActive, CreatedAt: step.CreatedAt, UpdatedAt: step.UpdatedAt}
}

func handleDBError(c *fiber.Ctx, err error) error {
	switch {
	case errors.Is(err, ErrAggregationNotFound):
		return response.NotFound(c, "aggregation not found")
	case errors.Is(err, ErrAggregationStepNotFound):
		return response.NotFound(c, "aggregation step not found")
	case errors.Is(err, ErrAggregationDuplicate):
		return response.Conflict(c, "aggregation already exists")
	case errors.Is(err, ErrAggregationStepDuplicate):
		return response.Conflict(c, "aggregation step sequence already exists")
	case errors.Is(err, ErrServiceUnavailable), errors.Is(err, ErrDependsOnUnavailable), errors.Is(err, ErrStepDependsOnSelf), errors.Is(err, ErrCORSPolicyUnavailable), errors.Is(err, ErrRequiredScopeUnavailable), errors.Is(err, ErrRateLimitUnavailable), errors.Is(err, ErrAggregationActiveWithoutSteps), errors.Is(err, ErrDependsOnSequenceInvalid), errors.Is(err, ErrDependsOnCycle), errors.Is(err, ErrResponseTargetDuplicate):
		return response.Error(c, fiber.StatusUnprocessableEntity, "invalid_reference", err.Error())
	default:
		return response.InternalServerError(c)
	}
}
