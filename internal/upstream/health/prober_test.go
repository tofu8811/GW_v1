package health

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"
)

func TestProbeHTTPStatusOK(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	target := targetFromHTTPServer(t, server, "/health")
	if _, err := Probe(context.Background(), target, time.Second); err != nil {
		t.Fatalf("expected healthy probe, got %v", err)
	}
}

func TestProbeHTTPStatus500(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	target := targetFromHTTPServer(t, server, "/health")
	if _, err := Probe(context.Background(), target, time.Second); err == nil {
		t.Fatalf("expected unhealthy probe")
	}
}

func TestProbeTCPRefused(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := listener.Addr().(*net.TCPAddr)
	_ = listener.Close()

	if _, err := Probe(context.Background(), Target{Host: "127.0.0.1", Port: addr.Port}, 100*time.Millisecond); err == nil {
		t.Fatalf("expected refused TCP probe")
	}
}

func TestProbeHTTPTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	target := targetFromHTTPServer(t, server, "/health")
	if _, err := Probe(context.Background(), target, 10*time.Millisecond); err == nil {
		t.Fatalf("expected timeout")
	}
}

func TestProbeHTTPAddsMissingLeadingSlash(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/products" {
			t.Fatalf("expected /api/products, got %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	target := targetFromHTTPServer(t, server, "api/products")
	if _, err := Probe(context.Background(), target, time.Second); err != nil {
		t.Fatalf("expected healthy probe, got %v", err)
	}
}

func targetFromHTTPServer(t *testing.T, server *httptest.Server, path string) Target {
	t.Helper()
	host, portRaw, err := net.SplitHostPort(server.Listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	port, err := strconv.Atoi(portRaw)
	if err != nil {
		t.Fatal(err)
	}
	return Target{Host: host, Port: port, HealthPath: path}
}
