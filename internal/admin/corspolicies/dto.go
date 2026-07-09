package corspolicies

import "time"

type CreateCORSPolicyRequest struct {
	Name             string   `json:"name"`
	AllowedOrigins   []string `json:"allowed_origins"`
	AllowedMethods   []string `json:"allowed_methods"`
	AllowedHeaders   []string `json:"allowed_headers"`
	ExposedHeaders   []string `json:"exposed_headers"`
	AllowCredentials bool     `json:"allow_credentials"`
	MaxAge           *int     `json:"max_age"`
	IsActive         *bool    `json:"is_active"`
}

type UpdateCORSPolicyRequest struct {
	Name             *string  `json:"name"`
	AllowedOrigins   []string `json:"allowed_origins"`
	AllowedMethods   []string `json:"allowed_methods"`
	AllowedHeaders   []string `json:"allowed_headers"`
	ExposedHeaders   []string `json:"exposed_headers"`
	AllowCredentials *bool    `json:"allow_credentials"`
	MaxAge           *int     `json:"max_age"`
	IsActive         *bool    `json:"is_active"`
}

type CORSPolicyResponse struct {
	ID               string    `json:"id"`
	Name             string    `json:"name"`
	AllowedOrigins   []string  `json:"allowed_origins"`
	AllowedMethods   []string  `json:"allowed_methods"`
	AllowedHeaders   []string  `json:"allowed_headers"`
	ExposedHeaders   []string  `json:"exposed_headers"`
	AllowCredentials bool      `json:"allow_credentials"`
	MaxAge           int       `json:"max_age"`
	IsActive         bool      `json:"is_active"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}
