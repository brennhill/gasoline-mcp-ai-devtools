// annotation_store_test.go — Tests for the annotation store.
package main

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestAnnotationStore_StoreAndGetSession(t *testing.T) {
	store := NewAnnotationStore(10 * time.Minute)
	defer store.Close()

	session := &AnnotationSession{
		Annotations: []Annotation{
			{
				ID:             "ann_1",
				Rect:           AnnotationRect{X: 100, Y: 200, Width: 150, Height: 50},
				Text:           "make this darker",
				Timestamp:      time.Now().UnixMilli(),
				PageURL:        "https://example.com",
				ElementSummary: "button.primary 'Submit'",
				CorrelationID:  "detail_1",
			},
		},
		ScreenshotPath: "/tmp/draw_test.png",
		PageURL:        "https://example.com",
		TabID:          42,
		Timestamp:      time.Now().UnixMilli(),
	}

	store.StoreSession(42, session)

	got := store.GetSession(42)
	if got == nil {
		t.Fatal("expected session, got nil")
	}
	if len(got.Annotations) != 1 {
		t.Fatalf("expected 1 annotation, got %d", len(got.Annotations))
	}
	if got.Annotations[0].Text != "make this darker" {
		t.Errorf("expected text 'make this darker', got %q", got.Annotations[0].Text)
	}
	if got.ScreenshotPath != "/tmp/draw_test.png" {
		t.Errorf("expected screenshot path, got %q", got.ScreenshotPath)
	}
}

func TestAnnotationStore_GetSessionNotFound(t *testing.T) {
	store := NewAnnotationStore(10 * time.Minute)
	defer store.Close()
	got := store.GetSession(999)
	if got != nil {
		t.Errorf("expected nil for non-existent session, got %+v", got)
	}
}

func TestAnnotationStore_SessionOverwrite(t *testing.T) {
	store := NewAnnotationStore(10 * time.Minute)
	defer store.Close()

	session1 := &AnnotationSession{
		Annotations: []Annotation{{Text: "first"}},
		TabID:       42,
		Timestamp:   100,
	}
	session2 := &AnnotationSession{
		Annotations: []Annotation{{Text: "second"}, {Text: "third"}},
		TabID:       42,
		Timestamp:   200,
	}

	store.StoreSession(42, session1)
	store.StoreSession(42, session2)

	got := store.GetSession(42)
	if got == nil {
		t.Fatal("expected session, got nil")
	}
	if len(got.Annotations) != 2 {
		t.Fatalf("expected 2 annotations after overwrite, got %d", len(got.Annotations))
	}
	if got.Annotations[0].Text != "second" {
		t.Errorf("expected text 'second', got %q", got.Annotations[0].Text)
	}
}

func TestAnnotationStore_GetLatestSession(t *testing.T) {
	store := NewAnnotationStore(10 * time.Minute)
	defer store.Close()

	store.StoreSession(1, &AnnotationSession{TabID: 1, Timestamp: 100, Annotations: []Annotation{{Text: "tab1"}}})
	store.StoreSession(2, &AnnotationSession{TabID: 2, Timestamp: 300, Annotations: []Annotation{{Text: "tab2"}}})
	store.StoreSession(3, &AnnotationSession{TabID: 3, Timestamp: 200, Annotations: []Annotation{{Text: "tab3"}}})

	latest := store.GetLatestSession()
	if latest == nil {
		t.Fatal("expected latest session, got nil")
	}
	if latest.TabID != 2 {
		t.Errorf("expected latest tab 2, got %d", latest.TabID)
	}
}

func TestAnnotationStore_GetLatestSessionEmpty(t *testing.T) {
	store := NewAnnotationStore(10 * time.Minute)
	defer store.Close()
	latest := store.GetLatestSession()
	if latest != nil {
		t.Errorf("expected nil for empty store, got %+v", latest)
	}
}

func TestAnnotationStore_StoreAndGetDetail(t *testing.T) {
	store := NewAnnotationStore(10 * time.Minute)
	defer store.Close()

	detail := AnnotationDetail{
		CorrelationID:  "detail_1",
		Selector:       "button.primary",
		Tag:            "button",
		TextContent:    "Submit",
		Classes:        []string{"primary", "rounded"},
		ID:             "submit-btn",
		ComputedStyles: map[string]string{"background-color": "rgb(59, 130, 246)"},
		ParentSelector: "form.checkout > div.actions",
		BoundingRect:   AnnotationRect{X: 100, Y: 200, Width: 150, Height: 50},
	}

	store.StoreDetail("detail_1", detail)

	got, found := store.GetDetail("detail_1")
	if !found {
		t.Fatal("expected to find detail")
	}
	if got.Selector != "button.primary" {
		t.Errorf("expected selector 'button.primary', got %q", got.Selector)
	}
	if got.ComputedStyles["background-color"] != "rgb(59, 130, 246)" {
		t.Errorf("unexpected computed styles")
	}
}

func TestAnnotationStore_DetailNotFound(t *testing.T) {
	store := NewAnnotationStore(10 * time.Minute)
	defer store.Close()
	_, found := store.GetDetail("nonexistent")
	if found {
		t.Error("expected not found for non-existent detail")
	}
}

func TestAnnotationStore_DetailExpired(t *testing.T) {
	// Use very short TTL
	store := NewAnnotationStore(1 * time.Millisecond)
	defer store.Close()

	store.StoreDetail("expire_test", AnnotationDetail{Selector: "div.test"})

	// Wait for expiration
	time.Sleep(5 * time.Millisecond)

	_, found := store.GetDetail("expire_test")
	if found {
		t.Error("expected detail to be expired")
	}
}

func TestAnnotationStore_ZeroAnnotations(t *testing.T) {
	store := NewAnnotationStore(10 * time.Minute)
	defer store.Close()

	session := &AnnotationSession{
		Annotations:    []Annotation{},
		ScreenshotPath: "/tmp/empty.png",
		TabID:          42,
		Timestamp:      time.Now().UnixMilli(),
	}
	store.StoreSession(42, session)

	got := store.GetSession(42)
	if got == nil {
		t.Fatal("expected session even with 0 annotations")
	}
	if len(got.Annotations) != 0 {
		t.Errorf("expected 0 annotations, got %d", len(got.Annotations))
	}
}

