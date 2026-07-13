package corspolicies

import "testing"

func TestNormalizePolicyRejectsWildcardOriginWithCredentials(t *testing.T) {
	_, _, _, _, _, err := normalizePolicyValues([]string{"*"}, []string{"GET"}, []string{"Authorization"}, nil, true, nil)
	if err == nil {
		t.Fatal("expected wildcard origin with credentials to be rejected")
	}
}

func TestNormalizePolicyNormalizesOriginMethodAndOptions(t *testing.T) {
	origins, methods, headers, _, _, err := normalizePolicyValues([]string{" HTTPS://Example.COM/ "}, []string{"get"}, []string{" authorization "}, nil, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(origins) != 1 || origins[0] != "https://example.com" {
		t.Fatalf("unexpected origins: %#v", origins)
	}
	if !containsExact(methods, "GET") || !containsExact(methods, "OPTIONS") {
		t.Fatalf("expected GET and OPTIONS, got %#v", methods)
	}
	if len(headers) != 1 || headers[0] != "authorization" {
		t.Fatalf("unexpected headers: %#v", headers)
	}
}

func TestNormalizePolicyRejectsOriginPath(t *testing.T) {
	_, _, _, _, _, err := normalizePolicyValues([]string{"https://example.com/app"}, []string{"GET"}, nil, nil, false, nil)
	if err == nil {
		t.Fatal("expected origin with path to be rejected")
	}
}
