package apikeys

import "testing"

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
