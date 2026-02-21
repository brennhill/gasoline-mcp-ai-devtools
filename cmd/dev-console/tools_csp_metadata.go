// tools_csp_metadata.go — CSP restriction metadata enrichment for observe/analyze responses.
package main

import (
	"encoding/json"
	"strings"
)

const cspRestrictedHint = "Current page blocks script execution. Tools requiring JS injection (page_summary, accessibility, get_readable, execute_js) will fail. Features that work: screenshot, performance, forms, storage, network observation."

// annotateCSPRestrictionMetadata adds proactive CSP warning metadata to JSON responses
// when recent command history indicates a CSP-restricted page context.
func (h *ToolHandler) annotateCSPRestrictionMetadata(resp JSONRPCResponse) JSONRPCResponse {
	if h == nil || h.capture == nil || resp.Result == nil {
		return resp
	}
	if !h.capture.HasRecentCSPRestriction() {
		return resp
	}

	var result MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return resp
	}
	if result.IsError {
		return resp
	}

	for i := range result.Content {
		if result.Content[i].Type != "text" {
			continue
		}
		updated, ok := injectCSPMetadataIntoText(result.Content[i].Text)
		if !ok {
			continue
		}
		result.Content[i].Text = updated

		resultJSON, err := json.Marshal(result)
		if err != nil {
			return resp
		}
		resp.Result = json.RawMessage(resultJSON)
		return resp
	}

	return resp
}

func injectCSPMetadataIntoText(text string) (string, bool) {
	jsonStart := strings.Index(text, "{")
	if jsonStart < 0 {
		return text, false
	}

	jsonPart := text[jsonStart:]
	var payload map[string]any
	if err := json.Unmarshal([]byte(jsonPart), &payload); err != nil {
		return text, false
	}

	metadata := map[string]any{}
	if existing, ok := payload["metadata"].(map[string]any); ok {
		for k, v := range existing {
			metadata[k] = v
		}
	}
	metadata["csp_restricted"] = true
	metadata["csp_hint"] = cspRestrictedHint
	payload["metadata"] = metadata

	updatedJSON, err := json.Marshal(payload)
	if err != nil {
		return text, false
	}
	return text[:jsonStart] + string(updatedJSON), true
}
