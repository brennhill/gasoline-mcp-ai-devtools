// tools_observe_inbox.go — Push piggyback helper (delegates to toolobserve package).
package main

import (
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolobserve"
)

// appendPushPiggyback drains the push inbox and inlines events into any tool response.
// Screenshots are delivered as image content blocks so the LLM sees them immediately.
func (h *ToolHandler) appendPushPiggyback(resp JSONRPCResponse) JSONRPCResponse {
	return toolobserve.AppendPushPiggyback(h, resp)
}
