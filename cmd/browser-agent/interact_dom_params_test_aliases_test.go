// Purpose: Test alias for DOM params helper that moved to internal/toolinteract.

package main

import (
	"encoding/json"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolinteract"
)

func parseDOMPrimitiveParams(args json.RawMessage) (toolinteract.DOMPrimitiveParams, error) {
	return toolinteract.ParseDOMPrimitiveParams(args)
}

func validateDOMActionParams(req JSONRPCRequest, action, text, value, name string) (JSONRPCResponse, bool) {
	return toolinteract.ValidateDOMActionParams(req, action, text, value, name)
}
