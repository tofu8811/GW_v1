package services

import "time"

type CreateServiceRequest struct {
	Name                  string  `json:"name"`
	Description           *string `json:"description"`
	Protocol              string  `json:"protocol"`
	LBStrategy            string  `json:"lb_strategy"`
	TimeoutMS             *int    `json:"timeout_ms"`
	RetryCount            *int16  `json:"retry_count"`
	CircuitBreakerEnabled *bool   `json:"circuit_breaker_enabled"`
	IsActive              *bool   `json:"is_active"`
}

type UpdateServiceRequest struct {
	Name                  *string `json:"name"`
	Description           *string `json:"description"`
	Protocol              *string `json:"protocol"`
	LBStrategy            *string `json:"lb_strategy"`
	TimeoutMS             *int    `json:"timeout_ms"`
	RetryCount            *int16  `json:"retry_count"`
	CircuitBreakerEnabled *bool   `json:"circuit_breaker_enabled"`
	IsActive              *bool   `json:"is_active"`
}

type ServiceResponse struct {
	ID                    string    `json:"id"`
	Name                  string    `json:"name"`
	Description           *string   `json:"description"`
	Protocol              string    `json:"protocol"`
	LBStrategy            string    `json:"lb_strategy"`
	TimeoutMS             int       `json:"timeout_ms"`
	RetryCount            int16     `json:"retry_count"`
	CircuitBreakerEnabled bool      `json:"circuit_breaker_enabled"`
	IsActive              bool      `json:"is_active"`
	CreatedAt             time.Time `json:"created_at"`
	UpdatedAt             time.Time `json:"updated_at"`
}
