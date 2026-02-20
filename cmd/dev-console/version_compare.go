// version_compare.go — Semantic version comparison for binary upgrade detection.
package main

import (
	"strconv"
	"strings"
)

// parseVersionParts splits a version string like "0.7.5" or "v0.7.5" into integer parts.
// Returns nil if the version string is empty or contains no valid numeric parts.
func parseVersionParts(v string) []int {
	v = strings.TrimPrefix(v, "v")
	if v == "" {
		return nil
	}
	segments := strings.Split(v, ".")
	parts := make([]int, 0, len(segments))
	for _, seg := range segments {
		n, err := strconv.Atoi(seg)
		if err != nil {
			break
		}
		parts = append(parts, n)
	}
	if len(parts) == 0 {
		return nil
	}
	return parts
}

// isNewerVersion returns true if candidate is strictly newer than current.
// Both strings are parsed as semver (with optional "v" prefix).
// Returns false for equal versions, malformed input, or empty strings.
func isNewerVersion(candidate, current string) bool {
	cParts := parseVersionParts(candidate)
	rParts := parseVersionParts(current)
	if cParts == nil || rParts == nil {
		return false
	}

	// Compare element-by-element, zero-padding the shorter slice.
	maxLen := len(cParts)
	if len(rParts) > maxLen {
		maxLen = len(rParts)
	}
	for i := 0; i < maxLen; i++ {
		c, r := 0, 0
		if i < len(cParts) {
			c = cParts[i]
		}
		if i < len(rParts) {
			r = rParts[i]
		}
		if c > r {
			return true
		}
		if c < r {
			return false
		}
	}
	return false // equal
}
