// Purpose: Renders annotation data into Playwright visual test source.
// Why: Keeps Playwright script generation isolated from JSON-RPC request orchestration.
// Docs: docs/features/feature/annotated-screenshots/index.md

package main

import "strings"

// generatePlaywrightFromAnnotations builds a Playwright script from annotation sessions.
func generatePlaywrightFromAnnotations(testName string, pages []*AnnotationSession, store *AnnotationStore) string {
	var b builder
	b.line("import { test, expect } from '@playwright/test';")
	b.line("")
	b.linef("test('%s', async ({ page }) => {", jsEscapeSingle(testName))

	for i, pg := range pages {
		if i > 0 {
			b.line("")
		}
		b.linef("  // --- Page: %s ---", pg.PageURL)
		b.linef("  await page.goto('%s');", jsEscapeSingle(pg.PageURL))
		b.line("")

		for j, ann := range pg.Annotations {
			b.linef("  // Annotation %d: %s", j+1, ann.Text)
			b.linef("  // Element: %s", ann.ElementSummary)

			// Try to get detail for richer selectors and a11y info.
			detail, found := store.GetDetail(ann.CorrelationID)
			if found {
				b.linef("  // Selector: %s", detail.Selector)
				if len(detail.A11yFlags) > 0 {
					for _, flag := range detail.A11yFlags {
						b.linef("  // TODO [a11y]: %s", flag)
					}
				}
				selector := detail.Selector
				if detail.ID != "" {
					selector = "#" + detail.ID
				}
				b.linef("  await expect(page.locator('%s')).toBeVisible();", jsEscapeSingle(selector))
			} else {
				b.line("  // (detail expired — re-run draw mode for full selectors)")
			}

			b.linef("  await expect(page.locator('%s')).toHaveScreenshot('annotation-%d-%d.png');",
				jsEscapeSingle(annotationLocator(ann, detail)), i+1, j+1)
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

// jsEscapeSingle escapes a string for safe embedding inside JS single-quoted literals.
// Handles backslashes, single quotes, and newlines to prevent code injection.
func jsEscapeSingle(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `'`, `\'`)
	s = strings.ReplaceAll(s, "\n", `\n`)
	s = strings.ReplaceAll(s, "\r", `\r`)
	return s
}
