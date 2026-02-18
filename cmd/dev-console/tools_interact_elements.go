// tools_interact_elements.go — Element indexing for interact tool.
// Implements list_interactive with indexed element store and index→selector resolution.
package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/dev-console/dev-console/internal/queries"
)

func (h *ToolHandler) handleListInteractive(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		TabID       int  `json:"tab_id,omitempty"`
		VisibleOnly bool `json:"visible_only,omitempty"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")}
	}

	if !h.capture.IsPilotEnabled() {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrCodePilotDisabled, "AI Web Pilot is disabled", "Enable AI Web Pilot in the extension popup", h.diagnosticHint())}
	}

	correlationID := fmt.Sprintf("dom_list_%d_%d", time.Now().UnixNano(), randomInt63())

	query := queries.PendingQuery{
		Type:          "dom_action",
		Params:        args,
		TabID:         params.TabID,
		CorrelationID: correlationID,
	}
	h.capture.CreatePendingQueryWithTimeout(query, queries.AsyncCommandTimeout, req.ClientID)

	h.recordAIAction("dom_list_interactive", "", nil)

	resp := h.maybeWaitForCommand(req, correlationID, args, "list_interactive queued")

	// Post-process: extract elements from result and build index→selector store
	h.buildElementIndexFromResponse(resp, req.ClientID)

	return resp
}

// buildElementIndexFromResponse parses list_interactive results and populates elementIndexStore
// for the given clientID, preventing concurrent clients from overwriting each other's index.
func (h *ToolHandler) buildElementIndexFromResponse(resp JSONRPCResponse, clientID string) {
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

		clientStore := make(map[int]string, len(elements))
		for _, elem := range elements {
			elemMap, ok := elem.(map[string]any)
			if !ok {
				continue
			}
			indexVal, _ := elemMap["index"].(float64)
			selector, _ := elemMap["selector"].(string)
			if selector != "" {
				clientStore[int(indexVal)] = selector
			}
		}

		h.elementIndexMu.Lock()
		if h.elementIndexStore == nil {
			h.elementIndexStore = make(map[string]map[int]string)
		}
		h.elementIndexStore[clientID] = clientStore
		h.elementIndexMu.Unlock()
		return
	}
}

// extractElementList walks nested result JSON to find the elements array.
func extractElementList(data map[string]any) []any {
	// Direct elements field
	if elems, ok := data["elements"].([]any); ok {
		return elems
	}
	// Nested in result field (json.Unmarshal into map[string]any always produces map[string]any)
	if resultData, ok := data["result"].(map[string]any); ok {
		if elems, ok := resultData["elements"].([]any); ok {
			return elems
		}
		// Recurse into nested result (command result wrapping)
		return extractElementList(resultData)
	}
	return nil
}

// resolveIndexToSelector looks up a selector from the element index store for a given client.
func (h *ToolHandler) resolveIndexToSelector(index int, clientID string) (string, bool) {
	h.elementIndexMu.RLock()
	defer h.elementIndexMu.RUnlock()
	clientStore, ok := h.elementIndexStore[clientID]
	if !ok {
		return "", false
	}
	sel, ok := clientStore[index]
	return sel, ok
}
