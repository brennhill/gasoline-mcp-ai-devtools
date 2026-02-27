// Purpose: Implements analyze tool handlers and response shaping.
// Why: Keeps analyze tool behavior aligned with diagnostic and schema contracts.
// Docs: docs/features/feature/analyze-tool/index.md

// tools_analyze_inspect.go — Analyze handlers for computed styles, forms, and visual regression.
package main

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"time"

	ai "github.com/dev-console/dev-console/internal/ai"
	"github.com/dev-console/dev-console/internal/queries"
	"github.com/dev-console/dev-console/internal/state"
	az "github.com/dev-console/dev-console/internal/tools/analyze"
	"github.com/dev-console/dev-console/internal/tools/observe"
)

// ============================================
// Computed Styles (#79)
// ============================================

func (h *ToolHandler) toolComputedStyles(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
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

func (h *ToolHandler) toolFormDiscovery(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
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

func (h *ToolHandler) toolFormValidation(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	parsed, err := az.ParseFormValidationArgs(args)
	if err != nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")}
	}

	var summaryParams struct {
		Summary bool `json:"summary"`
	}
	json.Unmarshal(args, &summaryParams)

	// Add mode=validate to params for the extension
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

		// Extract forms array from result or result.result
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

// ============================================
// Visual Regression (#82)
// ============================================

func (h *ToolHandler) toolVisualBaseline(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	parsed, err := az.ParseVisualBaselineArgs(args)
	if err != nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrMissingParam, err.Error(), "Add the 'name' parameter for the baseline", withParam("name"))}
	}

	// Step 1: Take a screenshot using existing flow
	screenshotResp := observe.GetScreenshot(h, req, json.RawMessage(`{}`))
	if isErrorResponse(screenshotResp) {
		return screenshotResp
	}

	// Parse screenshot result to get path
	screenshotPath := extractScreenshotPath(screenshotResp)
	if screenshotPath == "" {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrExtError, "Screenshot captured but path not available", "Try again or check extension connection")}
	}

	// Step 2: Store baseline metadata via session store
	now := time.Now()
	_, _, trackedURL := h.capture.GetTrackingStatus()

	metadata := az.BaselineMetadata{
		Path:      screenshotPath,
		URL:       trackedURL,
		SavedAt:   now.Format(time.RFC3339),
		Name:      parsed.Name,
		Timestamp: now.UnixMilli(),
	}

	metadataJSON, _ := json.Marshal(metadata)

	if h.sessionStoreImpl != nil {
		storeArgs := ai.SessionStoreArgs{
			Action:    "save",
			Namespace: "visual_baselines",
			Key:       parsed.Name,
			Data:      metadataJSON,
		}
		if _, err := h.sessionStoreImpl.HandleSessionStore(storeArgs); err != nil {
			return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidParam, "Failed to store baseline: "+err.Error(), "Check session store configuration")}
		}
	}

	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Visual baseline saved", map[string]any{
		"status":   "saved",
		"name":     parsed.Name,
		"path":     screenshotPath,
		"url":      trackedURL,
		"saved_at": metadata.SavedAt,
	})}
}

