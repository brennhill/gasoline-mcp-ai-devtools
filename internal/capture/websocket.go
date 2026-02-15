// websocket.go — WebSocket connection tracking and event buffering.
// Captures connection lifecycle (open/close/error) and message payloads
// with adaptive sampling for high-frequency streams.
// Design: Ring buffer with LRU eviction per connection. Messages are
// truncated at 4KB to bound memory. Connection map keyed by unique ID.
package capture

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/dev-console/dev-console/internal/util"
)

// ============================================
// WebSocket Events
// ============================================

// repairWSParallelArrays truncates WS event parallel arrays to equal length if mismatched.
func (c *Capture) repairWSParallelArrays() {
	if len(c.wsEvents) == len(c.wsAddedAt) {
		return
	}
	fmt.Fprintf(os.Stderr, "[gasoline] WARNING: wsEvents/wsAddedAt length mismatch: %d != %d (recovering by truncating)\n",
		len(c.wsEvents), len(c.wsAddedAt))
	minLen := min(len(c.wsEvents), len(c.wsAddedAt))
	c.wsMemoryTotal = 0
	for i := 0; i < minLen; i++ {
		c.wsMemoryTotal += wsEventMemory(&c.wsEvents[i])
	}
	c.wsEvents = c.wsEvents[:minLen]
	c.wsAddedAt = c.wsAddedAt[:minLen]
}

// detectWSBinaryFormat detects binary format in WebSocket message data.
func detectWSBinaryFormat(event *WebSocketEvent) {
	if event.Event != "message" || event.BinaryFormat != "" || len(event.Data) == 0 {
		return
	}
	if format := util.DetectBinaryFormat([]byte(event.Data)); format != nil {
		event.BinaryFormat = format.Name
		event.FormatConfidence = format.Confidence
	}
}

// evictWSByCount trims WebSocket events to MaxWSEvents, updating memory accounting.
func (c *Capture) evictWSByCount() {
	if len(c.wsEvents) <= MaxWSEvents {
		return
	}
	drop := len(c.wsEvents) - MaxWSEvents
	for j := 0; j < drop; j++ {
		c.wsMemoryTotal -= wsEventMemory(&c.wsEvents[j])
	}
	newEvents := make([]WebSocketEvent, MaxWSEvents)
	copy(newEvents, c.wsEvents[drop:])
	c.wsEvents = newEvents
	newAddedAt := make([]time.Time, MaxWSEvents)
	copy(newAddedAt, c.wsAddedAt[drop:])
	c.wsAddedAt = newAddedAt
}

// AddWebSocketEvents adds WebSocket events to the buffer.
func (c *Capture) AddWebSocketEvents(events []WebSocketEvent) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.repairWSParallelArrays()
	c.wsTotalAdded += int64(len(events))
	now := time.Now()

	activeTestIDs := make([]string, 0)
	for testID := range c.ext.activeTestIDs {
		activeTestIDs = append(activeTestIDs, testID)
	}

	for i := range events {
		events[i].TestIDs = activeTestIDs
		detectWSBinaryFormat(&events[i])
		c.trackConnection(events[i])
		c.wsEvents = append(c.wsEvents, events[i])
		c.wsAddedAt = append(c.wsAddedAt, now)
		c.wsMemoryTotal += wsEventMemory(&events[i])
	}

	c.evictWSByCount()
	c.evictWSForMemory()
}

// evictWSForMemory removes oldest events if memory exceeds limit.
// Calculates how many entries to drop in a single pass to avoid O(n²) re-scanning.
func (c *Capture) evictWSForMemory() {
	c.repairWSParallelArrays()
	excess := c.wsMemoryTotal - wsBufferMemoryLimit
	if excess <= 0 {
		return
	}
	drop := 0
	for drop < len(c.wsEvents) && excess > 0 {
		entryMem := wsEventMemory(&c.wsEvents[drop])
		excess -= entryMem
		c.wsMemoryTotal -= entryMem
		drop++
	}
	surviving := make([]WebSocketEvent, len(c.wsEvents)-drop)
	copy(surviving, c.wsEvents[drop:])
	c.wsEvents = surviving
	if len(c.wsAddedAt) >= drop {
		survivingAt := make([]time.Time, len(c.wsAddedAt)-drop)
		copy(survivingAt, c.wsAddedAt[drop:])
		c.wsAddedAt = survivingAt
	}
}

