package health

import (
	"context"
	"log/slog"
	"sync"
	"time"

	configcache "gateway-api/internal/config/cache"
	"gateway-api/internal/upstream/breaker"
)

type Config struct {
	Interval           time.Duration
	ProbeTimeout       time.Duration
	UnhealthyThreshold int
	HealthyThreshold   int
}

func DefaultConfig() Config {
	return Config{
		Interval:           10 * time.Second,
		ProbeTimeout:       2 * time.Second,
		UnhealthyThreshold: 3,
		HealthyThreshold:   2,
	}
}

type ConfigCache interface {
	ActiveInstances() []configcache.ActiveInstanceValue
	FindActiveInstance(instanceID string) (configcache.ActiveInstanceValue, bool)
}

type Checker struct {
	store    *Store
	cache    ConfigCache
	breakers *breaker.Registry
	cfg      Config
	logger   *slog.Logger

	mu       sync.Mutex
	counters map[string]probeCounter
}

type probeCounter struct {
	successes int
	failures  int
}

func NewChecker(store *Store, cache ConfigCache, breakers *breaker.Registry, cfg Config, logger *slog.Logger) *Checker {
	cfg = normalizeCheckerConfig(cfg)
	if logger == nil {
		logger = slog.Default()
	}
	return &Checker{
		store:    store,
		cache:    cache,
		breakers: breakers,
		cfg:      cfg,
		logger:   logger,
		counters: map[string]probeCounter{},
	}
}

func (c *Checker) Start(ctx context.Context) {
	c.runOnce(ctx)

	ticker := time.NewTicker(c.cfg.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.runOnce(ctx)
		}
	}
}

func (c *Checker) CheckInstance(ctx context.Context, instanceID string) (InstanceHealth, error) {
	instance, ok := c.cache.FindActiveInstance(instanceID)
	if !ok {
		return InstanceHealth{InstanceID: instanceID, Status: StatusUnknown}, ErrInstanceNotFound
	}
	return c.check(ctx, instance)
}

func (c *Checker) runOnce(ctx context.Context) {
	instances := c.cache.ActiveInstances()
	c.prune(instances)

	const maxConcurrent = 16
	sem := make(chan struct{}, maxConcurrent)
	var wg sync.WaitGroup

	for _, instance := range instances {
		instance := instance
		wg.Add(1)
		go func() {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				return
			}

			if _, err := c.check(ctx, instance); err != nil && err != context.Canceled {
				c.logger.Warn("upstream health check failed", "instance_id", instance.InstanceID, "service_id", instance.ServiceID, "error", err)
			}
		}()
	}

	wg.Wait()
}

func (c *Checker) check(ctx context.Context, instance configcache.ActiveInstanceValue) (InstanceHealth, error) {
	latencyMS, err := Probe(ctx, Target{
		Host:       instance.Host,
		Port:       instance.Port,
		HealthPath: instance.HealthPath,
	}, c.cfg.ProbeTimeout)
	status, failCount := c.recordResult(instance.InstanceID, err == nil)
	ih := InstanceHealth{
		InstanceID: instance.InstanceID,
		ServiceID:  instance.ServiceID,
		Status:     status,
		LatencyMS:  latencyMS,
		FailCount:  failCount,
		LastCheck:  time.Now().UTC(),
	}

	if err := c.store.SetInstanceHealth(ctx, ih); err != nil {
		return ih, err
	}
	if status == StatusAlive {
		if err := c.store.MarkAlive(ctx, instance.ServiceID, instance.InstanceID); err != nil {
			return ih, err
		}
	}
	if status == StatusDown {
		if err := c.store.MarkDown(ctx, instance.ServiceID, instance.InstanceID); err != nil {
			return ih, err
		}
	}

	return ih, err
}

func (c *Checker) recordResult(instanceID string, healthy bool) (Status, int) {
	c.mu.Lock()
	defer c.mu.Unlock()

	counter := c.counters[instanceID]
	if healthy {
		counter.successes++
		counter.failures = 0
		c.counters[instanceID] = counter
		if counter.successes >= c.cfg.HealthyThreshold {
			return StatusAlive, 0
		}
		return StatusUnknown, 0
	}

	counter.failures++
	counter.successes = 0
	c.counters[instanceID] = counter
	if counter.failures >= c.cfg.UnhealthyThreshold {
		return StatusDown, counter.failures
	}
	return StatusUnknown, counter.failures
}

func (c *Checker) prune(instances []configcache.ActiveInstanceValue) {
	keep := map[string]struct{}{}
	for _, instance := range instances {
		keep[instance.InstanceID] = struct{}{}
	}

	c.mu.Lock()
	for instanceID := range c.counters {
		if _, ok := keep[instanceID]; !ok {
			delete(c.counters, instanceID)
		}
	}
	c.mu.Unlock()

	if c.breakers != nil {
		c.breakers.Prune(keep)
	}
}

func normalizeCheckerConfig(cfg Config) Config {
	if cfg.Interval <= 0 {
		cfg.Interval = 10 * time.Second
	}
	if cfg.ProbeTimeout <= 0 {
		cfg.ProbeTimeout = 2 * time.Second
	}
	if cfg.UnhealthyThreshold <= 0 {
		cfg.UnhealthyThreshold = 3
	}
	if cfg.HealthyThreshold <= 0 {
		cfg.HealthyThreshold = 2
	}
	return cfg
}
