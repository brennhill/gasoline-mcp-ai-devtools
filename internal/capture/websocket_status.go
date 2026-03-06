// Purpose: Updates per-direction WebSocket connection statistics and recency windows.
// Why: Isolates WebSocket status mutation helpers from connection tracking and event handlers.
package capture

import (
	"time"

	"github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/util"
)

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

// formatDuration delegates to util.FormatDuration for human-readable duration formatting.
func formatDuration(d time.Duration) string {
	return util.FormatDuration(d)
}

// formatAge formats the age of a timestamp relative to now (e.g., "0.2s", "3s", "2m30s")
func formatAge(ts string) string {
	t := util.ParseTimestamp(ts)
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
	if openedTime := util.ParseTimestamp(conn.openedAt); !openedTime.IsZero() {
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
	return c.wsConnections.status(filter)
}
