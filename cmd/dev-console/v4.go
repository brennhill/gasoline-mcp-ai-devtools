package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
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

// EnhancedAction represents a captured user action with multi-strategy selectors
type EnhancedAction struct {
	Type          string                 `json:"type"`
	Timestamp     int64                  `json:"timestamp"`
	URL           string                 `json:"url,omitempty"`
	Selectors     map[string]interface{} `json:"selectors,omitempty"`
	Value         string                 `json:"value,omitempty"`
	InputType     string                 `json:"inputType,omitempty"`
	Key           string                 `json:"key,omitempty"`
	FromURL       string                 `json:"fromUrl,omitempty"`
	ToURL         string                 `json:"toUrl,omitempty"`
	SelectedValue string                 `json:"selectedValue,omitempty"`
	SelectedText  string                 `json:"selectedText,omitempty"`
	ScrollY       int                    `json:"scrollY,omitempty"`
}

// EnhancedActionFilter defines filtering criteria for enhanced actions
type EnhancedActionFilter struct {
	LastN     int
	URLFilter string
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
	total       int
	bytes       int
	lastAt      string
	lastData    string
	recentTimes []time.Time // timestamps within rate window for rate calculation
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
	maxEnhancedActions  = 50
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
	rateWindow          = 5 * time.Second // rolling window for msg/s calculation
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

	// Enhanced actions ring buffer (v5)
	enhancedActions []EnhancedAction

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

	// A11y audit cache
	a11yCache      map[string]*a11yCacheEntry
	a11yCacheOrder []string // Track insertion order for eviction
	lastKnownURL   string
	a11yInflight   map[string]*a11yInflightEntry
}

const maxA11yCacheEntries = 10
const a11yCacheTTL = 30 * time.Second

type a11yCacheEntry struct {
	result    json.RawMessage
	createdAt time.Time
	url       string
}

type a11yInflightEntry struct {
	done   chan struct{}
	result json.RawMessage
	err    error
}

// NewV4Server creates a new v4 server instance
func NewV4Server() *V4Server {
	v4 := &V4Server{
		wsEvents:        make([]WebSocketEvent, 0, maxWSEvents),
		networkBodies:   make([]NetworkBody, 0, maxNetworkBodies),
		enhancedActions: make([]EnhancedAction, 0, maxEnhancedActions),
		connections:     make(map[string]*connectionState),
		closedConns:    make([]WebSocketClosedConnection, 0),
		connOrder:      make([]string, 0),
		pendingQueries: make([]pendingQueryEntry, 0),
		queryResults:   make(map[string]json.RawMessage),
		rateResetTime:  time.Now(),
		queryTimeout:   defaultQueryTimeout,
		a11yCache:      make(map[string]*a11yCacheEntry),
		a11yCacheOrder: make([]string, 0),
		a11yInflight:   make(map[string]*a11yInflightEntry),
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
		msgTime := parseTimestamp(event.Timestamp)
		if event.Direction == "incoming" {
			conn.incoming.total++
			conn.incoming.bytes += event.Size
			conn.incoming.lastAt = event.Timestamp
			conn.incoming.lastData = event.Data
			conn.incoming.recentTimes = appendAndPrune(conn.incoming.recentTimes, msgTime)
		} else if event.Direction == "outgoing" {
			conn.outgoing.total++
			conn.outgoing.bytes += event.Size
			conn.outgoing.lastAt = event.Timestamp
			conn.outgoing.lastData = event.Data
			conn.outgoing.recentTimes = appendAndPrune(conn.outgoing.recentTimes, msgTime)
		}
		if event.Sampled != nil {
			conn.sampling = true
			conn.lastSample = event.Sampled
		}
	}
}

// parseTimestamp parses an RFC3339 timestamp string, returns zero time on failure
func parseTimestamp(ts string) time.Time {
	t, err := time.Parse(time.RFC3339Nano, ts)
	if err != nil {
		t, _ = time.Parse(time.RFC3339, ts)
	}
	return t
}

// appendAndPrune adds a timestamp to the slice and removes entries older than rateWindow
func appendAndPrune(times []time.Time, t time.Time) []time.Time {
	cutoff := time.Now().Add(-rateWindow)
	// Prune old entries
	start := 0
	for start < len(times) && times[start].Before(cutoff) {
		start++
	}
	pruned := times[start:]
	if !t.IsZero() {
		pruned = append(pruned, t)
	}
	return pruned
}

