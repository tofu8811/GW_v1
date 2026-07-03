package corsconfigs

import "testing"

func TestNormalizeConfigNormalizesAndDeduplicates(t *testing.T) {
	maxAge := 7200
	config, err := normalizeConfig(UpsertCORSConfigRequest{
		AllowedOrigins: []string{" http://localhost:5173 ", "http://localhost:5173"},
		AllowedMethods: []string{"get", " GET ", "post"},
		AllowedHeaders: []string{"Content-Type", " content-type ", "Authorization"},
		MaxAge:         &maxAge,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(config.AllowedOrigins) != 1 || config.AllowedOrigins[0] != "http://localhost:5173" {
		t.Fatalf("unexpected origins: %#v", config.AllowedOrigins)
	}
	if len(config.AllowedMethods) != 2 || config.AllowedMethods[0] != "GET" || config.AllowedMethods[1] != "POST" {
		t.Fatalf("unexpected methods: %#v", config.AllowedMethods)
	}
	if len(config.AllowedHeaders) != 2 || config.MaxAge != maxAge {
		t.Fatalf("unexpected config: %#v", config)
	}
}

func TestNormalizeConfigRejectsWildcardWithCredentials(t *testing.T) {
	_, err := normalizeConfig(UpsertCORSConfigRequest{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET"},
		AllowCredentials: true,
	})
	if err == nil {
		t.Fatal("expected wildcard credentials validation error")
	}
}

func TestNormalizeConfigRequiresOriginsAndMethods(t *testing.T) {
	if _, err := normalizeConfig(UpsertCORSConfigRequest{AllowedMethods: []string{"GET"}}); err == nil {
		t.Fatal("expected origins validation error")
	}
	if _, err := normalizeConfig(UpsertCORSConfigRequest{AllowedOrigins: []string{"http://localhost:5173"}}); err == nil {
		t.Fatal("expected methods validation error")
	}
}
