package push

import (
	"sync"
	"testing"
	"time"
)

func TestInbox_EnqueueAndDrain(t *testing.T) {
	q := NewPushInbox(5)
	q.Enqueue(PushEvent{ID: "a", Type: "screenshot"})
	q.Enqueue(PushEvent{ID: "b", Type: "chat"})

	if q.Len() != 2 {
		t.Fatalf("expected 2, got %d", q.Len())
	}

	events := q.DrainAll()
	if len(events) != 2 {
		t.Fatalf("expected 2 drained, got %d", len(events))
	}
	if q.Len() != 0 {
		t.Fatal("queue should be empty after drain")
	}
}

func TestInbox_Peek(t *testing.T) {
	q := NewPushInbox(5)
	q.Enqueue(PushEvent{ID: "a"})

	peeked := q.Peek()
	if len(peeked) != 1 {
		t.Fatalf("expected 1, got %d", len(peeked))
	}
	if q.Len() != 1 {
		t.Fatal("peek should not remove items")
	}
}

func TestInbox_FIFOEviction(t *testing.T) {
	q := NewPushInbox(3)
	q.Enqueue(PushEvent{ID: "1"})
	q.Enqueue(PushEvent{ID: "2"})
	q.Enqueue(PushEvent{ID: "3"})
	evicted := q.Enqueue(PushEvent{ID: "4"})

	if evicted != 1 {
		t.Fatalf("expected 1 eviction, got %d", evicted)
	}
	events := q.DrainAll()
	if events[0].ID != "2" {
		t.Fatalf("expected oldest evicted, got first=%s", events[0].ID)
	}
}

func TestInbox_EmptyDrain(t *testing.T) {
	q := NewPushInbox(5)
	events := q.DrainAll()
	if events != nil {
		t.Fatalf("expected nil for empty drain, got %v", events)
	}
}

func TestInbox_ConcurrentAccess(t *testing.T) {
	q := NewPushInbox(100)
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			q.Enqueue(PushEvent{ID: "concurrent"})
		}(i)
	}
	wg.Wait()
	if q.Len() != 50 {
		t.Fatalf("expected 50, got %d", q.Len())
	}
}

func TestInbox_TimestampAutoFill(t *testing.T) {
	q := NewPushInbox(5)
	q.Enqueue(PushEvent{ID: "no-ts"})
	events := q.Peek()
	if events[0].Timestamp.IsZero() {
		t.Fatal("timestamp should be auto-filled")
	}
}

func TestInbox_PreserveExplicitTimestamp(t *testing.T) {
	q := NewPushInbox(5)
	ts := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	q.Enqueue(PushEvent{ID: "with-ts", Timestamp: ts})
	events := q.Peek()
	if !events[0].Timestamp.Equal(ts) {
		t.Fatalf("expected preserved timestamp %v, got %v", ts, events[0].Timestamp)
	}
}

func TestInbox_ScreenshotDedup_ConsecutiveSameTabURL(t *testing.T) {
	q := NewPushInbox(10)
	q.Enqueue(PushEvent{ID: "ss-1", Type: "screenshot", TabID: 1, PageURL: "https://example.com", ScreenshotB64: "old"})
	q.Enqueue(PushEvent{ID: "ss-2", Type: "screenshot", TabID: 1, PageURL: "https://example.com", ScreenshotB64: "new"})

	if q.Len() != 1 {
		t.Fatalf("expected 1 (deduped), got %d", q.Len())
	}
	events := q.DrainAll()
	if events[0].ID != "ss-2" {
		t.Fatalf("expected replacement event ss-2, got %s", events[0].ID)
	}
	if events[0].ScreenshotB64 != "new" {
		t.Fatal("expected new screenshot data to replace old")
	}
}

func TestInbox_ScreenshotDedup_DifferentURL(t *testing.T) {
	q := NewPushInbox(10)
	q.Enqueue(PushEvent{ID: "ss-1", Type: "screenshot", TabID: 1, PageURL: "https://a.com"})
	q.Enqueue(PushEvent{ID: "ss-2", Type: "screenshot", TabID: 1, PageURL: "https://b.com"})

	if q.Len() != 2 {
		t.Fatalf("different URLs should not dedup, got %d", q.Len())
	}
}

func TestInbox_ScreenshotDedup_DifferentTab(t *testing.T) {
	q := NewPushInbox(10)
	q.Enqueue(PushEvent{ID: "ss-1", Type: "screenshot", TabID: 1, PageURL: "https://example.com"})
	q.Enqueue(PushEvent{ID: "ss-2", Type: "screenshot", TabID: 2, PageURL: "https://example.com"})

	if q.Len() != 2 {
		t.Fatalf("different tabs should not dedup, got %d", q.Len())
	}
}

func TestInbox_ScreenshotDedup_NonScreenshotNotDeduped(t *testing.T) {
	q := NewPushInbox(10)
	q.Enqueue(PushEvent{ID: "c-1", Type: "chat", Message: "hello"})
	q.Enqueue(PushEvent{ID: "c-2", Type: "chat", Message: "hello"})

	if q.Len() != 2 {
		t.Fatalf("chat events should never dedup, got %d", q.Len())
	}
}

func TestInbox_ScreenshotDedup_InterruptedByOtherEvent(t *testing.T) {
	q := NewPushInbox(10)
	q.Enqueue(PushEvent{ID: "ss-1", Type: "screenshot", TabID: 1, PageURL: "https://example.com"})
	q.Enqueue(PushEvent{ID: "c-1", Type: "chat", Message: "break"})
	q.Enqueue(PushEvent{ID: "ss-2", Type: "screenshot", TabID: 1, PageURL: "https://example.com"})

	if q.Len() != 3 {
		t.Fatalf("screenshot after non-screenshot should not dedup, got %d", q.Len())
	}
}

func TestInbox_ScreenshotDedup_PreservesNote(t *testing.T) {
	q := NewPushInbox(10)
	q.Enqueue(PushEvent{ID: "ss-1", Type: "screenshot", TabID: 1, PageURL: "https://example.com", Note: "old note"})
	q.Enqueue(PushEvent{ID: "ss-2", Type: "screenshot", TabID: 1, PageURL: "https://example.com", Note: "new note"})

	events := q.DrainAll()
	if len(events) != 1 {
		t.Fatalf("expected 1 deduped event, got %d", len(events))
	}
	if events[0].Note != "new note" {
		t.Fatalf("expected new note, got %q", events[0].Note)
	}
}

func TestInbox_BulkEviction(t *testing.T) {
	q := NewPushInbox(2)
	q.Enqueue(PushEvent{ID: "1"})
	q.Enqueue(PushEvent{ID: "2"})
	q.Enqueue(PushEvent{ID: "3"})
	evicted := q.Enqueue(PushEvent{ID: "4"})

	if evicted != 1 {
		t.Fatalf("expected 1 eviction per enqueue, got %d", evicted)
	}
	if q.Len() != 2 {
		t.Fatalf("expected len 2, got %d", q.Len())
	}
}
