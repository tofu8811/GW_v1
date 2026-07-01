package ratelimit

import "time"

const LimitTypeIP = "ip"

type Policy struct {
	ID            string
	LimitType     string
	MaxRequests   int
	WindowSeconds int
}

type Request struct {
	Policy     Policy
	Identifier string
	Now        time.Time
}

type Result struct {
	Allowed    bool
	Limit      int
	Remaining  int64
	ResetAt    int64
	RetryAfter int64
	Key        string
	Count      int64
}
