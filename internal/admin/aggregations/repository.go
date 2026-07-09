package aggregations

import (
	"context"
	"database/sql"
	"errors"

	"gateway-api/helper/pagination"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrAggregationNotFound = errors.New("aggregation not found")
var ErrAggregationStepNotFound = errors.New("aggregation step not found")
var ErrAggregationDuplicate = errors.New("aggregation name or path/method already exists")
var ErrAggregationStepDuplicate = errors.New("aggregation step sequence already exists")
var ErrServiceUnavailable = errors.New("service_id does not exist or is inactive")
var ErrDependsOnUnavailable = errors.New("depends_on step does not exist in this aggregation")
var ErrStepDependsOnSelf = errors.New("step cannot depend on itself")
var ErrCORSPolicyUnavailable = errors.New("cors_policy_id does not exist or is inactive")

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Create(ctx context.Context, aggregation *Aggregation) error {
	if err := r.validateCORSPolicy(ctx, aggregation.CORSPolicyID); err != nil {
		return err
	}
	err := r.db.QueryRow(ctx, `
		INSERT INTO aggregation_configs (id, name, path, method, cors_policy_id, is_active)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING created_at, updated_at
	`, aggregation.ID, aggregation.Name, aggregation.Path, aggregation.Method, aggregation.CORSPolicyID, aggregation.IsActive).Scan(&aggregation.CreatedAt, &aggregation.UpdatedAt)
	return mapAggregationDBError(err)
}

func (r *Repository) FindAll(ctx context.Context, p pagination.Pagination) ([]Aggregation, error) {
	rows, err := r.db.Query(ctx, `
		SELECT agg.id, agg.name, agg.path, agg.method, agg.cors_policy_id,
		       cp.id::text, COALESCE(cp.name, ''),
		       COALESCE(cp.allowed_origins, '{}'::text[]),
		       COALESCE(cp.allowed_methods, '{}'::text[]),
		       COALESCE(cp.allowed_headers, '{}'::text[]),
		       COALESCE(cp.exposed_headers, '{}'::text[]),
		       COALESCE(cp.allow_credentials, FALSE), COALESCE(cp.max_age, 0),
		       agg.is_active, agg.created_at, agg.updated_at
		FROM aggregation_configs agg
		LEFT JOIN cors_policies cp ON cp.id = agg.cors_policy_id AND cp.is_active = TRUE AND cp.deleted_at IS NULL
		WHERE agg.deleted_at IS NULL
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2
	`, p.Limit, p.Offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []Aggregation{}
	for rows.Next() {
		var aggregation Aggregation
		if err := scanAggregation(rows, &aggregation); err != nil {
			return nil, err
		}
		items = append(items, aggregation)
	}
	return items, rows.Err()
}

type aggregationScanner interface {
	Scan(dest ...any) error
}

func scanAggregation(row aggregationScanner, aggregation *Aggregation) error {
	var corsPolicyID *uuid.UUID
	var policyID sql.NullString
	var policy CORSPolicySummary
	err := row.Scan(&aggregation.ID, &aggregation.Name, &aggregation.Path, &aggregation.Method, &corsPolicyID, &policyID, &policy.Name, &policy.AllowedOrigins, &policy.AllowedMethods, &policy.AllowedHeaders, &policy.ExposedHeaders, &policy.AllowCredentials, &policy.MaxAge, &aggregation.IsActive, &aggregation.CreatedAt, &aggregation.UpdatedAt)
	if err != nil {
		return err
	}
	aggregation.CORSPolicyID = corsPolicyID
	if policyID.Valid {
		parsed, err := uuid.Parse(policyID.String)
		if err != nil {
			return err
		}
		policy.ID = parsed
		aggregation.CORSPolicy = &policy
	}
	return nil
}
func (r *Repository) Count(ctx context.Context) (int64, error) {
	var total int64
	err := r.db.QueryRow(ctx, `SELECT count(*) FROM aggregation_configs WHERE deleted_at IS NULL`).Scan(&total)
	return total, err
}

func (r *Repository) FindByID(ctx context.Context, id uuid.UUID) (*Aggregation, error) {
	var aggregation Aggregation
	row := r.db.QueryRow(ctx, `
		SELECT agg.id, agg.name, agg.path, agg.method, agg.cors_policy_id,
		       cp.id::text, COALESCE(cp.name, ''),
		       COALESCE(cp.allowed_origins, '{}'::text[]),
		       COALESCE(cp.allowed_methods, '{}'::text[]),
		       COALESCE(cp.allowed_headers, '{}'::text[]),
		       COALESCE(cp.exposed_headers, '{}'::text[]),
		       COALESCE(cp.allow_credentials, FALSE), COALESCE(cp.max_age, 0),
		       agg.is_active, agg.created_at, agg.updated_at
		FROM aggregation_configs agg
		LEFT JOIN cors_policies cp ON cp.id = agg.cors_policy_id AND cp.is_active = TRUE AND cp.deleted_at IS NULL
		WHERE agg.id = $1 AND agg.deleted_at IS NULL
	`, id)
	err := scanAggregation(row, &aggregation)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrAggregationNotFound
	}
	if err != nil {
		return nil, err
	}
	return &aggregation, nil
}

func (r *Repository) Update(ctx context.Context, aggregation *Aggregation) error {
	if err := r.validateCORSPolicy(ctx, aggregation.CORSPolicyID); err != nil {
		return err
	}
	err := r.db.QueryRow(ctx, `
		UPDATE aggregation_configs
		SET name = $2, path = $3, method = $4, cors_policy_id = $5, is_active = $6
		WHERE id = $1 AND deleted_at IS NULL
		RETURNING updated_at
	`, aggregation.ID, aggregation.Name, aggregation.Path, aggregation.Method, aggregation.CORSPolicyID, aggregation.IsActive).Scan(&aggregation.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrAggregationNotFound
	}
	return mapAggregationDBError(err)
}

func (r *Repository) Delete(ctx context.Context, id uuid.UUID) error {
	result, err := r.db.Exec(ctx, `UPDATE aggregation_configs SET is_active = FALSE, deleted_at = now() WHERE id = $1 AND deleted_at IS NULL`, id)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return ErrAggregationNotFound
	}
	return nil
}

