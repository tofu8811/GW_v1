package elasticsearch

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"gateway-api/internal/logservice/model"
)

type Client struct {
	baseURL     string
	indexPrefix string
	httpClient  *http.Client
}

func NewClient(baseURL string, indexPrefix string) *Client {
	if strings.TrimSpace(baseURL) == "" {
		baseURL = "http://localhost:9200"
	}
	if strings.TrimSpace(indexPrefix) == "" {
		indexPrefix = "gateway-logs"
	}
	return &Client{
		baseURL:     strings.TrimRight(baseURL, "/"),
		indexPrefix: indexPrefix,
		httpClient:  &http.Client{Timeout: 10 * time.Second},
	}
}

func (c *Client) Ping(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL, nil)
	if err != nil {
		return err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= http.StatusBadRequest {
		return fmt.Errorf("elasticsearch ping failed: %s", resp.Status)
	}
	return nil
}

func (c *Client) BulkIndex(ctx context.Context, logs []model.RequestLog) error {
	if len(logs) == 0 {
		return nil
	}

	var body bytes.Buffer
	encoder := json.NewEncoder(&body)
	for _, entry := range logs {
		index := c.indexFor(entry.Timestamp)
		meta := map[string]any{"index": map[string]any{"_index": index}}
		if strings.TrimSpace(entry.TraceID) != "" {
			meta["index"].(map[string]any)["_id"] = entry.TraceID
		}
		if err := encoder.Encode(meta); err != nil {
			return err
		}
		if err := encoder.Encode(entry); err != nil {
			return err
		}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/_bulk", &body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-ndjson")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	payload, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= http.StatusBadRequest {
		return fmt.Errorf("elasticsearch bulk index failed: %s: %s", resp.Status, strings.TrimSpace(string(payload)))
	}

	var bulkResp struct {
		Errors bool `json:"errors"`
	}
	if err := json.Unmarshal(payload, &bulkResp); err == nil && bulkResp.Errors {
		return fmt.Errorf("elasticsearch bulk index contained item errors")
	}
	return nil
}

func (c *Client) SearchLogs(ctx context.Context, query model.LogQuery) ([]model.RequestLog, int64, error) {
	if query.Page <= 0 {
		query.Page = 1
	}
	if query.Limit <= 0 || query.Limit > 200 {
		query.Limit = 20
	}

	body := map[string]any{
		"from":  (query.Page - 1) * query.Limit,
		"size":  query.Limit,
		"query": buildBoolQuery(query),
		"sort":  buildSort(query.Sort),
	}

	payload, err := c.doJSON(ctx, http.MethodGet, c.searchURL(), body)
	if err != nil {
		return nil, 0, err
	}

	var resp struct {
		Hits struct {
			Total struct {
				Value int64 `json:"value"`
			} `json:"total"`
			Hits []struct {
				Source model.RequestLog `json:"_source"`
			} `json:"hits"`
		} `json:"hits"`
	}
	if err := json.Unmarshal(payload, &resp); err != nil {
		return nil, 0, err
	}

	items := make([]model.RequestLog, 0, len(resp.Hits.Hits))
	for _, hit := range resp.Hits.Hits {
		items = append(items, hit.Source)
	}
	return items, resp.Hits.Total.Value, nil
}

func (c *Client) Summary(ctx context.Context, query model.LogQuery) (map[string]any, error) {
	body := map[string]any{
		"size":  0,
		"query": buildBoolQuery(query),
		"aggs": map[string]any{
			"avg_latency_ms": map[string]any{"avg": map[string]any{"field": "response_time_ms"}},
			"p95_latency_ms": map[string]any{"percentiles": map[string]any{"field": "response_time_ms", "percents": []int{95}}},
			"errors":         map[string]any{"filter": map[string]any{"term": map[string]any{"is_error": true}}},
		},
	}
	payload, err := c.doJSON(ctx, http.MethodGet, c.searchURL(), body)
	if err != nil {
		return nil, err
	}
	var resp genericAggResponse
	if err := json.Unmarshal(payload, &resp); err != nil {
		return nil, err
	}
	total := resp.Hits.Total.Value
	errorsCount := bucketDocCount(resp.Aggregations["errors"])
	avgLatency := valueOf(resp.Aggregations["avg_latency_ms"])
	p95 := percentileOf(resp.Aggregations["p95_latency_ms"], "95.0")
	errorRate := 0.0
	if total > 0 {
		errorRate = float64(errorsCount) / float64(total)
	}
	return map[string]any{
		"total_requests":              total,
		"error_count":                 errorsCount,
		"error_rate":                  errorRate,
		"avg_latency_ms":              avgLatency,
		"p95_latency_ms":              p95,
		"requests_per_second_average": averageRPS(total, query),
	}, nil
}

