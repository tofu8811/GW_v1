package users

import (
	"errors"
	"net/mail"
	"strings"

	"gateway-api/helper/dberror"
	"gateway-api/helper/pagination"
	passwordhelper "gateway-api/helper/password"
	"gateway-api/helper/response"
	"gateway-api/helper/validation"
	"gateway-api/internal/middleware"

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

func (h *Handler) Update(c *fiber.Ctx) error {
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

	var req UpdateUserRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "invalid request body")
	}

	if req.Username != nil {
		username, err := normalizeUsername(*req.Username)
		if err != nil {
			return response.BadRequest(c, err.Error())
		}
		item.Username = username
	}
	if req.Email != nil {
		email, err := normalizeEmail(*req.Email)
		if err != nil {
			return response.BadRequest(c, err.Error())
		}
		item.Email = email
	}
	if req.RoleID != nil {
		roleID, err := validation.ParseRequiredUUID("role_id", *req.RoleID)
		if err != nil {
			return response.BadRequest(c, err.Error())
		}
		item.RoleID = roleID
	}
	if req.IsActive != nil {
		if !*req.IsActive && middleware.GetUserID(c) == item.ID.String() {
			return response.BadRequest(c, "cannot deactivate your own user")
		}
		item.IsActive = *req.IsActive
	}

	var passwordHash *string
	if req.Password != nil {
		hash, err := normalizeAndHashPassword(*req.Password)
		if err != nil {
			return response.BadRequest(c, err.Error())
		}
		passwordHash = &hash
	}

	if err := h.repository.Update(c.Context(), item, passwordHash); err != nil {
		if errors.Is(err, ErrUserNotFound) {
			return response.NotFound(c, "user not found")
		}
		return handleDBError(c, err)
	}

	updated, err := h.repository.FindByID(c.Context(), item.ID)
	if err != nil {
		return response.InternalServerError(c)
	}
	return response.OK(c, toResponse(*updated))
}

func (h *Handler) Delete(c *fiber.Ctx) error {
	id, err := validation.ParseRequiredUUID("id", c.Params("id"))
	if err != nil {
		return response.BadRequest(c, err.Error())
	}
	if middleware.GetUserID(c) == id.String() {
		return response.BadRequest(c, "cannot delete your own user")
	}

	err = h.repository.Delete(c.Context(), id)
	if errors.Is(err, ErrUserNotFound) {
		return response.NotFound(c, "user not found")
	}
	if err != nil {
		return handleDBError(c, err)
	}
	return response.NoContent(c)
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

func normalizeUsername(username string) (string, error) {
	normalized := strings.TrimSpace(username)
	if normalized == "" {
		return "", validation.FieldError{Field: "username", Message: "username is required"}
	}
	if len(normalized) > 50 {
		return "", validation.FieldError{Field: "username", Message: "username must be at most 50 characters"}
	}
	return normalized, nil
}

func normalizeEmail(email string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(email))
	if normalized == "" {
		return "", validation.FieldError{Field: "email", Message: "email is required"}
	}
	if len(normalized) > 255 {
		return "", validation.FieldError{Field: "email", Message: "email must be at most 255 characters"}
	}
	if _, err := mail.ParseAddress(normalized); err != nil {
		return "", validation.FieldError{Field: "email", Message: "email is invalid"}
	}
	return normalized, nil
}

func normalizeAndHashPassword(password string) (string, error) {
	normalized := strings.TrimSpace(password)
	if normalized == "" {
		return "", validation.FieldError{Field: "password", Message: "password is required"}
	}
	if len(normalized) < 6 {
		return "", validation.FieldError{Field: "password", Message: "password must be at least 6 characters"}
	}
	return passwordhelper.HashPassword(normalized)
}

func handleDBError(c *fiber.Ctx, err error) error {
	if apiErr, ok := dberror.MapDBError(err); ok {
		return response.Error(c, apiErr.Status, apiErr.Code, apiErr.Message)
	}
	return response.InternalServerError(c)
}
