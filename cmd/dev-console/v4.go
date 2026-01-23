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
// Performance Budget Types
// ============================================

// PerformanceSnapshot represents a captured performance snapshot from a page load
type PerformanceSnapshot struct {
	URL       string            `json:"url"`
	Timestamp string            `json:"timestamp"`
	Timing    PerformanceTiming `json:"timing"`
	Network   NetworkSummary    `json:"network"`
	LongTasks LongTaskMetrics   `json:"longTasks"`
	CLS       *float64          `json:"cumulativeLayoutShift,omitempty"`
}

// PerformanceTiming holds navigation timing metrics
type PerformanceTiming struct {
	DomContentLoaded       float64  `json:"domContentLoaded"`
	Load                   float64  `json:"load"`
	FirstContentfulPaint   *float64 `json:"firstContentfulPaint"`
	LargestContentfulPaint *float64 `json:"largestContentfulPaint"`
	TimeToFirstByte        float64  `json:"timeToFirstByte"`
	DomInteractive         float64  `json:"domInteractive"`
}

// NetworkSummary holds aggregated network resource metrics
type NetworkSummary struct {
	RequestCount    int                    `json:"requestCount"`
	TransferSize    int64                  `json:"transferSize"`
	DecodedSize     int64                  `json:"decodedSize"`
	ByType          map[string]TypeSummary `json:"byType"`
	SlowestRequests []SlowRequest          `json:"slowestRequests"`
}

// TypeSummary holds per-type resource metrics
type TypeSummary struct {
	Count int   `json:"count"`
	Size  int64 `json:"size"`
}

// SlowRequest represents one of the slowest network requests
type SlowRequest struct {
	URL      string  `json:"url"`
	Duration float64 `json:"duration"`
	Size     int64   `json:"size"`
}

// LongTaskMetrics holds accumulated long task data
type LongTaskMetrics struct {
	Count             int     `json:"count"`
	TotalBlockingTime float64 `json:"totalBlockingTime"`
	Longest           float64 `json:"longest"`
}

// PerformanceBaseline holds averaged performance data for a URL path
type PerformanceBaseline struct {
	URL         string          `json:"url"`
	SampleCount int             `json:"sampleCount"`
	LastUpdated string          `json:"lastUpdated"`
	Timing      BaselineTiming  `json:"timing"`
	Network     BaselineNetwork `json:"network"`
	LongTasks   LongTaskMetrics `json:"longTasks"`
	CLS         *float64        `json:"cumulativeLayoutShift,omitempty"`
}

// BaselineTiming holds averaged timing metrics
type BaselineTiming struct {
	DomContentLoaded       float64  `json:"domContentLoaded"`
	Load                   float64  `json:"load"`
	FirstContentfulPaint   *float64 `json:"firstContentfulPaint"`
	LargestContentfulPaint *float64 `json:"largestContentfulPaint"`
	TimeToFirstByte        float64  `json:"timeToFirstByte"`
	DomInteractive         float64  `json:"domInteractive"`
}

// BaselineNetwork holds averaged network metrics
type BaselineNetwork struct {
	RequestCount int   `json:"requestCount"`
	TransferSize int64 `json:"transferSize"`
}

// PerformanceRegression describes a detected performance regression
type PerformanceRegression struct {
	Metric         string  `json:"metric"`
	Current        float64 `json:"current"`
	Baseline       float64 `json:"baseline"`
	ChangePercent  float64 `json:"changePercent"`
	AbsoluteChange float64 `json:"absoluteChange"`
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
	maxPerfSnapshots    = 20
	maxPerfBaselines    = 20
	defaultWSLimit      = 50
	defaultBodyLimit    = 20
	maxRequestBodySize  = 8192            // 8KB
	maxResponseBodySize = 16384           // 16KB
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
	connections map[string]*connectionState
	closedConns []WebSocketClosedConnection
	connOrder   []string // Track insertion order for eviction

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

	// Performance snapshots
	perfSnapshots     map[string]PerformanceSnapshot
	perfSnapshotOrder []string
	perfBaselines     map[string]PerformanceBaseline
	perfBaselineOrder []string

	// Query timeout
	queryTimeout time.Duration
}