func (c *Client) RPS(ctx context.Context, query model.LogQuery) ([]map[string]any, error) {
	return c.dateHistogram(ctx, query, "requests", nil)
}

func (c *Client) ErrorRate(ctx context.Context, query model.LogQuery) (map[string]any, error) {
	aggs := map[string]any{
		"series": map[string]any{
			"date_histogram": dateHistogramAgg(query),
			"aggs": map[string]any{
				"errors": map[string]any{"filter": map[string]any{"term": map[string]any{"is_error": true}}},
			},
		},
		"errors": map[string]any{"filter": map[string]any{"term": map[string]any{"is_error": true}}},
	}
	body := map[string]any{"size": 0, "query": buildBoolQuery(query), "aggs": aggs}
	payload, err := c.doJSON(ctx, http.MethodGet, c.searchURL(), body)
	if err != nil {
		return nil, err
	}
	var resp genericAggResponse
	if err := json.Unmarshal(payload, &resp); err != nil {
		return nil, err
	}
	total := resp.Hits.Total.Value
	errorsCount := bucketDocCount(resp.Aggregations["errors"])
	series := buckets(resp.Aggregations["series"])
	items := make([]map[string]any, 0, len(series))
	for _, bucket := range series {
		count := int64From(bucket["doc_count"])
		errCount := bucketDocCount(bucket["errors"])
		rate := 0.0
		if count > 0 {
			rate = float64(errCount) / float64(count)
		}
		items = append(items, map[string]any{
			"timestamp":  bucket["key_as_string"],
			"total":      count,
			"errors":     errCount,
			"error_rate": rate,
		})
	}
	rate := 0.0
	if total > 0 {
		rate = float64(errorsCount) / float64(total)
	}
	return map[string]any{"total": total, "errors": errorsCount, "error_rate": rate, "series": items}, nil
}

func (c *Client) Latency(ctx context.Context, query model.LogQuery) (map[string]any, error) {
	body := map[string]any{
		"size":  0,
		"query": buildBoolQuery(query),
		"aggs": map[string]any{
			"avg": map[string]any{"avg": map[string]any{"field": "response_time_ms"}},
			"min": map[string]any{"min": map[string]any{"field": "response_time_ms"}},
			"max": map[string]any{"max": map[string]any{"field": "response_time_ms"}},
			"pct": map[string]any{"percentiles": map[string]any{"field": "response_time_ms", "percents": []int{50, 95, 99}}},
		},
	}
	payload, err := c.doJSON(ctx, http.MethodGet, c.searchURL(), body)
	if err != nil {
		return nil, err
	}
	var resp genericAggResponse
	if err := json.Unmarshal(payload, &resp); err != nil {
		return nil, err
	}
	return map[string]any{
		"avg": valueOf(resp.Aggregations["avg"]),
		"min": valueOf(resp.Aggregations["min"]),
		"max": valueOf(resp.Aggregations["max"]),
		"p50": percentileOf(resp.Aggregations["pct"], "50.0"),
		"p95": percentileOf(resp.Aggregations["pct"], "95.0"),
		"p99": percentileOf(resp.Aggregations["pct"], "99.0"),
	}, nil
}

func (c *Client) StatusCodes(ctx context.Context, query model.LogQuery) (map[string]any, error) {
	body := map[string]any{
		"size":  0,
		"query": buildBoolQuery(query),
		"aggs": map[string]any{
			"status_class": map[string]any{"terms": map[string]any{"field": "status_class", "size": 10}},
			"status_code":  map[string]any{"terms": map[string]any{"field": "status_code", "size": 100}},
		},
	}
	payload, err := c.doJSON(ctx, http.MethodGet, c.searchURL(), body)
	if err != nil {
		return nil, err
	}
	var resp genericAggResponse
	if err := json.Unmarshal(payload, &resp); err != nil {
		return nil, err
	}
	return map[string]any{
		"by_class": buckets(resp.Aggregations["status_class"]),
		"by_code":  buckets(resp.Aggregations["status_code"]),
	}, nil
}

