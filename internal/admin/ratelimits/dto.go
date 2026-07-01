package ratelimits

import "time"

type CreateRateLimitPolicyRequest struct {
	Name          string `json:"name"`
	LimitType     string `json:"limit_type"`
	MaxRequests   int    `json:"max_requests"`
	WindowSeconds int    `json:"window_seconds"`
	IsActive      *bool  `json:"is_active"`
}

type UpdateRateLimitPolicyRequest struct {
	Name          *string `json:"name"`
	LimitType     *string `json:"limit_type"`
	MaxRequests   *int    `json:"max_requests"`
	WindowSeconds *int    `json:"window_seconds"`
	IsActive      *bool   `json:"is_active"`
}

type RateLimitPolicyResponse struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	LimitType     string    `json:"limit_type"`
	MaxRequests   int       `json:"max_requests"`
	WindowSeconds int       `json:"window_seconds"`
	IsActive      bool      `json:"is_active"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}
