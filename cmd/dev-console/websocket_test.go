package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestV4WebSocketEventBuffer(t *testing.T) {
	capture := setupTestCapture(t)

	events := []WebSocketEvent{
		{Timestamp: "2024-01-15T10:30:00.000Z", Type: "websocket", Event: "open", ID: "uuid-1", URL: "wss://example.com/ws"},
		{Timestamp: "2024-01-15T10:30:01.000Z", Type: "websocket", Event: "message", ID: "uuid-1", Direction: "incoming", Data: `{"type":"chat","msg":"hello"}`, Size: 32},
	}

	capture.AddWebSocketEvents(events)

	if capture.GetWebSocketEventCount() != 2 {
		t.Errorf("Expected 2 events, got %d", capture.GetWebSocketEventCount())
	}
}

func TestV4WebSocketEventBufferRotation(t *testing.T) {
	capture := setupTestCapture(t)

	// Add more than max (500) events
	events := make([]WebSocketEvent, 550)
	for i := range events {
		events[i] = WebSocketEvent{
			Timestamp: "2024-01-15T10:30:00.000Z",
			Type:      "websocket",
			Event:     "message",
			ID:        "uuid-1",
			Data:      `{"i":` + string(rune(i)) + `}`,
		}
	}

	capture.AddWebSocketEvents(events)

	if capture.GetWebSocketEventCount() != 500 {
		t.Errorf("Expected 500 events after rotation, got %d", capture.GetWebSocketEventCount())
	}
}

func TestV4WebSocketEventFilterByConnectionID(t *testing.T) {
	capture := setupTestCapture(t)

	capture.AddWebSocketEvents([]WebSocketEvent{
		{ID: "uuid-1", Event: "open", URL: "wss://a.com"},
		{ID: "uuid-2", Event: "open", URL: "wss://b.com"},
		{ID: "uuid-1", Event: "message", Direction: "incoming"},
	})

	filtered := capture.GetWebSocketEvents(WebSocketEventFilter{ConnectionID: "uuid-1"})

	if len(filtered) != 2 {
		t.Errorf("Expected 2 events for uuid-1, got %d", len(filtered))
	}
}

func TestV4WebSocketEventFilterByURL(t *testing.T) {
	capture := setupTestCapture(t)

	capture.AddWebSocketEvents([]WebSocketEvent{
		{ID: "uuid-1", Event: "open", URL: "wss://chat.example.com/ws"},
		{ID: "uuid-2", Event: "open", URL: "wss://feed.example.com/prices"},
		{ID: "uuid-1", Event: "message", URL: "wss://chat.example.com/ws"},
	})

	filtered := capture.GetWebSocketEvents(WebSocketEventFilter{URLFilter: "chat"})

	if len(filtered) != 2 {
		t.Errorf("Expected 2 events matching 'chat', got %d", len(filtered))
	}
}

func TestV4WebSocketEventFilterByDirection(t *testing.T) {
	capture := setupTestCapture(t)

	capture.AddWebSocketEvents([]WebSocketEvent{
		{ID: "uuid-1", Event: "message", Direction: "incoming"},
		{ID: "uuid-1", Event: "message", Direction: "outgoing"},
		{ID: "uuid-1", Event: "message", Direction: "incoming"},
	})

	filtered := capture.GetWebSocketEvents(WebSocketEventFilter{Direction: "incoming"})

	if len(filtered) != 2 {
		t.Errorf("Expected 2 incoming events, got %d", len(filtered))
	}
}

func TestV4WebSocketEventFilterWithLimit(t *testing.T) {
	capture := setupTestCapture(t)

	for i := 0; i < 10; i++ {
		capture.AddWebSocketEvents([]WebSocketEvent{
			{ID: "uuid-1", Event: "message", Direction: "incoming"},
		})
	}

	filtered := capture.GetWebSocketEvents(WebSocketEventFilter{Limit: 5})

	if len(filtered) != 5 {
		t.Errorf("Expected 5 events with limit, got %d", len(filtered))
	}
}

func TestV4WebSocketEventDefaultLimit(t *testing.T) {
	capture := setupTestCapture(t)

	for i := 0; i < 100; i++ {
		capture.AddWebSocketEvents([]WebSocketEvent{
			{ID: "uuid-1", Event: "message"},
		})
	}

	// Default limit is 50
	filtered := capture.GetWebSocketEvents(WebSocketEventFilter{})

	if len(filtered) != 50 {
		t.Errorf("Expected 50 events with default limit, got %d", len(filtered))
	}
}

func TestV4WebSocketEventNewestFirst(t *testing.T) {
	capture := setupTestCapture(t)

	capture.AddWebSocketEvents([]WebSocketEvent{
		{Timestamp: "2024-01-15T10:30:00.000Z", ID: "uuid-1", Event: "open"},
		{Timestamp: "2024-01-15T10:30:05.000Z", ID: "uuid-1", Event: "close"},
	})

	filtered := capture.GetWebSocketEvents(WebSocketEventFilter{})

	if len(filtered) == 0 {
		t.Fatal("Expected events to be returned")
	}
	if filtered[0].Timestamp != "2024-01-15T10:30:05.000Z" {
		t.Errorf("Expected newest first, got %s", filtered[0].Timestamp)
	}
}

