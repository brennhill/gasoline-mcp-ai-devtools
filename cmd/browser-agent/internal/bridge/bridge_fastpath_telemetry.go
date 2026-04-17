// bridge_fastpath_telemetry.go -- Records bridge fast-path telemetry counters and diagnostics logs.
// Why: Isolates mutable counter state and file logging from fast-path request handlers.
// Docs: docs/features/feature/bridge-restart/index.md

package bridge

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	statecfg "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/state"
)

type bridgeFastPathResourceReadCounters struct {
	mu      sync.Mutex
	success int64
	failure int64
}

var fastPathResourceReadCounters bridgeFastPathResourceReadCounters

// ResetFastPathResourceReadCounters resets the resource read telemetry counters.
func ResetFastPathResourceReadCounters() {
	fastPathResourceReadCounters.mu.Lock()
	defer fastPathResourceReadCounters.mu.Unlock()
	fastPathResourceReadCounters.success = 0
	fastPathResourceReadCounters.failure = 0
}

// RecordFastPathResourceRead is an exported wrapper for external callers.
func RecordFastPathResourceRead(uri string, success bool, errorCode int) {
	recordFastPathResourceRead(uri, success, errorCode)
}

func recordFastPathResourceRead(uri string, success bool, errorCode int) {
	fastPathResourceReadCounters.mu.Lock()
	defer fastPathResourceReadCounters.mu.Unlock()
	if success {
		fastPathResourceReadCounters.success++
	} else {
		fastPathResourceReadCounters.failure++
	}
	appendFastPathResourceReadTelemetry(uri, success, errorCode, fastPathResourceReadCounters.success, fastPathResourceReadCounters.failure)
}

// SnapshotFastPathResourceReadCounters returns the current success/failure counts.
func SnapshotFastPathResourceReadCounters() (success int64, failure int64) {
	fastPathResourceReadCounters.mu.Lock()
	defer fastPathResourceReadCounters.mu.Unlock()
	return fastPathResourceReadCounters.success, fastPathResourceReadCounters.failure
}

// FastPathResourceReadLogPath returns the log path for resource read telemetry.
func FastPathResourceReadLogPath() (string, error) {
	return statecfg.InRoot("logs", "bridge-fastpath-resource-read.jsonl")
}

func appendFastPathResourceReadTelemetry(uri string, success bool, errorCode int, successCount int64, failureCount int64) {
	path, err := FastPathResourceReadLogPath()
	if err != nil {
		return
	}
	if mkErr := os.MkdirAll(filepath.Dir(path), 0o750); mkErr != nil {
		return
	}
	entry := map[string]any{
		"timestamp":      time.Now().UTC().Format(time.RFC3339Nano),
		"event":          "bridge_fastpath_resources_read",
		"uri":            uri,
		"success":        success,
		"error_code":     errorCode,
		"success_count":  successCount,
		"failure_count":  failureCount,
		"pid":            os.Getpid(),
		"bridge_version": deps.Version,
	}
	line, marshalErr := json.Marshal(entry)
	if marshalErr != nil {
		return
	}
	// #nosec G304 -- path is deterministic under state root
	f, openErr := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if openErr != nil {
		return
	}
	defer func() { _ = f.Close() }()
	_, _ = f.Write(line)
	_, _ = f.Write([]byte("\n"))
}

type bridgeFastPathCounters struct {
	mu      sync.Mutex
	success int
	failure int
}

var fastPathCounters bridgeFastPathCounters

// ResetFastPathCounters resets the fast-path event counters.
func ResetFastPathCounters() {
	fastPathCounters.mu.Lock()
	defer fastPathCounters.mu.Unlock()
	fastPathCounters.success = 0
	fastPathCounters.failure = 0
}

// FastPathTelemetryLogPath returns the log path for fast-path event telemetry.
func FastPathTelemetryLogPath() (string, error) {
	return statecfg.InRoot("logs", "bridge-fastpath-events.jsonl")
}

// RecordFastPathEvent is an exported wrapper for external callers.
func RecordFastPathEvent(method string, success bool, errorCode int) {
	recordFastPathEvent(method, success, errorCode)
}

func recordFastPathEvent(method string, success bool, errorCode int) {
	successCount, failureCount := func() (int, int) {
		fastPathCounters.mu.Lock()
		defer fastPathCounters.mu.Unlock()
		if success {
			fastPathCounters.success++
		} else {
			fastPathCounters.failure++
		}
		return fastPathCounters.success, fastPathCounters.failure
	}()

	path, err := FastPathTelemetryLogPath()
	if err != nil {
		return
	}
	// #nosec G301 -- runtime state directory for local diagnostics.
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return
	}
	event := map[string]any{
		"timestamp":     time.Now().UTC().Format(time.RFC3339Nano),
		"event":         "bridge_fastpath_method",
		"method":        method,
		"success":       success,
		"error_code":    errorCode,
		"success_count": successCount,
		"failure_count": failureCount,
		"pid":           os.Getpid(),
		"version":       deps.Version,
	}
	payload, err := json.Marshal(event)
	if err != nil {
		return
	}
	// #nosec G304 -- deterministic diagnostics path rooted in runtime state directory.
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600) // nosemgrep: go_filesystem_rule-fileread -- local diagnostics log append
	if err != nil {
		return
	}
	defer func() { _ = f.Close() }()
	_, _ = f.Write(append(payload, '\n'))
}
