package api

import (
	"context"
	"strconv"
	"strings"
	"time"

	"gateway-api/helper/response"
	"gateway-api/internal/logservice/model"
	"gateway-api/internal/middleware"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Store interface {
	Ping(ctx context.Context) error
	SearchLogs(ctx context.Context, query model.LogQuery) ([]model.RequestLog, int64, error)
	Summary(ctx context.Context, query model.LogQuery) (map[string]any, error)
	RPS(ctx context.Context, query model.LogQuery) ([]map[string]any, error)
	ErrorRate(ctx context.Context, query model.LogQuery) (map[string]any, error)
	Latency(ctx context.Context, query model.LogQuery) (map[string]any, error)
	StatusCodes(ctx context.Context, query model.LogQuery) (map[string]any, error)
	TopRoutes(ctx context.Context, query model.LogQuery) ([]map[string]any, error)
	DashboardSnapshot(ctx context.Context, query model.RealtimeQuery) (model.DashboardSnapshot, error)
}

type Handler struct {
	store      Store
	routeScope RouteScopeStore
	realtime   *RealtimeHub
}

type RouteScopeStore interface {
	AllowedRouteIDs(ctx context.Context, userID string) ([]string, error)
}

type PostgresRouteScopeStore struct {
	db *pgxpool.Pool
}

func NewPostgresRouteScopeStore(db *pgxpool.Pool) *PostgresRouteScopeStore {
	return &PostgresRouteScopeStore{db: db}
}

func (s *PostgresRouteScopeStore) AllowedRouteIDs(ctx context.Context, userID string) ([]string, error) {
	rows, err := s.db.Query(ctx, `
		SELECT DISTINCT r.id::text
		FROM routes r
		JOIN api_scopes sc
		  ON sc.id = r.required_scope_id
		 AND sc.is_active = TRUE
		 AND sc.deleted_at IS NULL
		JOIN api_key_scopes aks
		  ON aks.scope_id = sc.id
		JOIN api_keys ak
		  ON ak.id = aks.api_key_id
		 AND ak.is_active = TRUE
		 AND ak.deleted_at IS NULL
		 AND ak.revoked_at IS NULL
		 AND (ak.expires_at IS NULL OR ak.expires_at > now())
		JOIN clients c
		  ON c.id = ak.client_id
		 AND c.is_active = TRUE
		 AND c.deleted_at IS NULL
		WHERE r.is_active = TRUE
		  AND r.deleted_at IS NULL
		  AND c.owner_user_id::text = $1
		ORDER BY r.id::text
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	routeIDs := make([]string, 0)
	for rows.Next() {
		var routeID string
		if err := rows.Scan(&routeID); err != nil {
			return nil, err
		}
		routeIDs = append(routeIDs, routeID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return routeIDs, nil
}

func NewHandler(store Store, routeScope RouteScopeStore, realtime ...*RealtimeHub) *Handler {
	var hub *RealtimeHub
	if len(realtime) > 0 {
		hub = realtime[0]
	}
	return &Handler{store: store, routeScope: routeScope, realtime: hub}
}

func RegisterRoutes(app *fiber.App, handler *Handler, middlewares ...fiber.Handler) {
	admin := app.Group("/admin")
	for _, middleware := range middlewares {
		if middleware != nil {
			admin.Use(middleware)
		}
	}

	admin.Get("/logging/health", middleware.RequirePermission("health:read"), handler.Health)
	admin.Get("/logs", middleware.RequirePermission("logs:read"), handler.Logs)
	admin.Get("/metrics/summary", middleware.RequirePermission("metrics:read"), handler.Summary)
	admin.Get("/metrics/rps", middleware.RequirePermission("metrics:read"), handler.RPS)
	admin.Get("/metrics/error-rate", middleware.RequirePermission("metrics:read"), handler.ErrorRate)
	admin.Get("/metrics/latency", middleware.RequirePermission("metrics:read"), handler.Latency)
	admin.Get("/metrics/status-codes", middleware.RequirePermission("metrics:read"), handler.StatusCodes)
	admin.Get("/metrics/top-routes", middleware.RequirePermission("metrics:read"), handler.TopRoutes)
	admin.Get("/metrics/realtime/stream", middleware.RequirePermission("metrics:read"), handler.RealtimeStream)
}

func (h *Handler) Health(c *fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(c.UserContext(), 2*time.Second)
	defer cancel()

	status := "ok"
	elasticsearchStatus := "ok"
	if err := h.store.Ping(ctx); err != nil {
		status = "error"
		elasticsearchStatus = "error"
	}
	data := fiber.Map{"status": status, "elasticsearch": elasticsearchStatus}
	if status != "ok" {
		return response.ErrorWithDetails(c, fiber.StatusServiceUnavailable, "service_unavailable", "Logging service dependency is unavailable", data)
	}
	return response.OK(c, data)
}

func (h *Handler) Logs(c *fiber.Ctx) error {
	query, err := h.scopeLogQuery(c, parseLogQuery(c))
	if err != nil {
		return response.InternalServerError(c)
	}
	items, total, err := h.store.SearchLogs(c.UserContext(), query)
	if err != nil {
		return response.InternalServerError(c)
	}
	return response.WithMeta(c, items, model.PageMeta{Page: query.Page, Limit: query.Limit, Total: total})
}

func (h *Handler) Summary(c *fiber.Ctx) error {
	query, err := h.scopedMetricQuery(c)
	if err != nil {
		return response.InternalServerError(c)
	}
	data, err := h.store.Summary(c.UserContext(), query)
	if err != nil {
		return response.InternalServerError(c)
	}
	return response.OK(c, metricResponse(query, data))
}

func (h *Handler) RPS(c *fiber.Ctx) error {
	query, err := h.scopedMetricQuery(c)
	if err != nil {
		return response.InternalServerError(c)
	}
	data, err := h.store.RPS(c.UserContext(), query)
	if err != nil {
		return response.InternalServerError(c)
	}
	return response.OK(c, metricResponse(query, data))
}

func (h *Handler) ErrorRate(c *fiber.Ctx) error {
	query, err := h.scopedMetricQuery(c)
	if err != nil {
		return response.InternalServerError(c)
	}
	data, err := h.store.ErrorRate(c.UserContext(), query)
	if err != nil {
		return response.InternalServerError(c)
	}
	return response.OK(c, metricResponse(query, data))
}

func (h *Handler) Latency(c *fiber.Ctx) error {
	query, err := h.scopedMetricQuery(c)
	if err != nil {
		return response.InternalServerError(c)
	}
	data, err := h.store.Latency(c.UserContext(), query)
	if err != nil {
		return response.InternalServerError(c)
	}
	return response.OK(c, metricResponse(query, data))
}

func (h *Handler) StatusCodes(c *fiber.Ctx) error {
	query, err := h.scopedMetricQuery(c)
	if err != nil {
		return response.InternalServerError(c)
	}
	data, err := h.store.StatusCodes(c.UserContext(), query)
	if err != nil {
		return response.InternalServerError(c)
	}
	return response.OK(c, metricResponse(query, data))
}

func (h *Handler) TopRoutes(c *fiber.Ctx) error {
	query, err := h.scopedMetricQuery(c)
	if err != nil {
		return response.InternalServerError(c)
	}
	query.TopSortBy = c.Query("sort_by", "requests")
	query.TopLimit = queryInt(c, "limit", 10)
	data, err := h.store.TopRoutes(c.UserContext(), query)
	if err != nil {
		return response.InternalServerError(c)
	}
	return response.OK(c, metricResponse(query, data))
}

func (h *Handler) scopedMetricQuery(c *fiber.Ctx) (model.LogQuery, error) {
	query := parseLogQuery(c)
	query.ExcludeControlPlane = true
	return h.scopeLogQuery(c, query)
}
func parseLogQuery(c *fiber.Ctx) model.LogQuery {
	return model.LogQuery{
		From:           c.Query("from"),
		To:             c.Query("to"),
		ServiceName:    c.Query("service_name"),
		RouteID:        c.Query("route_id"),
		UserID:         c.Query("user_id"),
		Method:         c.Query("method"),
		StatusClass:    c.Query("status_class"),
		StatusCode:     c.Query("status_code"),
		ClientIP:       c.Query("client_ip"),
		TraceID:        c.Query("trace_id"),
		Path:           c.Query("path"),
		NormalizedPath: c.Query("normalized_path"),
		APIKeyID:       c.Query("api_key_id"),
		ErrorMessage:   c.Query("error_message"),
		Query:          c.Query("q"),
		Page:           queryInt(c, "page", 1),
		Limit:          queryInt(c, "limit", 20),
		Sort:           c.Query("sort", "@timestamp:desc"),
		Interval:       c.Query("interval", "1s"),
	}
}

func (h *Handler) scopeLogQuery(c *fiber.Ctx, query model.LogQuery) (model.LogQuery, error) {
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

func containsRouteID(routeIDs []string, routeID string) bool {
	for _, allowed := range routeIDs {
		if strings.EqualFold(allowed, routeID) {
			return true
		}
	}
	return false
}

func queryInt(c *fiber.Ctx, key string, fallback int) int {
	value, err := strconv.Atoi(c.Query(key))
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}

func metricResponse(query model.LogQuery, data any) fiber.Map {
	return fiber.Map{
		"from":    query.From,
		"to":      query.To,
		"filters": filters(query),
		"data":    data,
	}
}

func filters(query model.LogQuery) fiber.Map {
	out := fiber.Map{}
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
	add("trace_id", query.TraceID)
	add("path", query.Path)
	add("normalized_path", query.NormalizedPath)
	add("api_key_id", query.APIKeyID)
	add("error_message", query.ErrorMessage)
	add("interval", query.Interval)
	return out
}
