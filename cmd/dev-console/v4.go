package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// ============================================
// v4 Types
// ============================================

// WebSocketEvent represents a captured WebSocket event
type WebSocketEvent struct {
	Timestamp   string        `json:"ts,omitempty"`
	Type        string        `json:"type,omitempty"`
	Event       string        `json:"event"`
	ID          string        `json:"id"`
	URL         string        `json:"url,omitempty"`
	Direction   string        `json:"direction,omitempty"`
	Data        string        `json:"data,omitempty"`
	Size        int           `json:"size,omitempty"`
	CloseCode   int           `json:"code,omitempty"`
	CloseReason string        `json:"reason,omitempty"`
	Sampled     *SamplingInfo `json:"sampled,omitempty"`
}

// SamplingInfo describes the sampling state when a message was captured
type SamplingInfo struct {
	Rate   string `json:"rate"`
	Logged string `json:"logged"`
	Window string `json:"window"`
}

// WebSocketEventFilter defines filtering criteria for events
type WebSocketEventFilter struct {
	ConnectionID string
	URLFilter    string
	Direction    string
	Limit        int
}

// WebSocketStatusFilter defines filtering criteria for status
type WebSocketStatusFilter struct {
	URLFilter    string
	ConnectionID string
}

// WebSocketStatusResponse is the response from get_websocket_status
type WebSocketStatusResponse struct {
	Connections []WebSocketConnection       `json:"connections"`
	Closed      []WebSocketClosedConnection `json:"closed"`
}

// WebSocketConnection represents an active WebSocket connection
type WebSocketConnection struct {
	ID          string                  `json:"id"`
	URL         string                  `json:"url"`
	State       string                  `json:"state"`
	OpenedAt    string                  `json:"openedAt,omitempty"`
	Duration    string                  `json:"duration,omitempty"`
	MessageRate WebSocketMessageRate    `json:"messageRate"`
	LastMessage WebSocketLastMessage    `json:"lastMessage"`
	Schema      *WebSocketSchema        `json:"schema,omitempty"`
	Sampling    WebSocketSamplingStatus `json:"sampling"`
}

// WebSocketClosedConnection represents a closed WebSocket connection
type WebSocketClosedConnection struct {
	ID            string `json:"id"`
	URL           string `json:"url"`
	State         string `json:"state"`
	OpenedAt      string `json:"openedAt,omitempty"`
	ClosedAt      string `json:"closedAt,omitempty"`
	CloseCode     int    `json:"closeCode"`
	CloseReason   string `json:"closeReason"`
	TotalMessages struct {
		Incoming int `json:"incoming"`
		Outgoing int `json:"outgoing"`
	} `json:"totalMessages"`
}

// WebSocketMessageRate contains rate info for a direction
type WebSocketMessageRate struct {
	Incoming WebSocketDirectionStats `json:"incoming"`
	Outgoing WebSocketDirectionStats `json:"outgoing"`
}

// WebSocketDirectionStats contains stats for a message direction
type WebSocketDirectionStats struct {
	PerSecond float64 `json:"perSecond"`
	Total     int     `json:"total"`
	Bytes     int     `json:"bytes"`
}

// WebSocketLastMessage contains last message info
type WebSocketLastMessage struct {
	Incoming *WebSocketMessagePreview `json:"incoming,omitempty"`
	Outgoing *WebSocketMessagePreview `json:"outgoing,omitempty"`
}

// WebSocketMessagePreview contains a preview of the last message
type WebSocketMessagePreview struct {
	At      string `json:"at"`
	Age     string `json:"age"`
	Preview string `json:"preview"`
}

// WebSocketSchema describes detected message schema
type WebSocketSchema struct {
	DetectedKeys []string `json:"detectedKeys,omitempty"`
	MessageCount int      `json:"messageCount"`
	Consistent   bool     `json:"consistent"`
	Variants     []string `json:"variants,omitempty"`
}

// WebSocketSamplingStatus describes sampling state
type WebSocketSamplingStatus struct {
	Active bool   `json:"active"`
	Rate   string `json:"rate,omitempty"`
	Reason string `json:"reason,omitempty"`
}

