// Purpose: Owns handlers.go runtime behavior and integration logic.
// Docs: docs/features/feature/backend-log-streaming/index.md

// handlers.go — HTTP handlers for extension ↔ server communication
// Implements endpoints for pending queries, query results, and extension status.
// Part of the async queue-and-poll architecture (see internal/capture/queries.go).
package capture

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
)

// HandleNetworkBodies handles POST /network-bodies from the extension.
// Reads go through GET /telemetry?type=network_bodies.
func (c *Capture) HandleNetworkBodies(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxExtensionPostBody)
	var payload struct {
		Bodies []NetworkBody `json:"bodies"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		fmt.Fprintf(os.Stderr, "[gasoline] HandleNetworkBodies: Invalid JSON - %v\n", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Invalid JSON"})
		return
	}
	c.AddNetworkBodies(payload.Bodies)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"status": "ok",
		"count":  len(payload.Bodies),
	})
}

// HandleNetworkWaterfall handles POST /network-waterfall from the extension.
// Reads go through GET /telemetry?type=network_waterfall.
func (c *Capture) HandleNetworkWaterfall(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxExtensionPostBody)
	var payload NetworkWaterfallPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		fmt.Fprintf(os.Stderr, "[gasoline] HandleNetworkWaterfall: Invalid JSON - %v\n", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Invalid JSON"})
		return
	}
	c.AddNetworkWaterfallEntries(payload.Entries, payload.PageURL)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"status": "ok",
		"count":  len(payload.Entries),
	})
}

// HandleQueryResult processes all query/command results from the extension.
// Unified handler replacing separate dom-result, a11y-result, state-result,
// execute-result, and highlight-result endpoints.
func (c *Capture) HandleQueryResult(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxExtensionPostBody)
	var body struct {
		ID            string          `json:"id"`
		CorrelationID string          `json:"correlation_id"`
		Status        string          `json:"status"`
		Result        json.RawMessage `json:"result"`
		Error         string          `json:"error"`
		ClientID      string          `json:"client_id,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		fmt.Fprintf(os.Stderr, "[gasoline] HandleQueryResult: Invalid JSON - %v\n", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Invalid JSON"})
		return
	}

	if body.ID != "" {
		mappedCorrelationID := c.SetQueryResultOnly(body.ID, body.Result, body.ClientID)
		correlationID := body.CorrelationID
		if correlationID == "" {
			correlationID = mappedCorrelationID
		}
		if correlationID != "" {
			c.CompleteCommandWithStatus(correlationID, body.Result, body.Status, body.Error)
		}
	} else if body.CorrelationID != "" {
		c.CompleteCommandWithStatus(body.CorrelationID, body.Result, body.Status, body.Error)
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"status": "ok",
	})
}

// HandleEnhancedActions handles POST /enhanced-actions from the extension.
// Reads go through GET /telemetry?type=actions.
func (c *Capture) HandleEnhancedActions(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxExtensionPostBody)
	var payload struct {
		Actions []EnhancedAction `json:"actions"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		fmt.Fprintf(os.Stderr, "[gasoline] HandleEnhancedActions: Invalid JSON - %v\n", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Invalid JSON"})
		return
	}
	c.AddEnhancedActions(payload.Actions)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"status": "ok",
		"count":  len(payload.Actions),
	})
}

// HandleRecordingStorage handles recording storage management.
// GET: returns storage info
// DELETE: deletes a recording (requires recording_id query param)
// POST: recalculates storage usage
func (c *Capture) HandleRecordingStorage(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		c.handleStorageGet(w)
	case "DELETE":
		c.handleStorageDelete(w, r)
	case "POST":
		c.handleStorageRecalculate(w)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (c *Capture) handleStorageGet(w http.ResponseWriter) {
	info, err := c.GetStorageInfo()
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(info)
}

func (c *Capture) handleStorageDelete(w http.ResponseWriter, r *http.Request) {
	recordingID := r.URL.Query().Get("recording_id")
	if recordingID == "" {
		fmt.Fprintf(os.Stderr, "[gasoline] HandleRecordingStorage: Missing recording_id query parameter\n")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Missing recording_id query parameter"})
		return
	}
	if err := c.DeleteRecording(recordingID); err != nil {
		fmt.Fprintf(os.Stderr, "[gasoline] HandleRecordingStorage: Failed to delete recording %s - %v\n", recordingID, err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok", "deleted": recordingID})
}

func (c *Capture) handleStorageRecalculate(w http.ResponseWriter) {
	if err := c.RecalculateStorageUsed(); err != nil {
		fmt.Fprintf(os.Stderr, "[gasoline] HandleRecordingStorage: Failed to recalculate storage - %v\n", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	info, err := c.GetStorageInfo()
	if err != nil {
		fmt.Fprintf(os.Stderr, "[gasoline] HandleRecordingStorage: Failed to get storage info - %v\n", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok", "storage": info})
}

// HandlePerformanceSnapshots handles POST /performance-snapshots from the extension.
// Reads go through GET /telemetry?type=performance_snapshots.
func (c *Capture) HandlePerformanceSnapshots(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxExtensionPostBody)
	var payload struct {
		Snapshots []PerformanceSnapshot `json:"snapshots"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Invalid JSON"})
		return
	}
	c.AddPerformanceSnapshots(payload.Snapshots)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"status": "ok",
		"count":  len(payload.Snapshots),
	})
}
