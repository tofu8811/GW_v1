package permissions

import (
	"testing"

	"github.com/google/uuid"
)

func TestToResponseBuildsPermissionName(t *testing.T) {
	item := Permission{ID: uuid.New(), Resource: "services", Action: "read"}
	got := toResponse(item)
	if got.Name != "services:read" {
		t.Fatalf("expected services:read, got %q", got.Name)
	}
}