// NetworkBody represents a captured network request/response
type NetworkBody struct {
	Timestamp         string `json:"ts,omitempty"`
	Method            string `json:"method"`
	URL               string `json:"url"`
	Status            int    `json:"status"`
	RequestBody       string `json:"requestBody,omitempty"`
	ResponseBody      string `json:"responseBody,omitempty"`
	ContentType       string `json:"contentType,omitempty"`
	Duration          int    `json:"duration,omitempty"`
	RequestTruncated  bool   `json:"requestTruncated,omitempty"`
	ResponseTruncated bool   `json:"responseTruncated,omitempty"`
}

// NetworkBodyFilter defines filtering criteria for network bodies
type NetworkBodyFilter struct {
	URLFilter string
	Method    string
	StatusMin int
	StatusMax int
	Limit     int
}

// PendingQuery represents a query waiting for extension response
type PendingQuery struct {
	Type   string          `json:"type"`
	Params json.RawMessage `json:"params"`
}

// PendingQueryResponse is the response format for pending queries
type PendingQueryResponse struct {
	ID     string          `json:"id"`
	Type   string          `json:"type"`
	Params json.RawMessage `json:"params"`
}

// ============================================
// Internal types
// ============================================

// connectionState tracks state for an active connection
type connectionState struct {
	id         string
	url        string
	state      string
	openedAt   string
	incoming   directionStats
	outgoing   directionStats
	sampling   bool
	lastSample *SamplingInfo
}

type directionStats struct {
	total     int
	bytes     int
	lastAt    string
	lastData  string
}

// pendingQueryEntry tracks a pending query with timeout
type pendingQueryEntry struct {
	query   PendingQueryResponse
	expires time.Time
}

// ============================================
// Constants
// ============================================

const (
	maxWSEvents         = 500
	maxNetworkBodies    = 100
	maxActiveConns      = 20
	maxClosedConns      = 10
	maxPendingQueries   = 5
	defaultWSLimit      = 50
	defaultBodyLimit    = 20
	maxRequestBodySize  = 8192  // 8KB
	maxResponseBodySize = 16384 // 16KB
	wsBufferMemoryLimit = 4 * 1024 * 1024 // 4MB
	nbBufferMemoryLimit = 8 * 1024 * 1024 // 8MB
	rateLimitThreshold  = 1000
	memoryHardLimit     = 50 * 1024 * 1024 // 50MB
	defaultQueryTimeout = 10 * time.Second
)

// ============================================
// V4Server
// ============================================

// V4Server handles v4-specific state and operations
type V4Server struct {
	mu sync.RWMutex

	// WebSocket event ring buffer
	wsEvents []WebSocketEvent

	// Network bodies ring buffer
	networkBodies []NetworkBody

	// Connection tracker
	connections    map[string]*connectionState
	closedConns    []WebSocketClosedConnection
	connOrder      []string // Track insertion order for eviction

	// Pending queries
	pendingQueries []pendingQueryEntry
	queryResults   map[string]json.RawMessage
	queryCond      *sync.Cond
	queryIDCounter int

	// Rate limiting
	eventCount    int
	rateResetTime time.Time

	// Memory simulation (for testing)
	simulatedMemory int64

	// Query timeout
	queryTimeout time.Duration
}

// NewV4Server creates a new v4 server instance
func NewV4Server() *V4Server {
	v4 := &V4Server{
		wsEvents:       make([]WebSocketEvent, 0, maxWSEvents),
		networkBodies:  make([]NetworkBody, 0, maxNetworkBodies),
		connections:    make(map[string]*connectionState),
		closedConns:    make([]WebSocketClosedConnection, 0),
		connOrder:      make([]string, 0),
		pendingQueries: make([]pendingQueryEntry, 0),
		queryResults:   make(map[string]json.RawMessage),
		rateResetTime:  time.Now(),
		queryTimeout:   defaultQueryTimeout,
	}
	v4.queryCond = sync.NewCond(&v4.mu)
	return v4
}

// ============================================
// WebSocket Events
// ============================================

// AddWebSocketEvents adds WebSocket events to the buffer
func (v *V4Server) AddWebSocketEvents(events []WebSocketEvent) {
	v.mu.Lock()
	defer v.mu.Unlock()

	for _, event := range events {
		// Track connection state
		v.trackConnection(event)

		// Add to ring buffer
		v.wsEvents = append(v.wsEvents, event)
	}

	// Enforce max count
	if len(v.wsEvents) > maxWSEvents {
		v.wsEvents = v.wsEvents[len(v.wsEvents)-maxWSEvents:]
	}

	// Enforce memory limit
	v.evictWSForMemory()
}

