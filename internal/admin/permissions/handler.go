package permissions

import (
	"errors"

	"gateway-api/helper/pagination"
	"gateway-api/helper/response"
	"gateway-api/helper/validation"

	"github.com/gofiber/fiber/v2"
)

type Handler struct{ repository *Repository }

func NewHandler(repository *Repository) *Handler { return &Handler{repository: repository} }

func (h *Handler) FindAll(c *fiber.Ctx) error {
	p := pagination.FromQuery(c)
	items, err := h.repository.FindAll(c.Context(), p)
	if err != nil {
		return response.InternalServerError(c)
	}
	result := make([]PermissionResponse, 0, len(items))
	for _, item := range items {
		result = append(result, toResponse(item))
	}
	total, err := h.repository.Count(c.Context())
	if err != nil {
		return response.InternalServerError(c)
	}
	return response.WithMeta(c, result, pagination.NewMeta(p, total))
}

func (h *Handler) FindByID(c *fiber.Ctx) error {
	id, err := validation.ParseRequiredUUID("id", c.Params("id"))
	if err != nil {
		return response.BadRequest(c, err.Error())
	}
	item, err := h.repository.FindByID(c.Context(), id)
	if errors.Is(err, ErrPermissionNotFound) {
		return response.NotFound(c, "permission not found")
	}
	if err != nil {
		return response.InternalServerError(c)
	}
	return response.OK(c, toResponse(*item))
}

func toResponse(item Permission) PermissionResponse {
	return PermissionResponse{
		ID: item.ID.String(), Name: item.Resource + ":" + item.Action,
		Resource: item.Resource, Action: item.Action, IsActive: item.IsActive,
		CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt,
	}
}
