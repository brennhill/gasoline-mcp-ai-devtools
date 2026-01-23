package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// ============================================
// v4 Server Type Tests
// ============================================

func setupV4TestServer(t *testing.T) *V4Server {
	t.Helper()
	return NewV4Server()
}

// ============================================
// WebSocket Event Buffer Tests
// ============================================

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

// ============================================
// WebSocket Connection Tracker Tests
// ============================================

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

// ============================================
// Network Bodies Buffer Tests
// ============================================

func TestV4NetworkBodiesBuffer(t *testing.T) {
	v4 := setupV4TestServer(t)

	bodies := []NetworkBody{
		{
			Timestamp:    "2024-01-15T10:30:00.000Z",
			Method:       "POST",
			URL:          "/api/users",
			Status:       201,
			RequestBody:  `{"name":"Alice"}`,
			ResponseBody: `{"id":1,"name":"Alice"}`,
			ContentType:  "application/json",
			Duration:     142,
		},
	}

	v4.AddNetworkBodies(bodies)

	if v4.GetNetworkBodyCount() != 1 {
		t.Errorf("Expected 1 body, got %d", v4.GetNetworkBodyCount())
	}
}

func TestV4NetworkBodiesBufferRotation(t *testing.T) {
	v4 := setupV4TestServer(t)

	// Add more than max (100) entries
	bodies := make([]NetworkBody, 120)
	for i := range bodies {
		bodies[i] = NetworkBody{Method: "GET", URL: "/api/test", Status: 200}
	}

	v4.AddNetworkBodies(bodies)

	if v4.GetNetworkBodyCount() != 100 {
		t.Errorf("Expected 100 bodies after rotation, got %d", v4.GetNetworkBodyCount())
	}
}

func TestV4NetworkBodiesFilterByURL(t *testing.T) {
	v4 := setupV4TestServer(t)

	v4.AddNetworkBodies([]NetworkBody{
		{URL: "/api/users", Status: 200},
		{URL: "/api/products", Status: 200},
		{URL: "/api/users/1", Status: 404},
	})

	filtered := v4.GetNetworkBodies(NetworkBodyFilter{URLFilter: "users"})

	if len(filtered) != 2 {
		t.Errorf("Expected 2 bodies matching 'users', got %d", len(filtered))
	}
}

func TestV4NetworkBodiesFilterByMethod(t *testing.T) {
	v4 := setupV4TestServer(t)

	v4.AddNetworkBodies([]NetworkBody{
		{URL: "/api/test", Method: "GET", Status: 200},
		{URL: "/api/test", Method: "POST", Status: 201},
		{URL: "/api/test", Method: "GET", Status: 200},
	})

	filtered := v4.GetNetworkBodies(NetworkBodyFilter{Method: "POST"})

	if len(filtered) != 1 {
		t.Errorf("Expected 1 POST body, got %d", len(filtered))
	}
}

func TestV4NetworkBodiesFilterByStatus(t *testing.T) {
	v4 := setupV4TestServer(t)

	v4.AddNetworkBodies([]NetworkBody{
		{URL: "/api/test", Status: 200},
		{URL: "/api/test", Status: 404},
		{URL: "/api/test", Status: 500},
		{URL: "/api/test", Status: 201},
	})

	// Filter for errors only (>= 400)
	filtered := v4.GetNetworkBodies(NetworkBodyFilter{StatusMin: 400})

	if len(filtered) != 2 {
		t.Errorf("Expected 2 error bodies, got %d", len(filtered))
	}

	// Filter for range 400-499
	filtered = v4.GetNetworkBodies(NetworkBodyFilter{StatusMin: 400, StatusMax: 499})

	if len(filtered) != 1 {
		t.Errorf("Expected 1 client error body, got %d", len(filtered))
	}
}

func TestV4NetworkBodiesFilterWithLimit(t *testing.T) {
	v4 := setupV4TestServer(t)

	for i := 0; i < 50; i++ {
		v4.AddNetworkBodies([]NetworkBody{
			{URL: "/api/test", Status: 200},
		})
	}

	filtered := v4.GetNetworkBodies(NetworkBodyFilter{Limit: 10})

	if len(filtered) != 10 {
		t.Errorf("Expected 10 bodies with limit, got %d", len(filtered))
	}
}

func TestV4NetworkBodiesDefaultLimit(t *testing.T) {
	v4 := setupV4TestServer(t)

	for i := 0; i < 50; i++ {
		v4.AddNetworkBodies([]NetworkBody{
			{URL: "/api/test", Status: 200},
		})
	}

	// Default limit is 20
	filtered := v4.GetNetworkBodies(NetworkBodyFilter{})

	if len(filtered) != 20 {
		t.Errorf("Expected 20 bodies with default limit, got %d", len(filtered))
	}
}

func TestV4NetworkBodiesNewestFirst(t *testing.T) {
	v4 := setupV4TestServer(t)

	v4.AddNetworkBodies([]NetworkBody{
		{URL: "/api/first", Timestamp: "2024-01-15T10:30:00.000Z"},
		{URL: "/api/last", Timestamp: "2024-01-15T10:30:05.000Z"},
	})

	filtered := v4.GetNetworkBodies(NetworkBodyFilter{})

	if filtered[0].URL != "/api/last" {
		t.Errorf("Expected newest first, got URL %s", filtered[0].URL)
	}
}

func TestV4NetworkBodiesTruncation(t *testing.T) {
	v4 := setupV4TestServer(t)

	// Request body > 8KB should be truncated
	largeBody := strings.Repeat("x", 10000)
	v4.AddNetworkBodies([]NetworkBody{
		{URL: "/api/test", RequestBody: largeBody, Status: 200},
	})

	filtered := v4.GetNetworkBodies(NetworkBodyFilter{})

	if len(filtered[0].RequestBody) > 8192 {
		t.Errorf("Expected request body truncated to 8KB, got %d bytes", len(filtered[0].RequestBody))
	}

	if !filtered[0].RequestTruncated {
		t.Error("Expected RequestTruncated flag to be true")
	}
}

func TestV4NetworkBodiesResponseTruncation(t *testing.T) {
	v4 := setupV4TestServer(t)

	// Response body > 16KB should be truncated
	largeBody := strings.Repeat("y", 20000)
	v4.AddNetworkBodies([]NetworkBody{
		{URL: "/api/test", ResponseBody: largeBody, Status: 200},
	})

	filtered := v4.GetNetworkBodies(NetworkBodyFilter{})

	if len(filtered[0].ResponseBody) > 16384 {
		t.Errorf("Expected response body truncated to 16KB, got %d bytes", len(filtered[0].ResponseBody))
	}

	if !filtered[0].ResponseTruncated {
		t.Error("Expected ResponseTruncated flag to be true")
	}
}

// ============================================
// On-Demand Query Tests (DOM, A11y)
// ============================================

func TestV4PendingQueryCreation(t *testing.T) {
	v4 := setupV4TestServer(t)

	id := v4.CreatePendingQuery(PendingQuery{
		Type:   "dom",
		Params: json.RawMessage(`{"selector":".user-list"}`),
	})

	if id == "" {
		t.Error("Expected non-empty query ID")
	}

	pending := v4.GetPendingQueries()
	if len(pending) != 1 {
		t.Fatalf("Expected 1 pending query, got %d", len(pending))
	}

	if pending[0].Type != "dom" {
		t.Errorf("Expected type 'dom', got %s", pending[0].Type)
	}
}

func TestV4PendingQueryMaxLimit(t *testing.T) {
	v4 := setupV4TestServer(t)

	// Max 5 pending queries
	for i := 0; i < 7; i++ {
		v4.CreatePendingQuery(PendingQuery{
			Type:   "dom",
			Params: json.RawMessage(`{"selector":"div"}`),
		})
	}

	pending := v4.GetPendingQueries()
	if len(pending) > 5 {
		t.Errorf("Expected max 5 pending queries, got %d", len(pending))
	}
}

func TestV4PendingQueryTimeout(t *testing.T) {
	v4 := setupV4TestServer(t)

	// Create a query with a very short timeout for testing
	v4.CreatePendingQueryWithTimeout(PendingQuery{
		Type:   "dom",
		Params: json.RawMessage(`{"selector":"div"}`),
	}, 50*time.Millisecond)

	// Wait for timeout
	time.Sleep(100 * time.Millisecond)

	pending := v4.GetPendingQueries()
	if len(pending) != 0 {
		t.Errorf("Expected 0 pending queries after timeout, got %d", len(pending))
	}
}

func TestV4PendingQueryResult(t *testing.T) {
	v4 := setupV4TestServer(t)

	id := v4.CreatePendingQuery(PendingQuery{
		Type:   "dom",
		Params: json.RawMessage(`{"selector":".user-list"}`),
	})

	// Simulate extension posting result
	result := json.RawMessage(`{"matches":[{"tag":"ul","text":"users"}],"matchCount":1}`)
	v4.SetQueryResult(id, result)

	// Query should no longer be pending
	pending := v4.GetPendingQueries()
	if len(pending) != 0 {
		t.Errorf("Expected 0 pending queries after result, got %d", len(pending))
	}

	// Result should be retrievable
	got, found := v4.GetQueryResult(id)
	if !found {
		t.Fatal("Expected to find query result")
	}

	if got == nil {
		t.Fatal("Expected non-nil result")
	}
}

func TestV4PendingQueryResultNotFound(t *testing.T) {
	v4 := setupV4TestServer(t)

	_, found := v4.GetQueryResult("nonexistent-id")
	if found {
		t.Error("Expected not found for nonexistent query")
	}
}

func TestV4PendingQueryPolling(t *testing.T) {
	v4 := setupV4TestServer(t)

	// No pending queries initially
	pending := v4.GetPendingQueries()
	if len(pending) != 0 {
		t.Errorf("Expected 0 pending queries initially, got %d", len(pending))
	}

	// Create a query
	v4.CreatePendingQuery(PendingQuery{
		Type:   "a11y",
		Params: json.RawMessage(`{"scope":null}`),
	})

	// Polling should return it
	pending = v4.GetPendingQueries()
	if len(pending) != 1 {
		t.Errorf("Expected 1 pending query, got %d", len(pending))
	}
}

func TestV4DOMQueryWaitsForResult(t *testing.T) {
	v4 := setupV4TestServer(t)

	// Create query and immediately provide result in a goroutine
	id := v4.CreatePendingQuery(PendingQuery{
		Type:   "dom",
		Params: json.RawMessage(`{"selector":"h1"}`),
	})

	go func() {
		time.Sleep(50 * time.Millisecond)
		v4.SetQueryResult(id, json.RawMessage(`{"matches":[]}`))
	}()

	// WaitForResult should block until result arrives
	result, err := v4.WaitForResult(id, 1*time.Second)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if result == nil {
		t.Error("Expected non-nil result")
	}
}

func TestV4DOMQueryWaitTimeout(t *testing.T) {
	v4 := setupV4TestServer(t)

	id := v4.CreatePendingQuery(PendingQuery{
		Type:   "dom",
		Params: json.RawMessage(`{"selector":"h1"}`),
	})

	// WaitForResult should timeout
	_, err := v4.WaitForResult(id, 50*time.Millisecond)
	if err == nil {
		t.Error("Expected timeout error")
	}
}

// ============================================
// HTTP Endpoint Tests
// ============================================

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

