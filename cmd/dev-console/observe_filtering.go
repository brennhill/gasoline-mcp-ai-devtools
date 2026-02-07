// observe_filtering.go — Filtering helpers for observe operations.
// Provides log level ranking and other filtering utilities.
package main

// logLevelRank returns the severity rank of a log level (higher = more severe).
func logLevelRank(level string) int {
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
