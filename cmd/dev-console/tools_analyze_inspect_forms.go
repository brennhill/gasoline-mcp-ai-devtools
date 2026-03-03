// Purpose: Implements analyze modes for computed_styles, forms, form_state, form_validation, and data_table.
// Why: Groups structured page inspection logic independently from visual diff and navigation analysis.
// Docs: docs/features/feature/analyze-tool/index.md

package main

import (
	"encoding/json"

	"github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/queries"
	az "github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/tools/analyze"
)

// ============================================
// Computed Styles (#79)
// ============================================

func toolComputedStyles(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	parsed, err := az.ParseComputedStylesArgs(args)
	if err != nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrMissingParam, err.Error(), "Add the 'selector' parameter with a CSS selector", withParam("selector"))}
	}

	correlationID := newCorrelationID("computed_styles")
	query := queries.PendingQuery{
		Type:          "computed_styles",
		Params:        args,
		TabID:         parsed.TabID,
		CorrelationID: correlationID,
	}
	h.capture.CreatePendingQueryWithTimeout(query, queries.AsyncCommandTimeout, req.ClientID)

	return h.MaybeWaitForCommand(req, correlationID, args, "Computed styles query queued")
}

// ============================================
// Form Discovery (#81)
// ============================================

func toolFormDiscovery(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	parsed, err := az.ParseFormsArgs(args)
	if err != nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")}
	}

	correlationID := newCorrelationID("form_discovery")
	query := queries.PendingQuery{
		Type:          "form_discovery",
		Params:        args,
		TabID:         parsed.TabID,
		CorrelationID: correlationID,
	}
	h.capture.CreatePendingQueryWithTimeout(query, queries.AsyncCommandTimeout, req.ClientID)

	return h.MaybeWaitForCommand(req, correlationID, args, "Form discovery queued")
}

func toolFormState(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	parsed, err := az.ParseFormsArgs(args)
	if err != nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")}
	}

	correlationID := newCorrelationID("form_state")
	query := queries.PendingQuery{
		Type:          "form_state",
		Params:        args,
		TabID:         parsed.TabID,
		CorrelationID: correlationID,
	}
	h.capture.CreatePendingQueryWithTimeout(query, queries.AsyncCommandTimeout, req.ClientID)

	return h.MaybeWaitForCommand(req, correlationID, args, "Form state extraction queued")
}

func toolDataTable(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	parsed, err := az.ParseDataTableArgs(args)
	if err != nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")}
	}

	correlationID := newCorrelationID("data_table")
	query := queries.PendingQuery{
		Type:          "data_table",
		Params:        args,
		TabID:         parsed.TabID,
		CorrelationID: correlationID,
	}
	h.capture.CreatePendingQueryWithTimeout(query, queries.AsyncCommandTimeout, req.ClientID)

	return h.MaybeWaitForCommand(req, correlationID, args, "Data table extraction queued")
}

func toolFormValidation(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	parsed, err := az.ParseFormValidationArgs(args)
	if err != nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")}
	}

	var summaryParams struct {
		Summary bool `json:"summary"`
	}
	json.Unmarshal(args, &summaryParams)

	// Add mode=validate to params for the extension.
	var rawParams map[string]any
	if json.Unmarshal(args, &rawParams) == nil {
		rawParams["mode"] = "validate"
	}
	augmentedArgs, _ := json.Marshal(rawParams)

	correlationID := newCorrelationID("form_validation")
	query := queries.PendingQuery{
		Type:          "form_discovery",
		Params:        augmentedArgs,
		TabID:         parsed.TabID,
		CorrelationID: correlationID,
	}
	h.capture.CreatePendingQueryWithTimeout(query, queries.AsyncCommandTimeout, req.ClientID)

	resp := h.MaybeWaitForCommand(req, correlationID, augmentedArgs, "Form validation queued")
	if summaryParams.Summary {
		resp = buildFormValidationSummary(resp)
	}

	return resp
}

// buildFormValidationSummary extracts counts from form validation response.
func buildFormValidationSummary(resp JSONRPCResponse) JSONRPCResponse {
	var result MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil || result.IsError {
		return resp
	}

	for _, block := range result.Content {
		idx := 0
		for i, ch := range block.Text {
			if ch == '{' {
				idx = i
				break
			}
		}
		if idx == 0 && block.Text[0] != '{' {
			continue
		}

		var data map[string]any
		if json.Unmarshal([]byte(block.Text[idx:]), &data) != nil {
			continue
		}

		forms := extractFormsList(data)
		if forms == nil {
			continue
		}

		totalForms := len(forms)
		valid := 0
		invalid := 0
		for _, f := range forms {
			fMap, ok := f.(map[string]any)
			if !ok {
				continue
			}
			if isValid, ok := fMap["valid"].(bool); ok && isValid {
				valid++
			} else {
				invalid++
			}
		}

		summaryData := map[string]any{
			"total_forms": totalForms,
			"valid":       valid,
			"invalid":     invalid,
		}
		summaryJSON, _ := json.Marshal(summaryData)
		result.Content = []MCPContentBlock{{Type: "text", Text: "Form validation summary\n" + string(summaryJSON)}}
		newResult, _ := json.Marshal(result)
		resp.Result = newResult
		return resp
	}

	return resp
}

func extractFormsList(data map[string]any) []any {
	if forms, ok := data["forms"].([]any); ok {
		return forms
	}
	if result, ok := data["result"].(map[string]any); ok {
		if forms, ok := result["forms"].([]any); ok {
			return forms
		}
		if inner, ok := result["result"].(map[string]any); ok {
			if forms, ok := inner["forms"].([]any); ok {
				return forms
			}
		}
	}
	return nil
}
