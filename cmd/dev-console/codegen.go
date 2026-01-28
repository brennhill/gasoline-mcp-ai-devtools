// codegen.go — Playwright script generation, test generation, and session timeline.
// Produces runnable Playwright scripts from captured user actions, with
// smart locator selection (testId > role > label > text > id > cssPath).
// Also merges actions, network, and console into a sorted session timeline.
// Design: Scripts capped at 50KB. Timing gaps annotated as comments.
// Test generation adds network assertions and response shape validation.
package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// ============================================
// Playwright Script Generation
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
// Priority: testID > role > ariaLabel > text > id > cssPath
func getPlaywrightLocator(selectors map[string]interface{}) string {
	if selectors == nil {
		return ""
	}

	if testID, ok := selectors["testId"].(string); ok && testID != "" {
		return fmt.Sprintf("getByTestId('%s')", escapeJSString(testID))
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
// Session Timeline
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
	ContentType   string                 `json:"content_type,omitempty"`  
	ResponseShape interface{}            `json:"response_shape,omitempty"`
	Message       string                 `json:"message,omitempty"`
	Level         string                 `json:"level,omitempty"`
	ToURL         string                 `json:"to_url,omitempty"`        
	Value         string                 `json:"value,omitempty"`
}

// TimelineSummary provides aggregate stats for the session timeline
type TimelineSummary struct {
	Actions         int   `json:"actions"`
	NetworkRequests int   `json:"network_requests"`
	ConsoleErrors   int   `json:"console_errors"`  
	DurationMs      int64 `json:"duration_ms"`     
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
func (c *Capture) GetSessionTimeline(filter TimelineFilter, logEntries []LogEntry) TimelineResponse {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var entries []TimelineEntry

	// Determine action subset
	actions := c.enhancedActions
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
		for _, nb := range c.networkBodies {
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
	if testID, ok := selectors["testId"].(string); ok {
		return fmt.Sprintf("[data-testid=\"%s\"]", testID) //nolint:gocritic // CSS selector needs exact quote format
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
		ErrorMessage       string `json:"error_message"`
		LastNActions       int    `json:"last_n"`
		BaseURL            string `json:"base_url"`
		IncludeScreenshots bool   `json:"include_screenshots"`
		GenerateFixtures   bool   `json:"generate_fixtures"`
		VisualAssertions   bool   `json:"visual_assertions"`
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

	// Check if any enhanced options are enabled
	hasEnhancedOptions := arguments.IncludeScreenshots || arguments.GenerateFixtures || arguments.VisualAssertions

	if !hasEnhancedOptions {
		// Use original simple generation
		script := generatePlaywrightScript(actions, arguments.ErrorMessage, arguments.BaseURL)
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpTextResponse(script)}
	}

	// Get network bodies for fixture generation
	var networkBodies []NetworkBody
	if arguments.GenerateFixtures {
		networkBodies = h.capture.GetNetworkBodies(NetworkBodyFilter{})
	}

	// Use enhanced generation
	opts := ReproductionOptions{
		ErrorMessage:       arguments.ErrorMessage,
		LastNActions:       arguments.LastNActions,
		BaseURL:            arguments.BaseURL,
		IncludeScreenshots: arguments.IncludeScreenshots,
		GenerateFixtures:   arguments.GenerateFixtures,
		VisualAssertions:   arguments.VisualAssertions,
	}

	result := generateEnhancedPlaywrightScript(actions, networkBodies, opts)

	// If fixtures were generated, return multiple content blocks
	if arguments.GenerateFixtures && len(result.Fixtures) > 0 {
		fixturesJSON, _ := json.MarshalIndent(result.Fixtures, "", "  ")
		mcpResult := MCPToolResult{
			Content: []MCPContentBlock{
				{Type: "text", Text: result.Script},
				{Type: "text", Text: string(fixturesJSON)},
			},
		}
		resultJSON, _ := json.Marshal(mcpResult)
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: json.RawMessage(resultJSON)}
	}

	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpTextResponse(result.Script)}
}

func (h *ToolHandler) toolGetSessionTimeline(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var arguments struct {
		LastNActions int      `json:"last_n"`
		URLFilter    string   `json:"url"`
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

// ============================================
// Workflow Integration (Session Summary + PR Summary)
// ============================================

// TrackPerformanceSnapshot records a performance snapshot with session tracking.
// It wraps AddPerformanceSnapshot to also record the first snapshot per URL for delta computation.
func (c *Capture) TrackPerformanceSnapshot(snapshot PerformanceSnapshot) {
	c.mu.Lock()
	if _, exists := c.session.firstSnapshots[snapshot.URL]; !exists {
		c.session.firstSnapshots[snapshot.URL] = snapshot
	}
	c.session.snapshotCount++
	c.mu.Unlock()

	c.AddPerformanceSnapshot(snapshot)
}

// GenerateSessionSummary compiles a session summary from performance snapshots and actions.
func (c *Capture) GenerateSessionSummary() SessionSummary {
	return c.GenerateSessionSummaryWithEntries(nil)
}

// GenerateSessionSummaryWithEntries compiles a session summary including console error analysis.
func (c *Capture) GenerateSessionSummaryWithEntries(entries []LogEntry) SessionSummary {
	c.mu.RLock()
	defer c.mu.RUnlock()

	summary := SessionSummary{
		Status: "ok",
	}

	// Compute metadata from enhanced actions
	if len(c.enhancedActions) > 0 {
		first := c.enhancedActions[0].Timestamp
		last := c.enhancedActions[len(c.enhancedActions)-1].Timestamp
		summary.Metadata.DurationMs = last - first

		for i := range c.enhancedActions {
			if c.enhancedActions[i].Type == "navigate" {
				summary.Metadata.ReloadCount++
			}
		}
	}

	// Performance delta computation
	snapshotCount := len(c.perf.snapshotOrder)
	if snapshotCount == 0 {
		summary.Status = "no_performance_data"
		return summary
	}
	if c.session.snapshotCount == 1 || (c.session.snapshotCount == 0 && snapshotCount == 1) {
		summary.Status = "insufficient_data"
		return summary
	}

	summary.Metadata.PerformanceCheckCount = c.session.snapshotCount
	if summary.Metadata.PerformanceCheckCount == 0 {
		summary.Metadata.PerformanceCheckCount = snapshotCount
	}

	// Get latest snapshot
	latestURL := c.perf.snapshotOrder[len(c.perf.snapshotOrder)-1]
	latestSnapshot := c.perf.snapshots[latestURL]

	// Get first snapshot (session-tracked or baseline fallback)
	firstSnapshot, hasFirst := c.session.firstSnapshots[latestURL]
	if !hasFirst {
		baseline, hasBaseline := c.perf.baselines[latestURL]
		if hasBaseline && baseline.SampleCount >= 2 {
			firstSnapshot = PerformanceSnapshot{
				URL: latestURL,
				Timing: PerformanceTiming{
					Load:                   baseline.Timing.Load,
					FirstContentfulPaint:   baseline.Timing.FirstContentfulPaint,
					LargestContentfulPaint: baseline.Timing.LargestContentfulPaint,
					TimeToFirstByte:        baseline.Timing.TimeToFirstByte,
					DomContentLoaded:       baseline.Timing.DomContentLoaded,
					DomInteractive:         baseline.Timing.DomInteractive,
				},
				Network: NetworkSummary{
					TransferSize: baseline.Network.TransferSize,
					RequestCount: baseline.Network.RequestCount,
				},
				CLS: baseline.CLS,
			}
			hasFirst = true
		}
	}

	if !hasFirst {
		summary.Status = "insufficient_data"
		return summary
	}

	// Compute delta between first and latest
	delta := &PerformanceDelta{}
	delta.LoadTimeBefore = firstSnapshot.Timing.Load
	delta.LoadTimeAfter = latestSnapshot.Timing.Load
	delta.LoadTimeDelta = latestSnapshot.Timing.Load - firstSnapshot.Timing.Load

	if latestSnapshot.Timing.FirstContentfulPaint != nil && firstSnapshot.Timing.FirstContentfulPaint != nil {
		delta.FCPBefore = *firstSnapshot.Timing.FirstContentfulPaint
		delta.FCPAfter = *latestSnapshot.Timing.FirstContentfulPaint
		delta.FCPDelta = *latestSnapshot.Timing.FirstContentfulPaint - *firstSnapshot.Timing.FirstContentfulPaint
	}

	if latestSnapshot.Timing.LargestContentfulPaint != nil && firstSnapshot.Timing.LargestContentfulPaint != nil {
		delta.LCPBefore = *firstSnapshot.Timing.LargestContentfulPaint
		delta.LCPAfter = *latestSnapshot.Timing.LargestContentfulPaint
		delta.LCPDelta = *latestSnapshot.Timing.LargestContentfulPaint - *firstSnapshot.Timing.LargestContentfulPaint
	}

	if latestSnapshot.CLS != nil && firstSnapshot.CLS != nil {
		delta.CLSBefore = *firstSnapshot.CLS
		delta.CLSAfter = *latestSnapshot.CLS
		delta.CLSDelta = *latestSnapshot.CLS - *firstSnapshot.CLS
	}

	delta.BundleSizeBefore = firstSnapshot.Network.TransferSize
	delta.BundleSizeAfter = latestSnapshot.Network.TransferSize
	delta.BundleSizeDelta = latestSnapshot.Network.TransferSize - firstSnapshot.Network.TransferSize

	summary.PerformanceDelta = delta

	// Extract errors from console entries
	for _, entry := range entries {
		level, _ := entry["level"].(string)
		if level != "error" {
			continue
		}
		msg, _ := entry["message"].(string)
		source, _ := entry["source"].(string)
		summary.Errors = append(summary.Errors, SessionError{
			Message: msg,
			Source:  source,
		})
	}

	return summary
}

// GeneratePRSummary generates a markdown-formatted performance summary for PR descriptions.
func (c *Capture) GeneratePRSummary(errors []SessionError) string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var sb strings.Builder

	snapshotCount := len(c.perf.snapshotOrder)
	if snapshotCount == 0 {
		sb.WriteString("## Performance Impact\n\n")
		sb.WriteString("No performance data collected during this session.\n\n")
		sb.WriteString("---\n*Generated by Gasoline*\n")
		return sb.String()
	}

	// Get latest snapshot and first snapshot
	latestURL := c.perf.snapshotOrder[len(c.perf.snapshotOrder)-1]
	latestSnapshot := c.perf.snapshots[latestURL]

	firstSnapshot, hasFirst := c.session.firstSnapshots[latestURL]
	if !hasFirst {
		baseline, hasBaseline := c.perf.baselines[latestURL]
		if hasBaseline && baseline.SampleCount >= 2 {
			firstSnapshot = PerformanceSnapshot{
				URL: latestURL,
				Timing: PerformanceTiming{
					Load:                   baseline.Timing.Load,
					FirstContentfulPaint:   baseline.Timing.FirstContentfulPaint,
					LargestContentfulPaint: baseline.Timing.LargestContentfulPaint,
				},
				Network: NetworkSummary{
					TransferSize: baseline.Network.TransferSize,
				},
				CLS: baseline.CLS,
			}
			hasFirst = true
		}
	}

	sb.WriteString("## Performance Impact\n\n")

	if !hasFirst || (c.session.snapshotCount < 2 && snapshotCount < 2) {
		sb.WriteString("No performance data collected during this session.\n\n")
		sb.WriteString("---\n*Generated by Gasoline*\n")
		return sb.String()
	}

	sb.WriteString("| Metric | Before | After | Delta |\n")
	sb.WriteString("|--------|--------|-------|-------|\n")

	// Load Time
	loadDelta := latestSnapshot.Timing.Load - firstSnapshot.Timing.Load
	loadPct := 0.0
	if firstSnapshot.Timing.Load > 0 {
		loadPct = loadDelta / firstSnapshot.Timing.Load * 100
	}
	sb.WriteString(fmt.Sprintf("| Load Time | %.1fs | %.1fs | %s |\n",
		firstSnapshot.Timing.Load/1000, latestSnapshot.Timing.Load/1000,
		formatDeltaMs(loadDelta, loadPct)))

	// FCP
	if latestSnapshot.Timing.FirstContentfulPaint != nil && firstSnapshot.Timing.FirstContentfulPaint != nil {
		fcpDelta := *latestSnapshot.Timing.FirstContentfulPaint - *firstSnapshot.Timing.FirstContentfulPaint
		fcpPct := 0.0
		if *firstSnapshot.Timing.FirstContentfulPaint > 0 {
			fcpPct = fcpDelta / *firstSnapshot.Timing.FirstContentfulPaint * 100
		}
		sb.WriteString(fmt.Sprintf("| FCP | %.1fs | %.1fs | %s |\n",
			*firstSnapshot.Timing.FirstContentfulPaint/1000, *latestSnapshot.Timing.FirstContentfulPaint/1000,
			formatDeltaMs(fcpDelta, fcpPct)))
	}

	// LCP
	if latestSnapshot.Timing.LargestContentfulPaint != nil && firstSnapshot.Timing.LargestContentfulPaint != nil {
		lcpDelta := *latestSnapshot.Timing.LargestContentfulPaint - *firstSnapshot.Timing.LargestContentfulPaint
		lcpPct := 0.0
		if *firstSnapshot.Timing.LargestContentfulPaint > 0 {
			lcpPct = lcpDelta / *firstSnapshot.Timing.LargestContentfulPaint * 100
		}
		sb.WriteString(fmt.Sprintf("| LCP | %.1fs | %.1fs | %s |\n",
			*firstSnapshot.Timing.LargestContentfulPaint/1000, *latestSnapshot.Timing.LargestContentfulPaint/1000,
			formatDeltaMs(lcpDelta, lcpPct)))
	}

	// CLS
	if latestSnapshot.CLS != nil && firstSnapshot.CLS != nil {
		clsDelta := *latestSnapshot.CLS - *firstSnapshot.CLS
		if clsDelta == 0 {
			sb.WriteString(fmt.Sprintf("| CLS | %.2f | %.2f | — |\n",
				*firstSnapshot.CLS, *latestSnapshot.CLS))
		} else {
			sb.WriteString(fmt.Sprintf("| CLS | %.2f | %.2f | %+.2f |\n",
				*firstSnapshot.CLS, *latestSnapshot.CLS, clsDelta))
		}
	}

	// Bundle Size
	sizeDelta := latestSnapshot.Network.TransferSize - firstSnapshot.Network.TransferSize
	sizePct := 0.0
	if firstSnapshot.Network.TransferSize > 0 {
		sizePct = float64(sizeDelta) / float64(firstSnapshot.Network.TransferSize) * 100
	}
	sb.WriteString(fmt.Sprintf("| Bundle Size | %s | %s | %s |\n",
		formatBytes(firstSnapshot.Network.TransferSize), formatBytes(latestSnapshot.Network.TransferSize),
		formatDeltaBytes(sizeDelta, sizePct)))

	sb.WriteString("\n")

	// Errors section
	if errors != nil {
		sb.WriteString("### Errors\n")
		if len(errors) == 0 {
			sb.WriteString("- No errors detected\n")
		} else {
			for _, e := range errors {
				if e.Resolved {
					src := ""
					if e.Source != "" {
						src = " (" + e.Source + ")"
					}
					sb.WriteString(fmt.Sprintf("- **Fixed**: `%s`%s\n", e.Message, src))
				}
			}
			newCount := 0
			for _, e := range errors {
				if !e.Resolved {
					newCount++
					src := ""
					if e.Source != "" {
						src = " (" + e.Source + ")"
					}
					sb.WriteString(fmt.Sprintf("- **New**: `%s`%s\n", e.Message, src))
				}
			}
			if newCount == 0 {
				sb.WriteString("- **New**: None\n")
			}
		}
		sb.WriteString("\n")
	}

	sb.WriteString("---\n")
	totalSamples := c.session.snapshotCount
	if totalSamples == 0 {
		totalSamples = snapshotCount
	}
	sb.WriteString(fmt.Sprintf("*Generated by Gasoline from %d performance samples*\n", totalSamples))

	return sb.String()
}

// GenerateOneLiner generates a compact one-liner summary for git hook annotations.
func (c *Capture) GenerateOneLiner(errors []SessionError) string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var parts []string

	snapshotCount := len(c.perf.snapshotOrder)
	if c.session.snapshotCount < 2 && snapshotCount < 2 {
		parts = append(parts, "no perf data")
	} else {
		latestURL := c.perf.snapshotOrder[len(c.perf.snapshotOrder)-1]
		latestSnapshot := c.perf.snapshots[latestURL]

		firstSnapshot, hasFirst := c.session.firstSnapshots[latestURL]
		if !hasFirst {
			baseline, hasBaseline := c.perf.baselines[latestURL]
			if hasBaseline && baseline.SampleCount >= 2 {
				firstSnapshot = PerformanceSnapshot{
					Timing:  PerformanceTiming{Load: baseline.Timing.Load},
					Network: NetworkSummary{TransferSize: baseline.Network.TransferSize},
				}
				hasFirst = true
			}
		}

		if hasFirst {
			loadDelta := latestSnapshot.Timing.Load - firstSnapshot.Timing.Load
			sizeDelta := latestSnapshot.Network.TransferSize - firstSnapshot.Network.TransferSize

			perfParts := []string{}
			if loadDelta != 0 {
				perfParts = append(perfParts, fmt.Sprintf("%+.0fms load", loadDelta))
			}
			if sizeDelta != 0 {
				perfParts = append(perfParts, fmt.Sprintf("%s bundle", formatSignedBytes(sizeDelta)))
			}

			if len(perfParts) > 0 {
				parts = append(parts, "perf: "+strings.Join(perfParts, ", "))
			} else {
				parts = append(parts, "perf: no change")
			}
		} else {
			parts = append(parts, "no perf data")
		}
	}

	// Errors section
	if errors != nil {
		fixedCount := 0
		newCount := 0
		for _, e := range errors {
			if e.Resolved {
				fixedCount++
			} else {
				newCount++
			}
		}
		errParts := []string{}
		if fixedCount > 0 {
			errParts = append(errParts, fmt.Sprintf("%d fixed", fixedCount))
		}
		if newCount > 0 {
			errParts = append(errParts, fmt.Sprintf("%d new", newCount))
		}
		if len(errParts) > 0 {
			parts = append(parts, "errors: "+strings.Join(errParts, ", "))
		} else {
			parts = append(parts, "errors: clean")
		}
	}

	return "[" + strings.Join(parts, " | ") + "]"
}

// formatDeltaMs formats a millisecond delta with sign and percentage.
func formatDeltaMs(deltaMs float64, pct float64) string {
	if deltaMs == 0 {
		return "—"
	}
	sign := "+"
	if deltaMs < 0 {
		sign = ""
	}
	return fmt.Sprintf("%s%.0fms (%s%.0f%%)", sign, deltaMs, sign, pct)
}

// formatDeltaBytes formats a byte delta with sign and percentage.
func formatDeltaBytes(deltaBytes int64, pct float64) string {
	if deltaBytes == 0 {
		return "—"
	}
	sign := "+"
	if deltaBytes < 0 {
		sign = ""
	}
	return fmt.Sprintf("%s%s (%s%.0f%%)", sign, formatBytes(abs64(deltaBytes)), sign, pct)
}

// formatSignedBytes formats bytes with a sign prefix.
func formatSignedBytes(b int64) string {
	sign := "+"
	if b < 0 {
		sign = "-"
		b = -b
	}
	return sign + formatBytes(b)
}

// abs64 returns the absolute value of an int64.
func abs64(n int64) int64 {
	if n < 0 {
		return -n
	}
	return n
}

// toolGeneratePRSummary handles the generate_pr_summary MCP tool.
func (h *ToolHandler) toolGeneratePRSummary(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	// Gather console errors for the summary
	h.server.mu.RLock()
	entries := make([]LogEntry, len(h.server.entries))
	copy(entries, h.server.entries)
	h.server.mu.RUnlock()

	var errors []SessionError
	for _, entry := range entries {
		level, _ := entry["level"].(string)
		if level == "error" {
			msg, _ := entry["message"].(string)
			source, _ := entry["source"].(string)
			errors = append(errors, SessionError{
				Message: msg,
				Source:  source,
			})
		}
	}

	markdownText := h.capture.GeneratePRSummary(errors)

	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpMarkdownResponse("PR summary", markdownText)}
}