// evictWSForMemory removes oldest events if memory exceeds limit
func (v *V4Server) evictWSForMemory() {
	for v.calcWSMemory() > wsBufferMemoryLimit && len(v.wsEvents) > 0 {
		v.wsEvents = v.wsEvents[1:]
	}
}

// calcWSMemory approximates memory usage of WS buffer
func (v *V4Server) calcWSMemory() int64 {
	var total int64
	for _, e := range v.wsEvents {
		total += int64(len(e.Data) + len(e.URL) + len(e.ID) + len(e.Timestamp) + len(e.Direction) + len(e.Event) + 64)
	}
	return total
}

// GetWebSocketEventCount returns the current number of buffered events
func (v *V4Server) GetWebSocketEventCount() int {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return len(v.wsEvents)
}

// GetWebSocketEvents returns filtered WebSocket events (newest first)
func (v *V4Server) GetWebSocketEvents(filter WebSocketEventFilter) []WebSocketEvent {
	v.mu.RLock()
	defer v.mu.RUnlock()

	limit := filter.Limit
	if limit <= 0 {
		limit = defaultWSLimit
	}

	// Filter events
	var filtered []WebSocketEvent
	for _, e := range v.wsEvents {
		if filter.ConnectionID != "" && e.ID != filter.ConnectionID {
			continue
		}
		if filter.URLFilter != "" && !strings.Contains(e.URL, filter.URLFilter) {
			continue
		}
		if filter.Direction != "" && e.Direction != filter.Direction {
			continue
		}
		filtered = append(filtered, e)
	}

	// Reverse for newest first
	for i, j := 0, len(filtered)-1; i < j; i, j = i+1, j-1 {
		filtered[i], filtered[j] = filtered[j], filtered[i]
	}

	// Apply limit
	if len(filtered) > limit {
		filtered = filtered[:limit]
	}

	return filtered
}

// trackConnection updates connection state from events
func (v *V4Server) trackConnection(event WebSocketEvent) {
	switch event.Event {
	case "open":
		// Enforce max active connections
		if len(v.connections) >= maxActiveConns {
			// Evict oldest
			if len(v.connOrder) > 0 {
				oldestID := v.connOrder[0]
				delete(v.connections, oldestID)
				v.connOrder = v.connOrder[1:]
			}
		}
		v.connections[event.ID] = &connectionState{
			id:       event.ID,
			url:      event.URL,
			state:    "open",
			openedAt: event.Timestamp,
		}
		v.connOrder = append(v.connOrder, event.ID)

	case "close":
		conn := v.connections[event.ID]
		if conn == nil {
			return
		}
		// Move to closed
		closed := WebSocketClosedConnection{
			ID:          event.ID,
			URL:         conn.url,
			State:       "closed",
			OpenedAt:    conn.openedAt,
			ClosedAt:    event.Timestamp,
			CloseCode:   event.CloseCode,
			CloseReason: event.CloseReason,
		}
		closed.TotalMessages.Incoming = conn.incoming.total
		closed.TotalMessages.Outgoing = conn.outgoing.total

		v.closedConns = append(v.closedConns, closed)
		if len(v.closedConns) > maxClosedConns {
			v.closedConns = v.closedConns[len(v.closedConns)-maxClosedConns:]
		}

		delete(v.connections, event.ID)
		// Remove from order
		for i, id := range v.connOrder {
			if id == event.ID {
				v.connOrder = append(v.connOrder[:i], v.connOrder[i+1:]...)
				break
			}
		}

	case "error":
		conn := v.connections[event.ID]
		if conn != nil {
			conn.state = "error"
		}

	case "message":
		conn := v.connections[event.ID]
		if conn == nil {
			return
		}
		if event.Direction == "incoming" {
			conn.incoming.total++
			conn.incoming.bytes += event.Size
			conn.incoming.lastAt = event.Timestamp
			conn.incoming.lastData = event.Data
		} else if event.Direction == "outgoing" {
			conn.outgoing.total++
			conn.outgoing.bytes += event.Size
			conn.outgoing.lastAt = event.Timestamp
			conn.outgoing.lastData = event.Data
		}
		if event.Sampled != nil {
			conn.sampling = true
			conn.lastSample = event.Sampled
		}
	}
}