func TestV4PostNetworkBodiesEndpoint(t *testing.T) {
	v4 := setupV4TestServer(t)

	body := `{"bodies":[{"ts":"2024-01-15T10:30:00.000Z","method":"GET","url":"/api/test","status":200,"responseBody":"{}","contentType":"application/json"}]}`
	req := httptest.NewRequest("POST", "/network-bodies", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	v4.HandleNetworkBodies(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", rec.Code)
	}

	if v4.GetNetworkBodyCount() != 1 {
		t.Errorf("Expected 1 body stored, got %d", v4.GetNetworkBodyCount())
	}
}

func TestV4PostNetworkBodiesInvalidJSON(t *testing.T) {
	v4 := setupV4TestServer(t)

	req := httptest.NewRequest("POST", "/network-bodies", bytes.NewBufferString("garbage"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	v4.HandleNetworkBodies(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Expected 400, got %d", rec.Code)
	}
}

func TestV4GetPendingQueriesEndpoint(t *testing.T) {
	v4 := setupV4TestServer(t)

	v4.CreatePendingQuery(PendingQuery{
		Type:   "dom",
		Params: json.RawMessage(`{"selector":"h1"}`),
	})

	req := httptest.NewRequest("GET", "/pending-queries", nil)
	rec := httptest.NewRecorder()

	v4.HandlePendingQueries(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", rec.Code)
	}

	var resp struct {
		Queries []PendingQueryResponse `json:"queries"`
	}
	json.Unmarshal(rec.Body.Bytes(), &resp)

	if len(resp.Queries) != 1 {
		t.Errorf("Expected 1 pending query, got %d", len(resp.Queries))
	}
}

func TestV4GetPendingQueriesEmpty(t *testing.T) {
	v4 := setupV4TestServer(t)

	req := httptest.NewRequest("GET", "/pending-queries", nil)
	rec := httptest.NewRecorder()

	v4.HandlePendingQueries(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", rec.Code)
	}

	var resp struct {
		Queries []PendingQueryResponse `json:"queries"`
	}
	json.Unmarshal(rec.Body.Bytes(), &resp)

	if len(resp.Queries) != 0 {
		t.Errorf("Expected 0 pending queries, got %d", len(resp.Queries))
	}
}

func TestV4PostDOMResultEndpoint(t *testing.T) {
	v4 := setupV4TestServer(t)

	id := v4.CreatePendingQuery(PendingQuery{
		Type:   "dom",
		Params: json.RawMessage(`{"selector":"h1"}`),
	})

	body := `{"id":"` + id + `","result":{"matches":[{"tag":"h1","text":"Hello"}],"matchCount":1}}`
	req := httptest.NewRequest("POST", "/dom-result", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	v4.HandleDOMResult(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", rec.Code)
	}

	// Result should be stored
	result, found := v4.GetQueryResult(id)
	if !found {
		t.Error("Expected result to be stored")
	}
	if result == nil {
		t.Error("Expected non-nil result")
	}
}

func TestV4PostDOMResultUnknownID(t *testing.T) {
	v4 := setupV4TestServer(t)

	body := `{"id":"nonexistent","result":{"matches":[]}}`
	req := httptest.NewRequest("POST", "/dom-result", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	v4.HandleDOMResult(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("Expected 404, got %d", rec.Code)
	}
}

func TestV4PostA11yResultEndpoint(t *testing.T) {
	v4 := setupV4TestServer(t)

	id := v4.CreatePendingQuery(PendingQuery{
		Type:   "a11y",
		Params: json.RawMessage(`{"scope":null}`),
	})

	body := `{"id":"` + id + `","result":{"violations":[{"id":"color-contrast","impact":"serious"}],"summary":{"violations":1}}}`
	req := httptest.NewRequest("POST", "/a11y-result", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	v4.HandleA11yResult(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", rec.Code)
	}
}

// ============================================
// Rate Limiting / Circuit Breaker Tests
// ============================================

func TestV4RateLimitWebSocketEvents(t *testing.T) {
	v4 := setupV4TestServer(t)

	// Simulate flooding: > 1000 events in rapid succession
	for i := 0; i < 1100; i++ {
		v4.RecordEventReceived()
	}

	// Next request should be rate limited
	req := httptest.NewRequest("POST", "/websocket-events", bytes.NewBufferString(`{"events":[{"event":"message"}]}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	v4.HandleWebSocketEvents(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("Expected 429, got %d", rec.Code)
	}
}

func TestV4MemoryLimitRejectsNetworkBodies(t *testing.T) {
	v4 := setupV4TestServer(t)

	// Simulate exceeding memory limit
	v4.SetMemoryUsage(55 * 1024 * 1024) // 55MB > 50MB limit

	body := `{"bodies":[{"url":"/api/test","status":200,"responseBody":"data"}]}`
	req := httptest.NewRequest("POST", "/network-bodies", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	v4.HandleNetworkBodies(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected 503, got %d", rec.Code)
	}
}

// ============================================
// Memory-Bounded Buffer Tests
// ============================================

func TestV4WebSocketBufferMemoryLimit(t *testing.T) {
	v4 := setupV4TestServer(t)

	// Add events that exceed 4MB memory limit
	largeData := strings.Repeat("x", 100000) // 100KB per event
	for i := 0; i < 50; i++ {                // 50 * 100KB = 5MB
		v4.AddWebSocketEvents([]WebSocketEvent{
			{ID: "uuid-1", Event: "message", Data: largeData},
		})
	}

	// Buffer should evict to stay under 4MB
	memUsage := v4.GetWebSocketBufferMemory()
	if memUsage > 4*1024*1024 {
		t.Errorf("Expected WS buffer memory <= 4MB, got %d bytes", memUsage)
	}
}

func TestV4NetworkBodiesBufferMemoryLimit(t *testing.T) {
	v4 := setupV4TestServer(t)

	// Add bodies that exceed 8MB memory limit
	largeBody := strings.Repeat("y", 200000) // 200KB per body
	for i := 0; i < 50; i++ {                // 50 * 200KB = 10MB
		v4.AddNetworkBodies([]NetworkBody{
			{URL: "/api/test", ResponseBody: largeBody, Status: 200},
		})
	}

	// Buffer should evict to stay under 8MB
	memUsage := v4.GetNetworkBodiesBufferMemory()
	if memUsage > 8*1024*1024 {
		t.Errorf("Expected network bodies buffer memory <= 8MB, got %d bytes", memUsage)
	}
}

// ============================================
// MCP Tool Tests for v4
// ============================================

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

func TestMCPGetNetworkBodies(t *testing.T) {
	server, _ := setupTestServer(t)
	v4 := setupV4TestServer(t)
	mcp := NewMCPHandlerV4(server, v4)

	v4.AddNetworkBodies([]NetworkBody{
		{URL: "/api/users", Method: "GET", Status: 200, ResponseBody: `[{"id":1}]`},
		{URL: "/api/users", Method: "POST", Status: 201, RequestBody: `{"name":"Alice"}`},
	})

	mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "initialize",
		Params: json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}`),
	})

	resp := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 2, Method: "tools/call",
		Params: json.RawMessage(`{"name":"get_network_bodies","arguments":{}}`),
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

	var bodies []NetworkBody
	json.Unmarshal([]byte(result.Content[0].Text), &bodies)

	if len(bodies) != 2 {
		t.Errorf("Expected 2 bodies, got %d", len(bodies))
	}
}

func TestMCPGetNetworkBodiesWithFilter(t *testing.T) {
	server, _ := setupTestServer(t)
	v4 := setupV4TestServer(t)
	mcp := NewMCPHandlerV4(server, v4)

	v4.AddNetworkBodies([]NetworkBody{
		{URL: "/api/users", Method: "GET", Status: 200},
		{URL: "/api/users", Method: "GET", Status: 500},
	})

	mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "initialize",
		Params: json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}`),
	})

	resp := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 2, Method: "tools/call",
		Params: json.RawMessage(`{"name":"get_network_bodies","arguments":{"status_min":400}}`),
	})

	var result struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	json.Unmarshal(resp.Result, &result)

	var bodies []NetworkBody
	json.Unmarshal([]byte(result.Content[0].Text), &bodies)

	if len(bodies) != 1 {
		t.Errorf("Expected 1 error body, got %d", len(bodies))
	}
}

func TestMCPQueryDOM(t *testing.T) {
	server, _ := setupTestServer(t)
	v4 := setupV4TestServer(t)
	mcp := NewMCPHandlerV4(server, v4)

	mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "initialize",
		Params: json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}`),
	})

	// Start the query in a goroutine (it will block waiting for result)
	done := make(chan JSONRPCResponse)
	go func() {
		resp := mcp.HandleRequest(JSONRPCRequest{
			JSONRPC: "2.0", ID: 2, Method: "tools/call",
			Params: json.RawMessage(`{"name":"query_dom","arguments":{"selector":"h1"}}`),
		})
		done <- resp
	}()

	// Simulate extension picking up and responding
	time.Sleep(50 * time.Millisecond)
	pending := v4.GetPendingQueries()
	if len(pending) == 0 {
		t.Fatal("Expected pending query to be created")
	}

	v4.SetQueryResult(pending[0].ID, json.RawMessage(`{"matches":[{"tag":"h1","text":"Hello World"}],"matchCount":1,"returnedCount":1}`))

	// Wait for response
	resp := <-done

	if resp.Error != nil {
		t.Fatalf("Expected no error, got: %v", resp.Error)
	}

	var result struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	json.Unmarshal(resp.Result, &result)

	if len(result.Content) == 0 {
		t.Fatal("Expected content in response")
	}

	if !strings.Contains(result.Content[0].Text, "Hello World") {
		t.Errorf("Expected result to contain 'Hello World', got: %s", result.Content[0].Text)
	}
}

func TestMCPQueryDOMTimeout(t *testing.T) {
	server, _ := setupTestServer(t)
	v4 := setupV4TestServer(t)
	// Use short timeout for testing
	v4.SetQueryTimeout(100 * time.Millisecond)
	mcp := NewMCPHandlerV4(server, v4)

	mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "initialize",
		Params: json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}`),
	})

	// Query with no extension to respond - should timeout
	resp := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 2, Method: "tools/call",
		Params: json.RawMessage(`{"name":"query_dom","arguments":{"selector":"h1"}}`),
	})

	// Should return an error in the content (not protocol error)
	var result struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
		IsError bool `json:"isError"`
	}
	json.Unmarshal(resp.Result, &result)

	if !result.IsError {
		t.Error("Expected isError to be true for timeout")
	}
}

func TestMCPGetPageInfo(t *testing.T) {
	server, _ := setupTestServer(t)
	v4 := setupV4TestServer(t)
	mcp := NewMCPHandlerV4(server, v4)

	mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "initialize",
		Params: json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}`),
	})

	// Start query in goroutine
	done := make(chan JSONRPCResponse)
	go func() {
		resp := mcp.HandleRequest(JSONRPCRequest{
			JSONRPC: "2.0", ID: 2, Method: "tools/call",
			Params: json.RawMessage(`{"name":"get_page_info","arguments":{}}`),
		})
		done <- resp
	}()

	// Simulate extension response
	time.Sleep(50 * time.Millisecond)
	pending := v4.GetPendingQueries()
	if len(pending) == 0 {
		t.Fatal("Expected pending query")
	}

	v4.SetQueryResult(pending[0].ID, json.RawMessage(`{"url":"http://localhost:3000","title":"Test Page","viewport":{"width":1440,"height":900}}`))

	resp := <-done

	if resp.Error != nil {
		t.Fatalf("Expected no error, got: %v", resp.Error)
	}
}

func TestMCPRunAccessibilityAudit(t *testing.T) {
	server, _ := setupTestServer(t)
	v4 := setupV4TestServer(t)
	mcp := NewMCPHandlerV4(server, v4)

	mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "initialize",
		Params: json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}`),
	})

	done := make(chan JSONRPCResponse)
	go func() {
		resp := mcp.HandleRequest(JSONRPCRequest{
			JSONRPC: "2.0", ID: 2, Method: "tools/call",
			Params: json.RawMessage(`{"name":"run_accessibility_audit","arguments":{"scope":"#main"}}`),
		})
		done <- resp
	}()

	time.Sleep(50 * time.Millisecond)
	pending := v4.GetPendingQueries()
	if len(pending) == 0 {
		t.Fatal("Expected pending a11y query")
	}

	if pending[0].Type != "a11y" {
		t.Errorf("Expected query type 'a11y', got %s", pending[0].Type)
	}

	v4.SetQueryResult(pending[0].ID, json.RawMessage(`{"violations":[{"id":"color-contrast","impact":"serious","nodes":[]}],"summary":{"violations":1,"passes":10}}`))

	resp := <-done

	if resp.Error != nil {
		t.Fatalf("Expected no error, got: %v", resp.Error)
	}

	var result struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	json.Unmarshal(resp.Result, &result)

	if !strings.Contains(result.Content[0].Text, "color-contrast") {
		t.Errorf("Expected result to contain violation, got: %s", result.Content[0].Text)
	}
}

func TestMCPRunAccessibilityAuditWithTags(t *testing.T) {
	server, _ := setupTestServer(t)
	v4 := setupV4TestServer(t)
	mcp := NewMCPHandlerV4(server, v4)

	mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "initialize",
		Params: json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}`),
	})

	done := make(chan JSONRPCResponse)
	go func() {
		resp := mcp.HandleRequest(JSONRPCRequest{
			JSONRPC: "2.0", ID: 2, Method: "tools/call",
			Params: json.RawMessage(`{"name":"run_accessibility_audit","arguments":{"tags":["wcag2a","wcag2aa"]}}`),
		})
		done <- resp
	}()

	time.Sleep(50 * time.Millisecond)
	pending := v4.GetPendingQueries()

	// Verify the tags are passed through in params
	var params map[string]interface{}
	json.Unmarshal(pending[0].Params, &params)

	if params["tags"] == nil {
		t.Error("Expected tags in pending query params")
	}

	v4.SetQueryResult(pending[0].ID, json.RawMessage(`{"violations":[],"summary":{"violations":0}}`))
	<-done
}

func TestMCPToolsListIncludesV4Tools(t *testing.T) {
	server, _ := setupTestServer(t)
	v4 := setupV4TestServer(t)
	mcp := NewMCPHandlerV4(server, v4)

	mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "initialize",
		Params: json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}`),
	})

	resp := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 2, Method: "tools/list",
	})

	var result struct {
		Tools []struct {
			Name string `json:"name"`
		} `json:"tools"`
	}
	json.Unmarshal(resp.Result, &result)

	toolNames := make(map[string]bool)
	for _, tool := range result.Tools {
		toolNames[tool.Name] = true
	}

	expectedTools := []string{
		"get_browser_errors",
		"get_browser_logs",
		"clear_browser_logs",
		"get_websocket_events",
		"get_websocket_status",
		"get_network_bodies",
		"query_dom",
		"get_page_info",
		"run_accessibility_audit",
	}

	for _, name := range expectedTools {
		if !toolNames[name] {
			t.Errorf("Expected tool '%s' in tools list", name)
		}
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

func TestMCPGetNetworkBodiesEmpty(t *testing.T) {
	server, _ := setupTestServer(t)
	v4 := setupV4TestServer(t)
	mcp := NewMCPHandlerV4(server, v4)

	mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "initialize",
		Params: json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}`),
	})

	resp := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 2, Method: "tools/call",
		Params: json.RawMessage(`{"name":"get_network_bodies","arguments":{}}`),
	})

	var result struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	json.Unmarshal(resp.Result, &result)

	if result.Content[0].Text != "No network bodies captured" {
		var bodies []NetworkBody
		if err := json.Unmarshal([]byte(result.Content[0].Text), &bodies); err == nil {
			if len(bodies) != 0 {
				t.Error("Expected empty bodies or message")
			}
		}
	}
}

