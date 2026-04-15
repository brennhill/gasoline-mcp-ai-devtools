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

// GetSessionID returns the current session ID, creating or rotating as needed.
// Does NOT refresh lastSeen on existing sessions — only TouchSession (called from
// UsageTracker.RecordToolCall and feature callbacks) extends the session.
// When minting a new session (first call or after timeout), sets lastSeen to now.
// Emits session_end("timeout") when rotating due to inactivity.
func GetSessionID() string {
	session.mu.Lock()
	defer session.mu.Unlock()

	rotated := false
	if session.id != "" && !session.lastSeen.IsZero() && time.Since(session.lastSeen) > sessionTimeout {
		rotated = true
		cb := sessionEndCallback
		// Unlock before callback to avoid deadlock (callback may call TouchSession).
		session.mu.Unlock()
		if cb != nil {
			cb("timeout")
		}
		session.mu.Lock()
	}

	if session.id == "" || rotated {
		session.id = generateSessionID()
		session.lastSeen = time.Now()
	}
	return session.id
}

// TouchSession refreshes the session's last-seen timestamp.
// Called when telemetry-bearing activity occurs (tool calls, feature usage).
// Mints a new session if none exists yet.
func TouchSession() {
	session.mu.Lock()
	if session.id == "" {
		session.id = generateSessionID()
	}
	session.lastSeen = time.Now()
	session.mu.Unlock()
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
