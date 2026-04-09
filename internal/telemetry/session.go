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

// GetSessionID returns the current session ID, creating or rotating as needed.
// Read-only: does NOT refresh lastSeen. Only TouchSession (called from
// UsageCounter.Increment and feature callbacks) extends the session.
// A new session is minted if none exists or the previous one expired.
func GetSessionID() string {
	session.mu.Lock()
	defer session.mu.Unlock()

	if session.id == "" || (!session.lastSeen.IsZero() && time.Since(session.lastSeen) > sessionTimeout) {
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
