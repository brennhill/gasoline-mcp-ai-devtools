// reproduction.go — Reproduction script generation from captured actions.
// Generates Playwright tests or Gasoline natural language scripts from
// EnhancedAction data captured by the browser extension.
// Design: Two output formats, shared selector extraction, single-pass generation.
package main

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/dev-console/dev-console/internal/capture"
)

// ============================================
// Types
// ============================================

// ReproductionParams are the parsed arguments for generate({format: "reproduction"}).
type ReproductionParams struct {
	Format             string `json:"format"`
	OutputFormat       string `json:"output_format"`
	LastN              int    `json:"last_n"`
	BaseURL            string `json:"base_url"`
	IncludeScreenshots bool   `json:"include_screenshots"`
	ErrorMessage       string `json:"error_message"`
}

// ReproductionResult is the response payload.
type ReproductionResult struct {
	Script      string           `json:"script"`
	Format      string           `json:"format"`
	ActionCount int              `json:"action_count"`
	DurationMs  int64            `json:"duration_ms"`
	StartURL    string           `json:"start_url"`
	Metadata    ReproductionMeta `json:"metadata"`
}

// ReproductionMeta provides traceability for the generated script.
type ReproductionMeta struct {
	GeneratedAt      string   `json:"generated_at"`
	SelectorsUsed    []string `json:"selectors_used"`
	ActionsAvailable int      `json:"actions_available"`
	ActionsIncluded  int      `json:"actions_included"`
}

const maxReproOutputBytes = 200 * 1024 // 200KB cap

// ============================================
// Entry Point (replaces stub)
// ============================================

// toolGetReproductionScriptImpl generates a reproduction script from captured actions.
func (h *ToolHandler) toolGetReproductionScriptImpl(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params ReproductionParams
	if len(args) > 0 {
		_ = json.Unmarshal(args, &params)
	}

	// Default output format
	if params.OutputFormat == "" {
		params.OutputFormat = "gasoline"
	}

	// Validate output format
	if params.OutputFormat != "gasoline" && params.OutputFormat != "playwright" {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(
			ErrInvalidParam,
			"Invalid output_format: "+params.OutputFormat,
			"Use 'gasoline' or 'playwright'",
			withParam("output_format"),
		)}
	}

	// Get actions from capture buffer
	allActions := h.capture.GetAllEnhancedActions()
	if len(allActions) == 0 {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(
			ErrNoActionsCaptured,
			"No user actions recorded in the session",
			"Interact with the page first (click, type, navigate), then retry",
		)}
	}

	// Apply last_n filter
	actions := allActions
	if params.LastN > 0 && params.LastN < len(actions) {
		actions = actions[len(actions)-params.LastN:]
	}

	// Generate script
	var script string
	switch params.OutputFormat {
	case "gasoline":
		script = generateGasolineScript(actions, params)
	case "playwright":
		script = generateReproPlaywrightScript(actions, params)
	}

	// Compute metadata
	startURL := ""
	if len(actions) > 0 {
		startURL = actions[0].URL
		if actions[0].Type == "navigate" && actions[0].ToURL != "" {
			startURL = actions[0].ToURL
		}
	}

	var durationMs int64
	if len(actions) > 1 {
		durationMs = actions[len(actions)-1].Timestamp - actions[0].Timestamp
	}

	selectorTypes := collectSelectorTypes(actions)

	result := ReproductionResult{
		Script:      script,
		Format:      params.OutputFormat,
		ActionCount: len(actions),
		DurationMs:  durationMs,
		StartURL:    startURL,
		Metadata: ReproductionMeta{
			GeneratedAt:      time.Now().Format(time.RFC3339),
			SelectorsUsed:    selectorTypes,
			ActionsAvailable: len(allActions),
			ActionsIncluded:  len(actions),
		},
	}

	summary := fmt.Sprintf("Reproduction script (%s, %d actions)", params.OutputFormat, len(actions))
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse(summary, result)}
}

// ============================================
// Gasoline (Natural Language) Format
// ============================================

