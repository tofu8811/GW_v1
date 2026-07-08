package routes

import (
	"encoding/json"
	"time"
)

type CreateRouteRequest struct {
	Path            string  `json:"path"`
	Method          string  `json:"method"`
	ServiceID       string  `json:"service_id"`
	StripPrefix     *bool   `json:"strip_prefix"`
	RewriteTarget   *string `json:"rewrite_target"`
	AuthRequired    *bool   `json:"auth_required"`
	RequiredScopeID *string `json:"required_scope_id"`
	RateLimitID     *string `json:"rate_limit_id"`
	Priority        *int    `json:"priority"`
	IsActive        *bool   `json:"is_active"`
}

type UpdateRouteRequest struct {
	Path            *string             `json:"path"`
	Method          *string             `json:"method"`
	ServiceID       *string             `json:"service_id"`
	StripPrefix     *bool               `json:"strip_prefix"`
	RewriteTarget   *string             `json:"rewrite_target"`
	AuthRequired    *bool               `json:"auth_required"`
	RequiredScopeID NullableStringField `json:"required_scope_id"`
	RateLimitID     *string             `json:"rate_limit_id"`
	Priority        *int                `json:"priority"`
	IsActive        *bool               `json:"is_active"`
}

type NullableStringField struct {
	Set   bool
	Value *string
}

func (f *NullableStringField) UnmarshalJSON(data []byte) error {
	f.Set = true
	if string(data) == "null" {
		f.Value = nil
		return nil
	}

	var value string
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	f.Value = &value
	return nil
}

type RouteResponse struct {
	ID              string    `json:"id"`
	Path            string    `json:"path"`
	Method          string    `json:"method"`
	ServiceID       string    `json:"service_id"`
	StripPrefix     bool      `json:"strip_prefix"`
	RewriteTarget   *string   `json:"rewrite_target"`
	AuthRequired    bool      `json:"auth_required"`
	RequiredScopeID *string   `json:"required_scope_id"`
	RateLimitID     *string   `json:"rate_limit_id"`
	Priority        int       `json:"priority"`
	IsActive        bool      `json:"is_active"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}
