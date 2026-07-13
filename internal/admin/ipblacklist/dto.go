package ipblacklist

import "time"

type CreateIPBlacklistRequest struct {
	IPOrCIDR  string  `json:"ip_or_cidr"`
	Reason    *string `json:"reason"`
	CreatedBy *string `json:"created_by"`
	ExpiresAt *string `json:"expires_at"`
	IsActive  *bool   `json:"is_active"`
}

type UpdateIPBlacklistRequest struct {
	IPOrCIDR  *string `json:"ip_or_cidr"`
	Reason    *string `json:"reason"`
	CreatedBy *string `json:"created_by"`
	ExpiresAt *string `json:"expires_at"`
	IsActive  *bool   `json:"is_active"`
}

type IPBlacklistResponse struct {
	ID        string     `json:"id"`
	IPOrCIDR  string     `json:"ip_or_cidr"`
	Reason    *string    `json:"reason,omitempty"`
	CreatedBy *string    `json:"created_by,omitempty"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
	IsActive  bool       `json:"is_active"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	DeletedAt *time.Time `json:"deleted_at,omitempty"`
}
