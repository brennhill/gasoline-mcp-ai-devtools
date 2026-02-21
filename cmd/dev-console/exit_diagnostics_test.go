package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/dev-console/dev-console/internal/state"
)

func TestWriteDiagnosticToCandidates_WritesFirstAvailable(t *testing.T) {
	t.Parallel()

	dir1 := t.TempDir()
	dir2 := t.TempDir()
	first := filepath.Join(dir1, "logs", "crash.log")
	second := filepath.Join(dir2, "logs", "crash.log")

	path, err := writeDiagnosticToCandidates(
		[]string{first, second},
		map[string]any{"event": "daemon_shutdown", "reason": "test"},
	)
	if err != nil {
		t.Fatalf("writeDiagnosticToCandidates error: %v", err)
	}
	if path != first {
		t.Fatalf("write path = %q, want %q", path, first)
	}

	data, err := os.ReadFile(first) // nosemgrep: go_filesystem_rule-fileread -- unit test reads temp file output
	if err != nil {
		t.Fatalf("read first candidate: %v", err)
	}
	if !strings.Contains(string(data), `"event":"daemon_shutdown"`) {
		t.Fatalf("expected event in diagnostic entry, got: %s", string(data))
	}
}

func TestWriteDiagnosticToCandidates_FallsBackOnInvalidPath(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	blocker := filepath.Join(base, "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
		t.Fatalf("write blocker: %v", err)
	}
	bad := filepath.Join(blocker, "logs", "crash.log")
	good := filepath.Join(base, "ok", "crash.log")

	path, err := writeDiagnosticToCandidates(
		[]string{bad, good},
		map[string]any{"event": "daemon_shutdown", "reason": "fallback"},
	)
	if err != nil {
		t.Fatalf("writeDiagnosticToCandidates error: %v", err)
	}
	if path != good {
		t.Fatalf("fallback path = %q, want %q", path, good)
	}
}

func TestAppendExitDiagnostic_UsesStateCrashPath(t *testing.T) {
	stateDir := t.TempDir()
	t.Setenv(state.StateDirEnv, stateDir)

	path := appendExitDiagnostic("daemon_shutdown", map[string]any{"reason": "unit_test"})
	if path == "" {
		t.Fatal("appendExitDiagnostic returned empty path")
	}
	want := filepath.Join(stateDir, "logs", "crash.log")
	if path != want {
		t.Fatalf("append path = %q, want %q", path, want)
	}

	data, err := os.ReadFile(path) // nosemgrep: go_filesystem_rule-fileread -- unit test reads temp file output
	if err != nil {
		t.Fatalf("read crash diagnostic: %v", err)
	}
	text := string(data)
	if !strings.Contains(text, `"event":"daemon_shutdown"`) {
		t.Fatalf("missing daemon_shutdown event: %s", text)
	}
	if !strings.Contains(text, `"reason":"unit_test"`) {
		t.Fatalf("missing reason field: %s", text)
	}
}

func TestBridgeShutdown_WritesBridgeExitDiagnostics(t *testing.T) {
	stateDir := t.TempDir()
	t.Setenv(state.StateDirEnv, stateDir)

	stats := &bridgeSessionStats{
		requests:             4,
		parseErrors:          1,
		invalidIDs:           1,
		fastPath:             2,
		forwarded:            2,
		methodNotFound:       1,
		starting:             1,
		lineFraming:          3,
		contentLengthFraming: 1,
		lastMethod:           "tools/call",
	}

	var wg sync.WaitGroup
	responseSent := make(chan bool, 1)
	responseSent <- true
	bridgeShutdown(&wg, nil, responseSent, stats)

	path, err := state.CrashLogFile()
	if err != nil {
		t.Fatalf("state.CrashLogFile error: %v", err)
	}
	data, err := os.ReadFile(path) // nosemgrep: go_filesystem_rule-fileread -- unit test reads temp file output
	if err != nil {
		t.Fatalf("read crash log: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) == 0 {
		t.Fatal("expected at least one crash log entry")
	}
	var last map[string]any
	if err := json.Unmarshal([]byte(lines[len(lines)-1]), &last); err != nil {
		t.Fatalf("parse crash log json: %v", err)
	}

	if last["event"] != "bridge_exit" {
		t.Fatalf("event = %v, want bridge_exit", last["event"])
	}
	if last["reason"] != "stdin_eof" {
		t.Fatalf("reason = %v, want stdin_eof", last["reason"])
	}
	if got, ok := last["requests"].(float64); !ok || int(got) != 4 {
		t.Fatalf("requests = %v, want 4", last["requests"])
	}
	if got, ok := last["forwarded"].(float64); !ok || int(got) != 2 {
		t.Fatalf("forwarded = %v, want 2", last["forwarded"])
	}
	if last["last_method"] != "tools/call" {
		t.Fatalf("last_method = %v, want tools/call", last["last_method"])
	}
}
