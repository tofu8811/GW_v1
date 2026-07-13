package corsconfigs

import "time"

type UpsertCORSConfigRequest struct {
	AllowedOrigins   []string `json:"allowed_origins"`
	AllowedMethods   []string `json:"allowed_methods"`
	AllowedHeaders   []string `json:"allowed_headers"`
	AllowCredentials bool     `json:"allow_credentials"`
	MaxAge           *int     `json:"max_age"`
	IsActive         *bool    `json:"is_active"`
}

type CORSConfigResponse struct {
	ID               string    `json:"id"`
	RouteID          string    `json:"route_id"`
	AllowedOrigins   []string  `json:"allowed_origins"`
	AllowedMethods   []string  `json:"allowed_methods"`
	AllowedHeaders   []string  `json:"allowed_headers"`
	AllowCredentials bool      `json:"allow_credentials"`
	MaxAge           int       `json:"max_age"`
	IsActive         bool      `json:"is_active"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}
