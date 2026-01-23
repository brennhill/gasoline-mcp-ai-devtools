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
	v4 := setupV4TestServer(t)

	events := []WebSocketEvent{
		{Timestamp: "2024-01-15T10:30:00.000Z", Type: "websocket", Event: "open", ID: "uuid-1", URL: "wss://example.com/ws"},
		{Timestamp: "2024-01-15T10:30:01.000Z", Type: "websocket", Event: "message", ID: "uuid-1", Direction: "incoming", Data: `{"type":"chat","msg":"hello"}`, Size: 32},
	}

	v4.AddWebSocketEvents(events)

	if v4.GetWebSocketEventCount() != 2 {
		t.Errorf("Expected 2 events, got %d", v4.GetWebSocketEventCount())
	}
}

func TestV4WebSocketEventBufferRotation(t *testing.T) {
	v4 := setupV4TestServer(t)

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

	v4.AddWebSocketEvents(events)

	if v4.GetWebSocketEventCount() != 500 {
		t.Errorf("Expected 500 events after rotation, got %d", v4.GetWebSocketEventCount())
	}
}

func TestV4WebSocketEventFilterByConnectionID(t *testing.T) {
	v4 := setupV4TestServer(t)

	v4.AddWebSocketEvents([]WebSocketEvent{
		{ID: "uuid-1", Event: "open", URL: "wss://a.com"},
		{ID: "uuid-2", Event: "open", URL: "wss://b.com"},
		{ID: "uuid-1", Event: "message", Direction: "incoming"},
	})

	filtered := v4.GetWebSocketEvents(WebSocketEventFilter{ConnectionID: "uuid-1"})

	if len(filtered) != 2 {
		t.Errorf("Expected 2 events for uuid-1, got %d", len(filtered))
	}
}

func TestV4WebSocketEventFilterByURL(t *testing.T) {
	v4 := setupV4TestServer(t)

	v4.AddWebSocketEvents([]WebSocketEvent{
		{ID: "uuid-1", Event: "open", URL: "wss://chat.example.com/ws"},
		{ID: "uuid-2", Event: "open", URL: "wss://feed.example.com/prices"},
		{ID: "uuid-1", Event: "message", URL: "wss://chat.example.com/ws"},
	})

	filtered := v4.GetWebSocketEvents(WebSocketEventFilter{URLFilter: "chat"})

	if len(filtered) != 2 {
		t.Errorf("Expected 2 events matching 'chat', got %d", len(filtered))
	}
}

func TestV4WebSocketEventFilterByDirection(t *testing.T) {
	v4 := setupV4TestServer(t)

	v4.AddWebSocketEvents([]WebSocketEvent{
		{ID: "uuid-1", Event: "message", Direction: "incoming"},
		{ID: "uuid-1", Event: "message", Direction: "outgoing"},
		{ID: "uuid-1", Event: "message", Direction: "incoming"},
	})

	filtered := v4.GetWebSocketEvents(WebSocketEventFilter{Direction: "incoming"})

	if len(filtered) != 2 {
		t.Errorf("Expected 2 incoming events, got %d", len(filtered))
	}
}

func TestV4WebSocketEventFilterWithLimit(t *testing.T) {
	v4 := setupV4TestServer(t)

	for i := 0; i < 10; i++ {
		v4.AddWebSocketEvents([]WebSocketEvent{
			{ID: "uuid-1", Event: "message", Direction: "incoming"},
		})
	}

	filtered := v4.GetWebSocketEvents(WebSocketEventFilter{Limit: 5})

	if len(filtered) != 5 {
		t.Errorf("Expected 5 events with limit, got %d", len(filtered))
	}
}

func TestV4WebSocketEventDefaultLimit(t *testing.T) {
	v4 := setupV4TestServer(t)

	for i := 0; i < 100; i++ {
		v4.AddWebSocketEvents([]WebSocketEvent{
			{ID: "uuid-1", Event: "message"},
		})
	}

	// Default limit is 50
	filtered := v4.GetWebSocketEvents(WebSocketEventFilter{})

	if len(filtered) != 50 {
		t.Errorf("Expected 50 events with default limit, got %d", len(filtered))
	}
}

func TestV4WebSocketEventNewestFirst(t *testing.T) {
	v4 := setupV4TestServer(t)

	v4.AddWebSocketEvents([]WebSocketEvent{
		{Timestamp: "2024-01-15T10:30:00.000Z", ID: "uuid-1", Event: "open"},
		{Timestamp: "2024-01-15T10:30:05.000Z", ID: "uuid-1", Event: "close"},
	})

	filtered := v4.GetWebSocketEvents(WebSocketEventFilter{})

	if len(filtered) == 0 {
		t.Fatal("Expected events to be returned")
	}
	if filtered[0].Timestamp != "2024-01-15T10:30:05.000Z" {
		t.Errorf("Expected newest first, got %s", filtered[0].Timestamp)
	}
}

