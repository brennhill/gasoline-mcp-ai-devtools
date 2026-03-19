// debug_log.go — File-based debug logging, enabled by default.
// Why: Enables append-only diagnostic tracing without polluting stderr or stdout MCP transport.
// Disable with STRUM_DEBUG=off. Override path with GASOLINE_MCP_DEBUG_FILE.

package main

import (
	"fmt"
	"os"
	"sync"
	"time"

	statecfg "github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/state"
)

var (
	debugLogPath    string
	debugLogOnce    sync.Once
	debugLogEnabled bool
)

func debugEnabled() bool {
	debugLogOnce.Do(func() {
		if os.Getenv("STRUM_DEBUG") == "off" {
			debugLogEnabled = false
			return
		}
		// Explicit path override takes priority
		if override := os.Getenv("GASOLINE_MCP_DEBUG_FILE"); override != "" {
			debugLogPath = override
			debugLogEnabled = true
			return
		}
		// Default: write to state dir
		if path, err := statecfg.InRoot("logs", "bridge-debug.jsonl"); err == nil {
			debugLogPath = path
			debugLogEnabled = true
		}
	})
	return debugLogEnabled
}

func debugf(format string, args ...any) {
	if !debugEnabled() {
		return
	}
	f, err := os.OpenFile(debugLogPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return
	}
	defer func() { _ = f.Close() }()
	ts := time.Now().UTC().Format(time.RFC3339Nano)
	_, _ = fmt.Fprintf(f, "%s "+format+"\n", append([]any{ts}, args...)...)
}