func TestAnnotationStore_ConcurrentAccess(t *testing.T) {
	store := NewAnnotationStore(10 * time.Minute)
	defer store.Close()

	var wg sync.WaitGroup
	// Concurrent session writes
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(tabID int) {
			defer wg.Done()
			store.StoreSession(tabID, &AnnotationSession{
				TabID:     tabID,
				Timestamp: time.Now().UnixMilli(),
				Annotations: []Annotation{{Text: fmt.Sprintf("tab%d", tabID)}},
			})
		}(i)
	}
	// Concurrent detail writes
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			store.StoreDetail(fmt.Sprintf("detail_%d", id), AnnotationDetail{
				Selector: fmt.Sprintf("div.item-%d", id),
			})
		}(i)
	}
	// Concurrent reads
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(tabID int) {
			defer wg.Done()
			store.GetSession(tabID)
			store.GetDetail(fmt.Sprintf("detail_%d", tabID))
			store.GetLatestSession()
		}(i)
	}
	wg.Wait()

	// Verify at least some data was stored
	found := 0
	for i := 0; i < 50; i++ {
		if store.GetSession(i) != nil {
			found++
		}
	}
	if found == 0 {
		t.Error("Expected at least some sessions to be stored after concurrent access")
	}
}

func TestAnnotationStore_SessionEvictionCap(t *testing.T) {
	store := NewAnnotationStore(10 * time.Minute)
	defer store.Close()

	// Store more sessions than maxSessions (100)
	for i := 1; i <= 110; i++ {
		store.StoreSession(i, &AnnotationSession{
			TabID:     i,
			Timestamp: int64(i),
			Annotations: []Annotation{{Text: fmt.Sprintf("session_%d", i)}},
		})
	}

	// Count surviving sessions (should be <= maxSessions)
	count := 0
	for i := 1; i <= 110; i++ {
		if store.GetSession(i) != nil {
			count++
		}
	}
	if count > 100 {
		t.Errorf("Expected at most 100 sessions after eviction, got %d", count)
	}
	// The newest sessions should survive
	if store.GetSession(110) == nil {
		t.Error("Expected newest session (110) to survive eviction")
	}
}

func TestAnnotationStore_MarkDrawStarted(t *testing.T) {
	store := NewAnnotationStore(10 * time.Minute)
	defer store.Close()

	before := time.Now().UnixMilli()
	store.MarkDrawStarted()
	after := time.Now().UnixMilli()

	store.mu.RLock()
	ts := store.lastDrawStartedAt
	store.mu.RUnlock()

	if ts < before || ts > after {
		t.Errorf("expected lastDrawStartedAt between %d and %d, got %d", before, after, ts)
	}
}

func TestAnnotationStore_WaitForSession_ImmediateReturn(t *testing.T) {
	store := NewAnnotationStore(10 * time.Minute)
	defer store.Close()

	// Mark draw started, then store a session with a newer timestamp
	store.MarkDrawStarted()
	time.Sleep(1 * time.Millisecond)
	store.StoreSession(1, &AnnotationSession{
		TabID:       1,
		Timestamp:   time.Now().UnixMilli(),
		Annotations: []Annotation{{Text: "immediate"}},
	})

	session, timedOut := store.WaitForSession(100 * time.Millisecond)
	if timedOut {
		t.Fatal("expected immediate return, got timeout")
	}
	if session == nil {
		t.Fatal("expected session, got nil")
	}
	if session.Annotations[0].Text != "immediate" {
		t.Errorf("expected text 'immediate', got %q", session.Annotations[0].Text)
	}
}

func TestAnnotationStore_WaitForSession_BlocksAndReturns(t *testing.T) {
	store := NewAnnotationStore(10 * time.Minute)
	defer store.Close()

	store.MarkDrawStarted()

	// Store session in a goroutine after a delay
	go func() {
		time.Sleep(50 * time.Millisecond)
		store.StoreSession(1, &AnnotationSession{
			TabID:       1,
			Timestamp:   time.Now().UnixMilli(),
			Annotations: []Annotation{{Text: "delayed"}},
		})
	}()

	start := time.Now()
	session, timedOut := store.WaitForSession(2 * time.Second)
	elapsed := time.Since(start)

	if timedOut {
		t.Fatal("expected session, got timeout")
	}
	if session == nil {
		t.Fatal("expected session, got nil")
	}
	if session.Annotations[0].Text != "delayed" {
		t.Errorf("expected text 'delayed', got %q", session.Annotations[0].Text)
	}
	if elapsed < 30*time.Millisecond {
		t.Error("expected to have blocked for at least 30ms")
	}
}

func TestAnnotationStore_WaitForSession_Timeout(t *testing.T) {
	store := NewAnnotationStore(10 * time.Minute)
	defer store.Close()

	store.MarkDrawStarted()

	start := time.Now()
	session, timedOut := store.WaitForSession(50 * time.Millisecond)
	elapsed := time.Since(start)

	if !timedOut {
		t.Error("expected timeout")
	}
	if session != nil {
		t.Errorf("expected nil session on timeout, got %+v", session)
	}
	if elapsed < 40*time.Millisecond {
		t.Error("expected to have waited at least 40ms")
	}
}

func TestAnnotationStore_WaitForSession_SkipsStaleSession(t *testing.T) {
	store := NewAnnotationStore(10 * time.Minute)
	defer store.Close()

	// Store an old session BEFORE marking draw started
	store.StoreSession(1, &AnnotationSession{
		TabID:       1,
		Timestamp:   time.Now().UnixMilli() - 5000,
		Annotations: []Annotation{{Text: "stale"}},
	})

	time.Sleep(1 * time.Millisecond)
	store.MarkDrawStarted()

	// The stale session should not be returned — it's from before draw started
	session, timedOut := store.WaitForSession(50 * time.Millisecond)

	if !timedOut {
		t.Error("expected timeout since only stale session exists")
	}
	if session != nil {
		t.Errorf("expected nil (stale session should be skipped), got %+v", session)
	}
}

func TestAnnotationStore_WaitForSession_NoDrawStarted(t *testing.T) {
	store := NewAnnotationStore(10 * time.Minute)
	defer store.Close()

	// Without MarkDrawStarted, lastDrawStartedAt is 0 — any session qualifies
	store.StoreSession(1, &AnnotationSession{
		TabID:       1,
		Timestamp:   time.Now().UnixMilli(),
		Annotations: []Annotation{{Text: "any"}},
	})

	session, timedOut := store.WaitForSession(50 * time.Millisecond)
	if timedOut {
		t.Fatal("expected immediate return, got timeout")
	}
	if session == nil {
		t.Fatal("expected session, got nil")
	}
}

func TestAnnotationStore_WaitForSession_CloseUnblocks(t *testing.T) {
	store := NewAnnotationStore(10 * time.Minute)

	store.MarkDrawStarted()

	go func() {
		time.Sleep(50 * time.Millisecond)
		store.Close()
	}()

	start := time.Now()
	session, _ := store.WaitForSession(5 * time.Second)
	elapsed := time.Since(start)

	if session != nil {
		t.Error("expected nil session after close")
	}
	if elapsed > 2*time.Second {
		t.Error("expected close to unblock promptly")
	}
}

