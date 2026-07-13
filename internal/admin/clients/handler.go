package clients

import (
	"context"
	"errors"
	"strings"

	"gateway-api/helper/dberror"
	"gateway-api/helper/idgen"
	"gateway-api/helper/pagination"
	"gateway-api/helper/response"
	"gateway-api/helper/validation"

	"github.com/gofiber/fiber/v2"
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
	var req CreateClientRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "invalid request body")
	}

	name, err := normalizeName(req.Name)
	if err != nil {
		return response.BadRequest(c, err.Error())
	}
	clientType, err := normalizeClientType(req.ClientType)
	if err != nil {
		return response.BadRequest(c, err.Error())
	}
	ownerUserID, err := validation.ParseOptionalUUID("owner_user_id", req.OwnerUserID)
	if err != nil {
		return response.BadRequest(c, err.Error())
	}

	id, err := idgen.NewUUID()
	if err != nil {
		return response.InternalServerError(c)
	}

	client := Client{
		ID:          id,
		Name:        name,
		ClientType:  clientType,
		OwnerUserID: ownerUserID,
		IsActive:    boolValue(req.IsActive, true),
	}

	if err := h.repository.Create(c.Context(), &client); err != nil {
		return handleDBError(c, err)
	}
	if err := h.notifyChange(c); err != nil {
		return response.InternalServerError(c)
	}

	return response.Created(c, toResponse(client))
}

func (h *Handler) FindAll(c *fiber.Ctx) error {
	p := pagination.FromQuery(c)
	search := strings.TrimSpace(c.Query("search"))

	clients, err := h.repository.FindAll(c.Context(), p, search)
	if err != nil {
		return response.InternalServerError(c)
	}

	responses := make([]ClientResponse, 0, len(clients))
	for _, client := range clients {
		responses = append(responses, toResponse(client))
	}

	total, err := h.repository.Count(c.Context(), search)
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

	client, err := h.repository.FindByID(c.Context(), id)
	if errors.Is(err, ErrClientNotFound) {
		return response.NotFound(c, "client not found")
	}
	if err != nil {
		return response.InternalServerError(c)
	}

	return response.OK(c, toResponse(*client))
}

func (h *Handler) Update(c *fiber.Ctx) error {
	id, err := validation.ParseRequiredUUID("id", c.Params("id"))
	if err != nil {
		return response.BadRequest(c, err.Error())
	}

	client, err := h.repository.FindByID(c.Context(), id)
	if errors.Is(err, ErrClientNotFound) {
		return response.NotFound(c, "client not found")
	}
	if err != nil {
		return response.InternalServerError(c)
	}

	var req UpdateClientRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "invalid request body")
	}

	if req.Name != nil {
		name, err := normalizeName(*req.Name)
		if err != nil {
			return response.BadRequest(c, err.Error())
		}
		client.Name = name
	}
	if req.ClientType != nil {
		clientType, err := normalizeClientType(*req.ClientType)
		if err != nil {
			return response.BadRequest(c, err.Error())
		}
		client.ClientType = clientType
	}
	if req.OwnerUserID != nil {
		ownerUserID, err := validation.ParseOptionalUUID("owner_user_id", req.OwnerUserID)
		if err != nil {
			return response.BadRequest(c, err.Error())
		}
		client.OwnerUserID = ownerUserID
	}
	if req.IsActive != nil {
		client.IsActive = *req.IsActive
	}

	if err := h.repository.Update(c.Context(), client); err != nil {
		if errors.Is(err, ErrClientNotFound) {
			return response.NotFound(c, "client not found")
		}
		return handleDBError(c, err)
	}
	if err := h.notifyChange(c); err != nil {
		return response.InternalServerError(c)
	}

	return response.OK(c, toResponse(*client))
}

func (h *Handler) Delete(c *fiber.Ctx) error {
	id, err := validation.ParseRequiredUUID("id", c.Params("id"))
	if err != nil {
		return response.BadRequest(c, err.Error())
	}

	err = h.repository.Delete(c.Context(), id)
	if errors.Is(err, ErrClientNotFound) {
		return response.NotFound(c, "client not found")
	}
	if err != nil {
		return handleDBError(c, err)
	}
	if err := h.notifyChange(c); err != nil {
		return response.InternalServerError(c)
	}

	return response.NoContent(c)
}

func (h *Handler) notifyChange(c *fiber.Ctx) error {
	if h.notifier == nil {
		return nil
	}
	return h.notifier.NotifyChange(c.Context(), "clients")
}

func normalizeName(name string) (string, error) {
	normalized := strings.TrimSpace(name)
	if normalized == "" {
		return "", validation.FieldError{Field: "name", Message: "name is required"}
	}
	return normalized, nil
}

func normalizeClientType(clientType string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(clientType))
	if err := validation.ValidateEnum("client_type", normalized, "internal_app", "partner", "service"); err != nil {
		return "", err
	}
	return normalized, nil
}

func boolValue(value *bool, defaultValue bool) bool {
	if value == nil {
		return defaultValue
	}
	return *value
}

func toResponse(client Client) ClientResponse {
	var ownerUserID *string
	if client.OwnerUserID != nil {
		value := client.OwnerUserID.String()
		ownerUserID = &value
	}

	return ClientResponse{
		ID:          client.ID.String(),
		Name:        client.Name,
		ClientType:  client.ClientType,
		OwnerUserID: ownerUserID,
		IsActive:    client.IsActive,
		CreatedAt:   client.CreatedAt,
		UpdatedAt:   client.UpdatedAt,
	}
}

func handleDBError(c *fiber.Ctx, err error) error {
	if apiErr, ok := dberror.MapDBError(err); ok {
		return response.Error(c, apiErr.Status, apiErr.Code, apiErr.Message)
	}

	return response.InternalServerError(c)
}
