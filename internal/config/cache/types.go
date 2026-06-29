package cache

import (
	"encoding/json"
	"time"
)

const (
	CurrentSchemaVersion = 1

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
}

func DefaultConfig() Config {
	return Config{
		PollInterval:    20 * time.Second,
		ConfigTTL:       0,
		RebuildLockTTL:  10 * time.Second,
		RebuildLockWait: 2 * time.Second,
		SchemaVersion:   CurrentSchemaVersion,
	}
}

type RouteValue struct {
	SchemaVersion int             `json:"schema_version"`
	RouteID       string          `json:"route_id"`
	Path          string          `json:"path"`
	Method        string          `json:"method"`
	StripPrefix   bool            `json:"strip_prefix"`
	RewriteTarget *string         `json:"rewrite_target"`
	AuthRequired  bool            `json:"auth_required"`
	RateLimitID   *string         `json:"rate_limit_id"`
	Priority      int             `json:"priority"`
	Service       ServiceValue    `json:"service"`
	Instances     []InstanceValue `json:"instances"`
}

type ServiceValue struct {
	ID                    string `json:"id"`
	Name                  string `json:"name"`
	Protocol              string `json:"protocol"`
	LBStrategy            string `json:"lb_strategy"`
	TimeoutMS             int    `json:"timeout_ms"`
	RetryCount            int    `json:"retry_count"`
	CircuitBreakerEnabled bool   `json:"circuit_breaker_enabled"`
}

type InstanceValue struct {
	ID     string `json:"id"`
	Host   string `json:"host"`
	Port   int    `json:"port"`
	Weight int    `json:"weight"`
}

type ActiveInstanceValue struct {
	ServiceID  string
	InstanceID string
	Host       string
	Port       int
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
	Routes     []RouteValue
	Pipelines  map[string][]PipelineValue
	PluginMeta map[string]PluginMetaValue
	Version    int64
}
