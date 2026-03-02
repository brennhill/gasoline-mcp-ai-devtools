// Purpose: Records bridge fast-path telemetry counters and diagnostics logs.
// Why: Isolates mutable counter state and file logging from fast-path request handlers.
// Docs: docs/features/feature/bridge-restart/index.md

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	statecfg "github.com/dev-console/dev-console/internal/state"
)

type bridgeFastPathResourceReadCounters struct {
	mu      sync.Mutex
	success int64
	failure int64
}

var fastPathResourceReadCounters bridgeFastPathResourceReadCounters

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

func snapshotFastPathResourceReadCounters() (success int64, failure int64) {
	fastPathResourceReadCounters.mu.Lock()
	defer fastPathResourceReadCounters.mu.Unlock()
	return fastPathResourceReadCounters.success, fastPathResourceReadCounters.failure
}

func resetFastPathResourceReadCounters() {
	fastPathResourceReadCounters.mu.Lock()
	defer fastPathResourceReadCounters.mu.Unlock()
	fastPathResourceReadCounters.success = 0
	fastPathResourceReadCounters.failure = 0
}

func fastPathResourceReadLogPath() (string, error) {
	return statecfg.InRoot("logs", "bridge-fastpath-resource-read.jsonl")
}

func appendFastPathResourceReadTelemetry(uri string, success bool, errorCode int, successCount int64, failureCount int64) {
	path, err := fastPathResourceReadLogPath()
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
		"bridge_version": version,
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

func resetFastPathCounters() {
	fastPathCounters.mu.Lock()
	defer fastPathCounters.mu.Unlock()
	fastPathCounters.success = 0
	fastPathCounters.failure = 0
}

func fastPathTelemetryLogPath() (string, error) {
	return statecfg.InRoot("logs", "bridge-fastpath-events.jsonl")
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

	path, err := fastPathTelemetryLogPath()
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
		"version":       version,
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
