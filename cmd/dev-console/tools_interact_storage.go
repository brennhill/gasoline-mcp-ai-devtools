// tools_interact_storage.go — Granular storage/cookie mutation handlers for interact tool.
package main

import (
	"encoding/json"
	"fmt"

	"github.com/dev-console/dev-console/internal/queries"
)

var validStorageTypes = map[string]string{
	"localStorage":   "localStorage",
	"sessionStorage": "sessionStorage",
}

func (h *ToolHandler) handleSetStorage(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		StorageType string  `json:"storage_type"`
		Key         string  `json:"key"`
		Value       *string `json:"value"`
		TabID       int     `json:"tab_id,omitempty"`
		TimeoutMs   int     `json:"timeout_ms,omitempty"`
		World       string  `json:"world,omitempty"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")}
	}

	storageExpr, ok := validStorageTypes[params.StorageType]
	if !ok {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidParam, "Invalid 'storage_type' value: "+params.StorageType, "Use 'localStorage' or 'sessionStorage'", withParam("storage_type"))}
	}
	if params.Key == "" {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrMissingParam, "Required parameter 'key' is missing for set_storage action", "Add the 'key' parameter and call again", withParam("key"))}
	}
	if params.Value == nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrMissingParam, "Required parameter 'value' is missing for set_storage action", "Add the 'value' parameter and call again", withParam("value"))}
	}

	script := fmt.Sprintf(`(() => { try { %s.setItem(%s, %s); return { ok: true, action: "set_storage", storage_type: %s, key: %s }; } catch (e) { return { ok: false, error: String((e && e.message) || e) }; } })()`,
		storageExpr, jsQuote(params.Key), jsQuote(*params.Value), jsQuote(params.StorageType), jsQuote(params.Key))
	return h.queueExecuteScript(req, args, "storage_set", params.TabID, params.TimeoutMs, params.World, script, "set_storage", "set_storage queued")
}

func (h *ToolHandler) handleDeleteStorage(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		StorageType string `json:"storage_type"`
		Key         string `json:"key"`
		TabID       int    `json:"tab_id,omitempty"`
		TimeoutMs   int    `json:"timeout_ms,omitempty"`
		World       string `json:"world,omitempty"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")}
	}

	storageExpr, ok := validStorageTypes[params.StorageType]
	if !ok {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidParam, "Invalid 'storage_type' value: "+params.StorageType, "Use 'localStorage' or 'sessionStorage'", withParam("storage_type"))}
	}
	if params.Key == "" {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrMissingParam, "Required parameter 'key' is missing for delete_storage action", "Add the 'key' parameter and call again", withParam("key"))}
	}

	script := fmt.Sprintf(`(() => { try { %s.removeItem(%s); return { ok: true, action: "delete_storage", storage_type: %s, key: %s }; } catch (e) { return { ok: false, error: String((e && e.message) || e) }; } })()`,
		storageExpr, jsQuote(params.Key), jsQuote(params.StorageType), jsQuote(params.Key))
	return h.queueExecuteScript(req, args, "storage_del", params.TabID, params.TimeoutMs, params.World, script, "delete_storage", "delete_storage queued")
}

