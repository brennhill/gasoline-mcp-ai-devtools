// Purpose: Provides panic-recovering goroutine launcher for background asynchronous tasks.
// Why: Prevents single background task panics from crashing the daemon process.
// Docs: docs/features/feature/backend-log-streaming/index.md

package util

import (
	"fmt"
	"os"
	"runtime/debug"
)

// SafeGo launches fn in a goroutine with deferred panic recovery.
// On panic: logs stack trace to stderr. Does NOT os.Exit — background
// panics should be survivable so the daemon stays up.
func SafeGo(fn func()) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				fmt.Fprintf(os.Stderr, "[Kaboom] PANIC in background goroutine: %v\n%s\n", r, debug.Stack())
			}
		}()
		fn()
	}()
}
