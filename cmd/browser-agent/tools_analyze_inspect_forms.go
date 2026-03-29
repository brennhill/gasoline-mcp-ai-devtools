// Purpose: Implements analyze modes for computed_styles, forms, form_state, form_validation, and data_table.
// Why: Groups structured page inspection logic independently from visual diff and navigation analysis.
// Docs: docs/features/feature/analyze-tool/index.md

package main

import (
	"encoding/json"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/queries"
	az "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/tools/analyze"
)

// ============================================
// Computed Styles (#79)
// ============================================

func queueAnalyzeInspectAction(h *ToolHandler, req JSONRPCRequest, correlationPrefix, queryType string, args json.RawMessage, tabID int, queuedSummary string) JSONRPCResponse {
	correlationID := newCorrelationID(correlationPrefix)
	query := queries.PendingQuery{
		Type:          queryType,
		Params:        args,
		TabID:         tabID,
		CorrelationID: correlationID,
	}
	if resp, blocked := h.enqueuePendingQuery(req, query, queries.AsyncCommandTimeout); blocked {
		return resp
	}

	return h.MaybeWaitForCommand(req, correlationID, args, queuedSummary)
}

func toolComputedStyles(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	parsed, err := az.ParseComputedStylesArgs(args)
	if err != nil {
		return fail(req, ErrMissingParam, err.Error(), "Add the 'selector' parameter with a CSS selector", withParam("selector"))
	}

	return queueAnalyzeInspectAction(h, req, "computed_styles", "computed_styles", args, parsed.TabID, "Computed styles query queued")
}

// ============================================
// Form Discovery (#81)
// ============================================

func toolFormDiscovery(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	parsed, err := az.ParseFormsArgs(args)
	if err != nil {
		return fail(req, ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")
	}

	return queueAnalyzeInspectAction(h, req, "form_discovery", "form_discovery", args, parsed.TabID, "Form discovery queued")
}

func toolFormState(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	parsed, err := az.ParseFormsArgs(args)
	if err != nil {
		return fail(req, ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")
	}

	return queueAnalyzeInspectAction(h, req, "form_state", "form_state", args, parsed.TabID, "Form state extraction queued")
}

func toolDataTable(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	parsed, err := az.ParseDataTableArgs(args)
	if err != nil {
		return fail(req, ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")
	}

	return queueAnalyzeInspectAction(h, req, "data_table", "data_table", args, parsed.TabID, "Data table extraction queued")
}

func toolFormValidation(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	parsed, err := az.ParseFormValidationArgs(args)
	if err != nil {
		return fail(req, ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")
	}

	// Parse once into a map, augment with mode=validate, and read summary flag from the same parse.
	var rawParams map[string]any
	if json.Unmarshal(args, &rawParams) == nil {
		rawParams["mode"] = "validate"
	}
	augmentedArgs, _ := json.Marshal(rawParams)
	wantSummary, _ := rawParams["summary"].(bool)

	correlationID := newCorrelationID("form_validation")
	query := queries.PendingQuery{
		Type:          "form_discovery",
		Params:        augmentedArgs,
		TabID:         parsed.TabID,
		CorrelationID: correlationID,
	}
	if enqueueResp, blocked := h.enqueuePendingQuery(req, query, queries.AsyncCommandTimeout); blocked {
		return enqueueResp
	}

	resp := h.MaybeWaitForCommand(req, correlationID, augmentedArgs, "Form validation queued")
	if wantSummary {
		resp = buildFormValidationSummary(resp)
	}

	return resp
}

// buildFormValidationSummary extracts counts from form validation response.
func buildFormValidationSummary(resp JSONRPCResponse) JSONRPCResponse {
	// Pre-check: skip error responses before mutation attempt.
	var peek MCPToolResult
	if err := json.Unmarshal(resp.Result, &peek); err != nil || peek.IsError {
		return resp
	}

	return mutateToolResult(resp, func(r *MCPToolResult) {
		for _, block := range r.Content {
			if block.Text == "" {
				continue
			}
			idx := -1
			for i, ch := range block.Text {
				if ch == '{' {
					idx = i
					break
				}
			}
			if idx < 0 {
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
			r.Content = []MCPContentBlock{{Type: "text", Text: "Form validation summary\n" + string(summaryJSON)}}
			return
		}
	})
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
