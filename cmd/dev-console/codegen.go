package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// ============================================
// Playwright Script Generation (v5)
// ============================================

// generatePlaywrightScript generates a Playwright test script from enhanced actions
func generatePlaywrightScript(actions []EnhancedAction, errorMessage, baseURL string) string {
	// Determine start URL
	startURL := ""
	if len(actions) > 0 && actions[0].URL != "" {
		startURL = actions[0].URL
	}
	if baseURL != "" && startURL != "" {
		startURL = replaceOrigin(startURL, baseURL)
	}

	// Build test name
	testName := "reproduction: captured user actions"
	if errorMessage != "" {
		name := errorMessage
		if len(name) > 80 {
			name = name[:80]
		}
		testName = "reproduction: " + name
	}

	// Generate steps
	var steps []string
	var prevTimestamp int64

	for i := range actions {
		action := &actions[i]
		// Add pause comment for gaps > 2 seconds
		if prevTimestamp > 0 && action.Timestamp-prevTimestamp > 2000 {
			gap := (action.Timestamp - prevTimestamp) / 1000
			steps = append(steps, fmt.Sprintf("  // [%ds pause]", gap))
		}
		prevTimestamp = action.Timestamp

		locator := getPlaywrightLocator(action.Selectors)

		switch action.Type {
		case "click":
			if locator != "" {
				steps = append(steps, fmt.Sprintf("  await page.%s.click();", locator))
			} else {
				steps = append(steps, "  // click action - no selector available")
			}
		case "input":
			value := action.Value
			if value == "[redacted]" {
				value = "[user-provided]"
			}
			if locator != "" {
				steps = append(steps, fmt.Sprintf("  await page.%s.fill('%s');", locator, escapeJSString(value)))
			}
		case "keypress":
			steps = append(steps, fmt.Sprintf("  await page.keyboard.press('%s');", escapeJSString(action.Key)))
		case "navigate":
			toURL := action.ToURL
			if baseURL != "" && toURL != "" {
				toURL = replaceOrigin(toURL, baseURL)
			}
			steps = append(steps, fmt.Sprintf("  await page.waitForURL('%s');", escapeJSString(toURL)))
		case "select":
			if locator != "" {
				steps = append(steps, fmt.Sprintf("  await page.%s.selectOption('%s');", locator, escapeJSString(action.SelectedValue)))
			}
		case "scroll":
			steps = append(steps, fmt.Sprintf("  // User scrolled to y=%d", action.ScrollY))
		}
	}

	// Assemble script
	script := "import { test, expect } from '@playwright/test';\n\n"
	script += fmt.Sprintf("test('%s', async ({ page }) => {\n", escapeJSString(testName))
	if startURL != "" {
		script += fmt.Sprintf("  await page.goto('%s');\n\n", escapeJSString(startURL))
	}
	script += strings.Join(steps, "\n")
	if len(steps) > 0 {
		script += "\n"
	}
	if errorMessage != "" {
		script += fmt.Sprintf("\n  // Error occurred here: %s\n", errorMessage)
	}
	script += "});\n"

	// Cap output size (50KB)
	if len(script) > 51200 {
		script = script[:51200]
	}

	return script
}

// getPlaywrightLocator returns the best Playwright locator for a set of selectors
// Priority: testId > role > ariaLabel > text > id > cssPath
func getPlaywrightLocator(selectors map[string]interface{}) string {
	if selectors == nil {
		return ""
	}

	if testId, ok := selectors["testId"].(string); ok && testId != "" {
		return fmt.Sprintf("getByTestId('%s')", escapeJSString(testId))
	}

	if roleData, ok := selectors["role"]; ok {
		if roleMap, ok := roleData.(map[string]interface{}); ok {
			role, _ := roleMap["role"].(string)
			name, _ := roleMap["name"].(string)
			if role != "" && name != "" {
				return fmt.Sprintf("getByRole('%s', { name: '%s' })", escapeJSString(role), escapeJSString(name))
			}
			if role != "" {
				return fmt.Sprintf("getByRole('%s')", escapeJSString(role))
			}
		}
	}

	if ariaLabel, ok := selectors["ariaLabel"].(string); ok && ariaLabel != "" {
		return fmt.Sprintf("getByLabel('%s')", escapeJSString(ariaLabel))
	}

	if text, ok := selectors["text"].(string); ok && text != "" {
		return fmt.Sprintf("getByText('%s')", escapeJSString(text))
	}

	if id, ok := selectors["id"].(string); ok && id != "" {
		return fmt.Sprintf("locator('#%s')", escapeJSString(id))
	}

	if cssPath, ok := selectors["cssPath"].(string); ok && cssPath != "" {
		return fmt.Sprintf("locator('%s')", escapeJSString(cssPath))
	}

	return ""
}

