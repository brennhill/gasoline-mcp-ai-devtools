// Purpose: Owns reproduction.go runtime behavior and integration logic.
// Docs: docs/features/feature/reproduction-scripts/index.md

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
	params := parseReproParams(args)

	if err := validateReproOutputFormat(params.OutputFormat); err != "" {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(
			ErrInvalidParam, err, "Use 'gasoline' or 'playwright'", withParam("output_format"),
		)}
	}

	allActions := h.capture.GetAllEnhancedActions()
	actions := filterLastN(allActions, params.LastN)

	script := generateReproScript(actions, params)
	result := buildReproResult(script, params, actions, allActions)

	summary := fmt.Sprintf("Reproduction script (%s, %d actions)", params.OutputFormat, len(actions))
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse(summary, result)}
}

func parseReproParams(args json.RawMessage) ReproductionParams {
	var params ReproductionParams
	if len(args) > 0 {
		_ = json.Unmarshal(args, &params)
	}
	if params.OutputFormat == "" {
		params.OutputFormat = "gasoline"
	}
	return params
}

func validateReproOutputFormat(format string) string {
	if format != "gasoline" && format != "playwright" {
		return "Invalid output_format: " + format
	}
	return ""
}

func filterLastN(actions []capture.EnhancedAction, lastN int) []capture.EnhancedAction {
	if lastN > 0 && lastN < len(actions) {
		return actions[len(actions)-lastN:]
	}
	return actions
}

func generateReproScript(actions []capture.EnhancedAction, params ReproductionParams) string {
	switch params.OutputFormat {
	case "playwright":
		return generateReproPlaywrightScript(actions, params)
	default:
		return generateGasolineScript(actions, params)
	}
}

func buildReproResult(script string, params ReproductionParams, actions, allActions []capture.EnhancedAction) ReproductionResult {
	startURL := reproStartURL(actions)
	var durationMs int64
	if len(actions) > 1 {
		durationMs = actions[len(actions)-1].Timestamp - actions[0].Timestamp
	}
	return ReproductionResult{
		Script:      script,
		Format:      params.OutputFormat,
		ActionCount: len(actions),
		DurationMs:  durationMs,
		StartURL:    startURL,
		Metadata: ReproductionMeta{
			GeneratedAt:      time.Now().Format(time.RFC3339),
			SelectorsUsed:    collectSelectorTypes(actions),
			ActionsAvailable: len(allActions),
			ActionsIncluded:  len(actions),
		},
	}
}

func reproStartURL(actions []capture.EnhancedAction) string {
	if len(actions) == 0 {
		return ""
	}
	if actions[0].Type == "navigate" && actions[0].ToURL != "" {
		return actions[0].ToURL
	}
	return actions[0].URL
}

// ============================================
// Gasoline (Natural Language) Format
// ============================================

// generateGasolineScript converts actions to numbered human-readable steps.
func generateGasolineScript(actions []capture.EnhancedAction, opts ReproductionParams) string {
	if len(actions) == 0 {
		return "# No actions captured\n"
	}
	actions = filterLastN(actions, opts.LastN)

	var b strings.Builder
	writeGasolineHeader(&b, actions, opts)
	writeGasolineSteps(&b, actions, opts)

	if opts.ErrorMessage != "" {
		b.WriteString(fmt.Sprintf("\n# Error: %s\n", opts.ErrorMessage))
	}
	return b.String()
}

func writeGasolineHeader(b *strings.Builder, actions []capture.EnhancedAction, opts ReproductionParams) {
	startURL := reproStartURL(actions)
	desc := "captured user actions"
	if opts.ErrorMessage != "" {
		desc = chopString(opts.ErrorMessage, 80)
	}
	fmt.Fprintf(b, "# Reproduction: %s\n", desc)
	fmt.Fprintf(b, "# Captured: %s | %d actions | %s\n\n",
		time.Now().Format(time.RFC3339), len(actions), startURL)
}

func writeGasolineSteps(b *strings.Builder, actions []capture.EnhancedAction, opts ReproductionParams) {
	stepNum := 0
	var prevTs int64
	for _, action := range actions {
		writePauseComment(b, prevTs, action.Timestamp, "   [%ds pause]\n")
		prevTs = action.Timestamp

		line := gasolineStep(action, opts)
		if line == "" {
			continue
		}
		stepNum++
		prefix := ""
		if action.Source == "ai" {
			prefix = "(AI) "
		}
		fmt.Fprintf(b, "%d. %s%s\n", stepNum, prefix, line)
	}
}

func writePauseComment(b *strings.Builder, prevTs, curTs int64, format string) {
	if prevTs > 0 && curTs-prevTs > 2000 {
		gap := (curTs - prevTs) / 1000
		fmt.Fprintf(b, format, gap)
	}
}

