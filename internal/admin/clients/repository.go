package clients

import (
	"context"
	"errors"

	"gateway-api/helper/pagination"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrClientNotFound = errors.New("client not found")

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Create(ctx context.Context, client *Client) error {
	query := `
		INSERT INTO clients (
			id, name, client_type, owner_user_id, is_active
		)
		VALUES ($1,$2,$3,$4,$5)
		RETURNING created_at, updated_at
	`

	return r.db.QueryRow(
		ctx,
		query,
		client.ID,
		client.Name,
		client.ClientType,
		nullableUUID(client.OwnerUserID),
		client.IsActive,
	).Scan(&client.CreatedAt, &client.UpdatedAt)
}

func (r *Repository) FindAll(ctx context.Context, p pagination.Pagination, search string) ([]Client, error) {
	query := `
		SELECT id, name, client_type, owner_user_id, is_active, created_at, updated_at
		FROM clients
		WHERE deleted_at IS NULL
		  AND ($3 = '' OR name ILIKE '%' || $3 || '%' OR client_type ILIKE '%' || $3 || '%')
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2
	`

	rows, err := r.db.Query(ctx, query, p.Limit, p.Offset, search)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var clients []Client
	for rows.Next() {
		client, err := scanClient(rows)
		if err != nil {
			return nil, err
		}
		clients = append(clients, client)
	}

	return clients, rows.Err()
}

func (r *Repository) Count(ctx context.Context, search string) (int64, error) {
	var total int64
	err := r.db.QueryRow(ctx, `
		SELECT count(*)
		FROM clients
		WHERE deleted_at IS NULL
		  AND ($1 = '' OR name ILIKE '%' || $1 || '%' OR client_type ILIKE '%' || $1 || '%')
	`, search).Scan(&total)
	return total, err
}

func (r *Repository) FindByID(ctx context.Context, id uuid.UUID) (*Client, error) {
	query := `
		SELECT id, name, client_type, owner_user_id, is_active, created_at, updated_at
		FROM clients
		WHERE id = $1 AND deleted_at IS NULL
	`

	row := r.db.QueryRow(ctx, query, id)
	client, err := scanClient(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrClientNotFound
	}
	if err != nil {
		return nil, err
	}

	return &client, nil
}

func (r *Repository) Update(ctx context.Context, client *Client) error {
	query := `
		UPDATE clients
		SET name = $2,
		    client_type = $3,
		    owner_user_id = $4,
		    is_active = $5
		WHERE id = $1 AND deleted_at IS NULL
		RETURNING updated_at
	`

	err := r.db.QueryRow(
		ctx,
		query,
		client.ID,
		client.Name,
		client.ClientType,
		nullableUUID(client.OwnerUserID),
		client.IsActive,
	).Scan(&client.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrClientNotFound
	}

	return err
}

func (r *Repository) Delete(ctx context.Context, id uuid.UUID) error {
	result, err := r.db.Exec(ctx, `UPDATE clients SET is_active = FALSE, deleted_at = now() WHERE id = $1 AND deleted_at IS NULL`, id)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return ErrClientNotFound
	}

	return nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanClient(row rowScanner) (Client, error) {
	var client Client
	var ownerID pgtype.UUID

	err := row.Scan(
		&client.ID,
		&client.Name,
		&client.ClientType,
		&ownerID,
		&client.IsActive,
		&client.CreatedAt,
		&client.UpdatedAt,
	)
	if err != nil {
		return Client{}, err
	}

	if ownerID.Valid {
		id := uuid.UUID(ownerID.Bytes)
		client.OwnerUserID = &id
	}

	return client, nil
}

func nullableUUID(id *uuid.UUID) any {
	if id == nil {
		return nil
	}
	return *id
}
