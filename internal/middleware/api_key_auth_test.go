package middleware

import (
	"net/http/httptest"
	"testing"
	"time"

	cryptoutil "gateway-api/helper/crypto"
	configcache "gateway-api/internal/config/cache"

	"github.com/gofiber/fiber/v2"
)

type fakeAPIKeyCache struct {
	items map[string]configcache.APIKeyValue
}

func (f fakeAPIKeyCache) FindAPIKeyByHash(hash string) (configcache.APIKeyValue, bool) {
	apiKey, ok := f.items[hash]
	return apiKey, ok
}

type fakeLastUsedRecorder struct {
	calls int
	ids   []string
}

func (f *fakeLastUsedRecorder) MarkUsed(apiKeyID string) {
	f.calls++
	f.ids = append(f.ids, apiKeyID)
}

func TestHasScope(t *testing.T) {
	required := "01972f6a-0002-7000-8000-000000000001"
	tests := []struct {
		name     string
		scopeIDs []string
		want     bool
	}{
		{name: "required scope", scopeIDs: []string{required}, want: true},
		{name: "among many", scopeIDs: []string{"01972f6a-0002-7000-8000-000000000002", required}, want: true},
		{name: "missing", scopeIDs: []string{"01972f6a-0002-7000-8000-000000000002"}, want: false},
		{name: "wildcard is not supported", scopeIDs: []string{"*"}, want: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := hasScope(test.scopeIDs, required)
			if got != test.want {
				t.Fatalf("expected %v, got %v", test.want, got)
			}
		})
	}
}

func TestAPIKeyAuthRequiresHeader(t *testing.T) {
	status := runAPIKeyAuthRequest(t, apiKeyAuthFixture{cache: fakeAPIKeyCache{items: map[string]configcache.APIKeyValue{}}}, "", nil, nil)
	if status != fiber.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", status)
	}
}

func TestAPIKeyAuthRejectsUnknownKey(t *testing.T) {
	status := runAPIKeyAuthRequest(t, apiKeyAuthFixture{cache: fakeAPIKeyCache{items: map[string]configcache.APIKeyValue{}}}, "unknown", nil, nil)
	if status != fiber.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", status)
	}
}

func TestAPIKeyAuthRejectsInactiveRevokedExpiredOrInactiveClient(t *testing.T) {
	rawKey := "gw_live_test"
	now := time.Now()
	past := now.Add(-time.Minute)
	tests := []struct {
		name string
		key  configcache.APIKeyValue
	}{
		{name: "inactive key", key: validAPIKeyValue("key-id", []string{"scope-id"}, now.Add(time.Hour), stringPtr("user-id"), false, true, nil)},
		{name: "inactive client", key: validAPIKeyValue("key-id", []string{"scope-id"}, now.Add(time.Hour), stringPtr("user-id"), true, false, nil)},
		{name: "revoked", key: validAPIKeyValue("key-id", []string{"scope-id"}, now.Add(time.Hour), stringPtr("user-id"), true, true, &now)},
		{name: "expired", key: validAPIKeyValue("key-id", []string{"scope-id"}, past, stringPtr("user-id"), true, true, nil)},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cache := cacheForRawKey(t, rawKey, test.key)
			status := runAPIKeyAuthRequest(t, apiKeyAuthFixture{cache: cache}, rawKey, nil, nil)
			if status != fiber.StatusUnauthorized {
				t.Fatalf("expected 401, got %d", status)
			}
		})
	}
}

func TestAPIKeyAuthRejectsMissingRequiredScope(t *testing.T) {
	rawKey := "gw_live_test"
	cache := cacheForRawKey(t, rawKey, validAPIKeyValue("key-id", []string{"other-scope"}, time.Now().Add(time.Hour), nil, true, true, nil))
	requiredScope := "scope-id"

	status := runAPIKeyAuthRequest(t, apiKeyAuthFixture{cache: cache}, rawKey, &requiredScope, nil)
	if status != fiber.StatusForbidden {
		t.Fatalf("expected 403, got %d", status)
	}
}

