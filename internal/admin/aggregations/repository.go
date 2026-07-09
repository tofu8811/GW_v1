package aggregations

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

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
var ErrRequiredScopeUnavailable = errors.New("required_scope_id does not exist or is inactive")
var ErrRateLimitUnavailable = errors.New("rate_limit_id does not exist or is inactive")
var ErrAggregationActiveWithoutSteps = errors.New("active aggregation must have at least one active step")
var ErrDependsOnSequenceInvalid = errors.New("depends_on step sequence must be smaller than current step sequence")
var ErrDependsOnCycle = errors.New("aggregation step dependency cycle detected")
var ErrResponseTargetDuplicate = errors.New("response_mapping.target already exists in this aggregation")

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

type dbQuerier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

func (r *Repository) Create(ctx context.Context, aggregation *Aggregation) error {
	if err := r.validateAggregationRefs(ctx, r.db, aggregation); err != nil {
		return err
	}
	if aggregation.IsActive {
		return ErrAggregationActiveWithoutSteps
	}
	err := r.db.QueryRow(ctx, `
		INSERT INTO aggregation_configs (id, name, path, method, auth_required, required_scope_id, rate_limit_id, cors_policy_id, is_active)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING created_at, updated_at
	`, aggregation.ID, aggregation.Name, aggregation.Path, aggregation.Method, aggregation.AuthRequired, aggregation.RequiredScopeID, aggregation.RateLimitID, aggregation.CORSPolicyID, aggregation.IsActive).Scan(&aggregation.CreatedAt, &aggregation.UpdatedAt)
	return mapAggregationDBError(err)
}