// calcRate returns messages per second from recent timestamps within the rate window
func calcRate(times []time.Time) float64 {
	now := time.Now()
	cutoff := now.Add(-rateWindow)
	count := 0
	for _, t := range times {
		if t.After(cutoff) {
			count++
		}
	}
	if count == 0 {
		return 0.0
	}
	return float64(count) / rateWindow.Seconds()
}

// formatDuration formats a duration as human-readable (e.g., "5s", "2m30s", "1h15m")
func formatDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		mins := int(d.Minutes())
		secs := int(d.Seconds()) % 60
		if secs == 0 {
			return fmt.Sprintf("%dm", mins)
		}
		return fmt.Sprintf("%dm%02ds", mins, secs)
	}
	hours := int(d.Hours())
	mins := int(d.Minutes()) % 60
	if mins == 0 {
		return fmt.Sprintf("%dh", hours)
	}
	return fmt.Sprintf("%dh%02dm", hours, mins)
}

// formatAge formats the age of a timestamp relative to now (e.g., "0.2s", "3s", "2m30s")
func formatAge(ts string) string {
	t := parseTimestamp(ts)
	if t.IsZero() {
		return ""
	}
	d := time.Since(t)
	if d < 0 {
		d = 0
	}
	return formatDuration(d)
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
					PerSecond: calcRate(conn.incoming.recentTimes),
					Total:     conn.incoming.total,
					Bytes:     conn.incoming.bytes,
				},
				Outgoing: WebSocketDirectionStats{
					PerSecond: calcRate(conn.outgoing.recentTimes),
					Total:     conn.outgoing.total,
					Bytes:     conn.outgoing.bytes,
				},
			},
			Sampling: WebSocketSamplingStatus{
				Active: conn.sampling,
			},
		}

		// Calculate connection duration
		openedTime := parseTimestamp(conn.openedAt)
		if !openedTime.IsZero() {
			wc.Duration = formatDuration(time.Since(openedTime))
		}

		if conn.incoming.lastData != "" {
			wc.LastMessage.Incoming = &WebSocketMessagePreview{
				At:      conn.incoming.lastAt,
				Age:     formatAge(conn.incoming.lastAt),
				Preview: conn.incoming.lastData,
			}
		}
		if conn.outgoing.lastData != "" {
			wc.LastMessage.Outgoing = &WebSocketMessagePreview{
				At:      conn.outgoing.lastAt,
				Age:     formatAge(conn.outgoing.lastAt),
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
// Enhanced Actions (v5)
// ============================================

// AddEnhancedActions adds enhanced actions to the buffer
func (v *V4Server) AddEnhancedActions(actions []EnhancedAction) {
	v.mu.Lock()
	defer v.mu.Unlock()

	for i := range actions {
		// Redact password values on ingest
		if actions[i].InputType == "password" && actions[i].Value != "[redacted]" {
			actions[i].Value = "[redacted]"
		}
		v.enhancedActions = append(v.enhancedActions, actions[i])
	}

	// Enforce max count
	if len(v.enhancedActions) > maxEnhancedActions {
		v.enhancedActions = v.enhancedActions[len(v.enhancedActions)-maxEnhancedActions:]
	}
}

// GetEnhancedActionCount returns the current number of buffered actions
func (v *V4Server) GetEnhancedActionCount() int {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return len(v.enhancedActions)
}

// GetEnhancedActions returns filtered enhanced actions
func (v *V4Server) GetEnhancedActions(filter EnhancedActionFilter) []EnhancedAction {
	v.mu.RLock()
	defer v.mu.RUnlock()

	var filtered []EnhancedAction
	for _, a := range v.enhancedActions {
		if filter.URLFilter != "" && !strings.Contains(a.URL, filter.URLFilter) {
			continue
		}
		filtered = append(filtered, a)
	}

	// Apply lastN (return most recent N)
	if filter.LastN > 0 && len(filtered) > filter.LastN {
		filtered = filtered[len(filtered)-filter.LastN:]
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

// GetQueryResult retrieves the result for a query and deletes it from storage
func (v *V4Server) GetQueryResult(id string) (json.RawMessage, bool) {
	v.mu.Lock()
	defer v.mu.Unlock()

	result, found := v.queryResults[id]
	if found {
		delete(v.queryResults, id)
	}
	return result, found
}

// WaitForResult blocks until a result is available or timeout, then deletes it
func (v *V4Server) WaitForResult(id string, timeout time.Duration) (json.RawMessage, error) {
	deadline := time.Now().Add(timeout)

	v.mu.Lock()
	defer v.mu.Unlock()

	for {
		if result, found := v.queryResults[id]; found {
			delete(v.queryResults, id)
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

// IsMemoryExceeded checks if memory is over the hard limit (acquires lock).
// Uses simulated memory if set (for testing), otherwise checks real buffer memory.
func (v *V4Server) IsMemoryExceeded() bool {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.isMemoryExceeded()
}

// isMemoryExceeded is the internal version (caller must hold lock)
func (v *V4Server) isMemoryExceeded() bool {
	if v.simulatedMemory > 0 {
		return v.simulatedMemory > memoryHardLimit
	}
	return v.calcTotalMemory() > memoryHardLimit
}

// GetTotalBufferMemory returns the sum of all buffer memory usage
func (v *V4Server) GetTotalBufferMemory() int64 {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.calcTotalMemory()
}

// calcTotalMemory returns total memory across all buffers (caller must hold lock)
func (v *V4Server) calcTotalMemory() int64 {
	return v.calcWSMemory() + v.calcNBMemory()
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

// HandleWebSocketEvents handles GET and POST /websocket-events
func (v *V4Server) HandleWebSocketEvents(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		events := v.GetWebSocketEvents(WebSocketEventFilter{})
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"events": events,
			"count":  len(events),
		})
		return
	}

	v.mu.RLock()
	rateLimited := v.isRateLimited()
	memExceeded := v.isMemoryExceeded()
	v.mu.RUnlock()

	if rateLimited {
		w.WriteHeader(http.StatusTooManyRequests)
		return
	}

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
		Events []WebSocketEvent `json:"events"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	v.AddWebSocketEvents(payload.Events)
	w.WriteHeader(http.StatusOK)
}

// HandleWebSocketStatus handles GET /websocket-status
func (v *V4Server) HandleWebSocketStatus(w http.ResponseWriter, r *http.Request) {
	status := v.GetWebSocketStatus(WebSocketStatusFilter{})
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
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

// HandleEnhancedActions handles POST /enhanced-actions
func (v *V4Server) HandleEnhancedActions(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	var payload struct {
		Actions []EnhancedAction `json:"actions"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	v.AddEnhancedActions(payload.Actions)
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
					"force_refresh": map[string]interface{}{
						"type":        "boolean",
						"description": "Bypass cache and re-run the audit",
					},
				},
			},
		},
		{
			Name:        "get_enhanced_actions",
			Description: "Get captured user actions with multi-strategy selectors. Useful for understanding what the user did before an error occurred.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"last_n": map[string]interface{}{
						"type":        "number",
						"description": "Return only the last N actions (default: all)",
					},
					"url": map[string]interface{}{
						"type":        "string",
						"description": "Filter by URL substring",
					},
				},
			},
		},
		{
			Name:        "get_reproduction_script",
			Description: "Generate a Playwright test script from captured user actions. Useful for reproducing bugs.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"error_message": map[string]interface{}{
						"type":        "string",
						"description": "Error message to include in the test (adds context comment)",
					},
					"last_n_actions": map[string]interface{}{
						"type":        "number",
						"description": "Use only the last N actions (default: all)",
					},
					"base_url": map[string]interface{}{
						"type":        "string",
						"description": "Replace the origin in URLs (e.g., 'https://staging.example.com')",
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
	case "get_enhanced_actions":
		return h.toolGetEnhancedActions(req, args), true
	case "get_reproduction_script":
		return h.toolGetReproductionScript(req, args), true
	case "get_session_timeline":
		return h.toolGetSessionTimeline(req, args), true
	case "generate_test":
		return h.toolGenerateTest(req, args), true
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
		Scope        string   `json:"scope"`
		Tags         []string `json:"tags"`
		ForceRefresh bool     `json:"force_refresh"`
	}
	json.Unmarshal(args, &arguments)

	cacheKey := h.v4.a11yCacheKey(arguments.Scope, arguments.Tags)

	// Check cache (unless force_refresh)
	if !arguments.ForceRefresh {
		if cached := h.v4.getA11yCacheEntry(cacheKey); cached != nil {
			resp := map[string]interface{}{
				"content": []map[string]string{
					{"type": "text", "text": string(cached)},
				},
			}
			resultJSON, _ := json.Marshal(resp)
			return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: resultJSON}
		}

		// Check if there's an inflight request for this key (concurrent dedup)
		if inflight := h.v4.getOrCreateInflight(cacheKey); inflight != nil {
			// Wait for the inflight request to complete
			<-inflight.done
			if inflight.err != nil {
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
					{"type": "text", "text": string(inflight.result)},
				},
			}
			resultJSON, _ := json.Marshal(resp)
			return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: resultJSON}
		}
	} else {
		// force_refresh: remove existing cache entry
		h.v4.removeA11yCacheEntry(cacheKey)
		// Register inflight
		h.v4.getOrCreateInflight(cacheKey)
	}

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
		// Don't cache errors — complete inflight with error
		h.v4.completeInflight(cacheKey, nil, err)
		errResult := map[string]interface{}{
			"content": []map[string]string{
				{"type": "text", "text": "Timeout waiting for accessibility audit result"},
			},
			"isError": true,
		}
		resultJSON, _ := json.Marshal(errResult)
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: resultJSON}
	}

	// Cache the successful result
	h.v4.setA11yCacheEntry(cacheKey, result)
	h.v4.completeInflight(cacheKey, result, nil)

	resp := map[string]interface{}{
		"content": []map[string]string{
			{"type": "text", "text": string(result)},
		},
	}
	resultJSON, _ := json.Marshal(resp)
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: resultJSON}
}