func TestAnnotationStore_NamedSession_AppendAndGet(t *testing.T) {
	store := NewAnnotationStore(10 * time.Minute)
	defer store.Close()

	page1 := &AnnotationSession{
		TabID:       1,
		Timestamp:   100,
		PageURL:     "https://example.com/login",
		Annotations: []Annotation{{Text: "fix button"}},
	}
	page2 := &AnnotationSession{
		TabID:       1,
		Timestamp:   200,
		PageURL:     "https://example.com/dashboard",
		Annotations: []Annotation{{Text: "wrong color"}, {Text: "misaligned"}},
	}

	store.AppendToNamedSession("qa-review", page1)
	store.AppendToNamedSession("qa-review", page2)

	ns := store.GetNamedSession("qa-review")
	if ns == nil {
		t.Fatal("expected named session")
	}
	if ns.Name != "qa-review" {
		t.Errorf("expected name 'qa-review', got %q", ns.Name)
	}
	if len(ns.Pages) != 2 {
		t.Fatalf("expected 2 pages, got %d", len(ns.Pages))
	}
	if ns.Pages[0].PageURL != "https://example.com/login" {
		t.Errorf("expected first page URL, got %q", ns.Pages[0].PageURL)
	}
	if len(ns.Pages[1].Annotations) != 2 {
		t.Errorf("expected 2 annotations on page 2, got %d", len(ns.Pages[1].Annotations))
	}
}

func TestAnnotationStore_NamedSession_NotFound(t *testing.T) {
	store := NewAnnotationStore(10 * time.Minute)
	defer store.Close()

	ns := store.GetNamedSession("nonexistent")
	if ns != nil {
		t.Errorf("expected nil for non-existent named session, got %+v", ns)
	}
}

func TestAnnotationStore_NamedSession_ListSessions(t *testing.T) {
	store := NewAnnotationStore(10 * time.Minute)
	defer store.Close()

	store.AppendToNamedSession("review-1", &AnnotationSession{TabID: 1, Timestamp: 100})
	store.AppendToNamedSession("review-2", &AnnotationSession{TabID: 1, Timestamp: 200})

	names := store.ListNamedSessions()
	if len(names) != 2 {
		t.Fatalf("expected 2 named sessions, got %d", len(names))
	}
}

func TestAnnotationStore_NamedSession_Clear(t *testing.T) {
	store := NewAnnotationStore(10 * time.Minute)
	defer store.Close()

	store.AppendToNamedSession("qa", &AnnotationSession{TabID: 1, Timestamp: 100})
	store.ClearNamedSession("qa")

	ns := store.GetNamedSession("qa")
	if ns != nil {
		t.Errorf("expected nil after clear, got %+v", ns)
	}
}

func TestAnnotationStore_NamedSession_WaitBlocks(t *testing.T) {
	store := NewAnnotationStore(10 * time.Minute)
	defer store.Close()

	store.MarkDrawStarted()

	go func() {
		time.Sleep(50 * time.Millisecond)
		store.AppendToNamedSession("qa", &AnnotationSession{
			TabID:       1,
			Timestamp:   time.Now().UnixMilli(),
			Annotations: []Annotation{{Text: "waited"}},
		})
	}()

	start := time.Now()
	ns, timedOut := store.WaitForNamedSession("qa", 2*time.Second)
	elapsed := time.Since(start)

	if timedOut {
		t.Fatal("expected session, got timeout")
	}
	if ns == nil {
		t.Fatal("expected named session")
	}
	if len(ns.Pages) != 1 {
		t.Fatalf("expected 1 page, got %d", len(ns.Pages))
	}
	if elapsed < 30*time.Millisecond {
		t.Error("expected to have blocked")
	}
}

func TestAnnotationStore_NamedSession_WaitTimeout(t *testing.T) {
	store := NewAnnotationStore(10 * time.Minute)
	defer store.Close()

	store.MarkDrawStarted()

	_, timedOut := store.WaitForNamedSession("qa", 50*time.Millisecond)
	if !timedOut {
		t.Error("expected timeout")
	}
}

func TestAnnotationStore_NamedSession_EvictionCap(t *testing.T) {
	store := NewAnnotationStore(10 * time.Minute)
	defer store.Close()

	// Fill up to maxNamedSessions + 1 (51 total)
	for i := 0; i < 51; i++ {
		name := "session_" + strings.Repeat("0", 3-len(strconv.Itoa(i))) + strconv.Itoa(i) // zero-padded
		store.AppendToNamedSession(name, &AnnotationSession{
			TabID:     i + 1,
			Timestamp: int64(1000 + i),
			Annotations: []Annotation{
				{Text: "annotation for " + name},
			},
		})
		// Small sleep to ensure UpdatedAt ordering
		time.Sleep(time.Millisecond)
	}

	names := store.ListNamedSessions()
	if len(names) != 50 {
		t.Fatalf("expected 50 named sessions after eviction, got %d", len(names))
	}

	// The first session (session_000) should have been evicted (oldest UpdatedAt)
	evicted := store.GetNamedSession("session_000")
	if evicted != nil {
		t.Error("expected session_000 to be evicted, but it still exists")
	}

	// The most recent session should still exist
	latest := store.GetNamedSession("session_050")
	if latest == nil {
		t.Error("expected session_050 to exist after eviction")
	}
}

// TestAnnotationStore_WaitForSession_SpuriousWakeup verifies that WaitForSession
// loops correctly when a notify fires for an unrelated session (different tab).
func TestAnnotationStore_SessionExpired(t *testing.T) {
	store := NewAnnotationStore(10 * time.Minute)
	defer store.Close()
	// Override session TTL to something very short for testing
	store.sessionTTL = 50 * time.Millisecond

	store.StoreSession(1, &AnnotationSession{
		TabID:     1,
		Timestamp: time.Now().UnixMilli(),
		PageURL:   "https://example.com",
	})

	// Should be accessible immediately
	if store.GetSession(1) == nil {
		t.Fatal("Expected session to exist immediately after store")
	}

	// Wait for TTL to expire
	time.Sleep(100 * time.Millisecond)

	// Should be nil after expiration
	if store.GetSession(1) != nil {
		t.Error("Expected session to be nil after TTL expiration")
	}

	// GetLatestSession should also return nil
	if store.GetLatestSession() != nil {
		t.Error("Expected GetLatestSession to return nil after TTL expiration")
	}
}

