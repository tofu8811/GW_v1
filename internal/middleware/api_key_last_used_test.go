package middleware

import (
	"sync"
	"testing"
)

func TestAPIKeyLastUsedBatcherMarkUsedIsConcurrentSafe(t *testing.T) {
	batcher := NewAPIKeyLastUsedBatcher(nil, nil, 0)

	const goroutines = 32
	const iterations = 100

	var wg sync.WaitGroup
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				batcher.MarkUsed("00000000-0000-0000-0000-000000000001")
				batcher.MarkUsed("00000000-0000-0000-0000-000000000002")
			}
		}()
	}
	wg.Wait()

	ids := batcher.drain()
	if len(ids) != 2 {
		t.Fatalf("expected 2 deduplicated ids, got %d: %#v", len(ids), ids)
	}
}