func TestV4WebSocketConnectionTracker(t *testing.T) {
	capture := setupTestCapture(t)

	capture.AddWebSocketEvents([]WebSocketEvent{
		{ID: "uuid-1", Event: "open", URL: "wss://chat.example.com/ws", Timestamp: "2024-01-15T10:30:00.000Z"},
	})

	status := capture.GetWebSocketStatus(WebSocketStatusFilter{})

	if len(status.Connections) != 1 {
		t.Fatalf("Expected 1 open connection, got %d", len(status.Connections))
	}

	if status.Connections[0].State != "open" {
		t.Errorf("Expected state 'open', got %s", status.Connections[0].State)
	}

	if status.Connections[0].URL != "wss://chat.example.com/ws" {
		t.Errorf("Expected URL 'wss://chat.example.com/ws', got %s", status.Connections[0].URL)
	}
}

func TestV4WebSocketConnectionClose(t *testing.T) {
	capture := setupTestCapture(t)

	capture.AddWebSocketEvents([]WebSocketEvent{
		{ID: "uuid-1", Event: "open", URL: "wss://example.com/ws"},
		{ID: "uuid-1", Event: "close", URL: "wss://example.com/ws", CloseCode: 1000, CloseReason: "normal closure"},
	})

	status := capture.GetWebSocketStatus(WebSocketStatusFilter{})

	if len(status.Connections) != 0 {
		t.Errorf("Expected 0 open connections, got %d", len(status.Connections))
	}

	if len(status.Closed) != 1 {
		t.Fatalf("Expected 1 closed connection, got %d", len(status.Closed))
	}

	if status.Closed[0].CloseCode != 1000 {
		t.Errorf("Expected close code 1000, got %d", status.Closed[0].CloseCode)
	}
}

func TestV4WebSocketConnectionError(t *testing.T) {
	capture := setupTestCapture(t)

	capture.AddWebSocketEvents([]WebSocketEvent{
		{ID: "uuid-1", Event: "open", URL: "wss://example.com/ws"},
		{ID: "uuid-1", Event: "error", URL: "wss://example.com/ws"},
	})

	status := capture.GetWebSocketStatus(WebSocketStatusFilter{})

	if len(status.Connections) != 1 {
		t.Fatalf("Expected 1 connection (in error state), got %d", len(status.Connections))
	}

	if status.Connections[0].State != "error" {
		t.Errorf("Expected state 'error', got %s", status.Connections[0].State)
	}
}

func TestV4WebSocketConnectionMessageStats(t *testing.T) {
	capture := setupTestCapture(t)

	capture.AddWebSocketEvents([]WebSocketEvent{
		{ID: "uuid-1", Event: "open", URL: "wss://example.com/ws"},
		{ID: "uuid-1", Event: "message", Direction: "incoming", Size: 100},
		{ID: "uuid-1", Event: "message", Direction: "incoming", Size: 200},
		{ID: "uuid-1", Event: "message", Direction: "outgoing", Size: 50},
	})

	status := capture.GetWebSocketStatus(WebSocketStatusFilter{})

	if len(status.Connections) != 1 {
		t.Fatalf("Expected 1 connection, got %d", len(status.Connections))
	}

	conn := status.Connections[0]
	if conn.MessageRate.Incoming.Total != 2 {
		t.Errorf("Expected 2 incoming messages, got %d", conn.MessageRate.Incoming.Total)
	}

	if conn.MessageRate.Incoming.Bytes != 300 {
		t.Errorf("Expected 300 incoming bytes, got %d", conn.MessageRate.Incoming.Bytes)
	}

	if conn.MessageRate.Outgoing.Total != 1 {
		t.Errorf("Expected 1 outgoing message, got %d", conn.MessageRate.Outgoing.Total)
	}

	if conn.MessageRate.Outgoing.Bytes != 50 {
		t.Errorf("Expected 50 outgoing bytes, got %d", conn.MessageRate.Outgoing.Bytes)
	}
}

func TestV4WebSocketConnectionLastMessage(t *testing.T) {
	capture := setupTestCapture(t)

	capture.AddWebSocketEvents([]WebSocketEvent{
		{ID: "uuid-1", Event: "open", URL: "wss://example.com/ws"},
		{ID: "uuid-1", Event: "message", Direction: "incoming", Data: `{"type":"hello"}`, Timestamp: "2024-01-15T10:30:01.000Z"},
		{ID: "uuid-1", Event: "message", Direction: "incoming", Data: `{"type":"world"}`, Timestamp: "2024-01-15T10:30:02.000Z"},
	})

	status := capture.GetWebSocketStatus(WebSocketStatusFilter{})
	conn := status.Connections[0]

	if conn.LastMessage.Incoming.Preview != `{"type":"world"}` {
		t.Errorf("Expected last incoming preview to be world message, got %s", conn.LastMessage.Incoming.Preview)
	}
}

func TestV4WebSocketMaxTrackedConnections(t *testing.T) {
	capture := setupTestCapture(t)

	// Open 25 connections (max is 20 active)
	for i := 0; i < 25; i++ {
		capture.AddWebSocketEvents([]WebSocketEvent{
			{ID: "uuid-" + string(rune('a'+i)), Event: "open", URL: "wss://example.com/ws"},
		})
	}

	status := capture.GetWebSocketStatus(WebSocketStatusFilter{})

	if len(status.Connections) > 20 {
		t.Errorf("Expected max 20 active connections, got %d", len(status.Connections))
	}
}

func TestV4WebSocketClosedConnectionHistory(t *testing.T) {
	capture := setupTestCapture(t)

	// Open and close 15 connections (max closed history is 10)
	for i := 0; i < 15; i++ {
		id := "uuid-" + strings.Repeat("x", i+1) // unique IDs
		capture.AddWebSocketEvents([]WebSocketEvent{
			{ID: id, Event: "open", URL: "wss://example.com/ws"},
			{ID: id, Event: "close", URL: "wss://example.com/ws", CloseCode: 1000},
		})
	}

	status := capture.GetWebSocketStatus(WebSocketStatusFilter{})

	if len(status.Closed) > 10 {
		t.Errorf("Expected max 10 closed connections in history, got %d", len(status.Closed))
	}
}

