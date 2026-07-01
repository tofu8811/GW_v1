package proxy

import (
	"errors"
	"net"
	"os"
	"strings"
	"syscall"
)

func isRetryable(method string, err error) bool {
	if err == nil {
		return false
	}

	if isDialError(err) {
		return true
	}

	return isIdempotent(method) && isNetworkError(err)
}

func isIdempotent(method string) bool {
	switch strings.ToUpper(method) {
	case "GET", "HEAD", "OPTIONS", "PUT", "DELETE":
		return true
	default:
		return false
	}
}

func isDialError(err error) bool {
	if errors.Is(err, syscall.ECONNREFUSED) || errors.Is(err, syscall.ECONNRESET) {
		return true
	}
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		return opErr.Op == "dial"
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "connection refused") || strings.Contains(msg, "dial")
}

func isNetworkError(err error) bool {
	if errors.Is(err, os.ErrDeadlineExceeded) || errors.Is(err, syscall.ETIMEDOUT) {
		return true
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "timeout") ||
		strings.Contains(msg, "connection refused") ||
		strings.Contains(msg, "connection reset") ||
		strings.Contains(msg, "no such host")
}
