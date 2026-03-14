// Purpose: Log-level ranking and string matching helpers for observe filters.
// Docs: docs/features/feature/observe/index.md

package observe

import "strings"

// LogLevelRank returns the severity rank of a log level (higher = more severe).
func LogLevelRank(level string) int {
	switch level {
	case "debug":
		return 0
	case "log":
		return 1
	case "info":
		return 2
	case "warn":
		return 3
	case "error":
		return 4
	default:
		return -1
	}
}

// ContainsIgnoreCase reports whether s contains substr (case-insensitive).
func ContainsIgnoreCase(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}
