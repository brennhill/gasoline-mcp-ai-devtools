// store_test.go — Tests for the annotation store.
package annotation

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestStore_StoreAndGetSession(t *testing.T) {
	store := NewStore(10 * time.Minute)
	defer store.Close()

	session := &Session{
		Annotations: []Annotation{
			{
				ID:             "ann_1",
				Rect:           Rect{X: 100, Y: 200, Width: 150, Height: 50},
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
	if got.Annotations[0].ID != "ann_1" {
		t.Errorf("expected annotation ID 'ann_1', got %q", got.Annotations[0].ID)
	}
	if got.Annotations[0].CorrelationID != "detail_1" {
		t.Errorf("expected correlation ID 'detail_1', got %q", got.Annotations[0].CorrelationID)
	}
	if got.Annotations[0].ElementSummary != "button.primary 'Submit'" {
		t.Errorf("expected element summary, got %q", got.Annotations[0].ElementSummary)
	}
	if got.Annotations[0].PageURL != "https://example.com" {
		t.Errorf("expected page URL 'https://example.com', got %q", got.Annotations[0].PageURL)
	}
	if got.Annotations[0].Rect.X != 100 || got.Annotations[0].Rect.Y != 200 || got.Annotations[0].Rect.Width != 150 || got.Annotations[0].Rect.Height != 50 {
		t.Errorf("expected rect {100 200 150 50}, got %+v", got.Annotations[0].Rect)
	}
	if got.ScreenshotPath != "/tmp/draw_test.png" {
		t.Errorf("expected screenshot path, got %q", got.ScreenshotPath)
	}
	if got.PageURL != "https://example.com" {
		t.Errorf("expected session page URL 'https://example.com', got %q", got.PageURL)
	}
	if got.TabID != 42 {
		t.Errorf("expected tab ID 42, got %d", got.TabID)
	}
}

func TestStore_GetSessionNotFound(t *testing.T) {
	store := NewStore(10 * time.Minute)
	defer store.Close()
	got := store.GetSession(999)
	if got != nil {
		t.Errorf("expected nil for non-existent session, got %+v", got)
	}
}

func TestStore_SessionOverwrite(t *testing.T) {
	store := NewStore(10 * time.Minute)
	defer store.Close()

	session1 := &Session{
		Annotations: []Annotation{{Text: "first"}},
		TabID:       42,
		Timestamp:   100,
	}
	session2 := &Session{
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
	if got.Annotations[1].Text != "third" {
		t.Errorf("expected text 'third', got %q", got.Annotations[1].Text)
	}
	if got.Timestamp != 200 {
		t.Errorf("expected timestamp 200 after overwrite, got %d", got.Timestamp)
	}
}

func TestStore_GetLatestSession(t *testing.T) {
	store := NewStore(10 * time.Minute)
	defer store.Close()

	store.StoreSession(1, &Session{TabID: 1, Timestamp: 100, Annotations: []Annotation{{Text: "tab1"}}})
	store.StoreSession(2, &Session{TabID: 2, Timestamp: 300, Annotations: []Annotation{{Text: "tab2"}}})
	store.StoreSession(3, &Session{TabID: 3, Timestamp: 200, Annotations: []Annotation{{Text: "tab3"}}})

	latest := store.GetLatestSession()
	if latest == nil {
		t.Fatal("expected latest session, got nil")
	}
	if latest.TabID != 2 {
		t.Errorf("expected latest tab 2, got %d", latest.TabID)
	}
	if latest.Timestamp != 300 {
		t.Errorf("expected latest timestamp 300, got %d", latest.Timestamp)
	}
	if len(latest.Annotations) != 1 || latest.Annotations[0].Text != "tab2" {
		t.Errorf("expected annotation text 'tab2', got %+v", latest.Annotations)
	}
}

func TestStore_GetLatestSessionEmpty(t *testing.T) {
	store := NewStore(10 * time.Minute)
	defer store.Close()
	latest := store.GetLatestSession()
	if latest != nil {
		t.Errorf("expected nil for empty store, got %+v", latest)
	}
}

func TestStore_StoreAndGetDetail(t *testing.T) {
	store := NewStore(10 * time.Minute)
	defer store.Close()

	detail := Detail{
		CorrelationID:  "detail_1",
		Selector:       "button.primary",
		Tag:            "button",
		TextContent:    "Submit",
		Classes:        []string{"primary", "rounded"},
		ID:             "submit-btn",
		ComputedStyles: map[string]string{"background-color": "rgb(59, 130, 246)"},
		ParentSelector: "form.checkout > div.actions",
		BoundingRect:   Rect{X: 100, Y: 200, Width: 150, Height: 50},
	}

	store.StoreDetail("detail_1", detail)

	got, found := store.GetDetail("detail_1")
	if !found {
		t.Fatal("expected to find detail")
	}
	if got.CorrelationID != "detail_1" {
		t.Errorf("expected correlation ID 'detail_1', got %q", got.CorrelationID)
	}
	if got.Selector != "button.primary" {
		t.Errorf("expected selector 'button.primary', got %q", got.Selector)
	}
	if got.Tag != "button" {
		t.Errorf("expected tag 'button', got %q", got.Tag)
	}
	if got.TextContent != "Submit" {
		t.Errorf("expected text content 'Submit', got %q", got.TextContent)
	}
	if len(got.Classes) != 2 || got.Classes[0] != "primary" || got.Classes[1] != "rounded" {
		t.Errorf("expected classes [primary rounded], got %v", got.Classes)
	}
	if got.ID != "submit-btn" {
		t.Errorf("expected ID 'submit-btn', got %q", got.ID)
	}
	if got.ComputedStyles["background-color"] != "rgb(59, 130, 246)" {
		t.Errorf("expected background-color 'rgb(59, 130, 246)', got %q", got.ComputedStyles["background-color"])
	}
	if got.ParentSelector != "form.checkout > div.actions" {
		t.Errorf("expected parent selector 'form.checkout > div.actions', got %q", got.ParentSelector)
	}
	if got.BoundingRect.X != 100 || got.BoundingRect.Y != 200 || got.BoundingRect.Width != 150 || got.BoundingRect.Height != 50 {
		t.Errorf("expected bounding rect {100 200 150 50}, got %+v", got.BoundingRect)
	}
}

func TestStore_DetailNotFound(t *testing.T) {
	store := NewStore(10 * time.Minute)
	defer store.Close()
	_, found := store.GetDetail("nonexistent")
	if found {
		t.Error("expected not found for non-existent detail")
	}
}

func TestStore_DetailExpired(t *testing.T) {
	// Use very short TTL
	store := NewStore(1 * time.Millisecond)
	defer store.Close()

	store.StoreDetail("expire_test", Detail{Selector: "div.test"})

	// Wait for expiration
	time.Sleep(5 * time.Millisecond)

	_, found := store.GetDetail("expire_test")
	if found {
		t.Error("expected detail to be expired")
	}
}

func TestStore_ZeroAnnotations(t *testing.T) {
	store := NewStore(10 * time.Minute)
	defer store.Close()

	session := &Session{
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
	if got.ScreenshotPath != "/tmp/empty.png" {
		t.Errorf("expected screenshot path '/tmp/empty.png', got %q", got.ScreenshotPath)
	}
	if got.TabID != 42 {
		t.Errorf("expected tab ID 42, got %d", got.TabID)
	}
}

func TestStore_ConcurrentAccess(t *testing.T) {
	store := NewStore(10 * time.Minute)
	defer store.Close()

	var wg sync.WaitGroup
	// Concurrent session writes
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(tabID int) {
			defer wg.Done()
			store.StoreSession(tabID, &Session{
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
			store.StoreDetail(fmt.Sprintf("detail_%d", id), Detail{
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

	// Verify all sessions were stored (writes all complete before reads in the WaitGroup)
	found := 0
	for i := 0; i < 50; i++ {
		s := store.GetSession(i)
		if s != nil {
			found++
			if s.TabID != i {
				t.Errorf("session %d: TabID = %d, want %d", i, s.TabID, i)
			}
			if len(s.Annotations) != 1 {
				t.Errorf("session %d: annotation count = %d, want 1", i, len(s.Annotations))
			}
		}
	}
	if found != 50 {
		t.Errorf("Expected all 50 sessions to be stored after concurrent access, got %d", found)
	}

	// Verify all details were stored
	detailFound := 0
	for i := 0; i < 50; i++ {
		d, ok := store.GetDetail(fmt.Sprintf("detail_%d", i))
		if ok {
			detailFound++
			expectedSelector := fmt.Sprintf("div.item-%d", i)
			if d.Selector != expectedSelector {
				t.Errorf("detail_%d: selector = %q, want %q", i, d.Selector, expectedSelector)
			}
		}
	}
	if detailFound != 50 {
		t.Errorf("Expected all 50 details to be stored, got %d", detailFound)
	}
}

func TestStore_SessionEvictionCap(t *testing.T) {
	store := NewStore(10 * time.Minute)
	defer store.Close()

	// Store more sessions than MaxSessions (100)
	for i := 1; i <= 110; i++ {
		store.StoreSession(i, &Session{
			TabID:     i,
			Timestamp: int64(i),
			Annotations: []Annotation{{Text: fmt.Sprintf("session_%d", i)}},
		})
	}

	// Count surviving sessions (should be <= MaxSessions)
	count := 0
	for i := 1; i <= 110; i++ {
		if store.GetSession(i) != nil {
			count++
		}
	}
	if count > 100 {
		t.Errorf("Expected at most 100 sessions after eviction, got %d", count)
	}
	// The newest sessions should survive with correct data
	newest := store.GetSession(110)
	if newest == nil {
		t.Fatal("Expected newest session (110) to survive eviction")
	}
	if newest.TabID != 110 {
		t.Errorf("newest session TabID = %d, want 110", newest.TabID)
	}
	if newest.Timestamp != 110 {
		t.Errorf("newest session Timestamp = %d, want 110", newest.Timestamp)
	}
	if len(newest.Annotations) != 1 || newest.Annotations[0].Text != "session_110" {
		t.Errorf("newest session annotation = %+v, want text 'session_110'", newest.Annotations)
	}
}

func TestStore_MarkDrawStarted(t *testing.T) {
	store := NewStore(10 * time.Minute)
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

func TestStore_WaitForSession_ImmediateReturn(t *testing.T) {
	store := NewStore(10 * time.Minute)
	defer store.Close()

	// Mark draw started, then store a session with a newer timestamp
	store.MarkDrawStarted()
	time.Sleep(1 * time.Millisecond)
	store.StoreSession(1, &Session{
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
	if session.TabID != 1 {
		t.Errorf("expected TabID 1, got %d", session.TabID)
	}
	if len(session.Annotations) != 1 {
		t.Fatalf("expected 1 annotation, got %d", len(session.Annotations))
	}
	if session.Annotations[0].Text != "immediate" {
		t.Errorf("expected text 'immediate', got %q", session.Annotations[0].Text)
	}
}

func TestStore_WaitForSession_BlocksAndReturns(t *testing.T) {
	store := NewStore(10 * time.Minute)
	defer store.Close()

	store.MarkDrawStarted()

	// Store session in a goroutine after a delay
	go func() {
		time.Sleep(50 * time.Millisecond)
		store.StoreSession(1, &Session{
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
	if session.TabID != 1 {
		t.Errorf("expected TabID 1, got %d", session.TabID)
	}
	if len(session.Annotations) != 1 {
		t.Fatalf("expected 1 annotation, got %d", len(session.Annotations))
	}
	if session.Annotations[0].Text != "delayed" {
		t.Errorf("expected text 'delayed', got %q", session.Annotations[0].Text)
	}
	if elapsed < 30*time.Millisecond {
		t.Error("expected to have blocked for at least 30ms")
	}
}

func TestStore_WaitForSession_Timeout(t *testing.T) {
	store := NewStore(10 * time.Minute)
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

func TestStore_WaitForSession_SkipsStaleSession(t *testing.T) {
	store := NewStore(10 * time.Minute)
	defer store.Close()

	// Store an old session BEFORE marking draw started
	store.StoreSession(1, &Session{
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

func TestStore_WaitForSession_NoDrawStarted(t *testing.T) {
	store := NewStore(10 * time.Minute)
	defer store.Close()

	// Without MarkDrawStarted, lastDrawStartedAt is 0 — any session qualifies
	store.StoreSession(1, &Session{
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
	if session.TabID != 1 {
		t.Errorf("expected TabID 1, got %d", session.TabID)
	}
	if len(session.Annotations) != 1 || session.Annotations[0].Text != "any" {
		t.Errorf("expected annotation text 'any', got %+v", session.Annotations)
	}
}

func TestStore_WaitForSession_CloseUnblocks(t *testing.T) {
	store := NewStore(10 * time.Minute)

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

func TestStore_NamedSession_AppendAndGet(t *testing.T) {
	store := NewStore(10 * time.Minute)
	defer store.Close()

	page1 := &Session{
		TabID:       1,
		Timestamp:   100,
		PageURL:     "https://example.com/login",
		Annotations: []Annotation{{Text: "fix button"}},
	}
	page2 := &Session{
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
	if ns.Pages[0].Timestamp != 100 {
		t.Errorf("expected first page timestamp 100, got %d", ns.Pages[0].Timestamp)
	}
	if len(ns.Pages[0].Annotations) != 1 || ns.Pages[0].Annotations[0].Text != "fix button" {
		t.Errorf("expected first page annotation 'fix button', got %+v", ns.Pages[0].Annotations)
	}
	if ns.Pages[1].PageURL != "https://example.com/dashboard" {
		t.Errorf("expected second page URL, got %q", ns.Pages[1].PageURL)
	}
	if ns.Pages[1].Timestamp != 200 {
		t.Errorf("expected second page timestamp 200, got %d", ns.Pages[1].Timestamp)
	}
	if len(ns.Pages[1].Annotations) != 2 {
		t.Errorf("expected 2 annotations on page 2, got %d", len(ns.Pages[1].Annotations))
	}
	if ns.Pages[1].Annotations[0].Text != "wrong color" {
		t.Errorf("expected annotation 'wrong color', got %q", ns.Pages[1].Annotations[0].Text)
	}
	if ns.Pages[1].Annotations[1].Text != "misaligned" {
		t.Errorf("expected annotation 'misaligned', got %q", ns.Pages[1].Annotations[1].Text)
	}
}

func TestStore_NamedSession_NotFound(t *testing.T) {
	store := NewStore(10 * time.Minute)
	defer store.Close()

	ns := store.GetNamedSession("nonexistent")
	if ns != nil {
		t.Errorf("expected nil for non-existent named session, got %+v", ns)
	}
}

func TestStore_NamedSession_ListSessions(t *testing.T) {
	store := NewStore(10 * time.Minute)
	defer store.Close()

	store.AppendToNamedSession("review-1", &Session{TabID: 1, Timestamp: 100})
	store.AppendToNamedSession("review-2", &Session{TabID: 1, Timestamp: 200})

	names := store.ListNamedSessions()
	if len(names) != 2 {
		t.Fatalf("expected 2 named sessions, got %d", len(names))
	}
}

func TestStore_NamedSession_Clear(t *testing.T) {
	store := NewStore(10 * time.Minute)
	defer store.Close()

	store.AppendToNamedSession("qa", &Session{TabID: 1, Timestamp: 100})
	store.ClearNamedSession("qa")

	ns := store.GetNamedSession("qa")
	if ns != nil {
		t.Errorf("expected nil after clear, got %+v", ns)
	}
}

func TestStore_NamedSession_WaitBlocks(t *testing.T) {
	store := NewStore(10 * time.Minute)
	defer store.Close()

	store.MarkDrawStarted()

	go func() {
		time.Sleep(50 * time.Millisecond)
		store.AppendToNamedSession("qa", &Session{
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
	if ns.Name != "qa" {
		t.Errorf("expected name 'qa', got %q", ns.Name)
	}
	if len(ns.Pages) != 1 {
		t.Fatalf("expected 1 page, got %d", len(ns.Pages))
	}
	if len(ns.Pages[0].Annotations) != 1 || ns.Pages[0].Annotations[0].Text != "waited" {
		t.Errorf("expected annotation 'waited', got %+v", ns.Pages[0].Annotations)
	}
	if elapsed < 30*time.Millisecond {
		t.Error("expected to have blocked")
	}
}

func TestStore_NamedSession_WaitTimeout(t *testing.T) {
	store := NewStore(10 * time.Minute)
	defer store.Close()

	store.MarkDrawStarted()

	_, timedOut := store.WaitForNamedSession("qa", 50*time.Millisecond)
	if !timedOut {
		t.Error("expected timeout")
	}
}

func TestStore_NamedSession_EvictionCap(t *testing.T) {
	store := NewStore(10 * time.Minute)
	defer store.Close()

	// Fill up to MaxNamedSessions + 1 (51 total)
	for i := 0; i < 51; i++ {
		name := "session_" + strings.Repeat("0", 3-len(strconv.Itoa(i))) + strconv.Itoa(i) // zero-padded
		store.AppendToNamedSession(name, &Session{
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

	// The most recent session should still exist with correct data
	latest := store.GetNamedSession("session_050")
	if latest == nil {
		t.Fatal("expected session_050 to exist after eviction")
	}
	if latest.Name != "session_050" {
		t.Errorf("latest session name = %q, want 'session_050'", latest.Name)
	}
	if len(latest.Pages) != 1 {
		t.Errorf("expected 1 page in latest session, got %d", len(latest.Pages))
	}
	if len(latest.Pages) > 0 && len(latest.Pages[0].Annotations) > 0 {
		if latest.Pages[0].Annotations[0].Text != "annotation for session_050" {
			t.Errorf("latest annotation text = %q, want 'annotation for session_050'", latest.Pages[0].Annotations[0].Text)
		}
	}
}

// TestStore_WaitForSession_SpuriousWakeup verifies that WaitForSession
// loops correctly when a notify fires for an unrelated session (different tab).
func TestStore_SessionExpired(t *testing.T) {
	store := NewStore(10 * time.Minute)
	defer store.Close()
	// Override session TTL to something very short for testing
	store.sessionTTL = 50 * time.Millisecond

	store.StoreSession(1, &Session{
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

func TestStore_WaitForSession_SpuriousWakeup(t *testing.T) {
	store := NewStore(10 * time.Minute)
	defer store.Close()

	store.MarkDrawStarted()
	time.Sleep(5 * time.Millisecond)

	var result *Session
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
	store.sessions[99] = &sessionEntry{
		Session:   &Session{TabID: 99, Timestamp: 1},
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
	store.StoreSession(42, &Session{
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

// TestStore_WaitForNamedSession_SpuriousWakeup verifies that WaitForNamedSession
// loops correctly when a notify fires for a different session name.
func TestStore_WaitForNamedSession_SpuriousWakeup(t *testing.T) {
	store := NewStore(10 * time.Minute)
	defer store.Close()

	store.MarkDrawStarted()
	time.Sleep(5 * time.Millisecond)

	var result *NamedSession
	var timedOut bool
	done := make(chan struct{})

	go func() {
		result, timedOut = store.WaitForNamedSession("target", 2*time.Second)
		close(done)
	}()

	// Spurious wake: append to a DIFFERENT named session
	time.Sleep(50 * time.Millisecond)
	store.AppendToNamedSession("other", &Session{
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
	store.AppendToNamedSession("target", &Session{
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

func TestStore_Close(t *testing.T) {
	store := NewStore(10 * time.Minute)
	store.Close()
	// Double close should not panic (sync.Once protects)
	store.Close()
	// Store should still work after close (just no background cleanup)
	store.StoreSession(1, &Session{TabID: 1, Timestamp: 1})
	if store.GetSession(1) == nil {
		t.Error("Expected store to still work after Close")
	}
}

func TestStore_ConcurrentClose(t *testing.T) {
	store := NewStore(10 * time.Minute)
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

func TestStore_GetLatestSession_SkipsExpired(t *testing.T) {
	store := NewStore(10 * time.Minute)
	defer store.Close()

	// Store a valid session with an older timestamp
	store.StoreSession(1, &Session{
		TabID:     1,
		PageURL:   "https://example.com/valid",
		Timestamp: 1000,
	})

	// Inject an expired session with a newer timestamp
	store.mu.Lock()
	store.sessions[2] = &sessionEntry{
		Session: &Session{
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
	if got.PageURL != "https://example.com/valid" {
		t.Errorf("expected page URL 'https://example.com/valid', got %q", got.PageURL)
	}
	if got.Timestamp != 1000 {
		t.Errorf("expected timestamp 1000, got %d", got.Timestamp)
	}
}

// --- Additional coverage: GetNamedSession returns nil for expired ---

func TestStore_GetNamedSession_Expired(t *testing.T) {
	store := NewStore(10 * time.Minute)
	defer store.Close()

	// Inject an expired named session
	store.mu.Lock()
	store.named["old-session"] = &namedSessionEntry{
		Session: &NamedSession{
			Name:  "old-session",
			Pages: []*Session{{TabID: 1, PageURL: "https://old.com"}},
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

func TestStore_GetNamedSession_ReturnsCopy(t *testing.T) {
	store := NewStore(10 * time.Minute)
	defer store.Close()

	store.AppendToNamedSession("copytest", &Session{
		TabID:   1,
		PageURL: "https://example.com/page1",
	})

	copy1 := store.GetNamedSession("copytest")
	if copy1 == nil {
		t.Fatal("expected named session")
	}
	if copy1.Name != "copytest" {
		t.Errorf("expected name 'copytest', got %q", copy1.Name)
	}
	if len(copy1.Pages) != 1 {
		t.Fatalf("expected 1 page, got %d", len(copy1.Pages))
	}
	if copy1.Pages[0].PageURL != "https://example.com/page1" {
		t.Errorf("expected page URL 'https://example.com/page1', got %q", copy1.Pages[0].PageURL)
	}

	// Mutate the returned copy's Pages slice
	copy1.Pages = append(copy1.Pages, &Session{
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

func TestStore_ListNamedSessions_ExcludesExpired(t *testing.T) {
	store := NewStore(10 * time.Minute)
	defer store.Close()

	store.AppendToNamedSession("active1", &Session{TabID: 1})
	store.AppendToNamedSession("active2", &Session{TabID: 2})

	// Inject an expired named session
	store.mu.Lock()
	store.named["expired-one"] = &namedSessionEntry{
		Session:   &NamedSession{Name: "expired-one"},
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

func TestStore_ListNamedSessions_Empty(t *testing.T) {
	store := NewStore(10 * time.Minute)
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

func TestStore_ClearNamedSession_Nonexistent(t *testing.T) {
	store := NewStore(10 * time.Minute)
	defer store.Close()

	// Should not panic
	store.ClearNamedSession("nonexistent")

	// Verify still works
	store.AppendToNamedSession("after-clear", &Session{TabID: 1})
	if store.GetNamedSession("after-clear") == nil {
		t.Error("expected store to work after clearing nonexistent session")
	}
}

// --- Additional coverage: WaitForSession ignores stale pre-mark session ---

func TestStore_WaitForSession_IgnoresStaleSession(t *testing.T) {
	store := NewStore(10 * time.Minute)
	defer store.Close()

	// Store a session BEFORE MarkDrawStarted
	store.StoreSession(1, &Session{
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

func TestStore_EvictExpiredEntries(t *testing.T) {
	store := NewStore(10 * time.Minute)
	defer store.Close()

	// Inject expired entries in all three maps
	store.mu.Lock()
	store.sessions[100] = &sessionEntry{
		Session:   &Session{TabID: 100, Timestamp: 1},
		ExpiresAt: time.Now().Add(-1 * time.Second),
	}
	store.details["expired-detail"] = &detailEntry{
		Detail:    Detail{CorrelationID: "expired-detail"},
		ExpiresAt: time.Now().Add(-1 * time.Second),
	}
	store.named["expired-named"] = &namedSessionEntry{
		Session:   &NamedSession{Name: "expired-named"},
		ExpiresAt: time.Now().Add(-1 * time.Second),
	}
	// Also add non-expired entries
	store.sessions[200] = &sessionEntry{
		Session:   &Session{TabID: 200, Timestamp: 2},
		ExpiresAt: time.Now().Add(10 * time.Minute),
	}
	store.details["valid-detail"] = &detailEntry{
		Detail:    Detail{CorrelationID: "valid-detail"},
		ExpiresAt: time.Now().Add(10 * time.Minute),
	}
	store.named["valid-named"] = &namedSessionEntry{
		Session:   &NamedSession{Name: "valid-named"},
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

	// Verify valid entries remain with correct data
	validSession := store.GetSession(200)
	if validSession == nil {
		t.Fatal("expected valid session to remain")
	}
	if validSession.TabID != 200 {
		t.Errorf("valid session TabID = %d, want 200", validSession.TabID)
	}
	if validSession.Timestamp != 2 {
		t.Errorf("valid session Timestamp = %d, want 2", validSession.Timestamp)
	}

	validDetail, found := store.GetDetail("valid-detail")
	if !found {
		t.Fatal("expected valid detail to remain")
	}
	if validDetail.CorrelationID != "valid-detail" {
		t.Errorf("valid detail CorrelationID = %q, want 'valid-detail'", validDetail.CorrelationID)
	}

	validNamed := store.GetNamedSession("valid-named")
	if validNamed == nil {
		t.Fatal("expected valid named session to remain")
	}
	if validNamed.Name != "valid-named" {
		t.Errorf("valid named session Name = %q, want 'valid-named'", validNamed.Name)
	}
}

// --- Additional coverage: StoreDetail overwrites existing ---

func TestStore_StoreDetail_Overwrite(t *testing.T) {
	store := NewStore(10 * time.Minute)
	defer store.Close()

	store.StoreDetail("corr1", Detail{
		CorrelationID: "corr1",
		Selector:      "div.old",
	})
	store.StoreDetail("corr1", Detail{
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

func TestStore_MultipleEvictions(t *testing.T) {
	store := NewStore(10 * time.Minute)
	defer store.Close()

	total := MaxSessions * 2
	for i := 1; i <= total; i++ {
		store.StoreSession(i, &Session{
			TabID:     i,
			Timestamp: int64(i),
		})
	}

	store.mu.RLock()
	count := len(store.sessions)
	store.mu.RUnlock()

	if count != MaxSessions {
		t.Errorf("expected %d sessions after multiple evictions, got %d", MaxSessions, count)
	}

	// The oldest half should be evicted
	for i := 1; i <= MaxSessions; i++ {
		if store.GetSession(i) != nil {
			t.Errorf("expected tab %d to be evicted", i)
		}
	}

	// The newest MaxSessions should remain
	for i := MaxSessions + 1; i <= total; i++ {
		if store.GetSession(i) == nil {
			t.Errorf("expected tab %d to still exist", i)
		}
	}
}

// --- Additional coverage: AppendToNamedSession updates TTL ---

func TestStore_AppendToNamedSession_UpdatesTTL(t *testing.T) {
	store := NewStore(10 * time.Minute)
	defer store.Close()

	store.AppendToNamedSession("ttl-test", &Session{TabID: 1})

	store.mu.RLock()
	firstExpiry := store.named["ttl-test"].ExpiresAt
	store.mu.RUnlock()

	time.Sleep(2 * time.Millisecond)

	store.AppendToNamedSession("ttl-test", &Session{TabID: 2})

	store.mu.RLock()
	secondExpiry := store.named["ttl-test"].ExpiresAt
	store.mu.RUnlock()

	if !secondExpiry.After(firstExpiry) {
		t.Error("expected TTL to be refreshed after append")
	}
}

// --- Additional coverage: Concurrent read/write with named sessions ---

func TestStore_ConcurrentNamedSessions(t *testing.T) {
	store := NewStore(10 * time.Minute)
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
				store.AppendToNamedSession(sessionName, &Session{
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

func TestStore_GetSession_Expired_Direct(t *testing.T) {
	store := NewStore(10 * time.Minute)
	defer store.Close()

	store.mu.Lock()
	store.sessions[5] = &sessionEntry{
		Session:   &Session{TabID: 5, PageURL: "https://expired.com", Timestamp: 1},
		ExpiresAt: time.Now().Add(-1 * time.Second),
	}
	store.mu.Unlock()

	got := store.GetSession(5)
	if got != nil {
		t.Error("expected nil for expired session")
	}
}

// --- Additional coverage: GetLatestSession with multiple tabs ---

func TestStore_GetLatestSession_MultipleTabs(t *testing.T) {
	store := NewStore(10 * time.Minute)
	defer store.Close()

	for i := 0; i < 10; i++ {
		store.StoreSession(i, &Session{
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

func TestStore_GetLatestSession_AllExpired(t *testing.T) {
	store := NewStore(10 * time.Minute)
	defer store.Close()

	store.mu.Lock()
	for i := 0; i < 5; i++ {
		store.sessions[i] = &sessionEntry{
			Session:   &Session{TabID: i, Timestamp: int64(i * 100)},
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

func TestStore_WaitForNamedSession_Returns(t *testing.T) {
	store := NewStore(10 * time.Minute)
	defer store.Close()

	store.MarkDrawStarted()

	go func() {
		time.Sleep(20 * time.Millisecond)
		store.AppendToNamedSession("wait-test", &Session{
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
	if ns.Pages[0].PageURL != "https://example.com/waited" {
		t.Errorf("expected page URL 'https://example.com/waited', got %q", ns.Pages[0].PageURL)
	}
	if len(ns.Pages[0].Annotations) != 1 || ns.Pages[0].Annotations[0].Text != "waited" {
		t.Errorf("expected annotation text 'waited', got %+v", ns.Pages[0].Annotations)
	}
}

// --- Additional coverage: Close unblocks WaitForSession with timedOut=false ---

func TestStore_Close_UnblocksWait(t *testing.T) {
	store := NewStore(10 * time.Minute)

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

func TestStore_DetailEvictionCap(t *testing.T) {
	store := NewStore(10 * time.Minute)
	defer store.Close()

	// Store MaxDetails + 10 entries
	for i := 0; i < MaxDetails+10; i++ {
		store.StoreDetail(fmt.Sprintf("detail-%d", i), Detail{
			CorrelationID: fmt.Sprintf("detail-%d", i),
			Selector:      fmt.Sprintf("div.item-%d", i),
		})
	}

	// Count should never exceed MaxDetails + 1 (at most one over before eviction)
	store.mu.RLock()
	count := len(store.details)
	store.mu.RUnlock()

	if count > MaxDetails+1 {
		t.Errorf("expected detail count <= %d, got %d", MaxDetails+1, count)
	}

	// The latest entries should still be retrievable
	_, found := store.GetDetail(fmt.Sprintf("detail-%d", MaxDetails+9))
	if !found {
		t.Error("expected latest detail entry to exist")
	}
}
