// ttl.go — TTL filtering utilities
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
