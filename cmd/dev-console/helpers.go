// helpers.go — Shared utility functions used across the server.
// Currently provides URL path extraction (stripping query params and fragments)
// used by the network and performance subsystems for path-based grouping.
package main

import (
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

// removeFromSlice removes the first occurrence of item from a string slice,
// preserving the order of remaining elements.
func removeFromSlice(slice []string, item string) []string {
	for i, v := range slice {
		if v == item {
			return append(slice[:i], slice[i+1:]...)
		}
	}
	return slice
}
