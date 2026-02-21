// bridge_fastpath.go — Fast-path telemetry, counters, and static response handling.
package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/dev-console/dev-console/internal/bridge"
	statecfg "github.com/dev-console/dev-console/internal/state"
)

type bridgeFastPathResourceReadCounters struct {
	mu      sync.Mutex
	success int64
	failure int64
}

var fastPathResourceReadCounters bridgeFastPathResourceReadCounters

func recordFastPathResourceRead(uri string, success bool, errorCode int) {
	fastPathResourceReadCounters.mu.Lock()
	defer fastPathResourceReadCounters.mu.Unlock()
	if success {
		fastPathResourceReadCounters.success++
	} else {
		fastPathResourceReadCounters.failure++
	}
	appendFastPathResourceReadTelemetry(uri, success, errorCode, fastPathResourceReadCounters.success, fastPathResourceReadCounters.failure)
}

func snapshotFastPathResourceReadCounters() (success int64, failure int64) {
	fastPathResourceReadCounters.mu.Lock()
	defer fastPathResourceReadCounters.mu.Unlock()
	return fastPathResourceReadCounters.success, fastPathResourceReadCounters.failure
}

func resetFastPathResourceReadCounters() {
	fastPathResourceReadCounters.mu.Lock()
	defer fastPathResourceReadCounters.mu.Unlock()
	fastPathResourceReadCounters.success = 0
	fastPathResourceReadCounters.failure = 0
}

func fastPathResourceReadLogPath() (string, error) {
	return statecfg.InRoot("logs", "bridge-fastpath-resource-read.jsonl")
}

func appendFastPathResourceReadTelemetry(uri string, success bool, errorCode int, successCount int64, failureCount int64) {
	path, err := fastPathResourceReadLogPath()
	if err != nil {
		return
	}
	if mkErr := os.MkdirAll(filepath.Dir(path), 0o750); mkErr != nil {
		return
	}
	entry := map[string]any{
		"timestamp":      time.Now().UTC().Format(time.RFC3339Nano),
		"event":          "bridge_fastpath_resources_read",
		"uri":            uri,
		"success":        success,
		"error_code":     errorCode,
		"success_count":  successCount,
		"failure_count":  failureCount,
		"pid":            os.Getpid(),
		"bridge_version": version,
	}
	line, marshalErr := json.Marshal(entry)
	if marshalErr != nil {
		return
	}
	// #nosec G304 -- path is deterministic under state root
	f, openErr := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if openErr != nil {
		return
	}
	defer func() { _ = f.Close() }()
	_, _ = f.Write(line)
	_, _ = f.Write([]byte("\n"))
}

type bridgeFastPathCounters struct {
	mu      sync.Mutex
	success int
	failure int
}

var fastPathCounters bridgeFastPathCounters

func resetFastPathCounters() {
	fastPathCounters.mu.Lock()
	fastPathCounters.success = 0
	fastPathCounters.failure = 0
	fastPathCounters.mu.Unlock()
}

func fastPathTelemetryLogPath() (string, error) {
	return statecfg.InRoot("logs", "bridge-fastpath-events.jsonl")
}

func recordFastPathEvent(method string, success bool, errorCode int) {
	fastPathCounters.mu.Lock()
	if success {
		fastPathCounters.success++
	} else {
		fastPathCounters.failure++
	}
	successCount := fastPathCounters.success
	failureCount := fastPathCounters.failure
	fastPathCounters.mu.Unlock()

	path, err := fastPathTelemetryLogPath()
	if err != nil {
		return
	}
	// #nosec G301 -- runtime state directory for local diagnostics.
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return
	}
	event := map[string]any{
		"timestamp":     time.Now().UTC().Format(time.RFC3339Nano),
		"event":         "bridge_fastpath_method",
		"method":        method,
		"success":       success,
		"error_code":    errorCode,
		"success_count": successCount,
		"failure_count": failureCount,
		"pid":           os.Getpid(),
		"version":       version,
	}
	payload, err := json.Marshal(event)
	if err != nil {
		return
	}
	// #nosec G304 -- deterministic diagnostics path rooted in runtime state directory.
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600) // nosemgrep: go_filesystem_rule-fileread -- local diagnostics log append
	if err != nil {
		return
	}
	defer func() { _ = f.Close() }()
	_, _ = f.Write(append(payload, '\n'))
}

// fastPathResponses maps MCP methods to their static JSON result bodies.
// Methods in this map are handled without waiting for the daemon.
var fastPathResponses = map[string]string{
	"ping":         `{}`,
	"prompts/list": `{"prompts":[]}`,
}

// sendFastResponse marshals and sends a JSON-RPC response for the fast path.
func sendFastResponse(id any, result json.RawMessage, framing bridge.StdioFraming) {
	resp := JSONRPCResponse{JSONRPC: "2.0", ID: id, Result: result}
	// Error impossible: simple struct with no circular refs or unsupported types
	respJSON, _ := json.Marshal(resp)
	writeMCPPayload(respJSON, framing)
}

func sendFastError(id any, code int, message string, framing bridge.StdioFraming) {
	resp := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error:   &JSONRPCError{Code: code, Message: message},
	}
	respJSON, _ := json.Marshal(resp)
	writeMCPPayload(respJSON, framing)
}

// handleFastPath handles MCP methods that don't require the daemon.
// Returns true if the method was handled.
func handleFastPath(req JSONRPCRequest, toolsList []MCPTool, framing bridge.StdioFraming) bool {
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
		result := map[string]any{
			"protocolVersion": negotiateProtocolVersion(req.Params),
			"serverInfo":      map[string]any{"name": "gasoline", "version": version},
			"capabilities":    map[string]any{"tools": map[string]any{}, "resources": map[string]any{}},
			"instructions":    serverInstructions,
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
		result := MCPResourcesListResult{Resources: mcpResources()}
		resultJSON, _ := json.Marshal(result)
		sendFastResponse(req.ID, resultJSON, framing)
		return true
	case "resources/templates/list":
		result := MCPResourceTemplatesListResult{ResourceTemplates: mcpResourceTemplates()}
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
		canonicalURI, text, ok := resolveResourceContent(params.URI)
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
