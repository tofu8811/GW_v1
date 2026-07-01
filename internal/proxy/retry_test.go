package proxy

import (
	"errors"
	"net"
	"syscall"
	"testing"
)

func TestIsRetryableAllowsDialErrorsForAnyMethod(t *testing.T) {
	err := &net.OpError{Op: "dial", Err: syscall.ECONNREFUSED}
	for _, method := range []string{"GET", "POST", "PATCH"} {
		if !isRetryable(method, err) {
			t.Fatalf("expected %s dial error to be retryable", method)
		}
	}
}

func TestIsRetryableAllowsNetworkErrorsForIdempotentMethods(t *testing.T) {
	err := errors.New("upstream timeout")
	for _, method := range []string{"GET", "HEAD", "OPTIONS", "PUT", "DELETE"} {
		if !isRetryable(method, err) {
			t.Fatalf("expected %s timeout to be retryable", method)
		}
	}
}

func TestIsRetryableRejectsAmbiguousNonIdempotentErrors(t *testing.T) {
	err := errors.New("upstream timeout after write")
	for _, method := range []string{"POST", "PATCH"} {
		if isRetryable(method, err) {
			t.Fatalf("expected %s ambiguous timeout to be non-retryable", method)
		}
	}
}