// escapeJSString escapes a string for use in JavaScript single-quoted strings
func escapeJSString(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "'", "\\'")
	s = strings.ReplaceAll(s, "\n", "\\n")
	s = strings.ReplaceAll(s, "\r", "\\r")
	return s
}

// replaceOrigin replaces the origin (scheme+host) in a URL with a new base URL
func replaceOrigin(original, baseURL string) string {
	// Find the path start (after scheme://host)
	schemeEnd := strings.Index(original, "://")
	if schemeEnd == -1 {
		return baseURL + original
	}
	rest := original[schemeEnd+3:]
	pathStart := strings.Index(rest, "/")
	if pathStart == -1 {
		return baseURL
	}
	path := rest[pathStart:]
	// Remove trailing slash from baseURL if path starts with /
	base := strings.TrimRight(baseURL, "/")
	return base + path
}

// ============================================
// Session Timeline (v5)
// ============================================

// TimelineFilter defines filtering criteria for timeline queries
type TimelineFilter struct {
	LastNActions int
	URLFilter    string
	Include      []string
}

// TimelineEntry represents a single entry in the session timeline
type TimelineEntry struct {
	Timestamp     int64                  `json:"timestamp"`
	Kind          string                 `json:"kind"`
	Type          string                 `json:"type,omitempty"`
	URL           string                 `json:"url,omitempty"`
	Selectors     map[string]interface{} `json:"selectors,omitempty"`
	Method        string                 `json:"method,omitempty"`
	Status        int                    `json:"status,omitempty"`
	ContentType   string                 `json:"contentType,omitempty"`
	ResponseShape interface{}            `json:"responseShape,omitempty"`
	Message       string                 `json:"message,omitempty"`
	Level         string                 `json:"level,omitempty"`
	ToURL         string                 `json:"toUrl,omitempty"`
	Value         string                 `json:"value,omitempty"`
}

// TimelineSummary provides aggregate stats for the session timeline
type TimelineSummary struct {
	Actions         int   `json:"actions"`
	NetworkRequests int   `json:"networkRequests"`
	ConsoleErrors   int   `json:"consoleErrors"`
	DurationMs      int64 `json:"durationMs"`
}

// TimelineResponse is the internal response from GetSessionTimeline
type TimelineResponse struct {
	Timeline []TimelineEntry `json:"timeline"`
	Summary  TimelineSummary `json:"summary"`
}

// SessionTimelineResponse is the JSON response for the MCP tool
type SessionTimelineResponse struct {
	Timeline []TimelineEntry `json:"timeline"`
	Summary  TimelineSummary `json:"summary"`
}

// TestGenerationOptions configures test script generation
type TestGenerationOptions struct {
	TestName            string `json:"test_name"`
	AssertNetwork       bool   `json:"assert_network"`
	AssertNoErrors      bool   `json:"assert_no_errors"`
	AssertResponseShape bool   `json:"assert_response_shape"`
	BaseURL             string `json:"base_url"`
}

// normalizeTimestamp converts an ISO timestamp string to unix milliseconds
func normalizeTimestamp(ts string) int64 {
	if ts == "" {
		return 0
	}
	formats := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05.000Z",
	}
	for _, format := range formats {
		if t, err := time.Parse(format, ts); err == nil {
			return t.UnixMilli()
		}
	}
	return 0
}

