// Purpose: Generates annotation-derived artifacts — visual_test (Playwright), annotation_report (Markdown), and annotation_issues (JSON).
// Why: Converts draw-mode annotations into actionable test scripts and structured issue reports.
// Docs: docs/features/feature/annotated-screenshots/index.md
package main

import (
	"encoding/json"
	"fmt"
)

// toolGenerateVisualTest generates a Playwright test from annotation session data.
func (h *ToolHandler) toolGenerateVisualTest(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		TestName     string `json:"test_name"`
		AnnotSession string `json:"annot_session"`
	}
	if len(args) > 0 {
		lenientUnmarshal(args, &params)
	}

	pages, noDataResp, noData := h.resolveAnnotationPages(req, params.AnnotSession)
	if noData {
		return noDataResp
	}

	testName := params.TestName
	if testName == "" {
		testName = "visual review annotations"
	}

	script := generatePlaywrightFromAnnotations(testName, pages, h.annotationStore)
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpTextResponse(script)}
}

// toolGenerateAnnotationReport generates a Markdown report from annotation session data.
func (h *ToolHandler) toolGenerateAnnotationReport(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		AnnotSession string `json:"annot_session"`
	}
	if len(args) > 0 {
		lenientUnmarshal(args, &params)
	}

	pages, noDataResp, noData := h.resolveAnnotationPages(req, params.AnnotSession)
	if noData {
		return noDataResp
	}

	report := generateMarkdownReport(pages, h.annotationStore)
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpTextResponse(report)}
}

// toolGenerateAnnotationIssues generates a structured JSON issue list from annotations.
func (h *ToolHandler) toolGenerateAnnotationIssues(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		AnnotSession string `json:"annot_session"`
	}
	if len(args) > 0 {
		lenientUnmarshal(args, &params)
	}

	pages, noDataResp, noData := h.resolveAnnotationPages(req, params.AnnotSession)
	if noData {
		return noDataResp
	}

	issues := buildIssueList(pages, h.annotationStore)
	result := map[string]any{
		"issues":      issues,
		"total_count": len(issues),
		"page_count":  len(pages),
	}

	summary := fmt.Sprintf("Annotation issues (%d issues across %d pages)", len(issues), len(pages))
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse(summary, result)}
}

func (h *ToolHandler) resolveAnnotationPages(req JSONRPCRequest, sessionName string) ([]*AnnotationSession, JSONRPCResponse, bool) {
	pages, err := h.collectAnnotationPages(sessionName)
	if err == "" {
		return pages, JSONRPCResponse{}, false
	}
	return nil, JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("No annotations", map[string]any{
		"status":  "no_data",
		"message": err,
	})}, true
}

// collectAnnotationPages gathers annotation pages from either a named or anonymous session.
// Returns (pages, errorMessage). errorMessage is empty on success.
func (h *ToolHandler) collectAnnotationPages(sessionName string) ([]*AnnotationSession, string) {
	if sessionName != "" {
		ns := h.annotationStore.GetNamedSession(sessionName)
		if ns == nil || len(ns.Pages) == 0 {
			return nil, "No annotations found in session '" + sessionName + "'. Use interact({action: 'draw_mode_start', annot_session: '" + sessionName + "'}) to create annotations."
		}
		return ns.Pages, ""
	}

	session := h.annotationStore.GetLatestSession()
	if session == nil || len(session.Annotations) == 0 {
		return nil, "No annotation session found. Use interact({action: 'draw_mode_start'}) to create annotations."
	}
	return []*AnnotationSession{session}, ""
}
