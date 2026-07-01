package middleware

import (
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
	localLogServiceName    = "log_service_name"
	localLogAPIKeyID       = "api_key_id"
	localUpstreamLatencyMS = "log_upstream_latency_ms"
)

type requestLogEntry struct {
	Timestamp         time.Time `json:"@timestamp"`
	TraceID           string    `json:"trace_id"`
	RouteID           string    `json:"route_id"`
	ServiceName       string    `json:"service_name"`
	Method            string    `json:"method"`
	Path              string    `json:"path"`
	ClientIP          string    `json:"client_ip"`
	UserID            string    `json:"user_id"`
	APIKeyID          string    `json:"api_key_id"`
	StatusCode        int       `json:"status_code"`
	ResponseTimeMS    float64   `json:"response_time_ms"`
	UpstreamLatencyMS float64   `json:"upstream_latency_ms"`
	RequestSize       int       `json:"request_size"`
	ResponseSize      int       `json:"response_size"`
	ErrorMessage      string    `json:"error_message"`
}

func RequestLogger(writer io.Writer, appLogger *slog.Logger) fiber.Handler {
	if writer == nil {
		writer = io.Discard
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

		entry := requestLogEntry{
			Timestamp:         time.Now().UTC(),
			TraceID:           traceID,
			RouteID:           localString(c, localLogRouteID),
			ServiceName:       localString(c, localLogServiceName),
			Method:            c.Method(),
			Path:              c.Path(),
			ClientIP:          c.IP(),
			UserID:            localString(c, LocalsUserID),
			APIKeyID:          localString(c, localLogAPIKeyID),
			StatusCode:        c.Response().StatusCode(),
			ResponseTimeMS:    milliseconds(time.Since(startedAt)),
			UpstreamLatencyMS: localFloat64(c, localUpstreamLatencyMS),
			RequestSize:       len(c.Request().Body()),
			ResponseSize:      len(c.Response().Body()),
			ErrorMessage:      responseErrorMessage(c),
		}

		payload, marshalErr := json.Marshal(entry)
		if marshalErr != nil {
			logWriteError(appLogger, marshalErr)
			return nil
		}
		payload = append(payload, '\n')
		if _, writeErr := writer.Write(payload); writeErr != nil {
			logWriteError(appLogger, writeErr)
		}

		return nil
	}
}

func SetRouteLogContext(c *fiber.Ctx, routeID string, serviceName string) {
	c.Locals(localLogRouteID, routeID)
	c.Locals(localLogServiceName, serviceName)
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
