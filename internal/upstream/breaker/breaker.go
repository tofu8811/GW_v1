package breaker

import (
	"sync"
	"time"
)

type State int

const (
	Closed State = iota
	Open
	HalfOpen
)

type Config struct {
	FailureThreshold int
	OpenTimeout      time.Duration
	HalfOpenMax      int
}

func DefaultConfig() Config {
	return Config{
		FailureThreshold: 5,
		OpenTimeout:      15 * time.Second,
		HalfOpenMax:      1,
	}
}

type Breaker struct {
	mu                  sync.Mutex
	state               State
	consecutiveFailures int
	openedAt            time.Time
	halfOpenInFlight    int
	cfg                 Config
}

func New(cfg Config) *Breaker {
	cfg = normalizeConfig(cfg)
	return &Breaker{state: Closed, cfg: cfg}
}

func (b *Breaker) Allow() bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.state == Open {
		if time.Since(b.openedAt) < b.cfg.OpenTimeout {
			return false
		}
		b.state = HalfOpen
		b.halfOpenInFlight = 0
	}

	if b.state == HalfOpen {
		if b.halfOpenInFlight >= b.cfg.HalfOpenMax {
			return false
		}
		b.halfOpenInFlight++
	}

	return true
}

func (b *Breaker) OnSuccess() {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.state = Closed
	b.consecutiveFailures = 0
	b.halfOpenInFlight = 0
}

func (b *Breaker) OnFailure() {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.state == HalfOpen {
		b.openLocked()
		return
	}

	b.consecutiveFailures++
	if b.consecutiveFailures >= b.cfg.FailureThreshold {
		b.openLocked()
	}
}

func (b *Breaker) State() State {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.state == Open && time.Since(b.openedAt) >= b.cfg.OpenTimeout {
		return HalfOpen
	}
	return b.state
}

func (b *Breaker) openLocked() {
	b.state = Open
	b.openedAt = time.Now()
	b.halfOpenInFlight = 0
}

func normalizeConfig(cfg Config) Config {
	if cfg.FailureThreshold <= 0 {
		cfg.FailureThreshold = 5
	}
	if cfg.OpenTimeout <= 0 {
		cfg.OpenTimeout = 15 * time.Second
	}
	if cfg.HalfOpenMax <= 0 {
		cfg.HalfOpenMax = 1
	}
	return cfg
}
