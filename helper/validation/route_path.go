package validation

import (
	"fmt"
	"strings"
)

func ValidateRouteRewriteTarget(routePath string, rewriteTarget *string) error {
	if rewriteTarget == nil || *rewriteTarget == "" {
		return nil
	}

	routeParams := ExtractRoutePathParams(routePath)
	for param := range ExtractRoutePathParams(*rewriteTarget) {
		if _, ok := routeParams[param]; !ok {
			return FieldError{Field: "rewrite_target", Message: fmt.Sprintf("param %q is not defined in route path", param)}
		}
	}

	return nil
}

func ValidateStripPrefix(routePath string, stripPrefix bool) error {
	if !stripPrefix || HasCatchAllRouteParam(routePath) {
		return nil
	}

	return FieldError{Field: "strip_prefix", Message: "strip_prefix requires route path to end with * or {name...}"}
}

func ExtractRoutePathParams(path string) map[string]struct{} {
	params := map[string]struct{}{}

	for _, part := range strings.Split(strings.Trim(path, "/"), "/") {
		if part == "*" {
			params["*"] = struct{}{}
			continue
		}

		if strings.HasPrefix(part, "{") && strings.HasSuffix(part, "}") {
			name := strings.TrimSuffix(strings.TrimPrefix(part, "{"), "}")
			if strings.HasSuffix(name, "...") {
				name = strings.TrimSuffix(name, "...")
			}
			if name != "" {
				params[name] = struct{}{}
			}
		}
	}

	return params
}

func HasCatchAllRouteParam(path string) bool {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) == 0 {
		return false
	}

	last := parts[len(parts)-1]
	return last == "*" || (strings.HasPrefix(last, "{") && strings.HasSuffix(last, "...}"))
}
