// Purpose: Implements interact tool handlers and browser action orchestration.
// Docs: docs/features/feature/interact-explore/index.md

// tools_interact_draw.go — MCP interact handler for draw_mode_start action.
package main

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/dev-console/dev-console/internal/queries"
)

// handleDrawModeStart queues a draw_mode query for the extension to activate draw mode.
func (h *ToolHandler) handleDrawModeStart(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		TabID   int    `json:"tab_id,omitempty"`
		Session string `json:"session,omitempty"`
	}
	if len(args) > 0 {
		lenientUnmarshal(args, &params)
	}

	if !h.capture.IsPilotEnabled() {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrCodePilotDisabled, "AI Web Pilot is disabled", "Enable AI Web Pilot in the extension popup")}
	}

	correlationID := fmt.Sprintf("draw_%d_%d", time.Now().UnixNano(), randomInt63())

	queryParams := map[string]string{"action": "start"}
	if params.Session != "" {
		queryParams["session"] = params.Session
	}
	// Error impossible: map contains only string values
	queryParamsJSON, _ := json.Marshal(queryParams)

	query := queries.PendingQuery{
		Type:          "draw_mode",
		Params:        queryParamsJSON,
		TabID:         params.TabID,
		CorrelationID: correlationID,
	}
	h.capture.CreatePendingQueryWithTimeout(query, queries.AsyncCommandTimeout, req.ClientID)

	// Mark draw started AFTER the query is queued, so WaitForSession's timestamp
	// baseline is never set before the command that triggers the session exists.
	h.annotationStore.MarkDrawStarted()

	// Record AI action
	h.recordAIAction("draw_mode_start", "", nil)

	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Draw mode activated", map[string]any{
		"status":         "queued",
		"correlation_id": correlationID,
		"message":        "Draw mode activation queued. The user can now draw annotations on the page. Use analyze({what: 'annotations', wait: true}) to block until the user finishes drawing.",
	})}
}
