// ttl.go — TTL (time-to-live) duration parsing and validation.
// Supports Go duration strings with a minimum threshold of 1 minute.
// Empty string means unlimited retention.
package main

import (
	"fmt"
	"time"
)

// minTTL is the minimum allowed TTL value (1 minute).
const minTTL = time.Minute

// ParseTTL parses a TTL duration string and validates it.
// An empty string means unlimited (returns 0, nil).
// Values below 1 minute are rejected with an error.
func ParseTTL(s string) (time.Duration, error) {
	if s == "" {
		return 0, nil
	}

	d, err := time.ParseDuration(s)
	if err != nil {
		return 0, fmt.Errorf("invalid TTL duration %q: %w", s, err)
	}

	if d < minTTL {
		return 0, fmt.Errorf("TTL %v is below minimum (%v)", d, minTTL)
	}

	return d, nil
}

// SetTTL sets the TTL on a Capture instance.
// TTL=0 means unlimited (no filtering).
func (c *Capture) SetTTL(ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.TTL = ttl
}

// SetTTL sets the TTL on a Server instance.
// TTL=0 means unlimited (no filtering).
func (s *Server) SetTTL(ttl time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.TTL = ttl
}

// getEntriesWithTTL returns log entries filtered by the server's TTL.
// If TTL is 0 (unlimited), all entries are returned.
func (s *Server) getEntriesWithTTL() []LogEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.TTL == 0 {
		// Unlimited - return a copy of all entries
		result := make([]LogEntry, len(s.entries))
		copy(result, s.entries)
		return result
	}

	cutoff := time.Now().Add(-s.TTL)
	var result []LogEntry
	for i, entry := range s.entries {
		if i < len(s.logAddedAt) && !s.logAddedAt[i].Before(cutoff) {
			result = append(result, entry)
		}
	}
	return result
}

// isExpiredByTTL checks if an entry at the given index is expired based on TTL.
// Returns true if the entry should be filtered out.
func isExpiredByTTL(addedAt time.Time, ttl time.Duration) bool {
	if ttl == 0 {
		return false
	}
	return time.Since(addedAt) >= ttl
}
