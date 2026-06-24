package validation

import (
	"fmt"
	"net/netip"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"
)

const (
	DefaultRouteMethod = "GET"
	MaxRoutePathLength = 255
)

var validMethods = map[string]struct{}{
	"GET":     {},
	"POST":    {},
	"PUT":     {},
	"PATCH":   {},
	"DELETE":  {},
	"HEAD":    {},
	"OPTIONS": {},
	"ANY":     {},
}

type FieldError struct {
	Field   string
	Message string
}

// Error trả về lỗi dạng "field: message" để log và response dễ đọc.
func (e FieldError) Error() string {
	if e.Field == "" {
		return e.Message
	}

	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

// NormalizeMethod chuẩn hóa HTTP method sang chữ hoa và kiểm tra method hợp lệ.
func NormalizeMethod(method string) (string, error) {
	normalized := strings.ToUpper(strings.TrimSpace(method))
	if normalized == "" {
		return "", FieldError{Field: "method", Message: "method is required"}
	}

	if _, ok := validMethods[normalized]; !ok {
		return "", FieldError{Field: "method", Message: "method is invalid"}
	}

	return normalized, nil
}

// // NormalizeMethodOrDefault dùng defaultMethod khi method rỗng, sau đó chuẩn hóa và validate.
// func NormalizeMethodOrDefault(method string, defaultMethod string) (string, error) {
// 	if strings.TrimSpace(method) == "" {
// 		method = defaultMethod
// 	}

// 	return NormalizeMethod(method)
// }

// NormalizeRouteMethod validate method cho route và có thể chặn ANY theo chính sách caller.
func NormalizeRouteMethod(method string, allowAny bool) (string, error) {
	normalized, err := NormalizeMethod(method)
	if err != nil {
		return "", err
	}

	if normalized == "ANY" && !allowAny {
		return "", FieldError{Field: "method", Message: "ANY method is not allowed"}
	}

	return normalized, nil
}

// NormalizeRouteMethodOrDefault dùng defaultMethod khi method rỗng rồi validate route method.
func NormalizeRouteMethodOrDefault(method string, defaultMethod string, allowAny bool) (string, error) {
	if strings.TrimSpace(method) == "" {
		method = defaultMethod
	}

	return NormalizeRouteMethod(method, allowAny)
}

// ValidatePath kiểm tra path API theo chính sách strict, không tự trim input sai.
func ValidatePath(path string) error {
	if path == "" {
		return FieldError{Field: "path", Message: "path is required"}
	}

	if path != strings.TrimSpace(path) {
		return FieldError{Field: "path", Message: "path must not have leading or trailing whitespace"}
	}

	if utf8.RuneCountInString(path) > MaxRoutePathLength {
		return FieldError{Field: "path", Message: "path must be at most 255 characters"}
	}

	if !strings.HasPrefix(path, "/") {
		return FieldError{Field: "path", Message: "path must start with /"}
	}

	for _, r := range path {
		if unicode.IsSpace(r) {
			return FieldError{Field: "path", Message: "path must not contain whitespace"}
		}

		if unicode.IsControl(r) {
			return FieldError{Field: "path", Message: "path must not contain control characters"}
		}
	}

	return nil
}

// NormalizePath trả về path giữ nguyên sau khi validate strict.
func NormalizePath(path string) (string, error) {
	if err := ValidatePath(path); err != nil {
		return "", err
	}

	return path, nil
}

// ParseRequiredUUID parse UUID bắt buộc và trả lỗi field nếu input rỗng hoặc sai định dạng.
func ParseRequiredUUID(field string, value string) (uuid.UUID, error) {
	normalized := strings.TrimSpace(value)
	if normalized == "" {
		return uuid.Nil, FieldError{Field: field, Message: "UUID is required"}
	}

	id, err := uuid.Parse(normalized)
	if err != nil {
		return uuid.Nil, FieldError{Field: field, Message: "UUID is invalid"}
	}

	return id, nil
}

// ParseOptionalUUID parse UUID tùy chọn; nil hoặc chuỗi rỗng được xem là không truyền.
func ParseOptionalUUID(field string, value *string) (*uuid.UUID, error) {
	if value == nil {
		return nil, nil
	}

	normalized := strings.TrimSpace(*value)
	if normalized == "" {
		return nil, nil
	}

	id, err := uuid.Parse(normalized)
	if err != nil {
		return nil, FieldError{Field: field, Message: "UUID is invalid"}
	}

	return &id, nil
}

// NormalizeCIDROrIP validate CIDR/IP và không tự mask CIDR có host bits.
func NormalizeCIDROrIP(field string, value string) (string, error) {
	normalized := strings.TrimSpace(value)
	if normalized == "" {
		return "", FieldError{Field: field, Message: "CIDR or IP is required"}
	}

	if prefix, err := netip.ParsePrefix(normalized); err == nil {
		if prefix != prefix.Masked() {
			return "", FieldError{Field: field, Message: "CIDR address must not contain host bits"}
		}

		return prefix.String(), nil
	}

	addr, err := netip.ParseAddr(normalized)
	if err != nil {
		return "", FieldError{Field: field, Message: "CIDR or IP is invalid"}
	}

	if addr.Is4() {
		return netip.PrefixFrom(addr, 32).String(), nil
	}

	return netip.PrefixFrom(addr, 128).String(), nil
}

// ValidateEnum kiểm tra enum theo danh sách allowed, không đổi hoa/thường và không tự trim.
func ValidateEnum(field string, value string, allowed ...string) error {
	if value == "" {
		return FieldError{Field: field, Message: "value is required"}
	}

	if value != strings.TrimSpace(value) {
		return FieldError{Field: field, Message: "value must not have leading or trailing whitespace"}
	}

	for _, item := range allowed {
		if value == item {
			return nil
		}
	}

	return FieldError{Field: field, Message: "value is invalid"}
}

// ValidateIntMin kiểm tra số nguyên phải lớn hơn hoặc bằng min.
func ValidateIntMin(field string, value int, min int) error {
	if value < min {
		return FieldError{Field: field, Message: fmt.Sprintf("value must be at least %d", min)}
	}

	return nil
}

// ValidateIntGreaterThan kiểm tra số nguyên phải lớn hơn min.
func ValidateIntGreaterThan(field string, value int, min int) error {
	if value <= min {
		return FieldError{Field: field, Message: fmt.Sprintf("value must be greater than %d", min)}
	}

	return nil
}

// ValidateIntBetween kiểm tra số nguyên nằm trong khoảng min..max, bao gồm hai đầu.
func ValidateIntBetween(field string, value int, min int, max int) error {
	if value < min || value > max {
		return FieldError{Field: field, Message: fmt.Sprintf("value must be between %d and %d", min, max)}
	}

	return nil
}

// ParseTimestamp parse timestamp bắt buộc, mặc định dùng RFC3339 cho TIMESTAMPTZ.
func ParseTimestamp(field string, value string, layouts ...string) (time.Time, error) {
	normalized := strings.TrimSpace(value)
	if normalized == "" {
		return time.Time{}, FieldError{Field: field, Message: "timestamp is required"}
	}

	if len(layouts) == 0 {
		layouts = []string{time.RFC3339}
	}

	for _, layout := range layouts {
		parsed, err := time.Parse(layout, normalized)
		if err == nil {
			return parsed, nil
		}
	}

	return time.Time{}, FieldError{Field: field, Message: "timestamp must be RFC3339"}
}

// ParseOptionalTimestamp parse timestamp tùy chọn; nil hoặc chuỗi rỗng được xem là không truyền.
func ParseOptionalTimestamp(field string, value *string, layouts ...string) (*time.Time, error) {
	if value == nil {
		return nil, nil
	}

	normalized := strings.TrimSpace(*value)
	if normalized == "" {
		return nil, nil
	}

	parsed, err := ParseTimestamp(field, normalized, layouts...)
	if err != nil {
		return nil, err
	}

	return &parsed, nil
}
