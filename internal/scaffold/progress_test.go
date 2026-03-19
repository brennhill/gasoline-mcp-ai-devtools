// progress_test.go — Tests for scaffold progress broadcasting.

package scaffold

import (
	"sync"
	"testing"
	"time"
)

// ============================================
// Progress Broadcaster
// ============================================

func TestBroadcaster_SubscribeReceivesEvents(t *testing.T) {
	b := NewBroadcaster()
	defer b.Close()

	ch := b.Subscribe("test-channel")
	defer b.Unsubscribe(ch)

	evt := StepEvent{Step: "create_project", Status: "running", Label: "Creating project"}
	b.Broadcast("test-channel", evt)

	select {
	case got := <-ch:
		if got.Step != "create_project" {
			t.Errorf("want step 'create_project', got %q", got.Step)
		}
		if got.Status != "running" {
			t.Errorf("want status 'running', got %q", got.Status)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for broadcast event")
	}
}

func TestBroadcaster_OnlyReceivesMatchingChannel(t *testing.T) {
	b := NewBroadcaster()
	defer b.Close()

	ch1 := b.Subscribe("channel-1")
	defer b.Unsubscribe(ch1)
	ch2 := b.Subscribe("channel-2")
	defer b.Unsubscribe(ch2)

	b.Broadcast("channel-1", StepEvent{Step: "test", Status: "done", Label: "Test"})

	select {
	case <-ch1:
		// Good, channel-1 got it.
	case <-time.After(100 * time.Millisecond):
		t.Fatal("channel-1 should have received the event")
	}

	select {
	case <-ch2:
		t.Fatal("channel-2 should NOT have received the event")
	case <-time.After(50 * time.Millisecond):
		// Good, timed out.
	}
}

func TestBroadcaster_MultipleSubscribers(t *testing.T) {
	b := NewBroadcaster()
	defer b.Close()

	ch1 := b.Subscribe("shared")
	defer b.Unsubscribe(ch1)
	ch2 := b.Subscribe("shared")
	defer b.Unsubscribe(ch2)

	b.Broadcast("shared", StepEvent{Step: "test", Status: "done", Label: "Test"})

	for _, ch := range []<-chan StepEvent{ch1, ch2} {
		select {
		case <-ch:
			// Good.
		case <-time.After(100 * time.Millisecond):
			t.Fatal("subscriber should have received the event")
		}
	}
}

func TestBroadcaster_UnsubscribedChannelDoesNotReceive(t *testing.T) {
	b := NewBroadcaster()
	defer b.Close()

	ch := b.Subscribe("test")
	b.Unsubscribe(ch)

	b.Broadcast("test", StepEvent{Step: "test", Status: "done", Label: "Test"})

	select {
	case _, ok := <-ch:
		if ok {
			t.Fatal("unsubscribed channel should not receive events")
		}
	case <-time.After(50 * time.Millisecond):
		// Good, timed out — channel was closed by Unsubscribe.
	}
}

func TestBroadcaster_ConcurrentSafety(t *testing.T) {
	b := NewBroadcaster()
	defer b.Close()

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ch := b.Subscribe("concurrent")
			defer b.Unsubscribe(ch)
			b.Broadcast("concurrent", StepEvent{Step: "test", Status: "done", Label: "Test"})
		}()
	}
	wg.Wait()
}

func TestBroadcaster_SlowConsumerDoesNotBlock(t *testing.T) {
	b := NewBroadcaster()
	defer b.Close()

	// Subscribe but never read.
	_ = b.Subscribe("slow")

	// Broadcast many events — should not block.
	done := make(chan struct{})
	go func() {
		for i := 0; i < 100; i++ {
			b.Broadcast("slow", StepEvent{Step: "test", Status: "done", Label: "Test"})
		}
		close(done)
	}()

	select {
	case <-done:
		// Good, didn't block.
	case <-time.After(1 * time.Second):
		t.Fatal("broadcaster blocked on slow consumer")
	}
}
