package roles

import "time"

type RoleResponse struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description *string   `json:"description,omitempty"`
	IsActive    bool      `json:"is_active"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type RolePermissionResponse struct {
	Name     string `json:"name"`
	Resource string `json:"resource"`
	Action   string `json:"action"`
}
