// Purpose: Implements TTL expiration checks for captured-entry filtering.
// Why: Ensures query results reflect current session context rather than stale buffered events.
// Docs: docs/features/feature/ttl-retention/index.md

package capture

import "time"

// isExpiredByTTL checks if an entry is expired based on TTL.
// Returns true if the entry should be filtered out.
func isExpiredByTTL(addedAt time.Time, ttl time.Duration) bool {
	if ttl == 0 {
		return false
	}
	return time.Since(addedAt) >= ttl
}
