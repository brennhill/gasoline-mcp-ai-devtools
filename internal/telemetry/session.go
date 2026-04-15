// session.go — Session ID management with inactivity-based rotation.

package telemetry

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"
)

// sessionTimeout is the inactivity duration after which a new session starts.
const sessionTimeout = 30 * time.Minute

// session holds the current session state. Thread-safe via mu.
var session struct {
	mu       sync.Mutex
	id       string
	lastSeen time.Time
}

// sessionEndCallback is called when a session rotates due to timeout.
// Set by UsageTracker to emit session_end beacons.
var sessionEndCallback func(reason string)

// SetSessionEndCallback registers a callback for session timeout rotation.
func SetSessionEndCallback(fn func(reason string)) {
	session.mu.Lock()
	sessionEndCallback = fn
	session.mu.Unlock()
}

// GetSessionID returns the current session ID, minting one if none exists.
// Does NOT detect timeout rotation — that happens in TouchSession, which
// is always called from RecordToolCall right after GetSessionID.
// This keeps GetSessionID simple and race-free (no unlock/relock).
func GetSessionID() string {
	session.mu.Lock()
	defer session.mu.Unlock()

	if session.id == "" {
		session.id = generateSessionID()
		session.lastSeen = time.Now()
	}
	return session.id
}

// TouchSession refreshes the session's last-seen timestamp.
// Detects timeout-based rotation: if lastSeen is older than sessionTimeout,
// fires the session_end callback and mints a new session.
// The callback is fired AFTER all state is updated and the lock is released,
// so it's safe for the callback to call GetSessionID (no deadlock, no TOCTOU).
func TouchSession() {
	session.mu.Lock()
	if session.id == "" {
		session.id = generateSessionID()
	}

	var cb func(string)
	if !session.lastSeen.IsZero() && time.Since(session.lastSeen) > sessionTimeout {
		cb = sessionEndCallback
		// Mint new session BEFORE releasing lock — no TOCTOU window.
		session.id = generateSessionID()
	}
	session.lastSeen = time.Now()
	session.mu.Unlock()

	// Fire callback outside the lock. State is already consistent.
	if cb != nil {
		cb("timeout")
	}
}

func generateSessionID() string {
	b := make([]byte, 8) // 16 hex chars
	if _, err := rand.Read(b); err != nil {
		return "0000000000000000"
	}
	return hex.EncodeToString(b)
}

// resetSessionState clears session state for testing.
func resetSessionState() {
	session.mu.Lock()
	session.id = ""
	session.lastSeen = time.Time{}
	session.mu.Unlock()
}