func TestAPIKeyAuthAcceptsCachedKeyAndScrubsHeader(t *testing.T) {
	rawKey := "gw_live_test"
	ownerUserID := "user-id"
	cache := cacheForRawKey(t, rawKey, validAPIKeyValue("key-id", []string{"scope-id"}, time.Now().Add(time.Hour), &ownerUserID, true, true, nil))
	recorder := &fakeLastUsedRecorder{}
	requiredScope := "scope-id"
	var sawAPIKeyID any
	var sawUserID any
	var sawHeader string

	status := runAPIKeyAuthRequest(t, apiKeyAuthFixture{cache: cache, recorder: recorder}, rawKey, &requiredScope, func(c *fiber.Ctx) {
		sawAPIKeyID = c.Locals(LocalsAPIKeyID)
		sawUserID = c.Locals(LocalsUserID)
		sawHeader = string(c.Request().Header.Peek(apiKeyHeader))
	})
	if status != fiber.StatusNoContent {
		t.Fatalf("expected 204, got %d", status)
	}
	if sawAPIKeyID != "key-id" || sawUserID != ownerUserID {
		t.Fatalf("unexpected locals: api_key_id=%#v user_id=%#v", sawAPIKeyID, sawUserID)
	}
	if sawHeader != "" {
		t.Fatalf("expected API key header to be removed, got %q", sawHeader)
	}
	if recorder.calls != 1 || len(recorder.ids) != 1 || recorder.ids[0] != "key-id" {
		t.Fatalf("expected last_used mark for key-id, got calls=%d ids=%#v", recorder.calls, recorder.ids)
	}
}

type apiKeyAuthFixture struct {
	cache    fakeAPIKeyCache
	recorder *fakeLastUsedRecorder
}

func runAPIKeyAuthRequest(t *testing.T, fixture apiKeyAuthFixture, rawKey string, requiredScope *string, afterAuth func(*fiber.Ctx)) int {
	t.Helper()

	auth := NewAPIKeyAuth(fixture.recorder, fixture.cache)
	app := fiber.New()
	app.Get("/", func(c *fiber.Ctx) error {
		if err := auth.Authenticate(c, requiredScope); err != nil {
			return err
		}
		if c.Response().StatusCode() >= fiber.StatusBadRequest {
			return nil
		}
		if afterAuth != nil {
			afterAuth(c)
		}
		return c.SendStatus(fiber.StatusNoContent)
	})

	req := httptest.NewRequest(fiber.MethodGet, "/", nil)
	if rawKey != "" {
		req.Header.Set(apiKeyHeader, rawKey)
	}
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	return resp.StatusCode
}

func cacheForRawKey(t *testing.T, rawKey string, apiKey configcache.APIKeyValue) fakeAPIKeyCache {
	t.Helper()
	hash, err := cryptoutil.HashAPIKey(rawKey)
	if err != nil {
		t.Fatal(err)
	}
	apiKey.KeyHash = hash
	return fakeAPIKeyCache{items: map[string]configcache.APIKeyValue{hash: apiKey}}
}

func validAPIKeyValue(id string, scopeIDs []string, expiresAt time.Time, ownerUserID *string, isActive bool, clientActive bool, revokedAt *time.Time) configcache.APIKeyValue {
	return configcache.APIKeyValue{
		SchemaVersion: 1,
		ID:            id,
		KeyPrefix:     "gw_live_test",
		ClientID:      "client-id",
		OwnerUserID:   ownerUserID,
		ScopeIDs:      scopeIDs,
		ExpiresAt:     &expiresAt,
		IsActive:      isActive,
		RevokedAt:     revokedAt,
		ClientActive:  clientActive,
	}
}

func stringPtr(value string) *string {
	return &value
}
