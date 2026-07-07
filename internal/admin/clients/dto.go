package clients

import "time"

type CreateClientRequest struct {
	Name        string  `json:"name"`
	ClientType  string  `json:"client_type"`
	OwnerUserID *string `json:"owner_user_id"`
	IsActive    *bool   `json:"is_active"`
}

type UpdateClientRequest struct {
	Name        *string `json:"name"`
	ClientType  *string `json:"client_type"`
	OwnerUserID *string `json:"owner_user_id"`
	IsActive    *bool   `json:"is_active"`
}

type ClientResponse struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	ClientType  string    `json:"client_type"`
	OwnerUserID *string   `json:"owner_user_id"`
	IsActive    bool      `json:"is_active"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}
