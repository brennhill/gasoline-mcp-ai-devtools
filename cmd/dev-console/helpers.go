// helpers.go — Shared utility functions used across the server.
// Currently provides URL path extraction (stripping query params and fragments)
// used by the network and performance subsystems for path-based grouping.
package main

import (
	"io"
	"net/http"
	"net/url"
)

// extractURLPath extracts the path portion from a URL string, stripping query parameters.
// Returns "/" if the URL has no path component.
// Returns the input unchanged if it cannot be parsed.
func extractURLPath(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	path := parsed.Path
	if path == "" {
		return "/"
	}
	return path
}

// ExtractURLPath is the exported version of extractURLPath for use in tests.
func ExtractURLPath(rawURL string) string {
	return extractURLPath(rawURL)
}

// reverseSlice reverses a slice in place.
func reverseSlice[T any](s []T) {
	for i, j := 0, len(s)-1; i < j; i, j = i+1, j-1 {
		s[i], s[j] = s[j], s[i]
	}
}

// removeFromSlice removes the first occurrence of item from a string slice,
// preserving the order of remaining elements. Operates in-place without allocation.
func removeFromSlice(slice []string, item string) []string {
	for i, v := range slice {
		if v == item {
			copy(slice[i:], slice[i+1:])
			return slice[:len(slice)-1]
		}
	}
	return slice
}

// readIngestBody handles rate-limit check and body reading for ingest endpoints.
// Returns the body bytes and true on success; on failure it writes the error response
// and returns nil, false.
func (v *Capture) readIngestBody(w http.ResponseWriter, r *http.Request) ([]byte, bool) {
	if v.CheckRateLimit() {
		v.WriteRateLimitResponse(w)
		return nil, false
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxPostBodySize)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Request body too large", http.StatusRequestEntityTooLarge)
		return nil, false
	}
	return body, true
}

// recordAndRecheck records a batch of events for rate limiting and rechecks.
// Returns true if the request may proceed; on rate limit it writes the 429 response.
func (v *Capture) recordAndRecheck(w http.ResponseWriter, count int) bool {
	v.RecordEvents(count)
	if v.CheckRateLimit() {
		v.WriteRateLimitResponse(w)
		return false
	}
	return true
}
