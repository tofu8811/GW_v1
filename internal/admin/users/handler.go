package users

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

	result := make([]UserResponse, 0, len(items))
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
	if errors.Is(err, ErrUserNotFound) {
		return response.NotFound(c, "user not found")
	}
	if err != nil {
		return response.InternalServerError(c)
	}
	return response.OK(c, toResponse(*item))
}

func toResponse(item User) UserResponse {
	return UserResponse{
		ID:        item.ID.String(),
		Username:  item.Username,
		Email:     item.Email,
		RoleID:    item.RoleID.String(),
		RoleName:  item.RoleName,
		IsActive:  item.IsActive,
		CreatedAt: item.CreatedAt,
		UpdatedAt: item.UpdatedAt,
	}
}