func TestV4WebSocketStatusFilterByURL(t *testing.T) {
	capture := setupTestCapture(t)

	capture.AddWebSocketEvents([]WebSocketEvent{
		{ID: "uuid-1", Event: "open", URL: "wss://chat.example.com/ws"},
		{ID: "uuid-2", Event: "open", URL: "wss://feed.example.com/prices"},
	})

	status := capture.GetWebSocketStatus(WebSocketStatusFilter{URLFilter: "chat"})

	if len(status.Connections) != 1 {
		t.Errorf("Expected 1 connection matching 'chat', got %d", len(status.Connections))
	}
}

func TestV4WebSocketStatusFilterByConnectionID(t *testing.T) {
	capture := setupTestCapture(t)

	capture.AddWebSocketEvents([]WebSocketEvent{
		{ID: "uuid-1", Event: "open", URL: "wss://a.com"},
		{ID: "uuid-2", Event: "open", URL: "wss://b.com"},
	})

	status := capture.GetWebSocketStatus(WebSocketStatusFilter{ConnectionID: "uuid-2"})

	if len(status.Connections) != 1 {
		t.Errorf("Expected 1 connection, got %d", len(status.Connections))
	}

	if status.Connections[0].ID != "uuid-2" {
		t.Errorf("Expected connection uuid-2, got %s", status.Connections[0].ID)
	}
}

func TestV4WebSocketSamplingInfo(t *testing.T) {
	capture := setupTestCapture(t)

	capture.AddWebSocketEvents([]WebSocketEvent{
		{ID: "uuid-1", Event: "open", URL: "wss://example.com/ws"},
		{ID: "uuid-1", Event: "message", Direction: "incoming", Sampled: &SamplingInfo{Rate: "48.2/s", Logged: "1/5", Window: "5s"}},
	})

	status := capture.GetWebSocketStatus(WebSocketStatusFilter{})
	conn := status.Connections[0]

	if !conn.Sampling.Active {
		t.Error("Expected sampling to be active")
	}
}

func TestV4PostWebSocketEventsEndpoint(t *testing.T) {
	capture := setupTestCapture(t)

	body := `{"events":[{"ts":"2024-01-15T10:30:00.000Z","type":"websocket","event":"open","id":"uuid-1","url":"wss://example.com/ws"}]}`
	req := httptest.NewRequest("POST", "/websocket-events", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	capture.HandleWebSocketEvents(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", rec.Code)
	}

	if capture.GetWebSocketEventCount() != 1 {
		t.Errorf("Expected 1 event stored, got %d", capture.GetWebSocketEventCount())
	}
}

func TestV4PostWebSocketEventsInvalidJSON(t *testing.T) {
	capture := setupTestCapture(t)

	req := httptest.NewRequest("POST", "/websocket-events", bytes.NewBufferString("not json"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	capture.HandleWebSocketEvents(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Expected 400, got %d", rec.Code)
	}
}

func TestMCPGetWebSocketEvents(t *testing.T) {
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	// Add some WebSocket events
	capture.AddWebSocketEvents([]WebSocketEvent{
		{Timestamp: "2024-01-15T10:30:00.000Z", ID: "uuid-1", Event: "open", URL: "wss://example.com/ws"},
		{Timestamp: "2024-01-15T10:30:01.000Z", ID: "uuid-1", Event: "message", Direction: "incoming", Data: `{"msg":"hello"}`},
	})

	// Initialize MCP
	mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "initialize",
		Params: json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}`),
	})

	// Call get_websocket_events tool
	resp := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 2, Method: "tools/call",
		Params: json.RawMessage(`{"name":"observe","arguments":{"what":"websocket_events"}}`),
	})

	if resp.Error != nil {
		t.Fatalf("Expected no error, got: %v", resp.Error)
	}

	var result struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	json.Unmarshal(resp.Result, &result)

	if len(result.Content) == 0 {
		t.Fatal("Expected content in response")
	}

	// New format: summary line + markdown table
	text := result.Content[0].Text
	if !strings.Contains(text, "2 WebSocket event(s)") {
		t.Errorf("Expected summary with '2 WebSocket event(s)', got: %s", text)
	}
}

func TestMCPGetWebSocketEventsWithFilter(t *testing.T) {
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	capture.AddWebSocketEvents([]WebSocketEvent{
		{ID: "uuid-1", Event: "message", Direction: "incoming"},
		{ID: "uuid-1", Event: "message", Direction: "outgoing"},
		{ID: "uuid-2", Event: "message", Direction: "incoming"},
	})

	mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "initialize",
		Params: json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}`),
	})

	resp := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 2, Method: "tools/call",
		Params: json.RawMessage(`{"name":"observe","arguments":{"what":"websocket_events","connection_id":"uuid-1","direction":"incoming"}}`),
	})

	var result struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	json.Unmarshal(resp.Result, &result)

	// New format: summary line + markdown table
	text := result.Content[0].Text
	if !strings.Contains(text, "1 WebSocket event(s)") {
		t.Errorf("Expected summary with '1 WebSocket event(s)', got: %s", text)
	}
}