// NewV4Server creates a new v4 server instance
func NewV4Server() *V4Server {
	v4 := &V4Server{
		wsEvents:          make([]WebSocketEvent, 0, maxWSEvents),
		networkBodies:     make([]NetworkBody, 0, maxNetworkBodies),
		enhancedActions:   make([]EnhancedAction, 0, maxEnhancedActions),
		connections:       make(map[string]*connectionState),
		closedConns:       make([]WebSocketClosedConnection, 0),
		connOrder:         make([]string, 0),
		pendingQueries:    make([]pendingQueryEntry, 0),
		queryResults:      make(map[string]json.RawMessage),
		rateResetTime:     time.Now(),
		queryTimeout:      defaultQueryTimeout,
		perfSnapshots:     make(map[string]PerformanceSnapshot),
		perfSnapshotOrder: make([]string, 0),
		perfBaselines:     make(map[string]PerformanceBaseline),
		perfBaselineOrder: make([]string, 0),
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

	for i := range events {
		// Track connection state
		v.trackConnection(events[i])

		// Add to ring buffer
		v.wsEvents = append(v.wsEvents, events[i])
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
	for i := range v.wsEvents {
		total += int64(len(v.wsEvents[i].Data) + len(v.wsEvents[i].URL) + len(v.wsEvents[i].ID) + len(v.wsEvents[i].Timestamp) + len(v.wsEvents[i].Direction) + len(v.wsEvents[i].Event) + 64)
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
	for i := range v.wsEvents {
		if filter.ConnectionID != "" && v.wsEvents[i].ID != filter.ConnectionID {
			continue
		}
		if filter.URLFilter != "" && !strings.Contains(v.wsEvents[i].URL, filter.URLFilter) {
			continue
		}
		if filter.Direction != "" && v.wsEvents[i].Direction != filter.Direction {
			continue
		}
		filtered = append(filtered, v.wsEvents[i])
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
		switch event.Direction {
		case "incoming":
			conn.incoming.total++
			conn.incoming.bytes += event.Size
			conn.incoming.lastAt = event.Timestamp
			conn.incoming.lastData = event.Data
			conn.incoming.recentTimes = appendAndPrune(conn.incoming.recentTimes, msgTime)
		case "outgoing":
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
			bodies[i].RequestBody = bodies[i].RequestBody[:maxRequestBodySize] //nolint:gosec // G602: i is bounded by range
			bodies[i].RequestTruncated = true
		}
		// Truncate response body
		if len(bodies[i].ResponseBody) > maxResponseBodySize {
			bodies[i].ResponseBody = bodies[i].ResponseBody[:maxResponseBodySize] //nolint:gosec // G602: i is bounded by range
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
	for i := range v.enhancedActions {
		if filter.URLFilter != "" && !strings.Contains(v.enhancedActions[i].URL, filter.URLFilter) {
			continue
		}
		filtered = append(filtered, v.enhancedActions[i])
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
// Performance Budget
// ============================================

// AddPerformanceSnapshot stores a performance snapshot and updates baselines
func (v *V4Server) AddPerformanceSnapshot(snapshot PerformanceSnapshot) {
	v.mu.Lock()
	defer v.mu.Unlock()

	url := snapshot.URL

	// LRU eviction for snapshots
	if _, exists := v.perfSnapshots[url]; exists {
		v.perfSnapshotOrder = removeFromOrder(v.perfSnapshotOrder, url)
	} else if len(v.perfSnapshotOrder) >= maxPerfSnapshots {
		oldest := v.perfSnapshotOrder[0]
		delete(v.perfSnapshots, oldest)
		v.perfSnapshotOrder = v.perfSnapshotOrder[1:]
	}
	v.perfSnapshots[url] = snapshot
	v.perfSnapshotOrder = append(v.perfSnapshotOrder, url)

	// Update baseline
	v.updateBaseline(snapshot)
}

// avgOptionalFloat computes a simple running average for nullable float64 pointers
func avgOptionalFloat(baseline *float64, snapshot *float64, n float64) *float64 {
	if snapshot == nil {
		return baseline
	}
	if baseline == nil {
		v := *snapshot
		return &v
	}
	v := *baseline*(n-1)/n + *snapshot/n
	return &v
}

// weightedOptionalFloat computes a weighted average for nullable float64 pointers
func weightedOptionalFloat(baseline *float64, snapshot *float64, baseWeight, newWeight float64) *float64 {
	if snapshot == nil {
		return baseline
	}
	if baseline == nil {
		v := *snapshot
		return &v
	}
	v := *baseline*baseWeight + *snapshot*newWeight
	return &v
}

// updateBaseline updates the running average baseline for a URL
func (v *V4Server) updateBaseline(snapshot PerformanceSnapshot) {
	url := snapshot.URL
	baseline, exists := v.perfBaselines[url]

	if !exists {
		// LRU eviction for baselines
		if len(v.perfBaselineOrder) >= maxPerfBaselines {
			oldest := v.perfBaselineOrder[0]
			delete(v.perfBaselines, oldest)
			v.perfBaselineOrder = v.perfBaselineOrder[1:]
		}

		// First sample: use snapshot values directly
		baseline = PerformanceBaseline{
			URL:         url,
			SampleCount: 1,
			LastUpdated: snapshot.Timestamp,
			Timing: BaselineTiming{
				DomContentLoaded:       snapshot.Timing.DomContentLoaded,
				Load:                   snapshot.Timing.Load,
				FirstContentfulPaint:   snapshot.Timing.FirstContentfulPaint,
				LargestContentfulPaint: snapshot.Timing.LargestContentfulPaint,
				TimeToFirstByte:        snapshot.Timing.TimeToFirstByte,
				DomInteractive:         snapshot.Timing.DomInteractive,
			},
			Network: BaselineNetwork{
				RequestCount: snapshot.Network.RequestCount,
				TransferSize: snapshot.Network.TransferSize,
			},
			LongTasks: snapshot.LongTasks,
			CLS:       snapshot.CLS,
		}
		v.perfBaselines[url] = baseline
		v.perfBaselineOrder = append(v.perfBaselineOrder, url)
		return
	}

	// Remove from order and re-append (LRU touch)
	v.perfBaselineOrder = removeFromOrder(v.perfBaselineOrder, url)
	v.perfBaselineOrder = append(v.perfBaselineOrder, url)

	baseline.SampleCount++
	baseline.LastUpdated = snapshot.Timestamp

	if baseline.SampleCount < 5 {
		// Simple average for first few samples
		n := float64(baseline.SampleCount)
		baseline.Timing.DomContentLoaded = baseline.Timing.DomContentLoaded*(n-1)/n + snapshot.Timing.DomContentLoaded/n
		baseline.Timing.Load = baseline.Timing.Load*(n-1)/n + snapshot.Timing.Load/n
		baseline.Timing.TimeToFirstByte = baseline.Timing.TimeToFirstByte*(n-1)/n + snapshot.Timing.TimeToFirstByte/n
		baseline.Timing.DomInteractive = baseline.Timing.DomInteractive*(n-1)/n + snapshot.Timing.DomInteractive/n
		baseline.Network.RequestCount = int(float64(baseline.Network.RequestCount)*(n-1)/n + float64(snapshot.Network.RequestCount)/n)
		baseline.Network.TransferSize = int64(float64(baseline.Network.TransferSize)*(n-1)/n + float64(snapshot.Network.TransferSize)/n)
		baseline.LongTasks.Count = int(float64(baseline.LongTasks.Count)*(n-1)/n + float64(snapshot.LongTasks.Count)/n)
		baseline.LongTasks.TotalBlockingTime = baseline.LongTasks.TotalBlockingTime*(n-1)/n + snapshot.LongTasks.TotalBlockingTime/n
		baseline.LongTasks.Longest = baseline.LongTasks.Longest*(n-1)/n + snapshot.LongTasks.Longest/n
		baseline.Timing.FirstContentfulPaint = avgOptionalFloat(baseline.Timing.FirstContentfulPaint, snapshot.Timing.FirstContentfulPaint, n)
		baseline.Timing.LargestContentfulPaint = avgOptionalFloat(baseline.Timing.LargestContentfulPaint, snapshot.Timing.LargestContentfulPaint, n)
		baseline.CLS = avgOptionalFloat(baseline.CLS, snapshot.CLS, n)
	} else {
		// Weighted average: 80% existing + 20% new
		baseline.Timing.DomContentLoaded = baseline.Timing.DomContentLoaded*0.8 + snapshot.Timing.DomContentLoaded*0.2
		baseline.Timing.Load = baseline.Timing.Load*0.8 + snapshot.Timing.Load*0.2
		baseline.Timing.TimeToFirstByte = baseline.Timing.TimeToFirstByte*0.8 + snapshot.Timing.TimeToFirstByte*0.2
		baseline.Timing.DomInteractive = baseline.Timing.DomInteractive*0.8 + snapshot.Timing.DomInteractive*0.2
		baseline.Network.RequestCount = int(float64(baseline.Network.RequestCount)*0.8 + float64(snapshot.Network.RequestCount)*0.2)
		baseline.Network.TransferSize = int64(float64(baseline.Network.TransferSize)*0.8 + float64(snapshot.Network.TransferSize)*0.2)
		baseline.LongTasks.Count = int(float64(baseline.LongTasks.Count)*0.8 + float64(snapshot.LongTasks.Count)*0.2)
		baseline.LongTasks.TotalBlockingTime = baseline.LongTasks.TotalBlockingTime*0.8 + snapshot.LongTasks.TotalBlockingTime*0.2
		baseline.LongTasks.Longest = baseline.LongTasks.Longest*0.8 + snapshot.LongTasks.Longest*0.2
		baseline.Timing.FirstContentfulPaint = weightedOptionalFloat(baseline.Timing.FirstContentfulPaint, snapshot.Timing.FirstContentfulPaint, 0.8, 0.2)
		baseline.Timing.LargestContentfulPaint = weightedOptionalFloat(baseline.Timing.LargestContentfulPaint, snapshot.Timing.LargestContentfulPaint, 0.8, 0.2)
		baseline.CLS = weightedOptionalFloat(baseline.CLS, snapshot.CLS, 0.8, 0.2)
	}

	v.perfBaselines[url] = baseline
}

// GetPerformanceSnapshot returns the snapshot for a given URL
func (v *V4Server) GetPerformanceSnapshot(url string) (PerformanceSnapshot, bool) {
	v.mu.RLock()
	defer v.mu.RUnlock()
	s, ok := v.perfSnapshots[url]
	return s, ok
}

// GetLatestPerformanceSnapshot returns the most recently added snapshot
func (v *V4Server) GetLatestPerformanceSnapshot() (PerformanceSnapshot, bool) {
	v.mu.RLock()
	defer v.mu.RUnlock()
	if len(v.perfSnapshotOrder) == 0 {
		return PerformanceSnapshot{}, false
	}
	url := v.perfSnapshotOrder[len(v.perfSnapshotOrder)-1]
	return v.perfSnapshots[url], true
}

// GetPerformanceBaseline returns the baseline for a given URL
func (v *V4Server) GetPerformanceBaseline(url string) (PerformanceBaseline, bool) {
	v.mu.RLock()
	defer v.mu.RUnlock()
	b, ok := v.perfBaselines[url]
	return b, ok
}

// DetectRegressions compares a snapshot against its baseline and returns regressions
func (v *V4Server) DetectRegressions(snapshot PerformanceSnapshot, baseline PerformanceBaseline) []PerformanceRegression {
	var regressions []PerformanceRegression

	// Load time: >50% increase AND >200ms absolute
	if baseline.Timing.Load > 0 {
		change := snapshot.Timing.Load - baseline.Timing.Load
		pct := change / baseline.Timing.Load * 100
		if pct > 50 && change > 200 {
			regressions = append(regressions, PerformanceRegression{
				Metric: "load", Current: snapshot.Timing.Load, Baseline: baseline.Timing.Load,
				ChangePercent: pct, AbsoluteChange: change,
			})
		}
	}

	// FCP: >50% increase AND >200ms absolute
	if snapshot.Timing.FirstContentfulPaint != nil && baseline.Timing.FirstContentfulPaint != nil && *baseline.Timing.FirstContentfulPaint > 0 {
		change := *snapshot.Timing.FirstContentfulPaint - *baseline.Timing.FirstContentfulPaint
		pct := change / *baseline.Timing.FirstContentfulPaint * 100
		if pct > 50 && change > 200 {
			regressions = append(regressions, PerformanceRegression{
				Metric: "firstContentfulPaint", Current: *snapshot.Timing.FirstContentfulPaint, Baseline: *baseline.Timing.FirstContentfulPaint,
				ChangePercent: pct, AbsoluteChange: change,
			})
		}
	}

	// LCP: >50% increase AND >200ms absolute
	if snapshot.Timing.LargestContentfulPaint != nil && baseline.Timing.LargestContentfulPaint != nil && *baseline.Timing.LargestContentfulPaint > 0 {
		change := *snapshot.Timing.LargestContentfulPaint - *baseline.Timing.LargestContentfulPaint
		pct := change / *baseline.Timing.LargestContentfulPaint * 100
		if pct > 50 && change > 200 {
			regressions = append(regressions, PerformanceRegression{
				Metric: "largestContentfulPaint", Current: *snapshot.Timing.LargestContentfulPaint, Baseline: *baseline.Timing.LargestContentfulPaint,
				ChangePercent: pct, AbsoluteChange: change,
			})
		}
	}

	// Request count: >50% increase AND >5 absolute
	if baseline.Network.RequestCount > 0 {
		change := float64(snapshot.Network.RequestCount - baseline.Network.RequestCount)
		pct := change / float64(baseline.Network.RequestCount) * 100
		if pct > 50 && change > 5 {
			regressions = append(regressions, PerformanceRegression{
				Metric: "requestCount", Current: float64(snapshot.Network.RequestCount), Baseline: float64(baseline.Network.RequestCount),
				ChangePercent: pct, AbsoluteChange: change,
			})
		}
	}

	// Transfer size: >100% increase AND >100KB absolute
	if baseline.Network.TransferSize > 0 {
		change := float64(snapshot.Network.TransferSize - baseline.Network.TransferSize)
		pct := change / float64(baseline.Network.TransferSize) * 100
		if pct > 100 && change > 102400 {
			regressions = append(regressions, PerformanceRegression{
				Metric: "transferSize", Current: float64(snapshot.Network.TransferSize), Baseline: float64(baseline.Network.TransferSize),
				ChangePercent: pct, AbsoluteChange: change,
			})
		}
	}

	// Long tasks: any increase from 0, or >100% increase
	if baseline.LongTasks.Count == 0 && snapshot.LongTasks.Count > 0 {
		regressions = append(regressions, PerformanceRegression{
			Metric: "longTaskCount", Current: float64(snapshot.LongTasks.Count), Baseline: 0,
			ChangePercent: 100, AbsoluteChange: float64(snapshot.LongTasks.Count),
		})
	} else if baseline.LongTasks.Count > 0 {
		change := float64(snapshot.LongTasks.Count - baseline.LongTasks.Count)
		pct := change / float64(baseline.LongTasks.Count) * 100
		if pct > 100 {
			regressions = append(regressions, PerformanceRegression{
				Metric: "longTaskCount", Current: float64(snapshot.LongTasks.Count), Baseline: float64(baseline.LongTasks.Count),
				ChangePercent: pct, AbsoluteChange: change,
			})
		}
	}

	// TBT: >100ms absolute increase
	tbtChange := snapshot.LongTasks.TotalBlockingTime - baseline.LongTasks.TotalBlockingTime
	if tbtChange > 100 {
		pct := 0.0
		if baseline.LongTasks.TotalBlockingTime > 0 {
			pct = tbtChange / baseline.LongTasks.TotalBlockingTime * 100
		}
		regressions = append(regressions, PerformanceRegression{
			Metric: "totalBlockingTime", Current: snapshot.LongTasks.TotalBlockingTime, Baseline: baseline.LongTasks.TotalBlockingTime,
			ChangePercent: pct, AbsoluteChange: tbtChange,
		})
	}

	// CLS: >0.05 absolute increase
	if snapshot.CLS != nil && baseline.CLS != nil {
		clsChange := *snapshot.CLS - *baseline.CLS
		if clsChange > 0.05 {
			pct := 0.0
			if *baseline.CLS > 0 {
				pct = clsChange / *baseline.CLS * 100
			}
			regressions = append(regressions, PerformanceRegression{
				Metric: "cumulativeLayoutShift", Current: *snapshot.CLS, Baseline: *baseline.CLS,
				ChangePercent: pct, AbsoluteChange: clsChange,
			})
		}
	}

	return regressions
}

// FormatPerformanceReport generates a human-readable performance report
func (v *V4Server) FormatPerformanceReport(snapshot PerformanceSnapshot, baseline *PerformanceBaseline) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("## Performance Snapshot: %s\n", snapshot.URL))
	sb.WriteString(fmt.Sprintf("Captured: %s\n\n", snapshot.Timestamp))

	sb.WriteString("### Navigation Timing\n")
	sb.WriteString(fmt.Sprintf("- TTFB: %.0fms\n", snapshot.Timing.TimeToFirstByte))
	if snapshot.Timing.FirstContentfulPaint != nil {
		sb.WriteString(fmt.Sprintf("- First Contentful Paint: %.0fms\n", *snapshot.Timing.FirstContentfulPaint))
	}
	if snapshot.Timing.LargestContentfulPaint != nil {
		sb.WriteString(fmt.Sprintf("- Largest Contentful Paint: %.0fms\n", *snapshot.Timing.LargestContentfulPaint))
	}
	sb.WriteString(fmt.Sprintf("- DOM Interactive: %.0fms\n", snapshot.Timing.DomInteractive))
	sb.WriteString(fmt.Sprintf("- DOM Content Loaded: %.0fms\n", snapshot.Timing.DomContentLoaded))
	sb.WriteString(fmt.Sprintf("- Load: %.0fms\n", snapshot.Timing.Load))

	sb.WriteString("\n### Network\n")
	sb.WriteString(fmt.Sprintf("- Requests: %d\n", snapshot.Network.RequestCount))
	sb.WriteString(fmt.Sprintf("- Transfer Size: %s\n", formatBytes(snapshot.Network.TransferSize)))
	sb.WriteString(fmt.Sprintf("- Decoded Size: %s\n", formatBytes(snapshot.Network.DecodedSize)))

	if len(snapshot.Network.SlowestRequests) > 0 {
		sb.WriteString("\n### Slowest Requests\n")
		for _, req := range snapshot.Network.SlowestRequests {
			sb.WriteString(fmt.Sprintf("- %.0fms %s (%s)\n", req.Duration, req.URL, formatBytes(req.Size)))
		}
	}

	sb.WriteString("\n### Long Tasks\n")
	sb.WriteString(fmt.Sprintf("- Count: %d\n", snapshot.LongTasks.Count))
	sb.WriteString(fmt.Sprintf("- Total Blocking Time: %.0fms\n", snapshot.LongTasks.TotalBlockingTime))

	if baseline != nil {
		regressions := v.DetectRegressions(snapshot, *baseline)
		if len(regressions) > 0 {
			sb.WriteString("\n### ⚠️ Regressions Detected\n")
			for _, r := range regressions {
				sb.WriteString(fmt.Sprintf("- **%s**: %.0f → %.0f (+%.0f%%, +%.0f)\n",
					r.Metric, r.Baseline, r.Current, r.ChangePercent, r.AbsoluteChange))
			}
		} else {
			sb.WriteString("\n### ✅ No Regressions\n")
			sb.WriteString(fmt.Sprintf("Baseline: %d samples\n", baseline.SampleCount))
		}
	} else {
		sb.WriteString("\n### 📊 No Baseline Yet\n")
		sb.WriteString("This is the first snapshot for this URL. A baseline will be built over subsequent loads.\n")
	}

	return sb.String()
}

// removeFromOrder removes a string from a slice preserving order
func removeFromOrder(order []string, item string) []string {
	for i, v := range order {
		if v == item {
			return append(order[:i], order[i+1:]...)
		}
	}
	return order
}

// formatBytes formats bytes as human-readable string
func formatBytes(b int64) string {
	if b < 1024 {
		return fmt.Sprintf("%dB", b)
	}
	if b < 1024*1024 {
		return fmt.Sprintf("%.1fKB", float64(b)/1024)
	}
	return fmt.Sprintf("%.1fMB", float64(b)/(1024*1024))
}

// ============================================
// HTTP Handlers
// ============================================

// HandlePerformanceSnapshot handles GET, POST, and DELETE /performance-snapshot
func (v *V4Server) HandlePerformanceSnapshot(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		urlFilter := r.URL.Query().Get("url")

		var snapshot PerformanceSnapshot
		var found bool
		if urlFilter != "" {
			snapshot, found = v.GetPerformanceSnapshot(urlFilter)
		} else {
			snapshot, found = v.GetLatestPerformanceSnapshot()
		}

		if !found {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"snapshot": nil,
				"baseline": nil,
			})
			return
		}

		baseline, baselineFound := v.GetPerformanceBaseline(snapshot.URL)

		resp := map[string]interface{}{
			"snapshot": snapshot,
		}
		if baselineFound {
			resp["baseline"] = &baseline
		} else {
			resp["baseline"] = nil
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
		return
	}

	if r.Method == "POST" {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		var snapshot PerformanceSnapshot
		if err := json.Unmarshal(body, &snapshot); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		v.AddPerformanceSnapshot(snapshot)

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"received":         true,
			"baseline_updated": true,
		})
		return
	}

	if r.Method == "DELETE" {
		v.mu.Lock()
		v.perfSnapshots = make(map[string]PerformanceSnapshot)
		v.perfSnapshotOrder = nil
		v.perfBaselines = make(map[string]PerformanceBaseline)
		v.perfBaselineOrder = nil
		v.mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"cleared": true,
		})
		return
	}

	w.WriteHeader(http.StatusMethodNotAllowed)
}

