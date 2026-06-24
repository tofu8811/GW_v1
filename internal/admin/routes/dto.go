package routes

import "time"

type CreateRouteRequest struct {
	Path          string  `json:"path"`
	Method        string  `json:"method"`
	ServiceID     string  `json:"service_id"`
	StripPrefix   *bool   `json:"strip_prefix"`
	RewriteTarget *string `json:"rewrite_target"`
	AuthRequired  *bool   `json:"auth_required"`
	RateLimitID   *string `json:"rate_limit_id"`
	Priority      *int    `json:"priority"`
	IsActive      *bool   `json:"is_active"`
}

type UpdateRouteRequest struct {
	Path          *string `json:"path"`
	Method        *string `json:"method"`
	ServiceID     *string `json:"service_id"`
	StripPrefix   *bool   `json:"strip_prefix"`
	RewriteTarget *string `json:"rewrite_target"`
	AuthRequired  *bool   `json:"auth_required"`
	RateLimitID   *string `json:"rate_limit_id"`
	Priority      *int    `json:"priority"`
	IsActive      *bool   `json:"is_active"`
}

type RouteResponse struct {
	ID            string    `json:"id"`
	Path          string    `json:"path"`
	Method        string    `json:"method"`
	ServiceID     string    `json:"service_id"`
	StripPrefix   bool      `json:"strip_prefix"`
	RewriteTarget *string   `json:"rewrite_target"`
	AuthRequired  bool      `json:"auth_required"`
	RateLimitID   *string   `json:"rate_limit_id"`
	Priority      int       `json:"priority"`
	IsActive      bool      `json:"is_active"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}