// network.go — Network and browser telemetry types.
// Contains WebSocket, HTTP, and user action types captured from the browser.
// Zero dependencies - foundational types used by capture and analysis packages.
package types

import "time"

// ============================================
// WebSocket Types
// ============================================

// WebSocketEvent represents a captured WebSocket event (message, open, close, etc.)
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
	TabId            int           `json:"tab_id,omitempty"` // Chrome tab ID that produced this event
	TestIDs          []string      `json:"test_ids,omitempty"` // Test IDs this event belongs to
}

// SamplingInfo describes the sampling state when a message was captured
type SamplingInfo struct {
	Rate   string `json:"rate"`
	Logged string `json:"logged"`
	Window string `json:"window"`
}

// WebSocketEventFilter defines filtering criteria for WebSocket events
type WebSocketEventFilter struct {
	ConnectionID string
	URLFilter    string
	Direction    string
	Limit        int
	TestID       string // If set, filter events where TestID is in event's TestIDs array
}

// WebSocketStatusFilter defines filtering criteria for WebSocket status
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

// ============================================
// HTTP Network Types
// ============================================

// NetworkBody represents a captured network request/response
type NetworkBody struct {
	Timestamp          string            `json:"ts,omitempty"`
	Method             string            `json:"method"`
	URL                string            `json:"url"`
	Status             int               `json:"status"`
	RequestBody        string            `json:"request_body,omitempty"`
	ResponseBody       string            `json:"response_body,omitempty"`
	ContentType        string            `json:"content_type,omitempty"`
	Duration           int               `json:"duration,omitempty"`
	RequestTruncated   bool              `json:"request_truncated,omitempty"`
	ResponseTruncated  bool              `json:"response_truncated,omitempty"`
	ResponseHeaders    map[string]string `json:"response_headers,omitempty"`
	HasAuthHeader      bool              `json:"has_auth_header,omitempty"`
	BinaryFormat       string            `json:"binary_format,omitempty"`
	FormatConfidence   float64           `json:"format_confidence,omitempty"`
	TabId              int               `json:"tab_id,omitempty"` // Chrome tab ID that produced this request
	TestIDs            []string          `json:"test_ids,omitempty"` // Test IDs this entry belongs to
}

// NetworkBodyFilter defines filtering criteria for network bodies
type NetworkBodyFilter struct {
	URLFilter string
	Method    string
	StatusMin int
	StatusMax int
	Limit     int
	TestID    string // If set, filter entries where TestID is in entry's TestIDs array
}

// NetworkWaterfallEntry represents a single network resource timing entry
// from the browser's PerformanceResourceTiming API
type NetworkWaterfallEntry struct {
	Name            string    `json:"name"`                  // Full URL
	URL             string    `json:"url"`                   // Same as name
	InitiatorType   string    `json:"initiator_type"`        // snake_case (from browser PerformanceResourceTiming)
	Duration        float64   `json:"duration"`              // snake_case (from browser PerformanceResourceTiming)
	StartTime       float64   `json:"start_time"`            // snake_case (from browser PerformanceResourceTiming)
	FetchStart      float64   `json:"fetch_start"`           // snake_case (from browser PerformanceResourceTiming)
	ResponseEnd     float64   `json:"response_end"`          // snake_case (from browser PerformanceResourceTiming)
	TransferSize    int       `json:"transfer_size"`         // snake_case (from browser PerformanceResourceTiming)
	DecodedBodySize int       `json:"decoded_body_size"`     // snake_case (from browser PerformanceResourceTiming)
	EncodedBodySize int       `json:"encoded_body_size"`     // snake_case (from browser PerformanceResourceTiming)
	PageURL         string    `json:"page_url,omitempty"`
	Timestamp       time.Time `json:"timestamp,omitempty"`   // Server-side timestamp
}

// NetworkWaterfallPayload is POSTed by the extension
type NetworkWaterfallPayload struct {
	Entries []NetworkWaterfallEntry `json:"entries"`
	PageURL string                  `json:"page_url"`
}

// ============================================
// User Action Types
// ============================================

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
	TabId         int                    `json:"tab_id,omitempty"` // Chrome tab ID that produced this action
	TestIDs       []string               `json:"test_ids,omitempty"` // Test IDs this action belongs to
}

// EnhancedActionFilter defines filtering criteria for enhanced actions
type EnhancedActionFilter struct {
	LastN     int
	URLFilter string
	TestID    string // If set, filter actions where TestID is in action's TestIDs array
}