// GetSessionTimeline merges actions, network, and console entries into a sorted timeline
func (v *Capture) GetSessionTimeline(filter TimelineFilter, logEntries []LogEntry) TimelineResponse {
	v.mu.RLock()
	defer v.mu.RUnlock()

	var entries []TimelineEntry

	// Determine action subset
	actions := v.enhancedActions
	if filter.LastNActions > 0 && len(actions) > filter.LastNActions {
		actions = actions[len(actions)-filter.LastNActions:]
	}

	// Determine time boundary
	var minTimestamp int64
	if len(actions) > 0 {
		minTimestamp = actions[0].Timestamp
	}

	// Include check helper
	shouldInclude := func(kind string) bool {
		if len(filter.Include) == 0 {
			return true
		}
		for _, inc := range filter.Include {
			if inc == kind+"s" || inc == kind {
				return true
			}
		}
		return false
	}

	// Add actions
	if shouldInclude("action") {
		for i := range actions {
			if filter.URLFilter != "" && !strings.Contains(actions[i].URL, filter.URLFilter) {
				continue
			}
			entries = append(entries, TimelineEntry{
				Timestamp: actions[i].Timestamp,
				Kind:      "action",
				Type:      actions[i].Type,
				URL:       actions[i].URL,
				Selectors: actions[i].Selectors,
				ToURL:     actions[i].ToURL,
				Value:     actions[i].Value,
			})
		}
	}

	// Add network bodies
	if shouldInclude("network") {
		for _, nb := range v.networkBodies {
			ts := normalizeTimestamp(nb.Timestamp)
			if minTimestamp > 0 && ts < minTimestamp {
				continue
			}
			if filter.URLFilter != "" && !strings.Contains(nb.URL, filter.URLFilter) {
				continue
			}
			entry := TimelineEntry{
				Timestamp:   ts,
				Kind:        "network",
				Method:      nb.Method,
				URL:         nb.URL,
				Status:      nb.Status,
				ContentType: nb.ContentType,
			}
			// Extract response shape for JSON responses
			if strings.Contains(nb.ContentType, "json") && nb.ResponseBody != "" {
				entry.ResponseShape = extractResponseShape(nb.ResponseBody)
			}
			entries = append(entries, entry)
		}
	}

	// Add console entries (error and warn only)
	if shouldInclude("console") {
		for _, le := range logEntries {
			level, _ := le["level"].(string)
			if level != "error" && level != "warn" {
				continue
			}
			ts := normalizeTimestamp(fmt.Sprintf("%v", le["ts"]))
			if minTimestamp > 0 && ts < minTimestamp {
				continue
			}
			msg, _ := le["message"].(string)
			entries = append(entries, TimelineEntry{
				Timestamp: ts,
				Kind:      "console",
				Level:     level,
				Message:   msg,
			})
		}
	}

	// Sort by timestamp
	for i := 1; i < len(entries); i++ {
		for j := i; j > 0 && entries[j].Timestamp < entries[j-1].Timestamp; j-- {
			entries[j], entries[j-1] = entries[j-1], entries[j]
		}
	}

	// Cap at 200
	if len(entries) > 200 {
		entries = entries[:200]
	}

	// Build summary
	summary := TimelineSummary{}
	for i := range entries {
		switch entries[i].Kind {
		case "action":
			summary.Actions++
		case "network":
			summary.NetworkRequests++
		case "console":
			if entries[i].Level == "error" {
				summary.ConsoleErrors++
			}
		}
	}
	if len(entries) >= 2 {
		summary.DurationMs = entries[len(entries)-1].Timestamp - entries[0].Timestamp
	}

	return TimelineResponse{Timeline: entries, Summary: summary}
}

