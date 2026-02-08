// handlers.go — HTTP handlers for extension ↔ server communication
// Implements endpoints for pending queries, query results, and extension status.
// Part of the async queue-and-poll architecture (see internal/capture/queries.go).
package capture

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"
)

// HandleNetworkBodies handles network request bodies.
// POST: receives and stores bodies from the extension
// GET: returns stored bodies
func (c *Capture) HandleNetworkBodies(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "POST":
		r.Body = http.MaxBytesReader(w, r.Body, maxExtensionPostBody)
		var payload struct {
			Bodies []NetworkBody `json:"bodies"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			fmt.Fprintf(os.Stderr, "[gasoline] HandleNetworkBodies: Invalid JSON - %v\n", err)
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "Invalid JSON"})
			return
		}
		c.AddNetworkBodies(payload.Bodies)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"status": "ok",
			"count":  len(payload.Bodies),
		})
	case "GET":
		bodies := c.GetNetworkBodies()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"bodies": bodies,
			"count":  len(bodies),
		})
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

// HandleNetworkWaterfall handles network waterfall data.
// POST: receives and stores waterfall entries from the extension
// GET: returns stored waterfall entries
func (c *Capture) HandleNetworkWaterfall(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "POST":
		c.handleNetworkWaterfallPOST(w, r)
	case "GET":
		c.handleNetworkWaterfallGET(w, r)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

// handleNetworkWaterfallPOST stores waterfall entries from the extension
func (c *Capture) handleNetworkWaterfallPOST(w http.ResponseWriter, r *http.Request) {
	var payload NetworkWaterfallPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		fmt.Fprintf(os.Stderr, "[gasoline] handleNetworkWaterfallPOST: Invalid JSON - %v\n", err)
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid JSON"})
		return
	}

	c.AddNetworkWaterfallEntries(payload.Entries, payload.PageURL)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"status": "ok",
		"count":  len(payload.Entries),
	})
}

// handleNetworkWaterfallGET returns stored waterfall entries
func (c *Capture) handleNetworkWaterfallGET(w http.ResponseWriter, r *http.Request) {
	c.mu.RLock()
	entries := make([]NetworkWaterfallEntry, len(c.nw.entries))
	copy(entries, c.nw.entries)
	c.mu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"waterfall": entries,
		"count":     len(entries),
	})
}

// HandlePendingQueries returns pending DOM queries for extension to execute.
// Extension polls this endpoint every 1-2 seconds to pick up commands.
func (c *Capture) HandlePendingQueries(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	// Record that extension polled (for extension_connected status)
	c.mu.Lock()
	c.ext.lastPollAt = time.Now()
	// Also capture pilot state from header if present
	if pilotHeader := r.Header.Get("X-Gasoline-Pilot"); pilotHeader != "" {
		c.ext.pilotEnabled = pilotHeader == "1"
		c.ext.pilotUpdatedAt = time.Now()
	}
	c.mu.Unlock()

	// Check for client ID in multi-client mode
	clientID := r.Header.Get("X-Gasoline-Client")

	var queries any
	if clientID != "" {
		queries = c.GetPendingQueriesForClient(clientID)
	} else {
		queries = c.GetPendingQueries()
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"queries": queries,
		"count": func() int {
			if q, ok := queries.([]PendingQueryResponse); ok {
				return len(q)
			}
			return 0
		}(),
	})
}

// HandlePilotStatus returns AI Pilot status
func (c *Capture) HandlePilotStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	c.mu.RLock()
	pilotEnabled := c.ext.pilotEnabled
	c.mu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"enabled": pilotEnabled,
	})
}

// HandleDOMResult processes DOM query results from extension.
// Extension posts results after executing DOM queries.
func (c *Capture) HandleDOMResult(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		fmt.Fprintf(os.Stderr, "[gasoline] HandleDOMResult: Method not allowed - %s\n", r.Method)
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var body struct {
		ID       string          `json:"id"`
		Result   json.RawMessage `json:"result"`
		ClientID string          `json:"client_id,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		fmt.Fprintf(os.Stderr, "[gasoline] HandleDOMResult: Invalid JSON - %v\n", err)
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid JSON"})
		return
	}

	if body.ID == "" {
		fmt.Fprintf(os.Stderr, "[gasoline] HandleDOMResult: Missing query ID\n")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Missing query ID"})
		return
	}

	// Store result
	c.SetQueryResultWithClient(body.ID, body.Result, body.ClientID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"status": "ok",
	})
}

// HandleA11yResult processes accessibility audit results from extension.
func (c *Capture) HandleA11yResult(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		fmt.Fprintf(os.Stderr, "[gasoline] HandleA11yResult: Method not allowed - %s\n", r.Method)
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var body struct {
		ID       string          `json:"id"`
		Result   json.RawMessage `json:"result"`
		ClientID string          `json:"client_id,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		fmt.Fprintf(os.Stderr, "[gasoline] HandleA11yResult: Invalid JSON - %v\n", err)
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid JSON"})
		return
	}

	if body.ID != "" {
		c.SetQueryResultWithClient(body.ID, body.Result, body.ClientID)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"status": "ok",
	})
}

// HandleStateResult processes page state snapshots from extension.
func (c *Capture) HandleStateResult(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		fmt.Fprintf(os.Stderr, "[gasoline] HandleStateResult: Method not allowed - %s\n", r.Method)
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var body struct {
		ID       string          `json:"id"`
		Result   json.RawMessage `json:"result"`
		ClientID string          `json:"client_id,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		fmt.Fprintf(os.Stderr, "[gasoline] HandleStateResult: Invalid JSON - %v\n", err)
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid JSON"})
		return
	}

	if body.ID != "" {
		c.SetQueryResultWithClient(body.ID, body.Result, body.ClientID)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"status": "ok",
	})
}

