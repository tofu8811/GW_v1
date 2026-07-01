package health

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"
)

type Target struct {
	Host       string
	Port       int
	HealthPath string
}

func Probe(ctx context.Context, target Target, timeout time.Duration) (float64, error) {
	if timeout <= 0 {
		timeout = 2 * time.Second
	}

	target.HealthPath = strings.TrimSpace(target.HealthPath)
	startedAt := time.Now()
	if target.HealthPath == "" {
		err := probeTCP(ctx, target, timeout)
		return float64(time.Since(startedAt).Microseconds()) / 1000, err
	}

	err := probeHTTP(ctx, target, timeout)
	return float64(time.Since(startedAt).Microseconds()) / 1000, err
}

func probeTCP(ctx context.Context, target Target, timeout time.Duration) error {
	dialer := net.Dialer{Timeout: timeout}
	conn, err := dialer.DialContext(ctx, "tcp", address(target))
	if err != nil {
		return err
	}
	return conn.Close()
}

func probeHTTP(ctx context.Context, target Target, timeout time.Duration) error {
	client := http.Client{Timeout: timeout}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, healthURL(target), nil)
	if err != nil {
		return err
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusBadRequest {
		return fmt.Errorf("health endpoint returned status %d", resp.StatusCode)
	}
	return nil
}

func healthURL(target Target) string {
	path := strings.TrimSpace(target.HealthPath)
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return fmt.Sprintf("http://%s%s", address(target), path)
}

func address(target Target) string {
	return net.JoinHostPort(target.Host, fmt.Sprintf("%d", target.Port))
}
