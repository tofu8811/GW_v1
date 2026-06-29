package auth

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrUserNotFound = errors.New("user not found")

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

func (r *Repository) FindUserByUsernameOrEmail(ctx context.Context, identifier string) (*User, error) {
	query := `
		SELECT
			u.id,
			u.username,
			u.email,
			u.password_hash,
			r.name,
			COALESCE(
				array_agg(p.resource || ':' || p.action)
				FILTER (WHERE p.id IS NOT NULL),
				'{}'
			) AS permissions,
			u.is_active
		FROM users u
		JOIN roles r ON r.id = u.role_id
		LEFT JOIN role_permissions rp ON rp.role_id = r.id
		LEFT JOIN permissions p ON p.id = rp.permission_id
		WHERE u.username = $1 OR u.email = $1
		GROUP BY u.id, r.name
	`

	var user User
	err := r.db.QueryRow(ctx, query, identifier).Scan(
		&user.ID,
		&user.Username,
		&user.Email,
		&user.PasswordHash,
		&user.Role,
		&user.Permissions,
		&user.IsActive,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, err
	}

	return &user, nil
}

func (r *Repository) FindUserByID(ctx context.Context, userID string) (*User, error) {
	query := `
		SELECT
			u.id,
			u.username,
			u.email,
			u.password_hash,
			r.name,
			COALESCE(
				array_agg(p.resource || ':' || p.action)
				FILTER (WHERE p.id IS NOT NULL),
				'{}'
			) AS permissions,
			u.is_active
		FROM users u
		JOIN roles r ON r.id = u.role_id
		LEFT JOIN role_permissions rp ON rp.role_id = r.id
		LEFT JOIN permissions p ON p.id = rp.permission_id
		WHERE u.id = $1
		GROUP BY u.id, r.name
	`

	var user User
	err := r.db.QueryRow(ctx, query, userID).Scan(
		&user.ID,
		&user.Username,
		&user.Email,
		&user.PasswordHash,
		&user.Role,
		&user.Permissions,
		&user.IsActive,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, err
	}

	return &user, nil
}