func (h *ToolHandler) handleClearStorage(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		StorageType string `json:"storage_type"`
		TabID       int    `json:"tab_id,omitempty"`
		TimeoutMs   int    `json:"timeout_ms,omitempty"`
		World       string `json:"world,omitempty"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")}
	}

	storageExpr, ok := validStorageTypes[params.StorageType]
	if !ok {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidParam, "Invalid 'storage_type' value: "+params.StorageType, "Use 'localStorage' or 'sessionStorage'", withParam("storage_type"))}
	}

	script := fmt.Sprintf(`(() => { try { %s.clear(); return { ok: true, action: "clear_storage", storage_type: %s }; } catch (e) { return { ok: false, error: String((e && e.message) || e) }; } })()`,
		storageExpr, jsQuote(params.StorageType))
	return h.queueExecuteScript(req, args, "storage_clear", params.TabID, params.TimeoutMs, params.World, script, "clear_storage", "clear_storage queued")
}

func (h *ToolHandler) handleSetCookie(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		Name      string  `json:"name"`
		Value     *string `json:"value"`
		Domain    string  `json:"domain,omitempty"`
		Path      string  `json:"path,omitempty"`
		TabID     int     `json:"tab_id,omitempty"`
		TimeoutMs int     `json:"timeout_ms,omitempty"`
		World     string  `json:"world,omitempty"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")}
	}
	if params.Name == "" {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrMissingParam, "Required parameter 'name' is missing for set_cookie action", "Add the 'name' parameter and call again", withParam("name"))}
	}
	if params.Value == nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrMissingParam, "Required parameter 'value' is missing for set_cookie action", "Add the 'value' parameter and call again", withParam("value"))}
	}

	cookie := params.Name + "=" + *params.Value
	if params.Path != "" {
		cookie += "; path=" + params.Path
	} else {
		cookie += "; path=/"
	}
	if params.Domain != "" {
		cookie += "; domain=" + params.Domain
	}

	script := fmt.Sprintf(`(() => { try { document.cookie = %s; return { ok: true, action: "set_cookie", name: %s }; } catch (e) { return { ok: false, error: String((e && e.message) || e) }; } })()`,
		jsQuote(cookie), jsQuote(params.Name))
	return h.queueExecuteScript(req, args, "cookie_set", params.TabID, params.TimeoutMs, params.World, script, "set_cookie", "set_cookie queued")
}

func (h *ToolHandler) handleDeleteCookie(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		Name      string `json:"name"`
		Domain    string `json:"domain,omitempty"`
		Path      string `json:"path,omitempty"`
		TabID     int    `json:"tab_id,omitempty"`
		TimeoutMs int    `json:"timeout_ms,omitempty"`
		World     string `json:"world,omitempty"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")}
	}
	if params.Name == "" {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrMissingParam, "Required parameter 'name' is missing for delete_cookie action", "Add the 'name' parameter and call again", withParam("name"))}
	}

	cookie := params.Name + "=; expires=Thu, 01 Jan 1970 00:00:00 GMT"
	if params.Path != "" {
		cookie += "; path=" + params.Path
	} else {
		cookie += "; path=/"
	}
	if params.Domain != "" {
		cookie += "; domain=" + params.Domain
	}

	script := fmt.Sprintf(`(() => { try { document.cookie = %s; return { ok: true, action: "delete_cookie", name: %s }; } catch (e) { return { ok: false, error: String((e && e.message) || e) }; } })()`,
		jsQuote(cookie), jsQuote(params.Name))
	return h.queueExecuteScript(req, args, "cookie_del", params.TabID, params.TimeoutMs, params.World, script, "delete_cookie", "delete_cookie queued")
}

func (h *ToolHandler) queueExecuteScript(
	req JSONRPCRequest,
	waitArgs json.RawMessage,
	correlationPrefix string,
	tabID, timeoutMs int,
	world, script, reason, queuedMsg string,
) JSONRPCResponse {
	if resp, blocked := h.requirePilot(req); blocked {
		return resp
	}

	if timeoutMs <= 0 {
		timeoutMs = 5000
	}
	if world == "" {
		world = "auto"
	}
	if !validWorldValues[world] {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidParam, "Invalid 'world' value: "+world, "Use 'auto' (default), 'main', or 'isolated'", withParam("world"))}
	}

	correlationID := newCorrelationID(correlationPrefix)
	execParams, _ := json.Marshal(map[string]any{
		"script":     script,
		"timeout_ms": timeoutMs,
		"world":      world,
		"reason":     reason,
	})

	query := queries.PendingQuery{
		Type:          "execute",
		Params:        execParams,
		TabID:         tabID,
		CorrelationID: correlationID,
	}
	h.capture.CreatePendingQueryWithTimeout(query, queries.AsyncCommandTimeout, req.ClientID)

	return h.MaybeWaitForCommand(req, correlationID, waitArgs, queuedMsg)
}

func jsQuote(v string) string {
	b, _ := json.Marshal(v)
	return string(b)
}
