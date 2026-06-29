package auth

import (
	"context"
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
	repository   *Repository
	refreshStore *RefreshStore
	jwtSecret    string
	accessTTL    time.Duration
	refreshTTL   time.Duration
	issuer       string
}

func NewHandler(repository *Repository, refreshStore *RefreshStore, jwtSecret string, accessTTL time.Duration, refreshTTL time.Duration, issuer string) *Handler {
	return &Handler{
		repository:   repository,
		refreshStore: refreshStore,
		jwtSecret:    jwtSecret,
		accessTTL:    accessTTL,
		refreshTTL:   refreshTTL,
		issuer:       issuer,
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

	tokens, err := h.issueTokenPair(c.Context(), *user)
	if err != nil {
		return response.InternalServerError(c)
	}

	return response.OK(c, LoginResponse{
		TokenResponse: tokens,
		User:          toUserResponse(*user),
	})
}

func (h *Handler) Refresh(c *fiber.Ctx) error {
	var req RefreshRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "invalid request body")
	}

	refreshToken := strings.TrimSpace(req.RefreshToken)
	if refreshToken == "" {
		return response.BadRequest(c, "refresh_token is required")
	}

	userID, err := h.refreshStore.Consume(c.Context(), refreshToken)
	if errors.Is(err, ErrRefreshTokenNotFound) {
		return response.Unauthorized(c, "invalid or expired refresh token")
	}
	if err != nil {
		return response.InternalServerError(c)
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

	tokens, err := h.issueTokenPair(c.Context(), *user)
	if err != nil {
		return response.InternalServerError(c)
	}

	return response.OK(c, tokens)
}

func (h *Handler) Logout(c *fiber.Ctx) error {
	var req LogoutRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "invalid request body")
	}

	refreshToken := strings.TrimSpace(req.RefreshToken)
	if refreshToken == "" {
		return response.BadRequest(c, "refresh_token is required")
	}

	claims, ok := middleware.GetJWTClaims(c)
	if !ok || claims.ID == "" || claims.UserID == "" || claims.ExpiresAt == nil {
		return response.Unauthorized(c, "invalid or expired token")
	}

	remainingTTL := time.Until(claims.ExpiresAt.Time)
	if remainingTTL <= 0 {
		return response.Unauthorized(c, "invalid or expired token")
	}

	revoked, err := h.refreshStore.RevokeSession(
		c.Context(),
		refreshToken,
		claims.UserID,
		claims.ID,
		remainingTTL,
	)
	if err != nil {
		return response.InternalServerError(c)
	}
	if !revoked {
		return response.Unauthorized(c, "invalid refresh token")
	}

	return response.OK(c, fiber.Map{"logged_out": true})
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

func (h *Handler) issueTokenPair(ctx context.Context, user User) (TokenResponse, error) {
	accessToken, err := tokenhelper.GenerateAccessToken(tokenhelper.AccessTokenInput{
		UserID:      user.ID.String(),
		Role:        user.Role,
		Permissions: user.Permissions,
		TTL:         h.accessTTL,
		Secret:      h.jwtSecret,
		Issuer:      h.issuer,
	})
	if err != nil {
		return TokenResponse{}, err
	}

	refreshToken, err := tokenhelper.GenerateRefreshToken()
	if err != nil {
		return TokenResponse{}, err
	}
	if err := h.refreshStore.Save(ctx, refreshToken, user.ID.String(), h.refreshTTL); err != nil {
		return TokenResponse{}, err
	}

	return TokenResponse{
		AccessToken:      accessToken,
		RefreshToken:     refreshToken,
		TokenType:        tokenTypeBearer,
		ExpiresIn:        int64(h.accessTTL.Seconds()),
		RefreshExpiresIn: int64(h.refreshTTL.Seconds()),
	}, nil
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
