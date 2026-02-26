// Purpose: Implements interact tool handlers and browser action orchestration.
// Why: Preserves deterministic browser action execution across agent workflows.
// Docs: docs/features/feature/interact-explore/index.md

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
		Limit       int  `json:"limit,omitempty"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")}
	}

	if resp, blocked := h.requirePilot(req); blocked {
		return resp
	}
	if resp, blocked := h.requireExtension(req); blocked {
		return resp
	}
	if resp, blocked := h.requireTabTracking(req); blocked {
		return resp
	}

	args = normalizeDOMActionArgs(args, "list_interactive")

	correlationID := newCorrelationID("dom_list")
	h.armEvidenceForCommand(correlationID, "list_interactive", args, req.ClientID)

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
	// IMPORTANT: index store is built from ALL elements before truncation
	h.buildElementIndexFromResponse(resp)

	if params.Limit > 0 {
		resp = truncateListInteractiveResponse(resp, params.Limit)
	}

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

// truncateListInteractiveResponse limits the elements array in a list_interactive response.
func truncateListInteractiveResponse(resp JSONRPCResponse, limit int) JSONRPCResponse {
	var result MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil || result.IsError {
		return resp
	}

	for i, block := range result.Content {
		idx := strings.Index(block.Text, "{")
		if idx < 0 {
			continue
		}
		jsonStr := block.Text[idx:]
		prefix := block.Text[:idx]

		var data map[string]any
		if json.Unmarshal([]byte(jsonStr), &data) != nil {
			continue
		}

		elements := act.ExtractElementList(data)
		if elements == nil || len(elements) <= limit {
			continue
		}

		total := len(elements)
		setNestedElements(data, elements[:limit])
		data["total"] = total
		data["truncated"] = true

		newJSON, err := json.Marshal(data)
		if err != nil {
			continue
		}
		result.Content[i].Text = prefix + string(newJSON)
		newResult, _ := json.Marshal(result)
		resp.Result = newResult
		return resp
	}

	return resp
}

// setNestedElements updates the elements array at whatever nesting level it was found.
func setNestedElements(data map[string]any, elements []any) {
	if _, ok := data["elements"]; ok {
		data["elements"] = elements
		return
	}
	if r, ok := data["result"].(map[string]any); ok {
		if _, ok := r["elements"]; ok {
			r["elements"] = elements
			return
		}
		if rr, ok := r["result"].(map[string]any); ok {
			if _, ok := rr["elements"]; ok {
				rr["elements"] = elements
				return
			}
		}
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
