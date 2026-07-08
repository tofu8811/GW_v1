package cache

import "testing"

func TestFindAPIKeyByHashReturnsClone(t *testing.T) {
	store := &Store{apiKeysByHash: map[string]APIKeyValue{
		"hash": {ID: "key-id", KeyHash: "hash", ScopeIDs: []string{"scope-a"}, IsActive: true, ClientActive: true},
	}}

	apiKey, ok := store.FindAPIKeyByHash("hash")
	if !ok {
		t.Fatal("expected API key to be found")
	}
	apiKey.ScopeIDs[0] = "mutated"

	again, ok := store.FindAPIKeyByHash("hash")
	if !ok {
		t.Fatal("expected API key to be found")
	}
	if again.ScopeIDs[0] != "scope-a" {
		t.Fatalf("expected cached scopes to be cloned, got %#v", again.ScopeIDs)
	}
}

func TestFindAPIKeyByHashMiss(t *testing.T) {
	store := &Store{apiKeysByHash: map[string]APIKeyValue{}}
	if _, ok := store.FindAPIKeyByHash("missing"); ok {
		t.Fatal("expected cache miss")
	}
}
