// bridge_sampling_test.go — Tests for bridge sampling response detection and forwarding.
package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestIsSamplingResponse_MethodEmptyIDPresent(t *testing.T) {
	raw := `{"jsonrpc":"2.0","id":12345,"result":{"content":[{"type":"text","text":"hello"}]}}`
	var req JSONRPCRequest
	if err := json.Unmarshal([]byte(raw), &req); err != nil {
		t.Fatal(err)
	}
	if !isSamplingResponse(req) {
		t.Fatal("expected sampling response detection for Method='' + ID present")
	}
}

func TestIsSamplingResponse_MethodNonEmpty(t *testing.T) {
	raw := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{}}`
	var req JSONRPCRequest
	if err := json.Unmarshal([]byte(raw), &req); err != nil {
		t.Fatal(err)
	}
	if isSamplingResponse(req) {
		t.Fatal("should not detect as sampling response when Method is non-empty")
	}
}

func TestIsSamplingResponse_NoID(t *testing.T) {
	raw := `{"jsonrpc":"2.0","method":"notifications/initialized"}`
	var req JSONRPCRequest
	if err := json.Unmarshal([]byte(raw), &req); err != nil {
		t.Fatal(err)
	}
	if isSamplingResponse(req) {
		t.Fatal("should not detect as sampling response when no ID")
	}
}

func TestForwardSamplingResponse_PostsToDaemon(t *testing.T) {
	var receivedBody []byte
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/chat/response" {
			receivedBody, _ = io.ReadAll(r.Body)
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	client := ts.Client()
	raw := json.RawMessage(`{"jsonrpc":"2.0","id":12345,"result":{"content":[{"type":"text","text":"I can help"}],"model":"claude","role":"assistant"}}`)

	forwardSamplingResponse(client, ts.URL, raw)

	if receivedBody == nil {
		t.Fatal("expected request to /chat/response")
	}

	var body struct {
		RequestID int64  `json:"request_id"`
		Text      string `json:"text"`
	}
	if err := json.Unmarshal(receivedBody, &body); err != nil {
		t.Fatalf("invalid body: %v; raw: %s", err, receivedBody)
	}
	if body.RequestID != 12345 {
		t.Fatalf("expected request_id=12345, got %d", body.RequestID)
	}
	if body.Text != "I can help" {
		t.Fatalf("expected text='I can help', got %q", body.Text)
	}
}

func TestForwardSamplingResponse_MultipleContentBlocks(t *testing.T) {
	var receivedBody []byte
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/chat/response" {
			receivedBody, _ = io.ReadAll(r.Body)
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	client := ts.Client()
	raw := json.RawMessage(`{"jsonrpc":"2.0","id":99,"result":{"content":[{"type":"text","text":"Part 1. "},{"type":"text","text":"Part 2."}],"model":"claude","role":"assistant"}}`)

	forwardSamplingResponse(client, ts.URL, raw)

	var body struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal(receivedBody, &body); err != nil {
		t.Fatalf("invalid body: %v", err)
	}
	if body.Text != "Part 1. Part 2." {
		t.Fatalf("expected concatenated text, got %q", body.Text)
	}
}
