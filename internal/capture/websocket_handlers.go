// Purpose: Handles HTTP POST ingestion of WebSocket events from the browser extension.
// Why: Separates WebSocket event ingestion HTTP handler from storage and query logic.
package capture

import (
	"encoding/json"
	"net/http"

	"github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/util"
)

// HandleWebSocketEvents handles POST /websocket-events from the extension.
// Reads go through GET /telemetry?type=websocket_events.
func (c *Capture) HandleWebSocketEvents(w http.ResponseWriter, r *http.Request) {
	if !util.RequireMethod(w, r, "POST") {
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
		util.JSONResponse(w, http.StatusBadRequest, map[string]string{"error": "Invalid JSON"})
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
	util.JSONResponse(w, http.StatusOK, status)
}