// HandleWebSocketEvents handles GET and POST /websocket-events
func (v *V4Server) HandleWebSocketEvents(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		events := v.GetWebSocketEvents(WebSocketEventFilter{})
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
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
	_ = json.NewEncoder(w).Encode(status)
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
	_ = json.NewEncoder(w).Encode(resp)
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
		{
			Name:        "check_performance",
			Description: "Get a performance snapshot of the current page including load timing, network weight, main-thread blocking, and regression detection against baselines.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"url": map[string]interface{}{
						"type":        "string",
						"description": "URL path to check (default: latest snapshot)",
					},
				},
			},
		},
		{
			Name:        "get_session_timeline",
			Description: "Get a unified timeline of user actions, network requests, and console errors sorted chronologically.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"last_n_actions": map[string]interface{}{
						"type":        "number",
						"description": "Only include the last N actions and events after them",
					},
					"url_filter": map[string]interface{}{
						"type":        "string",
						"description": "Filter entries by URL substring",
					},
					"include": map[string]interface{}{
						"type":        "array",
						"description": "Entry types to include: actions, network, console (default: all)",
						"items":       map[string]interface{}{"type": "string"},
					},
				},
			},
		},
		{
			Name:        "generate_test",
			Description: "Generate a Playwright test from the session timeline with configurable assertions.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"test_name": map[string]interface{}{
						"type":        "string",
						"description": "Name for the generated test",
					},
					"assert_network": map[string]interface{}{
						"type":        "boolean",
						"description": "Include network response assertions",
					},
					"assert_no_errors": map[string]interface{}{
						"type":        "boolean",
						"description": "Assert no console errors occurred",
					},
					"assert_response_shape": map[string]interface{}{
						"type":        "boolean",
						"description": "Assert response body shape matches",
					},
					"base_url": map[string]interface{}{
						"type":        "string",
						"description": "Replace origin in URLs",
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
	case "check_performance":
		return h.toolCheckPerformance(req, args), true
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
	if err := json.Unmarshal(args, &arguments); err != nil {
		result := map[string]interface{}{
			"content": []map[string]string{
				{"type": "text", "text": "Error parsing arguments: " + err.Error()},
			},
		}
		resultJSON, _ := json.Marshal(result)
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: resultJSON}
	}

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
	if err := json.Unmarshal(args, &arguments); err != nil {
		result := map[string]interface{}{
			"content": []map[string]string{
				{"type": "text", "text": "Error parsing arguments: " + err.Error()},
			},
		}
		resultJSON, _ := json.Marshal(result)
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: resultJSON}
	}

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
	_ = json.Unmarshal(args, &arguments) // Optional args - zero values are acceptable defaults

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
	_ = json.Unmarshal(args, &arguments) // Optional args - zero values are acceptable defaults

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
	_ = json.Unmarshal(args, &arguments) // Optional args - zero values are acceptable defaults

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

