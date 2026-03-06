// Purpose: Defines canonical wire schema for websocket event telemetry payload transport.
// Why: Prevents websocket event shape drift across extension producers and daemon consumers.
// Docs: docs/features/feature/normalized-event-schema/index.md

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