// ============================================
// v5 MCP Tool Implementations
// ============================================

func (h *MCPHandlerV4) toolGetEnhancedActions(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var arguments struct {
		LastN int    `json:"last_n"`
		URL   string `json:"url"`
	}
	json.Unmarshal(args, &arguments)

	actions := h.v4.GetEnhancedActions(EnhancedActionFilter{
		LastN:     arguments.LastN,
		URLFilter: arguments.URL,
	})

	var contentText string
	if len(actions) == 0 {
		contentText = "No enhanced actions captured"
	} else {
		actionsJSON, _ := json.Marshal(actions)
		contentText = string(actionsJSON)
	}

	result := map[string]interface{}{
		"content": []map[string]string{
			{"type": "text", "text": contentText},
		},
	}
	resultJSON, _ := json.Marshal(result)
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: resultJSON}
}

func (h *MCPHandlerV4) toolGetReproductionScript(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var arguments struct {
		ErrorMessage string `json:"error_message"`
		LastNActions int    `json:"last_n_actions"`
		BaseURL      string `json:"base_url"`
	}
	json.Unmarshal(args, &arguments)

	actions := h.v4.GetEnhancedActions(EnhancedActionFilter{})

	if len(actions) == 0 {
		result := map[string]interface{}{
			"content": []map[string]string{
				{"type": "text", "text": "No enhanced actions captured to generate script"},
			},
		}
		resultJSON, _ := json.Marshal(result)
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: resultJSON}
	}

	// Apply lastNActions filter
	if arguments.LastNActions > 0 && len(actions) > arguments.LastNActions {
		actions = actions[len(actions)-arguments.LastNActions:]
	}

	script := generatePlaywrightScript(actions, arguments.ErrorMessage, arguments.BaseURL)

	result := map[string]interface{}{
		"content": []map[string]string{
			{"type": "text", "text": script},
		},
	}
	resultJSON, _ := json.Marshal(result)
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: resultJSON}
}

