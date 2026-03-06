// Purpose: Async command helpers for correlation IDs and lifecycle status normalization.

package main

import (
	"fmt"
	"strings"
	"time"
)

// newCorrelationID generates a unique correlation ID with the given prefix.
// Format: prefix_timestamp_random (e.g., "nav_1708300000000000000_4821937562").
func newCorrelationID(prefix string) string {
	return fmt.Sprintf("%s_%d_%d", prefix, time.Now().UnixNano(), randomInt63())
}

func canonicalLifecycleStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "queued":
		return "queued"
	case "pending", "running", "still_processing":
		return "running"
	case "complete":
		return "complete"
	case "error":
		return "error"
	case "timeout", "expired":
		return "timeout"
	case "cancelled", "canceled":
		return "cancelled"
	default:
		return status
	}
}

var (
	asyncInitialWait  = 15 * time.Second
	asyncRetryWait    = 5 * time.Second
	asyncPollInterval = 500 * time.Millisecond
)
