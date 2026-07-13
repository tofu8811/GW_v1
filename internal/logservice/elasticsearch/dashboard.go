package elasticsearch

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"gateway-api/internal/logservice/model"
)

// query elasticsearch
func (c *Client) DashboardSnapshot(ctx context.Context, query model.RealtimeQuery) (model.DashboardSnapshot, error) {
	body := map[string]any{
		"size":             0,
		"track_total_hits": true,
		"query":            buildRealtimeBoolQuery(query),
		"aggs":             buildDashboardAggregations(query),
	}
	payload, err := c.doJSON(ctx, http.MethodGet, c.searchURL(), body)
	if err != nil {
		return model.DashboardSnapshot{}, err
	}

	var resp genericAggResponse
	if err := json.Unmarshal(payload, &resp); err != nil {
		return model.DashboardSnapshot{}, err
	}

	total := resp.Hits.Total.Value
	errorCount := bucketDocCount(resp.Aggregations["errors"])
	errorRate := 0.0
	if total > 0 {
		errorRate = float64(errorCount) / float64(total)
	}

	return model.DashboardSnapshot{
		Window:      query.Window,
		Interval:    query.Interval,
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		Filters:     realtimeFilters(query),
		Summary: model.MetricSummary{
			TotalRequests: total,
			ErrorCount:    errorCount,
			ErrorRate:     errorRate,
			AvgLatencyMS:  valueOf(resp.Aggregations["latency_avg"]),
			P50LatencyMS:  percentileOf(resp.Aggregations["latency_percentiles"], "50.0"),
			P95LatencyMS:  percentileOf(resp.Aggregations["latency_percentiles"], "95.0"),
			P99LatencyMS:  percentileOf(resp.Aggregations["latency_percentiles"], "99.0"),
		},
		RPS:             rpsBuckets(resp.Aggregations["rps"]),
		ErrorRateSeries: errorRateBuckets(resp.Aggregations["rps"]),
		StatusCodes: model.StatusCodeDistribution{
			ByClass: bucketMetrics(resp.Aggregations["status_class"]),
			ByCode:  bucketMetrics(resp.Aggregations["status_code"]),
		},
		TopRoutes: topRouteMetrics(resp.Aggregations["top_routes"]),
	}, nil
}

func buildRealtimeBoolQuery(query model.RealtimeQuery) map[string]any {
	if query.NoResults {
		return map[string]any{"match_none": map[string]any{}}
	}

	filters := []map[string]any{
		{"range": map[string]any{"@timestamp": map[string]any{
			"gte": "now-" + query.Window,
			"lte": "now",
		}}},
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

	return map[string]any{"bool": map[string]any{
		"filter":   filters,
		"must_not": controlPlaneExclusions(),
	}}
}

func buildDashboardAggregations(query model.RealtimeQuery) map[string]any {
	topLimit := query.TopLimit
	if topLimit <= 0 {
		topLimit = 10
	}
	return map[string]any{
		"rps": map[string]any{
			"date_histogram": map[string]any{
				"field":          "@timestamp",
				"fixed_interval": query.Interval,
				"min_doc_count":  0,
			},
			"aggs": map[string]any{
				"errors": map[string]any{"filter": map[string]any{"term": map[string]any{"is_error": true}}},
			},
		},
		"errors":              map[string]any{"filter": map[string]any{"term": map[string]any{"is_error": true}}},
		"latency_avg":         map[string]any{"avg": map[string]any{"field": "response_time_ms"}},
		"latency_percentiles": map[string]any{"percentiles": map[string]any{"field": "response_time_ms", "percents": []int{50, 95, 99}}},
		"status_class":        map[string]any{"terms": map[string]any{"field": "status_class", "size": 10}},
		"status_code":         map[string]any{"terms": map[string]any{"field": "status_code", "size": 100}},
		"top_routes": map[string]any{
			"terms": map[string]any{
				"field": "normalized_path",
				"size":  topLimit,
				"order": map[string]any{"_count": "desc"},
			},
			"aggs": map[string]any{
				"errors":      map[string]any{"filter": map[string]any{"term": map[string]any{"is_error": true}}},
				"avg_latency": map[string]any{"avg": map[string]any{"field": "response_time_ms"}},
				"services":    map[string]any{"terms": map[string]any{"field": "service_name", "size": 3}},
			},
		},
	}
}

func realtimeFilters(query model.RealtimeQuery) map[string]string {
	out := map[string]string{}
	add := func(key string, value string) {
		if strings.TrimSpace(value) != "" {
			out[key] = value
		}
	}
	add("service_name", query.ServiceName)
	add("route_id", query.RouteID)
	add("user_id", query.UserID)
	add("method", query.Method)
	add("status_class", query.StatusClass)
	add("status_code", query.StatusCode)
	add("client_ip", query.ClientIP)
	return out
}

func rpsBuckets(raw any) []model.TimeBucket {
	rawBuckets := buckets(raw)
	out := make([]model.TimeBucket, 0, len(rawBuckets))
	for _, bucket := range rawBuckets {
		timestamp, _ := bucket["key_as_string"].(string)
		out = append(out, model.TimeBucket{
			Timestamp: timestamp,
			Requests:  int64From(bucket["doc_count"]),
		})
	}
	return out
}

func errorRateBuckets(raw any) []model.ErrorRateTimeBucket {
	rawBuckets := buckets(raw)
	out := make([]model.ErrorRateTimeBucket, 0, len(rawBuckets))
	for _, bucket := range rawBuckets {
		total := int64From(bucket["doc_count"])
		errors := bucketDocCount(bucket["errors"])
		rate := 0.0
		if total > 0 {
			rate = float64(errors) / float64(total)
		}
		timestamp, _ := bucket["key_as_string"].(string)
		out = append(out, model.ErrorRateTimeBucket{
			Timestamp: timestamp,
			Total:     total,
			Errors:    errors,
			ErrorRate: rate,
		})
	}
	return out
}

func bucketMetrics(raw any) []model.BucketMetric {
	rawBuckets := buckets(raw)
	out := make([]model.BucketMetric, 0, len(rawBuckets))
	for _, bucket := range rawBuckets {
		out = append(out, model.BucketMetric{
			Key:      bucket["key"],
			DocCount: int64From(bucket["doc_count"]),
		})
	}
	return out
}

func topRouteMetrics(raw any) []model.TopRouteMetric {
	rawBuckets := buckets(raw)
	out := make([]model.TopRouteMetric, 0, len(rawBuckets))
	for _, bucket := range rawBuckets {
		key, _ := bucket["key"].(string)
		out = append(out, model.TopRouteMetric{
			Key:          key,
			DocCount:     int64From(bucket["doc_count"]),
			ErrorCount:   bucketDocCount(bucket["errors"]),
			AvgLatencyMS: valueOf(bucket["avg_latency"]),
			Services:     bucketMetrics(bucket["services"]),
		})
	}
	return out
}
