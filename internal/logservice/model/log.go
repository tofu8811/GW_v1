package model

import "time"

type RequestLog struct {
	Timestamp         time.Time         `json:"@timestamp"`
	TraceID           string            `json:"trace_id"`
	RouteID           string            `json:"route_id,omitempty"`
	ServiceID         string            `json:"service_id,omitempty"`
	ServiceName       string            `json:"service_name,omitempty"`
	InstanceID        string            `json:"instance_id,omitempty"`
	Method            string            `json:"method"`
	Path              string            `json:"path"`
	NormalizedPath    string            `json:"normalized_path,omitempty"`
	QueryParams       map[string]string `json:"query_params,omitempty"`
	ClientIP          string            `json:"client_ip"`
	UserID            string            `json:"user_id,omitempty"`
	APIKeyID          string            `json:"api_key_id,omitempty"`
	StatusCode        int               `json:"status_code"`
	StatusClass       string            `json:"status_class"`
	IsError           bool              `json:"is_error"`
	ResponseTimeMS    float64           `json:"response_time_ms"`
	GatewayLatencyMS  float64           `json:"gateway_latency_ms"`
	UpstreamLatencyMS float64           `json:"upstream_latency_ms"`
	RequestSize       int               `json:"request_size"`
	ResponseSize      int               `json:"response_size"`
	ErrorMessage      string            `json:"error_message,omitempty"`
	ErrorType         string            `json:"error_type,omitempty"`
	UserAgent         string            `json:"user_agent,omitempty"`
	Env               string            `json:"env,omitempty"`
	GatewayNode       string            `json:"gateway_node,omitempty"`
}

type LogQuery struct {
	From        string
	To          string
	ServiceName string
	RouteID     string
	Method      string
	StatusClass string
	StatusCode  string
	ClientIP    string
	Query       string
	Page        int
	Limit       int
	Sort        string
	Interval    string
	TopSortBy   string
	TopLimit    int
}

type PageMeta struct {
	Page  int   `json:"page"`
	Limit int   `json:"limit"`
	Total int64 `json:"total"`
}