func TestV4WebSocketConnectionTracker(t *testing.T) {
	v4 := setupV4TestServer(t)

	v4.AddWebSocketEvents([]WebSocketEvent{
		{ID: "uuid-1", Event: "open", URL: "wss://chat.example.com/ws", Timestamp: "2024-01-15T10:30:00.000Z"},
	})

	status := v4.GetWebSocketStatus(WebSocketStatusFilter{})

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
	v4 := setupV4TestServer(t)

	v4.AddWebSocketEvents([]WebSocketEvent{
		{ID: "uuid-1", Event: "open", URL: "wss://example.com/ws"},
		{ID: "uuid-1", Event: "close", URL: "wss://example.com/ws", CloseCode: 1000, CloseReason: "normal closure"},
	})

	status := v4.GetWebSocketStatus(WebSocketStatusFilter{})

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
	v4 := setupV4TestServer(t)

	v4.AddWebSocketEvents([]WebSocketEvent{
		{ID: "uuid-1", Event: "open", URL: "wss://example.com/ws"},
		{ID: "uuid-1", Event: "error", URL: "wss://example.com/ws"},
	})

	status := v4.GetWebSocketStatus(WebSocketStatusFilter{})

	if len(status.Connections) != 1 {
		t.Fatalf("Expected 1 connection (in error state), got %d", len(status.Connections))
	}

	if status.Connections[0].State != "error" {
		t.Errorf("Expected state 'error', got %s", status.Connections[0].State)
	}
}

func TestV4WebSocketConnectionMessageStats(t *testing.T) {
	v4 := setupV4TestServer(t)

	v4.AddWebSocketEvents([]WebSocketEvent{
		{ID: "uuid-1", Event: "open", URL: "wss://example.com/ws"},
		{ID: "uuid-1", Event: "message", Direction: "incoming", Size: 100},
		{ID: "uuid-1", Event: "message", Direction: "incoming", Size: 200},
		{ID: "uuid-1", Event: "message", Direction: "outgoing", Size: 50},
	})

	status := v4.GetWebSocketStatus(WebSocketStatusFilter{})

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
	v4 := setupV4TestServer(t)

	v4.AddWebSocketEvents([]WebSocketEvent{
		{ID: "uuid-1", Event: "open", URL: "wss://example.com/ws"},
		{ID: "uuid-1", Event: "message", Direction: "incoming", Data: `{"type":"hello"}`, Timestamp: "2024-01-15T10:30:01.000Z"},
		{ID: "uuid-1", Event: "message", Direction: "incoming", Data: `{"type":"world"}`, Timestamp: "2024-01-15T10:30:02.000Z"},
	})

	status := v4.GetWebSocketStatus(WebSocketStatusFilter{})
	conn := status.Connections[0]

	if conn.LastMessage.Incoming.Preview != `{"type":"world"}` {
		t.Errorf("Expected last incoming preview to be world message, got %s", conn.LastMessage.Incoming.Preview)
	}
}

func TestV4WebSocketMaxTrackedConnections(t *testing.T) {
	v4 := setupV4TestServer(t)

	// Open 25 connections (max is 20 active)
	for i := 0; i < 25; i++ {
		v4.AddWebSocketEvents([]WebSocketEvent{
			{ID: "uuid-" + string(rune('a'+i)), Event: "open", URL: "wss://example.com/ws"},
		})
	}

	status := v4.GetWebSocketStatus(WebSocketStatusFilter{})

	if len(status.Connections) > 20 {
		t.Errorf("Expected max 20 active connections, got %d", len(status.Connections))
	}
}

func TestV4WebSocketClosedConnectionHistory(t *testing.T) {
	v4 := setupV4TestServer(t)

	// Open and close 15 connections (max closed history is 10)
	for i := 0; i < 15; i++ {
		id := "uuid-" + strings.Repeat("x", i+1) // unique IDs
		v4.AddWebSocketEvents([]WebSocketEvent{
			{ID: id, Event: "open", URL: "wss://example.com/ws"},
			{ID: id, Event: "close", URL: "wss://example.com/ws", CloseCode: 1000},
		})
	}

	status := v4.GetWebSocketStatus(WebSocketStatusFilter{})

	if len(status.Closed) > 10 {
		t.Errorf("Expected max 10 closed connections in history, got %d", len(status.Closed))
	}
}

