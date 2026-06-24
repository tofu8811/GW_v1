package pagination

import "github.com/gofiber/fiber/v2"

const (
	DefaultPage  = 1
	DefaultLimit = 10
	MaxLimit     = 100
	MaxPage		 = 100
)

type Pagination struct {
	Page   int `json:"page"`
	Limit  int `json:"limit"`
	Offset int `json:"-"` // "-" : ẩn khỏi response
}

type Meta struct {
	Page       int   `json:"page"`
	Limit      int   `json:"limit"`
	Total      int64 `json:"total"`
	TotalPages int   `json:"total_pages"`
	HasNext    bool  `json:"has_next"`
	HasPrev    bool  `json:"has_prev"`
}

// lấy page, limit từ query
func FromQuery(c *fiber.Ctx) Pagination {
	page := c.QueryInt("page", DefaultPage)
	limit := c.QueryInt("limit", DefaultLimit)

	return New(page, limit)
}

func New(page int, limit int) Pagination {
	if page < 1 {
		page = DefaultPage
	}

	if limit < 1 {
		limit = DefaultLimit
	}

	if limit > MaxLimit {
		limit = MaxLimit
	}

	if page > MaxPage {
		page = MaxPage
	}

	return Pagination{
		Page:   page,
		Limit:  limit,
		Offset: (page - 1) * limit, // vị trí bắt đầu query
	}
}

func NewMeta(p Pagination, total int64) Meta {
	totalPages := 0
	if total > 0 {
		totalPages = int((total + int64(p.Limit) - 1) / int64(p.Limit))
	}

	return Meta{
		Page:       p.Page,
		Limit:      p.Limit,
		Total:      total,
		TotalPages: totalPages,
		HasNext:    p.Page < totalPages,
		HasPrev:    p.Page > 1,
	}
}

// sql có limit/ offset
// cách dùng : response.WithMeta(c, items, pagination.NewMeta(p, total))