// generateTestScript generates a Playwright test script from a timeline
func generateTestScript(timeline []TimelineEntry, opts TestGenerationOptions) string {
	var sb strings.Builder

	sb.WriteString("import { test, expect } from '@playwright/test'\n\n")

	testName := opts.TestName
	if testName == "" {
		testName = "recorded session"
		if len(timeline) > 0 {
			for i := range timeline {
				if timeline[i].URL != "" {
					testName = timeline[i].URL
					if opts.BaseURL != "" {
						testName = replaceOrigin(testName, opts.BaseURL)
					}
					break
				}
			}
		}
	}

	sb.WriteString(fmt.Sprintf("test('%s', async ({ page }) => {\n", testName))

	if opts.AssertNoErrors {
		sb.WriteString("  const consoleErrors = []\n")
		sb.WriteString("  page.on('console', msg => { if (msg.type() === 'error') consoleErrors.push(msg.text()) })\n\n")
	}

	// Determine start URL
	startURL := ""
	for i := range timeline {
		if timeline[i].Kind == "action" && timeline[i].URL != "" {
			startURL = timeline[i].URL
			break
		}
	}
	if startURL != "" {
		if opts.BaseURL != "" {
			startURL = replaceOrigin(startURL, opts.BaseURL)
		}
		sb.WriteString(fmt.Sprintf("  await page.goto('%s')\n\n", startURL))
	}

	// Track if errors were present in session
	hasErrors := false
	for i := range timeline {
		if timeline[i].Kind == "console" && timeline[i].Level == "error" {
			hasErrors = true
			break
		}
	}

	for i := range timeline {
		entry := &timeline[i]
		switch entry.Kind {
		case "action":
			if entry.Type == "click" && entry.Selectors != nil {
				selector := getSelectorFromMap(entry.Selectors)
				sb.WriteString(fmt.Sprintf("  await page.locator('%s').click()\n", selector))
			} else if entry.Type == "input" {
				value := entry.Value
				if value == "[redacted]" {
					value = "[user-provided]"
				}
				selector := getSelectorFromMap(entry.Selectors)
				sb.WriteString(fmt.Sprintf("  await page.locator('%s').fill('%s')\n", selector, value))
			} else if entry.Type == "navigate" {
				toURL := entry.ToURL
				if toURL == "" {
					toURL = entry.URL
				}
				if opts.BaseURL != "" {
					toURL = replaceOrigin(toURL, opts.BaseURL)
				}
				sb.WriteString(fmt.Sprintf("  await expect(page).toHaveURL(/%s/)\n", strings.TrimPrefix(toURL, "/")))
			}
		case "network":
			if opts.AssertNetwork {
				url := entry.URL
				if opts.BaseURL != "" {
					url = replaceOrigin(url, opts.BaseURL)
				}
				sb.WriteString(fmt.Sprintf("  const response%d = await page.waitForResponse(r => r.url().includes('%s'))\n", i, url))
				sb.WriteString(fmt.Sprintf("  expect(response%d.status()).toBe(%d)\n", i, entry.Status))
				if opts.AssertResponseShape && entry.ResponseShape != nil {
					shapeMap, ok := entry.ResponseShape.(map[string]interface{})
					if ok {
						for key := range shapeMap {
							sb.WriteString(fmt.Sprintf("  expect(await response%d.json()).toHaveProperty('%s')\n", i, key))
						}
					}
				}
			}
		case "console":
			if entry.Level == "error" {
				sb.WriteString(fmt.Sprintf("  // Captured error: %s\n", entry.Message))
			}
		}
	}

	if opts.AssertNoErrors {
		if hasErrors {
			sb.WriteString("\n  // Note: errors were observed during recording\n")
			sb.WriteString("  // expect(consoleErrors).toHaveLength(0)\n")
		} else {
			sb.WriteString("\n  expect(consoleErrors).toHaveLength(0)\n")
		}
	}

	sb.WriteString("})\n")

	return sb.String()
}

func getSelectorFromMap(selectors map[string]interface{}) string {
	if testId, ok := selectors["testId"].(string); ok {
		return fmt.Sprintf("[data-testid=\"%s\"]", testId) //nolint:gocritic // CSS selector needs exact quote format
	}
	if role, ok := selectors["role"].(string); ok {
		return fmt.Sprintf("[role=\"%s\"]", role) //nolint:gocritic // CSS selector needs exact quote format
	}
	return "unknown"
}

