// Purpose: Provides shared interact response helpers for queued detection and enrichment.
// Why: Keeps response-shaping logic isolated from interact dispatch orchestration.

package main

import (
	"encoding/json"
	"strings"

	"github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/tools/observe"
)

// isResponseQueued checks if an MCP response is a queued async response.
func isResponseQueued(resp JSONRPCResponse) bool {
	if resp.Result == nil {
		return false
	}
	var result MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return false
	}
	if len(result.Content) == 0 {
		return false
	}

	for _, c := range result.Content {
		if c.Type != "text" || len(c.Text) == 0 {
			continue
		}
		text := c.Text
		if idx := strings.Index(text, "\n{"); idx >= 0 {
			text = text[idx+1:]
		}
		var data map[string]any
		if err := json.Unmarshal([]byte(text), &data); err != nil {
			continue
		}
		if status, ok := data["status"].(string); ok && status == "queued" {
			return true
		}
	}
	return false
}

// appendScreenshotToResponse captures a screenshot and appends it as an inline image block.
func (h *ToolHandler) appendScreenshotToResponse(resp JSONRPCResponse, req JSONRPCRequest) JSONRPCResponse {
	screenshotReq := JSONRPCRequest{JSONRPC: "2.0", ID: req.ID}
	screenshotResp := observe.GetScreenshot(h, screenshotReq, nil)

	var screenshotResult MCPToolResult
	if err := json.Unmarshal(screenshotResp.Result, &screenshotResult); err != nil {
		return resp // best effort: keep original response on parse failure
	}

	for _, block := range screenshotResult.Content {
		if block.Type != "image" || block.Data == "" {
			continue
		}
		var result MCPToolResult
		if err := json.Unmarshal(resp.Result, &result); err != nil {
			return resp
		}
		result.Content = append(result.Content, block)
		resultJSON, err := json.Marshal(result)
		if err != nil {
			return resp
		}
		resp.Result = json.RawMessage(resultJSON)
		break
	}

	return resp
}

// appendInteractiveToResponse appends list_interactive text to the response.
func (h *ToolHandler) appendInteractiveToResponse(resp JSONRPCResponse, req JSONRPCRequest) JSONRPCResponse {
	listReq := JSONRPCRequest{JSONRPC: "2.0", ID: req.ID, ClientID: req.ClientID}
	listArgs, _ := json.Marshal(map[string]any{"what": "list_interactive", "visible_only": true})
	listResp := h.handleListInteractive(listReq, listArgs)

	var listResult MCPToolResult
	if err := json.Unmarshal(listResp.Result, &listResult); err != nil || listResult.IsError {
		return resp
	}

	for _, block := range listResult.Content {
		if block.Type != "text" || block.Text == "" {
			continue
		}
		var result MCPToolResult
		if err := json.Unmarshal(resp.Result, &result); err != nil {
			return resp
		}
		result.Content = append(result.Content, MCPContentBlock{
			Type: "text",
			Text: "\n--- Interactive Elements ---\n" + block.Text,
		})
		resultJSON, err := json.Marshal(result)
		if err != nil {
			return resp
		}
		resp.Result = json.RawMessage(resultJSON)
		break
	}
	return resp
}
