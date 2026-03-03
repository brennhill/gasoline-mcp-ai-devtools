// Purpose: Implements websocket event ingestion, repair, filtering, and query handlers for capture buffers.
// Why: Preserves websocket lifecycle/message evidence with consistent buffering and binary-format enrichment.
// Docs: docs/features/feature/backend-log-streaming/index.md

package capture

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/util"
)

// ============================================
// WebSocket Events
// ============================================

// repairWSParallelArrays repairs wsEvents/wsAddedAt index alignment.
//
// Invariants:
// - wsEvents and wsAddedAt lengths must match.
// - wsMemoryTotal must equal sum of surviving entries.
//
// Failure semantics:
// - Corruption is healed by truncating to common prefix and recomputing memory total.
func (c *Capture) repairWSParallelArrays() {
	if len(c.buffers.wsEvents) == len(c.buffers.wsAddedAt) {
		return
	}
	fmt.Fprintf(os.Stderr, "[gasoline] WARNING: wsEvents/wsAddedAt length mismatch: %d != %d (recovering by truncating)\n",
		len(c.buffers.wsEvents), len(c.buffers.wsAddedAt))
	minLen := min(len(c.buffers.wsEvents), len(c.buffers.wsAddedAt))
	c.buffers.wsMemoryTotal = 0
	for i := 0; i < minLen; i++ {
		c.buffers.wsMemoryTotal += wsEventMemory(&c.buffers.wsEvents[i])
	}
	c.buffers.wsEvents = c.buffers.wsEvents[:minLen]
	c.buffers.wsAddedAt = c.buffers.wsAddedAt[:minLen]
}

// detectWSBinaryFormat best-effort classifies message payload format.
//
// Failure semantics:
// - Non-message/empty/unrecognized payloads remain unannotated without ingestion failure.
func detectWSBinaryFormat(event *WebSocketEvent) {
	if event.Event != "message" || event.BinaryFormat != "" || len(event.Data) == 0 {
		return
	}
	if format := util.DetectBinaryFormat([]byte(event.Data)); format != nil {
		event.BinaryFormat = format.Name
		event.FormatConfidence = format.Confidence
	}
}

// evictWSByCount enforces count cap while preserving newest events.
//
// Invariants:
// - wsMemoryTotal is decremented for each dropped entry before slice replacement.
func (c *Capture) evictWSByCount() {
	if len(c.buffers.wsEvents) <= MaxWSEvents {
		return
	}
	drop := len(c.buffers.wsEvents) - MaxWSEvents
	for j := 0; j < drop; j++ {
		c.buffers.wsMemoryTotal -= wsEventMemory(&c.buffers.wsEvents[j])
	}
	newEvents := make([]WebSocketEvent, MaxWSEvents)
	copy(newEvents, c.buffers.wsEvents[drop:])
	c.buffers.wsEvents = newEvents
	newAddedAt := make([]time.Time, MaxWSEvents)
	copy(newAddedAt, c.buffers.wsAddedAt[drop:])
	c.buffers.wsAddedAt = newAddedAt
}

// AddWebSocketEvents ingests websocket telemetry and updates connection model.
//
// Invariants:
// - wsEvents/wsAddedAt are appended in lockstep for TTL and cursor correctness.
// - Connection tracking is updated from same event stream under the same lock.
// - Active test IDs are snapshotted once per batch for deterministic tagging.
//
// Failure semantics:
// - Over-capacity batches are accepted then oldest entries are evicted.
// - Unknown event kinds are retained in wsEvents even if they do not change connection state.
func (c *Capture) AddWebSocketEvents(events []WebSocketEvent) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.repairWSParallelArrays()
	c.buffers.wsTotalAdded += int64(len(events))
	now := time.Now()

	activeTestIDs := make([]string, 0)
	for testID := range c.extensionState.activeTestIDs {
		activeTestIDs = append(activeTestIDs, testID)
	}

	for i := range events {
		events[i].TestIDs = activeTestIDs
		detectWSBinaryFormat(&events[i])
		c.trackConnection(events[i])
		c.buffers.wsEvents = append(c.buffers.wsEvents, events[i])
		c.buffers.wsAddedAt = append(c.buffers.wsAddedAt, now)
		c.buffers.wsMemoryTotal += wsEventMemory(&events[i])
	}

	c.evictWSByCount()
	c.evictWSForMemory()
}

// evictWSForMemory enforces websocket memory budget with oldest-first trimming.
//
// Invariants:
// - Parallel arrays remain aligned after eviction.
//
// Failure semantics:
// - Can drop multiple oldest events in one pass; newer events are preserved.
func (c *Capture) evictWSForMemory() {
	c.repairWSParallelArrays()
	excess := c.buffers.wsMemoryTotal - wsBufferMemoryLimit
	if excess <= 0 {
		return
	}
	drop := 0
	for drop < len(c.buffers.wsEvents) && excess > 0 {
		entryMem := wsEventMemory(&c.buffers.wsEvents[drop])
		excess -= entryMem
		c.buffers.wsMemoryTotal -= entryMem
		drop++
	}
	surviving := make([]WebSocketEvent, len(c.buffers.wsEvents)-drop)
	copy(surviving, c.buffers.wsEvents[drop:])
	c.buffers.wsEvents = surviving
	if len(c.buffers.wsAddedAt) >= drop {
		survivingAt := make([]time.Time, len(c.buffers.wsAddedAt)-drop)
		copy(survivingAt, c.buffers.wsAddedAt[drop:])
		c.buffers.wsAddedAt = survivingAt
	}
}

// GetWebSocketEventCount returns the current number of buffered events
func (c *Capture) GetWebSocketEventCount() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.buffers.wsEvents)
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

// GetWebSocketEvents returns filtered WebSocket events (newest first).
// Iterates backward from newest and stops at limit for O(limit) instead of O(n).
func (c *Capture) GetWebSocketEvents(filter WebSocketEventFilter) []WebSocketEvent {
	c.mu.RLock()
	defer c.mu.RUnlock()

	limit := filter.Limit
	if limit <= 0 {
		limit = defaultWSLimit
	}

	filtered := make([]WebSocketEvent, 0, limit)
	for i := len(c.buffers.wsEvents) - 1; i >= 0; i-- {
		if c.TTL > 0 && i < len(c.buffers.wsAddedAt) && isExpiredByTTL(c.buffers.wsAddedAt[i], c.TTL) {
			break
		}
		if !matchesWSEventFilter(&c.buffers.wsEvents[i], filter) {
			continue
		}
		filtered = append(filtered, c.buffers.wsEvents[i])
		if len(filtered) >= limit {
			break
		}
	}
	return filtered
}
