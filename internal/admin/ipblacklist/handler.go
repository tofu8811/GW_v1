package ipblacklist

import (
	"context"
	"errors"
	"strings"

	"gateway-api/helper/dberror"
	"gateway-api/helper/idgen"
	"gateway-api/helper/pagination"
	"gateway-api/helper/response"
	"gateway-api/helper/validation"
	runtimeipblacklist "gateway-api/internal/security/ipblacklist"

	"github.com/gofiber/fiber/v2"
)

type Handler struct {
	repository *Repository
	reloader   Reloader
}

type Reloader interface {
	Reload(ctx context.Context) error
}

func NewHandler(repository *Repository, reloader Reloader) *Handler {
	return &Handler{repository: repository, reloader: reloader}
}

func (h *Handler) Create(c *fiber.Ctx) error {
	var req CreateIPBlacklistRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "invalid request body")
	}

	ipOrCIDR, err := validation.NormalizeCIDROrIP("ip_or_cidr", req.IPOrCIDR)
	if err != nil {
		return response.BadRequest(c, err.Error())
	}
	createdBy, err := validation.ParseOptionalUUID("created_by", req.CreatedBy)
	if err != nil {
		return response.BadRequest(c, err.Error())
	}
	expiresAt, err := validation.ParseOptionalTimestamp("expires_at", req.ExpiresAt)
	if err != nil {
		return response.BadRequest(c, err.Error())
	}

	id, err := idgen.NewUUID()
	if err != nil {
		return response.InternalServerError(c)
	}

	entry := IPBlacklistEntry{
		ID:        id,
		IPOrCIDR:  ipOrCIDR,
		Reason:    stringPtr(req.Reason),
		CreatedBy: createdBy,
		ExpiresAt: expiresAt,
		IsActive:  boolValue(req.IsActive, true),
	}
	if runtimeipblacklist.NowExpired(entry.ExpiresAt) {
		entry.IsActive = false
	}

	if err := h.repository.Create(c.Context(), &entry); err != nil {
		return handleDBError(c, err)
	}
	if err := h.reload(c); err != nil {
		return response.InternalServerError(c)
	}

	return response.Created(c, toResponse(entry))
}

func (h *Handler) FindAll(c *fiber.Ctx) error {
	p := pagination.FromQuery(c)
	filter := ListFilter{
		IncludeDeleted: c.QueryBool("include_deleted", false),
		DeletedOnly:    c.QueryBool("deleted_only", false),
	}

	entries, err := h.repository.FindAll(c.Context(), p, filter)
	if err != nil {
		return response.InternalServerError(c)
	}

	responses := make([]IPBlacklistResponse, 0, len(entries))
	for _, entry := range entries {
		responses = append(responses, toResponse(entry))
	}

	total, err := h.repository.Count(c.Context(), filter)
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

	entry, err := h.repository.FindByID(c.Context(), id)
	if errors.Is(err, ErrIPBlacklistEntryNotFound) {
		return response.NotFound(c, "IP blacklist entry not found")
	}
	if err != nil {
		return response.InternalServerError(c)
	}

	return response.OK(c, toResponse(*entry))
}

func (h *Handler) Update(c *fiber.Ctx) error {
	id, err := validation.ParseRequiredUUID("id", c.Params("id"))
	if err != nil {
		return response.BadRequest(c, err.Error())
	}

	entry, err := h.repository.FindByID(c.Context(), id)
	if errors.Is(err, ErrIPBlacklistEntryNotFound) {
		return response.NotFound(c, "IP blacklist entry not found")
	}
	if err != nil {
		return response.InternalServerError(c)
	}

	var req UpdateIPBlacklistRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "invalid request body")
	}

	if req.IPOrCIDR != nil {
		ipOrCIDR, err := validation.NormalizeCIDROrIP("ip_or_cidr", *req.IPOrCIDR)
		if err != nil {
			return response.BadRequest(c, err.Error())
		}
		entry.IPOrCIDR = ipOrCIDR
	}
	if req.Reason != nil {
		entry.Reason = stringPtr(req.Reason)
	}
	if req.CreatedBy != nil {
		createdBy, err := validation.ParseOptionalUUID("created_by", req.CreatedBy)
		if err != nil {
			return response.BadRequest(c, err.Error())
		}
		entry.CreatedBy = createdBy
	}
	if req.ExpiresAt != nil {
		expiresAt, err := validation.ParseOptionalTimestamp("expires_at", req.ExpiresAt)
		if err != nil {
			return response.BadRequest(c, err.Error())
		}
		entry.ExpiresAt = expiresAt
	}
	if req.IsActive != nil {
		entry.IsActive = *req.IsActive
	}
	if runtimeipblacklist.NowExpired(entry.ExpiresAt) {
		entry.IsActive = false
	}

	if err := h.repository.Update(c.Context(), entry); err != nil {
		if errors.Is(err, ErrIPBlacklistEntryNotFound) {
			return response.NotFound(c, "IP blacklist entry not found")
		}
		return handleDBError(c, err)
	}
	if err := h.reload(c); err != nil {
		return response.InternalServerError(c)
	}

	return response.OK(c, toResponse(*entry))
}

func (h *Handler) Delete(c *fiber.Ctx) error {
	id, err := validation.ParseRequiredUUID("id", c.Params("id"))
	if err != nil {
		return response.BadRequest(c, err.Error())
	}

	entry, err := h.repository.Delete(c.Context(), id)
	if errors.Is(err, ErrIPBlacklistEntryNotFound) {
		return response.NotFound(c, "IP blacklist entry not found")
	}
	if err != nil {
		return handleDBError(c, err)
	}
	if err := h.reload(c); err != nil {
		return response.InternalServerError(c)
	}

	return response.OK(c, toResponse(*entry))
}

func (h *Handler) reload(c *fiber.Ctx) error {
	if h.reloader == nil {
		return nil
	}
	return h.reloader.Reload(c.Context())
}

func boolValue(value *bool, defaultValue bool) bool {
	if value == nil {
		return defaultValue
	}
	return *value
}

func stringPtr(value *string) *string {
	if value == nil || strings.TrimSpace(*value) == "" {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	return &trimmed
}

func toResponse(entry IPBlacklistEntry) IPBlacklistResponse {
	var createdBy *string
	if entry.CreatedBy != nil {
		value := entry.CreatedBy.String()
		createdBy = &value
	}

	return IPBlacklistResponse{
		ID:        entry.ID.String(),
		IPOrCIDR:  entry.IPOrCIDR,
		Reason:    entry.Reason,
		CreatedBy: createdBy,
		ExpiresAt: entry.ExpiresAt,
		IsActive:  entry.IsActive,
		CreatedAt: entry.CreatedAt,
		UpdatedAt: entry.UpdatedAt,
		DeletedAt: entry.DeletedAt,
	}
}

func handleDBError(c *fiber.Ctx, err error) error {
	if apiErr, ok := dberror.MapDBError(err); ok {
		return response.Error(c, apiErr.Status, apiErr.Code, apiErr.Message)
	}

	return response.InternalServerError(c)
}
