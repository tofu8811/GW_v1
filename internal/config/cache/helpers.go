package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

func setJSON(ctx context.Context, pipe redis.Pipeliner, key string, value any, ttl time.Duration) error {
	payload, err := json.Marshal(value)
	if err != nil {
		return err
	}

	if ttl > 0 {
		pipe.Set(ctx, key, payload, ttl)
		return nil
	}

	pipe.Set(ctx, key, payload, 0)
	return nil
}

func routeKey(method string, path string) string {
	return fmt.Sprintf("cfg:route:%s:%s", strings.ToUpper(method), path)
}

func serviceKey(serviceID string) string {
	return fmt.Sprintf("cfg:service:%s", serviceID)
}

func serviceInstancesKey(serviceID string) string {
	return fmt.Sprintf("cfg:service-instances:%s", serviceID)
}

func aggregationKey(method string, path string) string {
	return fmt.Sprintf("cfg:aggregation:%s:%s", strings.ToUpper(method), path)
}
func phaseOrder(phase string) int {
	switch phase {
	case "before_request":
		return 10
	case "proxy":
		return 20
	case "after_response":
		return 30
	case "on_error":
		return 40
	default:
		return 100
	}
}

func sortRoutes(routes []RouteValue) {
	sort.SliceStable(routes, func(i, j int) bool {
		if routes[i].Priority != routes[j].Priority {
			return routes[i].Priority > routes[j].Priority
		}
		if len(routes[i].Path) != len(routes[j].Path) {
			return len(routes[i].Path) > len(routes[j].Path)
		}
		return routes[i].RouteID < routes[j].RouteID
	})
}

func cloneRoute(route RouteValue) *RouteValue {
	cloned := route
	if route.RateLimit != nil {
		rateLimit := *route.RateLimit
		cloned.RateLimit = &rateLimit
	}
	if route.CORS != nil {
		corsValue := *route.CORS
		corsValue.AllowedOrigins = append([]string(nil), route.CORS.AllowedOrigins...)
		corsValue.AllowedMethods = append([]string(nil), route.CORS.AllowedMethods...)
		corsValue.AllowedHeaders = append([]string(nil), route.CORS.AllowedHeaders...)
		corsValue.ExposedHeaders = append([]string(nil), route.CORS.ExposedHeaders...)
		cloned.CORS = &corsValue
	}
	return &cloned
}

func cloneAggregation(aggregation AggregationValue) *AggregationValue {
	cloned := aggregation
	if aggregation.CORS != nil {
		corsValue := *aggregation.CORS
		corsValue.AllowedOrigins = append([]string(nil), aggregation.CORS.AllowedOrigins...)
		corsValue.AllowedMethods = append([]string(nil), aggregation.CORS.AllowedMethods...)
		corsValue.AllowedHeaders = append([]string(nil), aggregation.CORS.AllowedHeaders...)
		corsValue.ExposedHeaders = append([]string(nil), aggregation.CORS.ExposedHeaders...)
		cloned.CORS = &corsValue
	}
	if aggregation.Steps != nil {
		cloned.Steps = make([]AggregationStepValue, len(aggregation.Steps))
		for i, step := range aggregation.Steps {
			cloned.Steps[i] = cloneAggregationStep(step)
		}
	}
	return &cloned
}

func cloneAggregationStep(step AggregationStepValue) AggregationStepValue {
	cloned := step
	if step.RequestTemplate != nil {
		cloned.RequestTemplate = append(json.RawMessage(nil), step.RequestTemplate...)
	}
	if step.ResponseMapping != nil {
		cloned.ResponseMapping = append(json.RawMessage(nil), step.ResponseMapping...)
	}
	return cloned
}

func cloneInstances(instances []InstanceValue) []InstanceValue {
	if instances == nil {
		return nil
	}
	cloned := make([]InstanceValue, len(instances))
	copy(cloned, instances)
	return cloned
}
func cloneAPIKey(apiKey APIKeyValue) APIKeyValue {
	cloned := apiKey
	if apiKey.ScopeIDs != nil {
		cloned.ScopeIDs = append([]string(nil), apiKey.ScopeIDs...)
	}
	return cloned
}
func ParseDurationSeconds(value string, fallback time.Duration) time.Duration {
	seconds, err := strconv.Atoi(value)
	if err != nil || seconds < 0 {
		return fallback
	}
	return time.Duration(seconds) * time.Second
}