func (r *Repository) CreateStep(ctx context.Context, step *AggregationStep) error {
	if err := r.validateAggregation(ctx, step.AggregationID); err != nil {
		return err
	}
	if err := r.validateService(ctx, step.ServiceID); err != nil {
		return err
	}
	if err := r.validateDependsOn(ctx, step.AggregationID, step.ID, step.DependsOn); err != nil {
		return err
	}

	err := r.db.QueryRow(ctx, `
		INSERT INTO aggregation_steps (
			id, aggregation_id, service_id, sequence, depends_on,
			is_required, request_template, response_mapping, is_active
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		RETURNING created_at, updated_at
	`, step.ID, step.AggregationID, step.ServiceID, step.Sequence, step.DependsOn, step.IsRequired, step.RequestTemplate, step.ResponseMapping, step.IsActive).Scan(&step.CreatedAt, &step.UpdatedAt)
	return mapAggregationDBError(err)
}

func (r *Repository) FindSteps(ctx context.Context, aggregationID uuid.UUID) ([]AggregationStep, error) {
	if err := r.validateAggregation(ctx, aggregationID); err != nil {
		return nil, err
	}
	rows, err := r.db.Query(ctx, `
		SELECT id, aggregation_id, service_id, sequence, depends_on, is_required, request_template, response_mapping, is_active, created_at, updated_at
		FROM aggregation_steps
		WHERE aggregation_id = $1 AND deleted_at IS NULL
		ORDER BY sequence ASC
	`, aggregationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	steps := []AggregationStep{}
	for rows.Next() {
		step, err := scanStep(rows)
		if err != nil {
			return nil, err
		}
		steps = append(steps, step)
	}
	return steps, rows.Err()
}

func (r *Repository) FindStepByID(ctx context.Context, id uuid.UUID) (*AggregationStep, error) {
	row := r.db.QueryRow(ctx, `
		SELECT id, aggregation_id, service_id, sequence, depends_on, is_required, request_template, response_mapping, is_active, created_at, updated_at
		FROM aggregation_steps
		WHERE id = $1 AND deleted_at IS NULL
	`, id)
	step, err := scanStep(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrAggregationStepNotFound
	}
	if err != nil {
		return nil, err
	}
	return &step, nil
}

func (r *Repository) UpdateStep(ctx context.Context, step *AggregationStep) error {
	if err := r.validateAggregation(ctx, step.AggregationID); err != nil {
		return err
	}
	if err := r.validateService(ctx, step.ServiceID); err != nil {
		return err
	}
	if err := r.validateDependsOn(ctx, step.AggregationID, step.ID, step.DependsOn); err != nil {
		return err
	}

	err := r.db.QueryRow(ctx, `
		UPDATE aggregation_steps
		SET service_id = $2,
		    sequence = $3,
		    depends_on = $4,
		    is_required = $5,
		    request_template = $6,
		    response_mapping = $7,
		    is_active = $8
		WHERE id = $1 AND deleted_at IS NULL
		RETURNING updated_at
	`, step.ID, step.ServiceID, step.Sequence, step.DependsOn, step.IsRequired, step.RequestTemplate, step.ResponseMapping, step.IsActive).Scan(&step.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrAggregationStepNotFound
	}
	return mapAggregationDBError(err)
}

func (r *Repository) DeleteStep(ctx context.Context, id uuid.UUID) error {
	result, err := r.db.Exec(ctx, `UPDATE aggregation_steps SET is_active = FALSE, deleted_at = now() WHERE id = $1 AND deleted_at IS NULL`, id)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return ErrAggregationStepNotFound
	}
	return nil
}

