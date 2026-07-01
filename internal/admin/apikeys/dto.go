package apikeys

import "time"

type CreateAPIKeyRequest struct {
	Label       *string    `json:"label"`
	UserID      *string    `json:"user_id"`
	Scopes      []string   `json:"scopes"`
	RateLimitID *string    `json:"rate_limit_id"`
	ExpiresAt   *time.Time `json:"expires_at"`
	IsActive    *bool      `json:"is_active"`
}

type UpdateAPIKeyRequest struct {
	Label       *string    `json:"label"`
	UserID      *string    `json:"user_id"`
	Scopes      *[]string  `json:"scopes"`
	RateLimitID *string    `json:"rate_limit_id"`
	ExpiresAt   *time.Time `json:"expires_at"`
	IsActive    *bool      `json:"is_active"`
}

type APIKeyResponse struct {
	ID          string     `json:"id"`
	KeyPrefix   string     `json:"key_prefix"`
	Label       *string    `json:"label"`
	UserID      *string    `json:"user_id"`
	Scopes      []string   `json:"scopes"`
	RateLimitID *string    `json:"rate_limit_id"`
	ExpiresAt   *time.Time `json:"expires_at"`
	IsActive    bool       `json:"is_active"`
	LastUsedAt  *time.Time `json:"last_used_at"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

type CreatedAPIKeyResponse struct {
	APIKeyResponse
	Key string `json:"key"`
}
