package apikeys

import (
	"testing"

	"github.com/google/uuid"
)

func TestParsePermissionIDs(t *testing.T) {
	ids, err := parsePermissionIDs([]string{
		"01972f6a-0002-7000-8000-000000000001",
		" 01972f6a-0002-7000-8000-000000000001 ",
		"01972f6a-0002-7000-8000-000000000002",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 2 {
		t.Fatalf("unexpected permission ids: %#v", ids)
	}
}

func TestParsePermissionIDsRequiresValue(t *testing.T) {
	if _, err := parsePermissionIDs([]string{" "}); err == nil {
		t.Fatal("expected an error")
	}
}

func TestParsePermissionIDsRejectsInvalidUUID(t *testing.T) {
	if _, err := parsePermissionIDs([]string{"GET:/api/orders"}); err == nil {
		t.Fatal("expected an error")
	}
}

func TestToResponseReturnsPermissionNamesInsteadOfIDs(t *testing.T) {
	key := APIKey{
		ID:            uuid.MustParse("01972f6a-0002-7000-8000-000000000001"),
		ClientID:      uuid.MustParse("01972f6a-0002-7000-8000-000000000002"),
		PermissionIDs: []uuid.UUID{uuid.MustParse("01972f6a-0002-7000-8000-000000000003")},
		Permissions:   []string{"services:read"},
	}

	response := toResponse(key)
	if len(response.Permissions) != 1 || response.Permissions[0].Name != "services:read" || response.Permissions[0].ID != key.PermissionIDs[0].String() {
		t.Fatalf("expected permission name, got %#v", response.Permissions)
	}
}
