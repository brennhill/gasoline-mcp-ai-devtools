// Purpose: Handles clipboard read and paste actions via navigator.clipboard API for testing copy/paste interactions.
// Why: Enables agents to verify clipboard content without injecting arbitrary JavaScript.
// Docs: docs/features/feature/interact-explore/index.md

package main

import (
	"encoding/json"

	"github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/queries"
)

// handleClipboardRead reads text from the clipboard via navigator.clipboard.readText().
func (h *interactActionHandler) handleClipboardRead(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	if resp, blocked := h.parent.requirePilot(req); blocked {
		return resp
	}
	if resp, blocked := h.parent.requireExtension(req); blocked {
		return resp
	}
	if resp, blocked := h.parent.requireTabTracking(req); blocked {
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
	if enqueueResp, blocked := h.parent.enqueuePendingQuery(req, query, queries.AsyncCommandTimeout); blocked {
		return enqueueResp
	}

	h.parent.recordAIAction("clipboard_read", "", nil)

	return h.parent.MaybeWaitForCommand(req, correlationID, args, "Clipboard read queued")
}

// handleClipboardWrite writes text to the clipboard via navigator.clipboard.writeText().
func (h *interactActionHandler) handleClipboardWrite(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		Text string `json:"text"`
	}
		if resp, stop := parseArgs(req, args, &params); stop {
		return resp
	}
	if params.Text == "" {
		return fail(req, ErrMissingParam,
			"Required parameter 'text' is missing",
			"Add the 'text' parameter with the content to write to clipboard",
			withParam("text"))
	}

	if resp, blocked := h.parent.requirePilot(req); blocked {
		return resp
	}
	if resp, blocked := h.parent.requireExtension(req); blocked {
		return resp
	}
	if resp, blocked := h.parent.requireTabTracking(req); blocked {
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
	if enqueueResp, blocked := h.parent.enqueuePendingQuery(req, query, queries.AsyncCommandTimeout); blocked {
		return enqueueResp
	}

	h.parent.recordAIAction("clipboard_write", "", map[string]any{"text_length": len(params.Text)})

	return h.parent.MaybeWaitForCommand(req, correlationID, args, "Clipboard write queued")
}
