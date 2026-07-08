package apikeys

import (
	"errors"
	"strconv"
	"strings"
	"time"

	cryptoutil "gateway-api/helper/crypto"
	"gateway-api/helper/dberror"
	"gateway-api/helper/idgen"
	"gateway-api/helper/pagination"
	"gateway-api/helper/response"
	"gateway-api/helper/validation"
	"gateway-api/internal/middleware"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

const apiKeyPrefix = "gw_live_"
const maxAPIKeyGenerationAttempts = 5

type Handler struct {
	repository *Repository
}

func NewHandler(repository *Repository) *Handler {
	return &Handler{repository: repository}
}

func (h *Handler) Create(c *fiber.Ctx) error {
	var req CreateAPIKeyRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "invalid request body")
	}

	clientID, err := validation.ParseRequiredUUID("client_id", req.ClientID)
	if err != nil {
		return response.BadRequest(c, err.Error())
	}
	scopeIDs, err := parseScopeIDs(req.ScopeIDs)
	if err != nil {
		return response.BadRequest(c, err.Error())
	}
	rateLimitID, err := validation.ParseOptionalUUID("rate_limit_id", req.RateLimitID)
	if err != nil {
		return response.BadRequest(c, err.Error())
	}
	if req.ExpiresAt != nil && !req.ExpiresAt.After(time.Now()) {
		return response.BadRequest(c, "expires_at must be in the future")
	}

	createdBy := currentUserID(c)

	for attempt := 0; attempt < maxAPIKeyGenerationAttempts; attempt++ {
		id, err := idgen.NewUUID()
		if err != nil {
			return response.InternalServerError(c)
		}
		rawKey, keyHash, keyPrefix, err := generateAPIKeyMaterial()
		if err != nil {
			return response.InternalServerError(c)
		}
		key := APIKey{
			ID: id, KeyHash: keyHash, KeyPrefix: keyPrefix, Label: normalizedOptionalString(req.Label),
			ClientID: clientID, ScopeIDs: scopeIDs, RateLimitID: rateLimitID,
			ExpiresAt: req.ExpiresAt, IsActive: boolValue(req.IsActive, true), CreatedBy: createdBy,
		}
		if err := h.repository.Create(c.Context(), &key); err != nil {
			if errors.Is(err, ErrAPIKeyHashExists) {
				continue
			}
			return handleDBError(c, err)
		}

		return response.Created(c, CreatedAPIKeyResponse{APIKeyResponse: toResponse(key), Key: rawKey})
	}

	return response.Error(c, fiber.StatusConflict, "conflict", "could not generate a unique API key")
}

func (h *Handler) FindAll(c *fiber.Ctx) error {
	p := pagination.FromQuery(c)
	filters, err := parseListFilters(c)
	if err != nil {
		return response.BadRequest(c, err.Error())
	}
	keys, err := h.repository.FindAll(c.Context(), p, filters)
	if err != nil {
		return response.InternalServerError(c)
	}
	items := make([]APIKeyResponse, 0, len(keys))
	for _, key := range keys {
		items = append(items, toResponse(key))
	}
	total, err := h.repository.Count(c.Context(), filters)
	if err != nil {
		return response.InternalServerError(c)
	}
	return response.WithMeta(c, items, pagination.NewMeta(p, total))
}

func (h *Handler) Options(c *fiber.Ctx) error {
	options, err := h.repository.FindOptions(c.Context())
	if err != nil {
		return response.InternalServerError(c)
	}
	return response.OK(c, APIKeyOptionsResponse{
		Clients:    toOptionResponses(options.Clients),
		Scopes:     toScopeOptionResponses(options.Scopes),
		RateLimits: toOptionResponses(options.RateLimits),
	})
}

func (h *Handler) FindByID(c *fiber.Ctx) error {
	id, err := validation.ParseRequiredUUID("id", c.Params("id"))
	if err != nil {
		return response.BadRequest(c, err.Error())
	}
	key, err := h.repository.FindByID(c.Context(), id)
	if errors.Is(err, ErrAPIKeyNotFound) {
		return response.NotFound(c, "API key not found")
	}
	if err != nil {
		return response.InternalServerError(c)
	}
	return response.OK(c, toResponse(*key))
}