// HandleExecuteResult processes script execution results from extension.
// Accepts both query_id (as "id") and correlation_id for async command tracking.
func (c *Capture) HandleExecuteResult(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		fmt.Fprintf(os.Stderr, "[gasoline] HandleExecuteResult: Method not allowed - %s\n", r.Method)
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var body struct {
		ID            string          `json:"id"`
		CorrelationID string          `json:"correlation_id"`
		Status        string          `json:"status"`
		Result        json.RawMessage `json:"result"`
		Error         string          `json:"error"`
		ClientID      string          `json:"client_id,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		fmt.Fprintf(os.Stderr, "[gasoline] HandleExecuteResult: Invalid JSON - %v\n", err)
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid JSON"})
		return
	}

	// Handle correlation_id for async commands (execute_js, browser actions)
	if body.CorrelationID != "" {
		c.CompleteCommand(body.CorrelationID, body.Result, body.Error)
	}

	// Handle query_id for synchronous query results
	if body.ID != "" {
		c.SetQueryResultWithClient(body.ID, body.Result, body.ClientID)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"status": "ok",
	})
}

// HandleHighlightResult processes element highlight results from extension.
func (c *Capture) HandleHighlightResult(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		fmt.Fprintf(os.Stderr, "[gasoline] HandleHighlightResult: Method not allowed - %s\n", r.Method)
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var body struct {
		ID       string          `json:"id"`
		Result   json.RawMessage `json:"result"`
		ClientID string          `json:"client_id,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		fmt.Fprintf(os.Stderr, "[gasoline] HandleHighlightResult: Invalid JSON - %v\n", err)
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid JSON"})
		return
	}

	if body.ID != "" {
		c.SetQueryResultWithClient(body.ID, body.Result, body.ClientID)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"status": "ok",
	})
}

// HandleEnhancedActions handles enhanced action events.
// POST: receives and stores actions from the extension
// GET: returns stored actions
func (c *Capture) HandleEnhancedActions(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "POST":
		r.Body = http.MaxBytesReader(w, r.Body, maxExtensionPostBody)
		var payload struct {
			Actions []EnhancedAction `json:"actions"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			fmt.Fprintf(os.Stderr, "[gasoline] HandleEnhancedActions: Invalid JSON - %v\n", err)
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "Invalid JSON"})
			return
		}
		c.AddEnhancedActions(payload.Actions)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"status": "ok",
			"count":  len(payload.Actions),
		})
	case "GET":
		actions := c.GetAllEnhancedActions()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"actions": actions,
			"count":   len(actions),
		})
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

// HandleRecordingStorage handles recording storage management.
// GET: returns storage info
// DELETE: deletes a recording (requires recording_id query param)
// POST: recalculates storage usage
func (c *Capture) HandleRecordingStorage(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		info, err := c.GetStorageInfo()
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(info)
	case "DELETE":
		recordingID := r.URL.Query().Get("recording_id")
		if recordingID == "" {
			fmt.Fprintf(os.Stderr, "[gasoline] HandleRecordingStorage: Missing recording_id query parameter\n")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "Missing recording_id query parameter"})
			return
		}
		err := c.DeleteRecording(recordingID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "[gasoline] HandleRecordingStorage: Failed to delete recording %s - %v\n", recordingID, err)
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"status":  "ok",
			"deleted": recordingID,
		})
	case "POST":
		err := c.RecalculateStorageUsed()
		if err != nil {
			fmt.Fprintf(os.Stderr, "[gasoline] HandleRecordingStorage: Failed to recalculate storage - %v\n", err)
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		info, err := c.GetStorageInfo()
		if err != nil {
			fmt.Fprintf(os.Stderr, "[gasoline] HandleRecordingStorage: Failed to get storage info - %v\n", err)
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"status":  "ok",
			"storage": info,
		})
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

// HandlePerformanceSnapshots handles performance data snapshots.
// POST: receives and stores snapshots from the extension
// GET: returns stored snapshots
func (c *Capture) HandlePerformanceSnapshots(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "POST":
		r.Body = http.MaxBytesReader(w, r.Body, maxExtensionPostBody)
		var payload struct {
			Snapshots []PerformanceSnapshot `json:"snapshots"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "Invalid JSON"})
			return
		}
		c.AddPerformanceSnapshots(payload.Snapshots)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"status": "ok",
			"count":  len(payload.Snapshots),
		})
	case "GET":
		snapshots := c.GetPerformanceSnapshots()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"snapshots": snapshots,
			"count":     len(snapshots),
		})
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}
