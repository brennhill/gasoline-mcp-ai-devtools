// Purpose: Handles HTTP POST ingestion of WebSocket events from the browser extension.
// Why: Separates WebSocket event ingestion HTTP handler from storage and query logic.
package capture

import (
	"encoding/json"
	"net/http"
)

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
