// time.go — Timestamp parsing utilities for RFC3339 and RFC3339Nano formats.
package util

import "time"

// ParseTimestamp parses an RFC3339 timestamp string, trying RFC3339Nano first
// (since it's a superset of RFC3339), then RFC3339 as a fallback.
// Returns zero time on failure.
func ParseTimestamp(s string) time.Time {
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		t, _ = time.Parse(time.RFC3339, s)
	}
	return t
}
