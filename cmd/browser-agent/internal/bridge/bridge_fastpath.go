// bridge_fastpath.go -- Handles bridge fast-path responses for MCP resource reads and tools/list without round-tripping to the daemon.
// Why: Reduces latency for high-frequency read-only MCP calls by serving them directly from the bridge process.
// Docs: docs/features/feature/bridge-restart/index.md

package bridge

import (
	"encoding/json"

	internbridge "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/bridge"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

// fastPathResponses maps MCP methods to their static JSON result bodies.
// Methods in this map are handled without waiting for the daemon.
var fastPathResponses = map[string]string{
	"ping":         `{}`,
	"prompts/list": `{"prompts":[]}`,
}

// sendFastResponse marshals and sends a JSON-RPC response for the fast path.
func sendFastResponse(id any, result json.RawMessage, framing internbridge.StdioFraming) {
	resp := mcp.JSONRPCResponse{JSONRPC: mcp.JSONRPCVersion, ID: id, Result: result}
	// Error impossible: simple struct with no circular refs or unsupported types
	respJSON, _ := json.Marshal(resp)
	deps.WriteMCPPayload(respJSON, framing)
}

func sendFastError(id any, code int, message string, framing internbridge.StdioFraming) {
	resp := mcp.JSONRPCResponse{
		JSONRPC: mcp.JSONRPCVersion,
		ID:      id,
		Error:   &mcp.JSONRPCError{Code: code, Message: message},
	}
	respJSON, _ := json.Marshal(resp)
	deps.WriteMCPPayload(respJSON, framing)
}

// handleFastPath handles MCP methods that don't require the daemon.
// Returns true if the method was handled.
func handleFastPath(req mcp.JSONRPCRequest, toolsList []mcp.MCPTool, framing internbridge.StdioFraming) bool {
	if req.HasInvalidID() {
		sendBridgeError(nil, -32600, "Invalid Request: id must be string or number when present", framing)
		return true
	}

	// JSON-RPC notifications are fire-and-forget; never respond on stdio.
	if !req.HasID() {
		return true
	}

	switch req.Method {
	case "initialize":
		// Extract client capabilities for push delivery pipeline
		caps := deps.ExtractClientCapabilities(req.Params)
		deps.SetPushClientCapabilities(caps)
		deps.StoreBridgeFraming(framing)

		result := map[string]any{
			"protocolVersion": deps.NegotiateProtocolVersion(req.Params),
			"serverInfo":      map[string]any{"name": deps.MCPServerName, "version": deps.Version},
			"capabilities":    map[string]any{"tools": map[string]any{}, "resources": map[string]any{}},
			"instructions":    deps.ServerInstructions,
		}
		// Error impossible: map contains only primitive types and nested maps
		resultJSON, _ := json.Marshal(result)
		sendFastResponse(req.ID, resultJSON, framing)
		recordFastPathEvent(req.Method, true, 0)
		return true

	case "initialized":
		if req.HasID() {
			sendFastResponse(req.ID, json.RawMessage(`{}`), framing)
			recordFastPathEvent(req.Method, true, 0)
		}
		return true

	case "tools/list":
		result := map[string]any{"tools": toolsList}
		// Error impossible: map contains only serializable tool definitions
		resultJSON, _ := json.Marshal(result)
		sendFastResponse(req.ID, resultJSON, framing)
		recordFastPathEvent(req.Method, true, 0)
		return true

	case "resources/list":
		result := mcp.MCPResourcesListResult{Resources: deps.MCPResources()}
		resultJSON, _ := json.Marshal(result)
		sendFastResponse(req.ID, resultJSON, framing)
		return true
	case "resources/templates/list":
		result := mcp.MCPResourceTemplatesListResult{ResourceTemplates: deps.MCPResourceTemplates()}
		resultJSON, _ := json.Marshal(result)
		sendFastResponse(req.ID, resultJSON, framing)
		return true
	case "resources/read":
		var params struct {
			URI string `json:"uri"`
		}
		if err := json.Unmarshal(req.Params, &params); err != nil {
			recordFastPathResourceRead("", false, -32602)
			recordFastPathEvent(req.Method, false, -32602)
			sendFastError(req.ID, -32602, "Invalid params: "+err.Error(), framing)
			return true
		}
		canonicalURI, text, ok := deps.ResolveResourceContent(params.URI)
		if !ok {
			recordFastPathResourceRead(params.URI, false, -32002)
			recordFastPathEvent(req.Method, false, -32002)
			sendFastError(req.ID, -32002, "Resource not found: "+params.URI, framing)
			return true
		}
		recordFastPathResourceRead(params.URI, true, 0)
		recordFastPathEvent(req.Method, true, 0)
		result := map[string]any{
			"contents": []map[string]any{
				{
					"uri":      canonicalURI,
					"mimeType": "text/markdown",
					"text":     text,
				},
			},
		}
		resultJSON, _ := json.Marshal(result)
		sendFastResponse(req.ID, resultJSON, framing)
		return true
	}

	if staticResult, ok := fastPathResponses[req.Method]; ok {
		sendFastResponse(req.ID, json.RawMessage(staticResult), framing)
		recordFastPathEvent(req.Method, true, 0)
		return true
	}

	return false
}
