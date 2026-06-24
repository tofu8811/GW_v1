package validation

import (
	"strings"
	"testing"
	"time"
)

func TestNormalizeMethod(t *testing.T) {
	got, err := NormalizeMethod(" post ")
	if err != nil {
		t.Fatalf("expected method to be valid: %v", err)
	}

	if got != "POST" {
		t.Fatalf("expected POST, got %q", got)
	}
}

func TestNormalizeMethodInvalid(t *testing.T) {
	if _, err := NormalizeMethod("TRACE"); err == nil {
		t.Fatal("expected invalid method error")
	}
}

func TestNormalizeMethodOrDefault(t *testing.T) {
	got, err := NormalizeMethodOrDefault("", DefaultRouteMethod)
	if err != nil {
		t.Fatalf("expected default method to be valid: %v", err)
	}

	if got != DefaultRouteMethod {
		t.Fatalf("expected %q, got %q", DefaultRouteMethod, got)
	}
}

func TestNormalizeRouteMethodRejectsAnyWhenNotAllowed(t *testing.T) {
	if _, err := NormalizeRouteMethod("ANY", false); err == nil {
		t.Fatal("expected ANY to be rejected when it is not allowed")
	}
}

func TestNormalizeRouteMethodAllowsAny(t *testing.T) {
	got, err := NormalizeRouteMethod("ANY", true)
	if err != nil {
		t.Fatalf("expected ANY to be valid when allowed: %v", err)
	}

	if got != "ANY" {
		t.Fatalf("expected ANY, got %q", got)
	}
}

func TestNormalizePath(t *testing.T) {
	got, err := NormalizePath("/api/products")
	if err != nil {
		t.Fatalf("expected path to be valid: %v", err)
	}

	if got != "/api/products" {
		t.Fatalf("expected path to be unchanged, got %q", got)
	}
}

func TestValidatePathRejectsOuterWhitespace(t *testing.T) {
	if err := ValidatePath(" /api/products "); err == nil {
		t.Fatal("expected path with outer whitespace to be invalid")
	}
}

func TestValidatePathRejectsInnerWhitespace(t *testing.T) {
	if err := ValidatePath("/api/products list"); err == nil {
		t.Fatal("expected path with inner whitespace to be invalid")
	}
}

func TestValidatePathRejectsTooLongPath(t *testing.T) {
	path := "/" + strings.Repeat("a", MaxRoutePathLength)
	if err := ValidatePath(path); err == nil {
		t.Fatal("expected path longer than max length to be invalid")
	}
}

func TestValidatePathInvalid(t *testing.T) {
	if err := ValidatePath("api/products"); err == nil {
		t.Fatal("expected path without slash to be invalid")
	}
}

func TestParseRequiredUUID(t *testing.T) {
	got, err := ParseRequiredUUID("service_id", "01972f6a-0001-7000-8000-000000000001")
	if err != nil {
		t.Fatalf("expected UUID to be valid: %v", err)
	}

	if got.String() != "01972f6a-0001-7000-8000-000000000001" {
		t.Fatalf("unexpected UUID: %s", got)
	}
}

func TestParseOptionalUUID(t *testing.T) {
	empty := " "
	got, err := ParseOptionalUUID("rate_limit_id", &empty)
	if err != nil {
		t.Fatalf("expected empty optional UUID to be valid: %v", err)
	}

	if got != nil {
		t.Fatalf("expected nil optional UUID, got %v", got)
	}
}

func TestParseOptionalUUIDInvalid(t *testing.T) {
	value := "not-a-uuid"
	if _, err := ParseOptionalUUID("rate_limit_id", &value); err == nil {
		t.Fatal("expected invalid optional UUID error")
	}
}

func TestNormalizeCIDROrIP(t *testing.T) {
	tests := map[string]string{
		"192.168.1.1":    "192.168.1.1/32",
		"192.168.1.0/24": "192.168.1.0/24",
		"2001:db8::1":    "2001:db8::1/128",
	}

	for input, want := range tests {
		got, err := NormalizeCIDROrIP("ip_or_cidr", input)
		if err != nil {
			t.Fatalf("expected %q to be valid: %v", input, err)
		}

		if got != want {
			t.Fatalf("expected %q, got %q", want, got)
		}
	}
}

func TestNormalizeCIDROrIPRejectsHostBits(t *testing.T) {
	if _, err := NormalizeCIDROrIP("ip_or_cidr", "192.168.1.9/24"); err == nil {
		t.Fatal("expected CIDR with host bits to be invalid")
	}
}

func TestNormalizeCIDROrIPInvalid(t *testing.T) {
	if _, err := NormalizeCIDROrIP("ip_or_cidr", "not-a-cidr"); err == nil {
		t.Fatal("expected invalid CIDR error")
	}
}

func TestValidateEnum(t *testing.T) {
	if err := ValidateEnum("protocol", "http", "http", "grpc"); err != nil {
		t.Fatalf("expected enum to be valid: %v", err)
	}
}

func TestValidateEnumRejectsWrongCase(t *testing.T) {
	if err := ValidateEnum("protocol", "HTTP", "http", "grpc"); err == nil {
		t.Fatal("expected enum with wrong case to be invalid")
	}
}

func TestValidateEnumRejectsWhitespace(t *testing.T) {
	if err := ValidateEnum("protocol", " http ", "http", "grpc"); err == nil {
		t.Fatal("expected enum with whitespace to be invalid")
	}
}

func TestValidateIntMin(t *testing.T) {
	if err := ValidateIntMin("retry_count", 0, 0); err != nil {
		t.Fatalf("expected value to be valid: %v", err)
	}

	if err := ValidateIntMin("retry_count", -1, 0); err == nil {
		t.Fatal("expected value below minimum to be invalid")
	}
}

func TestValidateIntGreaterThan(t *testing.T) {
	if err := ValidateIntGreaterThan("timeout_ms", 1, 0); err != nil {
		t.Fatalf("expected value to be valid: %v", err)
	}

	if err := ValidateIntGreaterThan("timeout_ms", 0, 0); err == nil {
		t.Fatal("expected value equal to minimum to be invalid")
	}
}

func TestValidateIntBetween(t *testing.T) {
	if err := ValidateIntBetween("port", 5432, 1, 65535); err != nil {
		t.Fatalf("expected value to be valid: %v", err)
	}

	if err := ValidateIntBetween("port", 65536, 1, 65535); err == nil {
		t.Fatal("expected value above range to be invalid")
	}
}

func TestParseTimestamp(t *testing.T) {
	got, err := ParseTimestamp("expires_at", "2026-06-24T10:30:00Z")
	if err != nil {
		t.Fatalf("expected timestamp to be valid: %v", err)
	}

	if got.Format(time.RFC3339) != "2026-06-24T10:30:00Z" {
		t.Fatalf("unexpected timestamp: %s", got.Format(time.RFC3339))
	}
}

func TestParseOptionalTimestamp(t *testing.T) {
	empty := ""
	got, err := ParseOptionalTimestamp("expires_at", &empty)
	if err != nil {
		t.Fatalf("expected empty optional timestamp to be valid: %v", err)
	}

	if got != nil {
		t.Fatalf("expected nil optional timestamp, got %v", got)
	}
}