// GetWebSocketStatus returns current connection states
func (v *V4Server) GetWebSocketStatus(filter WebSocketStatusFilter) WebSocketStatusResponse {
	v.mu.RLock()
	defer v.mu.RUnlock()

	resp := WebSocketStatusResponse{
		Connections: make([]WebSocketConnection, 0),
		Closed:      make([]WebSocketClosedConnection, 0),
	}

	for _, conn := range v.connections {
		if filter.URLFilter != "" && !strings.Contains(conn.url, filter.URLFilter) {
			continue
		}
		if filter.ConnectionID != "" && conn.id != filter.ConnectionID {
			continue
		}

		wc := WebSocketConnection{
			ID:       conn.id,
			URL:      conn.url,
			State:    conn.state,
			OpenedAt: conn.openedAt,
			MessageRate: WebSocketMessageRate{
				Incoming: WebSocketDirectionStats{
					Total: conn.incoming.total,
					Bytes: conn.incoming.bytes,
				},
				Outgoing: WebSocketDirectionStats{
					Total: conn.outgoing.total,
					Bytes: conn.outgoing.bytes,
				},
			},
			Sampling: WebSocketSamplingStatus{
				Active: conn.sampling,
			},
		}

		if conn.incoming.lastData != "" {
			wc.LastMessage.Incoming = &WebSocketMessagePreview{
				At:      conn.incoming.lastAt,
				Preview: conn.incoming.lastData,
			}
		}
		if conn.outgoing.lastData != "" {
			wc.LastMessage.Outgoing = &WebSocketMessagePreview{
				At:      conn.outgoing.lastAt,
				Preview: conn.outgoing.lastData,
			}
		}

		resp.Connections = append(resp.Connections, wc)
	}

	for _, closed := range v.closedConns {
		if filter.URLFilter != "" && !strings.Contains(closed.URL, filter.URLFilter) {
			continue
		}
		if filter.ConnectionID != "" && closed.ID != filter.ConnectionID {
			continue
		}
		resp.Closed = append(resp.Closed, closed)
	}

	return resp
}

// ============================================
// Network Bodies
// ============================================

// AddNetworkBodies adds network bodies to the buffer
func (v *V4Server) AddNetworkBodies(bodies []NetworkBody) {
	v.mu.Lock()
	defer v.mu.Unlock()

	for i := range bodies {
		// Truncate request body
		if len(bodies[i].RequestBody) > maxRequestBodySize {
			bodies[i].RequestBody = bodies[i].RequestBody[:maxRequestBodySize]
			bodies[i].RequestTruncated = true
		}
		// Truncate response body
		if len(bodies[i].ResponseBody) > maxResponseBodySize {
			bodies[i].ResponseBody = bodies[i].ResponseBody[:maxResponseBodySize]
			bodies[i].ResponseTruncated = true
		}
		v.networkBodies = append(v.networkBodies, bodies[i])
	}

	// Enforce max count
	if len(v.networkBodies) > maxNetworkBodies {
		v.networkBodies = v.networkBodies[len(v.networkBodies)-maxNetworkBodies:]
	}

	// Enforce memory limit
	v.evictNBForMemory()
}

// evictNBForMemory removes oldest bodies if memory exceeds limit
func (v *V4Server) evictNBForMemory() {
	for v.calcNBMemory() > nbBufferMemoryLimit && len(v.networkBodies) > 0 {
		v.networkBodies = v.networkBodies[1:]
	}
}

// calcNBMemory approximates memory usage of network bodies buffer
func (v *V4Server) calcNBMemory() int64 {
	var total int64
	for _, b := range v.networkBodies {
		total += int64(len(b.RequestBody) + len(b.ResponseBody) + len(b.URL) + len(b.Method) + 64)
	}
	return total
}

// GetNetworkBodyCount returns the current number of buffered bodies
func (v *V4Server) GetNetworkBodyCount() int {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return len(v.networkBodies)
}

// GetNetworkBodies returns filtered network bodies (newest first)
func (v *V4Server) GetNetworkBodies(filter NetworkBodyFilter) []NetworkBody {
	v.mu.RLock()
	defer v.mu.RUnlock()

	limit := filter.Limit
	if limit <= 0 {
		limit = defaultBodyLimit
	}

	var filtered []NetworkBody
	for _, b := range v.networkBodies {
		if filter.URLFilter != "" && !strings.Contains(b.URL, filter.URLFilter) {
			continue
		}
		if filter.Method != "" && b.Method != filter.Method {
			continue
		}
		if filter.StatusMin > 0 && b.Status < filter.StatusMin {
			continue
		}
		if filter.StatusMax > 0 && b.Status > filter.StatusMax {
			continue
		}
		filtered = append(filtered, b)
	}

	// Reverse for newest first
	for i, j := 0, len(filtered)-1; i < j; i, j = i+1, j-1 {
		filtered[i], filtered[j] = filtered[j], filtered[i]
	}

	// Apply limit
	if len(filtered) > limit {
		filtered = filtered[:limit]
	}

	return filtered
}