func (h *Handler) Update(c *fiber.Ctx) error {
	id, err := validation.ParseRequiredUUID("id", c.Params("id"))
	if err != nil {
		return response.BadRequest(c, err.Error())
	}
	key, err := h.repository.FindByID(c.Context(), id)
	if errors.Is(err, ErrAPIKeyNotFound) {
		return response.NotFound(c, "API key not found")
	}
	if err != nil {
		return response.InternalServerError(c)
	}

	var req UpdateAPIKeyRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "invalid request body")
	}
	if req.Label != nil {
		key.Label = normalizedOptionalString(req.Label)
	}
	if req.ClientID != nil {
		key.ClientID, err = validation.ParseRequiredUUID("client_id", *req.ClientID)
		if err != nil {
			return response.BadRequest(c, err.Error())
		}
	}
	if req.ScopeIDs != nil {
		key.ScopeIDs, err = parseScopeIDs(*req.ScopeIDs)
		if err != nil {
			return response.BadRequest(c, err.Error())
		}
	}
	if req.RateLimitID != nil {
		key.RateLimitID, err = validation.ParseOptionalUUID("rate_limit_id", req.RateLimitID)
		if err != nil {
			return response.BadRequest(c, err.Error())
		}
	}
	if req.IsActive != nil {
		key.IsActive = *req.IsActive
	}

	if err := h.repository.Update(c.Context(), key); err != nil {
		return handleDBError(c, err)
	}
	return response.OK(c, toResponse(*key))
}

func (h *Handler) Revoke(c *fiber.Ctx) error {
	id, err := validation.ParseRequiredUUID("id", c.Params("id"))
	if err != nil {
		return response.BadRequest(c, err.Error())
	}
	key, err := h.repository.Revoke(c.Context(), id)
	if errors.Is(err, ErrAPIKeyNotFound) {
		return response.NotFound(c, "API key not found")
	} else if err != nil {
		return handleDBError(c, err)
	}
	return response.OK(c, toResponse(*key))
}

func (h *Handler) Rotate(c *fiber.Ctx) error {
	id, err := validation.ParseRequiredUUID("id", c.Params("id"))
	if err != nil {
		return response.BadRequest(c, err.Error())
	}

	createdBy := currentUserID(c)
	for attempt := 0; attempt < maxAPIKeyGenerationAttempts; attempt++ {
		newID, err := idgen.NewUUID()
		if err != nil {
			return response.InternalServerError(c)
		}
		rawKey, keyHash, keyPrefix, err := generateAPIKeyMaterial()
		if err != nil {
			return response.InternalServerError(c)
		}

		key, err := h.repository.Rotate(c.Context(), id, newID, keyHash, keyPrefix, createdBy)
		if errors.Is(err, ErrAPIKeyNotFound) {
			return response.NotFound(c, "API key not found")
		}
		if errors.Is(err, ErrAPIKeyHashExists) {
			continue
		}
		if err != nil {
			return handleDBError(c, err)
		}

		return response.OK(c, CreatedAPIKeyResponse{APIKeyResponse: toResponse(*key), Key: rawKey})
	}

	return response.Error(c, fiber.StatusConflict, "conflict", "could not generate a unique API key")
}

func generateAPIKeyMaterial() (rawKey string, keyHash string, keyPrefix string, err error) {
	secret, err := cryptoutil.GenerateRandomToken()
	if err != nil {
		return "", "", "", err
	}
	rawKey = apiKeyPrefix + secret
	keyHash, err = cryptoutil.HashAPIKey(rawKey)
	if err != nil {
		return "", "", "", err
	}
	return rawKey, keyHash, rawKey[:12], nil
}

func parseListFilters(c *fiber.Ctx) (APIKeyListFilters, error) {
	var filters APIKeyListFilters
	if rawClientID := strings.TrimSpace(c.Query("client_id")); rawClientID != "" {
		clientID, err := validation.ParseRequiredUUID("client_id", rawClientID)
		if err != nil {
			return filters, err
		}
		filters.ClientID = &clientID
	}
	if rawIsActive := strings.TrimSpace(c.Query("is_active")); rawIsActive != "" {
		isActive, err := strconv.ParseBool(rawIsActive)
		if err != nil {
			return filters, validation.FieldError{Field: "is_active", Message: "must be true or false"}
		}
		filters.IsActive = &isActive
	}
	filters.IncludeRevoked = c.QueryBool("include_revoked", false)
	filters.IncludeDeleted = c.QueryBool("include_deleted", false)
	filters.DeletedOnly = c.QueryBool("deleted_only", false)
	return filters, nil
}

