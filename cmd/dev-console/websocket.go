// websocket.go — WebSocket connection tracking and event buffering.
// Captures connection lifecycle (open/close/error) and message payloads
// with adaptive sampling for high-frequency streams.
// Design: Ring buffer with LRU eviction per connection. Messages are
// truncated at 4KB to bound memory. Connection map keyed by unique ID.
package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// ============================================
// WebSocket Events
// ============================================

// AddWebSocketEvents adds WebSocket events to the buffer
func (c *Capture) AddWebSocketEvents(events []WebSocketEvent) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Enforce memory limits before adding
	c.enforceMemory()

	c.wsTotalAdded += int64(len(events))
	now := time.Now()
	for i := range events {
		// Detect binary format in message data
		if events[i].Event == "message" && events[i].BinaryFormat == "" && len(events[i].Data) > 0 {
			if format := DetectBinaryFormat([]byte(events[i].Data)); format != nil {
				events[i].BinaryFormat = format.Name
				events[i].FormatConfidence = format.Confidence
			}
		}

		// Track connection state
		c.trackConnection(events[i])

		// Add to ring buffer
		c.wsEvents = append(c.wsEvents, events[i])
		c.wsAddedAt = append(c.wsAddedAt, now)
	}

	// Enforce max count (respecting minimal mode)
	capacity := c.effectiveWSCapacity()
	if len(c.wsEvents) > capacity {
		keep := len(c.wsEvents) - capacity
		newEvents := make([]WebSocketEvent, capacity)
		copy(newEvents, c.wsEvents[keep:])
		c.wsEvents = newEvents
		newAddedAt := make([]time.Time, capacity)
		copy(newAddedAt, c.wsAddedAt[keep:])
		c.wsAddedAt = newAddedAt
	}

	// Enforce per-buffer memory limit
	c.evictWSForMemory()
}