// ============================================
// Playwright Script Generation (v5)
// ============================================

// generatePlaywrightScript generates a Playwright test script from enhanced actions
func generatePlaywrightScript(actions []EnhancedAction, errorMessage, baseURL string) string {
	// Determine start URL
	startURL := ""
	if len(actions) > 0 && actions[0].URL != "" {
		startURL = actions[0].URL
	}
	if baseURL != "" && startURL != "" {
		startURL = replaceOrigin(startURL, baseURL)
	}

	// Build test name
	testName := "reproduction: captured user actions"
	if errorMessage != "" {
		name := errorMessage
		if len(name) > 80 {
			name = name[:80]
		}
		testName = "reproduction: " + name
	}

	// Generate steps
	var steps []string
	var prevTimestamp int64

	for _, action := range actions {
		// Add pause comment for gaps > 2 seconds
		if prevTimestamp > 0 && action.Timestamp-prevTimestamp > 2000 {
			gap := (action.Timestamp - prevTimestamp) / 1000
			steps = append(steps, fmt.Sprintf("  // [%ds pause]", gap))
		}
		prevTimestamp = action.Timestamp

		locator := getPlaywrightLocator(action.Selectors)

		switch action.Type {
		case "click":
			if locator != "" {
				steps = append(steps, fmt.Sprintf("  await page.%s.click();", locator))
			} else {
				steps = append(steps, "  // click action - no selector available")
			}
		case "input":
			value := action.Value
			if value == "[redacted]" {
				value = "[user-provided]"
			}
			if locator != "" {
				steps = append(steps, fmt.Sprintf("  await page.%s.fill('%s');", locator, escapeJSString(value)))
			}
		case "keypress":
			steps = append(steps, fmt.Sprintf("  await page.keyboard.press('%s');", escapeJSString(action.Key)))
		case "navigate":
			toURL := action.ToURL
			if baseURL != "" && toURL != "" {
				toURL = replaceOrigin(toURL, baseURL)
			}
			steps = append(steps, fmt.Sprintf("  await page.waitForURL('%s');", escapeJSString(toURL)))
		case "select":
			if locator != "" {
				steps = append(steps, fmt.Sprintf("  await page.%s.selectOption('%s');", locator, escapeJSString(action.SelectedValue)))
			}
		case "scroll":
			steps = append(steps, fmt.Sprintf("  // User scrolled to y=%d", action.ScrollY))
		}
	}

	// Assemble script
	script := "import { test, expect } from '@playwright/test';\n\n"
	script += fmt.Sprintf("test('%s', async ({ page }) => {\n", escapeJSString(testName))
	if startURL != "" {
		script += fmt.Sprintf("  await page.goto('%s');\n\n", escapeJSString(startURL))
	}
	script += strings.Join(steps, "\n")
	if len(steps) > 0 {
		script += "\n"
	}
	if errorMessage != "" {
		script += fmt.Sprintf("\n  // Error occurred here: %s\n", errorMessage)
	}
	script += "});\n"

	// Cap output size (50KB)
	if len(script) > 51200 {
		script = script[:51200]
	}

	return script
}

