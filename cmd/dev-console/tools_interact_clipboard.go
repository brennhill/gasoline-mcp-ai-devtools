// Purpose: Handles clipboard read and paste actions via navigator.clipboard API for testing copy/paste interactions.
// Why: Enables agents to verify clipboard content without injecting arbitrary JavaScript.
// Docs: docs/features/feature/interact-explore/index.md

package main

import (
	"encoding/json"

	"github.com/dev-console/dev-console/internal/queries"
)

// handleClipboardRead reads text from the clipboard via navigator.clipboard.readText().
func (h *ToolHandler) handleClipboardRead(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	if resp, blocked := h.requirePilot(req); blocked {
		return resp
	}
	if resp, blocked := h.requireExtension(req); blocked {
		return resp
	}
	if resp, blocked := h.requireTabTracking(req); blocked {
		return resp
	}

	script := `(async () => {
  try {
    const text = await navigator.clipboard.readText();
    return { text };
  } catch (e) {
    return { error: 'clipboard_read_failed', message: e.message };
  }
})()`

	correlationID := newCorrelationID("exec")
	execArgs, _ := json.Marshal(map[string]any{
		"script": script,
		"world":  "main",
	})

	query := queries.PendingQuery{
		Type:          "execute",
		Params:        execArgs,
		CorrelationID: correlationID,
	}
	h.capture.CreatePendingQueryWithTimeout(query, queries.AsyncCommandTimeout, req.ClientID)

	h.recordAIAction("clipboard_read", "", nil)

	return h.MaybeWaitForCommand(req, correlationID, args, "Clipboard read queued")
}

// handleClipboardWrite writes text to the clipboard via navigator.clipboard.writeText().
func (h *ToolHandler) handleClipboardWrite(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(
			ErrInvalidJSON,
			"Invalid JSON arguments: "+err.Error(),
			"Fix JSON syntax and call again",
		)}
	}
	if params.Text == "" {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(
			ErrMissingParam,
			"Required parameter 'text' is missing",
			"Add the 'text' parameter with the content to write to clipboard",
			withParam("text"),
		)}
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

	// JSON-encode the text to safely embed it in the script
	textBytes, _ := json.Marshal(params.Text)

	script := `(async () => {
  try {
    await navigator.clipboard.writeText(` + string(textBytes) + `);
    return { success: true };
  } catch (e) {
    return { error: 'clipboard_write_failed', message: e.message };
  }
})()`

	correlationID := newCorrelationID("exec")
	execArgs, _ := json.Marshal(map[string]any{
		"script": script,
		"world":  "main",
	})

	query := queries.PendingQuery{
		Type:          "execute",
		Params:        execArgs,
		CorrelationID: correlationID,
	}
	h.capture.CreatePendingQueryWithTimeout(query, queries.AsyncCommandTimeout, req.ClientID)

	h.recordAIAction("clipboard_write", "", map[string]any{"text_length": len(params.Text)})

	return h.MaybeWaitForCommand(req, correlationID, args, "Clipboard write queued")
}