// extractResponseShape extracts the type shape of a JSON response (replaces values with type names)
func extractResponseShape(jsonStr string) interface{} {
	var raw interface{}
	if err := json.Unmarshal([]byte(jsonStr), &raw); err != nil {
		return nil
	}
	return extractShape(raw, 0)
}

func extractShape(val interface{}, depth int) interface{} {
	if depth >= 4 {
		return "..."
	}
	switch v := val.(type) {
	case map[string]interface{}:
		result := make(map[string]interface{})
		for key, value := range v {
			result[key] = extractShape(value, depth+1)
		}
		return result
	case []interface{}:
		if len(v) == 0 {
			return []interface{}{}
		}
		return []interface{}{extractShape(v[0], depth+1)}
	case string:
		return "string"
	case float64:
		return "number"
	case bool:
		return "boolean"
	case nil:
		return "null"
	default:
		return "unknown"
	}
}

func (h *ToolHandler) toolGetReproductionScript(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var arguments struct {
		ErrorMessage string `json:"error_message"`
		LastNActions int    `json:"last_n_actions"`
		BaseURL      string `json:"base_url"`
	}
	_ = json.Unmarshal(args, &arguments) // Optional args - zero values are acceptable defaults

	actions := h.capture.GetEnhancedActions(EnhancedActionFilter{})

	if len(actions) == 0 {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpTextResponse("No enhanced actions captured to generate script")}
	}

	// Apply lastNActions filter
	if arguments.LastNActions > 0 && len(actions) > arguments.LastNActions {
		actions = actions[len(actions)-arguments.LastNActions:]
	}

	script := generatePlaywrightScript(actions, arguments.ErrorMessage, arguments.BaseURL)

	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpTextResponse(script)}
}

func (h *ToolHandler) toolGetSessionTimeline(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var arguments struct {
		LastNActions int      `json:"last_n_actions"`
		URLFilter    string   `json:"url_filter"`
		Include      []string `json:"include"`
	}
	_ = json.Unmarshal(args, &arguments) // Optional args - zero values are acceptable defaults

	h.server.mu.RLock()
	entries := make([]LogEntry, len(h.server.entries))
	copy(entries, h.server.entries)
	h.server.mu.RUnlock()

	resp := h.capture.GetSessionTimeline(TimelineFilter{
		LastNActions: arguments.LastNActions,
		URLFilter:    arguments.URLFilter,
		Include:      arguments.Include,
	}, entries)

	respJSON, _ := json.Marshal(SessionTimelineResponse(resp))

	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpTextResponse(string(respJSON))}
}

func (h *ToolHandler) toolGenerateTest(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var arguments struct {
		TestName            string `json:"test_name"`
		AssertNetwork       bool   `json:"assert_network"`
		AssertNoErrors      bool   `json:"assert_no_errors"`
		AssertResponseShape bool   `json:"assert_response_shape"`
		BaseURL             string `json:"base_url"`
	}
	_ = json.Unmarshal(args, &arguments) // Optional args - zero values are acceptable defaults

	h.server.mu.RLock()
	entries := make([]LogEntry, len(h.server.entries))
	copy(entries, h.server.entries)
	h.server.mu.RUnlock()

	resp := h.capture.GetSessionTimeline(TimelineFilter{}, entries)

	if len(resp.Timeline) == 0 {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpTextResponse("No session data available. Navigate and interact with a page first.")}
	}

	script := generateTestScript(resp.Timeline, TestGenerationOptions{
		TestName:            arguments.TestName,
		AssertNetwork:       arguments.AssertNetwork,
		AssertNoErrors:      arguments.AssertNoErrors,
		AssertResponseShape: arguments.AssertResponseShape,
		BaseURL:             arguments.BaseURL,
	})

	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpTextResponse(script)}
}
