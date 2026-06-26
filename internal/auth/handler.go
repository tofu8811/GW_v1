package auth

import (
	"errors"
	"strings"
	"time"

	"gateway-api/helper/password"
	"gateway-api/helper/response"
	tokenhelper "gateway-api/helper/token"
	"gateway-api/internal/middleware"

	"github.com/gofiber/fiber/v2"
)

const tokenTypeBearer = "Bearer"

type Handler struct {
	repository *Repository
	jwtSecret  string
	accessTTL  time.Duration
	issuer     string
}

func NewHandler(repository *Repository, jwtSecret string, accessTTL time.Duration, issuer string) *Handler {
	return &Handler{
		repository: repository,
		jwtSecret:  jwtSecret,
		accessTTL:  accessTTL,
		issuer:     issuer,
	}
}

func (h *Handler) Login(c *fiber.Ctx) error {
	var req LoginRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "invalid request body")
	}

	identifier := strings.TrimSpace(req.Username)
	if identifier == "" {
		return response.BadRequest(c, "username is required")
	}
	if req.Password == "" {
		return response.BadRequest(c, "password is required")
	}

	user, err := h.repository.FindUserByUsernameOrEmail(c.Context(), identifier)
	if errors.Is(err, ErrUserNotFound) {
		return response.Unauthorized(c, "invalid username or password")
	}
	if err != nil {
		return response.InternalServerError(c)
	}
	if !user.IsActive {
		return response.Forbidden(c, "user is inactive")
	}
	if !password.VerifyPassword(req.Password, user.PasswordHash) {
		return response.Unauthorized(c, "invalid username or password")
	}

	accessToken, err := tokenhelper.GenerateAccessToken(tokenhelper.AccessTokenInput{
		UserID:      user.ID.String(),
		Role:        user.Role,
		Permissions: user.Permissions,
		TTL:         h.accessTTL,
		Secret:      h.jwtSecret,
		Issuer:      h.issuer,
	})
	if err != nil {
		return response.InternalServerError(c)
	}

	return response.OK(c, LoginResponse{
		AccessToken: accessToken,
		TokenType:   tokenTypeBearer,
		ExpiresIn:   int64(h.accessTTL.Seconds()),
		User:        toUserResponse(*user),
	})
}

func (h *Handler) Me(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	if userID == "" {
		return response.Unauthorized(c, "invalid or expired token")
	}

	user, err := h.repository.FindUserByID(c.Context(), userID)
	if errors.Is(err, ErrUserNotFound) {
		return response.Unauthorized(c, "user not found")
	}
	if err != nil {
		return response.InternalServerError(c)
	}
	if !user.IsActive {
		return response.Forbidden(c, "user is inactive")
	}

	return response.OK(c, toUserResponse(*user))
}

func toUserResponse(user User) UserResponse {
	return UserResponse{
		ID:          user.ID.String(),
		Username:    user.Username,
		Email:       user.Email,
		Role:        user.Role,
		Permissions: user.Permissions,
	}
}
