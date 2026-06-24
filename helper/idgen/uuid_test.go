package idgen

import "testing"

func TestNewUUIDReturnsVersion7(t *testing.T) {
	id, err := NewUUID()
	if err != nil {
		t.Fatalf("expected UUIDv7 to be generated: %v", err)
	}

	if id.Version() != 7 {
		t.Fatalf("expected UUID version 7, got %d", id.Version())
	}
}

func TestMustNewUUIDReturnsVersion7(t *testing.T) {
	id := MustNewUUID()

	if id.Version() != 7 {
		t.Fatalf("expected UUID version 7, got %d", id.Version())
	}
}