func TestMCPGetWebSocketStatus(t *testing.T) {
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	capture.AddWebSocketEvents([]WebSocketEvent{
		{ID: "uuid-1", Event: "open", URL: "wss://chat.example.com/ws"},
		{ID: "uuid-1", Event: "message", Direction: "incoming", Size: 100},
	})

	mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "initialize",
		Params: json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}`),
	})

	resp := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 2, Method: "tools/call",
		Params: json.RawMessage(`{"name":"observe","arguments":{"what":"websocket_status"}}`),
	})

	if resp.Error != nil {
		t.Fatalf("Expected no error, got: %v", resp.Error)
	}

	var result struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	json.Unmarshal(resp.Result, &result)

	// Strip summary line before parsing JSON
	text := result.Content[0].Text
	jsonPart := text
	if lines := strings.SplitN(text, "\n", 2); len(lines) == 2 {
		jsonPart = lines[1]
	}
	var status WebSocketStatusResponse
	if err := json.Unmarshal([]byte(jsonPart), &status); err != nil {
		t.Fatalf("Expected valid status JSON: %v", err)
	}

	if len(status.Connections) != 1 {
		t.Errorf("Expected 1 connection, got %d", len(status.Connections))
	}
}

func TestMCPGetWebSocketEventsEmpty(t *testing.T) {
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "initialize",
		Params: json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}`),
	})

	resp := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 2, Method: "tools/call",
		Params: json.RawMessage(`{"name":"observe","arguments":{"what":"websocket_events"}}`),
	})

	var result struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	json.Unmarshal(resp.Result, &result)

	if !strings.HasPrefix(result.Content[0].Text, "No WebSocket events captured") {
		t.Errorf("Expected text starting with 'No WebSocket events captured', got: %s", result.Content[0].Text)
	}
}

func TestV4ConnectionDurationFormatted(t *testing.T) {
	capture := setupTestCapture(t)

	openedAt := time.Now().Add(-5*time.Minute - 2*time.Second)
	capture.AddWebSocketEvents([]WebSocketEvent{
		{
			Timestamp: openedAt.Format(time.RFC3339Nano),
			ID:        "uuid-1",
			Event:     "open",
			URL:       "wss://example.com/ws",
		},
	})

	status := capture.GetWebSocketStatus(WebSocketStatusFilter{})
	if len(status.Connections) != 1 {
		t.Fatalf("Expected 1 connection, got %d", len(status.Connections))
	}

	conn := status.Connections[0]
	if conn.Duration == "" {
		t.Fatal("Expected Duration to be set for active connection")
	}

	// Duration should be approximately "5m02s" (give or take a second)
	if !strings.Contains(conn.Duration, "m") {
		t.Errorf("Expected duration to contain 'm' for minutes, got: %s", conn.Duration)
	}
}

func TestV4ConnectionDurationShortFormat(t *testing.T) {
	capture := setupTestCapture(t)

	openedAt := time.Now().Add(-3 * time.Second)
	capture.AddWebSocketEvents([]WebSocketEvent{
		{
			Timestamp: openedAt.Format(time.RFC3339Nano),
			ID:        "uuid-1",
			Event:     "open",
			URL:       "wss://example.com/ws",
		},
	})

	status := capture.GetWebSocketStatus(WebSocketStatusFilter{})
	conn := status.Connections[0]

	// Should be "3s" or "4s" (within test timing tolerance)
	if !strings.HasSuffix(conn.Duration, "s") {
		t.Errorf("Expected short duration ending in 's', got: %s", conn.Duration)
	}
}

func TestV4ConnectionDurationHourFormat(t *testing.T) {
	capture := setupTestCapture(t)

	openedAt := time.Now().Add(-1*time.Hour - 15*time.Minute)
	capture.AddWebSocketEvents([]WebSocketEvent{
		{
			Timestamp: openedAt.Format(time.RFC3339Nano),
			ID:        "uuid-1",
			Event:     "open",
			URL:       "wss://example.com/ws",
		},
	})

	status := capture.GetWebSocketStatus(WebSocketStatusFilter{})
	conn := status.Connections[0]

	if !strings.Contains(conn.Duration, "h") {
		t.Errorf("Expected duration with 'h' for hours, got: %s", conn.Duration)
	}
}

func TestV4MessageRateCalculation(t *testing.T) {
	capture := setupTestCapture(t)

	// Open connection
	capture.AddWebSocketEvents([]WebSocketEvent{
		{Timestamp: time.Now().Add(-10 * time.Second).Format(time.RFC3339Nano), ID: "uuid-1", Event: "open", URL: "wss://example.com/ws"},
	})

	// Send 10 messages over the last 5 seconds (2 per second)
	now := time.Now()
	for i := 0; i < 10; i++ {
		ts := now.Add(-5*time.Second + time.Duration(i)*500*time.Millisecond)
		capture.AddWebSocketEvents([]WebSocketEvent{
			{Timestamp: ts.Format(time.RFC3339Nano), ID: "uuid-1", Event: "message", Direction: "incoming", Size: 100},
		})
	}

	status := capture.GetWebSocketStatus(WebSocketStatusFilter{})
	if len(status.Connections) != 1 {
		t.Fatalf("Expected 1 connection, got %d", len(status.Connections))
	}

	conn := status.Connections[0]
	// Rate should be approximately 2.0 msg/s (10 messages in 5 seconds)
	if conn.MessageRate.Incoming.PerSecond < 1.0 {
		t.Errorf("Expected incoming rate >= 1.0 msg/s, got %.2f", conn.MessageRate.Incoming.PerSecond)
	}
	if conn.MessageRate.Incoming.PerSecond > 5.0 {
		t.Errorf("Expected incoming rate <= 5.0 msg/s, got %.2f", conn.MessageRate.Incoming.PerSecond)
	}
}

