package push

import (
	"encoding/json"
	"sync"
	"testing"
	"time"
)

func TestChatSession_AddAndMessages(t *testing.T) {
	cs := NewChatSession("conv-1")

	cs.AddMessage(ChatMessage{
		Role: ChatRoleUser,
		Text: "hello",
	})
	cs.AddMessage(ChatMessage{
		Role: ChatRoleAssistant,
		Text: "hi there",
	})

	msgs := cs.Messages()
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}
	if msgs[0].Role != ChatRoleUser || msgs[0].Text != "hello" {
		t.Fatalf("unexpected first message: %+v", msgs[0])
	}
	if msgs[1].Role != ChatRoleAssistant || msgs[1].Text != "hi there" {
		t.Fatalf("unexpected second message: %+v", msgs[1])
	}
}

func TestChatSession_ConversationID(t *testing.T) {
	cs := NewChatSession("conv-abc")
	if cs.ConversationID() != "conv-abc" {
		t.Fatalf("expected conv-abc, got %s", cs.ConversationID())
	}
}

func TestChatSession_TimestampAutoFill(t *testing.T) {
	cs := NewChatSession("conv-1")
	cs.AddMessage(ChatMessage{Role: ChatRoleUser, Text: "test"})

	msgs := cs.Messages()
	if msgs[0].Timestamp == 0 {
		t.Fatal("timestamp should be auto-filled")
	}
}

func TestChatSession_PreserveExplicitTimestamp(t *testing.T) {
	cs := NewChatSession("conv-1")
	ts := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC).UnixMilli()
	cs.AddMessage(ChatMessage{Role: ChatRoleUser, Text: "test", Timestamp: ts})

	msgs := cs.Messages()
	if msgs[0].Timestamp != ts {
		t.Fatalf("expected preserved timestamp %d, got %d", ts, msgs[0].Timestamp)
	}
}

func TestChatSession_ConversationIDAutoSet(t *testing.T) {
	cs := NewChatSession("conv-xyz")
	cs.AddMessage(ChatMessage{Role: ChatRoleUser, Text: "test"})

	msgs := cs.Messages()
	if msgs[0].ConversationID != "conv-xyz" {
		t.Fatalf("expected conv-xyz, got %s", msgs[0].ConversationID)
	}
}

func TestChatSession_EvictionAt100(t *testing.T) {
	cs := NewChatSession("conv-1")

	for i := 0; i < 110; i++ {
		cs.AddMessage(ChatMessage{Role: ChatRoleUser, Text: "msg"})
	}

	msgs := cs.Messages()
	if len(msgs) != maxChatMessages {
		t.Fatalf("expected %d messages after eviction, got %d", maxChatMessages, len(msgs))
	}
}

func TestChatSession_SubscribeReceivesMessages(t *testing.T) {
	cs := NewChatSession("conv-1")
	ch, unsub := cs.Subscribe()
	defer unsub()

	cs.AddMessage(ChatMessage{Role: ChatRoleUser, Text: "hello"})

	select {
	case msg := <-ch:
		if msg.Text != "hello" {
			t.Fatalf("expected hello, got %s", msg.Text)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for message")
	}
}

func TestChatSession_UnsubscribeStopsDelivery(t *testing.T) {
	cs := NewChatSession("conv-1")
	ch, unsub := cs.Subscribe()
	unsub()

	cs.AddMessage(ChatMessage{Role: ChatRoleUser, Text: "hello"})

	select {
	case _, ok := <-ch:
		if ok {
			t.Fatal("should not receive message after unsubscribe")
		}
	case <-time.After(50 * time.Millisecond):
		// expected — channel closed, no message
	}
}

func TestChatSession_MultipleSubscribers(t *testing.T) {
	cs := NewChatSession("conv-1")
	ch1, unsub1 := cs.Subscribe()
	defer unsub1()
	ch2, unsub2 := cs.Subscribe()
	defer unsub2()

	cs.AddMessage(ChatMessage{Role: ChatRoleUser, Text: "broadcast"})

	for i, ch := range []<-chan ChatMessage{ch1, ch2} {
		select {
		case msg := <-ch:
			if msg.Text != "broadcast" {
				t.Fatalf("subscriber %d: expected broadcast, got %s", i, msg.Text)
			}
		case <-time.After(time.Second):
			t.Fatalf("subscriber %d: timed out", i)
		}
	}
}

func TestChatSession_CloseUnblocksSubscribers(t *testing.T) {
	cs := NewChatSession("conv-1")
	ch, _ := cs.Subscribe()

	cs.Close()

	select {
	case _, ok := <-ch:
		if ok {
			t.Fatal("expected channel to be closed")
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for channel close")
	}
}

func TestChatSession_ConcurrentAccess(t *testing.T) {
	cs := NewChatSession("conv-1")
	var wg sync.WaitGroup

	// Concurrent writers
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			cs.AddMessage(ChatMessage{Role: ChatRoleUser, Text: "concurrent"})
		}()
	}

	// Concurrent readers
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = cs.Messages()
		}()
	}

	// Concurrent subscribe/unsubscribe
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, unsub := cs.Subscribe()
			unsub()
		}()
	}

	wg.Wait()
}

func TestChatSession_MessagesSnapshotIsolation(t *testing.T) {
	cs := NewChatSession("conv-1")
	cs.AddMessage(ChatMessage{Role: ChatRoleUser, Text: "first"})

	snapshot := cs.Messages()
	cs.AddMessage(ChatMessage{Role: ChatRoleUser, Text: "second"})

	if len(snapshot) != 1 {
		t.Fatalf("snapshot should be isolated, got %d messages", len(snapshot))
	}
}

func TestChatSession_AnnotationMessage(t *testing.T) {
	cs := NewChatSession("conv-1")

	annotations := json.RawMessage(`[{"label":"button color","rect":{"x":10,"y":20}}]`)
	cs.AddMessage(ChatMessage{
		Role:        ChatRoleAnnotation,
		Text:        "2 annotations from draw mode",
		Annotations: annotations,
	})

	msgs := cs.Messages()
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
	if msgs[0].Role != ChatRoleAnnotation {
		t.Fatalf("expected annotation role, got %s", msgs[0].Role)
	}
	if msgs[0].Annotations == nil {
		t.Fatal("annotations should not be nil")
	}
}
