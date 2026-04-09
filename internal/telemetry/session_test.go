// session_test.go — Tests for session ID management.

package telemetry

import (
	"regexp"
	"testing"
	"time"
)

func TestGetSessionID_Returns16CharHex(t *testing.T) {
	resetSessionState()
	id := GetSessionID()
	if !regexp.MustCompile(`^[0-9a-f]{16}$`).MatchString(id) {
		t.Errorf("GetSessionID() = %q, want 16-char hex", id)
	}
}

func TestGetSessionID_StableWithinTimeout(t *testing.T) {
	resetSessionState()
	id1 := GetSessionID()
	id2 := GetSessionID()
	if id1 != id2 {
		t.Errorf("GetSessionID() returned different IDs within same session: %q vs %q", id1, id2)
	}
}

func TestGetSessionID_RotatesAfterInactivity(t *testing.T) {
	resetSessionState()
	id1 := GetSessionID()

	// Simulate inactivity by backdating lastSeen beyond the timeout.
	session.mu.Lock()
	session.lastSeen = time.Now().Add(-sessionTimeout - time.Second)
	session.mu.Unlock()

	id2 := GetSessionID()
	if id1 == id2 {
		t.Error("GetSessionID() should have rotated after inactivity timeout")
	}
}

func TestTouchSession_RefreshesLastSeen(t *testing.T) {
	resetSessionState()
	id1 := GetSessionID()

	// Backdate to near-expiry.
	session.mu.Lock()
	session.lastSeen = time.Now().Add(-sessionTimeout + time.Second)
	session.mu.Unlock()

	// Touch should refresh without rotating.
	TouchSession()

	id2 := GetSessionID()
	if id1 != id2 {
		t.Error("TouchSession should have prevented rotation")
	}
}