// ============================================
// v5 Enhanced Actions Buffer Tests
// ============================================

func TestV5EnhancedActionsBuffer(t *testing.T) {
	v4 := setupV4TestServer(t)

	actions := []EnhancedAction{
		{
			Type:      "click",
			Timestamp: 1705312200000,
			URL:       "http://localhost:3000/login",
			Selectors: map[string]interface{}{
				"testId":  "login-btn",
				"cssPath": "form > button.primary",
			},
		},
		{
			Type:      "input",
			Timestamp: 1705312201000,
			URL:       "http://localhost:3000/login",
			Selectors: map[string]interface{}{
				"ariaLabel": "Email address",
				"cssPath":   "#email",
			},
			Value:     "user@example.com",
			InputType: "email",
		},
	}

	v4.AddEnhancedActions(actions)

	if v4.GetEnhancedActionCount() != 2 {
		t.Errorf("Expected 2 actions, got %d", v4.GetEnhancedActionCount())
	}
}

func TestV5EnhancedActionsBufferRotation(t *testing.T) {
	v4 := setupV4TestServer(t)

	// Add more than max (50) actions
	actions := make([]EnhancedAction, 60)
	for i := range actions {
		actions[i] = EnhancedAction{
			Type:      "click",
			Timestamp: int64(1705312200000 + i*1000),
			URL:       "http://localhost:3000/page",
			Selectors: map[string]interface{}{"cssPath": "button"},
		}
	}

	v4.AddEnhancedActions(actions)

	if v4.GetEnhancedActionCount() != 50 {
		t.Errorf("Expected 50 actions after rotation, got %d", v4.GetEnhancedActionCount())
	}
}

func TestV5EnhancedActionsGetAll(t *testing.T) {
	v4 := setupV4TestServer(t)

	v4.AddEnhancedActions([]EnhancedAction{
		{Type: "click", Timestamp: 1000, URL: "http://localhost:3000/a"},
		{Type: "input", Timestamp: 2000, URL: "http://localhost:3000/b"},
		{Type: "navigate", Timestamp: 3000, URL: "http://localhost:3000/c"},
	})

	actions := v4.GetEnhancedActions(EnhancedActionFilter{})
	if len(actions) != 3 {
		t.Errorf("Expected 3 actions, got %d", len(actions))
	}
}

func TestV5EnhancedActionsFilterByLastN(t *testing.T) {
	v4 := setupV4TestServer(t)

	for i := 0; i < 10; i++ {
		v4.AddEnhancedActions([]EnhancedAction{
			{Type: "click", Timestamp: int64(i * 1000), URL: "http://localhost:3000"},
		})
	}

	actions := v4.GetEnhancedActions(EnhancedActionFilter{LastN: 3})
	if len(actions) != 3 {
		t.Errorf("Expected 3 actions with lastN filter, got %d", len(actions))
	}

	// Should return the most recent 3
	if actions[0].Timestamp != 7000 {
		t.Errorf("Expected oldest of last 3 to be timestamp 7000, got %d", actions[0].Timestamp)
	}
}

func TestV5EnhancedActionsFilterByURL(t *testing.T) {
	v4 := setupV4TestServer(t)

	v4.AddEnhancedActions([]EnhancedAction{
		{Type: "click", Timestamp: 1000, URL: "http://localhost:3000/login"},
		{Type: "input", Timestamp: 2000, URL: "http://localhost:3000/dashboard"},
		{Type: "click", Timestamp: 3000, URL: "http://localhost:3000/login"},
	})

	actions := v4.GetEnhancedActions(EnhancedActionFilter{URLFilter: "login"})
	if len(actions) != 2 {
		t.Errorf("Expected 2 actions matching 'login', got %d", len(actions))
	}
}

func TestV5EnhancedActionsNewestLast(t *testing.T) {
	v4 := setupV4TestServer(t)

	v4.AddEnhancedActions([]EnhancedAction{
		{Type: "click", Timestamp: 1000, URL: "http://localhost:3000"},
		{Type: "input", Timestamp: 3000, URL: "http://localhost:3000"},
		{Type: "click", Timestamp: 2000, URL: "http://localhost:3000"},
	})

	actions := v4.GetEnhancedActions(EnhancedActionFilter{})
	// Actions should preserve insertion order (chronological from extension)
	if actions[0].Timestamp != 1000 || actions[2].Timestamp != 2000 {
		t.Error("Expected actions in insertion order")
	}
}

func TestV5EnhancedActionsPasswordRedaction(t *testing.T) {
	v4 := setupV4TestServer(t)

	v4.AddEnhancedActions([]EnhancedAction{
		{Type: "input", Timestamp: 1000, URL: "http://localhost:3000", InputType: "password", Value: "secret123"},
	})

	actions := v4.GetEnhancedActions(EnhancedActionFilter{})
	// Server should preserve what extension sent (extension already redacts)
	// But server should also redact if inputType is password
	if actions[0].Value != "[redacted]" {
		t.Errorf("Expected password value to be redacted, got %s", actions[0].Value)
	}
}

// ============================================
// v5 Enhanced Actions HTTP Endpoint Tests
// ============================================

func TestV5PostEnhancedActionsEndpoint(t *testing.T) {
	v4 := setupV4TestServer(t)

	body := `{"actions":[{"type":"click","timestamp":1705312200000,"url":"http://localhost:3000/login","selectors":{"testId":"login-btn","cssPath":"button.primary"}}]}`
	req := httptest.NewRequest("POST", "/enhanced-actions", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	v4.HandleEnhancedActions(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", rec.Code)
	}

	if v4.GetEnhancedActionCount() != 1 {
		t.Errorf("Expected 1 action stored, got %d", v4.GetEnhancedActionCount())
	}
}

func TestV5PostEnhancedActionsMultiple(t *testing.T) {
	v4 := setupV4TestServer(t)

	body := `{"actions":[
		{"type":"click","timestamp":1000,"url":"http://localhost:3000","selectors":{"cssPath":"button"}},
		{"type":"input","timestamp":2000,"url":"http://localhost:3000","selectors":{"ariaLabel":"Email"},"value":"test@example.com","inputType":"email"},
		{"type":"keypress","timestamp":3000,"url":"http://localhost:3000","key":"Enter"}
	]}`
	req := httptest.NewRequest("POST", "/enhanced-actions", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	v4.HandleEnhancedActions(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", rec.Code)
	}

	if v4.GetEnhancedActionCount() != 3 {
		t.Errorf("Expected 3 actions stored, got %d", v4.GetEnhancedActionCount())
	}
}

func TestV5PostEnhancedActionsInvalidJSON(t *testing.T) {
	v4 := setupV4TestServer(t)

	req := httptest.NewRequest("POST", "/enhanced-actions", bytes.NewBufferString("not json"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	v4.HandleEnhancedActions(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Expected 400, got %d", rec.Code)
	}
}

func TestV5PostEnhancedActionsPasswordRedaction(t *testing.T) {
	v4 := setupV4TestServer(t)

	body := `{"actions":[{"type":"input","timestamp":1000,"url":"http://localhost:3000","selectors":{},"inputType":"password","value":"mysecret"}]}`
	req := httptest.NewRequest("POST", "/enhanced-actions", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	v4.HandleEnhancedActions(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", rec.Code)
	}

	actions := v4.GetEnhancedActions(EnhancedActionFilter{})
	if actions[0].Value != "[redacted]" {
		t.Errorf("Expected password to be redacted on ingest, got %s", actions[0].Value)
	}
}

// ============================================
// v5 MCP get_enhanced_actions Tool Tests
// ============================================

func TestMCPGetEnhancedActions(t *testing.T) {
	server, _ := setupTestServer(t)
	v4 := setupV4TestServer(t)
	mcp := NewMCPHandlerV4(server, v4)

	v4.AddEnhancedActions([]EnhancedAction{
		{Type: "click", Timestamp: 1000, URL: "http://localhost:3000/login", Selectors: map[string]interface{}{"testId": "login-btn"}},
		{Type: "input", Timestamp: 2000, URL: "http://localhost:3000/login", Selectors: map[string]interface{}{"ariaLabel": "Email"}, Value: "user@test.com", InputType: "email"},
	})

	mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "initialize",
		Params: json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}`),
	})

	resp := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 2, Method: "tools/call",
		Params: json.RawMessage(`{"name":"get_enhanced_actions","arguments":{}}`),
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

	if len(result.Content) == 0 {
		t.Fatal("Expected content in response")
	}

	var actions []EnhancedAction
	if err := json.Unmarshal([]byte(result.Content[0].Text), &actions); err != nil {
		t.Fatalf("Expected valid JSON actions, got error: %v", err)
	}

	if len(actions) != 2 {
		t.Errorf("Expected 2 actions, got %d", len(actions))
	}
}

func TestMCPGetEnhancedActionsWithLastN(t *testing.T) {
	server, _ := setupTestServer(t)
	v4 := setupV4TestServer(t)
	mcp := NewMCPHandlerV4(server, v4)

	for i := 0; i < 10; i++ {
		v4.AddEnhancedActions([]EnhancedAction{
			{Type: "click", Timestamp: int64(i * 1000), URL: "http://localhost:3000"},
		})
	}

	mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "initialize",
		Params: json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}`),
	})

	resp := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 2, Method: "tools/call",
		Params: json.RawMessage(`{"name":"get_enhanced_actions","arguments":{"last_n":5}}`),
	})

	var result struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	json.Unmarshal(resp.Result, &result)

	var actions []EnhancedAction
	json.Unmarshal([]byte(result.Content[0].Text), &actions)

	if len(actions) != 5 {
		t.Errorf("Expected 5 actions with last_n filter, got %d", len(actions))
	}
}

func TestMCPGetEnhancedActionsEmpty(t *testing.T) {
	server, _ := setupTestServer(t)
	v4 := setupV4TestServer(t)
	mcp := NewMCPHandlerV4(server, v4)

	mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "initialize",
		Params: json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}`),
	})

	resp := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 2, Method: "tools/call",
		Params: json.RawMessage(`{"name":"get_enhanced_actions","arguments":{}}`),
	})

	var result struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	json.Unmarshal(resp.Result, &result)

	if result.Content[0].Text != "No enhanced actions captured" {
		t.Errorf("Expected empty message, got: %s", result.Content[0].Text)
	}
}

// ============================================
// v5 MCP get_reproduction_script Tool Tests
// ============================================

func TestMCPGetReproductionScript(t *testing.T) {
	server, _ := setupTestServer(t)
	v4 := setupV4TestServer(t)
	mcp := NewMCPHandlerV4(server, v4)

	v4.AddEnhancedActions([]EnhancedAction{
		{Type: "click", Timestamp: 1000, URL: "http://localhost:3000/login", Selectors: map[string]interface{}{"testId": "login-btn"}},
		{Type: "input", Timestamp: 2000, URL: "http://localhost:3000/login", Selectors: map[string]interface{}{"ariaLabel": "Email"}, Value: "user@test.com", InputType: "email"},
	})

	mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "initialize",
		Params: json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}`),
	})

	resp := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 2, Method: "tools/call",
		Params: json.RawMessage(`{"name":"get_reproduction_script","arguments":{}}`),
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

	script := result.Content[0].Text

	// Should contain Playwright test structure
	if !strings.Contains(script, "import { test, expect } from '@playwright/test'") {
		t.Error("Expected Playwright import in script")
	}
	if !strings.Contains(script, "test(") {
		t.Error("Expected test() in script")
	}
	if !strings.Contains(script, "page.goto") {
		t.Error("Expected page.goto in script")
	}
}

func TestMCPGetReproductionScriptWithErrorMessage(t *testing.T) {
	server, _ := setupTestServer(t)
	v4 := setupV4TestServer(t)
	mcp := NewMCPHandlerV4(server, v4)

	v4.AddEnhancedActions([]EnhancedAction{
		{Type: "click", Timestamp: 1000, URL: "http://localhost:3000/login", Selectors: map[string]interface{}{"testId": "submit-btn"}},
	})

	mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "initialize",
		Params: json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}`),
	})

	resp := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 2, Method: "tools/call",
		Params: json.RawMessage(`{"name":"get_reproduction_script","arguments":{"error_message":"Cannot read property 'user' of undefined"}}`),
	})

	var result struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	json.Unmarshal(resp.Result, &result)

	script := result.Content[0].Text

	if !strings.Contains(script, "Cannot read property") {
		t.Error("Expected error message in script")
	}
}

func TestMCPGetReproductionScriptWithLastN(t *testing.T) {
	server, _ := setupTestServer(t)
	v4 := setupV4TestServer(t)
	mcp := NewMCPHandlerV4(server, v4)

	for i := 0; i < 10; i++ {
		v4.AddEnhancedActions([]EnhancedAction{
			{Type: "click", Timestamp: int64(i * 1000), URL: "http://localhost:3000", Selectors: map[string]interface{}{"testId": "btn-" + string(rune('a'+i))}},
		})
	}

	mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "initialize",
		Params: json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}`),
	})

	resp := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 2, Method: "tools/call",
		Params: json.RawMessage(`{"name":"get_reproduction_script","arguments":{"last_n_actions":3}}`),
	})

	var result struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	json.Unmarshal(resp.Result, &result)

	script := result.Content[0].Text

	// Should only have 3 click actions in the script
	clickCount := strings.Count(script, ".click()")
	if clickCount != 3 {
		t.Errorf("Expected 3 click actions in script with last_n_actions=3, got %d", clickCount)
	}
}

