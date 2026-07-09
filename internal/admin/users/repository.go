package users

import (
	"context"
	"errors"

	"gateway-api/helper/pagination"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrUserNotFound = errors.New("user not found")

type Repository struct{ db *pgxpool.Pool }

func NewRepository(db *pgxpool.Pool) *Repository { return &Repository{db: db} }

func (r *Repository) FindAll(ctx context.Context, p pagination.Pagination) ([]User, error) {
	rows, err := r.db.Query(ctx, `
		SELECT u.id, u.username, u.email, u.role_id, r.name, u.is_active, u.created_at, u.updated_at
		FROM users u
		JOIN roles r ON r.id = u.role_id
		WHERE u.deleted_at IS NULL
		ORDER BY u.username
		LIMIT $1 OFFSET $2
	`, p.Limit, p.Offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]User, 0)
	for rows.Next() {
		var item User
		if err := rows.Scan(
			&item.ID,
			&item.Username,
			&item.Email,
			&item.RoleID,
			&item.RoleName,
			&item.IsActive,
			&item.CreatedAt,
			&item.UpdatedAt,
		); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *Repository) Count(ctx context.Context) (int64, error) {
	var total int64
	err := r.db.QueryRow(ctx, `SELECT count(*) FROM users WHERE deleted_at IS NULL`).Scan(&total)
	return total, err
}

func (r *Repository) FindByID(ctx context.Context, id uuid.UUID) (*User, error) {
	var item User
	err := r.db.QueryRow(ctx, `
		SELECT u.id, u.username, u.email, u.role_id, r.name, u.is_active, u.created_at, u.updated_at
		FROM users u
		JOIN roles r ON r.id = u.role_id
		WHERE u.id = $1 AND u.deleted_at IS NULL
	`, id).Scan(
		&item.ID,
		&item.Username,
		&item.Email,
		&item.RoleID,
		&item.RoleName,
		&item.IsActive,
		&item.CreatedAt,
		&item.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrUserNotFound
	}
	return &item, err
}

func (r *Repository) Update(ctx context.Context, user *User, passwordHash *string) error {
	err := r.db.QueryRow(ctx, `
		UPDATE users
		SET username = $2,
		    email = $3,
		    role_id = $4,
		    is_active = $5,
		    password_hash = COALESCE($6, password_hash)
		WHERE id = $1 AND deleted_at IS NULL
		RETURNING updated_at
	`, user.ID, user.Username, user.Email, user.RoleID, user.IsActive, passwordHash).Scan(&user.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrUserNotFound
	}
	return err
}

func (r *Repository) Delete(ctx context.Context, id uuid.UUID) error {
	result, err := r.db.Exec(ctx, `
		UPDATE users
		SET is_active = FALSE, deleted_at = now()
		WHERE id = $1 AND deleted_at IS NULL
	`, id)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return ErrUserNotFound
	}
	return nil
}
