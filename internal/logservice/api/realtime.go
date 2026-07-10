package api

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"gateway-api/helper/response"
	"gateway-api/internal/logservice/model"
	"gateway-api/internal/middleware"

	"github.com/gofiber/fiber/v2"
)

const (
	defaultRealtimeWindow   = 60 * time.Second
	minRealtimeWindow       = 10 * time.Second
	maxRealtimeWindow       = 15 * time.Minute
	defaultRealtimeInterval = time.Second
	minRealtimeInterval     = time.Second
	maxRealtimeInterval     = time.Minute
	defaultRealtimeTopLimit = 10
	maxRealtimeTopLimit     = 50
	realtimeReconcileEvery  = 10 * time.Second
	realtimeFlushEvery      = 200 * time.Millisecond
)

// sse handler
func (h *Handler) RealtimeStream(c *fiber.Ctx) error {
	query, err := parseRealtimeQuery(c)
	if err != nil {
		return response.BadRequest(c, err.Error())
	}
	query, err = h.scopeRealtimeQuery(c, query)
	if err != nil {
		return response.InternalServerError(c)
	}

	c.Set(fiber.HeaderContentType, "text/event-stream") // sse stream
	c.Set(fiber.HeaderCacheControl, "no-cache")
	c.Set(fiber.HeaderConnection, "keep-alive") // giữ kết nối
	c.Set("X-Accel-Buffering", "no")

	requestCtx := c.UserContext()
	if requestCtx == nil {
		requestCtx = context.Background()
	}

	// giữ connection + stream
	c.Context().SetBodyStreamWriter(func(w *bufio.Writer) {
		ticker := time.NewTicker(realtimeReconcileEvery)
		defer ticker.Stop()

		pingTicker := time.NewTicker(15 * time.Second) // heartbeat ping giữ kết nối sse
		defer pingTicker.Stop()

		flushTicker := time.NewTicker(realtimeFlushEvery)
		defer flushTicker.Stop()

		if !writeSSE(w, "ping", "", fiber.Map{}) {
			return
		}

		var liveLogs <-chan model.RequestLog
		if h.realtime != nil {
			var unsubscribe func()
			liveLogs, unsubscribe = h.realtime.Subscribe(query)
			defer unsubscribe()
		}

		var snapshot model.DashboardSnapshot
		dirty := false
		sendMetrics := func() bool {
			ctx, cancel := context.WithTimeout(requestCtx, 5*time.Second)
			defer cancel()

			nextSnapshot, err := h.store.DashboardSnapshot(ctx, query)
			if err != nil {
				return writeSSE(w, "error", "", fiber.Map{
					"code":    "elasticsearch_unavailable",
					"message": "metrics temporarily unavailable",
				})
			}
			snapshot = nextSnapshot
			dirty = false
			return writeSSE(w, "metrics", snapshot.GeneratedAt, snapshot)
		}

		if !sendMetrics() {
			return
		}

		for {
			select {
			case <-requestCtx.Done():
				return
			case <-ticker.C: // gửi event
				if !sendMetrics() {
					return
				}
			case entry, ok := <-liveLogs:
				if !ok {
					liveLogs = nil
					continue
				}
				snapshot = applyRealtimeLog(snapshot, entry, query)
				dirty = true
			case <-flushTicker.C:
				if !dirty {
					continue
				}
				dirty = false
				if !writeSSE(w, "metrics", snapshot.GeneratedAt, snapshot) {
					return
				}
			case <-pingTicker.C:
				if !writeSSE(w, "ping", "", fiber.Map{}) {
					return
				}
			}
		}
	})

	return nil
}

