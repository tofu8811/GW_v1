package instances

import "time"

type CreateInstanceRequest struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Weight   *int16 `json:"weight"`
	IsActive *bool  `json:"is_active"`
}

type UpdateInstanceRequest struct {
	Host      *string `json:"host"`
	Port      *int    `json:"port"`
	Weight    *int16  `json:"weight"`
	IsActive  *bool   `json:"is_active"`
	ServiceID *string `json:"service_id"`
}

type InstanceResponse struct {
	ID        string    `json:"id"`
	ServiceID string    `json:"service_id"`
	Host      string    `json:"host"`
	Port      int       `json:"port"`
	Weight    int16     `json:"weight"`
	IsActive  bool      `json:"is_active"`
	CreatedAt time.Time `json:"created_at"`
}