func (c *Client) TopRoutes(ctx context.Context, query model.LogQuery) ([]map[string]any, error) {
	if query.TopLimit <= 0 || query.TopLimit > 100 {
		query.TopLimit = 10
	}
	metricOrder := map[string]any{"_count": "desc"}
	aggs := map[string]any{
		"top_routes": map[string]any{
			"terms": map[string]any{
				"field": "normalized_path",
				"size":  query.TopLimit,
				"order": metricOrder,
			},
			"aggs": map[string]any{
				"errors":      map[string]any{"filter": map[string]any{"term": map[string]any{"is_error": true}}},
				"avg_latency": map[string]any{"avg": map[string]any{"field": "response_time_ms"}},
				"services":    map[string]any{"terms": map[string]any{"field": "service_name", "size": 3}},
			},
		},
	}
	if query.TopSortBy == "errors" {
		aggs["top_routes"].(map[string]any)["terms"].(map[string]any)["order"] = map[string]any{"errors._count": "desc"}
	}
	if query.TopSortBy == "latency" {
		aggs["top_routes"].(map[string]any)["terms"].(map[string]any)["order"] = map[string]any{"avg_latency": "desc"}
	}

	body := map[string]any{"size": 0, "query": buildBoolQuery(query), "aggs": aggs}
	payload, err := c.doJSON(ctx, http.MethodGet, c.searchURL(), body)
	if err != nil {
		return nil, err
	}
	var resp genericAggResponse
	if err := json.Unmarshal(payload, &resp); err != nil {
		return nil, err
	}
	return buckets(resp.Aggregations["top_routes"]), nil
}

func (c *Client) dateHistogram(ctx context.Context, query model.LogQuery, countName string, extraAggs map[string]any) ([]map[string]any, error) {
	aggs := map[string]any{
		"series": map[string]any{
			"date_histogram": dateHistogramAgg(query),
		},
	}
	if len(extraAggs) > 0 {
		aggs["series"].(map[string]any)["aggs"] = extraAggs
	}
	body := map[string]any{"size": 0, "query": buildBoolQuery(query), "aggs": aggs}
	payload, err := c.doJSON(ctx, http.MethodGet, c.searchURL(), body)
	if err != nil {
		return nil, err
	}
	var resp genericAggResponse
	if err := json.Unmarshal(payload, &resp); err != nil {
		return nil, err
	}
	raw := buckets(resp.Aggregations["series"])
	items := make([]map[string]any, 0, len(raw))
	for _, bucket := range raw {
		items = append(items, map[string]any{
			"timestamp": bucket["key_as_string"],
			countName:   int64From(bucket["doc_count"]),
		})
	}
	return items, nil
}

func (c *Client) doJSON(ctx context.Context, method string, target string, body map[string]any) ([]byte, error) {
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, method, target, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	respPayload, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= http.StatusBadRequest {
		return nil, fmt.Errorf("elasticsearch request failed: %s: %s", resp.Status, strings.TrimSpace(string(respPayload)))
	}
	return respPayload, nil
}

func (c *Client) searchURL() string {
	return c.baseURL + "/" + c.indexPrefix + "-*/_search"
}

func (c *Client) indexFor(timestamp time.Time) string {
	if timestamp.IsZero() {
		timestamp = time.Now().UTC()
	}
	return c.indexPrefix + "-" + timestamp.UTC().Format("2006.01.02")
}

