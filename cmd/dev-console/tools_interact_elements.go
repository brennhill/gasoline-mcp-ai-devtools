// tools_interact_elements.go — Element indexing for interact tool.
// Implements list_interactive with indexed element store and index→selector resolution.
package main

import (
	"encoding/json"
	"strings"

	"github.com/dev-console/dev-console/internal/queries"
	act "github.com/dev-console/dev-console/internal/tools/interact"
)

func (h *ToolHandler) handleListInteractive(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		TabID       int  `json:"tab_id,omitempty"`
		VisibleOnly bool `json:"visible_only,omitempty"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")}
	}

	if resp, blocked := h.requirePilot(req); blocked {
		return resp
	}

	args = normalizeDOMActionArgs(args, "list_interactive")

	correlationID := newCorrelationID("dom_list")

	query := queries.PendingQuery{
		Type:          "dom_action",
		Params:        args,
		TabID:         params.TabID,
		CorrelationID: correlationID,
	}
	h.capture.CreatePendingQueryWithTimeout(query, queries.AsyncCommandTimeout, req.ClientID)

	h.recordAIAction("dom_list_interactive", "", nil)

	resp := h.MaybeWaitForCommand(req, correlationID, args, "list_interactive queued")

	// Post-process: extract elements from result and build index→selector store
	h.buildElementIndexFromResponse(resp)

	return resp
}

// buildElementIndexFromResponse parses list_interactive results and populates elementIndexStore.
func (h *ToolHandler) buildElementIndexFromResponse(resp JSONRPCResponse) {
	var result MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil || result.IsError {
		return
	}

	// Extract the result JSON from the command response
	for _, block := range result.Content {
		// Try to find JSON in the text content
		idx := strings.Index(block.Text, "{")
		if idx < 0 {
			continue
		}
		jsonStr := block.Text[idx:]

		var data map[string]any
		if json.Unmarshal([]byte(jsonStr), &data) != nil {
			continue
		}

		// Look for result.elements or result.result.elements
		elements := extractElementList(data)
		if elements == nil {
			continue
		}

		h.elementIndexMu.Lock()
		h.elementIndexStore = make(map[int]string, len(elements))
		for _, elem := range elements {
			elemMap, ok := elem.(map[string]any)
			if !ok {
				continue
			}
			indexVal, _ := elemMap["index"].(float64)
			selector, _ := elemMap["selector"].(string)
			if selector != "" {
				h.elementIndexStore[int(indexVal)] = selector
			}
		}
		h.elementIndexMu.Unlock()
		return
	}
}

// extractElementList — delegated to internal/tools/interact package.
var extractElementList = act.ExtractElementList

// resolveIndexToSelector looks up a selector from the element index store.
func (h *ToolHandler) resolveIndexToSelector(index int) (string, bool) {
	h.elementIndexMu.RLock()
	defer h.elementIndexMu.RUnlock()
	sel, ok := h.elementIndexStore[index]
	return sel, ok
}
