package main

import (
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/dev-console/dev-console/internal/state"
)

func TestFindMCPConfigResolution(t *testing.T) {
	// Do not run in parallel; uses Setenv.
	t.Setenv("HOME", t.TempDir())
	t.Setenv("USERPROFILE", os.Getenv("HOME"))

	home := os.Getenv("HOME")
	continuePath := filepath.Join(home, ".continue", "config.json")
	if err := os.MkdirAll(filepath.Dir(continuePath), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", filepath.Dir(continuePath), err)
	}
	if err := os.WriteFile(continuePath, []byte(`{"tool":"gasoline-mcp"}`), 0o600); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", continuePath, err)
	}
	if got := findMCPConfig(); got != continuePath {
		t.Fatalf("findMCPConfig() = %q, want %q", got, continuePath)
	}
}

func TestFindMCPConfigResolutionClaudePath(t *testing.T) {
	// Do not run in parallel; uses Setenv.
	t.Setenv("HOME", t.TempDir())
	t.Setenv("USERPROFILE", os.Getenv("HOME"))

	home := os.Getenv("HOME")
	claudePath := filepath.Join(home, ".claude.json")
	if err := os.WriteFile(claudePath, []byte(`{"mcpServers":{"gasoline":{"command":"gasoline-mcp"}}}`), 0o600); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", claudePath, err)
	}
	if got := findMCPConfig(); got != claudePath {
		t.Fatalf("findMCPConfig() = %q, want %q", got, claudePath)
	}
}

func TestPIDFileLifecycleAndLegacyFallback(t *testing.T) {
	// Do not run in parallel; uses Setenv.
	stateRoot := t.TempDir()
	home := t.TempDir()
	t.Setenv(state.StateDirEnv, stateRoot)
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	const port = 56789
	if err := writePIDFile(port); err != nil {
		t.Fatalf("writePIDFile(%d) error = %v", port, err)
	}
	if got := readPIDFile(port); got != os.Getpid() {
		t.Fatalf("readPIDFile(%d) = %d, want current pid %d", port, got, os.Getpid())
	}
	removePIDFile(port)
	if got := readPIDFile(port); got != 0 {
		t.Fatalf("readPIDFile(%d) after remove = %d, want 0", port, got)
	}

	legacyPath, err := state.LegacyPIDFile(43210)
	if err != nil {
		t.Fatalf("LegacyPIDFile() error = %v", err)
	}
	if err := os.WriteFile(legacyPath, []byte("12345"), 0o600); err != nil {
		t.Fatalf("WriteFile(legacy pid) error = %v", err)
	}
	if got := readPIDFile(43210); got != 12345 {
		t.Fatalf("readPIDFile(legacy) = %d, want 12345", got)
	}
}

func TestRunSetupCheckPrintsDiagnostics(t *testing.T) {
	// Do not run in parallel; test redirects os.Stdout.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen error = %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe(stdout) error = %v", err)
	}
	oldOut := os.Stdout
	os.Stdout = w
	runSetupCheck(port)
	os.Stdout = oldOut
	_ = w.Close()
	out, err := io.ReadAll(r)
	_ = r.Close()
	if err != nil {
		t.Fatalf("ReadAll(stdout) error = %v", err)
	}
	text := string(out)
	if !strings.Contains(text, "GASOLINE SETUP CHECK") || !strings.Contains(text, "Next steps:") {
		t.Fatalf("runSetupCheck output missing expected sections:\n%s", text)
	}
	if !strings.Contains(text, "Port:    "+strconv.Itoa(port)) {
		t.Fatalf("runSetupCheck output missing port %d", port)
	}
}

func TestRunSetupCheckIncludesFastPathTelemetrySummary(t *testing.T) {
	// Do not run in parallel; test redirects os.Stdout and uses Setenv.
	t.Setenv(state.StateDirEnv, t.TempDir())
	resetFastPathResourceReadCounters()
	recordFastPathResourceRead("gasoline://capabilities", true, 0)
	recordFastPathResourceRead("gasoline://playbook/nonexistent/quick", false, -32002)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen error = %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe(stdout) error = %v", err)
	}
	oldOut := os.Stdout
	os.Stdout = w
	runSetupCheck(port)
	os.Stdout = oldOut
	_ = w.Close()

	out, err := io.ReadAll(r)
	_ = r.Close()
	if err != nil {
		t.Fatalf("ReadAll(stdout) error = %v", err)
	}
	text := string(out)
	if !strings.Contains(text, "Checking bridge fast-path telemetry...") {
		t.Fatalf("runSetupCheck output missing fast-path telemetry section:\n%s", text)
	}
	if !strings.Contains(text, "bridge-fastpath-resource-read.jsonl") {
		t.Fatalf("runSetupCheck output missing telemetry log path:\n%s", text)
	}
	if !strings.Contains(text, "success=1") || !strings.Contains(text, "failure=1") {
		t.Fatalf("runSetupCheck output missing telemetry summary counters:\n%s", text)
	}
	if !strings.Contains(text, "-32002=1") {
		t.Fatalf("runSetupCheck output missing error-code summary:\n%s", text)
	}
}