// generateGasolineScript converts actions to numbered human-readable steps.
func generateGasolineScript(actions []capture.EnhancedAction, opts ReproductionParams) string {
	if len(actions) == 0 {
		return "# No actions captured\n"
	}

	// Apply last_n filter
	if opts.LastN > 0 && opts.LastN < len(actions) {
		actions = actions[len(actions)-opts.LastN:]
	}

	var b strings.Builder

	// Header
	startURL := actions[0].URL
	if actions[0].Type == "navigate" && actions[0].ToURL != "" {
		startURL = actions[0].ToURL
	}
	desc := "captured user actions"
	if opts.ErrorMessage != "" {
		desc = opts.ErrorMessage
		if len(desc) > 80 {
			desc = desc[:80]
		}
	}
	b.WriteString(fmt.Sprintf("# Reproduction: %s\n", desc))
	b.WriteString(fmt.Sprintf("# Captured: %s | %d actions | %s\n\n",
		time.Now().Format(time.RFC3339), len(actions), startURL))

	stepNum := 0
	var prevTs int64

	for _, action := range actions {
		// Timing pause
		if prevTs > 0 && action.Timestamp-prevTs > 2000 {
			gap := (action.Timestamp - prevTs) / 1000
			b.WriteString(fmt.Sprintf("   [%ds pause]\n", gap))
		}
		prevTs = action.Timestamp

		line := gasolineStep(action, opts)
		if line == "" {
			continue // skip actions with no meaningful output (e.g. navigate with no URL)
		}
		stepNum++

		prefix := ""
		if action.Source == "ai" {
			prefix = "(AI) "
		}
		b.WriteString(fmt.Sprintf("%d. %s%s\n", stepNum, prefix, line))
	}

	if opts.ErrorMessage != "" {
		b.WriteString(fmt.Sprintf("\n# Error: %s\n", opts.ErrorMessage))
	}

	return b.String()
}

// gasolineStep converts a single action to a natural language step.
func gasolineStep(action capture.EnhancedAction, opts ReproductionParams) string {
	switch action.Type {
	case "navigate":
		toURL := action.ToURL
		if toURL == "" {
			return ""
		}
		if opts.BaseURL != "" {
			toURL = rewriteURL(toURL, opts.BaseURL)
		}
		return "Navigate to: " + toURL

	case "click":
		return "Click: " + describeElement(action)

	case "input":
		value := action.Value
		if value == "[redacted]" {
			value = "[user-provided]"
		}
		return fmt.Sprintf("Type %q into: %s", value, describeElement(action))

	case "select":
		text := action.SelectedText
		if text == "" {
			text = action.SelectedValue
		}
		return fmt.Sprintf("Select %q from: %s", text, describeElement(action))

	case "keypress":
		return "Press: " + action.Key

	case "scroll":
		return fmt.Sprintf("Scroll to: y=%d", action.ScrollY)

	default:
		return ""
	}
}

// ============================================
// Playwright Format
// ============================================

// generateReproPlaywrightScript converts actions to a Playwright test script.
func generateReproPlaywrightScript(actions []capture.EnhancedAction, opts ReproductionParams) string {
	if len(actions) == 0 {
		return "// No actions captured\n"
	}

	// Apply last_n filter
	if opts.LastN > 0 && opts.LastN < len(actions) {
		actions = actions[len(actions)-opts.LastN:]
	}

	var b strings.Builder

	b.WriteString("import { test, expect } from '@playwright/test';\n\n")

	testName := "reproduction: captured user actions"
	if opts.ErrorMessage != "" {
		name := opts.ErrorMessage
		if len(name) > 80 {
			name = name[:80]
		}
		testName = "reproduction: " + name
	}
	b.WriteString(fmt.Sprintf("test('%s', async ({ page }) => {\n", escapeJS(testName)))

	var prevTs int64

	for _, action := range actions {
		// Timing pause comment
		if prevTs > 0 && action.Timestamp-prevTs > 2000 {
			gap := (action.Timestamp - prevTs) / 1000
			b.WriteString(fmt.Sprintf("  // [%ds pause]\n", gap))
		}
		prevTs = action.Timestamp

		line := playwrightStep(action, opts)
		if line != "" {
			b.WriteString("  " + line + "\n")
		}
	}

	if opts.ErrorMessage != "" {
		b.WriteString(fmt.Sprintf("  // Error: %s\n", opts.ErrorMessage))
	}

	b.WriteString("});\n")

	script := b.String()
	if len(script) > maxReproOutputBytes {
		script = script[:maxReproOutputBytes]
	}
	return script
}

// playwrightStep converts a single action to a Playwright code line.
func playwrightStep(action capture.EnhancedAction, opts ReproductionParams) string {
	switch action.Type {
	case "navigate":
		toURL := action.ToURL
		if toURL == "" {
			return ""
		}
		if opts.BaseURL != "" {
			toURL = rewriteURL(toURL, opts.BaseURL)
		}
		return fmt.Sprintf("await page.goto('%s');", escapeJS(toURL))

	case "click":
		loc := playwrightLocator(action.Selectors)
		if loc == "" {
			return "// click - no selector available"
		}
		return fmt.Sprintf("await page.%s.click();", loc)

	case "input":
		loc := playwrightLocator(action.Selectors)
		if loc == "" {
			return "// input - no selector available"
		}
		value := action.Value
		if value == "[redacted]" {
			value = "[user-provided]"
		}
		return fmt.Sprintf("await page.%s.fill('%s');", loc, escapeJS(value))

	case "select":
		loc := playwrightLocator(action.Selectors)
		if loc == "" {
			return "// select - no selector available"
		}
		return fmt.Sprintf("await page.%s.selectOption('%s');", loc, escapeJS(action.SelectedValue))

	case "keypress":
		return fmt.Sprintf("await page.keyboard.press('%s');", escapeJS(action.Key))

	case "scroll":
		return fmt.Sprintf("// Scroll to y=%d", action.ScrollY)

	default:
		return ""
	}
}