// ============================================
// Pending Queries
// ============================================

// CreatePendingQuery creates a pending query and returns its ID
func (v *V4Server) CreatePendingQuery(query PendingQuery) string {
	return v.CreatePendingQueryWithTimeout(query, v.queryTimeout)
}

// CreatePendingQueryWithTimeout creates a pending query with a custom timeout
func (v *V4Server) CreatePendingQueryWithTimeout(query PendingQuery, timeout time.Duration) string {
	v.mu.Lock()
	defer v.mu.Unlock()

	// Enforce max pending queries
	if len(v.pendingQueries) >= maxPendingQueries {
		// Drop oldest
		v.pendingQueries = v.pendingQueries[1:]
	}

	v.queryIDCounter++
	id := fmt.Sprintf("q-%d", v.queryIDCounter)

	entry := pendingQueryEntry{
		query: PendingQueryResponse{
			ID:     id,
			Type:   query.Type,
			Params: query.Params,
		},
		expires: time.Now().Add(timeout),
	}

	v.pendingQueries = append(v.pendingQueries, entry)

	// Schedule cleanup
	go func() {
		time.Sleep(timeout)
		v.mu.Lock()
		defer v.mu.Unlock()
		v.cleanExpiredQueries()
		v.queryCond.Broadcast()
	}()

	return id
}

// cleanExpiredQueries removes expired pending queries (must hold lock)
func (v *V4Server) cleanExpiredQueries() {
	now := time.Now()
	remaining := v.pendingQueries[:0]
	for _, pq := range v.pendingQueries {
		if pq.expires.After(now) {
			remaining = append(remaining, pq)
		}
	}
	v.pendingQueries = remaining
}

// GetPendingQueries returns all pending queries
func (v *V4Server) GetPendingQueries() []PendingQueryResponse {
	v.mu.Lock()
	defer v.mu.Unlock()

	v.cleanExpiredQueries()

	result := make([]PendingQueryResponse, 0, len(v.pendingQueries))
	for _, pq := range v.pendingQueries {
		result = append(result, pq.query)
	}
	return result
}

// SetQueryResult stores the result for a pending query
func (v *V4Server) SetQueryResult(id string, result json.RawMessage) {
	v.mu.Lock()
	defer v.mu.Unlock()

	v.queryResults[id] = result

	// Remove from pending
	remaining := v.pendingQueries[:0]
	for _, pq := range v.pendingQueries {
		if pq.query.ID != id {
			remaining = append(remaining, pq)
		}
	}
	v.pendingQueries = remaining

	// Wake up waiters
	v.queryCond.Broadcast()
}

// GetQueryResult retrieves the result for a query
func (v *V4Server) GetQueryResult(id string) (json.RawMessage, bool) {
	v.mu.RLock()
	defer v.mu.RUnlock()

	result, found := v.queryResults[id]
	return result, found
}

// WaitForResult blocks until a result is available or timeout
func (v *V4Server) WaitForResult(id string, timeout time.Duration) (json.RawMessage, error) {
	deadline := time.Now().Add(timeout)

	v.mu.Lock()
	defer v.mu.Unlock()

	for {
		if result, found := v.queryResults[id]; found {
			return result, nil
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("timeout waiting for result %s", id)
		}
		// Wait with a short timeout to recheck
		go func() {
			time.Sleep(10 * time.Millisecond)
			v.queryCond.Broadcast()
		}()
		v.queryCond.Wait()
	}
}

// SetQueryTimeout sets the default timeout for on-demand queries
func (v *V4Server) SetQueryTimeout(timeout time.Duration) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.queryTimeout = timeout
}

// ============================================
// Rate Limiting & Memory
// ============================================

// RecordEventReceived records an event for rate limiting
func (v *V4Server) RecordEventReceived() {
	v.mu.Lock()
	defer v.mu.Unlock()

	now := time.Now()
	if now.Sub(v.rateResetTime) > time.Second {
		v.eventCount = 0
		v.rateResetTime = now
	}
	v.eventCount++
}

