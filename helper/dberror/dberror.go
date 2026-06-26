package dberror

import (
	"errors"
	"net/http"

	"github.com/jackc/pgx/v5/pgconn"
)

const (
	UniqueViolation     = "23505"
	ForeignKeyViolation = "23503"
	CheckViolation      = "23514"
	NotNullViolation    = "23502"
)

type APIError struct {
	Status  int
	Code    string
	Message string
}

func ClassifyPgError(err error) (sqlState string, constraint string, ok bool) {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return "", "", false
	}

	return pgErr.Code, pgErr.ConstraintName, true
}

var constraintErrors = map[string]APIError{
	// services
	"services_name_unique": {Status: http.StatusConflict, Code: "conflict", Message: "service name already exists"},

	// service_instances
	"service_instances_service_id_fkey":          {Status: http.StatusUnprocessableEntity, Code: "invalid_reference", Message: "service_id does not exist"},
	"service_instances_service_host_port_unique": {Status: http.StatusConflict, Code: "conflict", Message: "service instance already exists"},
	"service_instances_weight_check": {Status: http.StatusBadRequest, Code: "bad_request", Message: "weight must be greater than or equal to 1"},
	
	// rate_limit_policies
	"rate_limit_policies_name_unique": {Status: http.StatusConflict, Code: "conflict", Message: "rate limit policy name already exists"},

	// routes
	"routes_path_method_unique": {Status: http.StatusConflict, Code: "conflict", Message: "route path and method already exists"},
	"routes_service_id_fkey":    {Status: http.StatusUnprocessableEntity, Code: "invalid_reference", Message: "service_id does not exist"},
	"routes_rate_limit_id_fkey": {Status: http.StatusUnprocessableEntity, Code: "invalid_reference", Message: "rate_limit_id does not exist"},

	// cors_configs
	"cors_configs_route_id_unique": {Status: http.StatusConflict, Code: "conflict", Message: "route already has CORS config"},
	"cors_configs_route_id_fkey":   {Status: http.StatusUnprocessableEntity, Code: "invalid_reference", Message: "route_id does not exist"},

	// roles and permissions
	"roles_name_unique":                   {Status: http.StatusConflict, Code: "conflict", Message: "role name already exists"},
	"permissions_resource_action_unique":  {Status: http.StatusConflict, Code: "conflict", Message: "permission already exists"},
	"role_permissions_role_id_fkey":       {Status: http.StatusUnprocessableEntity, Code: "invalid_reference", Message: "role_id does not exist"},
	"role_permissions_permission_id_fkey": {Status: http.StatusUnprocessableEntity, Code: "invalid_reference", Message: "permission_id does not exist"},

	// users
	"users_username_unique": {Status: http.StatusConflict, Code: "conflict", Message: "username already exists"},
	"users_email_unique":    {Status: http.StatusConflict, Code: "conflict", Message: "email already exists"},
	"users_role_id_fkey":    {Status: http.StatusUnprocessableEntity, Code: "invalid_reference", Message: "role_id does not exist"},

	// api_keys
	"api_keys_key_hash_unique":    {Status: http.StatusConflict, Code: "conflict", Message: "API key already exists"},
	"api_keys_user_id_fkey":       {Status: http.StatusUnprocessableEntity, Code: "invalid_reference", Message: "user_id does not exist"},
	"api_keys_rate_limit_id_fkey": {Status: http.StatusUnprocessableEntity, Code: "invalid_reference", Message: "rate_limit_id does not exist"},

	// ip_blacklist
	"ip_blacklist_ip_or_cidr_unique": {Status: http.StatusConflict, Code: "conflict", Message: "IP or CIDR already exists in blacklist"},
	"ip_blacklist_created_by_fkey":   {Status: http.StatusUnprocessableEntity, Code: "invalid_reference", Message: "created_by user does not exist"},

	// aggregation
	"aggregation_configs_name_unique":               {Status: http.StatusConflict, Code: "conflict", Message: "aggregation config name already exists"},
	"aggregation_configs_path_method_unique":        {Status: http.StatusConflict, Code: "conflict", Message: "aggregation path and method already exists"},
	"aggregation_steps_aggregation_id_fkey":         {Status: http.StatusUnprocessableEntity, Code: "invalid_reference", Message: "aggregation_id does not exist"},
	"aggregation_steps_service_id_fkey":             {Status: http.StatusUnprocessableEntity, Code: "invalid_reference", Message: "service_id does not exist"},
	"aggregation_steps_depends_on_fkey":             {Status: http.StatusUnprocessableEntity, Code: "invalid_reference", Message: "depends_on step does not exist"},
	"aggregation_steps_aggregation_sequence_unique": {Status: http.StatusConflict, Code: "conflict", Message: "aggregation step sequence already exists"},

	// audit_logs
	"audit_logs_user_id_fkey": {Status: http.StatusUnprocessableEntity, Code: "invalid_reference", Message: "user_id does not exist"},

	// gateway_plugins and route_plugins
	"gateway_plugins_code_unique":       {Status: http.StatusConflict, Code: "conflict", Message: "plugin code already exists"},
	"route_plugins_route_id_fkey":       {Status: http.StatusUnprocessableEntity, Code: "invalid_reference", Message: "route_id does not exist"},
	"route_plugins_plugin_id_fkey":      {Status: http.StatusUnprocessableEntity, Code: "invalid_reference", Message: "plugin_id does not exist"},
	"route_plugins_route_plugin_unique": {Status: http.StatusConflict, Code: "conflict", Message: "plugin already exists on route"},
}

func MapDBError(err error) (APIError, bool) {
	sqlState, constraint, ok := ClassifyPgError(err)
	if !ok {
		return APIError{}, false
	}

	if apiErr, found := constraintErrors[constraint]; found {
		return apiErr, true
	}

	switch sqlState {
	case UniqueViolation:
		return APIError{Status: http.StatusConflict, Code: "conflict", Message: "record already exists"}, true
	case ForeignKeyViolation:
		return APIError{Status: http.StatusUnprocessableEntity, Code: "invalid_reference", Message: "referenced record does not exist"}, true
	case CheckViolation:
		return APIError{Status: http.StatusBadRequest, Code: "bad_request", Message: "invalid value"}, true
	case NotNullViolation:
		return APIError{Status: http.StatusBadRequest, Code: "bad_request", Message: "missing required field"}, true
	default:
		return APIError{}, false
	}
}