func TestAnnotationStore_WaitForSession_SpuriousWakeup(t *testing.T) {
	store := NewAnnotationStore(10 * time.Minute)
	defer store.Close()

	store.MarkDrawStarted()
	time.Sleep(5 * time.Millisecond)

	var result *AnnotationSession
	var timedOut bool
	done := make(chan struct{})

	go func() {
		result, timedOut = store.WaitForSession(2 * time.Second)
		close(done)
	}()

	// First wake-up: store a session with an old timestamp (before MarkDrawStarted).
	// This is a spurious notification — WaitForSession should NOT return.
	time.Sleep(50 * time.Millisecond)
	store.mu.Lock()
	store.sessions[99] = &annotationSessionEntry{
		Session:   &AnnotationSession{TabID: 99, Timestamp: 1},
		ExpiresAt: time.Now().Add(10 * time.Minute),
	}
	ch := store.sessionNotify
	store.sessionNotify = make(chan struct{})
	store.mu.Unlock()
	close(ch)

	// Verify waiter did NOT return yet (spurious wake-up should be ignored)
	select {
	case <-done:
		t.Fatal("WaitForSession returned after spurious wake-up; expected it to keep waiting")
	case <-time.After(100 * time.Millisecond):
		// Good — still blocked
	}

	// Second wake-up: store a qualifying session (timestamp after MarkDrawStarted)
	store.StoreSession(42, &AnnotationSession{
		TabID:     42,
		Timestamp: time.Now().UnixMilli(),
	})

	select {
	case <-done:
		// Good — waiter returned
	case <-time.After(2 * time.Second):
		t.Fatal("WaitForSession did not return after qualifying session was stored")
	}

	if timedOut {
		t.Error("Expected no timeout")
	}
	if result == nil || result.TabID != 42 {
		t.Errorf("Expected session for tab 42, got %+v", result)
	}
}

// TestAnnotationStore_WaitForNamedSession_SpuriousWakeup verifies that WaitForNamedSession
// loops correctly when a notify fires for a different session name.
func TestAnnotationStore_WaitForNamedSession_SpuriousWakeup(t *testing.T) {
	store := NewAnnotationStore(10 * time.Minute)
	defer store.Close()

	store.MarkDrawStarted()
	time.Sleep(5 * time.Millisecond)

	var result *NamedAnnotationSession
	var timedOut bool
	done := make(chan struct{})

	go func() {
		result, timedOut = store.WaitForNamedSession("target", 2*time.Second)
		close(done)
	}()

	// Spurious wake: append to a DIFFERENT named session
	time.Sleep(50 * time.Millisecond)
	store.AppendToNamedSession("other", &AnnotationSession{
		TabID:     10,
		Timestamp: time.Now().UnixMilli(),
		PageURL:   "https://other.com",
	})

	// Verify still blocked
	select {
	case <-done:
		t.Fatal("WaitForNamedSession returned after unrelated session update")
	case <-time.After(100 * time.Millisecond):
		// Good
	}

	// Now store the target named session
	store.AppendToNamedSession("target", &AnnotationSession{
		TabID:     20,
		Timestamp: time.Now().UnixMilli(),
		PageURL:   "https://target.com",
	})

	select {
	case <-done:
		// Good
	case <-time.After(2 * time.Second):
		t.Fatal("WaitForNamedSession did not return after target session update")
	}

	if timedOut {
		t.Error("Expected no timeout")
	}
	if result == nil || result.Name != "target" {
		t.Errorf("Expected named session 'target', got %+v", result)
	}
}

func TestAnnotationStore_Close(t *testing.T) {
	store := NewAnnotationStore(10 * time.Minute)
	store.Close()
	// Double close should not panic (sync.Once protects)
	store.Close()
	// Store should still work after close (just no background cleanup)
	store.StoreSession(1, &AnnotationSession{TabID: 1, Timestamp: 1})
	if store.GetSession(1) == nil {
		t.Error("Expected store to still work after Close")
	}
}

func TestAnnotationStore_ConcurrentClose(t *testing.T) {
	store := NewAnnotationStore(10 * time.Minute)
	// Concurrent Close calls should not panic
	done := make(chan struct{})
	for i := 0; i < 10; i++ {
		go func() {
			store.Close()
			done <- struct{}{}
		}()
	}
	for i := 0; i < 10; i++ {
		<-done
	}
}

// --- Additional coverage: GetLatestSession skips expired sessions ---

func TestAnnotationStore_GetLatestSession_SkipsExpired(t *testing.T) {
	store := NewAnnotationStore(10 * time.Minute)
	defer store.Close()

	// Store a valid session with an older timestamp
	store.StoreSession(1, &AnnotationSession{
		TabID:     1,
		PageURL:   "https://example.com/valid",
		Timestamp: 1000,
	})

	// Inject an expired session with a newer timestamp
	store.mu.Lock()
	store.sessions[2] = &annotationSessionEntry{
		Session: &AnnotationSession{
			TabID:     2,
			PageURL:   "https://example.com/expired-but-newer",
			Timestamp: 5000,
		},
		ExpiresAt: time.Now().Add(-1 * time.Second),
	}
	store.mu.Unlock()

	got := store.GetLatestSession()
	if got == nil {
		t.Fatal("expected a session, got nil")
	}
	if got.TabID != 1 {
		t.Errorf("expected tab 1 (expired tab 2 should be skipped), got tab %d", got.TabID)
	}
}

// --- Additional coverage: GetNamedSession returns nil for expired ---

func TestAnnotationStore_GetNamedSession_Expired(t *testing.T) {
	store := NewAnnotationStore(10 * time.Minute)
	defer store.Close()

	// Inject an expired named session
	store.mu.Lock()
	store.named["old-session"] = &namedSessionEntry{
		Session: &NamedAnnotationSession{
			Name:  "old-session",
			Pages: []*AnnotationSession{{TabID: 1, PageURL: "https://old.com"}},
		},
		ExpiresAt: time.Now().Add(-1 * time.Second),
	}
	store.mu.Unlock()

	ns := store.GetNamedSession("old-session")
	if ns != nil {
		t.Error("expected nil for expired named session")
	}
}

// --- Additional coverage: GetNamedSession returns a copy, not reference ---

func TestAnnotationStore_GetNamedSession_ReturnsCopy(t *testing.T) {
	store := NewAnnotationStore(10 * time.Minute)
	defer store.Close()

	store.AppendToNamedSession("copytest", &AnnotationSession{
		TabID:   1,
		PageURL: "https://example.com/page1",
	})

	copy1 := store.GetNamedSession("copytest")
	if copy1 == nil {
		t.Fatal("expected named session")
	}

	// Mutate the returned copy's Pages slice
	copy1.Pages = append(copy1.Pages, &AnnotationSession{
		TabID:   99,
		PageURL: "https://mutated.com",
	})

	// Get again and verify internal state was not affected
	copy2 := store.GetNamedSession("copytest")
	if copy2 == nil {
		t.Fatal("expected named session on second get")
	}
	if len(copy2.Pages) != 1 {
		t.Errorf("expected internal state to have 1 page, got %d (copy mutation leaked)", len(copy2.Pages))
	}
}