// evictWSForMemory removes oldest events if memory exceeds limit.
// Calculates how many entries to drop in a single pass to avoid O(n²) re-scanning.
func (c *Capture) evictWSForMemory() {
	excess := c.calcWSMemory() - wsBufferMemoryLimit
	if excess <= 0 {
		return
	}
	drop := 0
	for drop < len(c.wsEvents) && excess > 0 {
		excess -= int64(len(c.wsEvents[drop].Data)) + wsEventOverhead
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

// GetWebSocketEvents returns filtered WebSocket events (newest first)
func (c *Capture) GetWebSocketEvents(filter WebSocketEventFilter) []WebSocketEvent {
	c.mu.RLock()
	defer c.mu.RUnlock()

	limit := filter.Limit
	if limit <= 0 {
		limit = defaultWSLimit
	}

	// Filter events
	var filtered []WebSocketEvent
	for i := range c.wsEvents {
		// TTL filtering: skip entries older than TTL
		if c.TTL > 0 && i < len(c.wsAddedAt) && isExpiredByTTL(c.wsAddedAt[i], c.TTL) {
			continue
		}
		if filter.ConnectionID != "" && c.wsEvents[i].ID != filter.ConnectionID {
			continue
		}
		if filter.URLFilter != "" && !strings.Contains(c.wsEvents[i].URL, filter.URLFilter) {
			continue
		}
		if filter.Direction != "" && c.wsEvents[i].Direction != filter.Direction {
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
		// Enforce max active connections
		if len(c.connections) >= maxActiveConns {
			// Evict oldest
			if len(c.connOrder) > 0 {
				oldestID := c.connOrder[0]
				delete(c.connections, oldestID)
				newOrder := make([]string, len(c.connOrder)-1)
				copy(newOrder, c.connOrder[1:])
				c.connOrder = newOrder
			}
		}
		c.connections[event.ID] = &connectionState{
			id:       event.ID,
			url:      event.URL,
			state:    "open",
			openedAt: event.Timestamp,
		}
		c.connOrder = append(c.connOrder, event.ID)

	case "close":
		conn := c.connections[event.ID]
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

		c.closedConns = append(c.closedConns, closed)
		if len(c.closedConns) > maxClosedConns {
			keep := len(c.closedConns) - maxClosedConns
			surviving := make([]WebSocketClosedConnection, maxClosedConns)
			copy(surviving, c.closedConns[keep:])
			c.closedConns = surviving
		}

		delete(c.connections, event.ID)
		// Remove from order
		c.connOrder = removeFromSlice(c.connOrder, event.ID)

	case "error":
		conn := c.connections[event.ID]
		if conn != nil {
			conn.state = "error"
		}

	case "message":
		conn := c.connections[event.ID]
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

// GetWebSocketStatus returns current connection states
func (c *Capture) GetWebSocketStatus(filter WebSocketStatusFilter) WebSocketStatusResponse {
	c.mu.RLock()
	defer c.mu.RUnlock()

	resp := WebSocketStatusResponse{
		Connections: make([]WebSocketConnection, 0),
		Closed:      make([]WebSocketClosedConnection, 0),
	}

	for _, conn := range c.connections {
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

	for _, closed := range c.closedConns {
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

func (c *Capture) HandleWebSocketEvents(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		events := c.GetWebSocketEvents(WebSocketEventFilter{})
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"events": events,
			"count":  len(events),
		})
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
		w.WriteHeader(http.StatusBadRequest)
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

func (h *ToolHandler) toolGetWSEvents(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var arguments struct {
		ConnectionID string `json:"connection_id"`
		URL          string `json:"url"`
		Direction    string `json:"direction"`
		Limit        int    `json:"limit"`
	}
	if err := json.Unmarshal(args, &arguments); err != nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")}
	}

	events := h.capture.GetWebSocketEvents(WebSocketEventFilter{
		ConnectionID: arguments.ConnectionID,
		URLFilter:    arguments.URL,
		Direction:    arguments.Direction,
		Limit:        arguments.Limit,
	})

	// Apply noise filtering
	if h.noise != nil {
		var filtered []WebSocketEvent
		for i := range events {
			if !h.noise.IsWebSocketNoise(events[i]) {
				filtered = append(filtered, events[i])
			}
		}
		events = filtered
	}

	if len(events) == 0 {
		msg := "No WebSocket events captured"
		if h.captureOverrides != nil {
			overrides := h.captureOverrides.GetAll()
			wsMode := overrides["ws_mode"]
			if wsMode == "" {
				wsMode = "lifecycle" // default
			}
			switch wsMode {
			case "off":
				msg += "\n\nWebSocket capture is OFF. To enable, call:\nconfigure({action: \"capture\", settings: {ws_mode: \"lifecycle\"}})"
			case "lifecycle":
				msg += "\n\nws_mode is 'lifecycle' (open/close only, no message payloads). To capture message content, call:\nconfigure({action: \"capture\", settings: {ws_mode: \"messages\"}})"
			}
		}
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpTextResponse(msg)}
	}

	summary := fmt.Sprintf("%d WebSocket event(s)", len(events))
	rows := make([][]string, len(events))
	for i, e := range events {
		rows[i] = []string{
			e.ID,
			e.Event,
			truncate(e.URL, 60),
			e.Direction,
			fmt.Sprintf("%d", e.Size),
			e.Timestamp,
		}
	}
	table := markdownTable([]string{"ID", "Event", "URL", "Direction", "Size", "Time"}, rows)
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpMarkdownResponse(summary, table)}
}

func (h *ToolHandler) toolGetWSStatus(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var arguments struct {
		URL          string `json:"url"`
		ConnectionID string `json:"connection_id"`
	}
	if err := json.Unmarshal(args, &arguments); err != nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")}
	}

	status := h.capture.GetWebSocketStatus(WebSocketStatusFilter{
		URLFilter:    arguments.URL,
		ConnectionID: arguments.ConnectionID,
	})

	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("WebSocket connection status", status)}
}