// GetWebSocketEventCount returns the current number of buffered events
func (c *Capture) GetWebSocketEventCount() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.wsEvents)
}

// matchesWSEventFilter returns true if the event passes the filter criteria.
func matchesWSEventFilter(event *WebSocketEvent, filter WebSocketEventFilter) bool {
	if filter.ConnectionID != "" && event.ID != filter.ConnectionID {
		return false
	}
	if filter.URLFilter != "" && !strings.Contains(event.URL, filter.URLFilter) {
		return false
	}
	if filter.Direction != "" && event.Direction != filter.Direction {
		return false
	}
	if filter.TestID != "" && !containsTestID(event.TestIDs, filter.TestID) {
		return false
	}
	return true
}

// containsTestID checks if a test ID is present in the slice.
func containsTestID(testIDs []string, target string) bool {
	for _, tid := range testIDs {
		if tid == target {
			return true
		}
	}
	return false
}

// GetWebSocketEvents returns filtered WebSocket events (newest first)
func (c *Capture) GetWebSocketEvents(filter WebSocketEventFilter) []WebSocketEvent {
	c.mu.RLock()
	defer c.mu.RUnlock()

	limit := filter.Limit
	if limit <= 0 {
		limit = defaultWSLimit
	}

	var filtered []WebSocketEvent
	for i := range c.wsEvents {
		if c.TTL > 0 && i < len(c.wsAddedAt) && isExpiredByTTL(c.wsAddedAt[i], c.TTL) {
			continue
		}
		if !matchesWSEventFilter(&c.wsEvents[i], filter) {
			continue
		}
		filtered = append(filtered, c.wsEvents[i])
	}

	reverseSlice(filtered)
	if len(filtered) > limit {
		filtered = filtered[:limit]
	}
	return filtered
}

// trackConnection updates connection state from events
func (c *Capture) trackConnection(event WebSocketEvent) {
	switch event.Event {
	case "open":
		c.trackConnOpen(event)
	case "close":
		c.trackConnClose(event)
	case "error":
		if conn := c.ws.connections[event.ID]; conn != nil {
			conn.state = "error"
		}
	case "message":
		c.trackConnMessage(event)
	}
}

// trackConnOpen handles a new WebSocket connection opening.
func (c *Capture) trackConnOpen(event WebSocketEvent) {
	if len(c.ws.connections) >= maxActiveConns && len(c.ws.connOrder) > 0 {
		oldestID := c.ws.connOrder[0]
		delete(c.ws.connections, oldestID)
		newOrder := make([]string, len(c.ws.connOrder)-1)
		copy(newOrder, c.ws.connOrder[1:])
		c.ws.connOrder = newOrder
	}
	c.ws.connections[event.ID] = &connectionState{
		id: event.ID, url: event.URL, state: "open", openedAt: event.Timestamp,
	}
	c.ws.connOrder = append(c.ws.connOrder, event.ID)
}

// trackConnClose handles a WebSocket connection closing.
func (c *Capture) trackConnClose(event WebSocketEvent) {
	conn := c.ws.connections[event.ID]
	if conn == nil {
		return
	}
	closed := WebSocketClosedConnection{
		ID: event.ID, URL: conn.url, State: "closed",
		OpenedAt: conn.openedAt, ClosedAt: event.Timestamp,
		CloseCode: event.CloseCode, CloseReason: event.CloseReason,
	}
	closed.TotalMessages.Incoming = conn.incoming.total
	closed.TotalMessages.Outgoing = conn.outgoing.total

	c.ws.closedConns = append(c.ws.closedConns, closed)
	if len(c.ws.closedConns) > maxClosedConns {
		keep := len(c.ws.closedConns) - maxClosedConns
		surviving := make([]WebSocketClosedConnection, maxClosedConns)
		copy(surviving, c.ws.closedConns[keep:])
		c.ws.closedConns = surviving
	}
	delete(c.ws.connections, event.ID)
	c.ws.connOrder = removeFromSlice(c.ws.connOrder, event.ID)
}