func (r *Repository) FindAll(ctx context.Context, p pagination.Pagination) ([]Aggregation, error) {
	rows, err := r.db.Query(ctx, aggregationSelectSQL()+`
		WHERE agg.deleted_at IS NULL
		ORDER BY agg.created_at DESC
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

func (r *Repository) Count(ctx context.Context) (int64, error) {
	var total int64
	err := r.db.QueryRow(ctx, `SELECT count(*) FROM aggregation_configs WHERE deleted_at IS NULL`).Scan(&total)
	return total, err
}

func (r *Repository) FindByID(ctx context.Context, id uuid.UUID) (*Aggregation, error) {
	var aggregation Aggregation
	row := r.db.QueryRow(ctx, aggregationSelectSQL()+`WHERE agg.id = $1 AND agg.deleted_at IS NULL`, id)
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
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if err := r.validateAggregationRefs(ctx, tx, aggregation); err != nil {
		return err
	}
	if aggregation.IsActive {
		if err := r.validateAggregationHasActiveStep(ctx, tx, aggregation.ID); err != nil {
			return err
		}
	}
	err = tx.QueryRow(ctx, `
		UPDATE aggregation_configs
		SET name = $2, path = $3, method = $4, auth_required = $5, required_scope_id = $6,
		    rate_limit_id = $7, cors_policy_id = $8, is_active = $9
		WHERE id = $1 AND deleted_at IS NULL
		RETURNING updated_at
	`, aggregation.ID, aggregation.Name, aggregation.Path, aggregation.Method, aggregation.AuthRequired, aggregation.RequiredScopeID, aggregation.RateLimitID, aggregation.CORSPolicyID, aggregation.IsActive).Scan(&aggregation.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrAggregationNotFound
	}
	if err := mapAggregationDBError(err); err != nil {
		return err
	}
	return tx.Commit(ctx)
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
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if err := r.validateAggregationExists(ctx, tx, step.AggregationID); err != nil {
		return err
	}
	if err := r.validateService(ctx, tx, step.ServiceID); err != nil {
		return err
	}
	if err := r.validateStepGraph(ctx, tx, step); err != nil {
		return err
	}

	err = tx.QueryRow(ctx, `
		INSERT INTO aggregation_steps (
			id, aggregation_id, service_id, sequence, depends_on,
			is_required, request_template, response_mapping, is_active
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		RETURNING created_at, updated_at
	`, step.ID, step.AggregationID, step.ServiceID, step.Sequence, step.DependsOn, step.IsRequired, step.RequestTemplate, step.ResponseMapping, step.IsActive).Scan(&step.CreatedAt, &step.UpdatedAt)
	if err := mapAggregationDBError(err); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *Repository) FindSteps(ctx context.Context, aggregationID uuid.UUID) ([]AggregationStep, error) {
	if err := r.validateAggregationExists(ctx, r.db, aggregationID); err != nil {
		return nil, err
	}
	return r.findSteps(ctx, r.db, aggregationID, false)
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
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if err := r.validateAggregationExists(ctx, tx, step.AggregationID); err != nil {
		return err
	}
	if err := r.validateService(ctx, tx, step.ServiceID); err != nil {
		return err
	}
	if err := r.validateStepGraph(ctx, tx, step); err != nil {
		return err
	}

	err = tx.QueryRow(ctx, `
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
	if err := mapAggregationDBError(err); err != nil {
		return err
	}
	return tx.Commit(ctx)
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

func (r *Repository) validateAggregationRefs(ctx context.Context, q dbQuerier, aggregation *Aggregation) error {
	if aggregation.RequiredScopeID != nil {
		var exists bool
		err := q.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM api_scopes WHERE id = $1 AND is_active = TRUE AND deleted_at IS NULL)`, *aggregation.RequiredScopeID).Scan(&exists)
		if err != nil {
			return err
		}
		if !exists {
			return ErrRequiredScopeUnavailable
		}
	}
	if aggregation.RateLimitID != nil {
		var exists bool
		err := q.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM rate_limit_policies WHERE id = $1 AND is_active = TRUE AND deleted_at IS NULL)`, *aggregation.RateLimitID).Scan(&exists)
		if err != nil {
			return err
		}
		if !exists {
			return ErrRateLimitUnavailable
		}
	}
	return r.validateCORSPolicy(ctx, q, aggregation.CORSPolicyID)
}

func (r *Repository) validateCORSPolicy(ctx context.Context, q dbQuerier, corsPolicyID *uuid.UUID) error {
	if corsPolicyID == nil {
		return nil
	}
	var exists bool
	err := q.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM cors_policies WHERE id = $1 AND is_active = TRUE AND deleted_at IS NULL)`, *corsPolicyID).Scan(&exists)
	if err != nil {
		return err
	}
	if !exists {
		return ErrCORSPolicyUnavailable
	}
	return nil
}

func (r *Repository) validateAggregationExists(ctx context.Context, q dbQuerier, id uuid.UUID) error {
	var exists bool
	err := q.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM aggregation_configs WHERE id = $1 AND deleted_at IS NULL)`, id).Scan(&exists)
	if err != nil {
		return err
	}
	if !exists {
		return ErrAggregationNotFound
	}
	return nil
}
func (r *Repository) validateAggregation(ctx context.Context, q dbQuerier, id uuid.UUID) error {
	var exists bool
	err := q.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM aggregation_configs WHERE id = $1 AND is_active = TRUE AND deleted_at IS NULL)`, id).Scan(&exists)
	if err != nil {
		return err
	}
	if !exists {
		return ErrAggregationNotFound
	}
	return nil
}

func (r *Repository) validateAggregationHasActiveStep(ctx context.Context, q dbQuerier, id uuid.UUID) error {
	var exists bool
	err := q.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM aggregation_steps WHERE aggregation_id = $1 AND is_active = TRUE AND deleted_at IS NULL)`, id).Scan(&exists)
	if err != nil {
		return err
	}
	if !exists {
		return ErrAggregationActiveWithoutSteps
	}
	return nil
}

