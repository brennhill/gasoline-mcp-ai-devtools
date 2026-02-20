// url.go — URL parsing utilities: path extraction and origin extraction.
package util

import (
	"net/url"
	"strings"
)

// ExtractURLPath extracts the path portion from a URL string, stripping query parameters.
// Returns "/" if the URL has no path component.
// Returns the input unchanged if it cannot be parsed.
func ExtractURLPath(rawURL string) string {
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

// ExtractOrigin extracts the origin (scheme://host[:port]) from a URL.
// Returns empty string for data: URLs, blob: URLs (after extracting nested origin),
// and malformed URLs.
func ExtractOrigin(rawURL string) string {
	// Handle data: URLs
	if strings.HasPrefix(rawURL, "data:") {
		return ""
	}

	// Handle blob: URLs - extract the nested origin
	// blob:https://example.com/uuid -> https://example.com
	rawURL = strings.TrimPrefix(rawURL, "blob:")

	// Parse URL
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}

	// URL must have a scheme and host
	if parsed.Scheme == "" || parsed.Host == "" {
		return ""
	}

	// Reconstruct origin: scheme://host[:port]
	return parsed.Scheme + "://" + parsed.Host
}