// getPlaywrightLocator returns the best Playwright locator for a set of selectors
// Priority: testId > role > ariaLabel > text > id > cssPath
func getPlaywrightLocator(selectors map[string]interface{}) string {
	if selectors == nil {
		return ""
	}

	if testId, ok := selectors["testId"].(string); ok && testId != "" {
		return fmt.Sprintf("getByTestId('%s')", escapeJSString(testId))
	}

	if roleData, ok := selectors["role"]; ok {
		if roleMap, ok := roleData.(map[string]interface{}); ok {
			role, _ := roleMap["role"].(string)
			name, _ := roleMap["name"].(string)
			if role != "" && name != "" {
				return fmt.Sprintf("getByRole('%s', { name: '%s' })", escapeJSString(role), escapeJSString(name))
			}
			if role != "" {
				return fmt.Sprintf("getByRole('%s')", escapeJSString(role))
			}
		}
	}

	if ariaLabel, ok := selectors["ariaLabel"].(string); ok && ariaLabel != "" {
		return fmt.Sprintf("getByLabel('%s')", escapeJSString(ariaLabel))
	}

	if text, ok := selectors["text"].(string); ok && text != "" {
		return fmt.Sprintf("getByText('%s')", escapeJSString(text))
	}

	if id, ok := selectors["id"].(string); ok && id != "" {
		return fmt.Sprintf("locator('#%s')", escapeJSString(id))
	}

	if cssPath, ok := selectors["cssPath"].(string); ok && cssPath != "" {
		return fmt.Sprintf("locator('%s')", escapeJSString(cssPath))
	}

	return ""
}

// escapeJSString escapes a string for use in JavaScript single-quoted strings
func escapeJSString(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "'", "\\'")
	s = strings.ReplaceAll(s, "\n", "\\n")
	s = strings.ReplaceAll(s, "\r", "\\r")
	return s
}

// replaceOrigin replaces the origin (scheme+host) in a URL with a new base URL
func replaceOrigin(original, baseURL string) string {
	// Find the path start (after scheme://host)
	schemeEnd := strings.Index(original, "://")
	if schemeEnd == -1 {
		return baseURL + original
	}
	rest := original[schemeEnd+3:]
	pathStart := strings.Index(rest, "/")
	if pathStart == -1 {
		return baseURL
	}
	path := rest[pathStart:]
	// Remove trailing slash from baseURL if path starts with /
	base := strings.TrimRight(baseURL, "/")
	return base + path
}