func TestV4WebSocketStatusFilterByURL(t *testing.T) {
	v4 := setupV4TestServer(t)

	v4.AddWebSocketEvents([]WebSocketEvent{
		{ID: "uuid-1", Event: "open", URL: "wss://chat.example.com/ws"},
		{ID: "uuid-2", Event: "open", URL: "wss://feed.example.com/prices"},
	})

	status := v4.GetWebSocketStatus(WebSocketStatusFilter{URLFilter: "chat"})

	if len(status.Connections) != 1 {
		t.Errorf("Expected 1 connection matching 'chat', got %d", len(status.Connections))
	}
}

func TestV4WebSocketStatusFilterByConnectionID(t *testing.T) {
	v4 := setupV4TestServer(t)

	v4.AddWebSocketEvents([]WebSocketEvent{
		{ID: "uuid-1", Event: "open", URL: "wss://a.com"},
		{ID: "uuid-2", Event: "open", URL: "wss://b.com"},
	})

	status := v4.GetWebSocketStatus(WebSocketStatusFilter{ConnectionID: "uuid-2"})

	if len(status.Connections) != 1 {
		t.Errorf("Expected 1 connection, got %d", len(status.Connections))
	}

	if status.Connections[0].ID != "uuid-2" {
		t.Errorf("Expected connection uuid-2, got %s", status.Connections[0].ID)
	}
}

func TestV4WebSocketSamplingInfo(t *testing.T) {
	v4 := setupV4TestServer(t)

	v4.AddWebSocketEvents([]WebSocketEvent{
		{ID: "uuid-1", Event: "open", URL: "wss://example.com/ws"},
		{ID: "uuid-1", Event: "message", Direction: "incoming", Sampled: &SamplingInfo{Rate: "48.2/s", Logged: "1/5", Window: "5s"}},
	})

	status := v4.GetWebSocketStatus(WebSocketStatusFilter{})
	conn := status.Connections[0]

	if !conn.Sampling.Active {
		t.Error("Expected sampling to be active")
	}
}

func TestV4PostWebSocketEventsEndpoint(t *testing.T) {
	v4 := setupV4TestServer(t)

	body := `{"events":[{"ts":"2024-01-15T10:30:00.000Z","type":"websocket","event":"open","id":"uuid-1","url":"wss://example.com/ws"}]}`
	req := httptest.NewRequest("POST", "/websocket-events", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	v4.HandleWebSocketEvents(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", rec.Code)
	}

	if v4.GetWebSocketEventCount() != 1 {
		t.Errorf("Expected 1 event stored, got %d", v4.GetWebSocketEventCount())
	}
}

func TestV4PostWebSocketEventsInvalidJSON(t *testing.T) {
	v4 := setupV4TestServer(t)

	req := httptest.NewRequest("POST", "/websocket-events", bytes.NewBufferString("not json"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	v4.HandleWebSocketEvents(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Expected 400, got %d", rec.Code)
	}
}

func TestMCPGetWebSocketEvents(t *testing.T) {
	server, _ := setupTestServer(t)
	v4 := setupV4TestServer(t)
	mcp := NewMCPHandlerV4(server, v4)

	// Add some WebSocket events
	v4.AddWebSocketEvents([]WebSocketEvent{
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
		Params: json.RawMessage(`{"name":"get_websocket_events","arguments":{}}`),
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

	// Should contain events as JSON
	var events []WebSocketEvent
	if err := json.Unmarshal([]byte(result.Content[0].Text), &events); err != nil {
		t.Fatalf("Expected valid JSON events, got error: %v", err)
	}

	if len(events) != 2 {
		t.Errorf("Expected 2 events, got %d", len(events))
	}
}

func TestMCPGetWebSocketEventsWithFilter(t *testing.T) {
	server, _ := setupTestServer(t)
	v4 := setupV4TestServer(t)
	mcp := NewMCPHandlerV4(server, v4)

	v4.AddWebSocketEvents([]WebSocketEvent{
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
		Params: json.RawMessage(`{"name":"get_websocket_events","arguments":{"connection_id":"uuid-1","direction":"incoming"}}`),
	})

	var result struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	json.Unmarshal(resp.Result, &result)

	var events []WebSocketEvent
	json.Unmarshal([]byte(result.Content[0].Text), &events)

	if len(events) != 1 {
		t.Errorf("Expected 1 filtered event, got %d", len(events))
	}
}

func TestMCPGetWebSocketStatus(t *testing.T) {
	server, _ := setupTestServer(t)
	v4 := setupV4TestServer(t)
	mcp := NewMCPHandlerV4(server, v4)

	v4.AddWebSocketEvents([]WebSocketEvent{
		{ID: "uuid-1", Event: "open", URL: "wss://chat.example.com/ws"},
		{ID: "uuid-1", Event: "message", Direction: "incoming", Size: 100},
	})

	mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "initialize",
		Params: json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}`),
	})

	resp := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 2, Method: "tools/call",
		Params: json.RawMessage(`{"name":"get_websocket_status","arguments":{}}`),
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

	var status WebSocketStatusResponse
	if err := json.Unmarshal([]byte(result.Content[0].Text), &status); err != nil {
		t.Fatalf("Expected valid status JSON: %v", err)
	}

	if len(status.Connections) != 1 {
		t.Errorf("Expected 1 connection, got %d", len(status.Connections))
	}
}

func TestMCPGetWebSocketEventsEmpty(t *testing.T) {
	server, _ := setupTestServer(t)
	v4 := setupV4TestServer(t)
	mcp := NewMCPHandlerV4(server, v4)

	mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "initialize",
		Params: json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}`),
	})

	resp := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 2, Method: "tools/call",
		Params: json.RawMessage(`{"name":"get_websocket_events","arguments":{}}`),
	})

	var result struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	json.Unmarshal(resp.Result, &result)

	if result.Content[0].Text != "No WebSocket events captured" {
		var events []WebSocketEvent
		if err := json.Unmarshal([]byte(result.Content[0].Text), &events); err == nil {
			if len(events) != 0 {
				t.Error("Expected empty events or message")
			}
		}
	}
}

