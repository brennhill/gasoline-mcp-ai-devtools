// Purpose: Connection retry and daemon-version compatibility checks for bridge startup.
// Why: Keeps retry/version policy separate from high-level connection orchestration.

package main

import (
	"strings"
)

func normalizeVersionString(v string) string {
	return strings.TrimPrefix(strings.TrimSpace(v), "v")
}

func versionsMatch(a string, b string) bool {
	return normalizeVersionString(a) == normalizeVersionString(b)
}
