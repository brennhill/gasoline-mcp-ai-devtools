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
// A new session is minted if none exists or the previous one expired (30min inactivity).
func GetSessionID() string {
	session.mu.Lock()
	defer session.mu.Unlock()

	now := time.Now()
	if session.id == "" || now.Sub(session.lastSeen) > sessionTimeout {
		session.id = generateSessionID()
	}
	session.lastSeen = now
	return session.id
}

// TouchSession refreshes the session's last-seen timestamp without returning the ID.
// Called when telemetry-bearing activity occurs (tool calls, feature usage).
func TouchSession() {
	session.mu.Lock()
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