func (r *Repository) validateService(ctx context.Context, q dbQuerier, id uuid.UUID) error {
	var exists bool
	err := q.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM services WHERE id = $1 AND is_active = TRUE AND deleted_at IS NULL AND protocol = 'http')`, id).Scan(&exists)
	if err != nil {
		return err
	}
	if !exists {
		return ErrServiceUnavailable
	}
	return nil
}

func (r *Repository) validateStepGraph(ctx context.Context, q dbQuerier, candidate *AggregationStep) error {
	steps, err := r.findSteps(ctx, q, candidate.AggregationID, true)
	if err != nil {
		return err
	}
	byID := map[uuid.UUID]AggregationStep{}
	for _, step := range steps {
		if step.ID == candidate.ID {
			step = *candidate
		}
		if step.IsActive {
			byID[step.ID] = step
		}
	}
	if candidate.IsActive {
		byID[candidate.ID] = *candidate
	}
	if !candidate.IsActive {
		delete(byID, candidate.ID)
	}

	if candidate.DependsOn != nil {
		if *candidate.DependsOn == candidate.ID {
			return ErrStepDependsOnSelf
		}
		dependency, ok := byID[*candidate.DependsOn]
		if !ok {
			return ErrDependsOnUnavailable
		}
		if dependency.Sequence >= candidate.Sequence {
			return ErrDependsOnSequenceInvalid
		}
	}
	if hasStepCycle(byID) {
		return ErrDependsOnCycle
	}
	return validateUniqueResponseTargets(byID)
}

func (r *Repository) findSteps(ctx context.Context, q dbQuerier, aggregationID uuid.UUID, forUpdate bool) ([]AggregationStep, error) {
	lockClause := ""
	if forUpdate {
		lockClause = " FOR UPDATE"
	}
	rows, err := q.Query(ctx, `
		SELECT id, aggregation_id, service_id, sequence, depends_on, is_required, request_template, response_mapping, is_active, created_at, updated_at
		FROM aggregation_steps
		WHERE aggregation_id = $1 AND deleted_at IS NULL
		ORDER BY sequence ASC`+lockClause, aggregationID)
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

func hasStepCycle(steps map[uuid.UUID]AggregationStep) bool {
	visiting := map[uuid.UUID]bool{}
	visited := map[uuid.UUID]bool{}
	var visit func(uuid.UUID) bool
	visit = func(id uuid.UUID) bool {
		if visiting[id] {
			return true
		}
		if visited[id] {
			return false
		}
		step, ok := steps[id]
		if !ok || step.DependsOn == nil {
			visited[id] = true
			return false
		}
		visiting[id] = true
		if _, ok := steps[*step.DependsOn]; ok && visit(*step.DependsOn) {
			return true
		}
		visiting[id] = false
		visited[id] = true
		return false
	}
	for id := range steps {
		if visit(id) {
			return true
		}
	}
	return false
}

func validateUniqueResponseTargets(steps map[uuid.UUID]AggregationStep) error {
	seen := map[string]uuid.UUID{}
	for _, step := range steps {
		target, err := responseTarget(step.ResponseMapping)
		if err != nil {
			return err
		}
		if previous, ok := seen[target]; ok && previous != step.ID {
			return ErrResponseTargetDuplicate
		}
		seen[target] = step.ID
	}
	return nil
}

func responseTarget(raw json.RawMessage) (string, error) {
	var payload struct {
		Target string `json:"target"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return "", err
	}
	target := strings.TrimSpace(payload.Target)
	if target == "" {
		return "", fmt.Errorf("response_mapping.target is required")
	}
	return target, nil
}

type aggregationScanner interface {
	Scan(dest ...any) error
}

func scanAggregation(row aggregationScanner, aggregation *Aggregation) error {
	var corsPolicyID *string
	var policyID sql.NullString
	var policy CORSPolicySummary
	err := row.Scan(
		&aggregation.ID,
		&aggregation.Name,
		&aggregation.Path,
		&aggregation.Method,
		&aggregation.AuthRequired,
		&aggregation.RequiredScopeID,
		&aggregation.RateLimitID,
		&corsPolicyID,
		&policyID,
		&policy.Name,
		&policy.AllowedOrigins,
		&policy.AllowedMethods,
		&policy.AllowedHeaders,
		&policy.ExposedHeaders,
		&policy.AllowCredentials,
		&policy.MaxAge,
		&aggregation.IsActive,
		&aggregation.CreatedAt,
		&aggregation.UpdatedAt,
	)
	if err != nil {
		return err
	}
	if corsPolicyID != nil {
		parsed, err := uuid.Parse(*corsPolicyID)
		if err != nil {
			return err
		}
		aggregation.CORSPolicyID = &parsed
	}
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

func aggregationSelectSQL() string {
	return `
		SELECT agg.id, agg.name, agg.path, agg.method, agg.auth_required, agg.required_scope_id,
		       agg.rate_limit_id, agg.cors_policy_id,
		       cp.id::text, COALESCE(cp.name, ''),
		       COALESCE(cp.allowed_origins, '{}'::text[]),
		       COALESCE(cp.allowed_methods, '{}'::text[]),
		       COALESCE(cp.allowed_headers, '{}'::text[]),
		       COALESCE(cp.exposed_headers, '{}'::text[]),
		       COALESCE(cp.allow_credentials, FALSE), COALESCE(cp.max_age, 0),
		       agg.is_active, agg.created_at, agg.updated_at
		FROM aggregation_configs agg
		LEFT JOIN cors_policies cp ON cp.id = agg.cors_policy_id AND cp.is_active = TRUE AND cp.deleted_at IS NULL
	`
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
		case "aggregation_configs_required_scope_id_fkey":
			return ErrRequiredScopeUnavailable
		case "aggregation_configs_rate_limit_id_fkey":
			return ErrRateLimitUnavailable
		case "aggregation_configs_cors_policy_id_fkey":
			return ErrCORSPolicyUnavailable
		}
	}
	return err
}
