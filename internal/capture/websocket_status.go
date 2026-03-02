package capture

import (
	"fmt"
	"strings"
	"time"

	"github.com/dev-console/dev-console/internal/util"
)

// trackConnection applies one event to per-connection lifecycle state.
//
// Failure semantics:
// - Events for unknown IDs are tolerated and ignored where state cannot be reconciled.
func (c *Capture) trackConnection(event WebSocketEvent) {
	switch event.Event {
	case "open":
		c.trackConnOpen(event)
	case "close":
		c.trackConnClose(event)
	case "error":
		if conn := c.wsConnections.connections[event.ID]; conn != nil {
			conn.state = "error"
		}
	case "message":
		c.trackConnMessage(event)
	}
}

// trackConnOpen registers/refreshes active connection metadata.
//
// Invariants:
// - Active connection map is bounded by maxActiveConns using oldest-id eviction.
func (c *Capture) trackConnOpen(event WebSocketEvent) {
	if len(c.wsConnections.connections) >= maxActiveConns && len(c.wsConnections.connOrder) > 0 {
		oldestID := c.wsConnections.connOrder[0]
		delete(c.wsConnections.connections, oldestID)
		newOrder := make([]string, len(c.wsConnections.connOrder)-1)
		copy(newOrder, c.wsConnections.connOrder[1:])
		c.wsConnections.connOrder = newOrder
	}
	c.wsConnections.connections[event.ID] = &connectionState{
		id: event.ID, url: event.URL, state: "open", openedAt: event.Timestamp,
	}
	c.wsConnections.connOrder = append(c.wsConnections.connOrder, event.ID)
}

// trackConnClose finalizes a connection and moves summary into closed history.
//
// Invariants:
// - Closed connection history is bounded by maxClosedConns.
//
// Failure semantics:
// - Unknown close events are ignored; no synthetic connection is created.
func (c *Capture) trackConnClose(event WebSocketEvent) {
	conn := c.wsConnections.connections[event.ID]
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

	c.wsConnections.closedConns = append(c.wsConnections.closedConns, closed)
	if len(c.wsConnections.closedConns) > maxClosedConns {
		keep := len(c.wsConnections.closedConns) - maxClosedConns
		surviving := make([]WebSocketClosedConnection, maxClosedConns)
		copy(surviving, c.wsConnections.closedConns[keep:])
		c.wsConnections.closedConns = surviving
	}
	delete(c.wsConnections.connections, event.ID)
	c.wsConnections.connOrder = removeFromSlice(c.wsConnections.connOrder, event.ID)
}

// updateDirectionStats mutates per-direction counters and recency windows.
//
// Invariants:
// - recentTimes contains only timestamps within rateWindow after appendAndPrune.
func updateDirectionStats(stats *directionStats, event WebSocketEvent, msgTime time.Time) {
	stats.total++
	stats.bytes += event.Size
	stats.lastAt = event.Timestamp
	stats.lastData = event.Data
	stats.recentTimes = appendAndPrune(stats.recentTimes, msgTime)
}

// trackConnMessage updates rate/counter state for an active connection.
//
// Failure semantics:
// - Messages on unknown connections are ignored instead of creating implicit connection records.
func (c *Capture) trackConnMessage(event WebSocketEvent) {
	conn := c.wsConnections.connections[event.ID]
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

// parseTimestamp delegates to util.ParseTimestamp for RFC3339/RFC3339Nano parsing.
func parseTimestamp(ts string) time.Time {
	return util.ParseTimestamp(ts)
}

// appendAndPrune maintains a bounded-by-time event window.
//
// Invariants:
// - Returned slice preserves chronological order of surviving timestamps.
// - Prunes in-place to avoid allocation on every call.
func appendAndPrune(times []time.Time, t time.Time) []time.Time {
	cutoff := time.Now().Add(-rateWindow)
	// Prune old entries in-place
	start := 0
	for start < len(times) && times[start].Before(cutoff) {
		start++
	}
	times = times[start:]
	if !t.IsZero() {
		times = append(times, t)
	}
	return times
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

	for _, conn := range c.wsConnections.connections {
		if filter.URLFilter != "" && !strings.Contains(conn.url, filter.URLFilter) {
			continue
		}
		if filter.ConnectionID != "" && conn.id != filter.ConnectionID {
			continue
		}
		resp.Connections = append(resp.Connections, buildWSConnection(conn))
	}

	for _, closed := range c.wsConnections.closedConns {
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