// ============================================
// Selector Helpers
// ============================================

// describeElement returns the most human-readable description of the target element.
// Priority: text+role > ariaLabel+role > role.name+role > testId > text > ariaLabel > id > cssPath
func describeElement(action capture.EnhancedAction) string {
	s := action.Selectors
	if s == nil {
		return "(unknown element)"
	}

	text := selectorStr(s, "text")
	ariaLabel := selectorStr(s, "ariaLabel")
	testID := selectorStr(s, "testId")
	id := selectorStr(s, "id")
	cssPath := selectorStr(s, "cssPath")
	role, roleName := selectorRole(s)

	// Priority 1: text + role
	if text != "" && role != "" {
		return fmt.Sprintf("%q %s", text, role)
	}
	// Priority 2: ariaLabel + role
	if ariaLabel != "" && role != "" {
		return fmt.Sprintf("%q %s", ariaLabel, role)
	}
	// Priority 3: role name + role (from role map)
	if roleName != "" && role != "" {
		return fmt.Sprintf("%q %s", roleName, role)
	}
	// Priority 4: testId
	if testID != "" {
		return fmt.Sprintf("[data-testid=%q]", testID)
	}
	// Priority 5: text alone
	if text != "" {
		return fmt.Sprintf("%q", text)
	}
	// Priority 6: ariaLabel alone
	if ariaLabel != "" {
		return fmt.Sprintf("%q", ariaLabel)
	}
	// Priority 7: id
	if id != "" {
		return "#" + id
	}
	// Priority 8: cssPath
	if cssPath != "" {
		return cssPath
	}

	return "(unknown element)"
}

// playwrightLocator returns the best Playwright locator string for a selector map.
// Priority: testId > role > ariaLabel > text > id > cssPath
func playwrightLocator(selectors map[string]any) string {
	if selectors == nil {
		return ""
	}

	testID := selectorStr(selectors, "testId")
	if testID != "" {
		return fmt.Sprintf("getByTestId('%s')", escapeJS(testID))
	}

	role, roleName := selectorRole(selectors)
	if role != "" {
		if roleName != "" {
			return fmt.Sprintf("getByRole('%s', { name: '%s' })", escapeJS(role), escapeJS(roleName))
		}
		return fmt.Sprintf("getByRole('%s')", escapeJS(role))
	}

	ariaLabel := selectorStr(selectors, "ariaLabel")
	if ariaLabel != "" {
		return fmt.Sprintf("getByLabel('%s')", escapeJS(ariaLabel))
	}

	text := selectorStr(selectors, "text")
	if text != "" {
		return fmt.Sprintf("getByText('%s')", escapeJS(text))
	}

	id := selectorStr(selectors, "id")
	if id != "" {
		return fmt.Sprintf("locator('#%s')", escapeJS(id))
	}

	cssPath := selectorStr(selectors, "cssPath")
	if cssPath != "" {
		return fmt.Sprintf("locator('%s')", escapeJS(cssPath))
	}

	return ""
}

// selectorStr extracts a string value from the selectors map.
func selectorStr(selectors map[string]any, key string) string {
	v, ok := selectors[key].(string)
	if !ok {
		return ""
	}
	return v
}

// selectorRole extracts role and name from the selectors map.
func selectorRole(selectors map[string]any) (role, name string) {
	roleData, ok := selectors["role"]
	if !ok {
		return "", ""
	}
	roleMap, ok := roleData.(map[string]any)
	if !ok {
		return "", ""
	}
	role, _ = roleMap["role"].(string)
	name, _ = roleMap["name"].(string)
	return role, name
}

// ============================================
// Utility Helpers
// ============================================

// escapeJS escapes a string for embedding in JavaScript string literals.
func escapeJS(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `'`, `\'`)
	s = strings.ReplaceAll(s, "\n", `\n`)
	s = strings.ReplaceAll(s, "\r", `\r`)
	s = strings.ReplaceAll(s, "\t", `\t`)
	return s
}

// rewriteURL replaces the origin of a URL with baseURL.
func rewriteURL(originalURL, baseURL string) string {
	parsed, err := url.Parse(originalURL)
	if err != nil {
		return originalURL
	}
	return strings.TrimRight(baseURL, "/") + parsed.Path
}

// collectSelectorTypes returns the unique selector types present across actions.
func collectSelectorTypes(actions []capture.EnhancedAction) []string {
	types := make(map[string]bool)
	for _, a := range actions {
		if a.Selectors == nil {
			continue
		}
		for key := range a.Selectors {
			types[key] = true
		}
	}
	var result []string
	for t := range types {
		result = append(result, t)
	}
	return result
}
