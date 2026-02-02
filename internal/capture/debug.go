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
func PrintHTTPDebug(entry HTTPDebugEntry) {
	fmt.Fprintf(os.Stderr, "[gasoline] HTTP %s %s | session=%s client=%s status=%d duration=%dms\n",
		entry.Method, entry.Endpoint, entry.SessionID, entry.ClientID, entry.ResponseStatus, entry.DurationMs)
	if entry.RequestBody != "" {
		fmt.Fprintf(os.Stderr, "[gasoline]   Request: %s\n", entry.RequestBody)
	}
	if entry.ResponseBody != "" {
		fmt.Fprintf(os.Stderr, "[gasoline]   Response: %s\n", entry.ResponseBody)
	}
	if entry.Error != "" {
		fmt.Fprintf(os.Stderr, "[gasoline]   Error: %s\n", entry.Error)
	}
}