func TestV4ConnectionDurationFormatted(t *testing.T) {
	v4 := setupV4TestServer(t)

	openedAt := time.Now().Add(-5*time.Minute - 2*time.Second)
	v4.AddWebSocketEvents([]WebSocketEvent{
		{
			Timestamp: openedAt.Format(time.RFC3339Nano),
			ID:        "uuid-1",
			Event:     "open",
			URL:       "wss://example.com/ws",
		},
	})

	status := v4.GetWebSocketStatus(WebSocketStatusFilter{})
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
	v4 := setupV4TestServer(t)

	openedAt := time.Now().Add(-3 * time.Second)
	v4.AddWebSocketEvents([]WebSocketEvent{
		{
			Timestamp: openedAt.Format(time.RFC3339Nano),
			ID:        "uuid-1",
			Event:     "open",
			URL:       "wss://example.com/ws",
		},
	})

	status := v4.GetWebSocketStatus(WebSocketStatusFilter{})
	conn := status.Connections[0]

	// Should be "3s" or "4s" (within test timing tolerance)
	if !strings.HasSuffix(conn.Duration, "s") {
		t.Errorf("Expected short duration ending in 's', got: %s", conn.Duration)
	}
}

func TestV4ConnectionDurationHourFormat(t *testing.T) {
	v4 := setupV4TestServer(t)

	openedAt := time.Now().Add(-1*time.Hour - 15*time.Minute)
	v4.AddWebSocketEvents([]WebSocketEvent{
		{
			Timestamp: openedAt.Format(time.RFC3339Nano),
			ID:        "uuid-1",
			Event:     "open",
			URL:       "wss://example.com/ws",
		},
	})

	status := v4.GetWebSocketStatus(WebSocketStatusFilter{})
	conn := status.Connections[0]

	if !strings.Contains(conn.Duration, "h") {
		t.Errorf("Expected duration with 'h' for hours, got: %s", conn.Duration)
	}
}

