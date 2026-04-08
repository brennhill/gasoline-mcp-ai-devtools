// Purpose: Implements selector/index resolution and parameter validation for DOM primitive actions.
// Why: Keeps action dispatch focused on orchestration while centralizing validation policy and errors.
// Docs: docs/features/feature/interact-explore/index.md

package toolinteract

import (
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"encoding/json"
	"fmt"
)

// resolveDOMSelectorFromIndex resolves index -> selector for primitive actions that omitted selector/element_id.
func (h *InteractActionHandler) resolveDOMSelectorFromIndex(req mcp.JSONRPCRequest, args json.RawMessage, params *DOMPrimitiveParams) (json.RawMessage, mcp.JSONRPCResponse, bool) {
	if params.Index == nil || params.Selector != "" || params.ElementID != "" {
		return args, mcp.JSONRPCResponse{}, false
	}

	sel, ok, stale, latestGeneration := h.resolveIndexToSelector(req.ClientID, params.TabID, *params.Index, params.IndexGen)
	if stale {
		return args, mcp.Fail(req, mcp.ErrInvalidParam,
			formatIndexGenerationConflict(params.IndexGen, latestGeneration),
			"Re-run interact with what='list_interactive' for the current page context, then retry with the returned index_generation.",
			mcp.WithParam("index_generation"), mcp.WithParam("index"),
		), true
	}
	if !ok {
		return args, mcp.Fail(req, mcp.ErrInvalidParam,
			fmt.Sprintf("Element index %d not found for tab_id=%d. Call list_interactive first to refresh the element index for this tab/client scope.", *params.Index, params.TabID),
			"Call interact with what='list_interactive' first (same tab/client scope), then use the returned index.",
			mcp.WithParam("index"), mcp.WithParam("tab_id"),
		), true
	}

	params.Selector = sel
	return updateArgsSelector(args, sel), mcp.JSONRPCResponse{}, false
}

func validateDOMSelectorRequirement(req mcp.JSONRPCRequest, action string, params DOMPrimitiveParams) (mcp.JSONRPCResponse, bool) {
	_, selectorOptional := domSelectorOptionalActions[action]
	if params.Selector != "" || params.ElementID != "" || selectorOptional {
		return mcp.JSONRPCResponse{}, false
	}

	return mcp.Fail(req, mcp.ErrMissingParam,
		"Required parameter 'selector', 'element_id', or 'index' is missing",
		"Add 'selector' (CSS or semantic selector), or use 'element_id'/'index' from list_interactive results.",
		mcp.WithParam("selector"),
	), true
}

func validateWaitForConditions(req mcp.JSONRPCRequest, action string, params DOMPrimitiveParams) (mcp.JSONRPCResponse, bool) {
	if action != "wait_for" {
		return mcp.JSONRPCResponse{}, false
	}

	hasSelector := params.Selector != "" || params.ElementID != ""
	hasText := params.Text != ""
	hasURL := params.URLContains != ""

	conditionCount := 0
	if hasSelector || params.Absent {
		conditionCount++
	}
	if hasText {
		conditionCount++
	}
	if hasURL {
		conditionCount++
	}

	if conditionCount == 0 {
		return mcp.Fail(req, mcp.ErrMissingParam,
			"wait_for requires at least one condition: selector, text, or url_contains",
			"Provide 'selector' (wait for element), 'text' (wait for text), or 'url_contains' (wait for URL change).",
			mcp.WithParam("selector"),
		), true
	}
	if conditionCount > 1 {
		return mcp.Fail(req, mcp.ErrInvalidParam,
			"wait_for conditions are mutually exclusive: use only one of selector, text, or url_contains",
			"Choose a single wait condition per call.",
		), true
	}
	if params.Absent && !hasSelector {
		return mcp.Fail(req, mcp.ErrMissingParam,
			"wait_for with absent requires a selector",
			"Provide 'selector' to specify which element to wait to disappear.",
			mcp.WithParam("selector"),
		), true
	}
	return mcp.JSONRPCResponse{}, false
}

func domActionContextOptions(action, selector string) []func(*mcp.StructuredError) {
	opts := []func(*mcp.StructuredError){mcp.WithAction(action)}
	if selector != "" {
		opts = append(opts, mcp.WithSelector(selector))
	}
	return opts
}

// ValidateDOMActionParams checks action-specific required parameters.
// Returns (response, true) if validation failed, or (zero, false) if valid.
func ValidateDOMActionParams(req mcp.JSONRPCRequest, action, text, value, name string) (mcp.JSONRPCResponse, bool) {
	rule, ok := domActionRequiredParams[action]
	if !ok {
		return mcp.JSONRPCResponse{}, false
	}

	var paramValue string
	switch rule.Field {
	case "text":
		paramValue = text
	case "value":
		paramValue = value
	case "name":
		paramValue = name
	}
	if paramValue == "" {
		return mcp.Fail(req, mcp.ErrMissingParam, rule.Message, rule.Retry, mcp.WithParam(rule.Field)), true
	}
	return mcp.JSONRPCResponse{}, false
}
