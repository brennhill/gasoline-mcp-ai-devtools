// annotation_store_test.go — Tests for the annotation store.
package main

import (
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
