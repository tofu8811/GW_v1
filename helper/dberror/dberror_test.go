package dberror

import (
	"errors"
	"net/http"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

func TestClassifyPgError(t *testing.T) {
	err := &pgconn.PgError{Code: UniqueViolation, ConstraintName: "routes_path_method_unique"}

	code, constraint, ok := ClassifyPgError(err)
	if !ok {
		t.Fatal("expected PgError to be classified")
	}
	if code != UniqueViolation || constraint != "routes_path_method_unique" {
		t.Fatalf("unexpected classification: code=%q constraint=%q", code, constraint)
	}
}

func TestClassifyPgErrorWrapped(t *testing.T) {
	err := errors.Join(errors.New("insert failed"), &pgconn.PgError{Code: ForeignKeyViolation, ConstraintName: "routes_service_id_fkey"})

	code, constraint, ok := ClassifyPgError(err)
	if !ok {
		t.Fatal("expected wrapped PgError to be classified")
	}
	if code != ForeignKeyViolation || constraint != "routes_service_id_fkey" {
		t.Fatalf("unexpected classification: code=%q constraint=%q", code, constraint)
	}
}

func TestMapDBErrorConstraintSpecific(t *testing.T) {
	err := &pgconn.PgError{Code: UniqueViolation, ConstraintName: "routes_path_method_unique"}

	apiErr, ok := MapDBError(err)
	if !ok {
		t.Fatal("expected db error to be mapped")
	}
	if apiErr.Status != http.StatusConflict || apiErr.Code != "conflict" || apiErr.Message != "route path and method already exists" {
		t.Fatalf("unexpected api error: %+v", apiErr)
	}
}

func TestMapDBErrorFallbackBySQLState(t *testing.T) {
	err := &pgconn.PgError{Code: ForeignKeyViolation, ConstraintName: "unknown_fkey"}

	apiErr, ok := MapDBError(err)
	if !ok {
		t.Fatal("expected fallback db error to be mapped")
	}
	if apiErr.Status != http.StatusUnprocessableEntity || apiErr.Code != "invalid_reference" {
		t.Fatalf("unexpected fallback api error: %+v", apiErr)
	}
}

func TestMapDBErrorUnknownPgError(t *testing.T) {
	err := &pgconn.PgError{Code: "40001", ConstraintName: ""}

	_, ok := MapDBError(err)
	if ok {
		t.Fatal("expected unknown PgError to remain unmapped")
	}
}

func TestMapDBErrorNonPgError(t *testing.T) {
	_, ok := MapDBError(errors.New("connection failed"))
	if ok {
		t.Fatal("expected non-PgError to remain unmapped")
	}
}
