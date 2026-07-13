package api

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"gateway-api/internal/logservice/model"
)

const realtimeSubscriberBuffer = 256

type RealtimeHub struct {
	mu          sync.RWMutex
	subscribers map[chan model.RequestLog]model.RealtimeQuery
}

func NewRealtimeHub() *RealtimeHub {
	return &RealtimeHub{subscribers: map[chan model.RequestLog]model.RealtimeQuery{}}
}

func (h *RealtimeHub) Subscribe(query model.RealtimeQuery) (<-chan model.RequestLog, func()) {
	if h == nil {
		return nil, func() {}
	}
	ch := make(chan model.RequestLog, realtimeSubscriberBuffer)
	h.mu.Lock()
	h.subscribers[ch] = query
	h.mu.Unlock()

	return ch, func() {
		h.mu.Lock()
		if _, ok := h.subscribers[ch]; ok {
			delete(h.subscribers, ch)
			close(ch)
		}
		h.mu.Unlock()
	}
}

func (h *RealtimeHub) PublishLog(entry model.RequestLog) {
	if h == nil {
		return
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	for ch, query := range h.subscribers {
		if !matchesRealtimeQuery(entry, query) {
			continue
		}
		select {
		case ch <- entry:
		default:
		}
	}
}

func matchesRealtimeQuery(entry model.RequestLog, query model.RealtimeQuery) bool {
	if query.NoResults {
		return false
	}
	if !matchString(query.ServiceName, entry.ServiceName) {
		return false
	}
	if !matchString(query.RouteID, entry.RouteID) {
		return false
	}
	if len(query.RouteIDs) > 0 && !containsRouteID(query.RouteIDs, entry.RouteID) {
		return false
	}
	if !matchString(query.UserID, entry.UserID) {
		return false
	}
	if !matchString(strings.ToUpper(query.Method), strings.ToUpper(entry.Method)) {
		return false
	}
	if !matchString(query.StatusClass, entry.StatusClass) {
		return false
	}
	if strings.TrimSpace(query.StatusCode) != "" && query.StatusCode != strconv.Itoa(entry.StatusCode) {
		return false
	}
	if !matchString(query.ClientIP, entry.ClientIP) {
		return false
	}
	return true
}

func matchString(want string, got string) bool {
	want = strings.TrimSpace(want)
	if want == "" {
		return true
	}
	return strings.EqualFold(want, strings.TrimSpace(got))
}

func applyRealtimeLog(snapshot model.DashboardSnapshot, entry model.RequestLog, query model.RealtimeQuery) model.DashboardSnapshot {
	if snapshot.Filters == nil {
		snapshot.Filters = map[string]string{}
	}
	snapshot.GeneratedAt = time.Now().UTC().Format(time.RFC3339)

	previousTotal := snapshot.Summary.TotalRequests
	snapshot.Summary.TotalRequests++
	if entry.IsError {
		snapshot.Summary.ErrorCount++
	}
	if snapshot.Summary.TotalRequests > 0 {
		snapshot.Summary.ErrorRate = float64(snapshot.Summary.ErrorCount) / float64(snapshot.Summary.TotalRequests)
	}
	latency := entry.ResponseTimeMS
	if latency > 0 {
		snapshot.Summary.AvgLatencyMS = movingAverage(snapshot.Summary.AvgLatencyMS, previousTotal, latency)
		if snapshot.Summary.P50LatencyMS == 0 || latency > snapshot.Summary.P50LatencyMS {
			snapshot.Summary.P50LatencyMS = latency
		}
		if snapshot.Summary.P95LatencyMS == 0 || latency > snapshot.Summary.P95LatencyMS {
			snapshot.Summary.P95LatencyMS = latency
		}
		if snapshot.Summary.P99LatencyMS == 0 || latency > snapshot.Summary.P99LatencyMS {
			snapshot.Summary.P99LatencyMS = latency
		}
	}

	snapshot.RPS = incrementRPS(snapshot.RPS, entry, query)
	snapshot.ErrorRateSeries = incrementErrorRateSeries(snapshot.ErrorRateSeries, entry, query)
	snapshot.StatusCodes.ByClass = incrementBucket(snapshot.StatusCodes.ByClass, entry.StatusClass)
	snapshot.StatusCodes.ByCode = incrementBucket(snapshot.StatusCodes.ByCode, entry.StatusCode)
	snapshot.TopRoutes = incrementTopRoute(snapshot.TopRoutes, entry, query.TopLimit)
	return snapshot
}

func movingAverage(current float64, previousCount int64, next float64) float64 {
	if previousCount <= 0 {
		return next
	}
	return ((current * float64(previousCount)) + next) / float64(previousCount+1)
}

func incrementRPS(items []model.TimeBucket, entry model.RequestLog, query model.RealtimeQuery) []model.TimeBucket {
	ts := bucketTimestamp(entry.Timestamp, query.Interval)
	for i := range items {
		if items[i].Timestamp == ts {
			items[i].Requests++
			return pruneTimeBuckets(items, query.Window)
		}
	}
	items = append(items, model.TimeBucket{Timestamp: ts, Requests: 1})
	return pruneTimeBuckets(items, query.Window)
}

func incrementErrorRateSeries(items []model.ErrorRateTimeBucket, entry model.RequestLog, query model.RealtimeQuery) []model.ErrorRateTimeBucket {
	ts := bucketTimestamp(entry.Timestamp, query.Interval)
	for i := range items {
		if items[i].Timestamp == ts {
			items[i].Total++
			if entry.IsError {
				items[i].Errors++
			}
			items[i].ErrorRate = float64(items[i].Errors) / float64(items[i].Total)
			return pruneErrorBuckets(items, query.Window)
		}
	}
	next := model.ErrorRateTimeBucket{Timestamp: ts, Total: 1}
	if entry.IsError {
		next.Errors = 1
		next.ErrorRate = 1
	}
	items = append(items, next)
	return pruneErrorBuckets(items, query.Window)
}

func bucketTimestamp(ts time.Time, interval string) string {
	if ts.IsZero() {
		ts = time.Now().UTC()
	}
	duration, err := time.ParseDuration(interval)
	if err != nil || duration <= 0 {
		duration = time.Second
	}
	return ts.UTC().Truncate(duration).Format(time.RFC3339)
}

func pruneTimeBuckets(items []model.TimeBucket, window string) []model.TimeBucket {
	duration, err := time.ParseDuration(window)
	if err != nil || duration <= 0 {
		return items
	}
	cutoff := time.Now().UTC().Add(-duration)
	out := items[:0]
	for _, item := range items {
		ts, err := time.Parse(time.RFC3339, item.Timestamp)
		if err != nil || !ts.Before(cutoff) {
			out = append(out, item)
		}
	}
	return out
}

func pruneErrorBuckets(items []model.ErrorRateTimeBucket, window string) []model.ErrorRateTimeBucket {
	duration, err := time.ParseDuration(window)
	if err != nil || duration <= 0 {
		return items
	}
	cutoff := time.Now().UTC().Add(-duration)
	out := items[:0]
	for _, item := range items {
		ts, err := time.Parse(time.RFC3339, item.Timestamp)
		if err != nil || !ts.Before(cutoff) {
			out = append(out, item)
		}
	}
	return out
}

func incrementBucket(items []model.BucketMetric, key any) []model.BucketMetric {
	for i := range items {
		if bucketKeyEqual(items[i].Key, key) {
			items[i].DocCount++
			return items
		}
	}
	return append(items, model.BucketMetric{Key: key, DocCount: 1})
}

func bucketKeyEqual(left any, right any) bool {
	return strings.EqualFold(strings.TrimSpace(toString(left)), strings.TrimSpace(toString(right)))
}

func toString(value any) string {
	switch v := value.(type) {
	case string:
		return v
	case int:
		return strconv.Itoa(v)
	case int64:
		return strconv.FormatInt(v, 10)
	case float64:
		return strconv.Itoa(int(v))
	default:
		return fmt.Sprint(v)
	}
}

func incrementTopRoute(items []model.TopRouteMetric, entry model.RequestLog, limit int) []model.TopRouteMetric {
	key := strings.TrimSpace(entry.NormalizedPath)
	if key == "" {
		key = entry.Path
	}
	for i := range items {
		if strings.EqualFold(items[i].Key, key) {
			previous := items[i].DocCount
			items[i].DocCount++
			if entry.IsError {
				items[i].ErrorCount++
			}
			items[i].AvgLatencyMS = movingAverage(items[i].AvgLatencyMS, previous, entry.ResponseTimeMS)
			items[i].Services = incrementBucket(items[i].Services, entry.ServiceName)
			return items
		}
	}
	items = append(items, model.TopRouteMetric{
		Key:          key,
		DocCount:     1,
		ErrorCount:   boolInt(entry.IsError),
		AvgLatencyMS: entry.ResponseTimeMS,
		Services:     []model.BucketMetric{{Key: entry.ServiceName, DocCount: 1}},
	})
	if limit <= 0 || len(items) <= limit {
		return items
	}
	return items[:limit]
}

func boolInt(value bool) int64 {
	if value {
		return 1
	}
	return 0
}