// ============================================
// v5 MCP Tool Implementations
// ============================================

func (h *MCPHandlerV4) toolGetEnhancedActions(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var arguments struct {
		LastN int    `json:"last_n"`
		URL   string `json:"url"`
	}
	_ = json.Unmarshal(args, &arguments) // Optional args - zero values are acceptable defaults

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
	_ = json.Unmarshal(args, &arguments) // Optional args - zero values are acceptable defaults

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

func (h *MCPHandlerV4) toolCheckPerformance(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var arguments struct {
		URL string `json:"url"`
	}
	_ = json.Unmarshal(args, &arguments) // Optional args - zero values are acceptable defaults

	var snapshot PerformanceSnapshot
	var found bool
	if arguments.URL != "" {
		snapshot, found = h.v4.GetPerformanceSnapshot(arguments.URL)
	} else {
		snapshot, found = h.v4.GetLatestPerformanceSnapshot()
	}

	if !found {
		result := map[string]interface{}{
			"content": []map[string]string{
				{"type": "text", "text": "No performance snapshot available. Navigate to a page to capture one."},
			},
		}
		resultJSON, _ := json.Marshal(result)
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: resultJSON}
	}

	baseline, baselineFound := h.v4.GetPerformanceBaseline(snapshot.URL)
	var baselinePtr *PerformanceBaseline
	if baselineFound {
		baselinePtr = &baseline
	}

	report := h.v4.FormatPerformanceReport(snapshot, baselinePtr)

	result := map[string]interface{}{
		"content": []map[string]string{
			{"type": "text", "text": report},
		},
	}
	resultJSON, _ := json.Marshal(result)
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: resultJSON}
}

