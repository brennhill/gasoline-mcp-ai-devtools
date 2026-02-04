// debug.go — HTTP debug logging helpers for the Capture type
package capture

import (
	"fmt"
	"os"
)

// logPollingActivity adds an entry to the circular polling log buffer.
// Thread-safe: caller must hold c.mu lock.
func (c *Capture) logPollingActivity(entry PollingLogEntry) {
	c.pollingLog[c.pollingLogIndex] = entry
	c.pollingLogIndex = (c.pollingLogIndex + 1) % 50
}

// logHTTPDebugEntry adds an entry to the circular HTTP debug log buffer.
// Thread-safe: caller must hold c.mu lock.
// Does NOT print to stderr - caller should call PrintHTTPDebug() after unlocking.
func (c *Capture) logHTTPDebugEntry(entry HTTPDebugEntry) {
	c.httpDebugLog[c.httpDebugLogIndex] = entry
	c.httpDebugLogIndex = (c.httpDebugLogIndex + 1) % 50
}

// PrintHTTPDebug prints an HTTP debug entry to stderr.
// Must be called WITHOUT holding the lock to avoid deadlock.
// Quiet mode: All HTTP debug output suppressed for clean MCP experience.
// Debug entries are still stored in circular buffer for get_health tool.
func PrintHTTPDebug(entry HTTPDebugEntry) {
	// Quiet mode: HTTP debug goes to circular buffer only, not stderr
	// Only print errors (non-2xx status codes)
	if entry.ResponseStatus >= 400 {
		fmt.Fprintf(os.Stderr, "[gasoline] HTTP %s %s | status=%d\n",
			entry.Method, entry.Endpoint, entry.ResponseStatus)
		if entry.Error != "" {
			fmt.Fprintf(os.Stderr, "[gasoline]   Error: %s\n", entry.Error)
		}
	}
}