func buildBoolQuery(query model.LogQuery) map[string]any {
	if query.NoResults {
		return map[string]any{"match_none": map[string]any{}}
	}

	filters := make([]map[string]any, 0)
	if query.From != "" || query.To != "" {
		rng := map[string]any{}
		if query.From != "" {
			rng["gte"] = query.From
		}
		if query.To != "" {
			rng["lte"] = query.To
		}
		filters = append(filters, map[string]any{"range": map[string]any{"@timestamp": rng}})
	}
	addTerm := func(field string, value string) {
		if strings.TrimSpace(value) != "" {
			filters = append(filters, map[string]any{"term": map[string]any{field: value}})
		}
	}
	addTerm("service_name", query.ServiceName)
	addTerm("route_id", query.RouteID)
	if len(query.RouteIDs) > 0 {
		filters = append(filters, map[string]any{"terms": map[string]any{"route_id": query.RouteIDs}})
	}
	addTerm("user_id", query.UserID)
	addTerm("method", strings.ToUpper(query.Method))
	addTerm("status_class", query.StatusClass)
	addTerm("status_code", query.StatusCode)
	addTerm("client_ip", query.ClientIP)

	boolQuery := map[string]any{"filter": filters}
	if query.ExcludeControlPlane {
		boolQuery["must_not"] = controlPlaneExclusions()
	}
	if strings.TrimSpace(query.Query) != "" {
		boolQuery["must"] = []map[string]any{
			{"multi_match": map[string]any{
				"query":  query.Query,
				"fields": []string{"path", "normalized_path", "service_name", "trace_id", "error_message", "user_agent"},
			}},
		}
	}
	return map[string]any{"bool": boolQuery}
}

func controlPlaneExclusions() []map[string]any {
	return []map[string]any{
		{"term": map[string]any{"method": "OPTIONS"}},
		{"terms": map[string]any{"path": []string{"/health", "/ready"}}},
		{"prefix": map[string]any{"path": "/admin/"}},
		{"prefix": map[string]any{"path": "/auth/"}},
	}
}

func buildSort(value string) []map[string]any {
	direction := "desc"
	field := "@timestamp"
	if strings.TrimSpace(value) != "" {
		parts := strings.Split(value, ":")
		field = parts[0]
		if len(parts) > 1 && strings.EqualFold(parts[1], "asc") {
			direction = "asc"
		}
	}
	return []map[string]any{{field: map[string]any{"order": direction}}}
}

func dateHistogramAgg(query model.LogQuery) map[string]any {
	interval := query.Interval
	if strings.TrimSpace(interval) == "" {
		interval = "1s"
	}
	return map[string]any{
		"field":          "@timestamp",
		"fixed_interval": interval,
		"min_doc_count":  0,
	}
}

func averageRPS(total int64, query model.LogQuery) float64 {
	from, fromErr := time.Parse(time.RFC3339, query.From)
	to, toErr := time.Parse(time.RFC3339, query.To)
	if fromErr != nil || toErr != nil || !to.After(from) {
		return 0
	}
	seconds := to.Sub(from).Seconds()
	if seconds <= 0 {
		return 0
	}
	return float64(total) / seconds
}

type genericAggResponse struct {
	Hits struct {
		Total struct {
			Value int64 `json:"value"`
		} `json:"total"`
	} `json:"hits"`
	Aggregations map[string]any `json:"aggregations"`
}

func valueOf(raw any) float64 {
	m, _ := raw.(map[string]any)
	value, _ := m["value"].(float64)
	return value
}

func percentileOf(raw any, key string) float64 {
	m, _ := raw.(map[string]any)
	values, _ := m["values"].(map[string]any)
	value, _ := values[key].(float64)
	return value
}

func bucketDocCount(raw any) int64 {
	m, _ := raw.(map[string]any)
	return int64From(m["doc_count"])
}

func buckets(raw any) []map[string]any {
	m, _ := raw.(map[string]any)
	rawBuckets, _ := m["buckets"].([]any)
	out := make([]map[string]any, 0, len(rawBuckets))
	for _, item := range rawBuckets {
		bucket, ok := item.(map[string]any)
		if ok {
			out = append(out, bucket)
		}
	}
	return out
}

func int64From(value any) int64 {
	switch v := value.(type) {
	case float64:
		return int64(v)
	case int:
		return int64(v)
	case int64:
		return v
	case json.Number:
		n, _ := v.Int64()
		return n
	case string:
		n, _ := strconv.ParseInt(v, 10, 64)
		return n
	default:
		return 0
	}
}