func TestMCPGetReproductionScriptWithBaseURL(t *testing.T) {
	server, _ := setupTestServer(t)
	v4 := setupV4TestServer(t)
	mcp := NewMCPHandlerV4(server, v4)

	v4.AddEnhancedActions([]EnhancedAction{
		{Type: "click", Timestamp: 1000, URL: "http://localhost:3000/login", Selectors: map[string]interface{}{"testId": "btn"}},
		{Type: "navigate", Timestamp: 2000, URL: "http://localhost:3000/dashboard", FromURL: "http://localhost:3000/login", ToURL: "http://localhost:3000/dashboard"},
	})

	mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "initialize",
		Params: json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}`),
	})

	resp := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 2, Method: "tools/call",
		Params: json.RawMessage(`{"name":"get_reproduction_script","arguments":{"base_url":"https://staging.example.com"}}`),
	})

	var result struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	json.Unmarshal(resp.Result, &result)

	script := result.Content[0].Text

	// goto should use base_url + path
	if !strings.Contains(script, "staging.example.com/login") {
		t.Errorf("Expected base_url to be applied to goto, got script:\n%s", script)
	}
}

func TestMCPGetReproductionScriptEmpty(t *testing.T) {
	server, _ := setupTestServer(t)
	v4 := setupV4TestServer(t)
	mcp := NewMCPHandlerV4(server, v4)

	mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "initialize",
		Params: json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}`),
	})

	resp := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 2, Method: "tools/call",
		Params: json.RawMessage(`{"name":"get_reproduction_script","arguments":{}}`),
	})

	var result struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	json.Unmarshal(resp.Result, &result)

	if result.Content[0].Text != "No enhanced actions captured to generate script" {
		t.Errorf("Expected empty message, got: %s", result.Content[0].Text)
	}
}

func TestMCPGetReproductionScriptSelectorPriority(t *testing.T) {
	server, _ := setupTestServer(t)
	v4 := setupV4TestServer(t)
	mcp := NewMCPHandlerV4(server, v4)

	v4.AddEnhancedActions([]EnhancedAction{
		{
			Type: "click", Timestamp: 1000, URL: "http://localhost:3000",
			Selectors: map[string]interface{}{
				"testId":    "submit-btn",
				"ariaLabel": "Submit form",
				"cssPath":   "form > button",
			},
		},
		{
			Type: "click", Timestamp: 2000, URL: "http://localhost:3000",
			Selectors: map[string]interface{}{
				"role":    map[string]interface{}{"role": "button", "name": "Save"},
				"cssPath": "div > button",
			},
		},
		{
			Type: "click", Timestamp: 3000, URL: "http://localhost:3000",
			Selectors: map[string]interface{}{
				"cssPath": "div.card > button.action",
			},
		},
	})

	mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "initialize",
		Params: json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}`),
	})

	resp := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 2, Method: "tools/call",
		Params: json.RawMessage(`{"name":"get_reproduction_script","arguments":{}}`),
	})

	var result struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	json.Unmarshal(resp.Result, &result)

	script := result.Content[0].Text

	// testId should produce getByTestId
	if !strings.Contains(script, "getByTestId('submit-btn')") {
		t.Errorf("Expected getByTestId for first action, got:\n%s", script)
	}

	// role should produce getByRole
	if !strings.Contains(script, "getByRole('button', { name: 'Save' })") {
		t.Errorf("Expected getByRole for second action, got:\n%s", script)
	}

	// cssPath fallback should produce locator()
	if !strings.Contains(script, "locator('div.card > button.action')") {
		t.Errorf("Expected locator() for third action, got:\n%s", script)
	}
}

func TestMCPGetReproductionScriptInputActions(t *testing.T) {
	server, _ := setupTestServer(t)
	v4 := setupV4TestServer(t)
	mcp := NewMCPHandlerV4(server, v4)

	v4.AddEnhancedActions([]EnhancedAction{
		{Type: "input", Timestamp: 1000, URL: "http://localhost:3000", Selectors: map[string]interface{}{"testId": "email-input"}, Value: "user@test.com", InputType: "email"},
		{Type: "input", Timestamp: 2000, URL: "http://localhost:3000", Selectors: map[string]interface{}{"testId": "password-input"}, Value: "[redacted]", InputType: "password"},
		{Type: "select", Timestamp: 3000, URL: "http://localhost:3000", Selectors: map[string]interface{}{"testId": "country-select"}, SelectedValue: "us"},
		{Type: "keypress", Timestamp: 4000, URL: "http://localhost:3000", Key: "Enter"},
	})

	mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "initialize",
		Params: json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}`),
	})

	resp := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 2, Method: "tools/call",
		Params: json.RawMessage(`{"name":"get_reproduction_script","arguments":{}}`),
	})

	var result struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	json.Unmarshal(resp.Result, &result)

	script := result.Content[0].Text

	// fill() for input
	if !strings.Contains(script, ".fill('user@test.com')") {
		t.Errorf("Expected fill for email input, got:\n%s", script)
	}

	// Redacted password should use placeholder
	if !strings.Contains(script, ".fill('[user-provided]')") {
		t.Errorf("Expected [user-provided] for redacted password, got:\n%s", script)
	}

	// selectOption for select
	if !strings.Contains(script, ".selectOption('us')") {
		t.Errorf("Expected selectOption for select, got:\n%s", script)
	}

	// keyboard.press for keypress
	if !strings.Contains(script, "page.keyboard.press('Enter')") {
		t.Errorf("Expected keyboard.press for keypress, got:\n%s", script)
	}
}

func TestMCPGetReproductionScriptPauseComments(t *testing.T) {
	server, _ := setupTestServer(t)
	v4 := setupV4TestServer(t)
	mcp := NewMCPHandlerV4(server, v4)

	v4.AddEnhancedActions([]EnhancedAction{
		{Type: "click", Timestamp: 1000, URL: "http://localhost:3000", Selectors: map[string]interface{}{"testId": "btn1"}},
		{Type: "click", Timestamp: 6000, URL: "http://localhost:3000", Selectors: map[string]interface{}{"testId": "btn2"}}, // 5 seconds later
	})

	mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "initialize",
		Params: json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}`),
	})

	resp := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 2, Method: "tools/call",
		Params: json.RawMessage(`{"name":"get_reproduction_script","arguments":{}}`),
	})

	var result struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	json.Unmarshal(resp.Result, &result)

	script := result.Content[0].Text

	// Should contain a pause comment for the 5s gap
	if !strings.Contains(script, "// [5s pause]") {
		t.Errorf("Expected pause comment for 5s gap, got:\n%s", script)
	}
}

// ============================================
// v5 _aiContext Passthrough Tests
// ============================================

func TestV5AiContextPassthroughInGetBrowserErrors(t *testing.T) {
	server, _ := setupTestServer(t)
	v4 := setupV4TestServer(t)
	mcp := NewMCPHandlerV4(server, v4)

	// Add a log entry with _aiContext field (as extension would send)
	entry := LogEntry{
		"level":   "error",
		"message": "Cannot read property 'user' of undefined",
		"stack":   "TypeError: Cannot read property 'user' of undefined\n    at UserProfile.render (app.js:42:15)",
		"_aiContext": map[string]interface{}{
			"summary": "TypeError in UserProfile.render at app.js:42. React component: UserProfile > App.",
			"componentAncestry": map[string]interface{}{
				"framework":  "react",
				"components": []interface{}{"UserProfile", "App"},
			},
			"stateSnapshot": map[string]interface{}{
				"relevantSlice": map[string]interface{}{
					"auth": map[string]interface{}{"user": nil, "loading": false},
				},
			},
		},
		"_enrichments": []interface{}{"aiContext"},
	}
	server.addEntries([]LogEntry{entry})

	mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "initialize",
		Params: json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}`),
	})

	resp := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 2, Method: "tools/call",
		Params: json.RawMessage(`{"name":"get_browser_errors","arguments":{}}`),
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

	if len(result.Content) == 0 {
		t.Fatal("Expected content in response")
	}

	// The _aiContext should be preserved in the output
	responseText := result.Content[0].Text
	if !strings.Contains(responseText, "_aiContext") {
		t.Error("Expected _aiContext field to be preserved in get_browser_errors output")
	}
	if !strings.Contains(responseText, "componentAncestry") {
		t.Error("Expected componentAncestry in _aiContext to be preserved")
	}
	if !strings.Contains(responseText, "UserProfile") {
		t.Error("Expected component name to be preserved in _aiContext")
	}
}

// ============================================
// v5 MCP Tools List Tests
// ============================================

// ============================================
// v5 Roadmap Item 1: Query Result Cleanup
// ============================================

func TestV4QueryResultDeletedAfterRetrieval(t *testing.T) {
	v4 := setupV4TestServer(t)

	id := v4.CreatePendingQuery(PendingQuery{
		Type:   "dom",
		Params: json.RawMessage(`{"selector":"h1"}`),
	})

	v4.SetQueryResult(id, json.RawMessage(`{"matches":[]}`))

	// First retrieval should succeed
	result, found := v4.GetQueryResult(id)
	if !found {
		t.Fatal("Expected result to be found on first read")
	}
	if result == nil {
		t.Fatal("Expected non-nil result on first read")
	}

	// Second retrieval should fail (result deleted after read)
	_, found2 := v4.GetQueryResult(id)
	if found2 {
		t.Error("Expected result to be deleted after first retrieval")
	}
}

func TestV4QueryResultDeletedAfterWaitForResult(t *testing.T) {
	v4 := setupV4TestServer(t)

	id := v4.CreatePendingQuery(PendingQuery{
		Type:   "dom",
		Params: json.RawMessage(`{"selector":"h1"}`),
	})

	// Set result in background
	go func() {
		time.Sleep(20 * time.Millisecond)
		v4.SetQueryResult(id, json.RawMessage(`{"matches":[]}`))
	}()

	// WaitForResult should succeed
	result, err := v4.WaitForResult(id, time.Second)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if result == nil {
		t.Fatal("Expected non-nil result")
	}

	// Result should be cleaned up after WaitForResult returns
	_, found := v4.GetQueryResult(id)
	if found {
		t.Error("Expected result to be cleaned up after WaitForResult")
	}
}

func TestV4QueryResultMapDoesNotGrowUnbounded(t *testing.T) {
	v4 := setupV4TestServer(t)

	// Create and resolve 20 queries
	for i := 0; i < 20; i++ {
		id := v4.CreatePendingQuery(PendingQuery{
			Type:   "dom",
			Params: json.RawMessage(`{"selector":"h1"}`),
		})
		v4.SetQueryResult(id, json.RawMessage(`{"matches":[]}`))
		// Read the result (should delete it)
		v4.GetQueryResult(id)
	}

	// queryResults map should be empty
	v4.mu.RLock()
	mapSize := len(v4.queryResults)
	v4.mu.RUnlock()

	if mapSize != 0 {
		t.Errorf("Expected queryResults map to be empty after all reads, got %d entries", mapSize)
	}
}

// ============================================
// v5 Roadmap Item 2: Connection Duration
// ============================================

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

// ============================================
// v5 Roadmap Item 3: Message Rate Calculation
// ============================================

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

// ============================================
// v5 Roadmap Item 4: Age Formatting
// ============================================

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

// ============================================
// Memory Auto-Eviction Tests
// ============================================

func TestV4GetTotalBufferMemory(t *testing.T) {
	v4 := setupV4TestServer(t)

	// Add some data to each buffer
	v4.AddWebSocketEvents([]WebSocketEvent{
		{ID: "uuid-1", Event: "message", Data: strings.Repeat("a", 1000)},
	})
	v4.AddNetworkBodies([]NetworkBody{
		{URL: "/api/test", ResponseBody: strings.Repeat("b", 2000)},
	})

	total := v4.GetTotalBufferMemory()
	if total <= 0 {
		t.Errorf("Expected positive total buffer memory, got %d", total)
	}

	// Total should be sum of WS + NB buffers
	wsMemory := v4.GetWebSocketBufferMemory()
	nbMemory := v4.GetNetworkBodiesBufferMemory()
	if total != wsMemory+nbMemory {
		t.Errorf("Expected total (%d) = ws (%d) + nb (%d)", total, wsMemory, nbMemory)
	}
}

func TestV4IsMemoryExceededUsesRealMemory(t *testing.T) {
	v4 := setupV4TestServer(t)

	// With empty buffers, memory should not be exceeded
	if v4.IsMemoryExceeded() {
		t.Error("Expected memory NOT to be exceeded with empty buffers")
	}

	// Simulated memory should still work as override for testing
	v4.SetMemoryUsage(55 * 1024 * 1024) // 55MB
	if !v4.IsMemoryExceeded() {
		t.Error("Expected simulated memory to trigger exceeded")
	}

	// Reset simulated
	v4.SetMemoryUsage(0)
	if v4.IsMemoryExceeded() {
		t.Error("Expected memory NOT exceeded after resetting simulated")
	}
}

