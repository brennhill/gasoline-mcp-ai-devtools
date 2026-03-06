// media.go — Shared media file helpers (filename sanitization, data URL decoding).
package util

import (
	"encoding/base64"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"
)

var unsafeFilenameChars = regexp.MustCompile(`[^a-zA-Z0-9._-]`)

// SanitizeForFilename replaces unsafe characters with underscores and truncates to 50 chars.
func SanitizeForFilename(s string) string {
	s = unsafeFilenameChars.ReplaceAllString(s, "_")
	if len(s) > 50 {
		s = s[:50]
	}
	return s
}

// DecodeDataURL extracts and base64-decodes the payload from a data URL.
func DecodeDataURL(dataURL string) ([]byte, error) {
	if dataURL == "" {
		return nil, errors.New("missing dataUrl")
	}
	parts := strings.SplitN(dataURL, ",", 2)
	if len(parts) != 2 {
		return nil, errors.New("invalid dataUrl format")
	}
	return base64.StdEncoding.DecodeString(parts[1])
}

// BuildScreenshotFilename creates a timestamped filename from a page URL and optional correlation ID.
func BuildScreenshotFilename(pageURL, correlationID string) string {
	hostname := "unknown"
	if pageURL != "" {
		if u, err := url.Parse(pageURL); err == nil && u.Host != "" {
			hostname = u.Host
		}
	}
	ts := time.Now().Format("20060102-150405")
	if correlationID != "" {
		return fmt.Sprintf("%s-%s-%s.jpg", SanitizeForFilename(hostname), ts, SanitizeForFilename(correlationID))
	}
	return fmt.Sprintf("%s-%s.jpg", SanitizeForFilename(hostname), ts)
}
