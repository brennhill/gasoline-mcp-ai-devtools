package push

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestBuildSamplingRequest_Screenshot(t *testing.T) {
	ev := PushEvent{
		ID:            "s1",
		Type:          "screenshot",
		ScreenshotB64: "iVBORw0KGgo=",
		PageURL:       "https://example.com",
		Note:          "Bug on login",
	}
	req := BuildSamplingRequest(ev)

	if req.Method != "sampling/createMessage" {
		t.Fatalf("expected sampling/createMessage, got %s", req.Method)
	}
	if len(req.Params.Messages) < 2 {
		t.Fatalf("expected at least 2 messages (image+text), got %d", len(req.Params.Messages))
	}
	if req.Params.Messages[0].Content.Type != "image" {
		t.Fatal("first message should be image")
	}
}

func TestBuildSamplingRequest_Chat(t *testing.T) {
	ev := PushEvent{
		ID:      "c1",
		Type:    "chat",
		Message: "Fix the login button",
		PageURL: "https://app.com/login",
	}
	req := BuildSamplingRequest(ev)

	if len(req.Params.Messages) != 1 {
		t.Fatalf("expected 1 message for chat, got %d", len(req.Params.Messages))
	}
	msg := req.Params.Messages[0]
	if msg.Content.Type != "text" {
		t.Fatal("chat should be text type")
	}
	if !strings.Contains(msg.Content.Text, "Fix the login button") {
		t.Fatal("chat text should contain the message")
	}
	if !strings.Contains(msg.Content.Text, "app.com") {
		t.Fatal("chat text should contain the page URL")
	}
}

func TestBuildSamplingRequest_Annotations(t *testing.T) {
	ev := PushEvent{
		ID:          "a1",
		Type:        "annotations",
		Annotations: json.RawMessage(`[{"text":"broken"}]`),
		PageURL:     "https://test.com",
	}
	req := BuildSamplingRequest(ev)

	if len(req.Params.Messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(req.Params.Messages))
	}
	if !strings.Contains(req.Params.Messages[0].Content.Text, "broken") {
		t.Fatal("should include annotation content")
	}
}

func TestBuildSamplingRequest_ValidJSON(t *testing.T) {
	ev := PushEvent{ID: "j1", Type: "chat", Message: "hello"}
	req := BuildSamplingRequest(ev)
	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("should produce valid JSON: %v", err)
	}
	if !json.Valid(data) {
		t.Fatal("output should be valid JSON")
	}
}

func TestBuildSamplingRequest_IDFormat(t *testing.T) {
	ev := PushEvent{ID: "f1", Type: "chat", Message: "test"}
	req := BuildSamplingRequest(ev)
	if req.ID <= 0 {
		t.Fatal("ID should be positive")
	}
}
