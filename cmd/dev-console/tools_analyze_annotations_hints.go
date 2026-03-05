// Purpose: Builds LLM guidance hints and correlates errors for annotation responses.
// Why: Isolates presentation/hint logic from annotation handler flow control.
// Docs: docs/features/feature/annotated-screenshots/index.md

package main

import "time"

// findErrorsNearTimestamp returns up to 5 error-level log entries within ±window of the
// given timestamp (millis). Returns a slice of maps with message and ts fields.
func (h *ToolHandler) findErrorsNearTimestamp(tsMillis int64, window time.Duration) []map[string]string {
	entries, _ := h.GetLogEntries()
	annotTime := time.UnixMilli(tsMillis)
	windowStart := annotTime.Add(-window)
	windowEnd := annotTime.Add(window)

	var matched []map[string]string
	for i := len(entries) - 1; i >= 0 && len(matched) < 5; i-- {
		entry := entries[i]
		level, _ := entry["level"].(string)
		if level != "error" {
			continue
		}
		tsStr, _ := entry["ts"].(string)
		if tsStr == "" {
			continue
		}
		entryTime, err := time.Parse(time.RFC3339, tsStr)
		if err != nil {
			continue
		}
		if entryTime.Before(windowStart) || entryTime.After(windowEnd) {
			continue
		}
		msg, _ := entry["message"].(string)
		matched = append(matched, map[string]string{
			"message": msg,
			"ts":      tsStr,
		})
	}
	return matched
}

// buildSessionHints returns LLM guidance hints for annotation session responses.
func buildSessionHints(screenshotPath string) map[string]any {
	hints := map[string]any{
		"checklist": []string{
			"Present annotations as a numbered checklist with suggested priority.",
			"For each annotation, call analyze({what:'annotation_detail', correlation_id:'...'}) for DOM/style context.",
			"If css_framework is detected, use framework-idiomatic code in fixes.",
			"Check correlated_errors — errors near the annotation timestamp may explain visual issues.",
			"After fixes, screenshot each page to compare against the baseline screenshot.",
		},
	}
	if screenshotPath != "" {
		hints["screenshot_baseline"] = "A pre-alteration screenshot was captured at " + screenshotPath + ". Compare after changes."
	}
	return hints
}

// buildDetailHints returns context-aware LLM hints for annotation detail responses.
// Returns nil if no hints apply (no framework, no a11y flags, no correlated errors).
func buildDetailHints(cssFramework string, jsFramework string, a11yFlags []string, hasCorrelatedErrors bool) map[string]any {
	hints := make(map[string]any)

	if cssFramework != "" {
		switch cssFramework {
		case "tailwind":
			hints["design_system"] = "This element uses Tailwind CSS. Prefer utility classes (e.g., bg-blue-500, p-4, text-sm) over custom CSS."
		case "bootstrap":
			hints["design_system"] = "This element uses Bootstrap. Use Bootstrap component classes (e.g., btn-primary, form-control) and grid system."
		case "css-modules":
			hints["design_system"] = "This element uses CSS Modules. Styles are scoped — modify the corresponding .module.css file."
		case "styled-components":
			hints["design_system"] = "This element uses styled-components/Emotion. Modify the component's styled template literal."
		default:
			hints["design_system"] = "CSS framework detected: " + cssFramework + ". Use framework-idiomatic patterns."
		}
	}

	if jsFramework != "" {
		switch jsFramework {
		case "react":
			hints["runtime_framework"] = "Runtime framework appears to be React. Prefer component-level fixes over direct DOM mutation."
		case "vue":
			hints["runtime_framework"] = "Runtime framework appears to be Vue. Keep template bindings and reactive state in sync with style/layout changes."
		case "angular":
			hints["runtime_framework"] = "Runtime framework appears to be Angular. Prefer template/component stylesheet updates instead of manual DOM patches."
		case "svelte":
			hints["runtime_framework"] = "Runtime framework appears to be Svelte. Apply fixes in component markup/style so compiled DOM stays consistent."
		default:
			hints["runtime_framework"] = "Runtime framework detected: " + jsFramework + ". Prefer framework-native component changes."
		}
	}

	if len(a11yFlags) > 0 {
		hints["accessibility"] = "Accessibility issues detected. Address a11y_flags before visual changes — screen reader compatibility and contrast ratios affect all users."
	}

	if hasCorrelatedErrors {
		hints["error_context"] = "Console errors occurred near this annotation's timestamp. The visual issue may be caused by a JavaScript error — check correlated_errors first."
	}

	if len(hints) == 0 {
		return nil
	}
	return hints
}
