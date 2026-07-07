package middleware

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
)

const (
	localLogRouteID        = "log_route_id"
	localLogServiceID      = "log_service_id"
	localLogServiceName    = "log_service_name"
	localLogInstanceID     = "log_instance_id"
	localLogNormalizedPath = "log_normalized_path"
	localLogAPIKeyID       = "api_key_id"
	localUpstreamLatencyMS = "log_upstream_latency_ms"
)

type RequestLogEntry struct {
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

type requestLogEntry = RequestLogEntry

type RequestLogSink interface {
	WriteLog(ctx context.Context, entry RequestLogEntry) error
}

type JSONLineSink struct {
	writer io.Writer
}

func NewJSONLineSink(writer io.Writer) *JSONLineSink {
	if writer == nil {
		writer = io.Discard
	}
	return &JSONLineSink{writer: writer}
}

func (s *JSONLineSink) WriteLog(ctx context.Context, entry RequestLogEntry) error {
	payload, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	payload = append(payload, '\n')
	_, err = s.writer.Write(payload)
	return err
}

type MultiLogSink struct {
	sinks []RequestLogSink
}

func NewMultiLogSink(sinks ...RequestLogSink) *MultiLogSink {
	filtered := make([]RequestLogSink, 0, len(sinks))
	for _, sink := range sinks {
		if sink != nil {
			filtered = append(filtered, sink)
		}
	}
	return &MultiLogSink{sinks: filtered}
}

func (s *MultiLogSink) WriteLog(ctx context.Context, entry RequestLogEntry) error {
	var firstErr error
	for _, sink := range s.sinks {
		if err := sink.WriteLog(ctx, entry); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func Logger(writer io.Writer, appLogger *slog.Logger) fiber.Handler {
	return LoggerWithSink(NewJSONLineSink(writer), appLogger, "", "")
}

func LoggerWithSink(sink RequestLogSink, appLogger *slog.Logger, env string, gatewayNode string) fiber.Handler {
	if sink == nil {
		sink = NewJSONLineSink(io.Discard)
	}
	return func(c *fiber.Ctx) error {
		startedAt := time.Now()
		traceID := c.GetRespHeader(fiber.HeaderXRequestID)
		if traceID == "" {
			traceID = c.Get(fiber.HeaderXRequestID)
		}
		traceID = strings.Clone(traceID)
		err := c.Next()
		if err != nil {
			if handlerErr := c.App().ErrorHandler(c, err); handlerErr != nil {
				return handlerErr
			}
		}

		statusCode := c.Response().StatusCode()
		upstreamLatencyMS := localFloat64(c, localUpstreamLatencyMS)
		responseTimeMS := milliseconds(time.Since(startedAt))
		entry := requestLogEntry{
			Timestamp:         time.Now().UTC(),
			TraceID:           traceID,
			RouteID:           localString(c, localLogRouteID),
			ServiceID:         localString(c, localLogServiceID),
			ServiceName:       localString(c, localLogServiceName),
			InstanceID:        localString(c, localLogInstanceID),
			Method:            c.Method(),
			Path:              c.Path(),
			NormalizedPath:    localString(c, localLogNormalizedPath),
			QueryParams:       queryParams(c),
			ClientIP:          c.IP(),
			UserID:            localString(c, LocalsUserID),
			APIKeyID:          localString(c, localLogAPIKeyID),
			StatusCode:        statusCode,
			StatusClass:       statusClass(statusCode),
			IsError:           statusCode >= fiber.StatusBadRequest,
			ResponseTimeMS:    responseTimeMS,
			GatewayLatencyMS:  responseTimeMS - upstreamLatencyMS,
			UpstreamLatencyMS: upstreamLatencyMS,
			RequestSize:       len(c.Request().Body()),
			ResponseSize:      len(c.Response().Body()),
			ErrorMessage:      responseErrorMessage(c),
			ErrorType:         errorType(statusCode),
			UserAgent:         c.Get(fiber.HeaderUserAgent),
			Env:               env,
			GatewayNode:       gatewayNode,
		}

		if entry.GatewayLatencyMS < 0 {
			entry.GatewayLatencyMS = 0
		}

		if writeErr := sink.WriteLog(c.Context(), entry); writeErr != nil {
			logWriteError(appLogger, writeErr)
		}

		return nil
	}
}

func SetRouteLogContext(c *fiber.Ctx, routeID string, serviceID string, serviceName string, normalizedPath string) {
	c.Locals(localLogRouteID, routeID)
	c.Locals(localLogServiceID, serviceID)
	c.Locals(localLogServiceName, serviceName)
	c.Locals(localLogNormalizedPath, normalizedPath)
}

func SetInstanceLogContext(c *fiber.Ctx, instanceID string) {
	c.Locals(localLogInstanceID, instanceID)
}

func SetAPIKeyLogContext(c *fiber.Ctx, apiKeyID string) {
	c.Locals(localLogAPIKeyID, apiKeyID)
}

func AddUpstreamLatency(c *fiber.Ctx, latency time.Duration) {
	current := localFloat64(c, localUpstreamLatencyMS)
	c.Locals(localUpstreamLatencyMS, current+milliseconds(latency))
}

func localString(c *fiber.Ctx, key string) string {
	value, _ := c.Locals(key).(string)
	return value
}

func localFloat64(c *fiber.Ctx, key string) float64 {
	value, _ := c.Locals(key).(float64)
	return value
}

func milliseconds(duration time.Duration) float64 {
	return float64(duration) / float64(time.Millisecond)
}

func queryParams(c *fiber.Ctx) map[string]string {
	values := map[string]string{}
	c.Context().QueryArgs().VisitAll(func(key []byte, value []byte) {
		values[string(key)] = string(value)
	})
	if len(values) == 0 {
		return nil
	}
	return values
}

func statusClass(status int) string {
	switch {
	case status >= 100 && status < 200:
		return "1xx"
	case status >= 200 && status < 300:
		return "2xx"
	case status >= 300 && status < 400:
		return "3xx"
	case status >= 400 && status < 500:
		return "4xx"
	case status >= 500 && status < 600:
		return "5xx"
	default:
		return "unknown"
	}
}

func errorType(status int) string {
	switch {
	case status >= 500:
		return "server_error"
	case status >= 400:
		return "client_error"
	default:
		return ""
	}
}

func responseErrorMessage(c *fiber.Ctx) string {
	status := c.Response().StatusCode()
	if status < fiber.StatusBadRequest {
		return ""
	}

	var body struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(c.Response().Body(), &body); err == nil {
		if message := strings.TrimSpace(body.Error.Message); message != "" {
			return message
		}
		if message := strings.TrimSpace(body.Message); message != "" {
			return message
		}
	}

	return http.StatusText(status)
}

func logWriteError(logger *slog.Logger, err error) {
	if logger != nil {
		logger.Error("failed to write request log", "error", err)
	}
}