// --- Additional coverage: ListNamedSessions excludes expired ---

func TestAnnotationStore_ListNamedSessions_ExcludesExpired(t *testing.T) {
	store := NewAnnotationStore(10 * time.Minute)
	defer store.Close()

	store.AppendToNamedSession("active1", &AnnotationSession{TabID: 1})
	store.AppendToNamedSession("active2", &AnnotationSession{TabID: 2})

	// Inject an expired named session
	store.mu.Lock()
	store.named["expired-one"] = &namedSessionEntry{
		Session:   &NamedAnnotationSession{Name: "expired-one"},
		ExpiresAt: time.Now().Add(-1 * time.Second),
	}
	store.mu.Unlock()

	names := store.ListNamedSessions()
	nameSet := make(map[string]bool)
	for _, n := range names {
		nameSet[n] = true
	}

	if !nameSet["active1"] {
		t.Error("expected 'active1' in list")
	}
	if !nameSet["active2"] {
		t.Error("expected 'active2' in list")
	}
	if nameSet["expired-one"] {
		t.Error("expected 'expired-one' to be excluded (expired)")
	}
	if len(names) != 2 {
		t.Errorf("expected 2 names, got %d", len(names))
	}
}

// --- Additional coverage: ListNamedSessions on empty store ---

func TestAnnotationStore_ListNamedSessions_Empty(t *testing.T) {
	store := NewAnnotationStore(10 * time.Minute)
	defer store.Close()

	names := store.ListNamedSessions()
	if names == nil {
		t.Error("expected non-nil empty slice")
	}
	if len(names) != 0 {
		t.Errorf("expected 0 names, got %d", len(names))
	}
}

// --- Additional coverage: ClearNamedSession on nonexistent does not panic ---

func TestAnnotationStore_ClearNamedSession_Nonexistent(t *testing.T) {
	store := NewAnnotationStore(10 * time.Minute)
	defer store.Close()

	// Should not panic
	store.ClearNamedSession("nonexistent")

	// Verify still works
	store.AppendToNamedSession("after-clear", &AnnotationSession{TabID: 1})
	if store.GetNamedSession("after-clear") == nil {
		t.Error("expected store to work after clearing nonexistent session")
	}
}

// --- Additional coverage: WaitForSession ignores stale pre-mark session ---

func TestAnnotationStore_WaitForSession_IgnoresStaleSession(t *testing.T) {
	store := NewAnnotationStore(10 * time.Minute)
	defer store.Close()

	// Store a session BEFORE MarkDrawStarted
	store.StoreSession(1, &AnnotationSession{
		TabID:     1,
		Timestamp: time.Now().UnixMilli(),
	})

	time.Sleep(2 * time.Millisecond)
	store.MarkDrawStarted()

	// WaitForSession should NOT return the stale session
	session, timedOut := store.WaitForSession(50 * time.Millisecond)
	if session != nil {
		t.Error("expected nil session (stale session before MarkDrawStarted)")
	}
	if !timedOut {
		t.Error("expected timeout since no new session was stored")
	}
}

// --- Additional coverage: evictExpiredEntries cleans all three maps ---

func TestAnnotationStore_EvictExpiredEntries(t *testing.T) {
	store := NewAnnotationStore(10 * time.Minute)
	defer store.Close()

	// Inject expired entries in all three maps
	store.mu.Lock()
	store.sessions[100] = &annotationSessionEntry{
		Session:   &AnnotationSession{TabID: 100, Timestamp: 1},
		ExpiresAt: time.Now().Add(-1 * time.Second),
	}
	store.details["expired-detail"] = &annotationDetailEntry{
		Detail:    AnnotationDetail{CorrelationID: "expired-detail"},
		ExpiresAt: time.Now().Add(-1 * time.Second),
	}
	store.named["expired-named"] = &namedSessionEntry{
		Session:   &NamedAnnotationSession{Name: "expired-named"},
		ExpiresAt: time.Now().Add(-1 * time.Second),
	}
	// Also add non-expired entries
	store.sessions[200] = &annotationSessionEntry{
		Session:   &AnnotationSession{TabID: 200, Timestamp: 2},
		ExpiresAt: time.Now().Add(10 * time.Minute),
	}
	store.details["valid-detail"] = &annotationDetailEntry{
		Detail:    AnnotationDetail{CorrelationID: "valid-detail"},
		ExpiresAt: time.Now().Add(10 * time.Minute),
	}
	store.named["valid-named"] = &namedSessionEntry{
		Session:   &NamedAnnotationSession{Name: "valid-named"},
		ExpiresAt: time.Now().Add(10 * time.Minute),
	}
	store.mu.Unlock()

	// Manually trigger eviction
	store.evictExpiredEntries()

	// Verify expired entries are gone
	if store.GetSession(100) != nil {
		t.Error("expected expired session to be evicted")
	}
	if _, found := store.GetDetail("expired-detail"); found {
		t.Error("expected expired detail to be evicted")
	}
	if store.GetNamedSession("expired-named") != nil {
		t.Error("expected expired named session to be evicted")
	}

	// Verify valid entries remain
	if store.GetSession(200) == nil {
		t.Error("expected valid session to remain")
	}
	if _, found := store.GetDetail("valid-detail"); !found {
		t.Error("expected valid detail to remain")
	}
	if store.GetNamedSession("valid-named") == nil {
		t.Error("expected valid named session to remain")
	}
}

// --- Additional coverage: StoreDetail overwrites existing ---

func TestAnnotationStore_StoreDetail_Overwrite(t *testing.T) {
	store := NewAnnotationStore(10 * time.Minute)
	defer store.Close()

	store.StoreDetail("corr1", AnnotationDetail{
		CorrelationID: "corr1",
		Selector:      "div.old",
	})
	store.StoreDetail("corr1", AnnotationDetail{
		CorrelationID: "corr1",
		Selector:      "div.new",
	})

	got, found := store.GetDetail("corr1")
	if !found {
		t.Fatal("expected to find detail")
	}
	if got.Selector != "div.new" {
		t.Errorf("expected overwritten selector 'div.new', got %q", got.Selector)
	}
}

// --- Additional coverage: Multiple evictions when storing 2x max ---