func (r *Repository) validateCORSPolicy(ctx context.Context, corsPolicyID *uuid.UUID) error {
	if corsPolicyID == nil {
		return nil
	}
	var exists bool
	err := r.db.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM cors_policies WHERE id = $1 AND is_active = TRUE AND deleted_at IS NULL)`, *corsPolicyID).Scan(&exists)
	if err != nil {
		return err
	}
	if !exists {
		return ErrCORSPolicyUnavailable
	}
	return nil
}
func (r *Repository) validateAggregation(ctx context.Context, id uuid.UUID) error {
	var exists bool
	err := r.db.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM aggregation_configs WHERE id = $1 AND is_active = TRUE AND deleted_at IS NULL)`, id).Scan(&exists)
	if err != nil {
		return err
	}
	if !exists {
		return ErrAggregationNotFound
	}
	return nil
}

func (r *Repository) validateService(ctx context.Context, id uuid.UUID) error {
	var exists bool
	err := r.db.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM services WHERE id = $1 AND is_active = TRUE AND deleted_at IS NULL AND protocol = 'http')`, id).Scan(&exists)
	if err != nil {
		return err
	}
	if !exists {
		return ErrServiceUnavailable
	}
	return nil
}

func (r *Repository) validateDependsOn(ctx context.Context, aggregationID uuid.UUID, stepID uuid.UUID, dependsOn *uuid.UUID) error {
	if dependsOn == nil {
		return nil
	}
	if *dependsOn == stepID {
		return ErrStepDependsOnSelf
	}
	var exists bool
	err := r.db.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM aggregation_steps WHERE id = $1 AND aggregation_id = $2 AND deleted_at IS NULL)`, *dependsOn, aggregationID).Scan(&exists)
	if err != nil {
		return err
	}
	if !exists {
		return ErrDependsOnUnavailable
	}
	return nil
}

type stepScanner interface {
	Scan(dest ...any) error
}

func scanStep(row stepScanner) (AggregationStep, error) {
	var step AggregationStep
	err := row.Scan(&step.ID, &step.AggregationID, &step.ServiceID, &step.Sequence, &step.DependsOn, &step.IsRequired, &step.RequestTemplate, &step.ResponseMapping, &step.IsActive, &step.CreatedAt, &step.UpdatedAt)
	return step, err
}

func mapAggregationDBError(err error) error {
	if err == nil {
		return nil
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.ConstraintName {
		case "aggregation_configs_name_active_unique", "aggregation_configs_path_method_active_unique", "aggregation_configs_name_unique", "aggregation_configs_path_method_unique":
			return ErrAggregationDuplicate
		case "aggregation_steps_aggregation_sequence_active_unique", "aggregation_steps_aggregation_sequence_unique":
			return ErrAggregationStepDuplicate
		case "aggregation_steps_service_id_fkey":
			return ErrServiceUnavailable
		case "aggregation_steps_depends_on_fkey":
			return ErrDependsOnUnavailable
		}
	}
	return err
}
