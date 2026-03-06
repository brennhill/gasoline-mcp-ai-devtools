// store_sessions_test.go — Regression tests for WaitForSession stale timestamp (T8).
// Purpose: Validates that WaitForSession re-reads lastDrawStartedAt on each check,
// so a second MarkDrawStarted mid-wait invalidates sessions from the first draw cycle.
// Docs: docs/features/feature/annotated-screenshots/index.md

package annotation

import (
	"sync"
	"testing"
	"time"
)

// TestWaitForSession_ReReadsDrawTimestamp_T8 is the regression test for T8.
// Scenario:
//  1. MarkDrawStarted (first draw cycle)
//  2. Start WaitForSession in a goroutine
//  3. Store a session from the first draw cycle
//  4. MarkDrawStarted again (second draw cycle) — invalidates the first session
//  5. Verify WaitForSession does NOT return the first session
//  6. Store a session from the second draw cycle
//  7. Verify WaitForSession returns the second session
func TestWaitForSession_ReReadsDrawTimestamp_T8(t *testing.T) {
	store := NewStore(10 * time.Minute)
	defer store.Close()

	// Step 1: first draw cycle
	store.MarkDrawStarted()
	time.Sleep(2 * time.Millisecond)

	// Store a session from the first draw cycle.
	firstSession := &Session{
		Annotations: []Annotation{{ID: "first", Text: "first cycle"}},
		PageURL:     "https://example.com/page1",
		TabID:       1,
		Timestamp:   time.Now().UnixMilli(),
	}
	store.StoreSession(1, firstSession)

	// Step 4: second draw cycle — advances the threshold past firstSession.
	time.Sleep(2 * time.Millisecond)
	store.MarkDrawStarted()
	time.Sleep(2 * time.Millisecond)

	// Step 2: start WaitForSession. Because lastDrawStartedAt is now after
	// firstSession.Timestamp, the checker must NOT return firstSession.
	var mu sync.Mutex
	var resultSession *Session
	var timedOut bool
	done := make(chan struct{})

	go func() {
		s, to := store.WaitForSession(500 * time.Millisecond)
		mu.Lock()
		resultSession = s
		timedOut = to
		mu.Unlock()
		close(done)
	}()

	// Give the goroutine time to enter the wait loop and verify it hasn't returned yet.
	time.Sleep(20 * time.Millisecond)
	select {
	case <-done:
		// If we get here, WaitForSession returned the stale first session — that's the T8 bug.
		mu.Lock()
		if resultSession != nil && resultSession.Annotations[0].ID == "first" {
			mu.Unlock()
			t.Fatal("T8 regression: WaitForSession returned session from previous draw cycle")
		}
		mu.Unlock()
		// It returned something else or timed out, which is fine; fall through.
	default:
		// Still waiting — correct behavior. Now store the second session.
	}

	// Step 6: store a session from the second draw cycle.
	secondSession := &Session{
		Annotations: []Annotation{{ID: "second", Text: "second cycle"}},
		PageURL:     "https://example.com/page2",
		TabID:       2,
		Timestamp:   time.Now().UnixMilli(),
	}
	store.StoreSession(2, secondSession)

	// Step 7: WaitForSession should now return the second session.
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("WaitForSession did not return within timeout")
	}

	mu.Lock()
	defer mu.Unlock()
	if timedOut {
		t.Fatal("WaitForSession timed out unexpectedly")
	}
	if resultSession == nil {
		t.Fatal("WaitForSession returned nil session")
	}
	if resultSession.Annotations[0].ID != "second" {
		t.Errorf("expected session from second draw cycle (id=second), got id=%s",
			resultSession.Annotations[0].ID)
	}
}

// TestWaitForSession_SingleDrawCycle_StillWorks verifies the basic case:
// a single draw cycle where WaitForSession returns the session correctly.
func TestWaitForSession_SingleDrawCycle_StillWorks(t *testing.T) {
	store := NewStore(10 * time.Minute)
	defer store.Close()

	store.MarkDrawStarted()
	time.Sleep(2 * time.Millisecond)

	done := make(chan struct{})
	var result *Session
	var timedOut bool

	go func() {
		result, timedOut = store.WaitForSession(500 * time.Millisecond)
		close(done)
	}()

	time.Sleep(10 * time.Millisecond)
	session := &Session{
		Annotations: []Annotation{{ID: "a1", Text: "test"}},
		PageURL:     "https://example.com",
		TabID:       1,
		Timestamp:   time.Now().UnixMilli(),
	}
	store.StoreSession(1, session)

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("WaitForSession did not return within timeout")
	}

	if timedOut {
		t.Fatal("WaitForSession timed out unexpectedly")
	}
	if result == nil {
		t.Fatal("WaitForSession returned nil")
	}
	if result.Annotations[0].ID != "a1" {
		t.Errorf("expected annotation id 'a1', got %q", result.Annotations[0].ID)
	}
}