func TestAnnotationStore_MultipleEvictions(t *testing.T) {
	store := NewAnnotationStore(10 * time.Minute)
	defer store.Close()

	total := maxSessions * 2
	for i := 1; i <= total; i++ {
		store.StoreSession(i, &AnnotationSession{
			TabID:     i,
			Timestamp: int64(i),
		})
	}

	store.mu.RLock()
	count := len(store.sessions)
	store.mu.RUnlock()

	if count != maxSessions {
		t.Errorf("expected %d sessions after multiple evictions, got %d", maxSessions, count)
	}

	// The oldest half should be evicted
	for i := 1; i <= maxSessions; i++ {
		if store.GetSession(i) != nil {
			t.Errorf("expected tab %d to be evicted", i)
		}
	}

	// The newest maxSessions should remain
	for i := maxSessions + 1; i <= total; i++ {
		if store.GetSession(i) == nil {
			t.Errorf("expected tab %d to still exist", i)
		}
	}
}

// --- Additional coverage: AppendToNamedSession updates TTL ---

func TestAnnotationStore_AppendToNamedSession_UpdatesTTL(t *testing.T) {
	store := NewAnnotationStore(10 * time.Minute)
	defer store.Close()

	store.AppendToNamedSession("ttl-test", &AnnotationSession{TabID: 1})

	store.mu.RLock()
	firstExpiry := store.named["ttl-test"].ExpiresAt
	store.mu.RUnlock()

	time.Sleep(2 * time.Millisecond)

	store.AppendToNamedSession("ttl-test", &AnnotationSession{TabID: 2})

	store.mu.RLock()
	secondExpiry := store.named["ttl-test"].ExpiresAt
	store.mu.RUnlock()

	if !secondExpiry.After(firstExpiry) {
		t.Error("expected TTL to be refreshed after append")
	}
}

// --- Additional coverage: Concurrent read/write with named sessions ---

func TestAnnotationStore_ConcurrentNamedSessions(t *testing.T) {
	store := NewAnnotationStore(10 * time.Minute)
	defer store.Close()

	const goroutines = 20
	const iterations = 50
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for g := 0; g < goroutines; g++ {
		go func(id int) {
			defer wg.Done()
			sessionName := fmt.Sprintf("concurrent-%d", id)
			for i := 0; i < iterations; i++ {
				store.AppendToNamedSession(sessionName, &AnnotationSession{
					TabID:     id*iterations + i,
					Timestamp: int64(id*iterations + i),
				})
				store.GetNamedSession(sessionName)
				store.ListNamedSessions()
			}
		}(g)
	}
	wg.Wait()
	// If we get here without panic or data race, the test passes
}

// --- Additional coverage: GetSession with expired entry injected directly ---

func TestAnnotationStore_GetSession_Expired_Direct(t *testing.T) {
	store := NewAnnotationStore(10 * time.Minute)
	defer store.Close()

	store.mu.Lock()
	store.sessions[5] = &annotationSessionEntry{
		Session:   &AnnotationSession{TabID: 5, PageURL: "https://expired.com", Timestamp: 1},
		ExpiresAt: time.Now().Add(-1 * time.Second),
	}
	store.mu.Unlock()

	got := store.GetSession(5)
	if got != nil {
		t.Error("expected nil for expired session")
	}
}

// --- Additional coverage: GetLatestSession with multiple tabs ---

func TestAnnotationStore_GetLatestSession_MultipleTabs(t *testing.T) {
	store := NewAnnotationStore(10 * time.Minute)
	defer store.Close()

	for i := 0; i < 10; i++ {
		store.StoreSession(i, &AnnotationSession{
			TabID:     i,
			Timestamp: int64(i * 100),
		})
	}

	got := store.GetLatestSession()
	if got == nil {
		t.Fatal("expected session")
	}
	if got.TabID != 9 {
		t.Errorf("expected tab 9 (highest timestamp), got tab %d", got.TabID)
	}
}

// --- Additional coverage: All expired in GetLatestSession returns nil ---

func TestAnnotationStore_GetLatestSession_AllExpired(t *testing.T) {
	store := NewAnnotationStore(10 * time.Minute)
	defer store.Close()

	store.mu.Lock()
	for i := 0; i < 5; i++ {
		store.sessions[i] = &annotationSessionEntry{
			Session:   &AnnotationSession{TabID: i, Timestamp: int64(i * 100)},
			ExpiresAt: time.Now().Add(-1 * time.Second),
		}
	}
	store.mu.Unlock()

	got := store.GetLatestSession()
	if got != nil {
		t.Error("expected nil when all sessions are expired")
	}
}

// --- Additional coverage: WaitForNamedSession returns named session ---

func TestAnnotationStore_WaitForNamedSession_Returns(t *testing.T) {
	store := NewAnnotationStore(10 * time.Minute)
	defer store.Close()

	store.MarkDrawStarted()

	go func() {
		time.Sleep(20 * time.Millisecond)
		store.AppendToNamedSession("wait-test", &AnnotationSession{
			TabID:       1,
			PageURL:     "https://example.com/waited",
			Timestamp:   time.Now().UnixMilli(),
			Annotations: []Annotation{{Text: "waited"}},
		})
	}()

	ns, timedOut := store.WaitForNamedSession("wait-test", 2*time.Second)
	if timedOut {
		t.Fatal("expected named session but got timeout")
	}
	if ns == nil {
		t.Fatal("expected named session, got nil")
	}
	if ns.Name != "wait-test" {
		t.Errorf("expected name 'wait-test', got %q", ns.Name)
	}
	if len(ns.Pages) != 1 {
		t.Fatalf("expected 1 page, got %d", len(ns.Pages))
	}
}

// --- Additional coverage: Close unblocks WaitForSession with timedOut=false ---

func TestAnnotationStore_Close_UnblocksWait(t *testing.T) {
	store := NewAnnotationStore(10 * time.Minute)

	store.MarkDrawStarted()

	go func() {
		time.Sleep(20 * time.Millisecond)
		store.Close()
	}()

	session, timedOut := store.WaitForSession(5 * time.Second)
	if session != nil {
		t.Error("expected nil session after close")
	}
	// When store is closed, waitForCondition returns (nil, false)
	if timedOut {
		t.Error("expected timedOut=false after close (done channel)")
	}
}

// --- parseAnnotations edge case tests ---

func TestParseAnnotations_EmptyText(t *testing.T) {
	raw := []json.RawMessage{
		json.RawMessage(`{"id":"a1","text":"","rect":{"x":10,"y":20,"width":100,"height":50}}`),
	}
	annotations, warnings := parseAnnotations(raw)

	if len(warnings) != 0 {
		t.Errorf("expected no warnings for empty text, got %v", warnings)
	}
	if len(annotations) != 1 {
		t.Fatalf("expected 1 annotation, got %d", len(annotations))
	}
	if annotations[0].Text != "" {
		t.Errorf("expected empty text, got %q", annotations[0].Text)
	}
	if annotations[0].ID != "a1" {
		t.Errorf("expected id 'a1', got %q", annotations[0].ID)
	}
}

