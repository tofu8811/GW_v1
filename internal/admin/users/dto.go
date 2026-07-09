package users

import "time"

type UpdateUserRequest struct {
	Username *string `json:"username"`
	Email    *string `json:"email"`
	RoleID   *string `json:"role_id"`
	Password *string `json:"password"`
	IsActive *bool   `json:"is_active"`
}

type UserResponse struct {
	ID        string    `json:"id"`
	Username  string    `json:"username"`
	Email     string    `json:"email"`
	RoleID    string    `json:"role_id"`
	RoleName  string    `json:"role_name"`
	IsActive  bool      `json:"is_active"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
