// chat_handlers_test.go — Tests for chat SSE streaming and response handlers.
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/push"
)

func newTestChatServer() *Server {
	inbox := push.NewPushInbox(50)
	return &Server{
		pushInbox:  inbox,
		pushRouter: push.NewRouter(inbox, nil, nil, push.ClientCapabilities{}),
	}
}

func TestHandleChatStream_RequiresConversationID(t *testing.T) {
	s := newTestChatServer()
	req := httptest.NewRequest("GET", "/chat/stream", nil)
	w := httptest.NewRecorder()

	s.handleChatStream(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestHandleChatStream_RejectsNonGET(t *testing.T) {
	s := newTestChatServer()
	req := httptest.NewRequest("POST", "/chat/stream?conversation_id=abc", nil)
	w := httptest.NewRecorder()

	s.handleChatStream(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}

func TestHandleChatStream_SendsHistoryAndMessages(t *testing.T) {
	s := newTestChatServer()

	// Pre-populate a session with a message
	session := s.getOrCreateChatSession("conv-1")
	session.AddMessage(push.ChatMessage{Role: push.ChatRoleUser, Text: "existing msg"})

	// Set up a cancellable context
	ctx, cancel := context.WithCancel(context.Background())

	// Use a real HTTP server to get proper flushing
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.handleChatStream(w, r)
	})
	ts := httptest.NewServer(handler)
	defer ts.Close()

	// Make request using the test server
	client := ts.Client()
	reqHTTP, _ := http.NewRequestWithContext(ctx, "GET", ts.URL+"/chat/stream?conversation_id=conv-1", nil)
	resp, err := client.Do(reqHTTP)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.Header.Get("Content-Type") != "text/event-stream" {
		t.Fatalf("expected text/event-stream, got %s", resp.Header.Get("Content-Type"))
	}

	scanner := bufio.NewScanner(resp.Body)
	// Read first SSE event (history)
	var eventType, eventData string
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "event: ") {
			eventType = strings.TrimPrefix(line, "event: ")
		} else if strings.HasPrefix(line, "data: ") {
			eventData = strings.TrimPrefix(line, "data: ")
		} else if line == "" && eventType != "" {
			break // end of event
		}
	}

	if eventType != "history" {
		t.Fatalf("expected history event, got %s", eventType)
	}

	var history []push.ChatMessage
	if err := json.Unmarshal([]byte(eventData), &history); err != nil {
		t.Fatalf("failed to parse history: %v", err)
	}
	if len(history) != 1 || history[0].Text != "existing msg" {
		t.Fatalf("unexpected history: %+v", history)
	}

	// Add a new message and read it from SSE
	session.AddMessage(push.ChatMessage{Role: push.ChatRoleAssistant, Text: "new response"})

	eventType, eventData = "", ""
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "event: ") {
			eventType = strings.TrimPrefix(line, "event: ")
		} else if strings.HasPrefix(line, "data: ") {
			eventData = strings.TrimPrefix(line, "data: ")
		} else if line == "" && eventType != "" {
			break
		}
	}

	if eventType != "message" {
		t.Fatalf("expected message event, got %s", eventType)
	}

	var msg push.ChatMessage
	if err := json.Unmarshal([]byte(eventData), &msg); err != nil {
		t.Fatalf("failed to parse message: %v", err)
	}
	if msg.Text != "new response" || msg.Role != push.ChatRoleAssistant {
		t.Fatalf("unexpected message: %+v", msg)
	}

	cancel()
}

func TestHandleChatResponse_Success(t *testing.T) {
	s := newTestChatServer()
	session := s.getOrCreateChatSession("conv-1")

	// Track a sampling request
	var requestID int64 = 12345
	s.samplingRequests.Store(requestID, "conv-1")

	body := `{"request_id":12345,"text":"I can help with that."}`
	req := httptest.NewRequest("POST", "/chat/response", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	s.handleChatResponse(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	msgs := session.Messages()
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
	if msgs[0].Role != push.ChatRoleAssistant || msgs[0].Text != "I can help with that." {
		t.Fatalf("unexpected message: %+v", msgs[0])
	}
}

func TestHandleChatResponse_UnknownRequestID(t *testing.T) {
	s := newTestChatServer()

	body := `{"request_id":99999,"text":"orphan response"}`
	req := httptest.NewRequest("POST", "/chat/response", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	s.handleChatResponse(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestHandleChatResponse_RejectsNonPOST(t *testing.T) {
	s := newTestChatServer()
	req := httptest.NewRequest("GET", "/chat/response", nil)
	w := httptest.NewRecorder()

	s.handleChatResponse(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}

func TestHandleChatResponse_InvalidJSON(t *testing.T) {
	s := newTestChatServer()
	req := httptest.NewRequest("POST", "/chat/response", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	s.handleChatResponse(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestHandlePushMessage_WithConversationID(t *testing.T) {
	s := newTestChatServer()

	body := `{"message":"hello from chat","page_url":"https://example.com","tab_id":1,"conversation_id":"conv-42"}`
	req := httptest.NewRequest("POST", "/push/message", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	s.handlePushMessage(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp["conversation_id"] != "conv-42" {
		t.Fatalf("expected conversation_id=conv-42, got %v", resp["conversation_id"])
	}

	// Verify the message was added to the chat session
	s.chatSessionMu.Lock()
	session := s.chatSession
	s.chatSessionMu.Unlock()

	if session == nil {
		t.Fatal("expected chat session to be created")
	}
	msgs := session.Messages()
	if len(msgs) != 1 || msgs[0].Text != "hello from chat" {
		t.Fatalf("unexpected session messages: %+v", msgs)
	}
}

func TestPushDrawModeCompletion_InjectsIntoChatSession(t *testing.T) {
	s := newTestChatServer()

	// Create an active chat session
	session := s.getOrCreateChatSession("conv-draw")
	ch, unsub := session.Subscribe()
	defer unsub()

	// Simulate draw mode completion
	body := &drawModeRequest{
		PageURL:          "https://example.com",
		TabID:            1,
		AnnotSessionName: "test-annot",
	}
	annotations := []Annotation{
		{Text: "button", Rect: AnnotationRect{X: 10, Y: 20, Width: 100, Height: 50}},
	}
	s.pushDrawModeCompletion(body, "", annotations)

	// Verify annotation message was injected
	select {
	case msg := <-ch:
		if msg.Role != push.ChatRoleAnnotation {
			t.Fatalf("expected annotation role, got %s", msg.Role)
		}
		if msg.Annotations == nil {
			t.Fatal("expected annotations data")
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for annotation injection")
	}
}

func TestGetOrCreateChatSession_ReusesExisting(t *testing.T) {
	s := newTestChatServer()
	s1 := s.getOrCreateChatSession("conv-1")
	s2 := s.getOrCreateChatSession("conv-1")

	if s1 != s2 {
		t.Fatal("expected same session instance for same conversation ID")
	}
}

func TestGetOrCreateChatSession_ClosesPreviousOnNewID(t *testing.T) {
	s := newTestChatServer()
	s1 := s.getOrCreateChatSession("conv-1")
	ch, _ := s1.Subscribe()

	_ = s.getOrCreateChatSession("conv-2")

	// Previous session should be closed
	select {
	case _, ok := <-ch:
		if ok {
			t.Fatal("expected channel to be closed after session replacement")
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for old session close")
	}
}
