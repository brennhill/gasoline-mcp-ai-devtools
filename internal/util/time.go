// Purpose: Provides shared timestamp parsing helpers for RFC3339/RFC3339Nano inputs.
// Why: Centralizes tolerant timestamp parsing behavior used across telemetry ingestion paths.
// Docs: docs/features/feature/backend-log-streaming/index.md

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
