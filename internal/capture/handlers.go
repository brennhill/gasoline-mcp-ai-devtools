// Purpose: Implements HTTP ingest handlers for network, query, and telemetry capture endpoints.
// Why: Provides the daemon-side ingress boundary that validates and routes extension event batches.
// Docs: docs/features/feature/backend-log-streaming/index.md
// Docs: docs/features/feature/query-service/index.md

package capture

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/util"
)

// HandleNetworkBodies handles POST /network-bodies from the extension.
// Reads go through GET /telemetry?type=network_bodies.
func (c *Capture) HandleNetworkBodies(w http.ResponseWriter, r *http.Request) {
	if !util.RequireMethod(w, r, "POST") {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxExtensionPostBody)
	var payload struct {
		Bodies []NetworkBody `json:"bodies"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		fmt.Fprintf(os.Stderr, "[Kaboom] HandleNetworkBodies: Invalid JSON - %v\n", err)
		util.JSONResponse(w, http.StatusBadRequest, map[string]string{"error": "Invalid JSON"})
		return
	}
	c.AddNetworkBodies(payload.Bodies)
	util.JSONResponse(w, http.StatusOK, map[string]any{
		"status": "ok",
		"count":  len(payload.Bodies),
	})
}

// HandleNetworkWaterfall handles POST /network-waterfall from the extension.
// Reads go through GET /telemetry?type=network_waterfall.
func (c *Capture) HandleNetworkWaterfall(w http.ResponseWriter, r *http.Request) {
	if !util.RequireMethod(w, r, "POST") {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxExtensionPostBody)
	var payload NetworkWaterfallPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		fmt.Fprintf(os.Stderr, "[Kaboom] HandleNetworkWaterfall: Invalid JSON - %v\n", err)
		util.JSONResponse(w, http.StatusBadRequest, map[string]string{"error": "Invalid JSON"})
		return
	}
	c.AddNetworkWaterfallEntries(payload.Entries, payload.PageURL)
	util.JSONResponse(w, http.StatusOK, map[string]any{
		"status": "ok",
		"count":  len(payload.Entries),
	})
}

// HandleQueryResult processes all query/command results from the extension.
// Unified handler replacing separate dom-result, a11y-result, state-result,
// execute-result, and highlight-result endpoints.
func (c *Capture) HandleQueryResult(w http.ResponseWriter, r *http.Request) {
	if !util.RequireMethod(w, r, "POST") {
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
		fmt.Fprintf(os.Stderr, "[Kaboom] HandleQueryResult: Invalid JSON - %v\n", err)
		util.JSONResponse(w, http.StatusBadRequest, map[string]string{"error": "Invalid JSON"})
		return
	}

	// Handle query_id for synchronous query results
	if body.ID != "" {
		if body.CorrelationID != "" {
			// Correlated async commands carry explicit lifecycle status below.
			// Do not force "complete" from query-id bookkeeping.
			c.SetQueryResultWithClientNoCommandComplete(body.ID, body.Result, body.ClientID)
		} else {
			c.SetQueryResultWithClient(body.ID, body.Result, body.ClientID)
		}
	}

	// Handle correlation_id for async commands (execute_js, browser actions)
	if body.CorrelationID != "" {
		c.ApplyCommandResult(body.CorrelationID, body.Status, body.Result, body.Error)
	}

	util.JSONResponse(w, http.StatusOK, map[string]any{
		"status": "ok",
	})
}

// HandleEnhancedActions handles POST /enhanced-actions from the extension.
// Reads go through GET /telemetry?type=actions.
func (c *Capture) HandleEnhancedActions(w http.ResponseWriter, r *http.Request) {
	if !util.RequireMethod(w, r, "POST") {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxExtensionPostBody)
	var payload struct {
		Actions []EnhancedAction `json:"actions"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		fmt.Fprintf(os.Stderr, "[Kaboom] HandleEnhancedActions: Invalid JSON - %v\n", err)
		util.JSONResponse(w, http.StatusBadRequest, map[string]string{"error": "Invalid JSON"})
		return
	}
	c.AddEnhancedActions(payload.Actions)
	util.JSONResponse(w, http.StatusOK, map[string]any{
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
		util.JSONResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
	}
}

func (c *Capture) handleStorageGet(w http.ResponseWriter) {
	info, err := c.GetStorageInfo()
	if err != nil {
		util.JSONResponse(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	util.JSONResponse(w, http.StatusOK, info)
}

func (c *Capture) handleStorageDelete(w http.ResponseWriter, r *http.Request) {
	recordingID := r.URL.Query().Get("recording_id")
	if recordingID == "" {
		fmt.Fprintf(os.Stderr, "[Kaboom] HandleRecordingStorage: Missing recording_id query parameter\n")
		util.JSONResponse(w, http.StatusBadRequest, map[string]string{"error": "Missing recording_id query parameter"})
		return
	}
	if err := c.DeleteRecording(recordingID); err != nil {
		fmt.Fprintf(os.Stderr, "[Kaboom] HandleRecordingStorage: Failed to delete recording %s - %v\n", recordingID, err)
		util.JSONResponse(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}
	util.JSONResponse(w, http.StatusOK, map[string]any{"status": "ok", "deleted": recordingID})
}

func (c *Capture) handleStorageRecalculate(w http.ResponseWriter) {
	if err := c.RecalculateStorageUsed(); err != nil {
		fmt.Fprintf(os.Stderr, "[Kaboom] HandleRecordingStorage: Failed to recalculate storage - %v\n", err)
		util.JSONResponse(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	info, err := c.GetStorageInfo()
	if err != nil {
		fmt.Fprintf(os.Stderr, "[Kaboom] HandleRecordingStorage: Failed to get storage info - %v\n", err)
		util.JSONResponse(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	util.JSONResponse(w, http.StatusOK, map[string]any{"status": "ok", "storage": info})
}

// HandlePerformanceSnapshots handles POST /performance-snapshots from the extension.
// Reads go through GET /telemetry?type=performance_snapshots.
func (c *Capture) HandlePerformanceSnapshots(w http.ResponseWriter, r *http.Request) {
	if !util.RequireMethod(w, r, "POST") {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxExtensionPostBody)
	var payload struct {
		Snapshots []PerformanceSnapshot `json:"snapshots"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		util.JSONResponse(w, http.StatusBadRequest, map[string]string{"error": "Invalid JSON"})
		return
	}
	c.AddPerformanceSnapshots(payload.Snapshots)
	util.JSONResponse(w, http.StatusOK, map[string]any{
		"status": "ok",
		"count":  len(payload.Snapshots),
	})
}