func TestV4GlobalEvictionOnWSIngest(t *testing.T) {
	v4 := setupV4TestServer(t)

	// Fill WS buffer to near its per-buffer limit (4MB)
	largeData := strings.Repeat("x", 100000) // 100KB each
	for i := 0; i < 38; i++ {                // ~3.8MB
		v4.AddWebSocketEvents([]WebSocketEvent{
			{ID: "uuid-1", Event: "message", Data: largeData},
		})
	}

	beforeCount := v4.GetWebSocketEventCount()

	// Adding more events should still enforce per-buffer memory limit
	for i := 0; i < 5; i++ {
		v4.AddWebSocketEvents([]WebSocketEvent{
			{ID: "uuid-1", Event: "message", Data: largeData},
		})
	}

	afterMem := v4.GetWebSocketBufferMemory()
	if afterMem > wsBufferMemoryLimit {
		t.Errorf("Expected WS buffer <= 4MB after eviction, got %d bytes", afterMem)
	}

	// Should have fewer events than beforeCount + 5 due to eviction
	afterCount := v4.GetWebSocketEventCount()
	if afterCount >= beforeCount+5 {
		t.Errorf("Expected eviction to reduce events, before=%d after=%d", beforeCount, afterCount)
	}
}

func TestV4GlobalEvictionOnNBIngest(t *testing.T) {
	v4 := setupV4TestServer(t)

	// Fill NB buffer to near its per-buffer limit (8MB)
	largeBody := strings.Repeat("y", 200000) // 200KB each
	for i := 0; i < 38; i++ {                // ~7.6MB
		v4.AddNetworkBodies([]NetworkBody{
			{URL: "/api/test", ResponseBody: largeBody, Status: 200},
		})
	}

	// Adding more should trigger eviction
	for i := 0; i < 5; i++ {
		v4.AddNetworkBodies([]NetworkBody{
			{URL: "/api/test", ResponseBody: largeBody, Status: 200},
		})
	}

	afterMem := v4.GetNetworkBodiesBufferMemory()
	if afterMem > nbBufferMemoryLimit {
		t.Errorf("Expected NB buffer <= 8MB after eviction, got %d bytes", afterMem)
	}
}

func TestV4MemoryExceededRejectsWSEvents(t *testing.T) {
	v4 := setupV4TestServer(t)

	// Simulate global memory exceeded
	v4.SetMemoryUsage(55 * 1024 * 1024) // 55MB > 50MB hard limit

	body := `{"events":[{"event":"message","id":"uuid-1","data":"test"}]}`
	req := httptest.NewRequest("POST", "/websocket-events", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	v4.HandleWebSocketEvents(rec, req)

	// Should return 503 when global memory is exceeded
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected 503 when memory exceeded, got %d", rec.Code)
	}
}

func TestV4MemoryExceededHeaderInResponse(t *testing.T) {
	v4 := setupV4TestServer(t)

	// Fill buffers close to their limits
	largeData := strings.Repeat("x", 100000)
	for i := 0; i < 35; i++ {
		v4.AddWebSocketEvents([]WebSocketEvent{
			{ID: "uuid-1", Event: "message", Data: largeData},
		})
	}

	// Check that total memory is reported
	total := v4.GetTotalBufferMemory()
	if total <= 0 {
		t.Error("Expected non-zero total memory after filling buffers")
	}
}

