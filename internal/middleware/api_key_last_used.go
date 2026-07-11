package middleware

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

const defaultAPIKeyLastUsedFlushInterval = 30 * time.Second

type APIKeyLastUsedBatcher struct {
	db       *pgxpool.Pool
	logger   *slog.Logger
	interval time.Duration

	mu      sync.Mutex
	pending map[string]struct{}
}

func NewAPIKeyLastUsedBatcher(db *pgxpool.Pool, logger *slog.Logger, interval time.Duration) *APIKeyLastUsedBatcher {
	if interval <= 0 {
		interval = defaultAPIKeyLastUsedFlushInterval
	}
	return &APIKeyLastUsedBatcher{
		db:       db,
		logger:   logger,
		interval: interval,
		pending:  map[string]struct{}{},
	}
}

func (b *APIKeyLastUsedBatcher) MarkUsed(apiKeyID string) {
	if b == nil || apiKeyID == "" {
		return
	}

	b.mu.Lock()
	b.pending[apiKeyID] = struct{}{}
	b.mu.Unlock()
}

func (b *APIKeyLastUsedBatcher) Start(ctx context.Context) {
	if b == nil || b.db == nil {
		return
	}

	ticker := time.NewTicker(b.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			b.Flush(context.Background())
			return
		case <-ticker.C:
			b.Flush(ctx)
		}
	}
}

func (b *APIKeyLastUsedBatcher) Flush(ctx context.Context) {
	ids := b.drain()
	if len(ids) == 0 {
		return
	}

	_, err := b.db.Exec(ctx, `UPDATE api_keys SET last_used_at = now() WHERE id = ANY($1::uuid[])`, ids)
	if err == nil {
		return
	}

	if b.logger != nil {
		b.logger.Warn("failed to flush api key last_used_at batch", "count", len(ids), "error", err)
	}
	b.requeue(ids)
}

func (b *APIKeyLastUsedBatcher) drain() []string {
	b.mu.Lock()
	defer b.mu.Unlock()

	if len(b.pending) == 0 {
		return nil
	}

	ids := make([]string, 0, len(b.pending))
	for id := range b.pending {
		ids = append(ids, id)
	}
	b.pending = map[string]struct{}{}
	return ids
}

func (b *APIKeyLastUsedBatcher) requeue(ids []string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	for _, id := range ids {
		b.pending[id] = struct{}{}
	}
}