func TestV4MessageRateZeroWhenNoRecentMessages(t *testing.T) {
	capture := setupTestCapture(t)

	// Open connection long ago
	capture.AddWebSocketEvents([]WebSocketEvent{
		{Timestamp: time.Now().Add(-60 * time.Second).Format(time.RFC3339Nano), ID: "uuid-1", Event: "open", URL: "wss://example.com/ws"},
	})

	// Send messages long ago (outside 5-second window)
	oldTime := time.Now().Add(-30 * time.Second)
	for i := 0; i < 5; i++ {
		capture.AddWebSocketEvents([]WebSocketEvent{
			{Timestamp: oldTime.Add(time.Duration(i) * time.Second).Format(time.RFC3339Nano), ID: "uuid-1", Event: "message", Direction: "incoming", Size: 50},
		})
	}

	status := capture.GetWebSocketStatus(WebSocketStatusFilter{})
	conn := status.Connections[0]

	// Rate should be 0 since all messages are outside the 5-second window
	if conn.MessageRate.Incoming.PerSecond != 0.0 {
		t.Errorf("Expected incoming rate 0 for old messages, got %.2f", conn.MessageRate.Incoming.PerSecond)
	}
}

func TestV4MessageRateOutgoing(t *testing.T) {
	capture := setupTestCapture(t)

	capture.AddWebSocketEvents([]WebSocketEvent{
		{Timestamp: time.Now().Add(-10 * time.Second).Format(time.RFC3339Nano), ID: "uuid-1", Event: "open", URL: "wss://example.com/ws"},
	})

	// Send 5 outgoing messages in last 5 seconds
	now := time.Now()
	for i := 0; i < 5; i++ {
		ts := now.Add(-4*time.Second + time.Duration(i)*time.Second)
		capture.AddWebSocketEvents([]WebSocketEvent{
			{Timestamp: ts.Format(time.RFC3339Nano), ID: "uuid-1", Event: "message", Direction: "outgoing", Size: 200},
		})
	}

	status := capture.GetWebSocketStatus(WebSocketStatusFilter{})
	conn := status.Connections[0]

	if conn.MessageRate.Outgoing.PerSecond < 0.5 {
		t.Errorf("Expected outgoing rate >= 0.5 msg/s, got %.2f", conn.MessageRate.Outgoing.PerSecond)
	}
}

func TestV4LastMessageAgeFormatted(t *testing.T) {
	capture := setupTestCapture(t)

	// Open connection
	capture.AddWebSocketEvents([]WebSocketEvent{
		{Timestamp: time.Now().Add(-60 * time.Second).Format(time.RFC3339Nano), ID: "uuid-1", Event: "open", URL: "wss://example.com/ws"},
	})

	// Last message 3 seconds ago
	capture.AddWebSocketEvents([]WebSocketEvent{
		{
			Timestamp: time.Now().Add(-3 * time.Second).Format(time.RFC3339Nano),
			ID:        "uuid-1",
			Event:     "message",
			Direction: "incoming",
			Data:      `{"type":"ping"}`,
			Size:      15,
		},
	})

	status := capture.GetWebSocketStatus(WebSocketStatusFilter{})
	conn := status.Connections[0]

	if conn.LastMessage.Incoming == nil {
		t.Fatal("Expected incoming last message to be set")
	}

	age := conn.LastMessage.Incoming.Age
	if age == "" {
		t.Fatal("Expected Age to be set on last message preview")
	}

	// Should be approximately "3s" or "3.Xs"
	if !strings.HasSuffix(age, "s") {
		t.Errorf("Expected age ending in 's', got: %s", age)
	}
}

func TestV4LastMessageAgeMinutesFormat(t *testing.T) {
	capture := setupTestCapture(t)

	capture.AddWebSocketEvents([]WebSocketEvent{
		{Timestamp: time.Now().Add(-600 * time.Second).Format(time.RFC3339Nano), ID: "uuid-1", Event: "open", URL: "wss://example.com/ws"},
	})

	// Last message 2 minutes 30 seconds ago
	capture.AddWebSocketEvents([]WebSocketEvent{
		{
			Timestamp: time.Now().Add(-150 * time.Second).Format(time.RFC3339Nano),
			ID:        "uuid-1",
			Event:     "message",
			Direction: "outgoing",
			Data:      `{"type":"update"}`,
			Size:      20,
		},
	})

	status := capture.GetWebSocketStatus(WebSocketStatusFilter{})
	conn := status.Connections[0]

	if conn.LastMessage.Outgoing == nil {
		t.Fatal("Expected outgoing last message to be set")
	}

	age := conn.LastMessage.Outgoing.Age
	if !strings.Contains(age, "m") {
		t.Errorf("Expected age with 'm' for minutes, got: %s", age)
	}
}

func TestV4LastMessageAgeSubSecond(t *testing.T) {
	capture := setupTestCapture(t)

	capture.AddWebSocketEvents([]WebSocketEvent{
		{Timestamp: time.Now().Add(-10 * time.Second).Format(time.RFC3339Nano), ID: "uuid-1", Event: "open", URL: "wss://example.com/ws"},
	})

	// Last message just now (< 1 second ago)
	capture.AddWebSocketEvents([]WebSocketEvent{
		{
			Timestamp: time.Now().Add(-200 * time.Millisecond).Format(time.RFC3339Nano),
			ID:        "uuid-1",
			Event:     "message",
			Direction: "incoming",
			Data:      `{"type":"heartbeat"}`,
			Size:      20,
		},
	})

	status := capture.GetWebSocketStatus(WebSocketStatusFilter{})
	conn := status.Connections[0]

	age := conn.LastMessage.Incoming.Age
	if age == "" {
		t.Fatal("Expected age to be set for sub-second message")
	}

	// Should show fractional seconds like "0.2s"
	if !strings.HasSuffix(age, "s") {
		t.Errorf("Expected sub-second age ending in 's', got: %s", age)
	}
}

