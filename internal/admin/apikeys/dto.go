package apikeys

import "time"

type CreateAPIKeyRequest struct {
	Label         *string    `json:"label"`
	ClientID      string     `json:"client_id"`
	PermissionIDs []string   `json:"permission_ids"`
	RateLimitID   *string    `json:"rate_limit_id"`
	ExpiresAt     *time.Time `json:"expires_at"`
	IsActive      *bool      `json:"is_active"`
}

type UpdateAPIKeyRequest struct {
	Label         *string    `json:"label"`
	ClientID      *string    `json:"client_id"`
	PermissionIDs *[]string  `json:"permission_ids"`
	RateLimitID   *string    `json:"rate_limit_id"`
	ExpiresAt     *time.Time `json:"expires_at"`
	IsActive      *bool      `json:"is_active"`
}

type APIKeyResponse struct {
	ID          string                     `json:"id"`
	KeyPrefix   string                     `json:"key_prefix"`
	Label       *string                    `json:"label"`
	ClientID    string                     `json:"client_id"`
	Permissions []APIKeyPermissionResponse `json:"permissions"`
	RateLimitID *string                    `json:"rate_limit_id"`
	ExpiresAt   *time.Time                 `json:"expires_at"`
	IsActive    bool                       `json:"is_active"`
	RevokedAt   *time.Time                 `json:"revoked_at"`
	LastUsedAt  *time.Time                 `json:"last_used_at"`
	CreatedBy   *string                    `json:"created_by"`
	CreatedAt   time.Time                  `json:"created_at"`
	UpdatedAt   time.Time                  `json:"updated_at"`
}

type APIKeyPermissionResponse struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type APIKeyOptionResponse struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type APIKeyOptionsResponse struct {
	Clients     []APIKeyOptionResponse `json:"clients"`
	Permissions []APIKeyOptionResponse `json:"permissions"`
	RateLimits  []APIKeyOptionResponse `json:"rate_limits"`
}

type CreatedAPIKeyResponse struct {
	APIKeyResponse
	Key string `json:"key"`
}