// gasolineStep converts a single action to a natural language step.
func gasolineStep(action capture.EnhancedAction, opts ReproductionParams) string {
	switch action.Type {
	case "navigate":
		return gasolineNavigateStep(action, opts)
	case "click":
		return "Click: " + describeElement(action)
	case "input":
		return gasolineInputStep(action)
	case "select":
		return gasolineSelectStep(action)
	case "keypress":
		return "Press: " + action.Key
	case "scroll":
		return fmt.Sprintf("Scroll to: y=%d", action.ScrollY)
	case "scroll_element":
		return "Scroll to element: " + describeElement(action)
	case "refresh":
		return "Refresh page"
	case "back":
		return "Navigate back"
	case "forward":
		return "Navigate forward"
	case "new_tab":
		return gasolineNewTabStep(action, opts)
	case "focus":
		return "Focus: " + describeElement(action)
	default:
		return ""
	}
}

func gasolineNavigateStep(action capture.EnhancedAction, opts ReproductionParams) string {
	toURL := action.ToURL
	if toURL == "" {
		return ""
	}
	if opts.BaseURL != "" {
		toURL = rewriteURL(toURL, opts.BaseURL)
	}
	return "Navigate to: " + toURL
}

func gasolineNewTabStep(action capture.EnhancedAction, opts ReproductionParams) string {
	targetURL := action.URL
	if targetURL == "" {
		return "Open new tab"
	}
	if opts.BaseURL != "" {
		targetURL = rewriteURL(targetURL, opts.BaseURL)
	}
	return "Open new tab: " + targetURL
}

func gasolineInputStep(action capture.EnhancedAction) string {
	value := action.Value
	if value == "[redacted]" {
		value = "[user-provided]"
	}
	return fmt.Sprintf("Type %q into: %s", value, describeElement(action))
}

func gasolineSelectStep(action capture.EnhancedAction) string {
	text := action.SelectedText
	if text == "" {
		text = action.SelectedValue
	}
	return fmt.Sprintf("Select %q from: %s", text, describeElement(action))
}

// ============================================
// Playwright Format
// ============================================

// generateReproPlaywrightScript converts actions to a Playwright test script.
func generateReproPlaywrightScript(actions []capture.EnhancedAction, opts ReproductionParams) string {
	if len(actions) == 0 {
		return "// No actions captured\n"
	}
	actions = filterLastN(actions, opts.LastN)

	var b strings.Builder
	writePlaywrightHeader(&b, opts)
	writePlaywrightSteps(&b, actions, opts)
	writePlaywrightFooter(&b, opts)

	script := b.String()
	if len(script) > maxReproOutputBytes {
		script = script[:maxReproOutputBytes]
	}
	return script
}

func writePlaywrightHeader(b *strings.Builder, opts ReproductionParams) {
	b.WriteString("import { test, expect } from '@playwright/test';\n\n")
	testName := "reproduction: captured user actions"
	if opts.ErrorMessage != "" {
		testName = "reproduction: " + chopString(opts.ErrorMessage, 80)
	}
	fmt.Fprintf(b, "test('%s', async ({ page }) => {\n", escapeJS(testName))
}

func writePlaywrightSteps(b *strings.Builder, actions []capture.EnhancedAction, opts ReproductionParams) {
	var prevTs int64
	for _, action := range actions {
		writePauseComment(b, prevTs, action.Timestamp, "  // [%ds pause]\n")
		prevTs = action.Timestamp
		line := playwrightStep(action, opts)
		if line != "" {
			b.WriteString("  " + line + "\n")
		}
	}
}

func writePlaywrightFooter(b *strings.Builder, opts ReproductionParams) {
	if opts.ErrorMessage != "" {
		fmt.Fprintf(b, "  // Error: %s\n", opts.ErrorMessage)
	}
	b.WriteString("});\n")
}

// playwrightStep converts a single action to a Playwright code line.
func playwrightStep(action capture.EnhancedAction, opts ReproductionParams) string {
	switch action.Type {
	case "navigate":
		return pwNavigateStep(action, opts)
	case "click":
		return pwLocatorAction(action, "click", "click")
	case "input":
		return pwInputStep(action)
	case "select":
		return pwSelectStep(action)
	case "keypress":
		return fmt.Sprintf("await page.keyboard.press('%s');", escapeJS(action.Key))
	case "scroll":
		return fmt.Sprintf("// Scroll to y=%d", action.ScrollY)
	case "scroll_element":
		return pwLocatorAction(action, "scrollIntoViewIfNeeded", "scroll element into view")
	case "refresh":
		return "await page.reload();"
	case "back":
		return "await page.goBack();"
	case "forward":
		return "await page.goForward();"
	case "new_tab":
		return pwNewTabStep(action, opts)
	case "focus":
		return pwLocatorAction(action, "focus", "focus")
	default:
		return ""
	}
}

