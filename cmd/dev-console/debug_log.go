// debug_log.go — Debug logging runtime behavior and integration logic.
// Docs: docs/features/feature/observe/index.md
package main

import (
	"fmt"
	"os"
	"sync"
	"time"
)

var (
	debugLogPath string
	debugLogOnce sync.Once
)

func debugEnabled() bool {
	debugLogOnce.Do(func() {
		debugLogPath = os.Getenv("GASOLINE_MCP_DEBUG_FILE")
	})
	return debugLogPath != ""
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