func (h *MCPHandlerV4) toolGetSessionTimeline(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var arguments struct {
		LastNActions int      `json:"last_n_actions"`
		URLFilter    string   `json:"url_filter"`
		Include      []string `json:"include"`
	}
	_ = json.Unmarshal(args, &arguments) // Optional args - zero values are acceptable defaults

	h.server.mu.RLock()
	entries := make([]LogEntry, len(h.server.entries))
	copy(entries, h.server.entries)
	h.server.mu.RUnlock()

	resp := h.v4.GetSessionTimeline(TimelineFilter{
		LastNActions: arguments.LastNActions,
		URLFilter:    arguments.URLFilter,
		Include:      arguments.Include,
	}, entries)

	respJSON, _ := json.Marshal(SessionTimelineResponse(resp))

	result := map[string]interface{}{
		"content": []map[string]string{
			{"type": "text", "text": string(respJSON)},
		},
	}
	resultJSON, _ := json.Marshal(result)
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: resultJSON}
}

func (h *MCPHandlerV4) toolGenerateTest(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var arguments struct {
		TestName            string `json:"test_name"`
		AssertNetwork       bool   `json:"assert_network"`
		AssertNoErrors      bool   `json:"assert_no_errors"`
		AssertResponseShape bool   `json:"assert_response_shape"`
		BaseURL             string `json:"base_url"`
	}
	_ = json.Unmarshal(args, &arguments) // Optional args - zero values are acceptable defaults

	h.server.mu.RLock()
	entries := make([]LogEntry, len(h.server.entries))
	copy(entries, h.server.entries)
	h.server.mu.RUnlock()

	resp := h.v4.GetSessionTimeline(TimelineFilter{}, entries)

	if len(resp.Timeline) == 0 {
		result := map[string]interface{}{
			"content": []map[string]string{
				{"type": "text", "text": "No session data available. Navigate and interact with a page first."},
			},
		}
		resultJSON, _ := json.Marshal(result)
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: resultJSON}
	}

	script := generateTestScript(resp.Timeline, TestGenerationOptions{
		TestName:            arguments.TestName,
		AssertNetwork:       arguments.AssertNetwork,
		AssertNoErrors:      arguments.AssertNoErrors,
		AssertResponseShape: arguments.AssertResponseShape,
		BaseURL:             arguments.BaseURL,
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

	for i := range actions {
		action := &actions[i]
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
// Session Timeline (v5)
// ============================================

// TimelineFilter defines filtering criteria for timeline queries
type TimelineFilter struct {
	LastNActions int
	URLFilter    string
	Include      []string
}

// TimelineEntry represents a single entry in the session timeline
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
	Message       string                 `json:"message,omitempty"`
	Level         string                 `json:"level,omitempty"`
	ToURL         string                 `json:"toUrl,omitempty"`
	Value         string                 `json:"value,omitempty"`
}

// TimelineSummary provides aggregate stats for the session timeline
type TimelineSummary struct {
	Actions         int   `json:"actions"`
	NetworkRequests int   `json:"networkRequests"`
	ConsoleErrors   int   `json:"consoleErrors"`
	DurationMs      int64 `json:"durationMs"`
}

// TimelineResponse is the internal response from GetSessionTimeline
type TimelineResponse struct {
	Timeline []TimelineEntry `json:"timeline"`
	Summary  TimelineSummary `json:"summary"`
}

// SessionTimelineResponse is the JSON response for the MCP tool
type SessionTimelineResponse struct {
	Timeline []TimelineEntry `json:"timeline"`
	Summary  TimelineSummary `json:"summary"`
}

// TestGenerationOptions configures test script generation
type TestGenerationOptions struct {
	TestName            string `json:"test_name"`
	AssertNetwork       bool   `json:"assert_network"`
	AssertNoErrors      bool   `json:"assert_no_errors"`
	AssertResponseShape bool   `json:"assert_response_shape"`
	BaseURL             string `json:"base_url"`
}

// normalizeTimestamp converts an ISO timestamp string to unix milliseconds
func normalizeTimestamp(ts string) int64 {
	if ts == "" {
		return 0
	}
	formats := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05.000Z",
	}
	for _, format := range formats {
		if t, err := time.Parse(format, ts); err == nil {
			return t.UnixMilli()
		}
	}
	return 0
}

// GetSessionTimeline merges actions, network, and console entries into a sorted timeline
func (v *V4Server) GetSessionTimeline(filter TimelineFilter, logEntries []LogEntry) TimelineResponse {
	v.mu.RLock()
	defer v.mu.RUnlock()

	var entries []TimelineEntry

	// Determine action subset
	actions := v.enhancedActions
	if filter.LastNActions > 0 && len(actions) > filter.LastNActions {
		actions = actions[len(actions)-filter.LastNActions:]
	}

	// Determine time boundary
	var minTimestamp int64
	if len(actions) > 0 {
		minTimestamp = actions[0].Timestamp
	}

	// Include check helper
	shouldInclude := func(kind string) bool {
		if len(filter.Include) == 0 {
			return true
		}
		for _, inc := range filter.Include {
			if inc == kind+"s" || inc == kind {
				return true
			}
		}
		return false
	}

	// Add actions
	if shouldInclude("action") {
		for i := range actions {
			if filter.URLFilter != "" && !strings.Contains(actions[i].URL, filter.URLFilter) {
				continue
			}
			entries = append(entries, TimelineEntry{
				Timestamp: actions[i].Timestamp,
				Kind:      "action",
				Type:      actions[i].Type,
				URL:       actions[i].URL,
				Selectors: actions[i].Selectors,
				ToURL:     actions[i].ToURL,
				Value:     actions[i].Value,
			})
		}
	}

	// Add network bodies
	if shouldInclude("network") {
		for _, nb := range v.networkBodies {
			ts := normalizeTimestamp(nb.Timestamp)
			if minTimestamp > 0 && ts < minTimestamp {
				continue
			}
			if filter.URLFilter != "" && !strings.Contains(nb.URL, filter.URLFilter) {
				continue
			}
			entry := TimelineEntry{
				Timestamp:   ts,
				Kind:        "network",
				Method:      nb.Method,
				URL:         nb.URL,
				Status:      nb.Status,
				ContentType: nb.ContentType,
			}
			// Extract response shape for JSON responses
			if strings.Contains(nb.ContentType, "json") && nb.ResponseBody != "" {
				entry.ResponseShape = extractResponseShape(nb.ResponseBody)
			}
			entries = append(entries, entry)
		}
	}

	// Add console entries (error and warn only)
	if shouldInclude("console") {
		for _, le := range logEntries {
			level, _ := le["level"].(string)
			if level != "error" && level != "warn" {
				continue
			}
			ts := normalizeTimestamp(fmt.Sprintf("%v", le["ts"]))
			if minTimestamp > 0 && ts < minTimestamp {
				continue
			}
			msg, _ := le["message"].(string)
			entries = append(entries, TimelineEntry{
				Timestamp: ts,
				Kind:      "console",
				Level:     level,
				Message:   msg,
			})
		}
	}

	// Sort by timestamp
	for i := 1; i < len(entries); i++ {
		for j := i; j > 0 && entries[j].Timestamp < entries[j-1].Timestamp; j-- {
			entries[j], entries[j-1] = entries[j-1], entries[j]
		}
	}

	// Cap at 200
	if len(entries) > 200 {
		entries = entries[:200]
	}

	// Build summary
	summary := TimelineSummary{}
	for i := range entries {
		switch entries[i].Kind {
		case "action":
			summary.Actions++
		case "network":
			summary.NetworkRequests++
		case "console":
			if entries[i].Level == "error" {
				summary.ConsoleErrors++
			}
		}
	}
	if len(entries) >= 2 {
		summary.DurationMs = entries[len(entries)-1].Timestamp - entries[0].Timestamp
	}

	return TimelineResponse{Timeline: entries, Summary: summary}
}

// generateTestScript generates a Playwright test script from a timeline
func generateTestScript(timeline []TimelineEntry, opts TestGenerationOptions) string {
	var sb strings.Builder

	sb.WriteString("import { test, expect } from '@playwright/test'\n\n")

	testName := opts.TestName
	if testName == "" {
		testName = "recorded session"
		if len(timeline) > 0 {
			for i := range timeline {
				if timeline[i].URL != "" {
					testName = timeline[i].URL
					if opts.BaseURL != "" {
						testName = replaceOrigin(testName, opts.BaseURL)
					}
					break
				}
			}
		}
	}

	sb.WriteString(fmt.Sprintf("test('%s', async ({ page }) => {\n", testName))

	if opts.AssertNoErrors {
		sb.WriteString("  const consoleErrors = []\n")
		sb.WriteString("  page.on('console', msg => { if (msg.type() === 'error') consoleErrors.push(msg.text()) })\n\n")
	}

	// Determine start URL
	startURL := ""
	for i := range timeline {
		if timeline[i].Kind == "action" && timeline[i].URL != "" {
			startURL = timeline[i].URL
			break
		}
	}
	if startURL != "" {
		if opts.BaseURL != "" {
			startURL = replaceOrigin(startURL, opts.BaseURL)
		}
		sb.WriteString(fmt.Sprintf("  await page.goto('%s')\n\n", startURL))
	}

	// Track if errors were present in session
	hasErrors := false
	for i := range timeline {
		if timeline[i].Kind == "console" && timeline[i].Level == "error" {
			hasErrors = true
			break
		}
	}

	for i := range timeline {
		entry := &timeline[i]
		switch entry.Kind {
		case "action":
			if entry.Type == "click" && entry.Selectors != nil {
				selector := getSelectorFromMap(entry.Selectors)
				sb.WriteString(fmt.Sprintf("  await page.locator('%s').click()\n", selector))
			} else if entry.Type == "input" {
				value := entry.Value
				if value == "[redacted]" {
					value = "[user-provided]"
				}
				selector := getSelectorFromMap(entry.Selectors)
				sb.WriteString(fmt.Sprintf("  await page.locator('%s').fill('%s')\n", selector, value))
			} else if entry.Type == "navigate" {
				toURL := entry.ToURL
				if toURL == "" {
					toURL = entry.URL
				}
				if opts.BaseURL != "" {
					toURL = replaceOrigin(toURL, opts.BaseURL)
				}
				sb.WriteString(fmt.Sprintf("  await expect(page).toHaveURL(/%s/)\n", strings.TrimPrefix(toURL, "/")))
			}
		case "network":
			if opts.AssertNetwork {
				url := entry.URL
				if opts.BaseURL != "" {
					url = replaceOrigin(url, opts.BaseURL)
				}
				sb.WriteString(fmt.Sprintf("  const response%d = await page.waitForResponse(r => r.url().includes('%s'))\n", i, url))
				sb.WriteString(fmt.Sprintf("  expect(response%d.status()).toBe(%d)\n", i, entry.Status))
				if opts.AssertResponseShape && entry.ResponseShape != nil {
					shapeMap, ok := entry.ResponseShape.(map[string]interface{})
					if ok {
						for key := range shapeMap {
							sb.WriteString(fmt.Sprintf("  expect(await response%d.json()).toHaveProperty('%s')\n", i, key))
						}
					}
				}
			}
		case "console":
			if entry.Level == "error" {
				sb.WriteString(fmt.Sprintf("  // Captured error: %s\n", entry.Message))
			}
		}
	}

	if opts.AssertNoErrors {
		if hasErrors {
			sb.WriteString("\n  // Note: errors were observed during recording\n")
			sb.WriteString("  // expect(consoleErrors).toHaveLength(0)\n")
		} else {
			sb.WriteString("\n  expect(consoleErrors).toHaveLength(0)\n")
		}
	}

	sb.WriteString("})\n")

	return sb.String()
}

func getSelectorFromMap(selectors map[string]interface{}) string {
	if testId, ok := selectors["testId"].(string); ok {
		return fmt.Sprintf("[data-testid=\"%s\"]", testId) //nolint:gocritic // CSS selector needs exact quote format
	}
	if role, ok := selectors["role"].(string); ok {
		return fmt.Sprintf("[role=\"%s\"]", role) //nolint:gocritic // CSS selector needs exact quote format
	}
	return "unknown"
}

// extractResponseShape extracts the type shape of a JSON response (replaces values with type names)
func extractResponseShape(jsonStr string) interface{} {
	var raw interface{}
	if err := json.Unmarshal([]byte(jsonStr), &raw); err != nil {
		return nil
	}
	return extractShape(raw, 0)
}

func extractShape(val interface{}, depth int) interface{} {
	if depth >= 4 {
		return "..."
	}
	switch v := val.(type) {
	case map[string]interface{}:
		result := make(map[string]interface{})
		for key, value := range v {
			result[key] = extractShape(value, depth+1)
		}
		return result
	case []interface{}:
		if len(v) == 0 {
			return []interface{}{}
		}
		return []interface{}{extractShape(v[0], depth+1)}
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
