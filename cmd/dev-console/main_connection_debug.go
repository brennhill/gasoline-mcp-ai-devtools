// Purpose: Debug logging helper for connection lifecycle failures.
// Why: Keeps diagnostic file-writing details out of orchestration flow.

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// debugWriter accumulates debug info to a lazily-created temp file.
type debugWriter struct {
	path string
	port int
}

// write appends a debug entry to the debug file, creating it on first call.
func (d *debugWriter) write(phase string, err error, details map[string]any) {
	if d.path == "" {
		timestamp := time.Now().Format("20060102-150405")
		d.path = filepath.Join(os.TempDir(), fmt.Sprintf("gasoline-debug-%s.log", timestamp))
	}

	info := map[string]any{
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"phase":     phase,
		"error":     fmt.Sprintf("%v", err),
		"port":      d.port,
		"pid":       os.Getpid(),
	}
	for k, v := range details {
		info[k] = v
	}

	// Error impossible: map contains only primitive types from input
	data, _ := json.MarshalIndent(info, "", "  ")
	// #nosec G703 -- debug path is always under os.TempDir with server-generated timestamp
	f, err := os.OpenFile(d.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600) // nosemgrep: go_filesystem_rule-fileread -- CLI tool writes to local debug log
	if err == nil {
		_, _ = f.Write(data)
		_, _ = f.WriteString("\n")
		_ = f.Close()
	}
}
