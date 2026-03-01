// tools_observe_inbox.go — observe({what: "inbox"}) handler and push piggyback.
package main

import (
	"encoding/json"
	"fmt"
)

// toolObserveInbox drains the push inbox and returns pending events.
func (h *ToolHandler) toolObserveInbox(req JSONRPCRequest, _ json.RawMessage) JSONRPCResponse {
	if h.server.pushInbox == nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Push inbox empty", map[string]any{
			"events": []any{},
			"count":  0,
		})}
	}

	events := h.server.pushInbox.DrainAll()
	if events == nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Push inbox empty", map[string]any{
			"events": []any{},
			"count":  0,
		})}
	}

	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Push inbox drained", map[string]any{
		"events": events,
		"count":  len(events),
	})}
}

// appendPushPiggyback adds a hint to any tool response when the inbox has pending events.
func (h *ToolHandler) appendPushPiggyback(resp JSONRPCResponse) JSONRPCResponse {
	if h.server.pushInbox == nil {
		return resp
	}

	count := h.server.pushInbox.Len()
	if count == 0 {
		return resp
	}

	var result MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return resp
	}

	hint := fmt.Sprintf("\n\n_pending_push: %d event(s) in inbox. Use observe({what: \"inbox\"}) to retrieve.", count)
	result.Content = append(result.Content, MCPContentBlock{
		Type: "text",
		Text: hint,
	})

	resultJSON, _ := json.Marshal(result)
	resp.Result = json.RawMessage(resultJSON)
	return resp
}
