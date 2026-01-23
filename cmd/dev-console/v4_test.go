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
		Type: "dom",
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
	for i := 0; i < 50; i++ { // 50 * 100KB = 5MB
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
	for i := 0; i < 50; i++ { // 50 * 200KB = 10MB
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
		Content []struct{ Text string `json:"text"` } `json:"content"`
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
		Content []struct{ Text string `json:"text"` } `json:"content"`
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
		Content []struct{ Text string `json:"text"` } `json:"content"`
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
		Content []struct{ Text string `json:"text"` } `json:"content"`
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
		Content []struct{ Text string `json:"text"` } `json:"content"`
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
		Content []struct{ Text string `json:"text"` } `json:"content"`
		IsError bool                                   `json:"isError"`
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
		Content []struct{ Text string `json:"text"` } `json:"content"`
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
		Tools []struct{ Name string `json:"name"` } `json:"tools"`
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
		Content []struct{ Text string `json:"text"` } `json:"content"`
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
		Content []struct{ Text string `json:"text"` } `json:"content"`
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
