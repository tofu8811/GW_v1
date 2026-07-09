package cache

import (
	"encoding/json"
	"time"
)

const (
	CurrentSchemaVersion = 6

	KeyVersion     = "cfg:version"
	KeyReload      = "cfg:reload"
	KeyRebuildLock = "cfg:rebuild:lock"
)

type Config struct {
	PollInterval    time.Duration
	ConfigTTL       time.Duration
	RebuildLockTTL  time.Duration
	RebuildLockWait time.Duration
	SchemaVersion   int
	CORSSource      string
}

func DefaultConfig() Config {
	return Config{
		PollInterval:    20 * time.Second,
		ConfigTTL:       0,
		RebuildLockTTL:  10 * time.Second,
		RebuildLockWait: 2 * time.Second,
		SchemaVersion:   CurrentSchemaVersion,
		CORSSource:      "policy",
	}
}

type RouteValue struct {
	SchemaVersion   int                   `json:"schema_version"`
	RouteID         string                `json:"route_id"`
	Path            string                `json:"path"`
	Method          string                `json:"method"`
	StripPrefix     bool                  `json:"strip_prefix"`
	RewriteTarget   *string               `json:"rewrite_target"`
	AuthRequired    bool                  `json:"auth_required"`
	RequiredScopeID *string               `json:"required_scope_id"`
	RateLimitID     *string               `json:"rate_limit_id"`
	RateLimit       *RateLimitPolicyValue `json:"rate_limit,omitempty"`
	CORS            *CORSValue            `json:"cors,omitempty"`
	Priority        int                   `json:"priority"`
	ServiceID       string                `json:"service_id"`
}

type CORSValue struct {
	AllowedOrigins   []string `json:"allowed_origins"`
	AllowedMethods   []string `json:"allowed_methods"`
	AllowedHeaders   []string `json:"allowed_headers"`
	ExposedHeaders   []string `json:"exposed_headers"`
	AllowCredentials bool     `json:"allow_credentials"`
	MaxAge           int      `json:"max_age"`
}

type RateLimitPolicyValue struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	LimitType     string `json:"limit_type"`
	MaxRequests   int    `json:"max_requests"`
	WindowSeconds int    `json:"window_seconds"`
}

type APIKeyValue struct {
	SchemaVersion int        `json:"schema_version"`
	ID            string     `json:"id"`
	KeyHash       string     `json:"key_hash"`
	KeyPrefix     string     `json:"key_prefix"`
	ClientID      string     `json:"client_id"`
	OwnerUserID   *string    `json:"owner_user_id"`
	ScopeIDs      []string   `json:"scope_ids"`
	RateLimitID   *string    `json:"rate_limit_id"`
	ExpiresAt     *time.Time `json:"expires_at"`
	IsActive      bool       `json:"is_active"`
	RevokedAt     *time.Time `json:"revoked_at"`
	ClientActive  bool       `json:"client_active"`
}

type AggregationValue struct {
	SchemaVersion   int                    `json:"schema_version"`
	ID              string                 `json:"id"`
	Name            string                 `json:"name"`
	Path            string                 `json:"path"`
	Method          string                 `json:"method"`
	AuthRequired    bool                   `json:"auth_required"`
	RequiredScopeID *string                `json:"required_scope_id"`
	RateLimitID     *string                `json:"rate_limit_id"`
	RateLimit       *RateLimitPolicyValue  `json:"rate_limit,omitempty"`
	CORSPolicyID    *string                `json:"cors_policy_id"`
	CORS            *CORSValue             `json:"cors,omitempty"`
	Steps           []AggregationStepValue `json:"steps"`
}

type AggregationStepValue struct {
	ID              string          `json:"id"`
	Sequence        int             `json:"sequence"`
	DependsOn       *string         `json:"depends_on"`
	IsRequired      bool            `json:"is_required"`
	RequestTemplate json.RawMessage `json:"request_template"`
	ResponseMapping json.RawMessage `json:"response_mapping"`
	ServiceID       string          `json:"service_id"`
}

type ServiceValue struct {
	SchemaVersion         int    `json:"schema_version"`
	ID                    string `json:"id"`
	Name                  string `json:"name"`
	Protocol              string `json:"protocol"`
	LBStrategy            string `json:"lb_strategy"`
	HealthPath            string `json:"health_path"`
	TimeoutMS             int    `json:"timeout_ms"`
	RetryCount            int    `json:"retry_count"`
	CircuitBreakerEnabled bool   `json:"circuit_breaker_enabled"`
}

type InstanceValue struct {
	ID        string `json:"id"`
	ServiceID string `json:"service_id"`
	Host      string `json:"host"`
	Port      int    `json:"port"`
	Weight    int    `json:"weight"`
}

type ServiceInstancesValue struct {
	SchemaVersion int             `json:"schema_version"`
	ServiceID     string          `json:"service_id"`
	Items         []InstanceValue `json:"items"`
}

type ActiveInstanceValue struct {
	ServiceID  string
	InstanceID string
	Host       string
	Port       int
	HealthPath string
}

type PluginMetaValue struct {
	SchemaVersion   int             `json:"schema_version"`
	Code            string          `json:"code"`
	Name            string          `json:"name"`
	Description     *string         `json:"description"`
	Phase           string          `json:"phase"`
	DefaultPriority int             `json:"default_priority"`
	ConfigSchema    json.RawMessage `json:"config_schema"`
}

type RoutePluginsValue struct {
	SchemaVersion int             `json:"schema_version"`
	Items         []PipelineValue `json:"items"`
}

type PipelineCacheValue struct {
	SchemaVersion int             `json:"schema_version"`
	Items         []PipelineValue `json:"items"`
}

type PipelineValue struct {
	Code       string          `json:"code"`
	Phase      string          `json:"phase"`
	Priority   int             `json:"priority"`
	IsRequired bool            `json:"is_required"`
	Config     json.RawMessage `json:"config"`
}

type snapshot struct {
	Services             []ServiceValue
	InstancesByServiceID map[string][]InstanceValue
	Routes               []RouteValue
	APIKeys              []APIKeyValue
	Aggregations         []AggregationValue
	Pipelines            map[string][]PipelineValue
	PluginMeta           map[string]PluginMetaValue
	Version              int64
}