func TestParseAnnotations_EmptyID(t *testing.T) {
	raw := []json.RawMessage{
		json.RawMessage(`{"id":"","text":"some text","rect":{"x":0,"y":0,"width":50,"height":50}}`),
	}
	annotations, warnings := parseAnnotations(raw)

	if len(warnings) != 0 {
		t.Errorf("expected no warnings for empty ID, got %v", warnings)
	}
	if len(annotations) != 1 {
		t.Fatalf("expected 1 annotation, got %d", len(annotations))
	}
	if annotations[0].ID != "" {
		t.Errorf("expected empty ID, got %q", annotations[0].ID)
	}
}

func TestParseAnnotations_ZeroRect(t *testing.T) {
	raw := []json.RawMessage{
		json.RawMessage(`{"id":"a1","text":"zero rect","rect":{"x":0,"y":0,"width":0,"height":0}}`),
	}
	annotations, warnings := parseAnnotations(raw)

	if len(warnings) != 0 {
		t.Errorf("expected no warnings for zero rect, got %v", warnings)
	}
	if len(annotations) != 1 {
		t.Fatalf("expected 1 annotation, got %d", len(annotations))
	}
	if annotations[0].Rect.Width != 0 || annotations[0].Rect.Height != 0 {
		t.Errorf("expected zero width/height, got %+v", annotations[0].Rect)
	}
}

func TestParseAnnotations_MixedValidAndInvalid(t *testing.T) {
	raw := []json.RawMessage{
		json.RawMessage(`{"id":"good","text":"valid annotation","rect":{"x":10,"y":20,"width":30,"height":40}}`),
		json.RawMessage(`not-valid-json`),
		json.RawMessage(`{"id":"also-good","text":"another valid","rect":{"x":0,"y":0,"width":10,"height":10}}`),
	}
	annotations, warnings := parseAnnotations(raw)

	if len(annotations) != 2 {
		t.Errorf("expected 2 valid annotations, got %d", len(annotations))
	}
	if len(warnings) != 1 {
		t.Errorf("expected 1 warning, got %d: %v", len(warnings), warnings)
	}
}

func TestParseAnnotations_AllInvalid(t *testing.T) {
	raw := []json.RawMessage{
		json.RawMessage(`invalid-1`),
		json.RawMessage(`invalid-2`),
	}
	annotations, warnings := parseAnnotations(raw)

	if len(annotations) != 0 {
		t.Errorf("expected 0 annotations, got %d", len(annotations))
	}
	if len(warnings) != 2 {
		t.Errorf("expected 2 warnings, got %d", len(warnings))
	}
}

func TestParseAnnotations_EmptyInput(t *testing.T) {
	annotations, warnings := parseAnnotations(nil)
	if len(annotations) != 0 {
		t.Errorf("expected 0 annotations for nil input, got %d", len(annotations))
	}
	if warnings != nil {
		t.Errorf("expected nil warnings for nil input, got %v", warnings)
	}
}

func TestParseAnnotations_EmptySlice(t *testing.T) {
	annotations, warnings := parseAnnotations([]json.RawMessage{})
	if len(annotations) != 0 {
		t.Errorf("expected 0 annotations for empty slice, got %d", len(annotations))
	}
	if warnings != nil {
		t.Errorf("expected nil warnings for empty slice, got %v", warnings)
	}
}

func TestParseAnnotations_FullAnnotationFields(t *testing.T) {
	raw := []json.RawMessage{
		json.RawMessage(`{
			"id": "ann_full",
			"text": "make this darker",
			"element_summary": "button.primary 'Submit'",
			"correlation_id": "detail_full",
			"rect": {"x": 100, "y": 200, "width": 150, "height": 50},
			"page_url": "https://example.com",
			"timestamp": 1700000000000
		}`),
	}
	annotations, warnings := parseAnnotations(raw)

	if len(warnings) != 0 {
		t.Errorf("expected no warnings, got %v", warnings)
	}
	if len(annotations) != 1 {
		t.Fatalf("expected 1 annotation, got %d", len(annotations))
	}

	ann := annotations[0]
	if ann.ID != "ann_full" {
		t.Errorf("expected ID 'ann_full', got %q", ann.ID)
	}
	if ann.Text != "make this darker" {
		t.Errorf("expected text, got %q", ann.Text)
	}
	if ann.ElementSummary != "button.primary 'Submit'" {
		t.Errorf("expected element summary, got %q", ann.ElementSummary)
	}
	if ann.CorrelationID != "detail_full" {
		t.Errorf("expected correlation ID, got %q", ann.CorrelationID)
	}
	if ann.Rect.X != 100 || ann.Rect.Y != 200 || ann.Rect.Width != 150 || ann.Rect.Height != 50 {
		t.Errorf("unexpected rect: %+v", ann.Rect)
	}
	if ann.PageURL != "https://example.com" {
		t.Errorf("expected page URL, got %q", ann.PageURL)
	}
	if ann.Timestamp != 1700000000000 {
		t.Errorf("expected timestamp, got %d", ann.Timestamp)
	}
}

func TestParseAnnotations_NegativeRectValues(t *testing.T) {
	raw := []json.RawMessage{
		json.RawMessage(`{"id":"neg","text":"negative","rect":{"x":-10,"y":-20,"width":-5,"height":-5}}`),
	}
	annotations, warnings := parseAnnotations(raw)

	if len(warnings) != 0 {
		t.Errorf("expected no warnings for negative rect, got %v", warnings)
	}
	if len(annotations) != 1 {
		t.Fatalf("expected 1 annotation, got %d", len(annotations))
	}
	if annotations[0].Rect.X != -10 {
		t.Errorf("expected x=-10, got %f", annotations[0].Rect.X)
	}
}

func TestParseAnnotations_MinimalValidJSON(t *testing.T) {
	// An empty JSON object should parse successfully (all fields zero-valued)
	raw := []json.RawMessage{
		json.RawMessage(`{}`),
	}
	annotations, warnings := parseAnnotations(raw)

	if len(warnings) != 0 {
		t.Errorf("expected no warnings for empty object, got %v", warnings)
	}
	if len(annotations) != 1 {
		t.Fatalf("expected 1 annotation, got %d", len(annotations))
	}
	if annotations[0].ID != "" || annotations[0].Text != "" {
		t.Errorf("expected empty fields, got ID=%q Text=%q", annotations[0].ID, annotations[0].Text)
	}
}