// isRateLimited checks if the server is rate limited (must hold lock)
func (v *V4Server) isRateLimited() bool {
	now := time.Now()
	if now.Sub(v.rateResetTime) > time.Second {
		return false
	}
	return v.eventCount > rateLimitThreshold
}

// SetMemoryUsage sets simulated memory usage for testing
func (v *V4Server) SetMemoryUsage(bytes int64) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.simulatedMemory = bytes
}

// isMemoryExceeded checks if memory is over the hard limit
func (v *V4Server) isMemoryExceeded() bool {
	if v.simulatedMemory > 0 {
		return v.simulatedMemory > memoryHardLimit
	}
	return false
}

// GetWebSocketBufferMemory returns approximate memory usage of WS buffer
func (v *V4Server) GetWebSocketBufferMemory() int64 {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.calcWSMemory()
}

// GetNetworkBodiesBufferMemory returns approximate memory usage of network bodies buffer
func (v *V4Server) GetNetworkBodiesBufferMemory() int64 {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.calcNBMemory()
}

// ============================================
// HTTP Handlers
// ============================================

// HandleWebSocketEvents handles POST /websocket-events
func (v *V4Server) HandleWebSocketEvents(w http.ResponseWriter, r *http.Request) {
	v.mu.RLock()
	rateLimited := v.isRateLimited()
	v.mu.RUnlock()

	if rateLimited {
		w.WriteHeader(http.StatusTooManyRequests)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	var payload struct {
		Events []WebSocketEvent `json:"events"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	v.AddWebSocketEvents(payload.Events)
	w.WriteHeader(http.StatusOK)
}

// HandleNetworkBodies handles POST /network-bodies
func (v *V4Server) HandleNetworkBodies(w http.ResponseWriter, r *http.Request) {
	v.mu.RLock()
	memExceeded := v.isMemoryExceeded()
	v.mu.RUnlock()

	if memExceeded {
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	var payload struct {
		Bodies []NetworkBody `json:"bodies"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	v.AddNetworkBodies(payload.Bodies)
	w.WriteHeader(http.StatusOK)
}

// HandlePendingQueries handles GET /pending-queries
func (v *V4Server) HandlePendingQueries(w http.ResponseWriter, r *http.Request) {
	queries := v.GetPendingQueries()
	if queries == nil {
		queries = make([]PendingQueryResponse, 0)
	}

	resp := map[string]interface{}{
		"queries": queries,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}

// HandleDOMResult handles POST /dom-result
func (v *V4Server) HandleDOMResult(w http.ResponseWriter, r *http.Request) {
	v.handleQueryResult(w, r)
}

// HandleA11yResult handles POST /a11y-result
func (v *V4Server) HandleA11yResult(w http.ResponseWriter, r *http.Request) {
	v.handleQueryResult(w, r)
}

// handleQueryResult handles a query result POST (shared between DOM and A11y)
func (v *V4Server) handleQueryResult(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	var payload struct {
		ID     string          `json:"id"`
		Result json.RawMessage `json:"result"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// Check if this query ID exists (in pending or already answered)
	v.mu.RLock()
	found := false
	for _, pq := range v.pendingQueries {
		if pq.query.ID == payload.ID {
			found = true
			break
		}
	}
	if _, exists := v.queryResults[payload.ID]; exists {
		found = true
	}
	v.mu.RUnlock()

	if !found {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	v.SetQueryResult(payload.ID, payload.Result)
	w.WriteHeader(http.StatusOK)
}

// ============================================
// MCP Handler V4
// ============================================

// MCPHandlerV4 extends MCPHandler with v4 tools
type MCPHandlerV4 struct {
	*MCPHandler
	v4 *V4Server
}

// NewMCPHandlerV4 creates an MCP handler with v4 capabilities
func NewMCPHandlerV4(server *Server, v4 *V4Server) *MCPHandler {
	handler := &MCPHandlerV4{
		MCPHandler: NewMCPHandler(server),
		v4:         v4,
	}
	// Return as MCPHandler but with overridden methods via the wrapper
	return &MCPHandler{
		server:      server,
		initialized: false,
		v4Handler:   handler,
	}
}

// v4ToolsList returns the list of v4 tools
func (h *MCPHandlerV4) v4ToolsList() []MCPTool {
	return []MCPTool{
		{
			Name:        "get_websocket_events",
			Description: "Get captured WebSocket events (messages, lifecycle). Useful for debugging real-time communication.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"connection_id": map[string]interface{}{
						"type":        "string",
						"description": "Filter by connection ID",
					},
					"url": map[string]interface{}{
						"type":        "string",
						"description": "Filter by URL substring",
					},
					"direction": map[string]interface{}{
						"type":        "string",
						"description": "Filter by direction (incoming/outgoing)",
						"enum":        []string{"incoming", "outgoing"},
					},
					"limit": map[string]interface{}{
						"type":        "number",
						"description": "Maximum events to return (default: 50)",
					},
				},
			},
		},
		{
			Name:        "get_websocket_status",
			Description: "Get current WebSocket connection states, rates, and schemas.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"url": map[string]interface{}{
						"type":        "string",
						"description": "Filter by URL substring",
					},
					"connection_id": map[string]interface{}{
						"type":        "string",
						"description": "Filter by connection ID",
					},
				},
			},
		},
		{
			Name:        "get_network_bodies",
			Description: "Get captured network request/response bodies.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"url": map[string]interface{}{
						"type":        "string",
						"description": "Filter by URL substring",
					},
					"method": map[string]interface{}{
						"type":        "string",
						"description": "Filter by HTTP method",
					},
					"status_min": map[string]interface{}{
						"type":        "number",
						"description": "Minimum status code",
					},
					"status_max": map[string]interface{}{
						"type":        "number",
						"description": "Maximum status code",
					},
					"limit": map[string]interface{}{
						"type":        "number",
						"description": "Maximum entries to return (default: 20)",
					},
				},
			},
		},
		{
			Name:        "query_dom",
			Description: "Query the live DOM in the browser using a CSS selector. Returns matching elements.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"selector": map[string]interface{}{
						"type":        "string",
						"description": "CSS selector to query",
					},
				},
				"required": []string{"selector"},
			},
		},
		{
			Name:        "get_page_info",
			Description: "Get information about the current page (URL, title, viewport).",
			InputSchema: map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
		{
			Name:        "run_accessibility_audit",
			Description: "Run an accessibility audit on the current page or a scoped element.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"scope": map[string]interface{}{
						"type":        "string",
						"description": "CSS selector to scope the audit",
					},
					"tags": map[string]interface{}{
						"type":        "array",
						"description": "WCAG tags to test (e.g., wcag2a, wcag2aa)",
						"items":       map[string]interface{}{"type": "string"},
					},
				},
			},
		},
	}
}

// handleV4ToolCall handles a v4-specific tool call
func (h *MCPHandlerV4) handleV4ToolCall(req JSONRPCRequest, name string, args json.RawMessage) (JSONRPCResponse, bool) {
	switch name {
	case "get_websocket_events":
		return h.toolGetWSEvents(req, args), true
	case "get_websocket_status":
		return h.toolGetWSStatus(req, args), true
	case "get_network_bodies":
		return h.toolGetNetworkBodies(req, args), true
	case "query_dom":
		return h.toolQueryDOM(req, args), true
	case "get_page_info":
		return h.toolGetPageInfo(req, args), true
	case "run_accessibility_audit":
		return h.toolRunA11yAudit(req, args), true
	}
	return JSONRPCResponse{}, false
}

func (h *MCPHandlerV4) toolGetWSEvents(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var arguments struct {
		ConnectionID string `json:"connection_id"`
		URL          string `json:"url"`
		Direction    string `json:"direction"`
		Limit        int    `json:"limit"`
	}
	json.Unmarshal(args, &arguments)

	events := h.v4.GetWebSocketEvents(WebSocketEventFilter{
		ConnectionID: arguments.ConnectionID,
		URLFilter:    arguments.URL,
		Direction:    arguments.Direction,
		Limit:        arguments.Limit,
	})

	var contentText string
	if len(events) == 0 {
		contentText = "No WebSocket events captured"
	} else {
		eventsJSON, _ := json.Marshal(events)
		contentText = string(eventsJSON)
	}

	result := map[string]interface{}{
		"content": []map[string]string{
			{"type": "text", "text": contentText},
		},
	}
	resultJSON, _ := json.Marshal(result)
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: resultJSON}
}

func (h *MCPHandlerV4) toolGetWSStatus(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var arguments struct {
		URL          string `json:"url"`
		ConnectionID string `json:"connection_id"`
	}
	json.Unmarshal(args, &arguments)

	status := h.v4.GetWebSocketStatus(WebSocketStatusFilter{
		URLFilter:    arguments.URL,
		ConnectionID: arguments.ConnectionID,
	})

	statusJSON, _ := json.Marshal(status)
	result := map[string]interface{}{
		"content": []map[string]string{
			{"type": "text", "text": string(statusJSON)},
		},
	}
	resultJSON, _ := json.Marshal(result)
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: resultJSON}
}

func (h *MCPHandlerV4) toolGetNetworkBodies(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var arguments struct {
		URL       string `json:"url"`
		Method    string `json:"method"`
		StatusMin int    `json:"status_min"`
		StatusMax int    `json:"status_max"`
		Limit     int    `json:"limit"`
	}
	json.Unmarshal(args, &arguments)

	bodies := h.v4.GetNetworkBodies(NetworkBodyFilter{
		URLFilter: arguments.URL,
		Method:    arguments.Method,
		StatusMin: arguments.StatusMin,
		StatusMax: arguments.StatusMax,
		Limit:     arguments.Limit,
	})

	var contentText string
	if len(bodies) == 0 {
		contentText = "No network bodies captured"
	} else {
		bodiesJSON, _ := json.Marshal(bodies)
		contentText = string(bodiesJSON)
	}

	result := map[string]interface{}{
		"content": []map[string]string{
			{"type": "text", "text": contentText},
		},
	}
	resultJSON, _ := json.Marshal(result)
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: resultJSON}
}

func (h *MCPHandlerV4) toolQueryDOM(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var arguments struct {
		Selector string `json:"selector"`
	}
	json.Unmarshal(args, &arguments)

	params, _ := json.Marshal(map[string]string{"selector": arguments.Selector})
	id := h.v4.CreatePendingQuery(PendingQuery{
		Type:   "dom",
		Params: params,
	})

	result, err := h.v4.WaitForResult(id, h.v4.queryTimeout)
	if err != nil {
		errResult := map[string]interface{}{
			"content": []map[string]string{
				{"type": "text", "text": "Timeout waiting for DOM query result"},
			},
			"isError": true,
		}
		resultJSON, _ := json.Marshal(errResult)
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: resultJSON}
	}

	resp := map[string]interface{}{
		"content": []map[string]string{
			{"type": "text", "text": string(result)},
		},
	}
	resultJSON, _ := json.Marshal(resp)
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: resultJSON}
}

func (h *MCPHandlerV4) toolGetPageInfo(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	id := h.v4.CreatePendingQuery(PendingQuery{
		Type:   "page_info",
		Params: json.RawMessage(`{}`),
	})

	result, err := h.v4.WaitForResult(id, h.v4.queryTimeout)
	if err != nil {
		errResult := map[string]interface{}{
			"content": []map[string]string{
				{"type": "text", "text": "Timeout waiting for page info"},
			},
			"isError": true,
		}
		resultJSON, _ := json.Marshal(errResult)
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: resultJSON}
	}

	resp := map[string]interface{}{
		"content": []map[string]string{
			{"type": "text", "text": string(result)},
		},
	}
	resultJSON, _ := json.Marshal(resp)
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: resultJSON}
}

func (h *MCPHandlerV4) toolRunA11yAudit(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var arguments struct {
		Scope string   `json:"scope"`
		Tags  []string `json:"tags"`
	}
	json.Unmarshal(args, &arguments)

	params := map[string]interface{}{}
	if arguments.Scope != "" {
		params["scope"] = arguments.Scope
	}
	if arguments.Tags != nil {
		params["tags"] = arguments.Tags
	}
	paramsJSON, _ := json.Marshal(params)

	id := h.v4.CreatePendingQuery(PendingQuery{
		Type:   "a11y",
		Params: paramsJSON,
	})

	result, err := h.v4.WaitForResult(id, h.v4.queryTimeout)
	if err != nil {
		errResult := map[string]interface{}{
			"content": []map[string]string{
				{"type": "text", "text": "Timeout waiting for accessibility audit result"},
			},
			"isError": true,
		}
		resultJSON, _ := json.Marshal(errResult)
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: resultJSON}
	}

	resp := map[string]interface{}{
		"content": []map[string]string{
			{"type": "text", "text": string(result)},
		},
	}
	resultJSON, _ := json.Marshal(resp)
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: resultJSON}
}