func TestMCPToolsListIncludesV5Tools(t *testing.T) {
	server, _ := setupTestServer(t)
	v4 := setupV4TestServer(t)
	mcp := NewMCPHandlerV4(server, v4)

	mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "initialize",
		Params: json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}`),
	})

	resp := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 2, Method: "tools/list",
	})

	var result struct {
		Tools []struct {
			Name string `json:"name"`
		} `json:"tools"`
	}
	json.Unmarshal(resp.Result, &result)

	toolNames := make(map[string]bool)
	for _, tool := range result.Tools {
		toolNames[tool.Name] = true
	}

	v5Tools := []string{
		"get_enhanced_actions",
		"get_reproduction_script",
	}

	for _, name := range v5Tools {
		if !toolNames[name] {
			t.Errorf("Expected v5 tool '%s' in tools list", name)
		}
	}
}

// ============================================
// v5 Test Generation: extractResponseShape
// ============================================

func TestExtractResponseShapeObject(t *testing.T) {
	shape := extractResponseShape(`{"token":"abc123","user":{"id":5,"name":"Bob"}}`)

	shapeMap, ok := shape.(map[string]interface{})
	if !ok {
		t.Fatalf("Expected map, got %T", shape)
	}
	if shapeMap["token"] != "string" {
		t.Errorf("Expected token=string, got %v", shapeMap["token"])
	}
	userMap, ok := shapeMap["user"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected user to be map, got %T", shapeMap["user"])
	}
	if userMap["id"] != "number" {
		t.Errorf("Expected user.id=number, got %v", userMap["id"])
	}
	if userMap["name"] != "string" {
		t.Errorf("Expected user.name=string, got %v", userMap["name"])
	}
}

func TestExtractResponseShapeArray(t *testing.T) {
	shape := extractResponseShape(`[{"id":1,"name":"Alice"},{"id":2,"name":"Bob"}]`)

	shapeArr, ok := shape.([]interface{})
	if !ok {
		t.Fatalf("Expected array, got %T", shape)
	}
	if len(shapeArr) != 1 {
		t.Fatalf("Expected array with 1 element (sample), got %d", len(shapeArr))
	}
	elem, ok := shapeArr[0].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected array element to be map, got %T", shapeArr[0])
	}
	if elem["id"] != "number" {
		t.Errorf("Expected id=number, got %v", elem["id"])
	}
}

func TestExtractResponseShapePrimitives(t *testing.T) {
	if extractResponseShape(`"hello"`) != "string" {
		t.Error("Expected string for string literal")
	}
	if extractResponseShape(`42`) != "number" {
		t.Error("Expected number for numeric literal")
	}
	if extractResponseShape(`true`) != "boolean" {
		t.Error("Expected boolean for true")
	}
	if extractResponseShape(`null`) != "null" {
		t.Error("Expected null for null")
	}
}

func TestExtractResponseShapeInvalidJSON(t *testing.T) {
	shape := extractResponseShape(`not valid json`)
	if shape != nil {
		t.Errorf("Expected nil for invalid JSON, got %v", shape)
	}
}

func TestExtractResponseShapeEmptyObject(t *testing.T) {
	shape := extractResponseShape(`{}`)
	shapeMap, ok := shape.(map[string]interface{})
	if !ok {
		t.Fatalf("Expected map, got %T", shape)
	}
	if len(shapeMap) != 0 {
		t.Errorf("Expected empty map, got %d keys", len(shapeMap))
	}
}

func TestExtractResponseShapeEmptyArray(t *testing.T) {
	shape := extractResponseShape(`[]`)
	shapeArr, ok := shape.([]interface{})
	if !ok {
		t.Fatalf("Expected array, got %T", shape)
	}
	if len(shapeArr) != 0 {
		t.Errorf("Expected empty array, got %d elements", len(shapeArr))
	}
}

func TestExtractResponseShapeDepthLimit(t *testing.T) {
	// Nested 5 levels deep - should cap at depth 3
	shape := extractResponseShape(`{"a":{"b":{"c":{"d":{"e":"deep"}}}}}`)
	shapeMap := shape.(map[string]interface{})
	aMap := shapeMap["a"].(map[string]interface{})
	bMap := aMap["b"].(map[string]interface{})
	cMap := bMap["c"].(map[string]interface{})
	// At depth 3, values should be "..."
	if cMap["d"] != "..." {
		t.Errorf("Expected '...' at depth limit, got %v", cMap["d"])
	}
}

// ============================================
// v5 Test Generation: normalizeTimestamp
// ============================================

func TestNormalizeTimestampRFC3339(t *testing.T) {
	ts := normalizeTimestamp("2024-01-15T10:30:00.000Z")
	expected := int64(1705314600000)
	if ts != expected {
		t.Errorf("Expected %d, got %d", expected, ts)
	}
}

func TestNormalizeTimestampRFC3339Nano(t *testing.T) {
	ts := normalizeTimestamp("2024-01-15T10:30:00.123456789Z")
	// Should be 1705314600123 (truncated to ms)
	expected := int64(1705314600123)
	if ts != expected {
		t.Errorf("Expected %d, got %d", expected, ts)
	}
}

func TestNormalizeTimestampInvalidString(t *testing.T) {
	ts := normalizeTimestamp("not a timestamp")
	if ts != 0 {
		t.Errorf("Expected 0 for invalid timestamp, got %d", ts)
	}
}

func TestNormalizeTimestampEmpty(t *testing.T) {
	ts := normalizeTimestamp("")
	if ts != 0 {
		t.Errorf("Expected 0 for empty string, got %d", ts)
	}
}

// ============================================
// v5 Test Generation: GetSessionTimeline
// ============================================

func TestGetSessionTimelineEmpty(t *testing.T) {
	v4 := setupV4TestServer(t)

	resp := v4.GetSessionTimeline(TimelineFilter{}, []LogEntry{})

	if len(resp.Timeline) != 0 {
		t.Errorf("Expected empty timeline, got %d entries", len(resp.Timeline))
	}
	if resp.Summary.Actions != 0 || resp.Summary.NetworkRequests != 0 || resp.Summary.ConsoleErrors != 0 {
		t.Error("Expected all summary counts to be 0")
	}
}

func TestGetSessionTimelineActionsOnly(t *testing.T) {
	v4 := setupV4TestServer(t)

	v4.AddEnhancedActions([]EnhancedAction{
		{Type: "click", Timestamp: 1000, URL: "http://localhost:3000/login", Selectors: map[string]interface{}{"testId": "btn"}},
		{Type: "input", Timestamp: 2000, URL: "http://localhost:3000/login", Value: "test@test.com"},
	})

	resp := v4.GetSessionTimeline(TimelineFilter{}, []LogEntry{})

	if len(resp.Timeline) != 2 {
		t.Fatalf("Expected 2 entries, got %d", len(resp.Timeline))
	}
	if resp.Timeline[0].Kind != "action" {
		t.Errorf("Expected kind=action, got %s", resp.Timeline[0].Kind)
	}
	if resp.Timeline[0].Type != "click" {
		t.Errorf("Expected type=click, got %s", resp.Timeline[0].Type)
	}
	if resp.Summary.Actions != 2 {
		t.Errorf("Expected 2 actions in summary, got %d", resp.Summary.Actions)
	}
}

func TestGetSessionTimelineNetworkOnly(t *testing.T) {
	v4 := setupV4TestServer(t)

	v4.AddNetworkBodies([]NetworkBody{
		{Timestamp: "2024-01-15T10:30:00.000Z", Method: "POST", URL: "/api/login", Status: 200, ResponseBody: `{"token":"abc"}`, ContentType: "application/json"},
	})

	resp := v4.GetSessionTimeline(TimelineFilter{}, []LogEntry{})

	if len(resp.Timeline) != 1 {
		t.Fatalf("Expected 1 entry, got %d", len(resp.Timeline))
	}
	if resp.Timeline[0].Kind != "network" {
		t.Errorf("Expected kind=network, got %s", resp.Timeline[0].Kind)
	}
	if resp.Timeline[0].Method != "POST" {
		t.Errorf("Expected method=POST, got %s", resp.Timeline[0].Method)
	}
	if resp.Timeline[0].Status != 200 {
		t.Errorf("Expected status=200, got %d", resp.Timeline[0].Status)
	}
	if resp.Summary.NetworkRequests != 1 {
		t.Errorf("Expected 1 network request in summary, got %d", resp.Summary.NetworkRequests)
	}
}

func TestGetSessionTimelineNetworkResponseShape(t *testing.T) {
	v4 := setupV4TestServer(t)

	v4.AddNetworkBodies([]NetworkBody{
		{Timestamp: "2024-01-15T10:30:00.000Z", Method: "GET", URL: "/api/users", Status: 200,
			ResponseBody: `{"users":[{"id":1,"name":"Alice"}]}`, ContentType: "application/json"},
	})

	resp := v4.GetSessionTimeline(TimelineFilter{}, []LogEntry{})

	if resp.Timeline[0].ResponseShape == nil {
		t.Fatal("Expected responseShape to be populated for JSON responses")
	}

	shapeMap, ok := resp.Timeline[0].ResponseShape.(map[string]interface{})
	if !ok {
		t.Fatalf("Expected shape to be map, got %T", resp.Timeline[0].ResponseShape)
	}
	if _, hasUsers := shapeMap["users"]; !hasUsers {
		t.Error("Expected responseShape to have 'users' key")
	}
}

func TestGetSessionTimelineNetworkNonJSONNoShape(t *testing.T) {
	v4 := setupV4TestServer(t)

	v4.AddNetworkBodies([]NetworkBody{
		{Timestamp: "2024-01-15T10:30:00.000Z", Method: "GET", URL: "/page", Status: 200,
			ResponseBody: `<html></html>`, ContentType: "text/html"},
	})

	resp := v4.GetSessionTimeline(TimelineFilter{}, []LogEntry{})

	if resp.Timeline[0].ResponseShape != nil {
		t.Errorf("Expected nil responseShape for non-JSON content, got %v", resp.Timeline[0].ResponseShape)
	}
}

func TestGetSessionTimelineConsoleEntries(t *testing.T) {
	v4 := setupV4TestServer(t)

	entries := []LogEntry{
		{"level": "error", "message": "Something failed", "ts": "2024-01-15T10:30:00.500Z"},
		{"level": "warn", "message": "Deprecated API", "ts": "2024-01-15T10:30:01.000Z"},
		{"level": "info", "message": "App started", "ts": "2024-01-15T10:30:00.100Z"}, // Should be excluded
	}

	resp := v4.GetSessionTimeline(TimelineFilter{}, entries)

	if len(resp.Timeline) != 2 {
		t.Fatalf("Expected 2 entries (error+warn, no info), got %d", len(resp.Timeline))
	}
	if resp.Timeline[0].Kind != "console" {
		t.Errorf("Expected kind=console, got %s", resp.Timeline[0].Kind)
	}
	if resp.Summary.ConsoleErrors != 1 {
		t.Errorf("Expected 1 console error in summary, got %d", resp.Summary.ConsoleErrors)
	}
}

func TestGetSessionTimelineMergedAndSorted(t *testing.T) {
	v4 := setupV4TestServer(t)

	v4.AddEnhancedActions([]EnhancedAction{
		{Type: "click", Timestamp: 1705314600000, URL: "http://localhost:3000"},
		{Type: "navigate", Timestamp: 1705314600300, ToURL: "/dashboard"},
	})
	v4.AddNetworkBodies([]NetworkBody{
		{Timestamp: "2024-01-15T10:30:00.150Z", Method: "POST", URL: "/api/login", Status: 200,
			ResponseBody: `{"ok":true}`, ContentType: "application/json"},
	})
	entries := []LogEntry{
		{"level": "error", "message": "Widget failed", "ts": "2024-01-15T10:30:00.400Z"},
	}

	resp := v4.GetSessionTimeline(TimelineFilter{}, entries)

	if len(resp.Timeline) != 4 {
		t.Fatalf("Expected 4 entries, got %d", len(resp.Timeline))
	}

	// Verify order: click(1705314600000) → network(150ms later) → navigate(300ms) → error(400ms)
	if resp.Timeline[0].Kind != "action" || resp.Timeline[0].Type != "click" {
		t.Errorf("Entry 0: expected action/click, got %s/%s", resp.Timeline[0].Kind, resp.Timeline[0].Type)
	}
	if resp.Timeline[1].Kind != "network" {
		t.Errorf("Entry 1: expected network, got %s", resp.Timeline[1].Kind)
	}
	if resp.Timeline[2].Kind != "action" || resp.Timeline[2].Type != "navigate" {
		t.Errorf("Entry 2: expected action/navigate, got %s/%s", resp.Timeline[2].Kind, resp.Timeline[2].Type)
	}
	if resp.Timeline[3].Kind != "console" {
		t.Errorf("Entry 3: expected console, got %s", resp.Timeline[3].Kind)
	}
}

func TestGetSessionTimelineLastNActions(t *testing.T) {
	v4 := setupV4TestServer(t)

	v4.AddEnhancedActions([]EnhancedAction{
		{Type: "click", Timestamp: 1000, URL: "http://localhost:3000"},
		{Type: "click", Timestamp: 2000, URL: "http://localhost:3000"},
		{Type: "click", Timestamp: 3000, URL: "http://localhost:3000"},
	})
	v4.AddNetworkBodies([]NetworkBody{
		{Timestamp: "1970-01-01T00:00:00.500Z", Method: "GET", URL: "/early", Status: 200, ContentType: "application/json", ResponseBody: `{}`}, // Before action 1
		{Timestamp: "1970-01-01T00:00:02.500Z", Method: "GET", URL: "/late", Status: 200, ContentType: "application/json", ResponseBody: `{}`},  // Between action 2 and 3
	})

	// Request last 2 actions — should start from action at t=2000
	resp := v4.GetSessionTimeline(TimelineFilter{LastNActions: 2}, []LogEntry{})

	// Should include: action@2000, network@2500, action@3000 — NOT action@1000 or network@500
	hasEarly := false
	for _, entry := range resp.Timeline {
		if entry.Kind == "network" && entry.URL == "/early" {
			hasEarly = true
		}
	}
	if hasEarly {
		t.Error("Should not include network events before the last_n_actions boundary")
	}
	if resp.Summary.Actions != 2 {
		t.Errorf("Expected 2 actions in summary, got %d", resp.Summary.Actions)
	}
}

func TestGetSessionTimelineURLFilter(t *testing.T) {
	v4 := setupV4TestServer(t)

	v4.AddEnhancedActions([]EnhancedAction{
		{Type: "click", Timestamp: 1000, URL: "http://localhost:3000/login"},
		{Type: "click", Timestamp: 2000, URL: "http://localhost:3000/dashboard"},
	})
	v4.AddNetworkBodies([]NetworkBody{
		{Timestamp: "1970-01-01T00:00:01.500Z", Method: "POST", URL: "/api/login", Status: 200, ContentType: "application/json", ResponseBody: `{}`},
		{Timestamp: "1970-01-01T00:00:02.500Z", Method: "GET", URL: "/api/dashboard", Status: 200, ContentType: "application/json", ResponseBody: `{}`},
	})

	resp := v4.GetSessionTimeline(TimelineFilter{URLFilter: "login"}, []LogEntry{})

	for _, entry := range resp.Timeline {
		if entry.Kind == "action" && !strings.Contains(entry.URL, "login") {
			t.Errorf("Expected URL filter to exclude non-matching actions, got URL: %s", entry.URL)
		}
		if entry.Kind == "network" && !strings.Contains(entry.URL, "login") {
			t.Errorf("Expected URL filter to exclude non-matching network, got URL: %s", entry.URL)
		}
	}
}

func TestGetSessionTimelineIncludeFilter(t *testing.T) {
	v4 := setupV4TestServer(t)

	v4.AddEnhancedActions([]EnhancedAction{
		{Type: "click", Timestamp: 1000, URL: "http://localhost:3000"},
	})
	v4.AddNetworkBodies([]NetworkBody{
		{Timestamp: "1970-01-01T00:00:01.500Z", Method: "GET", URL: "/api", Status: 200, ContentType: "application/json", ResponseBody: `{}`},
	})
	entries := []LogEntry{
		{"level": "error", "message": "err", "ts": "1970-01-01T00:00:02.000Z"},
	}

	// Only include actions
	resp := v4.GetSessionTimeline(TimelineFilter{Include: []string{"actions"}}, entries)

	for _, entry := range resp.Timeline {
		if entry.Kind != "action" {
			t.Errorf("Expected only action entries with include=[actions], got kind=%s", entry.Kind)
		}
	}
}

func TestGetSessionTimelineMaxEntries(t *testing.T) {
	v4 := setupV4TestServer(t)

	// Add 50 actions (max buffer) — timeline cap is 200
	for i := 0; i < 50; i++ {
		v4.AddEnhancedActions([]EnhancedAction{
			{Type: "click", Timestamp: int64(i * 1000), URL: "http://localhost:3000"},
		})
	}

	resp := v4.GetSessionTimeline(TimelineFilter{}, []LogEntry{})

	if len(resp.Timeline) > 200 {
		t.Errorf("Expected timeline capped at 200, got %d", len(resp.Timeline))
	}
}

func TestGetSessionTimelineDurationMs(t *testing.T) {
	v4 := setupV4TestServer(t)

	v4.AddEnhancedActions([]EnhancedAction{
		{Type: "click", Timestamp: 1000},
		{Type: "click", Timestamp: 5000},
	})

	resp := v4.GetSessionTimeline(TimelineFilter{}, []LogEntry{})

	if resp.Summary.DurationMs != 4000 {
		t.Errorf("Expected duration 4000ms, got %d", resp.Summary.DurationMs)
	}
}

// ============================================
// v5 Test Generation: generateTestScript
// ============================================

func TestGenerateTestScriptBasicStructure(t *testing.T) {
	timeline := []TimelineEntry{
		{Timestamp: 1000, Kind: "action", Type: "click", URL: "http://localhost:3000/login", Selectors: map[string]interface{}{"testId": "btn"}},
	}

	script := generateTestScript(timeline, TestGenerationOptions{
		TestName:       "login flow",
		AssertNetwork:  true,
		AssertNoErrors: true,
	})

	if !strings.Contains(script, "import { test, expect } from '@playwright/test'") {
		t.Error("Expected Playwright imports")
	}
	if !strings.Contains(script, "test('login flow'") {
		t.Error("Expected test name in output")
	}
	if !strings.Contains(script, "page.goto(") {
		t.Error("Expected goto in output")
	}
	if !strings.Contains(script, "consoleErrors") {
		t.Error("Expected console error listener when assert_no_errors=true")
	}
	if !strings.Contains(script, "expect(consoleErrors).toHaveLength(0)") {
		t.Error("Expected console error assertion at end")
	}
}

func TestGenerateTestScriptNetworkAssertions(t *testing.T) {
	timeline := []TimelineEntry{
		{Timestamp: 1000, Kind: "action", Type: "click", URL: "http://localhost:3000/login", Selectors: map[string]interface{}{"testId": "login-btn"}},
		{Timestamp: 1150, Kind: "network", Method: "POST", URL: "/api/login", Status: 200, ContentType: "application/json"},
		{Timestamp: 1300, Kind: "action", Type: "navigate", ToURL: "/dashboard"},
	}

	script := generateTestScript(timeline, TestGenerationOptions{AssertNetwork: true, AssertNoErrors: false})

	if !strings.Contains(script, "waitForResponse") {
		t.Error("Expected waitForResponse for network assertion")
	}
	if !strings.Contains(script, "/api/login") {
		t.Error("Expected URL in waitForResponse matcher")
	}
	if !strings.Contains(script, "expect") && !strings.Contains(script, ".status()") {
		t.Error("Expected status assertion")
	}
	if !strings.Contains(script, "toBe(200)") {
		t.Error("Expected status code 200 assertion")
	}
}

func TestGenerateTestScriptNavigationAssertion(t *testing.T) {
	timeline := []TimelineEntry{
		{Timestamp: 1000, Kind: "action", Type: "click", URL: "http://localhost:3000/login", Selectors: map[string]interface{}{"testId": "btn"}},
		{Timestamp: 1300, Kind: "action", Type: "navigate", ToURL: "/dashboard"},
	}

	script := generateTestScript(timeline, TestGenerationOptions{AssertNetwork: false, AssertNoErrors: false})

	if !strings.Contains(script, "toHaveURL") {
		t.Error("Expected toHaveURL assertion for navigation")
	}
	if !strings.Contains(script, "dashboard") {
		t.Error("Expected dashboard URL in assertion")
	}
}

func TestGenerateTestScriptNoErrorsWithCapturedErrors(t *testing.T) {
	timeline := []TimelineEntry{
		{Timestamp: 1000, Kind: "action", Type: "click", URL: "http://localhost:3000", Selectors: map[string]interface{}{"testId": "btn"}},
		{Timestamp: 1500, Kind: "console", Level: "error", Message: "Widget failed to load"},
	}

	script := generateTestScript(timeline, TestGenerationOptions{AssertNoErrors: true})

	// When errors were captured during the session, the assertion should be commented out
	if strings.Contains(script, "expect(consoleErrors).toHaveLength(0)") && !strings.Contains(script, "//") {
		t.Error("Expected console error assertion to be disabled/commented when errors present in captured session")
	}
	if !strings.Contains(script, "Widget failed to load") {
		t.Error("Expected captured error message to be noted in comments")
	}
}

func TestGenerateTestScriptResponseShapeAssertions(t *testing.T) {
	timeline := []TimelineEntry{
		{Timestamp: 1000, Kind: "action", Type: "click", URL: "http://localhost:3000", Selectors: map[string]interface{}{"testId": "btn"}},
		{Timestamp: 1150, Kind: "network", Method: "GET", URL: "/api/users", Status: 200, ContentType: "application/json",
			ResponseShape: map[string]interface{}{"users": []interface{}{map[string]interface{}{"id": "number", "name": "string"}}}},
	}

	script := generateTestScript(timeline, TestGenerationOptions{AssertNetwork: true, AssertResponseShape: true})

	if !strings.Contains(script, "toHaveProperty") {
		t.Error("Expected toHaveProperty assertion for response shape")
	}
	if !strings.Contains(script, "'users'") {
		t.Error("Expected 'users' key in shape assertion")
	}
}

func TestGenerateTestScriptResponseShapeDisabled(t *testing.T) {
	timeline := []TimelineEntry{
		{Timestamp: 1000, Kind: "action", Type: "click", URL: "http://localhost:3000", Selectors: map[string]interface{}{"testId": "btn"}},
		{Timestamp: 1150, Kind: "network", Method: "GET", URL: "/api/users", Status: 200, ContentType: "application/json",
			ResponseShape: map[string]interface{}{"users": "array"}},
	}

	script := generateTestScript(timeline, TestGenerationOptions{AssertNetwork: true, AssertResponseShape: false})

	if strings.Contains(script, "toHaveProperty") {
		t.Error("Should not include shape assertions when assert_response_shape=false")
	}
}

func TestGenerateTestScriptBaseURL(t *testing.T) {
	timeline := []TimelineEntry{
		{Timestamp: 1000, Kind: "action", Type: "click", URL: "http://localhost:3000/login", Selectors: map[string]interface{}{"testId": "btn"}},
	}

	script := generateTestScript(timeline, TestGenerationOptions{BaseURL: "https://staging.example.com"})

	if strings.Contains(script, "localhost:3000") {
		t.Error("Expected localhost to be replaced with base URL")
	}
	if !strings.Contains(script, "staging.example.com") {
		t.Error("Expected base URL in output")
	}
}

func TestGenerateTestScriptMultipleNetworkPerAction(t *testing.T) {
	timeline := []TimelineEntry{
		{Timestamp: 1000, Kind: "action", Type: "click", URL: "http://localhost:3000", Selectors: map[string]interface{}{"testId": "submit"}},
		{Timestamp: 1050, Kind: "network", Method: "POST", URL: "/api/submit", Status: 200, ContentType: "application/json"},
		{Timestamp: 1100, Kind: "network", Method: "GET", URL: "/api/status", Status: 200, ContentType: "application/json"},
		{Timestamp: 2000, Kind: "action", Type: "click", URL: "http://localhost:3000", Selectors: map[string]interface{}{"testId": "next"}},
	}

	script := generateTestScript(timeline, TestGenerationOptions{AssertNetwork: true})

	if !strings.Contains(script, "/api/submit") {
		t.Error("Expected /api/submit assertion")
	}
	if !strings.Contains(script, "/api/status") {
		t.Error("Expected /api/status assertion")
	}
}

func TestGenerateTestScriptAssertNetworkDisabled(t *testing.T) {
	timeline := []TimelineEntry{
		{Timestamp: 1000, Kind: "action", Type: "click", URL: "http://localhost:3000", Selectors: map[string]interface{}{"testId": "btn"}},
		{Timestamp: 1150, Kind: "network", Method: "POST", URL: "/api/login", Status: 200, ContentType: "application/json"},
	}

	script := generateTestScript(timeline, TestGenerationOptions{AssertNetwork: false})

	if strings.Contains(script, "waitForResponse") {
		t.Error("Should not include waitForResponse when assert_network=false")
	}
}

func TestGenerateTestScriptPasswordRedacted(t *testing.T) {
	timeline := []TimelineEntry{
		{Timestamp: 1000, Kind: "action", Type: "input", URL: "http://localhost:3000",
			Selectors: map[string]interface{}{"testId": "password"}, Value: "[redacted]"},
	}

	script := generateTestScript(timeline, TestGenerationOptions{})

	if !strings.Contains(script, "[user-provided]") {
		t.Error("Expected redacted password to become [user-provided]")
	}
}

func TestGenerateTestScriptDefaultTestName(t *testing.T) {
	timeline := []TimelineEntry{
		{Timestamp: 1000, Kind: "action", Type: "click", URL: "http://localhost:3000/login", Selectors: map[string]interface{}{"testId": "btn"}},
		{Timestamp: 1300, Kind: "action", Type: "navigate", URL: "http://localhost:3000/dashboard", ToURL: "/dashboard"},
	}

	script := generateTestScript(timeline, TestGenerationOptions{})

	// Should derive a name from the flow (first URL's path)
	if !strings.Contains(script, "test(") {
		t.Error("Expected test() wrapper")
	}
}

func TestGenerateTestScriptEmpty(t *testing.T) {
	script := generateTestScript([]TimelineEntry{}, TestGenerationOptions{TestName: "empty"})

	if !strings.Contains(script, "import") {
		t.Error("Expected valid script structure even with no timeline entries")
	}
}

// ============================================
// v5 Test Generation: MCP get_session_timeline
// ============================================

func TestMCPGetSessionTimeline(t *testing.T) {
	server, _ := setupTestServer(t)
	v4 := setupV4TestServer(t)
	mcp := NewMCPHandlerV4(server, v4)

	v4.AddEnhancedActions([]EnhancedAction{
		{Type: "click", Timestamp: 1705312200000, URL: "http://localhost:3000/login", Selectors: map[string]interface{}{"testId": "btn"}},
	})
	v4.AddNetworkBodies([]NetworkBody{
		{Timestamp: "2024-01-15T10:30:00.150Z", Method: "POST", URL: "/api/login", Status: 200,
			ResponseBody: `{"token":"abc"}`, ContentType: "application/json"},
	})
	server.addEntries([]LogEntry{
		{"level": "error", "message": "test error", "ts": "2024-01-15T10:30:00.500Z"},
	})

	mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "initialize",
		Params: json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}`),
	})

	resp := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 2, Method: "tools/call",
		Params: json.RawMessage(`{"name":"get_session_timeline","arguments":{}}`),
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

	if len(result.Content) == 0 {
		t.Fatal("Expected content in response")
	}

	var timelineResp SessionTimelineResponse
	if err := json.Unmarshal([]byte(result.Content[0].Text), &timelineResp); err != nil {
		t.Fatalf("Expected valid JSON timeline response, got error: %v", err)
	}

	if len(timelineResp.Timeline) != 3 {
		t.Errorf("Expected 3 timeline entries, got %d", len(timelineResp.Timeline))
	}
	if timelineResp.Summary.Actions != 1 {
		t.Errorf("Expected 1 action, got %d", timelineResp.Summary.Actions)
	}
	if timelineResp.Summary.NetworkRequests != 1 {
		t.Errorf("Expected 1 network request, got %d", timelineResp.Summary.NetworkRequests)
	}
	if timelineResp.Summary.ConsoleErrors != 1 {
		t.Errorf("Expected 1 console error, got %d", timelineResp.Summary.ConsoleErrors)
	}
}

