// Purpose: Defines internal cache/store structs used by capture for a11y and performance sub-state.
// Why: Isolates non-exported supporting state to keep core capture structs focused and readable.
// Docs: docs/features/feature/backend-log-streaming/index.md

package capture

import (
	"github.com/dev-console/dev-console/internal/performance"
)

// A11yCache manages accessibility audit result cache with LRU eviction
// and concurrent deduplication of inflight requests.
type A11yCache struct {
	cache      map[string]*a11yCacheEntry
	cacheOrder []string // Track insertion order for eviction
	inflight   map[string]*a11yInflightEntry
}

type a11yCacheEntry struct{}

type a11yInflightEntry struct{}

// PerformanceStore manages performance snapshots and baselines with LRU eviction.
type PerformanceStore struct {
	snapshots       map[string]performance.PerformanceSnapshot
	snapshotOrder   []string
	baselines       map[string]performance.PerformanceBaseline
	baselineOrder   []string
	beforeSnapshots map[string]performance.PerformanceSnapshot // keyed by correlation_id, for perf_diff
}
