// Purpose: Owns websocket-types.go runtime behavior and integration logic.
// Docs: docs/features/feature/backend-log-streaming/index.md

// websocket-types.go — WebSocket event and connection tracking types.
// WebSocketEvent represents captured WebSocket events with sampling and schema info.
//
// JSON CONVENTION: All fields MUST use snake_case. See .claude/refs/api-naming-standards.md
// Deviations from snake_case MUST be tagged with // SPEC:<spec-name> at the field level.
package capture

import (
	"time"
)

// WebSocketEvent represents a captured WebSocket event
type WebSocketEvent struct {
	Timestamp        string        `json:"ts,omitempty"`
	Type             string        `json:"type,omitempty"`
	Event            string        `json:"event"`
	ID               string        `json:"id"`
	URL              string        `json:"url,omitempty"`
	Direction        string        `json:"direction,omitempty"`
	Data             string        `json:"data,omitempty"`
	Size             int           `json:"size,omitempty"`
	CloseCode        int           `json:"code,omitempty"`
	CloseReason      string        `json:"reason,omitempty"`
	Sampled          *SamplingInfo `json:"sampled,omitempty"`
	BinaryFormat     string        `json:"binary_format,omitempty"`
	FormatConfidence float64       `json:"format_confidence,omitempty"`
	TabId            int           `json:"tab_id,omitempty"`   // Chrome tab ID that produced this event
	TestIDs          []string      `json:"test_ids,omitempty"` // Test IDs this event belongs to (for test boundary correlation)
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
	TestID       string // If set, filter events where TestID is in event's TestIDs array
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
	OpenedAt    string                  `json:"opened_at,omitempty"`
	Duration    string                  `json:"duration,omitempty"`
	MessageRate WebSocketMessageRate    `json:"message_rate"`
	LastMessage WebSocketLastMessage    `json:"last_message"`
	Schema      *WebSocketSchema        `json:"schema,omitempty"`
	Sampling    WebSocketSamplingStatus `json:"sampling"`
}

// WebSocketClosedConnection represents a closed WebSocket connection
type WebSocketClosedConnection struct {
	ID            string `json:"id"`
	URL           string `json:"url"`
	State         string `json:"state"`
	OpenedAt      string `json:"opened_at,omitempty"`
	ClosedAt      string `json:"closed_at,omitempty"`
	CloseCode     int    `json:"close_code"`
	CloseReason   string `json:"close_reason"`
	TotalMessages struct {
		Incoming int `json:"incoming"`
		Outgoing int `json:"outgoing"`
	} `json:"total_messages"`
}

// WebSocketMessageRate contains rate info for a direction
type WebSocketMessageRate struct {
	Incoming WebSocketDirectionStats `json:"incoming"`
	Outgoing WebSocketDirectionStats `json:"outgoing"`
}

// WebSocketDirectionStats contains stats for a message direction
type WebSocketDirectionStats struct {
	PerSecond float64 `json:"per_second"`
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
	DetectedKeys []string `json:"detected_keys,omitempty"`
	MessageCount int      `json:"message_count"`
	Consistent   bool     `json:"consistent"`
	Variants     []string `json:"variants,omitempty"`
}

// WebSocketSamplingStatus describes sampling state
type WebSocketSamplingStatus struct {
	Active bool   `json:"active"`
	Rate   string `json:"rate,omitempty"`
	Reason string `json:"reason,omitempty"`
}

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
