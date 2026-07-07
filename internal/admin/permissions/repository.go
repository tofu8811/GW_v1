package permissions

import (
	"context"
	"errors"

	"gateway-api/helper/pagination"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrPermissionNotFound = errors.New("permission not found")

type Repository struct{ db *pgxpool.Pool }

func NewRepository(db *pgxpool.Pool) *Repository { return &Repository{db: db} }

func (r *Repository) FindAll(ctx context.Context, p pagination.Pagination) ([]Permission, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, resource, action, is_active, created_at, updated_at
		FROM permissions
		WHERE deleted_at IS NULL
		ORDER BY resource, action
		LIMIT $1 OFFSET $2
	`, p.Limit, p.Offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]Permission, 0)
	for rows.Next() {
		var item Permission
		if err := rows.Scan(&item.ID, &item.Resource, &item.Action, &item.IsActive, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *Repository) Count(ctx context.Context) (int64, error) {
	var total int64
	err := r.db.QueryRow(ctx, `SELECT count(*) FROM permissions WHERE deleted_at IS NULL`).Scan(&total)
	return total, err
}

func (r *Repository) FindByID(ctx context.Context, id uuid.UUID) (*Permission, error) {
	var item Permission
	err := r.db.QueryRow(ctx, `
		SELECT id, resource, action, is_active, created_at, updated_at
		FROM permissions
		WHERE id = $1 AND deleted_at IS NULL
	`, id).Scan(&item.ID, &item.Resource, &item.Action, &item.IsActive, &item.CreatedAt, &item.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrPermissionNotFound
	}
	return &item, err
}