// ============================================
// v5 Test Generation (stubs — TDD RED for separate feature)
// ============================================

// TimelineFilter controls what gets included in the session timeline
type TimelineFilter struct {
	LastNActions int
	URLFilter    string
	Include      []string
}

// TimelineEntry represents a single event in the session timeline
type TimelineEntry struct {
	Timestamp     int64                  `json:"timestamp"`
	Kind          string                 `json:"kind"`
	Type          string                 `json:"type,omitempty"`
	URL           string                 `json:"url,omitempty"`
	Selectors     map[string]interface{} `json:"selectors,omitempty"`
	Method        string                 `json:"method,omitempty"`
	Status        int                    `json:"status,omitempty"`
	ContentType   string                 `json:"contentType,omitempty"`
	ResponseShape interface{}            `json:"responseShape,omitempty"`
	Level         string                 `json:"level,omitempty"`
	Message       string                 `json:"message,omitempty"`
	ToURL         string                 `json:"toUrl,omitempty"`
	Value         string                 `json:"value,omitempty"`
}

// TimelineSummary provides aggregate counts for a session timeline
type TimelineSummary struct {
	Actions         int   `json:"actions"`
	NetworkRequests int   `json:"networkRequests"`
	ConsoleErrors   int   `json:"consoleErrors"`
	DurationMs      int64 `json:"durationMs"`
}

// TimelineResponse is returned by GetSessionTimeline
type TimelineResponse struct {
	Timeline []TimelineEntry `json:"timeline"`
	Summary  TimelineSummary `json:"summary"`
}

// SessionTimelineResponse is the MCP response format for session timeline
type SessionTimelineResponse struct {
	Timeline []TimelineEntry `json:"timeline"`
	Summary  TimelineSummary `json:"summary"`
}

// TestGenerationOptions controls test script generation
type TestGenerationOptions struct {
	TestName            string `json:"test_name"`
	AssertNetwork       bool   `json:"assert_network"`
	AssertNoErrors      bool   `json:"assert_no_errors"`
	AssertResponseShape bool   `json:"assert_response_shape"`
	BaseURL             string `json:"base_url"`
}

// extractResponseShape extracts the structural type signature from a JSON response body.
// TODO: Implementation pending (tests exist in TDD RED state)
func extractResponseShape(body string) interface{} {
	var raw interface{}
	if err := json.Unmarshal([]byte(body), &raw); err != nil {
		return nil
	}
	return shapeOf(raw, 0, 3)
}

func shapeOf(v interface{}, depth, maxDepth int) interface{} {
	if depth > maxDepth {
		return "..."
	}
	switch val := v.(type) {
	case map[string]interface{}:
		result := make(map[string]interface{})
		for k, child := range val {
			result[k] = shapeOf(child, depth+1, maxDepth)
		}
		return result
	case []interface{}:
		if len(val) == 0 {
			return []interface{}{}
		}
		// Return only the first element as a representative sample
		return []interface{}{shapeOf(val[0], depth+1, maxDepth)}
	case string:
		return "string"
	case float64:
		return "number"
	case bool:
		return "boolean"
	case nil:
		return "null"
	default:
		return "unknown"
	}
}

// normalizeTimestamp converts various timestamp formats to Unix milliseconds.
func normalizeTimestamp(s string) int64 {
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		return 0
	}
	return t.UnixMilli()
}