// updateDirectionStats updates message stats for a connection direction.
func updateDirectionStats(stats *directionStats, event WebSocketEvent, msgTime time.Time) {
	stats.total++
	stats.bytes += event.Size
	stats.lastAt = event.Timestamp
	stats.lastData = event.Data
	stats.recentTimes = appendAndPrune(stats.recentTimes, msgTime)
}

// trackConnMessage handles a WebSocket message event.
func (c *Capture) trackConnMessage(event WebSocketEvent) {
	conn := c.ws.connections[event.ID]
	if conn == nil {
		return
	}
	msgTime := parseTimestamp(event.Timestamp)
	switch event.Direction {
	case "incoming":
		updateDirectionStats(&conn.incoming, event, msgTime)
	case "outgoing":
		updateDirectionStats(&conn.outgoing, event, msgTime)
	}
	if event.Sampled != nil {
		conn.sampling = true
		conn.lastSample = event.Sampled
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
	surviving := make([]time.Time, len(times)-start)
	copy(surviving, times[start:])
	if !t.IsZero() {
		surviving = append(surviving, t)
	}
	return surviving
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

// buildWSConnection converts internal connection state to the API response type.
func buildWSConnection(conn *connectionState) WebSocketConnection {
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
		Sampling: WebSocketSamplingStatus{Active: conn.sampling},
	}
	if openedTime := parseTimestamp(conn.openedAt); !openedTime.IsZero() {
		wc.Duration = formatDuration(time.Since(openedTime))
	}
	if conn.incoming.lastData != "" {
		wc.LastMessage.Incoming = &WebSocketMessagePreview{
			At: conn.incoming.lastAt, Age: formatAge(conn.incoming.lastAt), Preview: conn.incoming.lastData,
		}
	}
	if conn.outgoing.lastData != "" {
		wc.LastMessage.Outgoing = &WebSocketMessagePreview{
			At: conn.outgoing.lastAt, Age: formatAge(conn.outgoing.lastAt), Preview: conn.outgoing.lastData,
		}
	}
	return wc
}

// GetWebSocketStatus returns current connection states
func (c *Capture) GetWebSocketStatus(filter WebSocketStatusFilter) WebSocketStatusResponse {
	c.mu.RLock()
	defer c.mu.RUnlock()

	resp := WebSocketStatusResponse{
		Connections: make([]WebSocketConnection, 0),
		Closed:      make([]WebSocketClosedConnection, 0),
	}

	for _, conn := range c.ws.connections {
		if filter.URLFilter != "" && !strings.Contains(conn.url, filter.URLFilter) {
			continue
		}
		if filter.ConnectionID != "" && conn.id != filter.ConnectionID {
			continue
		}
		resp.Connections = append(resp.Connections, buildWSConnection(conn))
	}

	for _, closed := range c.ws.closedConns {
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

// HandleWebSocketEvents handles POST /websocket-events from the extension.
// Reads go through GET /telemetry?type=websocket_events.
func (c *Capture) HandleWebSocketEvents(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	body, ok := c.readIngestBody(w, r)
	if !ok {
		return
	}
	var payload struct {
		Events []WebSocketEvent `json:"events"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Invalid JSON"})
		return
	}
	if !c.recordAndRecheck(w, len(payload.Events)) {
		return
	}
	c.AddWebSocketEvents(payload.Events)
	w.WriteHeader(http.StatusOK)
}

// HandleWebSocketStatus handles GET /websocket-status
func (c *Capture) HandleWebSocketStatus(w http.ResponseWriter, r *http.Request) {
	status := c.GetWebSocketStatus(WebSocketStatusFilter{})
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(status)
}