func pwNavigateStep(action capture.EnhancedAction, opts ReproductionParams) string {
	toURL := action.ToURL
	if toURL == "" {
		return ""
	}
	if opts.BaseURL != "" {
		toURL = rewriteURL(toURL, opts.BaseURL)
	}
	return fmt.Sprintf("await page.goto('%s');", escapeJS(toURL))
}

func pwNewTabStep(action capture.EnhancedAction, opts ReproductionParams) string {
	targetURL := action.URL
	if targetURL == "" {
		return "// Open new tab"
	}
	if opts.BaseURL != "" {
		targetURL = rewriteURL(targetURL, opts.BaseURL)
	}
	return fmt.Sprintf("// Open new tab: %s", escapeJS(targetURL))
}

func pwLocatorAction(action capture.EnhancedAction, actionName, fallbackLabel string) string {
	loc := playwrightLocator(action.Selectors)
	if loc == "" {
		return fmt.Sprintf("// %s - no selector available", fallbackLabel)
	}
	return fmt.Sprintf("await page.%s.%s();", loc, actionName)
}

func pwInputStep(action capture.EnhancedAction) string {
	loc := playwrightLocator(action.Selectors)
	if loc == "" {
		return "// input - no selector available"
	}
	value := action.Value
	if value == "[redacted]" {
		value = "[user-provided]"
	}
	return fmt.Sprintf("await page.%s.fill('%s');", loc, escapeJS(value))
}

func pwSelectStep(action capture.EnhancedAction) string {
	loc := playwrightLocator(action.Selectors)
	if loc == "" {
		return "// select - no selector available"
	}
	return fmt.Sprintf("await page.%s.selectOption('%s');", loc, escapeJS(action.SelectedValue))
}

// ============================================
// Selector Helpers
// ============================================

type elementCandidate struct {
	label string
	ok    bool
}

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

	desc := describeWithRole(text, ariaLabel, roleName, role)
	if desc != "" {
		return desc
	}
	return describeWithoutRole(testID, text, ariaLabel, id, cssPath)
}

func describeWithRole(text, ariaLabel, roleName, role string) string {
	if role == "" {
		return ""
	}
	candidates := []elementCandidate{
		{text, text != ""},
		{ariaLabel, ariaLabel != ""},
		{roleName, roleName != ""},
	}
	for _, c := range candidates {
		if c.ok {
			return fmt.Sprintf("%q %s", c.label, role)
		}
	}
	return ""
}

func describeWithoutRole(testID, text, ariaLabel, id, cssPath string) string {
	if testID != "" {
		return fmt.Sprintf("[data-testid=%q]", testID)
	}
	if text != "" {
		return fmt.Sprintf("%q", text)
	}
	if ariaLabel != "" {
		return fmt.Sprintf("%q", ariaLabel)
	}
	if id != "" {
		return "#" + id
	}
	if cssPath != "" {
		return cssPath
	}
	return "(unknown element)"
}

type pwLocatorCandidate struct {
	value  string
	format func(string) string
}

// playwrightLocator returns the best Playwright locator string for a selector map.
// Priority: testId > role > ariaLabel > text > id > cssPath
func playwrightLocator(selectors map[string]any) string {
	if selectors == nil {
		return ""
	}

	// Role has special handling for optional name parameter.
	role, roleName := selectorRole(selectors)
	if loc := pwRoleLocator(role, roleName); loc != "" {
		// Role is priority 2; check testId first.
		testID := selectorStr(selectors, "testId")
		if testID != "" {
			return fmt.Sprintf("getByTestId('%s')", escapeJS(testID))
		}
		return loc
	}

	candidates := []pwLocatorCandidate{
		{selectorStr(selectors, "testId"), func(v string) string { return fmt.Sprintf("getByTestId('%s')", escapeJS(v)) }},
		{selectorStr(selectors, "ariaLabel"), func(v string) string { return fmt.Sprintf("getByLabel('%s')", escapeJS(v)) }},
		{selectorStr(selectors, "text"), func(v string) string { return fmt.Sprintf("getByText('%s')", escapeJS(v)) }},
		{selectorStr(selectors, "id"), func(v string) string { return fmt.Sprintf("locator('#%s')", escapeJS(v)) }},
		{selectorStr(selectors, "cssPath"), func(v string) string { return fmt.Sprintf("locator('%s')", escapeJS(v)) }},
	}
	for _, c := range candidates {
		if c.value != "" {
			return c.format(c.value)
		}
	}
	return ""
}

func pwRoleLocator(role, roleName string) string {
	if role == "" {
		return ""
	}
	if roleName != "" {
		return fmt.Sprintf("getByRole('%s', { name: '%s' })", escapeJS(role), escapeJS(roleName))
	}
	return fmt.Sprintf("getByRole('%s')", escapeJS(role))
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

func chopString(s string, maxLen int) string {
	if len(s) > maxLen {
		return s[:maxLen]
	}
	return s
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
