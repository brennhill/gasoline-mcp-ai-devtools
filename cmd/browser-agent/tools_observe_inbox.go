// tools_observe_inbox.go — Push piggyback helper (delegates to toolobserve package).
// The inbox handler itself is now in cmd/browser-agent/internal/toolobserve/inbox.go.
package main

import (
	"encoding/json"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolobserve"
)

// toolObserveInbox delegates to the extracted toolobserve.HandleInbox handler.
func (h *ToolHandler) toolObserveInbox(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	return toolobserve.HandleInbox(h, req, args)
}

// appendPushPiggyback drains the push inbox and inlines events into any tool response.
// Screenshots are delivered as image content blocks so the LLM sees them immediately.
func (h *ToolHandler) appendPushPiggyback(resp JSONRPCResponse) JSONRPCResponse {
	return toolobserve.AppendPushPiggyback(h, resp)
}
