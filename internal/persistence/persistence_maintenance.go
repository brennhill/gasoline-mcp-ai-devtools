// Purpose: Calculates project storage size and enforces size-based eviction of oldest namespaces.
// Why: Separates storage maintenance and eviction from CRUD and initialization.
package persistence

import (
	"os"
	"path/filepath"
	"slices"
	"time"
)

func (s *SessionStore) projectSize() (int64, error) {
	var total int64
	err := filepath.Walk(s.projectDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil //nolint:nilerr // intentionally skip errors to continue walking
		}
		if !info.IsDir() {
			total += info.Size()
		}
		return nil
	})
	return total, err
}

func enforceErrorHistoryCap(entries []ErrorHistoryEntry) []ErrorHistoryEntry {
	if len(entries) <= maxErrorHistory {
		return entries
	}

	slices.SortFunc(entries, func(a, b ErrorHistoryEntry) int {
		if a.FirstSeen.Before(b.FirstSeen) {
			return -1
		}
		if a.FirstSeen.After(b.FirstSeen) {
			return 1
		}
		return 0
	})

	return entries[len(entries)-maxErrorHistory:]
}

func evictStaleErrors(entries []ErrorHistoryEntry, threshold time.Duration) []ErrorHistoryEntry {
	if len(entries) == 0 {
		return entries
	}

	cutoff := time.Now().Add(-threshold)
	filtered := make([]ErrorHistoryEntry, 0, len(entries))
	for _, e := range entries {
		if e.LastSeen.IsZero() || e.LastSeen.After(cutoff) {
			filtered = append(filtered, e)
		}
	}
	return filtered
}
