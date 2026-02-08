// internal-types.go — Internal types used by Capture struct.
// A11yCache, PerformanceStore, and related internal helper types.
package capture

import (
	"encoding/json"
	"time"

	"github.com/dev-console/dev-console/internal/performance"
)

// A11yCache manages accessibility audit result cache with LRU eviction
// and concurrent deduplication of inflight requests.
type A11yCache struct {
	cache      map[string]*a11yCacheEntry
	cacheOrder []string // Track insertion order for eviction
	lastURL    string
	inflight   map[string]*a11yInflightEntry
}

const maxA11yCacheEntries = 10
const a11yCacheTTL = 30 * time.Second

type a11yCacheEntry struct {
	result    json.RawMessage
	createdAt time.Time
	url       string
}

type a11yInflightEntry struct {
	done   chan struct{}
	result json.RawMessage
	err    error
}

// PerformanceStore manages performance snapshots and baselines with LRU eviction.
type PerformanceStore struct {
	snapshots       map[string]performance.PerformanceSnapshot
	snapshotOrder   []string
	baselines       map[string]performance.PerformanceBaseline
	baselineOrder   []string
	beforeSnapshots map[string]performance.PerformanceSnapshot // keyed by correlation_id, for perf_diff
}
