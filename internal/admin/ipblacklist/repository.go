package ipblacklist

import (
	"context"
	"errors"

	"gateway-api/helper/pagination"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrIPBlacklistEntryNotFound = errors.New("IP blacklist entry not found")

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Create(ctx context.Context, entry *IPBlacklistEntry) error {
	query := `
		INSERT INTO ip_blacklist (
			id, ip_or_cidr, reason, created_by, expires_at, is_active
		)
		VALUES ($1,$2::cidr,$3,$4,$5,$6)
		RETURNING ip_or_cidr::text, created_at, updated_at, deleted_at
	`

	return r.db.QueryRow(
		ctx,
		query,
		entry.ID,
		entry.IPOrCIDR,
		entry.Reason,
		entry.CreatedBy,
		entry.ExpiresAt,
		entry.IsActive,
	).Scan(&entry.IPOrCIDR, &entry.CreatedAt, &entry.UpdatedAt, &entry.DeletedAt)
}

type ListFilter struct {
	IncludeDeleted bool
	DeletedOnly    bool
}

func (r *Repository) FindAll(ctx context.Context, p pagination.Pagination, filter ListFilter) ([]IPBlacklistEntry, error) {
	where := "WHERE deleted_at IS NULL"
	if filter.DeletedOnly {
		where = "WHERE deleted_at IS NOT NULL"
	} else if filter.IncludeDeleted {
		where = ""
	}

	query := `
		SELECT id, ip_or_cidr::text, reason, created_by, expires_at, is_active, created_at, updated_at, deleted_at
		FROM ip_blacklist
		` + where + `
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2
	`

	rows, err := r.db.Query(ctx, query, p.Limit, p.Offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []IPBlacklistEntry
	for rows.Next() {
		var entry IPBlacklistEntry
		err := rows.Scan(
			&entry.ID,
			&entry.IPOrCIDR,
			&entry.Reason,
			&entry.CreatedBy,
			&entry.ExpiresAt,
			&entry.IsActive,
			&entry.CreatedAt,
			&entry.UpdatedAt,
			&entry.DeletedAt,
		)
		if err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}

	return entries, rows.Err()
}

func (r *Repository) Count(ctx context.Context, filter ListFilter) (int64, error) {
	where := "WHERE deleted_at IS NULL"
	if filter.DeletedOnly {
		where = "WHERE deleted_at IS NOT NULL"
	} else if filter.IncludeDeleted {
		where = ""
	}

	var total int64
	err := r.db.QueryRow(ctx, `SELECT count(*) FROM ip_blacklist `+where).Scan(&total)
	return total, err
}

func (r *Repository) FindByID(ctx context.Context, id uuid.UUID) (*IPBlacklistEntry, error) {
	query := `
		SELECT id, ip_or_cidr::text, reason, created_by, expires_at, is_active, created_at, updated_at, deleted_at
		FROM ip_blacklist
		WHERE id = $1
		  AND deleted_at IS NULL
	`

	var entry IPBlacklistEntry
	err := r.db.QueryRow(ctx, query, id).Scan(
		&entry.ID,
		&entry.IPOrCIDR,
		&entry.Reason,
		&entry.CreatedBy,
		&entry.ExpiresAt,
		&entry.IsActive,
		&entry.CreatedAt,
		&entry.UpdatedAt,
		&entry.DeletedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrIPBlacklistEntryNotFound
	}
	if err != nil {
		return nil, err
	}

	return &entry, nil
}

func (r *Repository) Update(ctx context.Context, entry *IPBlacklistEntry) error {
	query := `
		UPDATE ip_blacklist
		SET ip_or_cidr = $2::cidr,
		    reason = $3,
		    created_by = $4,
		    expires_at = $5,
		    is_active = $6,
		    updated_at = now()
		WHERE id = $1
		  AND deleted_at IS NULL
		RETURNING ip_or_cidr::text, updated_at, deleted_at
	`

	err := r.db.QueryRow(
		ctx,
		query,
		entry.ID,
		entry.IPOrCIDR,
		entry.Reason,
		entry.CreatedBy,
		entry.ExpiresAt,
		entry.IsActive,
	).Scan(&entry.IPOrCIDR, &entry.UpdatedAt, &entry.DeletedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrIPBlacklistEntryNotFound
	}

	return err
}

func (r *Repository) Delete(ctx context.Context, id uuid.UUID) (*IPBlacklistEntry, error) {
	query := `
		UPDATE ip_blacklist
		SET is_active = FALSE,
		    deleted_at = now(),
		    updated_at = now()
		WHERE id = $1
		  AND deleted_at IS NULL
		RETURNING id, ip_or_cidr::text, reason, created_by, expires_at, is_active, created_at, updated_at, deleted_at
	`

	var entry IPBlacklistEntry
	err := r.db.QueryRow(ctx, query, id).Scan(
		&entry.ID,
		&entry.IPOrCIDR,
		&entry.Reason,
		&entry.CreatedBy,
		&entry.ExpiresAt,
		&entry.IsActive,
		&entry.CreatedAt,
		&entry.UpdatedAt,
		&entry.DeletedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrIPBlacklistEntryNotFound
	}

	return &entry, err
}