func (h *Handler) scopeRealtimeQuery(c *fiber.Ctx, query model.RealtimeQuery) (model.RealtimeQuery, error) {
	if strings.EqualFold(middleware.GetUserRole(c), "admin") {
		return query, nil
	}
	userID := middleware.GetUserID(c)
	query.UserID = userID
	if h.routeScope == nil {
		return query, nil
	}

	allowedRouteIDs, err := h.routeScope.AllowedRouteIDs(c.UserContext(), userID)
	if err != nil {
		return query, err
	}
	if len(allowedRouteIDs) == 0 {
		query.NoResults = true
		return query, nil
	}
	if strings.TrimSpace(query.RouteID) != "" {
		if !containsRouteID(allowedRouteIDs, query.RouteID) {
			query.NoResults = true
			return query, nil
		}
		return query, nil
	}
	query.RouteIDs = allowedRouteIDs
	return query, nil
}
func parseRealtimeQuery(c *fiber.Ctx) (model.RealtimeQuery, error) {
	window, err := parseClampedDuration(c.Query("window"), defaultRealtimeWindow, minRealtimeWindow, maxRealtimeWindow)
	if err != nil {
		return model.RealtimeQuery{}, fmt.Errorf("invalid window")
	}
	interval, err := parseClampedDuration(c.Query("interval"), defaultRealtimeInterval, minRealtimeInterval, maxRealtimeInterval)
	if err != nil {
		return model.RealtimeQuery{}, fmt.Errorf("invalid interval")
	}

	method := strings.ToUpper(strings.TrimSpace(c.Query("method")))
	statusCode := strings.TrimSpace(c.Query("status_code"))
	if statusCode != "" {
		code, err := strconv.Atoi(statusCode)
		if err != nil || code < 100 || code > 599 {
			return model.RealtimeQuery{}, fmt.Errorf("invalid status_code")
		}
	}

	clientIP := strings.TrimSpace(c.Query("client_ip"))
	if clientIP != "" && net.ParseIP(clientIP) == nil {
		return model.RealtimeQuery{}, fmt.Errorf("invalid client_ip")
	}

	topLimit := queryInt(c, "top_limit", defaultRealtimeTopLimit)
	if topLimit > maxRealtimeTopLimit {
		topLimit = maxRealtimeTopLimit
	}

	return model.RealtimeQuery{
		Window:      formatDuration(window),
		Interval:    formatDuration(interval),
		ServiceName: strings.TrimSpace(c.Query("service_name")),
		RouteID:     strings.TrimSpace(c.Query("route_id")),
		Method:      method,
		StatusClass: strings.TrimSpace(c.Query("status_class")),
		StatusCode:  statusCode,
		ClientIP:    clientIP,
		TopLimit:    topLimit,
	}, nil
}

func parseClampedDuration(value string, fallback time.Duration, min time.Duration, max time.Duration) (time.Duration, error) {
	if strings.TrimSpace(value) == "" {
		return fallback, nil
	}
	duration, err := time.ParseDuration(value)
	if err != nil {
		return 0, err
	}
	if duration < min {
		return min, nil
	}
	if duration > max {
		return max, nil
	}
	return duration, nil
}

func formatDuration(duration time.Duration) string {
	if duration != defaultRealtimeWindow && duration%time.Minute == 0 {
		return fmt.Sprintf("%dm", int(duration/time.Minute))
	}
	return fmt.Sprintf("%ds", int(duration/time.Second))
}

func durationFromQuery(value string, fallback time.Duration) time.Duration {
	duration, err := time.ParseDuration(value)
	if err != nil || duration <= 0 {
		return fallback
	}
	return duration
}

func writeSSE(w *bufio.Writer, event string, id string, data any) bool {
	payload, err := json.Marshal(data)
	if err != nil {
		payload = []byte(`{"code":"marshal_error","message":"failed to encode event"}`)
		event = "error"
		id = ""
	}
	if event != "" {
		if _, err := fmt.Fprintf(w, "event: %s\n", event); err != nil {
			return false
		}
	}
	if id != "" {
		if _, err := fmt.Fprintf(w, "id: %s\n", id); err != nil {
			return false
		}
	}
	if _, err := fmt.Fprintf(w, "data: %s\n\n", payload); err != nil {
		return false
	}
	return w.Flush() == nil
}