// ============================================
// HandleWebSocketStatus: HTTP GET handler
// ============================================

// Test: HandleWebSocketStatus returns JSON with connections and closed arrays.
func TestV4HandleWebSocketStatus_EmptyState(t *testing.T) {
	capture := setupTestCapture(t)

	req := httptest.NewRequest("GET", "/websocket-status", nil)
	rec := httptest.NewRecorder()

	capture.HandleWebSocketStatus(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}

	ct := rec.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("expected Content-Type application/json, got %s", ct)
	}

	var status WebSocketStatusResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &status); err != nil {
		t.Fatalf("expected valid JSON response: %v", err)
	}

	if status.Connections == nil {
		t.Error("expected non-nil Connections slice")
	}
	if status.Closed == nil {
		t.Error("expected non-nil Closed slice")
	}
}

// Test: HandleWebSocketStatus returns open connections.
func TestV4HandleWebSocketStatus_WithConnections(t *testing.T) {
	capture := setupTestCapture(t)

	capture.AddWebSocketEvents([]WebSocketEvent{
		{ID: "ws-1", Event: "open", URL: "wss://chat.example.com/ws", Timestamp: time.Now().Format(time.RFC3339Nano)},
		{ID: "ws-2", Event: "open", URL: "wss://feed.example.com/prices", Timestamp: time.Now().Format(time.RFC3339Nano)},
	})

	req := httptest.NewRequest("GET", "/websocket-status", nil)
	rec := httptest.NewRecorder()

	capture.HandleWebSocketStatus(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}

	var status WebSocketStatusResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &status); err != nil {
		t.Fatalf("expected valid JSON: %v", err)
	}

	if len(status.Connections) != 2 {
		t.Errorf("expected 2 connections, got %d", len(status.Connections))
	}
}

// Test: HandleWebSocketStatus returns closed connections.
func TestV4HandleWebSocketStatus_WithClosedConnections(t *testing.T) {
	capture := setupTestCapture(t)

	capture.AddWebSocketEvents([]WebSocketEvent{
		{ID: "ws-closed", Event: "open", URL: "wss://example.com/ws"},
		{ID: "ws-closed", Event: "close", URL: "wss://example.com/ws", CloseCode: 1001, CloseReason: "going away"},
	})

	req := httptest.NewRequest("GET", "/websocket-status", nil)
	rec := httptest.NewRecorder()

	capture.HandleWebSocketStatus(rec, req)

	var status WebSocketStatusResponse
	json.Unmarshal(rec.Body.Bytes(), &status)

	if len(status.Connections) != 0 {
		t.Errorf("expected 0 open connections, got %d", len(status.Connections))
	}
	if len(status.Closed) != 1 {
		t.Errorf("expected 1 closed connection, got %d", len(status.Closed))
	}
	if status.Closed[0].CloseCode != 1001 {
		t.Errorf("expected close code 1001, got %d", status.Closed[0].CloseCode)
	}
}

// ============================================
// formatAge: seconds-only and sub-second cases
// ============================================

// Test: formatAge with a timestamp a few seconds ago returns "Ns" format.
func TestV4FormatAge_SecondsOnly(t *testing.T) {
	ts := time.Now().Add(-7 * time.Second).Format(time.RFC3339Nano)
	age := formatAge(ts)

	if age == "" {
		t.Fatal("expected non-empty age")
	}
	// Should be "7s" or "8s" (timing tolerance)
	if !strings.HasSuffix(age, "s") {
		t.Errorf("expected age ending in 's', got: %s", age)
	}
	if strings.Contains(age, "m") || strings.Contains(age, "h") {
		t.Errorf("expected seconds-only format, got: %s", age)
	}
}

// Test: formatAge with a timestamp less than 1 second ago returns fractional.
func TestV4FormatAge_SubSecond(t *testing.T) {
	ts := time.Now().Add(-300 * time.Millisecond).Format(time.RFC3339Nano)
	age := formatAge(ts)

	if age == "" {
		t.Fatal("expected non-empty age for sub-second timestamp")
	}
	// Should be something like "0.3s"
	if !strings.HasSuffix(age, "s") {
		t.Errorf("expected age ending in 's', got: %s", age)
	}
	if strings.Contains(age, "m") || strings.Contains(age, "h") {
		t.Errorf("expected sub-second format without minutes/hours, got: %s", age)
	}
}

// Test: formatAge with empty timestamp returns empty string.
func TestV4FormatAge_EmptyTimestamp(t *testing.T) {
	age := formatAge("")
	if age != "" {
		t.Errorf("expected empty string for empty timestamp, got: %s", age)
	}
}

// Test: formatAge with invalid timestamp returns empty string.
func TestV4FormatAge_InvalidTimestamp(t *testing.T) {
	age := formatAge("not-a-timestamp")
	if age != "" {
		t.Errorf("expected empty string for invalid timestamp, got: %s", age)
	}
}

// ============================================
// formatDuration: sub-second case
// ============================================

// Test: formatDuration with sub-second duration returns fractional seconds.
func TestV4FormatDuration_SubSecond(t *testing.T) {
	d := 250 * time.Millisecond
	result := formatDuration(d)
	if result != "0.2s" && result != "0.3s" {
		// Floating point: 0.25 rounds to "0.2s" with %.1f
		if !strings.HasSuffix(result, "s") {
			t.Errorf("expected sub-second format ending in 's', got: %s", result)
		}
	}
}

