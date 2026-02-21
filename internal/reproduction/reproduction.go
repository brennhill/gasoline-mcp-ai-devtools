// reproduction.go — Reproduction script generation from captured actions.
// Generates Playwright tests or Gasoline natural language scripts from
// EnhancedAction data captured by the browser extension.
// Design: Two output formats, shared selector extraction, single-pass generation.
package reproduction

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

// Params are the parsed arguments for generate({format: "reproduction"}).
type Params struct {
	Format             string `json:"format"`
	OutputFormat       string `json:"output_format"`
	LastN              int    `json:"last_n"`
	BaseURL            string `json:"base_url"`
	IncludeScreenshots bool   `json:"include_screenshots"`
	ErrorMessage       string `json:"error_message"`
}

// Result is the response payload.
type Result struct {
	Script      string `json:"script"`
	Format      string `json:"format"`
	ActionCount int    `json:"action_count"`
	DurationMs  int64  `json:"duration_ms"`
	StartURL    string `json:"start_url"`
	Metadata    Meta   `json:"metadata"`
}

// Meta provides traceability for the generated script.
type Meta struct {
	GeneratedAt      string   `json:"generated_at"`
	SelectorsUsed    []string `json:"selectors_used"`
	ActionsAvailable int      `json:"actions_available"`
	ActionsIncluded  int      `json:"actions_included"`
}

const maxReproOutputBytes = 200 * 1024 // 200KB cap

// ============================================
// Entry Point Helpers
// ============================================

// ParseParams unmarshals and defaults the reproduction parameters.
func ParseParams(args json.RawMessage) Params {
	var params Params
	if len(args) > 0 {
		_ = json.Unmarshal(args, &params)
	}
	if params.OutputFormat == "" {
		params.OutputFormat = "playwright"
	}
	return params
}

// ValidateOutputFormat returns an error message if format is invalid, empty string if OK.
func ValidateOutputFormat(format string) string {
	if format != "gasoline" && format != "playwright" {
		return "Invalid output_format: " + format
	}
	return ""
}

// FilterLastN returns the last N actions, or all if lastN <= 0.
func FilterLastN(actions []capture.EnhancedAction, lastN int) []capture.EnhancedAction {
	if lastN > 0 && lastN < len(actions) {
		return actions[len(actions)-lastN:]
	}
	return actions
}

// GenerateScript dispatches to the correct format generator.
func GenerateScript(actions []capture.EnhancedAction, params Params) string {
	switch params.OutputFormat {
	case "playwright":
		return GeneratePlaywrightScript(actions, params)
	default:
		return GenerateGasolineScript(actions, params)
	}
}

