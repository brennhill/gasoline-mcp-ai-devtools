// Purpose: Logging and file-path helpers for stop/force-cleanup command lifecycle events.
// Why: Keeps command orchestration functions concise and focused on shutdown flow.

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/state"
)

// logCommandInvocation writes a lifecycle log entry for a stop or cleanup command.
func logCommandInvocation(event string, source string, port int) {
	logFile := resolveLogFile()
	entry := map[string]any{
		"type":       "lifecycle",
		"event":      event,
		"port":       port,
		"source":     source,
		"caller_pid": os.Getpid(),
		"timestamp":  time.Now().UTC().Format(time.RFC3339),
	}
	writeJSONLogEntry(logFile, entry)
}

// resolveLogFile determines the log file path with fallbacks.
func resolveLogFile() string {
	logFile, err := state.DefaultLogFile()
	if err != nil {
		if legacy, legacyErr := state.LegacyDefaultLogFile(); legacyErr == nil {
			return legacy
		}
		return filepath.Join(os.TempDir(), "kaboom.jsonl")
	}
	return logFile
}

// writeJSONLogEntry marshals and appends a JSON entry to the given log file.
func writeJSONLogEntry(logFile string, entry map[string]any) {
	data, err := json.Marshal(entry)
	if err != nil {
		return
	}
	// #nosec G301 -- runtime state directory: owner rwx, group rx for diagnostics
	_ = os.MkdirAll(filepath.Dir(logFile), 0o750)
	// #nosec G304 -- log file path resolved from trusted runtime state directory
	f, err := os.OpenFile(logFile, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0600) // nosemgrep: go_filesystem_rule-fileread
	if err != nil {
		return
	}
	_, _ = f.Write(data)
	_, _ = f.Write([]byte{'\n'})
	_ = f.Close()
}
