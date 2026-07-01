package apikeys

import (
	"errors"
	"strings"
	"time"

	cryptoutil "gateway-api/helper/crypto"
	"gateway-api/helper/dberror"
	"gateway-api/helper/idgen"
	"gateway-api/helper/pagination"
	"gateway-api/helper/response"
	"gateway-api/helper/validation"

	"github.com/gofiber/fiber/v2"
)

const apiKeyPrefix = "gw_live_"

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

	userID, err := validation.ParseOptionalUUID("user_id", req.UserID)
	if err != nil {
		return response.BadRequest(c, err.Error())
	}
	rateLimitID, err := validation.ParseOptionalUUID("rate_limit_id", req.RateLimitID)
	if err != nil {
		return response.BadRequest(c, err.Error())
	}
	scopes, err := normalizeScopes(req.Scopes)
	if err != nil {
		return response.BadRequest(c, err.Error())
	}
	if req.ExpiresAt != nil && !req.ExpiresAt.After(time.Now()) {
		return response.BadRequest(c, "expires_at must be in the future")
	}

	id, err := idgen.NewUUID()
	if err != nil {
		return response.InternalServerError(c)
	}
	secret, err := cryptoutil.GenerateRandomToken()
	if err != nil {
		return response.InternalServerError(c)
	}
	rawKey := apiKeyPrefix + secret
	keyHash, err := cryptoutil.HashAPIKey(rawKey)
	if err != nil {
		return response.InternalServerError(c)
	}

	key := APIKey{
		ID: id, KeyHash: keyHash, KeyPrefix: rawKey[:12], Label: normalizedOptionalString(req.Label),
		UserID: userID, Scopes: scopes, RateLimitID: rateLimitID, ExpiresAt: req.ExpiresAt,
		IsActive: boolValue(req.IsActive, true),
	}
	if err := h.repository.Create(c.Context(), &key); err != nil {
		return handleDBError(c, err)
	}

	return response.Created(c, CreatedAPIKeyResponse{APIKeyResponse: toResponse(key), Key: rawKey})
}

func (h *Handler) FindAll(c *fiber.Ctx) error {
	p := pagination.FromQuery(c)
	keys, err := h.repository.FindAll(c.Context(), p)
	if err != nil {
		return response.InternalServerError(c)
	}
	items := make([]APIKeyResponse, 0, len(keys))
	for _, key := range keys {
		items = append(items, toResponse(key))
	}
	total, err := h.repository.Count(c.Context())
	if err != nil {
		return response.InternalServerError(c)
	}
	return response.WithMeta(c, items, pagination.NewMeta(p, total))
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
	if req.UserID != nil {
		key.UserID, err = validation.ParseOptionalUUID("user_id", req.UserID)
		if err != nil {
			return response.BadRequest(c, err.Error())
		}
	}
	if req.Scopes != nil {
		key.Scopes, err = normalizeScopes(*req.Scopes)
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
	if req.ExpiresAt != nil {
		if !req.ExpiresAt.After(time.Now()) {
			return response.BadRequest(c, "expires_at must be in the future")
		}
		key.ExpiresAt = req.ExpiresAt
	}
	if req.IsActive != nil {
		key.IsActive = *req.IsActive
	}

	if err := h.repository.Update(c.Context(), key); err != nil {
		return handleDBError(c, err)
	}
	return response.OK(c, toResponse(*key))
}

func (h *Handler) Delete(c *fiber.Ctx) error {
	id, err := validation.ParseRequiredUUID("id", c.Params("id"))
	if err != nil {
		return response.BadRequest(c, err.Error())
	}
	if err := h.repository.Delete(c.Context(), id); errors.Is(err, ErrAPIKeyNotFound) {
		return response.NotFound(c, "API key not found")
	} else if err != nil {
		return handleDBError(c, err)
	}
	return response.NoContent(c)
}

func normalizeScopes(scopes []string) ([]string, error) {
	unique := make(map[string]struct{}, len(scopes))
	result := make([]string, 0, len(scopes))
	for _, scope := range scopes {
		normalized := strings.TrimSpace(scope)
		if normalized == "" {
			continue
		}
		if _, exists := unique[normalized]; exists {
			continue
		}
		unique[normalized] = struct{}{}
		result = append(result, normalized)
	}
	if len(result) == 0 {
		return nil, validation.FieldError{Field: "scopes", Message: "at least one scope is required"}
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

func toResponse(key APIKey) APIKeyResponse {
	var userID *string
	if key.UserID != nil {
		value := key.UserID.String()
		userID = &value
	}
	var rateLimitID *string
	if key.RateLimitID != nil {
		value := key.RateLimitID.String()
		rateLimitID = &value
	}
	return APIKeyResponse{
		ID: key.ID.String(), KeyPrefix: key.KeyPrefix, Label: key.Label, UserID: userID,
		Scopes: key.Scopes, RateLimitID: rateLimitID, ExpiresAt: key.ExpiresAt,
		IsActive: key.IsActive, LastUsedAt: key.LastUsedAt,
		CreatedAt: key.CreatedAt, UpdatedAt: key.UpdatedAt,
	}
}

func handleDBError(c *fiber.Ctx, err error) error {
	if apiErr, ok := dberror.MapDBError(err); ok {
		return response.Error(c, apiErr.Status, apiErr.Code, apiErr.Message)
	}
	return response.InternalServerError(c)
}
