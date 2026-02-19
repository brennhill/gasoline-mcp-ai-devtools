// wire_websocket_event.go — Wire type for WebSocket events over HTTP.
// Defines the JSON fields sent by the extension for WebSocket events.
// Changes here MUST be mirrored in src/types/wire-websocket-event.ts.
//
// JSON CONVENTION: All fields MUST use snake_case. See .claude/refs/api-naming-standards.md
package types

// WireWebSocketEvent is the canonical wire format for captured WebSocket events.
type WireWebSocketEvent struct {
	Timestamp   string `json:"ts,omitempty"`
	Type        string `json:"type,omitempty"`
	Event       string `json:"event"`
	ID          string `json:"id"`
	URL         string `json:"url,omitempty"`
	Direction   string `json:"direction,omitempty"`
	Data        string `json:"data,omitempty"`
	Size        int    `json:"size,omitempty"`
	CloseCode   int    `json:"code,omitempty"`
	CloseReason string `json:"reason,omitempty"`
}