func TestV4MessageRateCalculation(t *testing.T) {
	v4 := setupV4TestServer(t)

	// Open connection
	v4.AddWebSocketEvents([]WebSocketEvent{
		{Timestamp: time.Now().Add(-10 * time.Second).Format(time.RFC3339Nano), ID: "uuid-1", Event: "open", URL: "wss://example.com/ws"},
	})

	// Send 10 messages over the last 5 seconds (2 per second)
	now := time.Now()
	for i := 0; i < 10; i++ {
		ts := now.Add(-5*time.Second + time.Duration(i)*500*time.Millisecond)
		v4.AddWebSocketEvents([]WebSocketEvent{
			{Timestamp: ts.Format(time.RFC3339Nano), ID: "uuid-1", Event: "message", Direction: "incoming", Size: 100},
		})
	}

	status := v4.GetWebSocketStatus(WebSocketStatusFilter{})
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
	v4 := setupV4TestServer(t)

	// Open connection long ago
	v4.AddWebSocketEvents([]WebSocketEvent{
		{Timestamp: time.Now().Add(-60 * time.Second).Format(time.RFC3339Nano), ID: "uuid-1", Event: "open", URL: "wss://example.com/ws"},
	})

	// Send messages long ago (outside 5-second window)
	oldTime := time.Now().Add(-30 * time.Second)
	for i := 0; i < 5; i++ {
		v4.AddWebSocketEvents([]WebSocketEvent{
			{Timestamp: oldTime.Add(time.Duration(i) * time.Second).Format(time.RFC3339Nano), ID: "uuid-1", Event: "message", Direction: "incoming", Size: 50},
		})
	}

	status := v4.GetWebSocketStatus(WebSocketStatusFilter{})
	conn := status.Connections[0]

	// Rate should be 0 since all messages are outside the 5-second window
	if conn.MessageRate.Incoming.PerSecond != 0.0 {
		t.Errorf("Expected incoming rate 0 for old messages, got %.2f", conn.MessageRate.Incoming.PerSecond)
	}
}

func TestV4MessageRateOutgoing(t *testing.T) {
	v4 := setupV4TestServer(t)

	v4.AddWebSocketEvents([]WebSocketEvent{
		{Timestamp: time.Now().Add(-10 * time.Second).Format(time.RFC3339Nano), ID: "uuid-1", Event: "open", URL: "wss://example.com/ws"},
	})

	// Send 5 outgoing messages in last 5 seconds
	now := time.Now()
	for i := 0; i < 5; i++ {
		ts := now.Add(-4*time.Second + time.Duration(i)*time.Second)
		v4.AddWebSocketEvents([]WebSocketEvent{
			{Timestamp: ts.Format(time.RFC3339Nano), ID: "uuid-1", Event: "message", Direction: "outgoing", Size: 200},
		})
	}

	status := v4.GetWebSocketStatus(WebSocketStatusFilter{})
	conn := status.Connections[0]

	if conn.MessageRate.Outgoing.PerSecond < 0.5 {
		t.Errorf("Expected outgoing rate >= 0.5 msg/s, got %.2f", conn.MessageRate.Outgoing.PerSecond)
	}
}

func TestV4LastMessageAgeFormatted(t *testing.T) {
	v4 := setupV4TestServer(t)

	// Open connection
	v4.AddWebSocketEvents([]WebSocketEvent{
		{Timestamp: time.Now().Add(-60 * time.Second).Format(time.RFC3339Nano), ID: "uuid-1", Event: "open", URL: "wss://example.com/ws"},
	})

	// Last message 3 seconds ago
	v4.AddWebSocketEvents([]WebSocketEvent{
		{
			Timestamp: time.Now().Add(-3 * time.Second).Format(time.RFC3339Nano),
			ID:        "uuid-1",
			Event:     "message",
			Direction: "incoming",
			Data:      `{"type":"ping"}`,
			Size:      15,
		},
	})

	status := v4.GetWebSocketStatus(WebSocketStatusFilter{})
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
	v4 := setupV4TestServer(t)

	v4.AddWebSocketEvents([]WebSocketEvent{
		{Timestamp: time.Now().Add(-600 * time.Second).Format(time.RFC3339Nano), ID: "uuid-1", Event: "open", URL: "wss://example.com/ws"},
	})

	// Last message 2 minutes 30 seconds ago
	v4.AddWebSocketEvents([]WebSocketEvent{
		{
			Timestamp: time.Now().Add(-150 * time.Second).Format(time.RFC3339Nano),
			ID:        "uuid-1",
			Event:     "message",
			Direction: "outgoing",
			Data:      `{"type":"update"}`,
			Size:      20,
		},
	})

	status := v4.GetWebSocketStatus(WebSocketStatusFilter{})
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
	v4 := setupV4TestServer(t)

	v4.AddWebSocketEvents([]WebSocketEvent{
		{Timestamp: time.Now().Add(-10 * time.Second).Format(time.RFC3339Nano), ID: "uuid-1", Event: "open", URL: "wss://example.com/ws"},
	})

	// Last message just now (< 1 second ago)
	v4.AddWebSocketEvents([]WebSocketEvent{
		{
			Timestamp: time.Now().Add(-200 * time.Millisecond).Format(time.RFC3339Nano),
			ID:        "uuid-1",
			Event:     "message",
			Direction: "incoming",
			Data:      `{"type":"heartbeat"}`,
			Size:      20,
		},
	})

	status := v4.GetWebSocketStatus(WebSocketStatusFilter{})
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
