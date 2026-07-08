package apikeys

import (
	"testing"

	"github.com/google/uuid"
)

func TestParseScopeIDs(t *testing.T) {
	ids, err := parseScopeIDs([]string{
		"01972f6a-0002-7000-8000-000000000001",
		" 01972f6a-0002-7000-8000-000000000001 ",
		"01972f6a-0002-7000-8000-000000000002",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 2 {
		t.Fatalf("unexpected scope ids: %#v", ids)
	}
}

func TestParseScopeIDsRequiresValue(t *testing.T) {
	if _, err := parseScopeIDs([]string{" "}); err == nil {
		t.Fatal("expected an error")
	}
}

func TestParseScopeIDsRejectsInvalidUUID(t *testing.T) {
	if _, err := parseScopeIDs([]string{"products:read"}); err == nil {
		t.Fatal("expected an error")
	}
}

func TestToResponseReturnsScopeDetails(t *testing.T) {
	scopeID := uuid.MustParse("01972f6a-0002-7000-8000-000000000003")
	key := APIKey{
		ID:       uuid.MustParse("01972f6a-0002-7000-8000-000000000001"),
		ClientID: uuid.MustParse("01972f6a-0002-7000-8000-000000000002"),
		ScopeIDs: []uuid.UUID{scopeID},
		Scopes: []APIScope{{
			ID: scopeID, Code: "products:read", Resource: "products", Action: "read",
		}},
	}

	response := toResponse(key)
	if len(response.Scopes) != 1 || response.Scopes[0].Code != "products:read" || response.Scopes[0].ID != scopeID.String() {
		t.Fatalf("expected scope details, got %#v", response.Scopes)
	}
}