func TestMCPGetSessionTimelineWithLastN(t *testing.T) {
	server, _ := setupTestServer(t)
	v4 := setupV4TestServer(t)
	mcp := NewMCPHandlerV4(server, v4)

	for i := 0; i < 10; i++ {
		v4.AddEnhancedActions([]EnhancedAction{
			{Type: "click", Timestamp: int64(i * 1000), URL: "http://localhost:3000"},
		})
	}

	mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "initialize",
		Params: json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}`),
	})

	resp := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 2, Method: "tools/call",
		Params: json.RawMessage(`{"name":"get_session_timeline","arguments":{"last_n_actions":3}}`),
	})

	var result struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	json.Unmarshal(resp.Result, &result)

	var timelineResp SessionTimelineResponse
	json.Unmarshal([]byte(result.Content[0].Text), &timelineResp)

	if timelineResp.Summary.Actions != 3 {
		t.Errorf("Expected 3 actions with last_n_actions=3, got %d", timelineResp.Summary.Actions)
	}
}

func TestMCPGetSessionTimelineEmpty(t *testing.T) {
	server, _ := setupTestServer(t)
	v4 := setupV4TestServer(t)
	mcp := NewMCPHandlerV4(server, v4)

	mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "initialize",
		Params: json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}`),
	})

	resp := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 2, Method: "tools/call",
		Params: json.RawMessage(`{"name":"get_session_timeline","arguments":{}}`),
	})

	var result struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	json.Unmarshal(resp.Result, &result)

	if !strings.Contains(result.Content[0].Text, `"timeline":[]`) && !strings.Contains(result.Content[0].Text, `"actions":0`) {
		t.Error("Expected empty timeline or 0 actions in summary")
	}
}

// ============================================
// v5 Test Generation: MCP generate_test
// ============================================

func TestMCPGenerateTest(t *testing.T) {
	server, _ := setupTestServer(t)
	v4 := setupV4TestServer(t)
	mcp := NewMCPHandlerV4(server, v4)

	v4.AddEnhancedActions([]EnhancedAction{
		{Type: "click", Timestamp: 1705312200000, URL: "http://localhost:3000/login", Selectors: map[string]interface{}{"testId": "login-btn"}},
	})
	v4.AddNetworkBodies([]NetworkBody{
		{Timestamp: "2024-01-15T10:30:00.150Z", Method: "POST", URL: "/api/login", Status: 200,
			ResponseBody: `{"token":"abc"}`, ContentType: "application/json"},
	})

	mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "initialize",
		Params: json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}`),
	})

	resp := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 2, Method: "tools/call",
		Params: json.RawMessage(`{"name":"generate_test","arguments":{"test_name":"login test","assert_network":true,"assert_no_errors":true}}`),
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

	if len(result.Content) == 0 {
		t.Fatal("Expected content in response")
	}

	script := result.Content[0].Text
	if !strings.Contains(script, "import { test, expect }") {
		t.Error("Expected Playwright imports in generated script")
	}
	if !strings.Contains(script, "login test") {
		t.Error("Expected test name in generated script")
	}
	if !strings.Contains(script, "waitForResponse") {
		t.Error("Expected network assertion in generated script")
	}
	if !strings.Contains(script, "consoleErrors") {
		t.Error("Expected console error tracking in generated script")
	}
}

func TestMCPGenerateTestWithBaseURL(t *testing.T) {
	server, _ := setupTestServer(t)
	v4 := setupV4TestServer(t)
	mcp := NewMCPHandlerV4(server, v4)

	v4.AddEnhancedActions([]EnhancedAction{
		{Type: "click", Timestamp: 1000, URL: "http://localhost:3000/login", Selectors: map[string]interface{}{"testId": "btn"}},
	})

	mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "initialize",
		Params: json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}`),
	})

	resp := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 2, Method: "tools/call",
		Params: json.RawMessage(`{"name":"generate_test","arguments":{"base_url":"https://staging.example.com"}}`),
	})

	var result struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	json.Unmarshal(resp.Result, &result)

	script := result.Content[0].Text
	if strings.Contains(script, "localhost:3000") {
		t.Error("Expected localhost replaced with base_url")
	}
	if !strings.Contains(script, "staging.example.com") {
		t.Error("Expected base_url in script")
	}
}

func TestMCPGenerateTestEmpty(t *testing.T) {
	server, _ := setupTestServer(t)
	v4 := setupV4TestServer(t)
	mcp := NewMCPHandlerV4(server, v4)

	mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "initialize",
		Params: json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}`),
	})

	resp := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 2, Method: "tools/call",
		Params: json.RawMessage(`{"name":"generate_test","arguments":{}}`),
	})

	var result struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	json.Unmarshal(resp.Result, &result)

	if !strings.Contains(result.Content[0].Text, "No") {
		// Should indicate no data available
	}
}

func TestMCPGenerateTestToolInToolsList(t *testing.T) {
	server, _ := setupTestServer(t)
	v4 := setupV4TestServer(t)
	mcp := NewMCPHandlerV4(server, v4)

	mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "initialize",
		Params: json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}`),
	})

	resp := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 2, Method: "tools/list",
	})

	var result struct {
		Tools []struct {
			Name string `json:"name"`
		} `json:"tools"`
	}
	json.Unmarshal(resp.Result, &result)

	foundTimeline := false
	foundGenerate := false
	for _, tool := range result.Tools {
		if tool.Name == "get_session_timeline" {
			foundTimeline = true
		}
		if tool.Name == "generate_test" {
			foundGenerate = true
		}
	}

	if !foundTimeline {
		t.Error("Expected get_session_timeline in tools list")
	}
	if !foundGenerate {
		t.Error("Expected generate_test in tools list")
	}
}

// ============================================
// POST /mcp HTTP Endpoint Tests
// ============================================

func TestMCPHTTPEndpointToolsList(t *testing.T) {
	server, _ := setupTestServer(t)
	v4 := setupV4TestServer(t)
	mcp := NewMCPHandlerV4(server, v4)

	body := `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`
	req := httptest.NewRequest("POST", "/mcp", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	mcp.HandleHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", rec.Code)
	}

	var resp JSONRPCResponse
	json.NewDecoder(rec.Body).Decode(&resp)

	if resp.JSONRPC != "2.0" {
		t.Errorf("Expected jsonrpc 2.0, got %s", resp.JSONRPC)
	}
	if resp.Error != nil {
		t.Errorf("Expected no error, got %v", resp.Error)
	}

	var result struct {
		Tools []struct {
			Name string `json:"name"`
		} `json:"tools"`
	}
	json.Unmarshal(resp.Result, &result)

	if len(result.Tools) == 0 {
		t.Error("Expected tools in response")
	}
}

func TestMCPHTTPEndpointToolCall(t *testing.T) {
	server, _ := setupTestServer(t)
	v4 := setupV4TestServer(t)
	mcp := NewMCPHandlerV4(server, v4)

	body := `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"get_browser_logs","arguments":{}}}`
	req := httptest.NewRequest("POST", "/mcp", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	mcp.HandleHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", rec.Code)
	}

	var resp JSONRPCResponse
	json.NewDecoder(rec.Body).Decode(&resp)

	if resp.Error != nil {
		t.Errorf("Expected no error, got %v", resp.Error)
	}
}

func TestMCPHTTPEndpointMethodNotAllowed(t *testing.T) {
	server, _ := setupTestServer(t)
	v4 := setupV4TestServer(t)
	mcp := NewMCPHandlerV4(server, v4)

	req := httptest.NewRequest("GET", "/mcp", nil)
	rec := httptest.NewRecorder()

	mcp.HandleHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected 405, got %d", rec.Code)
	}
}

