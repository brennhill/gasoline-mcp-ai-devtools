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

// appendPushPiggyback drains the push inbox and inlines events into any tool response.
// Screenshots are delivered as image content blocks so the LLM sees them immediately.
func (h *ToolHandler) appendPushPiggyback(resp JSONRPCResponse) JSONRPCResponse {
	if h.server.pushInbox == nil {
		return resp
	}

	events := h.server.pushInbox.DrainAll()
	if len(events) == 0 {
		return resp
	}

	var result MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return resp
	}

	for _, ev := range events {
		switch ev.Type {
		case "screenshot":
			label := fmt.Sprintf("\n\n_push_screenshot: captured from %s", ev.PageURL)
			if ev.Note != "" {
				label += " — " + ev.Note
			}
			result.Content = append(result.Content, MCPContentBlock{Type: "text", Text: label})
			if ev.ScreenshotB64 != "" {
				result.Content = append(result.Content, MCPContentBlock{
					Type:     "image",
					Data:     ev.ScreenshotB64,
					MimeType: "image/jpeg",
				})
			}
		case "annotations":
			label := fmt.Sprintf("\n\n_push_annotations: from %s", ev.PageURL)
			if ev.AnnotSession != "" {
				label += fmt.Sprintf(" (session: %s)", ev.AnnotSession)
			}
			if len(ev.Annotations) > 0 {
				label += "\n" + string(ev.Annotations)
			}
			result.Content = append(result.Content, MCPContentBlock{Type: "text", Text: label})
		case "chat":
			result.Content = append(result.Content, MCPContentBlock{
				Type: "text",
				Text: fmt.Sprintf("\n\n_push_chat: %s\n[from: %s]", ev.Message, ev.PageURL),
			})
		default:
			result.Content = append(result.Content, MCPContentBlock{
				Type: "text",
				Text: fmt.Sprintf("\n\n_push_%s: event from %s", ev.Type, ev.PageURL),
			})
		}
	}

	resultJSON, _ := json.Marshal(result)
	resp.Result = json.RawMessage(resultJSON)
	return resp
}