func parseScopeIDs(values []string) ([]uuid.UUID, error) {
	unique := make(map[uuid.UUID]struct{}, len(values))
	result := make([]uuid.UUID, 0, len(values))
	for _, value := range values {
		normalized := strings.TrimSpace(value)
		if normalized == "" {
			continue
		}
		id, err := uuid.Parse(normalized)
		if err != nil {
			return nil, validation.FieldError{Field: "scope_ids", Message: "contains invalid scope id"}
		}
		if _, exists := unique[id]; exists {
			continue
		}
		unique[id] = struct{}{}
		result = append(result, id)
	}
	if len(result) == 0 {
		return nil, validation.FieldError{Field: "scope_ids", Message: "at least one scope is required"}
	}
	return result, nil
}

func normalizedOptionalString(value *string) *string {
	if value == nil {
		return nil
	}
	normalized := strings.TrimSpace(*value)
	if normalized == "" {
		return nil
	}
	return &normalized
}

func boolValue(value *bool, fallback bool) bool {
	if value == nil {
		return fallback
	}
	return *value
}

func currentUserID(c *fiber.Ctx) *uuid.UUID {
	value := middleware.GetUserID(c)
	if value == "" {
		return nil
	}
	id, err := uuid.Parse(value)
	if err != nil {
		return nil
	}
	return &id
}

func toResponse(key APIKey) APIKeyResponse {
	var rateLimitID *string
	if key.RateLimitID != nil {
		value := key.RateLimitID.String()
		rateLimitID = &value
	}
	var createdBy *string
	if key.CreatedBy != nil {
		value := key.CreatedBy.String()
		createdBy = &value
	}
	scopes := make([]APIKeyScopeResponse, 0, len(key.Scopes))
	for _, scope := range key.Scopes {
		scopes = append(scopes, APIKeyScopeResponse{
			ID: scope.ID.String(), ServiceID: scope.ServiceID.String(), Code: scope.Code, Resource: scope.Resource, Action: scope.Action,
		})
	}
	return APIKeyResponse{
		ID: key.ID.String(), KeyPrefix: key.KeyPrefix, Label: key.Label, ClientID: key.ClientID.String(),
		Scopes: scopes, RateLimitID: rateLimitID, ExpiresAt: key.ExpiresAt,
		IsActive: key.IsActive, RevokedAt: key.RevokedAt, LastUsedAt: key.LastUsedAt,
		CreatedBy: createdBy, CreatedAt: key.CreatedAt, UpdatedAt: key.UpdatedAt,
	}
}

func toOptionResponses(options []APIKeyOption) []APIKeyOptionResponse {
	responses := make([]APIKeyOptionResponse, 0, len(options))
	for _, option := range options {
		responses = append(responses, APIKeyOptionResponse{ID: option.ID.String(), Name: option.Name})
	}
	return responses
}

func toScopeOptionResponses(scopes []APIScope) []APIKeyScopeOptionResponse {
	responses := make([]APIKeyScopeOptionResponse, 0, len(scopes))
	for _, scope := range scopes {
		responses = append(responses, APIKeyScopeOptionResponse{
			ID: scope.ID.String(), ServiceID: scope.ServiceID.String(), Code: scope.Code, Resource: scope.Resource, Action: scope.Action,
		})
	}
	return responses
}

func handleDBError(c *fiber.Ctx, err error) error {
	if errors.Is(err, ErrClientUnavailable) || errors.Is(err, ErrScopeUnavailable) || errors.Is(err, ErrRateLimitUnavailable) {
		return response.Error(c, fiber.StatusUnprocessableEntity, "invalid_reference", err.Error())
	}
	if apiErr, ok := dberror.MapDBError(err); ok {
		return response.Error(c, apiErr.Status, apiErr.Code, apiErr.Message)
	}
	return response.InternalServerError(c)
}