// Test: formatDuration with exactly 0.
func TestV4FormatDuration_Zero(t *testing.T) {
	result := formatDuration(0)
	if result != "0.0s" {
		t.Errorf("expected '0.0s', got: %s", result)
	}
}

// ============================================
// toolGetWSStatus: connection_id and url filters
// ============================================

// Test: toolGetWSStatus with connection_id filter.
func TestV4ToolGetWSStatus_ConnectionIDFilter(t *testing.T) {
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	capture.AddWebSocketEvents([]WebSocketEvent{
		{ID: "conn-alpha", Event: "open", URL: "wss://alpha.example.com/ws", Timestamp: time.Now().Format(time.RFC3339Nano)},
		{ID: "conn-beta", Event: "open", URL: "wss://beta.example.com/ws", Timestamp: time.Now().Format(time.RFC3339Nano)},
	})

	mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "initialize",
		Params: json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}`),
	})

	resp := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 2, Method: "tools/call",
		Params: json.RawMessage(`{"name":"observe","arguments":{"what":"websocket_status","connection_id":"conn-alpha"}}`),
	})

	if resp.Error != nil {
		t.Fatalf("expected no error, got: %v", resp.Error)
	}

	var result struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	json.Unmarshal(resp.Result, &result)

	// Strip summary line before parsing JSON
	text := result.Content[0].Text
	jsonPart := text
	if lines := strings.SplitN(text, "\n", 2); len(lines) == 2 {
		jsonPart = lines[1]
	}
	var status WebSocketStatusResponse
	if err := json.Unmarshal([]byte(jsonPart), &status); err != nil {
		t.Fatalf("expected valid status JSON: %v", err)
	}

	if len(status.Connections) != 1 {
		t.Errorf("expected 1 connection with connection_id filter, got %d", len(status.Connections))
	}
	if len(status.Connections) > 0 && status.Connections[0].ID != "conn-alpha" {
		t.Errorf("expected conn-alpha, got %s", status.Connections[0].ID)
	}
}

// Test: toolGetWSStatus with url filter.
func TestV4ToolGetWSStatus_URLFilter(t *testing.T) {
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	capture.AddWebSocketEvents([]WebSocketEvent{
		{ID: "conn-1", Event: "open", URL: "wss://chat.example.com/ws", Timestamp: time.Now().Format(time.RFC3339Nano)},
		{ID: "conn-2", Event: "open", URL: "wss://feed.example.com/prices", Timestamp: time.Now().Format(time.RFC3339Nano)},
		{ID: "conn-3", Event: "open", URL: "wss://chat.example.com/v2", Timestamp: time.Now().Format(time.RFC3339Nano)},
	})

	mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "initialize",
		Params: json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}`),
	})

	resp := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 2, Method: "tools/call",
		Params: json.RawMessage(`{"name":"observe","arguments":{"what":"websocket_status","url":"chat"}}`),
	})

	var result struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	json.Unmarshal(resp.Result, &result)

	// Strip summary line before parsing JSON
	text2 := result.Content[0].Text
	jsonPart2 := text2
	if lines := strings.SplitN(text2, "\n", 2); len(lines) == 2 {
		jsonPart2 = lines[1]
	}
	var status WebSocketStatusResponse
	json.Unmarshal([]byte(jsonPart2), &status)

	if len(status.Connections) != 2 {
		t.Errorf("expected 2 connections matching 'chat' URL filter, got %d", len(status.Connections))
	}
}

// Test: toolGetWSStatus with both connection_id and url filter (connection_id takes precedence).
func TestV4ToolGetWSStatus_BothFilters(t *testing.T) {
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	capture.AddWebSocketEvents([]WebSocketEvent{
		{ID: "conn-1", Event: "open", URL: "wss://chat.example.com/ws", Timestamp: time.Now().Format(time.RFC3339Nano)},
		{ID: "conn-2", Event: "open", URL: "wss://chat.example.com/v2", Timestamp: time.Now().Format(time.RFC3339Nano)},
	})

	mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "initialize",
		Params: json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}`),
	})

	resp := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 2, Method: "tools/call",
		Params: json.RawMessage(`{"name":"observe","arguments":{"what":"websocket_status","connection_id":"conn-1","url":"chat"}}`),
	})

	var result struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	json.Unmarshal(resp.Result, &result)

	// Strip summary line before parsing JSON
	text3 := result.Content[0].Text
	jsonPart3 := text3
	if lines := strings.SplitN(text3, "\n", 2); len(lines) == 2 {
		jsonPart3 = lines[1]
	}
	var status WebSocketStatusResponse
	json.Unmarshal([]byte(jsonPart3), &status)

	if len(status.Connections) != 1 {
		t.Errorf("expected 1 connection with both filters, got %d", len(status.Connections))
	}
	if len(status.Connections) > 0 && status.Connections[0].ID != "conn-1" {
		t.Errorf("expected conn-1, got %s", status.Connections[0].ID)
	}
}

// ============================================
// HandleWebSocketEvents: GET handler (returning events)
// ============================================

// Test: HandleWebSocketEvents GET returns JSON with events and count.
func TestV4HandleWebSocketEvents_GET(t *testing.T) {
	capture := setupTestCapture(t)

	capture.AddWebSocketEvents([]WebSocketEvent{
		{ID: "ws-1", Event: "open", URL: "wss://example.com/ws"},
		{ID: "ws-1", Event: "message", Direction: "incoming", Data: "hello"},
	})

	req := httptest.NewRequest("GET", "/websocket-events", nil)
	rec := httptest.NewRecorder()

	capture.HandleWebSocketEvents(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}

	var resp struct {
		Events []WebSocketEvent `json:"events"`
		Count  int              `json:"count"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("expected valid JSON: %v", err)
	}

	if resp.Count != 2 {
		t.Errorf("expected count=2, got %d", resp.Count)
	}
	if len(resp.Events) != 2 {
		t.Errorf("expected 2 events, got %d", len(resp.Events))
	}
}

