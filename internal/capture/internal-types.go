// internal-types.go — Internal types used by Capture struct.
// A11yCache, PerformanceStore, and related internal helper types.
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
