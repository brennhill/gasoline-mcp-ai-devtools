// tools_generate_annotations.go — Generate handlers for annotation-derived artifacts.
// Provides: visual_test (Playwright), annotation_report (Markdown), annotation_issues (JSON).
package main

import (
	"encoding/json"
	"fmt"
	"time"
)

// toolGenerateVisualTest generates a Playwright test from annotation session data.
func (h *ToolHandler) toolGenerateVisualTest(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		TestName string `json:"test_name"`
		Session  string `json:"session"`
	}
	if len(args) > 0 {
		_ = json.Unmarshal(args, &params)
	}

	pages, err := h.collectAnnotationPages(params.Session)
	if err != "" {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("No annotations", map[string]any{
			"status":  "no_data",
			"message": err,
		})}
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
		Session string `json:"session"`
	}
	if len(args) > 0 {
		_ = json.Unmarshal(args, &params)
	}

	pages, err := h.collectAnnotationPages(params.Session)
	if err != "" {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("No annotations", map[string]any{
			"status":  "no_data",
			"message": err,
		})}
	}

	report := generateMarkdownReport(pages, h.annotationStore)
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpTextResponse(report)}
}

// toolGenerateAnnotationIssues generates a structured JSON issue list from annotations.
func (h *ToolHandler) toolGenerateAnnotationIssues(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		Session string `json:"session"`
	}
	if len(args) > 0 {
		_ = json.Unmarshal(args, &params)
	}

	pages, err := h.collectAnnotationPages(params.Session)
	if err != "" {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("No annotations", map[string]any{
			"status":  "no_data",
			"message": err,
		})}
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

// collectAnnotationPages gathers annotation pages from either a named or anonymous session.
// Returns (pages, errorMessage). errorMessage is empty on success.
func (h *ToolHandler) collectAnnotationPages(sessionName string) ([]*AnnotationSession, string) {
	if sessionName != "" {
		ns := h.annotationStore.GetNamedSession(sessionName)
		if ns == nil || len(ns.Pages) == 0 {
			return nil, "No annotations found in session '" + sessionName + "'. Use interact({action: 'draw_mode_start', session: '" + sessionName + "'}) to create annotations."
		}
		return ns.Pages, ""
	}

	session := h.annotationStore.GetLatestSession()
	if session == nil || len(session.Annotations) == 0 {
		return nil, "No annotation session found. Use interact({action: 'draw_mode_start'}) to create annotations."
	}
	return []*AnnotationSession{session}, ""
}

// ============================================
// Playwright test generation
// ============================================

func generatePlaywrightFromAnnotations(testName string, pages []*AnnotationSession, store *AnnotationStore) string {
	var b builder
	b.line("import { test, expect } from '@playwright/test';")
	b.line("")
	b.linef("test('%s', async ({ page }) => {", testName)

	for i, pg := range pages {
		if i > 0 {
			b.line("")
		}
		b.linef("  // --- Page: %s ---", pg.PageURL)
		b.linef("  await page.goto('%s');", pg.PageURL)
		b.line("")

		for j, ann := range pg.Annotations {
			b.linef("  // Annotation %d: %s", j+1, ann.Text)
			b.linef("  // Element: %s", ann.ElementSummary)

			// Try to get detail for richer selectors and a11y info
			detail, found := store.GetDetail(ann.CorrelationID)
			if found {
				b.linef("  // Selector: %s", detail.Selector)
				if len(detail.A11yFlags) > 0 {
					for _, flag := range detail.A11yFlags {
						b.linef("  // TODO [a11y]: %s", flag)
					}
				}
				// Generate a locator-based assertion
				selector := detail.Selector
				if detail.ID != "" {
					selector = "#" + detail.ID
				}
				b.linef("  await expect(page.locator('%s')).toBeVisible();", selector)
			} else {
				b.linef("  // (detail expired — re-run draw mode for full selectors)")
			}

			// Add screenshot assertion at annotation region
			b.linef("  await expect(page.locator('%s')).toHaveScreenshot('annotation-%d-%d.png');",
				annotationLocator(ann, detail), i+1, j+1)
			b.line("")
		}
	}

	b.line("});")
	return b.string()
}

func annotationLocator(ann Annotation, detail *AnnotationDetail) string {
	if detail != nil && detail.ID != "" {
		return "#" + detail.ID
	}
	if detail != nil && detail.Selector != "" {
		return detail.Selector
	}
	return "body"
}

// ============================================
// Markdown report generation
// ============================================

func generateMarkdownReport(pages []*AnnotationSession, store *AnnotationStore) string {
	var b builder
	b.line("# Annotation Report")
	b.linef("Generated: %s", time.Now().Format("2006-01-02 15:04"))
	b.line("")

	totalCount := 0
	for _, pg := range pages {
		totalCount += len(pg.Annotations)
	}
	b.linef("**Total annotations:** %d across %d page(s)", totalCount, len(pages))
	b.line("")

	for i, pg := range pages {
		b.linef("## Page %d: %s", i+1, pg.PageURL)
		b.line("")

		if pg.ScreenshotPath != "" {
			b.linef("Screenshot: `%s`", pg.ScreenshotPath)
			b.line("")
		}

		for j, ann := range pg.Annotations {
			b.linef("### %d. %s", j+1, ann.Text)
			b.linef("- **Element:** %s", ann.ElementSummary)
			b.linef("- **Region:** (%.0f, %.0f) %0.fx%.0f", ann.Rect.X, ann.Rect.Y, ann.Rect.Width, ann.Rect.Height)

			detail, found := store.GetDetail(ann.CorrelationID)
			if found {
				b.linef("- **Selector:** `%s`", detail.Selector)
				if len(detail.ComputedStyles) > 0 {
					b.line("- **Styles:**")
					for prop, val := range detail.ComputedStyles {
						b.linef("  - `%s`: `%s`", prop, val)
					}
				}
				if len(detail.A11yFlags) > 0 {
					b.line("- **Accessibility issues:**")
					for _, flag := range detail.A11yFlags {
						b.linef("  - %s", flag)
					}
				}
			}
			b.line("")
		}
	}

	return b.string()
}

// ============================================
// Structured issue list
// ============================================

func buildIssueList(pages []*AnnotationSession, store *AnnotationStore) []map[string]any {
	var issues []map[string]any

	for _, pg := range pages {
		for _, ann := range pg.Annotations {
			issue := map[string]any{
				"annotation_id":  ann.ID,
				"text":           ann.Text,
				"element":        ann.ElementSummary,
				"page_url":       pg.PageURL,
				"rect":           ann.Rect,
				"correlation_id": ann.CorrelationID,
			}

			detail, found := store.GetDetail(ann.CorrelationID)
			if found {
				issue["selector"] = detail.Selector
				issue["tag"] = detail.Tag
				issue["computed_styles"] = detail.ComputedStyles
				if len(detail.A11yFlags) > 0 {
					issue["a11y_flags"] = detail.A11yFlags
				}
			}

			issues = append(issues, issue)
		}
	}

	return issues
}

// ============================================
// String builder helper
// ============================================

type builder struct {
	buf []byte
}

func (b *builder) line(s string) {
	b.buf = append(b.buf, s...)
	b.buf = append(b.buf, '\n')
}

func (b *builder) linef(format string, args ...any) {
	b.buf = append(b.buf, fmt.Sprintf(format, args...)...)
	b.buf = append(b.buf, '\n')
}

func (b *builder) string() string {
	return string(b.buf)
}