// --- persistDrawSession edge cases ---

func TestPersistDrawSession_WithSessionName(t *testing.T) {
	body := &drawModeRequest{
		PageURL:     "https://example.com",
		TabID:       42,
		SessionName: "qa-review",
	}
	annotations := []Annotation{
		{ID: "a1", Text: "fix this"},
	}

	// persistDrawSession writes to screenshotsDir; just ensure it does not panic
	persistDrawSession(body, "/tmp/test.png", annotations)
}

func TestPersistDrawSession_WithoutSessionName(t *testing.T) {
	body := &drawModeRequest{
		PageURL: "https://example.com",
		TabID:   43,
	}
	annotations := []Annotation{
		{ID: "a1", Text: "fix that"},
	}

	// persistDrawSession writes to screenshotsDir; just ensure it does not panic
	persistDrawSession(body, "/tmp/test2.png", annotations)
}

func TestPersistDrawSession_EmptyAnnotations(t *testing.T) {
	body := &drawModeRequest{
		PageURL: "https://example.com",
		TabID:   44,
	}

	// Should not panic with empty annotations
	persistDrawSession(body, "", []Annotation{})
}

// --- storeElementDetails edge cases ---

func TestStoreElementDetails_MultipleDetails(t *testing.T) {
	// Save and restore the global store
	oldStore := globalAnnotationStore
	globalAnnotationStore = NewAnnotationStore(10 * time.Minute)
	defer func() {
		globalAnnotationStore.Close()
		globalAnnotationStore = oldStore
	}()

	details := map[string]json.RawMessage{
		"d1": json.RawMessage(`{"selector":"div.a","tag":"div","text_content":"A"}`),
		"d2": json.RawMessage(`{"selector":"span.b","tag":"span","text_content":"B"}`),
	}
	storeElementDetails(details)

	got1, found1 := globalAnnotationStore.GetDetail("d1")
	if !found1 || got1.Selector != "div.a" {
		t.Errorf("expected detail d1 with selector 'div.a', found=%v got=%+v", found1, got1)
	}
	got2, found2 := globalAnnotationStore.GetDetail("d2")
	if !found2 || got2.Selector != "span.b" {
		t.Errorf("expected detail d2 with selector 'span.b', found=%v got=%+v", found2, got2)
	}
	// CorrelationID should be set from the map key
	if got1.CorrelationID != "d1" {
		t.Errorf("expected correlationID 'd1', got %q", got1.CorrelationID)
	}
}

func TestStoreElementDetails_InvalidJSON(t *testing.T) {
	oldStore := globalAnnotationStore
	globalAnnotationStore = NewAnnotationStore(10 * time.Minute)
	defer func() {
		globalAnnotationStore.Close()
		globalAnnotationStore = oldStore
	}()

	details := map[string]json.RawMessage{
		"bad": json.RawMessage(`not-valid-json`),
	}
	// Should not panic; invalid JSON is silently ignored
	storeElementDetails(details)

	_, found := globalAnnotationStore.GetDetail("bad")
	if found {
		t.Error("expected invalid JSON detail to not be stored")
	}
}

func TestStoreElementDetails_Empty(t *testing.T) {
	oldStore := globalAnnotationStore
	globalAnnotationStore = NewAnnotationStore(10 * time.Minute)
	defer func() {
		globalAnnotationStore.Close()
		globalAnnotationStore = oldStore
	}()

	// Should not panic with empty map
	storeElementDetails(map[string]json.RawMessage{})

	// Should not panic with nil map
	storeElementDetails(nil)
}

// --- storeAnnotationSession edge cases ---

func TestStoreAnnotationSession_WithSessionName(t *testing.T) {
	oldStore := globalAnnotationStore
	globalAnnotationStore = NewAnnotationStore(10 * time.Minute)
	defer func() {
		globalAnnotationStore.Close()
		globalAnnotationStore = oldStore
	}()

	body := &drawModeRequest{
		PageURL:     "https://example.com",
		TabID:       50,
		SessionName: "named-test",
	}
	annotations := []Annotation{{ID: "a1", Text: "test"}}

	storeAnnotationSession(body, "/tmp/ss.png", annotations)

	// Should be stored in both anonymous and named
	session := globalAnnotationStore.GetSession(50)
	if session == nil {
		t.Fatal("expected session in anonymous store")
	}
	if session.ScreenshotPath != "/tmp/ss.png" {
		t.Errorf("expected screenshot path, got %q", session.ScreenshotPath)
	}

	ns := globalAnnotationStore.GetNamedSession("named-test")
	if ns == nil {
		t.Fatal("expected named session")
	}
	if len(ns.Pages) != 1 {
		t.Fatalf("expected 1 page in named session, got %d", len(ns.Pages))
	}
}

func TestStoreAnnotationSession_WithoutSessionName(t *testing.T) {
	oldStore := globalAnnotationStore
	globalAnnotationStore = NewAnnotationStore(10 * time.Minute)
	defer func() {
		globalAnnotationStore.Close()
		globalAnnotationStore = oldStore
	}()

	body := &drawModeRequest{
		PageURL: "https://example.com",
		TabID:   51,
	}
	annotations := []Annotation{{ID: "a1", Text: "test"}}

	storeAnnotationSession(body, "", annotations)

	session := globalAnnotationStore.GetSession(51)
	if session == nil {
		t.Fatal("expected session in anonymous store")
	}

	// With empty session name, no named session should be created
	names := globalAnnotationStore.ListNamedSessions()
	if len(names) != 0 {
		t.Errorf("expected no named sessions, got %v", names)
	}
}

func TestAnnotationStore_DetailEvictionCap(t *testing.T) {
	store := NewAnnotationStore(10 * time.Minute)
	defer store.Close()

	// Store maxDetails + 10 entries
	for i := 0; i < maxDetails+10; i++ {
		store.StoreDetail(fmt.Sprintf("detail-%d", i), AnnotationDetail{
			CorrelationID: fmt.Sprintf("detail-%d", i),
			Selector:      fmt.Sprintf("div.item-%d", i),
		})
	}

	// Count should never exceed maxDetails + 1 (at most one over before eviction)
	store.mu.RLock()
	count := len(store.details)
	store.mu.RUnlock()

	if count > maxDetails+1 {
		t.Errorf("expected detail count <= %d, got %d", maxDetails+1, count)
	}

	// The latest entries should still be retrievable
	_, found := store.GetDetail(fmt.Sprintf("detail-%d", maxDetails+9))
	if !found {
		t.Error("expected latest detail entry to exist")
	}
}
