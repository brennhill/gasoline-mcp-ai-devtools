package push

import (
	"errors"
	"sync"
	"testing"
)

type mockSender struct {
	mu       sync.Mutex
	calls    []SamplingRequest
	failNext bool
}

func (m *mockSender) SendSampling(req SamplingRequest) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.failNext {
		m.failNext = false
		return errors.New("send failed")
	}
	m.calls = append(m.calls, req)
	return nil
}

type mockNotifier struct {
	mu    sync.Mutex
	calls []string
}

func (m *mockNotifier) SendNotification(method string, _ map[string]any) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls = append(m.calls, method)
}

func TestRouter_SamplingPath(t *testing.T) {
	inbox := NewPushInbox(10)
	sender := &mockSender{}
	notifier := &mockNotifier{}
	caps := ClientCapabilities{SupportsSampling: true, ClientName: "test"}

	r := NewRouter(inbox, sender, notifier, caps)
	result, err := r.DeliverPush(PushEvent{ID: "1", Type: "chat", Message: "hello"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Method != DeliveredViaSampling {
		t.Fatalf("expected sampling delivery, got %s", result.Method)
	}
	if len(sender.calls) != 1 {
		t.Fatalf("expected 1 sampling call, got %d", len(sender.calls))
	}
	if inbox.Len() != 0 {
		t.Fatal("inbox should be empty when sampling succeeds")
	}
}

func TestRouter_SamplingFailFallsToInbox(t *testing.T) {
	inbox := NewPushInbox(10)
	sender := &mockSender{failNext: true}
	notifier := &mockNotifier{}
	caps := ClientCapabilities{SupportsSampling: true, ClientName: "test"}

	r := NewRouter(inbox, sender, notifier, caps)
	result, _ := r.DeliverPush(PushEvent{ID: "1", Type: "screenshot"})

	if result.Method != DeliveredViaInbox {
		t.Fatalf("expected inbox delivery after sampling failure, got %s", result.Method)
	}
	if inbox.Len() != 1 {
		t.Fatal("event should be in inbox after sampling failure")
	}
}

func TestRouter_NotificationPath(t *testing.T) {
	inbox := NewPushInbox(10)
	notifier := &mockNotifier{}
	caps := ClientCapabilities{SupportsNotifications: true, ClientName: "test"}

	r := NewRouter(inbox, nil, notifier, caps)
	result, _ := r.DeliverPush(PushEvent{ID: "1", Type: "chat", Message: "hi"})

	if result.Method != DeliveredViaNotification {
		t.Fatalf("expected notification delivery method, got %s", result.Method)
	}
	if len(notifier.calls) != 1 {
		t.Fatalf("expected 1 notification, got %d", len(notifier.calls))
	}
	if inbox.Len() != 1 {
		t.Fatal("event should also be in inbox for full content retrieval")
	}
}

func TestRouter_InboxOnlyFallback(t *testing.T) {
	inbox := NewPushInbox(10)
	caps := ClientCapabilities{ClientName: "basic"}

	r := NewRouter(inbox, nil, nil, caps)
	result, _ := r.DeliverPush(PushEvent{ID: "1", Type: "chat"})

	if result.Method != DeliveredViaInbox {
		t.Fatalf("expected inbox delivery, got %s", result.Method)
	}
	if inbox.Len() != 1 {
		t.Fatal("event should be in inbox")
	}
}

func TestRouter_UpdateCapabilities(t *testing.T) {
	inbox := NewPushInbox(10)
	r := NewRouter(inbox, nil, nil, ClientCapabilities{})

	r.UpdateCapabilities(ClientCapabilities{SupportsSampling: true, ClientName: "claude"})
	caps := r.GetCapabilities()
	if !caps.SupportsSampling {
		t.Fatal("capabilities should be updated")
	}
}

func TestRouter_ChatMessageSampling(t *testing.T) {
	inbox := NewPushInbox(10)
	sender := &mockSender{}
	caps := ClientCapabilities{SupportsSampling: true, ClientName: "test"}

	r := NewRouter(inbox, sender, nil, caps)
	result, _ := r.DeliverPush(PushEvent{ID: "1", Type: "chat", Message: "fix the login bug", PageURL: "https://app.example.com"})

	if result.Method != DeliveredViaSampling {
		t.Fatalf("expected sampling delivery, got %s", result.Method)
	}
	if len(sender.calls) != 1 {
		t.Fatal("expected sampling call for chat")
	}
	req := sender.calls[0]
	if req.Method != "sampling/createMessage" {
		t.Fatalf("expected sampling/createMessage, got %s", req.Method)
	}
}
