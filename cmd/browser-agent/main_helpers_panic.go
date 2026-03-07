// Purpose: Panic crash logging and crash-file resolution helpers.
// Why: Isolates crash diagnostics from normal startup/dispatch control flow in main.go.
// Docs: docs/features/reliability/index.md

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/state"
)

// handlePanicRecovery logs crash details and writes a crash file for diagnostic discovery.
func handlePanicRecovery(r any) {
	stack := make([]byte, 4096)
	n := runtime.Stack(stack, false)
	stack = stack[:n]

	fmt.Fprintf(os.Stderr, "\n[gasoline] FATAL ERROR\n")

	logFile, err := state.DefaultLogFile()
	if err != nil {
		logFile = filepath.Join(os.TempDir(), "gasoline.jsonl")
	}
	entry := map[string]any{
		"type":       "lifecycle",
		"event":      "crash",
		"reason":     fmt.Sprintf("%v", r),
		"stack":      string(stack),
		"timestamp":  time.Now().UTC().Format(time.RFC3339),
		"go_version": runtime.Version(),
		"os":         runtime.GOOS,
		"arch":       runtime.GOARCH,
	}
	if data, err := json.Marshal(entry); err == nil {
		// #nosec G301 -- runtime state directory: owner rwx, group rx for diagnostics
		_ = os.MkdirAll(filepath.Dir(logFile), 0o750)
		// #nosec G304 -- crash logs path resolved from trusted runtime state directory
		if f, err := os.OpenFile(logFile, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0600); err == nil { // nosemgrep: go_filesystem_rule-fileread -- CLI tool writes to local crash log
			_, _ = f.Write(data)         // #nosec G104 -- best-effort crash logging
			_, _ = f.Write([]byte{'\n'}) // #nosec G104 -- best-effort crash logging
			_ = f.Close()                // #nosec G104 -- best-effort crash logging
		}
	}

	if diagPath := appendExitDiagnostic("panic", map[string]any{
		"reason": fmt.Sprintf("%v", r),
		"stack":  string(stack),
	}); diagPath != "" {
		fmt.Fprintf(os.Stderr, "[gasoline] Crash details written to: %s\n", diagPath)
	}
	os.Exit(1)
}
