// session_test.go — Tests for session ID management.

package telemetry

import (
	"regexp"
	"sync"
	"testing"
	"time"
)

func TestGetSessionID_Returns16CharHex(t *testing.T) {
	resetSessionState()
	TouchSession() // mint a session
	id := GetSessionID()
	if !regexp.MustCompile(`^[0-9a-f]{16}$`).MatchString(id) {
		t.Errorf("GetSessionID() = %q, want 16-char hex", id)
	}
}

func TestGetSessionID_StableWithinTimeout(t *testing.T) {
	resetSessionState()
	TouchSession()
	id1 := GetSessionID()
	id2 := GetSessionID()
	if id1 != id2 {
		t.Errorf("GetSessionID() returned different IDs within same session: %q vs %q", id1, id2)
	}
}

func TestTouchSession_RotatesAfterInactivity(t *testing.T) {
	resetSessionState()
	TouchSession()
	id1 := GetSessionID()

	// Simulate inactivity beyond timeout.
	session.mu.Lock()
	session.lastSeen = time.Now().Add(-sessionTimeout - time.Second)
	session.mu.Unlock()

	// TouchSession detects expiry and mints a new session.
	TouchSession()
	id2 := GetSessionID()
	if id1 == id2 {
		t.Error("session should have rotated after inactivity timeout")
	}
}

func TestGetSessionID_DoesNotExtendSession(t *testing.T) {
	// GetSessionID must be read-only — it must NOT refresh lastSeen.
	// Only TouchSession (and Increment) should extend the session.
	resetSessionState()
	TouchSession()

	// Record lastSeen before calling GetSessionID.
	session.mu.Lock()
	before := session.lastSeen
	session.mu.Unlock()

	// Small delay so time.Now() would differ if GetSessionID touches.
	time.Sleep(2 * time.Millisecond)

	_ = GetSessionID()

	session.mu.Lock()
	after := session.lastSeen
	session.mu.Unlock()

	if !after.Equal(before) {
		t.Errorf("GetSessionID modified lastSeen: before=%v, after=%v", before, after)
	}
}

func TestTouchSession_RefreshesLastSeen(t *testing.T) {
	resetSessionState()
	TouchSession()
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

func TestTouchSession_BeforeFirstGetSessionID(t *testing.T) {
	resetSessionState()

	// TouchSession on empty session should mint a new session.
	TouchSession()

	id := GetSessionID()
	if !regexp.MustCompile(`^[0-9a-f]{16}$`).MatchString(id) {
		t.Errorf("GetSessionID() after TouchSession on empty = %q, want 16-char hex", id)
	}
}

func TestGetSessionID_ConcurrentAccess(t *testing.T) {
	resetSessionState()
	TouchSession()

	var wg sync.WaitGroup
	ids := make([]string, 100)
	for i := range ids {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			ids[idx] = GetSessionID()
		}(i)
	}
	wg.Wait()

	// All should return the same ID (session is active).
	for i, id := range ids {
		if id != ids[0] {
			t.Errorf("goroutine %d got %q, goroutine 0 got %q — expected same session", i, id, ids[0])
			break
		}
	}
}

func TestUsageTracker_Increment_TouchesSession(t *testing.T) {
	resetSessionState()
	TouchSession()
	id1 := GetSessionID()

	counter := NewUsageTracker()

	// Backdate to near-expiry.
	session.mu.Lock()
	session.lastSeen = time.Now().Add(-sessionTimeout + time.Second)
	session.mu.Unlock()

	// Increment should touch session, refreshing lastSeen.
	counter.RecordToolCall("observe:errors", 0, false)

	// Verify lastSeen was refreshed (not still near-expiry).
	session.mu.Lock()
	ls := session.lastSeen
	session.mu.Unlock()

	if time.Since(ls) > 5*time.Second {
		t.Errorf("Increment did not touch session: lastSeen is %v ago", time.Since(ls))
	}

	// Session should not have rotated.
	id2 := GetSessionID()
	if id1 != id2 {
		t.Error("Increment should have touched session, preventing rotation")
	}
}