// BuildResult assembles the response payload from a generated script.
func BuildResult(script string, params Params, actions, allActions []capture.EnhancedAction) Result {
	startURL := reproStartURL(actions)
	var durationMs int64
	if len(actions) > 1 {
		durationMs = actions[len(actions)-1].Timestamp - actions[0].Timestamp
	}
	return Result{
		Script:      script,
		Format:      params.OutputFormat,
		ActionCount: len(actions),
		DurationMs:  durationMs,
		StartURL:    startURL,
		Metadata: Meta{
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

// GenerateGasolineScript converts actions to numbered human-readable steps.
func GenerateGasolineScript(actions []capture.EnhancedAction, opts Params) string {
	if len(actions) == 0 {
		return "# No actions captured\n"
	}
	actions = FilterLastN(actions, opts.LastN)

	var b strings.Builder
	writeGasolineHeader(&b, actions, opts)
	writeGasolineSteps(&b, actions, opts)

	if opts.ErrorMessage != "" {
		b.WriteString(fmt.Sprintf("\n# Error: %s\n", opts.ErrorMessage))
	}
	return b.String()
}

func writeGasolineHeader(b *strings.Builder, actions []capture.EnhancedAction, opts Params) {
	startURL := reproStartURL(actions)
	desc := "captured user actions"
	if opts.ErrorMessage != "" {
		desc = ChopString(opts.ErrorMessage, 80)
	}
	fmt.Fprintf(b, "# Reproduction: %s\n", desc)
	fmt.Fprintf(b, "# Captured: %s | %d actions | %s\n\n",
		time.Now().Format(time.RFC3339), len(actions), startURL)
}

func writeGasolineSteps(b *strings.Builder, actions []capture.EnhancedAction, opts Params) {
	stepNum := 0
	var prevTs int64
	for _, action := range actions {
		WritePauseComment(b, prevTs, action.Timestamp, "   [%ds pause]\n")
		prevTs = action.Timestamp

		line := GasolineStep(action, opts)
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

// WritePauseComment writes a timing pause comment if the gap exceeds 2 seconds.
func WritePauseComment(b *strings.Builder, prevTs, curTs int64, format string) {
	if prevTs > 0 && curTs-prevTs > 2000 {
		gap := (curTs - prevTs) / 1000
		fmt.Fprintf(b, format, gap)
	}
}

// GasolineStep converts a single action to a natural language step.
func GasolineStep(action capture.EnhancedAction, opts Params) string {
	switch action.Type {
	case "navigate":
		return gasolineNavigateStep(action, opts)
	case "click":
		return "Click: " + DescribeElement(action)
	case "input":
		return gasolineInputStep(action)
	case "select":
		return gasolineSelectStep(action)
	case "keypress":
		return "Press: " + action.Key
	case "scroll":
		return fmt.Sprintf("Scroll to: y=%d", action.ScrollY)
	case "scroll_element":
		return "Scroll to element: " + DescribeElement(action)
	case "refresh":
		return "Refresh page"
	case "back":
		return "Navigate back"
	case "forward":
		return "Navigate forward"
	case "new_tab":
		return gasolineNewTabStep(action, opts)
	case "focus":
		return "Focus: " + DescribeElement(action)
	default:
		return ""
	}
}

func gasolineNavigateStep(action capture.EnhancedAction, opts Params) string {
	toURL := action.ToURL
	if toURL == "" {
		return ""
	}
	if opts.BaseURL != "" {
		toURL = RewriteURL(toURL, opts.BaseURL)
	}
	return "Navigate to: " + toURL
}

func gasolineNewTabStep(action capture.EnhancedAction, opts Params) string {
	targetURL := action.URL
	if targetURL == "" {
		return "Open new tab"
	}
	if opts.BaseURL != "" {
		targetURL = RewriteURL(targetURL, opts.BaseURL)
	}
	return "Open new tab: " + targetURL
}

func gasolineInputStep(action capture.EnhancedAction) string {
	value := action.Value
	if value == "[redacted]" {
		value = "[user-provided]"
	}
	return fmt.Sprintf("Type %q into: %s", value, DescribeElement(action))
}

func gasolineSelectStep(action capture.EnhancedAction) string {
	text := action.SelectedText
	if text == "" {
		text = action.SelectedValue
	}
	return fmt.Sprintf("Select %q from: %s", text, DescribeElement(action))
}

// ============================================
// Playwright Format
// ============================================

// GeneratePlaywrightScript converts actions to a Playwright test script.
func GeneratePlaywrightScript(actions []capture.EnhancedAction, opts Params) string {
	if len(actions) == 0 {
		return "// No actions captured\n"
	}
	actions = FilterLastN(actions, opts.LastN)

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

func writePlaywrightHeader(b *strings.Builder, opts Params) {
	b.WriteString("import { test, expect } from '@playwright/test';\n\n")
	testName := "reproduction: captured user actions"
	if opts.ErrorMessage != "" {
		testName = "reproduction: " + ChopString(opts.ErrorMessage, 80)
	}
	fmt.Fprintf(b, "test('%s', async ({ page }) => {\n", EscapeJS(testName))
}

func writePlaywrightSteps(b *strings.Builder, actions []capture.EnhancedAction, opts Params) {
	var prevTs int64
	for _, action := range actions {
		WritePauseComment(b, prevTs, action.Timestamp, "  // [%ds pause]\n")
		prevTs = action.Timestamp
		line := PlaywrightStep(action, opts)
		if line != "" {
			b.WriteString("  " + line + "\n")
		}
	}
}

func writePlaywrightFooter(b *strings.Builder, opts Params) {
	if opts.ErrorMessage != "" {
		fmt.Fprintf(b, "  // Error: %s\n", opts.ErrorMessage)
	}
	b.WriteString("});\n")
}

// PlaywrightStep converts a single action to a Playwright code line.
func PlaywrightStep(action capture.EnhancedAction, opts Params) string {
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
		return fmt.Sprintf("await page.keyboard.press('%s');", EscapeJS(action.Key))
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

func pwNavigateStep(action capture.EnhancedAction, opts Params) string {
	toURL := action.ToURL
	if toURL == "" {
		return ""
	}
	if opts.BaseURL != "" {
		toURL = RewriteURL(toURL, opts.BaseURL)
	}
	return fmt.Sprintf("await page.goto('%s');", EscapeJS(toURL))
}

func pwNewTabStep(action capture.EnhancedAction, opts Params) string {
	targetURL := action.URL
	if targetURL == "" {
		return "// Open new tab"
	}
	if opts.BaseURL != "" {
		targetURL = RewriteURL(targetURL, opts.BaseURL)
	}
	return fmt.Sprintf("// Open new tab: %s", EscapeJS(targetURL))
}

func pwLocatorAction(action capture.EnhancedAction, actionName, fallbackLabel string) string {
	loc := PlaywrightLocator(action.Selectors)
	if loc == "" {
		return fmt.Sprintf("// %s - no selector available", fallbackLabel)
	}
	return fmt.Sprintf("await page.%s.%s();", loc, actionName)
}

func pwInputStep(action capture.EnhancedAction) string {
	loc := PlaywrightLocator(action.Selectors)
	if loc == "" {
		return "// input - no selector available"
	}
	value := action.Value
	if value == "[redacted]" {
		value = "[user-provided]"
	}
	return fmt.Sprintf("await page.%s.fill('%s');", loc, EscapeJS(value))
}

func pwSelectStep(action capture.EnhancedAction) string {
	loc := PlaywrightLocator(action.Selectors)
	if loc == "" {
		return "// select - no selector available"
	}
	return fmt.Sprintf("await page.%s.selectOption('%s');", loc, EscapeJS(action.SelectedValue))
}

// ============================================
// Selector Helpers
// ============================================

type elementCandidate struct {
	label string
	ok    bool
}

// DescribeElement returns the most human-readable description of the target element.
// Priority: text+role > ariaLabel+role > role.name+role > testId > text > ariaLabel > id > cssPath
func DescribeElement(action capture.EnhancedAction) string {
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

// PlaywrightLocator returns the best Playwright locator string for a selector map.
// Priority: testId > role > ariaLabel > text > id > cssPath
func PlaywrightLocator(selectors map[string]any) string {
	if selectors == nil {
		return ""
	}

	// Role has special handling for optional name parameter.
	role, roleName := selectorRole(selectors)
	if loc := pwRoleLocator(role, roleName); loc != "" {
		// Role is priority 2; check testId first.
		testID := selectorStr(selectors, "testId")
		if testID != "" {
			return fmt.Sprintf("getByTestId('%s')", EscapeJS(testID))
		}
		return loc
	}

	candidates := []pwLocatorCandidate{
		{selectorStr(selectors, "testId"), func(v string) string { return fmt.Sprintf("getByTestId('%s')", EscapeJS(v)) }},
		{selectorStr(selectors, "ariaLabel"), func(v string) string { return fmt.Sprintf("getByLabel('%s')", EscapeJS(v)) }},
		{selectorStr(selectors, "text"), func(v string) string { return fmt.Sprintf("getByText('%s')", EscapeJS(v)) }},
		{selectorStr(selectors, "id"), func(v string) string { return fmt.Sprintf("locator('#%s')", EscapeJS(v)) }},
		{selectorStr(selectors, "cssPath"), func(v string) string { return fmt.Sprintf("locator('%s')", EscapeJS(v)) }},
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
		return fmt.Sprintf("getByRole('%s', { name: '%s' })", EscapeJS(role), EscapeJS(roleName))
	}
	return fmt.Sprintf("getByRole('%s')", EscapeJS(role))
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

// EscapeJS escapes a string for embedding in JavaScript string literals.
func EscapeJS(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `'`, `\'`)
	s = strings.ReplaceAll(s, "\n", `\n`)
	s = strings.ReplaceAll(s, "\r", `\r`)
	s = strings.ReplaceAll(s, "\t", `\t`)
	return s
}

// RewriteURL replaces the origin of a URL with baseURL.
func RewriteURL(originalURL, baseURL string) string {
	parsed, err := url.Parse(originalURL)
	if err != nil {
		return originalURL
	}
	return strings.TrimRight(baseURL, "/") + parsed.Path
}

// ChopString truncates s to maxLen characters.
func ChopString(s string, maxLen int) string {
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