// GetSessionTimeline returns a merged, sorted timeline of all session events.
func (v *V4Server) GetSessionTimeline(filter TimelineFilter, entries []LogEntry) TimelineResponse {
	var timeline []TimelineEntry
	var summary TimelineSummary

	// Add enhanced actions
	v.mu.RLock()
	actions := make([]EnhancedAction, len(v.enhancedActions))
	copy(actions, v.enhancedActions)
	networkBodies := make([]NetworkBody, len(v.networkBodies))
	copy(networkBodies, v.networkBodies)
	v.mu.RUnlock()

	for _, a := range actions {
		entry := TimelineEntry{
			Kind:      "action",
			Timestamp: a.Timestamp,
			Type:      a.Type,
			URL:       a.URL,
		}
		timeline = append(timeline, entry)
		summary.Actions++
	}

	// Add network bodies
	for _, nb := range networkBodies {
		ts := normalizeTimestamp(nb.Timestamp)
		entry := TimelineEntry{
			Kind:      "network",
			Timestamp: ts,
			URL:       nb.URL,
			Method:    nb.Method,
			Status:    nb.Status,
		}
		if strings.Contains(nb.ContentType, "json") && nb.ResponseBody != "" {
			entry.ResponseShape = extractResponseShape(nb.ResponseBody)
		}
		timeline = append(timeline, entry)
		summary.NetworkRequests++
	}

	// Add console entries (errors and warnings only)
	for _, e := range entries {
		level, _ := e["level"].(string)
		if level != "error" && level != "warn" {
			continue
		}
		ts, _ := e["ts"].(string)
		msg, _ := e["message"].(string)
		entry := TimelineEntry{
			Kind:      "console",
			Timestamp: normalizeTimestamp(ts),
			Level:     level,
			Message:   msg,
		}
		timeline = append(timeline, entry)
		if level == "error" {
			summary.ConsoleErrors++
		}
	}

	// Sort by timestamp
	sort.Slice(timeline, func(i, j int) bool {
		return timeline[i].Timestamp < timeline[j].Timestamp
	})

	// Apply LastNActions filter
	if filter.LastNActions > 0 {
		actionCount := 0
		startIdx := len(timeline)
		for i := len(timeline) - 1; i >= 0; i-- {
			if timeline[i].Kind == "action" {
				actionCount++
				if actionCount >= filter.LastNActions {
					startIdx = i
					break
				}
			}
		}
		if startIdx < len(timeline) {
			startTs := timeline[startIdx].Timestamp
			var filtered []TimelineEntry
			for _, e := range timeline {
				if e.Timestamp >= startTs {
					filtered = append(filtered, e)
				}
			}
			timeline = filtered
			// Recount summary
			summary = TimelineSummary{}
			for _, e := range timeline {
				switch e.Kind {
				case "action":
					summary.Actions++
				case "network":
					summary.NetworkRequests++
				case "console":
					if e.Level == "error" {
						summary.ConsoleErrors++
					}
				}
			}
		}
	}

	// Apply URL filter
	if filter.URLFilter != "" {
		var filtered []TimelineEntry
		for _, e := range timeline {
			if strings.Contains(e.URL, filter.URLFilter) {
				filtered = append(filtered, e)
			}
		}
		timeline = filtered
	}

	return TimelineResponse{
		Timeline: timeline,
		Summary:  summary,
	}
}

// generateTestScript generates a Playwright test script from a timeline.
// TODO: Implementation pending (tests exist in TDD RED state)
func generateTestScript(timeline []TimelineEntry, opts TestGenerationOptions) string {
	return ""
}

func (h *MCPHandlerV4) toolGetSessionTimeline(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var arguments struct {
		LastNActions int    `json:"last_n_actions"`
		URLFilter   string `json:"url_filter"`
		Include     string `json:"include"`
	}
	json.Unmarshal(args, &arguments)

	filter := TimelineFilter{
		LastNActions: arguments.LastNActions,
		URLFilter:   arguments.URLFilter,
	}
	resp := h.v4.GetSessionTimeline(filter, h.server.entries)
	contentBytes, _ := json.Marshal(SessionTimelineResponse{
		Timeline: resp.Timeline,
		Summary:  resp.Summary,
	})
	result := map[string]interface{}{
		"content": []map[string]string{
			{"type": "text", "text": string(contentBytes)},
		},
	}
	resultJSON, _ := json.Marshal(result)
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: resultJSON}
}

func (h *MCPHandlerV4) toolGenerateTest(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var arguments struct {
		LastNActions   int    `json:"last_n_actions"`
		BaseURL        string `json:"base_url"`
		TestName       string `json:"test_name"`
		AssertNetwork  bool   `json:"assert_network"`
		ResponseShapes bool   `json:"response_shapes"`
	}
	json.Unmarshal(args, &arguments)

	filter := TimelineFilter{LastNActions: arguments.LastNActions}
	resp := h.v4.GetSessionTimeline(filter, h.server.entries)
	script := generateTestScript(resp.Timeline, TestGenerationOptions{
		BaseURL:             arguments.BaseURL,
		TestName:            arguments.TestName,
		AssertNetwork:       arguments.AssertNetwork,
		AssertResponseShape: arguments.ResponseShapes,
	})
	result := map[string]interface{}{
		"content": []map[string]string{
			{"type": "text", "text": script},
		},
	}
	resultJSON, _ := json.Marshal(result)
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: resultJSON}
}

// ============================================
// A11y Audit Cache
// ============================================

