package apikeys

import "time"

type CreateAPIKeyRequest struct {
	Label       *string    `json:"label"`
	ClientID    string     `json:"client_id"`
	ScopeIDs    []string   `json:"scope_ids"`
	RateLimitID *string    `json:"rate_limit_id"`
	ExpiresAt   *time.Time `json:"expires_at"`
	IsActive    *bool      `json:"is_active"`
}

type UpdateAPIKeyRequest struct {
	Label       *string   `json:"label"`
	ClientID    *string   `json:"client_id"`
	ScopeIDs    *[]string `json:"scope_ids"`
	RateLimitID *string   `json:"rate_limit_id"`
	IsActive    *bool     `json:"is_active"`
}

type APIKeyResponse struct {
	ID          string                `json:"id"`
	KeyPrefix   string                `json:"key_prefix"`
	Label       *string               `json:"label"`
	ClientID    string                `json:"client_id"`
	Scopes      []APIKeyScopeResponse `json:"scopes"`
	RateLimitID *string               `json:"rate_limit_id"`
	ExpiresAt   *time.Time            `json:"expires_at"`
	IsActive    bool                  `json:"is_active"`
	RevokedAt   *time.Time            `json:"revoked_at"`
	LastUsedAt  *time.Time            `json:"last_used_at"`
	CreatedBy   *string               `json:"created_by"`
	CreatedAt   time.Time             `json:"created_at"`
	UpdatedAt   time.Time             `json:"updated_at"`
}

type APIKeyScopeResponse struct {
	ID       string `json:"id"`
	Code     string `json:"code"`
	Resource string `json:"resource"`
	Action   string `json:"action"`
}

type APIKeyOptionResponse struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type APIKeyScopeOptionResponse struct {
	ID       string `json:"id"`
	Code     string `json:"code"`
	Resource string `json:"resource"`
	Action   string `json:"action"`
}

type APIKeyOptionsResponse struct {
	Clients    []APIKeyOptionResponse      `json:"clients"`
	Scopes     []APIKeyScopeOptionResponse `json:"api_scopes"`
	RateLimits []APIKeyOptionResponse      `json:"rate_limits"`
}

type CreatedAPIKeyResponse struct {
	APIKeyResponse
	Key string `json:"key"`
}
