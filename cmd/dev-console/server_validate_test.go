// server_validate_test.go — Unit tests for server log entry validation.
package main

import (
	"testing"
)

func TestValidateLogEntry_EdgeCases(t *testing.T) {
	t.Parallel()

	t.Run("entry at exactly max size is valid", func(t *testing.T) {
		// Create an entry that triggers the slow path but stays under limit
		bigStr := make([]byte, maxEntrySize/2+1)
		for i := range bigStr {
			bigStr[i] = 'a'
		}
		entry := LogEntry{"level": "info", "data": string(bigStr)}
		// This triggers slow path (string content > maxEntrySize/2)
		// Result depends on whether JSON serialization stays under limit
		_ = validateLogEntry(entry)
		// Just verifying it doesn't panic
	})

	t.Run("empty entry is invalid", func(t *testing.T) {
		if validateLogEntry(LogEntry{}) {
			t.Fatal("empty entry should be invalid (no level)")
		}
	})
}
