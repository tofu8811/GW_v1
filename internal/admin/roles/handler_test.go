package roles

import (
	"testing"

	"github.com/google/uuid"
)

func TestToResponseReturnsRoleName(t *testing.T) {
	item := Role{ID: uuid.New(), Name: "admin"}
	got := toResponse(item)
	if got.Name != "admin" {
		t.Fatalf("expected admin, got %q", got.Name)
	}
}