// a11yCacheKey generates a cache key from scope and tags (tags sorted for normalization)
func (v *V4Server) a11yCacheKey(scope string, tags []string) string {
	sortedTags := make([]string, len(tags))
	copy(sortedTags, tags)
	sort.Strings(sortedTags)
	return scope + "|" + strings.Join(sortedTags, ",")
}

// getA11yCacheEntry returns cached result if valid, nil otherwise
func (v *V4Server) getA11yCacheEntry(key string) json.RawMessage {
	v.mu.RLock()
	defer v.mu.RUnlock()

	entry, exists := v.a11yCache[key]
	if !exists {
		return nil
	}
	if time.Since(entry.createdAt) > a11yCacheTTL {
		return nil
	}
	// Check URL change (navigation invalidation)
	if v.lastKnownURL != "" && entry.url != "" && entry.url != v.lastKnownURL {
		return nil
	}
	return entry.result
}

// setA11yCacheEntry stores a result in the cache with eviction
func (v *V4Server) setA11yCacheEntry(key string, result json.RawMessage) {
	v.mu.Lock()
	defer v.mu.Unlock()

	// Evict oldest if at capacity
	if _, exists := v.a11yCache[key]; !exists && len(v.a11yCache) >= maxA11yCacheEntries {
		if len(v.a11yCacheOrder) > 0 {
			oldest := v.a11yCacheOrder[0]
			v.a11yCacheOrder = v.a11yCacheOrder[1:]
			delete(v.a11yCache, oldest)
		}
	}

	v.a11yCache[key] = &a11yCacheEntry{
		result:    result,
		createdAt: time.Now(),
		url:       v.lastKnownURL,
	}

	// Update order tracking (remove old position if exists, add to end)
	newOrder := make([]string, 0, len(v.a11yCacheOrder)+1)
	for _, k := range v.a11yCacheOrder {
		if k != key {
			newOrder = append(newOrder, k)
		}
	}
	newOrder = append(newOrder, key)
	v.a11yCacheOrder = newOrder
}

// removeA11yCacheEntry removes a specific cache entry
func (v *V4Server) removeA11yCacheEntry(key string) {
	v.mu.Lock()
	defer v.mu.Unlock()

	delete(v.a11yCache, key)
	newOrder := make([]string, 0, len(v.a11yCacheOrder))
	for _, k := range v.a11yCacheOrder {
		if k != key {
			newOrder = append(newOrder, k)
		}
	}
	v.a11yCacheOrder = newOrder
}

// getOrCreateInflight returns an existing inflight entry to wait on, or nil if this caller should proceed.
// If nil is returned, the caller is the "owner" and should complete the inflight when done.
func (v *V4Server) getOrCreateInflight(key string) *a11yInflightEntry {
	v.mu.Lock()
	defer v.mu.Unlock()

	if existing, ok := v.a11yInflight[key]; ok {
		return existing
	}
	// Create new inflight — caller is the owner
	v.a11yInflight[key] = &a11yInflightEntry{
		done: make(chan struct{}),
	}
	return nil
}

// completeInflight signals waiters and removes the inflight entry
func (v *V4Server) completeInflight(key string, result json.RawMessage, err error) {
	v.mu.Lock()
	entry, exists := v.a11yInflight[key]
	if exists {
		entry.result = result
		entry.err = err
		delete(v.a11yInflight, key)
	}
	v.mu.Unlock()

	if exists {
		close(entry.done)
	}
}

// ExpireA11yCache forces all cache entries to expire (for testing)
func (v *V4Server) ExpireA11yCache() {
	v.mu.Lock()
	defer v.mu.Unlock()

	for key, entry := range v.a11yCache {
		entry.createdAt = time.Now().Add(-a11yCacheTTL - time.Second)
		v.a11yCache[key] = entry
	}
}

// GetA11yCacheSize returns the number of entries in the a11y cache
func (v *V4Server) GetA11yCacheSize() int {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return len(v.a11yCache)
}

// SetLastKnownURL updates the last known page URL for navigation detection.
// If the URL changes, the cache is cleared.
func (v *V4Server) SetLastKnownURL(url string) {
	v.mu.Lock()
	defer v.mu.Unlock()

	if v.lastKnownURL != "" && url != v.lastKnownURL {
		// URL changed — clear cache
		v.a11yCache = make(map[string]*a11yCacheEntry)
		v.a11yCacheOrder = make([]string, 0)
	}
	v.lastKnownURL = url
}