func (h *ToolHandler) toolVisualDiff(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	parsed, err := az.ParseVisualDiffArgs(args)
	if err != nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrMissingParam, err.Error(), "Add the 'baseline' parameter with the baseline name", withParam("baseline"))}
	}

	// Step 1: Load baseline metadata from session store
	if h.sessionStoreImpl == nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrNotInitialized, "Session store not initialized", "Internal error — do not retry")}
	}

	loadArgs := ai.SessionStoreArgs{
		Action:    "load",
		Namespace: "visual_baselines",
		Key:       parsed.Baseline,
	}
	loadResult, err := h.sessionStoreImpl.HandleSessionStore(loadArgs)
	if err != nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidParam, "Baseline '"+parsed.Baseline+"' not found: "+err.Error(), "Save a baseline first with analyze(what='visual_baseline', name='"+parsed.Baseline+"')")}
	}

	var storeResp struct {
		Data json.RawMessage `json:"data"`
	}
	json.Unmarshal(loadResult, &storeResp)

	var baseline az.BaselineMetadata
	if err := json.Unmarshal(storeResp.Data, &baseline); err != nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidJSON, "Failed to parse baseline metadata: "+err.Error(), "Re-save the baseline")}
	}

	// Step 2: Take a current screenshot
	screenshotResp := observe.GetScreenshot(h, req, json.RawMessage(`{}`))
	if isErrorResponse(screenshotResp) {
		return screenshotResp
	}

	currentPath := extractScreenshotPath(screenshotResp)
	if currentPath == "" {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrExtError, "Current screenshot path not available", "Try again")}
	}

	// Step 3: Compare images
	diffResult, err := az.CompareImages(baseline.Path, currentPath, parsed.Threshold)
	if err != nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrExtError, "Image comparison failed: "+err.Error(), "Check that baseline image exists at: "+baseline.Path)}
	}

	// Step 4: Save diff image
	var diffPath string
	if diffResult.PixelsChanged > 0 {
		screenshotsDir, err := state.ScreenshotsDir()
		if err == nil {
			diffPath = filepath.Join(screenshotsDir, fmt.Sprintf("diff-%s-%d.png", parsed.Baseline, time.Now().UnixMilli()))
			baselineImg, err1 := az.LoadImagePublic(baseline.Path)
			currentImg, err2 := az.LoadImagePublic(currentPath)
			if err1 == nil && err2 == nil {
				// Rebuild changed grid for diff image
				changedGrid := az.RebuildChangedGrid(baselineImg, currentImg, parsed.Threshold)
				az.WriteDiffImagePublic(baselineImg, currentImg, changedGrid, diffPath)
			}
		}
	}

	response := map[string]any{
		"diff_percentage":  diffResult.DiffPercentage,
		"pixels_changed":   diffResult.PixelsChanged,
		"pixels_total":     diffResult.PixelsTotal,
		"dimensions_match": diffResult.DimensionsMatch,
		"verdict":          diffResult.Verdict,
		"threshold":        diffResult.Threshold,
		"regions":          diffResult.Regions,
		"baseline": map[string]any{
			"path":     baseline.Path,
			"url":      baseline.URL,
			"saved_at": baseline.SavedAt,
		},
		"current_path": currentPath,
	}

	if diffPath != "" {
		response["diff_path"] = diffPath
	}

	if diffResult.DimensionDelta != nil {
		response["dimension_delta"] = map[string]int{
			"width":  diffResult.DimensionDelta[0],
			"height": diffResult.DimensionDelta[1],
		}
	}

	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Visual diff complete", response)}
}

func (h *ToolHandler) toolListVisualBaselines(req JSONRPCRequest, _ json.RawMessage) JSONRPCResponse {
	if h.sessionStoreImpl == nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrNotInitialized, "Session store not initialized", "Internal error — do not retry")}
	}

	listArgs := ai.SessionStoreArgs{
		Action:    "list",
		Namespace: "visual_baselines",
	}
	listResult, err := h.sessionStoreImpl.HandleSessionStore(listArgs)
	if err != nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidParam, "Failed to list baselines: "+err.Error(), "Check session store")}
	}

	var listData map[string]any
	if err := json.Unmarshal(listResult, &listData); err != nil {
		listData = map[string]any{"raw": string(listResult)}
	}

	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Visual baselines", listData)}
}

// ============================================
// Navigation / SPA Route Discovery (#335)
// ============================================

func (h *ToolHandler) toolAnalyzeNavigation(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		TabID int `json:"tab_id"`
	}
	if len(args) > 0 {
		if err := json.Unmarshal(args, &params); err != nil {
			return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")}
		}
	}

	correlationID := newCorrelationID("navigation")

	query := queries.PendingQuery{
		Type:          "navigation",
		Params:        args,
		TabID:         params.TabID,
		CorrelationID: correlationID,
	}
	h.capture.CreatePendingQueryWithTimeout(query, queries.AsyncCommandTimeout, req.ClientID)

	return h.MaybeWaitForCommand(req, correlationID, args, "Navigation discovery queued")
}

// ---- helpers ----

// extractScreenshotPath extracts the file path from a screenshot response.
func extractScreenshotPath(resp JSONRPCResponse) string {
	var result MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil || len(result.Content) == 0 {
		return ""
	}
	text := result.Content[0].Text
	// Find JSON object in response text
	idx := 0
	for i, ch := range text {
		if ch == '{' {
			idx = i
			break
		}
	}
	var data map[string]any
	if err := json.Unmarshal([]byte(text[idx:]), &data); err != nil {
		return ""
	}
	if p, ok := data["path"].(string); ok && p != "" {
		return p
	}
	if filename, ok := data["filename"].(string); ok && filename != "" {
		if dir, err := state.ScreenshotsDir(); err == nil {
			return filepath.Join(dir, filename)
		}
	}
	return ""
}