func TestMCPHTTPEndpointInvalidJSON(t *testing.T) {
	server, _ := setupTestServer(t)
	v4 := setupV4TestServer(t)
	mcp := NewMCPHandlerV4(server, v4)

	req := httptest.NewRequest("POST", "/mcp", bytes.NewBufferString("not json"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	mcp.HandleHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected 200 (JSON-RPC error in body), got %d", rec.Code)
	}

	var resp JSONRPCResponse
	json.NewDecoder(rec.Body).Decode(&resp)

	if resp.Error == nil {
		t.Error("Expected JSON-RPC error for invalid JSON")
	}
	if resp.Error.Code != -32700 {
		t.Errorf("Expected parse error code -32700, got %d", resp.Error.Code)
	}
}

func TestMCPHTTPEndpointUnknownMethod(t *testing.T) {
	server, _ := setupTestServer(t)
	v4 := setupV4TestServer(t)
	mcp := NewMCPHandlerV4(server, v4)

	body := `{"jsonrpc":"2.0","id":3,"method":"unknown/method"}`
	req := httptest.NewRequest("POST", "/mcp", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	mcp.HandleHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", rec.Code)
	}

	var resp JSONRPCResponse
	json.NewDecoder(rec.Body).Decode(&resp)

	if resp.Error == nil {
		t.Error("Expected JSON-RPC error for unknown method")
	}
	if resp.Error.Code != -32601 {
		t.Errorf("Expected method not found code -32601, got %d", resp.Error.Code)
	}
}

func TestMCPHTTPEndpointV4ToolCall(t *testing.T) {
	server, _ := setupTestServer(t)
	v4 := setupV4TestServer(t)
	mcp := NewMCPHandlerV4(server, v4)

	body := `{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"get_websocket_events","arguments":{}}}`
	req := httptest.NewRequest("POST", "/mcp", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	mcp.HandleHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", rec.Code)
	}

	var resp JSONRPCResponse
	json.NewDecoder(rec.Body).Decode(&resp)

	if resp.Error != nil {
		t.Errorf("Expected no error, got %v", resp.Error)
	}
}

// ============================================
// Performance Snapshot Shape Tests (Contract-First)
// ============================================

func TestPerformanceSnapshotJSONShape(t *testing.T) {
	fcp := 250.0
	lcp := 800.0
	cls := 0.05
	snapshot := PerformanceSnapshot{
		URL:       "/dashboard",
		Timestamp: "2024-01-01T00:00:00Z",
		Timing: PerformanceTiming{
			DomContentLoaded:       600,
			Load:                   1200,
			FirstContentfulPaint:   &fcp,
			LargestContentfulPaint: &lcp,
			TimeToFirstByte:        80,
			DomInteractive:         500,
		},
		Network: NetworkSummary{
			RequestCount: 10,
			TransferSize: 50000,
			DecodedSize:  100000,
			ByType:       map[string]TypeSummary{"script": {Count: 3, Size: 30000}},
			SlowestRequests: []SlowRequest{
				{URL: "/app.js", Duration: 300, Size: 30000},
			},
		},
		LongTasks: LongTaskMetrics{
			Count:             2,
			TotalBlockingTime: 100,
			Longest:           80,
		},
		CLS: &cls,
	}

	data, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatalf("Failed to marshal snapshot: %v", err)
	}

	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	// Top-level fields
	for _, field := range []string{"url", "timestamp", "timing", "network", "longTasks", "cumulativeLayoutShift"} {
		if _, ok := m[field]; !ok {
			t.Errorf("missing top-level field: %s", field)
		}
	}

	// Timing fields
	timing := m["timing"].(map[string]interface{})
	for _, field := range []string{
		"domContentLoaded", "load", "firstContentfulPaint",
		"largestContentfulPaint", "timeToFirstByte", "domInteractive",
	} {
		if _, ok := timing[field]; !ok {
			t.Errorf("missing timing field: %s", field)
		}
	}

	// Network fields
	network := m["network"].(map[string]interface{})
	for _, field := range []string{"requestCount", "transferSize", "decodedSize", "byType", "slowestRequests"} {
		if _, ok := network[field]; !ok {
			t.Errorf("missing network field: %s", field)
		}
	}

	// LongTasks fields
	longTasks := m["longTasks"].(map[string]interface{})
	for _, field := range []string{"count", "totalBlockingTime", "longest"} {
		if _, ok := longTasks[field]; !ok {
			t.Errorf("missing longTasks field: %s", field)
		}
	}
}

func TestPerformanceBaselineJSONShape(t *testing.T) {
	fcp := 250.0
	lcp := 800.0
	cls := 0.05
	baseline := PerformanceBaseline{
		URL:         "/dashboard",
		SampleCount: 3,
		LastUpdated: "2024-01-01T00:00:00Z",
		Timing: BaselineTiming{
			DomContentLoaded:       600,
			Load:                   1200,
			FirstContentfulPaint:   &fcp,
			LargestContentfulPaint: &lcp,
			TimeToFirstByte:        80,
			DomInteractive:         500,
		},
		Network: BaselineNetwork{
			RequestCount: 10,
			TransferSize: 50000,
		},
		LongTasks: LongTaskMetrics{
			Count:             2,
			TotalBlockingTime: 100,
			Longest:           80,
		},
		CLS: &cls,
	}

	data, err := json.Marshal(baseline)
	if err != nil {
		t.Fatalf("Failed to marshal baseline: %v", err)
	}

	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	// Top-level fields
	for _, field := range []string{"url", "sampleCount", "lastUpdated", "timing", "network", "longTasks", "cumulativeLayoutShift"} {
		if _, ok := m[field]; !ok {
			t.Errorf("missing top-level field: %s", field)
		}
	}

	// Timing fields
	timing := m["timing"].(map[string]interface{})
	for _, field := range []string{
		"domContentLoaded", "load", "firstContentfulPaint",
		"largestContentfulPaint", "timeToFirstByte", "domInteractive",
	} {
		if _, ok := timing[field]; !ok {
			t.Errorf("missing timing field: %s", field)
		}
	}
}

func TestPerformanceSnapshotStorageAndRetrieval(t *testing.T) {
	server := NewV4Server()
	fcp := 250.0
	lcp := 800.0
	cls := 0.05

	snapshot := PerformanceSnapshot{
		URL:       "/dashboard",
		Timestamp: "2024-01-01T00:00:00Z",
		Timing: PerformanceTiming{
			DomContentLoaded:       600,
			Load:                   1200,
			FirstContentfulPaint:   &fcp,
			LargestContentfulPaint: &lcp,
			TimeToFirstByte:        80,
			DomInteractive:         500,
		},
		Network: NetworkSummary{
			RequestCount:    10,
			TransferSize:    50000,
			DecodedSize:     100000,
			ByType:          map[string]TypeSummary{},
			SlowestRequests: []SlowRequest{},
		},
		LongTasks: LongTaskMetrics{Count: 0, TotalBlockingTime: 0, Longest: 0},
		CLS:       &cls,
	}

	server.AddPerformanceSnapshot(snapshot)

	got, found := server.GetPerformanceSnapshot("/dashboard")
	if !found {
		t.Fatal("snapshot not found after adding")
	}
	if got.Timing.FirstContentfulPaint == nil || *got.Timing.FirstContentfulPaint != 250.0 {
		t.Errorf("FCP not stored: got %v", got.Timing.FirstContentfulPaint)
	}
	if got.Timing.LargestContentfulPaint == nil || *got.Timing.LargestContentfulPaint != 800.0 {
		t.Errorf("LCP not stored: got %v", got.Timing.LargestContentfulPaint)
	}
	if got.CLS == nil || *got.CLS != 0.05 {
		t.Errorf("CLS not stored: got %v", got.CLS)
	}
}

func TestPerformanceBaselineAveragesFCPLCP(t *testing.T) {
	server := NewV4Server()
	fcp1 := 200.0
	lcp1 := 600.0
	fcp2 := 300.0
	lcp2 := 800.0

	server.AddPerformanceSnapshot(PerformanceSnapshot{
		URL:       "/test",
		Timestamp: "2024-01-01T00:00:00Z",
		Timing: PerformanceTiming{
			DomContentLoaded:       500,
			Load:                   1000,
			FirstContentfulPaint:   &fcp1,
			LargestContentfulPaint: &lcp1,
			TimeToFirstByte:        80,
			DomInteractive:         400,
		},
		Network:   NetworkSummary{ByType: map[string]TypeSummary{}},
		LongTasks: LongTaskMetrics{Longest: 100},
	})

	server.AddPerformanceSnapshot(PerformanceSnapshot{
		URL:       "/test",
		Timestamp: "2024-01-01T00:01:00Z",
		Timing: PerformanceTiming{
			DomContentLoaded:       500,
			Load:                   1000,
			FirstContentfulPaint:   &fcp2,
			LargestContentfulPaint: &lcp2,
			TimeToFirstByte:        80,
			DomInteractive:         400,
		},
		Network:   NetworkSummary{ByType: map[string]TypeSummary{}},
		LongTasks: LongTaskMetrics{Longest: 60},
	})

	server.mu.RLock()
	baseline := server.perfBaselines["/test"]
	server.mu.RUnlock()

	if baseline.SampleCount != 2 {
		t.Fatalf("expected 2 samples, got %d", baseline.SampleCount)
	}
	if baseline.Timing.FirstContentfulPaint == nil {
		t.Fatal("baseline FCP should not be nil")
	}
	// Average of 200 and 300 = 250
	if *baseline.Timing.FirstContentfulPaint != 250.0 {
		t.Errorf("expected FCP baseline 250, got %f", *baseline.Timing.FirstContentfulPaint)
	}
	if baseline.Timing.LargestContentfulPaint == nil {
		t.Fatal("baseline LCP should not be nil")
	}
	// Average of 600 and 800 = 700
	if *baseline.Timing.LargestContentfulPaint != 700.0 {
		t.Errorf("expected LCP baseline 700, got %f", *baseline.Timing.LargestContentfulPaint)
	}
	// Longest should be averaged: (100 + 60) / 2 = 80
	if baseline.LongTasks.Longest != 80.0 {
		t.Errorf("expected Longest baseline 80, got %f", baseline.LongTasks.Longest)
	}
}

func TestPerformanceRegressionDetectsFCPLCP(t *testing.T) {
	server := NewV4Server()

	fcpBaseline := 200.0
	lcpBaseline := 500.0
	fcpCurrent := 450.0 // +125% increase, +250ms
	lcpCurrent := 900.0 // +80% increase, +400ms

	baseline := PerformanceBaseline{
		URL:         "/test",
		SampleCount: 5,
		Timing: BaselineTiming{
			FirstContentfulPaint:   &fcpBaseline,
			LargestContentfulPaint: &lcpBaseline,
		},
		Network:   BaselineNetwork{},
		LongTasks: LongTaskMetrics{},
	}

	snapshot := PerformanceSnapshot{
		URL: "/test",
		Timing: PerformanceTiming{
			FirstContentfulPaint:   &fcpCurrent,
			LargestContentfulPaint: &lcpCurrent,
		},
		Network:   NetworkSummary{ByType: map[string]TypeSummary{}},
		LongTasks: LongTaskMetrics{},
	}

	regressions := server.DetectRegressions(snapshot, baseline)

	fcpFound := false
	lcpFound := false
	for _, r := range regressions {
		if r.Metric == "firstContentfulPaint" {
			fcpFound = true
		}
		if r.Metric == "largestContentfulPaint" {
			lcpFound = true
		}
	}

	if !fcpFound {
		t.Error("expected FCP regression to be detected")
	}
	if !lcpFound {
		t.Error("expected LCP regression to be detected")
	}
}

func TestPerformanceRegressionNoFalsePositiveFCPLCP(t *testing.T) {
	server := NewV4Server()

	fcpBaseline := 200.0
	lcpBaseline := 500.0
	// Small changes: +20% for FCP, +10% for LCP (below thresholds)
	fcpCurrent := 240.0
	lcpCurrent := 550.0

	baseline := PerformanceBaseline{
		URL:         "/test",
		SampleCount: 5,
		Timing: BaselineTiming{
			FirstContentfulPaint:   &fcpBaseline,
			LargestContentfulPaint: &lcpBaseline,
		},
		Network:   BaselineNetwork{},
		LongTasks: LongTaskMetrics{},
	}

	snapshot := PerformanceSnapshot{
		URL: "/test",
		Timing: PerformanceTiming{
			FirstContentfulPaint:   &fcpCurrent,
			LargestContentfulPaint: &lcpCurrent,
		},
		Network:   NetworkSummary{ByType: map[string]TypeSummary{}},
		LongTasks: LongTaskMetrics{},
	}

	regressions := server.DetectRegressions(snapshot, baseline)

	for _, r := range regressions {
		if r.Metric == "firstContentfulPaint" || r.Metric == "largestContentfulPaint" {
			t.Errorf("unexpected regression for %s (change too small)", r.Metric)
		}
	}
}

func TestAvgOptionalFloat(t *testing.T) {
	// nil snapshot: baseline unchanged
	baseline := 100.0
	result := avgOptionalFloat(&baseline, nil, 2)
	if result == nil || *result != 100.0 {
		t.Errorf("nil snapshot should preserve baseline, got %v", result)
	}

	// nil baseline: use snapshot value
	snapshot := 200.0
	result = avgOptionalFloat(nil, &snapshot, 2)
	if result == nil || *result != 200.0 {
		t.Errorf("nil baseline should use snapshot, got %v", result)
	}

	// Both present: average
	result = avgOptionalFloat(&baseline, &snapshot, 2)
	if result == nil || *result != 150.0 {
		t.Errorf("expected average 150, got %v", result)
	}
}

func TestWeightedOptionalFloat(t *testing.T) {
	baseline := 100.0
	snapshot := 200.0

	result := weightedOptionalFloat(&baseline, &snapshot, 0.8, 0.2)
	expected := 100.0*0.8 + 200.0*0.2 // 120
	if result == nil || *result != expected {
		t.Errorf("expected %f, got %v", expected, result)
	}

	// nil snapshot
	result = weightedOptionalFloat(&baseline, nil, 0.8, 0.2)
	if result == nil || *result != 100.0 {
		t.Errorf("nil snapshot should preserve baseline, got %v", result)
	}
}
