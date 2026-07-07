package model

type RealtimeQuery struct {
	Window      string
	Interval    string
	ServiceName string
	RouteID     string
	Method      string
	StatusClass string
	StatusCode  string
	ClientIP    string
	TopLimit    int
}

// tổng hợp dữ liệu của dashboard
type DashboardSnapshot struct {
	Window          string                 `json:"window"`
	Interval        string                 `json:"interval"`
	GeneratedAt     string                 `json:"generated_at"`
	Filters         map[string]string      `json:"filters"`
	Summary         MetricSummary          `json:"summary"`
	RPS             []TimeBucket           `json:"rps"`
	ErrorRateSeries []ErrorRateTimeBucket  `json:"error_rate_series"`
	StatusCodes     StatusCodeDistribution `json:"status_codes"`
	TopRoutes       []TopRouteMetric       `json:"top_routes"`
}

type MetricSummary struct {
	TotalRequests int64   `json:"total_requests"`
	ErrorCount    int64   `json:"error_count"`
	ErrorRate     float64 `json:"error_rate"`
	AvgLatencyMS  float64 `json:"avg_latency_ms"`
	P50LatencyMS  float64 `json:"p50_latency_ms"`
	P95LatencyMS  float64 `json:"p95_latency_ms"`
	P99LatencyMS  float64 `json:"p99_latency_ms"`
}

// số requests theo thời gian
type TimeBucket struct {
	Timestamp string `json:"timestamp"`
	Requests  int64  `json:"requests"`
}

type ErrorRateTimeBucket struct {
	Timestamp string  `json:"timestamp"`
	Total     int64   `json:"total"`
	Errors    int64   `json:"errors"`
	ErrorRate float64 `json:"error_rate"`
}

type StatusCodeDistribution struct {
	ByClass []BucketMetric `json:"by_class"`
	ByCode  []BucketMetric `json:"by_code"`
}

type BucketMetric struct {
	Key      any   `json:"key"`
	DocCount int64 `json:"doc_count"`
}

type TopRouteMetric struct {
	Key          string         `json:"key"`
	DocCount     int64          `json:"doc_count"`
	ErrorCount   int64          `json:"error_count"`
	AvgLatencyMS float64        `json:"avg_latency_ms"`
	Services     []BucketMetric `json:"services"`
}