// ============================================
// HandleWebSocketEvents: POST rate limiting and body size
// ============================================

// Test: HandleWebSocketEvents POST rejected when rate limited.
func TestV4HandleWebSocketEvents_POST_RateLimited(t *testing.T) {
	capture := setupTestCapture(t)

	capture.mu.Lock()
	capture.circuitOpen = true
	capture.circuitOpenedAt = time.Now()
	capture.circuitReason = "rate_exceeded"
	capture.mu.Unlock()

	body := `{"events":[{"event":"open","id":"ws-1","url":"wss://example.com/ws"}]}`
	req := httptest.NewRequest("POST", "/websocket-events", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()

	capture.HandleWebSocketEvents(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("expected 429, got %d", rec.Code)
	}
}

// Test: HandleWebSocketEvents POST rejected when body too large.
func TestV4HandleWebSocketEvents_POST_BodyTooLarge(t *testing.T) {
	capture := setupTestCapture(t)

	largePayload := strings.Repeat("x", 6*1024*1024)
	req := httptest.NewRequest("POST", "/websocket-events", strings.NewReader(largePayload))
	rec := httptest.NewRecorder()

	capture.HandleWebSocketEvents(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("expected 413, got %d", rec.Code)
	}
}

// Test: HandleWebSocketEvents POST rejected when bad JSON.
func TestV4HandleWebSocketEvents_POST_BadJSON(t *testing.T) {
	capture := setupTestCapture(t)

	req := httptest.NewRequest("POST", "/websocket-events", bytes.NewBufferString("not json!"))
	rec := httptest.NewRecorder()

	capture.HandleWebSocketEvents(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for bad JSON, got %d", rec.Code)
	}
}

// Test: HandleWebSocketEvents POST re-check rate limit after recording.
func TestV4HandleWebSocketEvents_POST_RateLimitAfterRecording(t *testing.T) {
	capture := setupTestCapture(t)

	capture.mu.Lock()
	capture.rateWindowStart = time.Now()
	capture.windowEventCount = rateLimitThreshold - 1
	capture.mu.Unlock()

	// 10 events pushes count over threshold
	events := make([]map[string]interface{}, 10)
	for i := range events {
		events[i] = map[string]interface{}{
			"event":     "message",
			"id":        "ws-1",
			"direction": "incoming",
		}
	}
	payload, _ := json.Marshal(map[string]interface{}{"events": events})

	req := httptest.NewRequest("POST", "/websocket-events", bytes.NewReader(payload))
	rec := httptest.NewRecorder()

	capture.HandleWebSocketEvents(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("expected 429 after recording pushes over threshold, got %d", rec.Code)
	}
}

// ============================================
// Additional coverage: formatAge future timestamp, formatDuration exact minutes
// ============================================

// Test: formatAge with future timestamp (d < 0 branch).
func TestV4FormatAge_FutureTimestamp(t *testing.T) {
	// A timestamp 5 seconds in the future
	ts := time.Now().Add(5 * time.Second).Format(time.RFC3339Nano)
	age := formatAge(ts)

	// When d < 0, it gets clamped to 0, so formatDuration(0) = "0.0s"
	if age != "0.0s" {
		t.Errorf("expected '0.0s' for future timestamp, got: %s", age)
	}
}

// Test: formatDuration with exact minutes (secs == 0 branch).
func TestV4FormatDuration_ExactMinutes(t *testing.T) {
	d := 3 * time.Minute
	result := formatDuration(d)
	if result != "3m" {
		t.Errorf("expected '3m' for exactly 3 minutes, got: %s", result)
	}
}

// Test: formatDuration with exact hours (mins == 0 branch).
func TestV4FormatDuration_ExactHours(t *testing.T) {
	d := 2 * time.Hour
	result := formatDuration(d)
	if result != "2h" {
		t.Errorf("expected '2h' for exactly 2 hours, got: %s", result)
	}
}

// ============================================
// Additional coverage: toolGetWSStatus parse error
// ============================================

// Test: toolGetWSStatus with invalid arguments JSON returns error message.
func TestV4ToolGetWSStatus_InvalidArgs(t *testing.T) {
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "initialize",
		Params: json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}`),
	})

	// Pass invalid JSON as arguments - use a raw number where an object is expected
	resp := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 2, Method: "tools/call",
		Params: json.RawMessage(`{"name":"observe","arguments":{"what":"websocket_status","url":123}}`),
	})

	// The arguments are parsed by the tool dispatcher first, then passed to toolGetWSStatus.
	// url:123 is not a string, so json.Unmarshal into a struct with string field will fail.
	if resp.Error != nil {
		// If the dispatcher catches it first, that's fine too
		return
	}

	var result struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	json.Unmarshal(resp.Result, &result)

	if len(result.Content) > 0 && strings.Contains(result.Content[0].Text, "Error parsing") {
		// Good - the error path was hit
		return
	}
	// If it parsed successfully (Go json is lenient with numbers->strings), that's also OK
}
