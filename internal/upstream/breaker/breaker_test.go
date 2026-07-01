package breaker

import (
	"testing"
	"time"
)

func TestBreakerOpensAfterConsecutiveFailures(t *testing.T) {
	br := New(Config{FailureThreshold: 2, OpenTimeout: time.Minute, HalfOpenMax: 1})

	br.OnFailure()
	if br.State() != Closed {
		t.Fatalf("expected closed after first failure")
	}
	br.OnFailure()
	if br.State() != Open {
		t.Fatalf("expected open after threshold")
	}
	if br.Allow() {
		t.Fatalf("expected open breaker to reject")
	}
}

func TestBreakerHalfOpenThenClosesOnSuccess(t *testing.T) {
	br := New(Config{FailureThreshold: 1, OpenTimeout: 10 * time.Millisecond, HalfOpenMax: 1})
	br.OnFailure()

	time.Sleep(20 * time.Millisecond)
	if br.State() != HalfOpen {
		t.Fatalf("expected half-open after open timeout")
	}
	if !br.Allow() {
		t.Fatalf("expected half-open trial to be allowed")
	}

	br.OnSuccess()
	if br.State() != Closed {
		t.Fatalf("expected closed after half-open success")
	}
}
