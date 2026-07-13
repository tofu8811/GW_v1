package roles

import (
	"context"
	"errors"

	"gateway-api/helper/pagination"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrRoleNotFound = errors.New("role not found")

type Repository struct{ db *pgxpool.Pool }

func NewRepository(db *pgxpool.Pool) *Repository { return &Repository{db: db} }

func (r *Repository) FindAll(ctx context.Context, p pagination.Pagination) ([]Role, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, name, description, is_active, created_at, updated_at
		FROM roles
		WHERE deleted_at IS NULL
		ORDER BY name
		LIMIT $1 OFFSET $2
	`, p.Limit, p.Offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]Role, 0)
	for rows.Next() {
		var item Role
		if err := rows.Scan(&item.ID, &item.Name, &item.Description, &item.IsActive, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *Repository) Count(ctx context.Context) (int64, error) {
	var total int64
	err := r.db.QueryRow(ctx, `SELECT count(*) FROM roles WHERE deleted_at IS NULL`).Scan(&total)
	return total, err
}

func (r *Repository) FindByID(ctx context.Context, id uuid.UUID) (*Role, error) {
	var item Role
	err := r.db.QueryRow(ctx, `
		SELECT id, name, description, is_active, created_at, updated_at
		FROM roles
		WHERE id = $1 AND deleted_at IS NULL
	`, id).Scan(&item.ID, &item.Name, &item.Description, &item.IsActive, &item.CreatedAt, &item.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrRoleNotFound
	}
	return &item, err
}

func (r *Repository) FindPermissions(ctx context.Context, roleID uuid.UUID) ([]Permission, error) {
	var exists bool
	if err := r.db.QueryRow(ctx, `SELECT EXISTS (
		SELECT 1 FROM roles WHERE id = $1 AND deleted_at IS NULL
	)`, roleID).Scan(&exists); err != nil {
		return nil, err
	}
	if !exists {
		return nil, ErrRoleNotFound
	}
	rows, err := r.db.Query(ctx, `
		SELECT p.id, p.resource, p.action
		FROM role_permissions rp
		JOIN permissions p ON p.id = rp.permission_id
		WHERE rp.role_id = $1
		  AND p.deleted_at IS NULL
		ORDER BY p.resource, p.action
	`, roleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]Permission, 0)
	for rows.Next() {
		var item Permission
		if err := rows.Scan(&item.ID, &item.Resource, &item.Action); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}
