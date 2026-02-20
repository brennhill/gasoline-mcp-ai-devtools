// Purpose: Owns helpers.go runtime behavior and integration logic.
// Docs: docs/features/feature/backend-log-streaming/index.md

// helpers.go — Shared utility functions used across the server.
// Currently provides URL path extraction (stripping query params and fragments)
// used by the network and performance subsystems for path-based grouping.
package capture

import (
	"io"
	"net/http"

	"github.com/dev-console/dev-console/internal/util"
)

// extractURLPath delegates to util.ExtractURLPath for URL path extraction.
func extractURLPath(rawURL string) string {
	return util.ExtractURLPath(rawURL)
}

// ExtractURLPath is the exported version for cross-package callers.
func ExtractURLPath(rawURL string) string {
	return util.ExtractURLPath(rawURL)
}

// reverseSlice reverses a slice in place.
func reverseSlice[T any](s []T) {
	for i, j := 0, len(s)-1; i < j; i, j = i+1, j-1 {
		s[i], s[j] = s[j], s[i]
	}
}

// removeFromSlice removes the first occurrence of item from a string slice,
// preserving the order of remaining elements. Allocates a new backing array to avoid GC pinning.
func removeFromSlice(slice []string, item string) []string {
	for i, v := range slice {
		if v == item {
			newSlice := make([]string, len(slice)-1)
			copy(newSlice, slice[:i])
			copy(newSlice[i:], slice[i+1:])
			return newSlice
		}
	}
	return slice
}

// readIngestBody handles rate-limit check and body reading for ingest endpoints.
// Returns the body bytes and true on success; on failure it writes the error response
// and returns nil, false.
func (c *Capture) readIngestBody(w http.ResponseWriter, r *http.Request) ([]byte, bool) {
	if c.CheckRateLimit() {
		c.WriteRateLimitResponse(w)
		return nil, false
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxExtensionPostBody)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Request body too large", http.StatusRequestEntityTooLarge)
		return nil, false
	}
	return body, true
}

// recordAndRecheck records a batch of events for rate limiting and rechecks.
// Returns true if the request may proceed; on rate limit it writes the 429 response.
func (c *Capture) recordAndRecheck(w http.ResponseWriter, count int) bool {
	c.RecordEvents(count)
	if c.CheckRateLimit() {
		c.WriteRateLimitResponse(w)
		return false
	}
	return true
}
