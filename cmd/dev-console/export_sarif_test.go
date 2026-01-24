package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ============================================
// Conversion Tests
// ============================================

func TestAxeViolationToSARIFResult(t *testing.T) {
	a11yResult := json.RawMessage(`{
		"violations": [{
			"id": "color-contrast",
			"impact": "serious",
			"description": "Ensures the contrast between foreground and background colors meets WCAG 2 AA minimum contrast ratio thresholds",
			"help": "Elements must meet minimum color contrast ratio thresholds",
			"helpUrl": "https://dequeuniversity.com/rules/axe/4.10/color-contrast",
			"tags": ["cat.color", "wcag2aa", "wcag143"],
			"nodes": [{
				"html": "<span class=\"low-contrast\">Hard to read</span>",
				"target": ["#main > .content > span.low-contrast"],
				"impact": "serious"
			}]
		}],
		"passes": [],
		"incomplete": [],
		"inapplicable": []
	}`)

	opts := SARIFExportOptions{}
	log, err := ExportSARIF(a11yResult, opts)
	if err != nil {
		t.Fatalf("ExportSARIF failed: %v", err)
	}

	if len(log.Runs) != 1 {
		t.Fatalf("Expected 1 run, got %d", len(log.Runs))
	}

	if len(log.Runs[0].Results) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(log.Runs[0].Results))
	}

	result := log.Runs[0].Results[0]
	if result.RuleID != "color-contrast" {
		t.Errorf("Expected ruleId 'color-contrast', got %q", result.RuleID)
	}
	if result.Level != "error" {
		t.Errorf("Expected level 'error' for serious impact, got %q", result.Level)
	}
	if result.Message.Text == "" {
		t.Error("Expected non-empty message text")
	}
}

func TestAxeImpactToSARIFLevel(t *testing.T) {
	tests := []struct {
		impact   string
		expected string
	}{
		{"critical", "error"},
		{"serious", "error"},
		{"moderate", "warning"},
		{"minor", "note"},
		{"", "warning"},       // unknown defaults to warning
		{"unknown", "warning"}, // unknown defaults to warning
	}

	for _, tc := range tests {
		t.Run(tc.impact, func(t *testing.T) {
			got := axeImpactToLevel(tc.impact)
			if got != tc.expected {
				t.Errorf("axeImpactToLevel(%q) = %q, want %q", tc.impact, got, tc.expected)
			}
		})
	}
}

func TestAxeNodeToSARIFLocation(t *testing.T) {
	a11yResult := json.RawMessage(`{
		"violations": [{
			"id": "image-alt",
			"impact": "critical",
			"description": "Images must have alternate text",
			"help": "Images must have alternate text",
			"helpUrl": "https://dequeuniversity.com/rules/axe/4.10/image-alt",
			"tags": ["wcag2a"],
			"nodes": [{
				"html": "<img src=\"photo.jpg\">",
				"target": ["img.hero-image"],
				"impact": "critical"
			}]
		}],
		"passes": [],
		"incomplete": [],
		"inapplicable": []
	}`)

	opts := SARIFExportOptions{}
	log, err := ExportSARIF(a11yResult, opts)
	if err != nil {
		t.Fatalf("ExportSARIF failed: %v", err)
	}

	result := log.Runs[0].Results[0]
	if len(result.Locations) != 1 {
		t.Fatalf("Expected 1 location, got %d", len(result.Locations))
	}

	loc := result.Locations[0]
	if loc.PhysicalLocation.ArtifactLocation.URI != "img.hero-image" {
		t.Errorf("Expected URI 'img.hero-image', got %q", loc.PhysicalLocation.ArtifactLocation.URI)
	}
	if loc.PhysicalLocation.Region.Snippet.Text != "<img src=\"photo.jpg\">" {
		t.Errorf("Expected snippet with img html, got %q", loc.PhysicalLocation.Region.Snippet.Text)
	}
}

func TestAxeViolationToSARIFRule(t *testing.T) {
	a11yResult := json.RawMessage(`{
		"violations": [{
			"id": "color-contrast",
			"impact": "serious",
			"description": "Ensures the contrast between foreground and background colors meets WCAG 2 AA",
			"help": "Elements must meet minimum color contrast ratio thresholds",
			"helpUrl": "https://dequeuniversity.com/rules/axe/4.10/color-contrast",
			"tags": ["cat.color", "wcag2aa", "wcag143"],
			"nodes": [{
				"html": "<span>text</span>",
				"target": ["span.low"],
				"impact": "serious"
			}]
		}],
		"passes": [],
		"incomplete": [],
		"inapplicable": []
	}`)

	opts := SARIFExportOptions{}
	log, err := ExportSARIF(a11yResult, opts)
	if err != nil {
		t.Fatalf("ExportSARIF failed: %v", err)
	}

	rules := log.Runs[0].Tool.Driver.Rules
	if len(rules) != 1 {
		t.Fatalf("Expected 1 rule, got %d", len(rules))
	}

	rule := rules[0]
	if rule.ID != "color-contrast" {
		t.Errorf("Expected rule ID 'color-contrast', got %q", rule.ID)
	}
	if rule.ShortDescription.Text != "Ensures the contrast between foreground and background colors meets WCAG 2 AA" {
		t.Errorf("Unexpected shortDescription: %q", rule.ShortDescription.Text)
	}
	if rule.FullDescription.Text != "Elements must meet minimum color contrast ratio thresholds" {
		t.Errorf("Unexpected fullDescription: %q", rule.FullDescription.Text)
	}
	if rule.HelpURI != "https://dequeuniversity.com/rules/axe/4.10/color-contrast" {
		t.Errorf("Unexpected helpUri: %q", rule.HelpURI)
	}
}

func TestMultipleNodesCreateMultipleResults(t *testing.T) {
	a11yResult := json.RawMessage(`{
		"violations": [{
			"id": "color-contrast",
			"impact": "serious",
			"description": "Color contrast check",
			"help": "Must meet contrast",
			"helpUrl": "https://example.com",
			"tags": ["wcag2aa"],
			"nodes": [
				{"html": "<span>one</span>", "target": ["span.a"], "impact": "serious"},
				{"html": "<span>two</span>", "target": ["span.b"], "impact": "serious"},
				{"html": "<span>three</span>", "target": ["span.c"], "impact": "serious"}
			]
		}],
		"passes": [],
		"incomplete": [],
		"inapplicable": []
	}`)

	opts := SARIFExportOptions{}
	log, err := ExportSARIF(a11yResult, opts)
	if err != nil {
		t.Fatalf("ExportSARIF failed: %v", err)
	}

	if len(log.Runs[0].Results) != 3 {
		t.Errorf("Expected 3 results (one per node), got %d", len(log.Runs[0].Results))
	}

	// All results should reference the same rule
	for _, r := range log.Runs[0].Results {
		if r.RuleID != "color-contrast" {
			t.Errorf("Expected all results to have ruleId 'color-contrast', got %q", r.RuleID)
		}
		if r.RuleIndex != 0 {
			t.Errorf("Expected ruleIndex 0, got %d", r.RuleIndex)
		}
	}
}

func TestWCAGTagExtraction(t *testing.T) {
	tests := []struct {
		name     string
		tags     []string
		expected []string
	}{
		{
			name:     "mixed tags",
			tags:     []string{"cat.color", "wcag2aa", "wcag143", "TTv5", "TT6.a"},
			expected: []string{"wcag2aa", "wcag143"},
		},
		{
			name:     "no wcag tags",
			tags:     []string{"cat.color", "TTv5"},
			expected: []string{},
		},
		{
			name:     "all wcag tags",
			tags:     []string{"wcag2a", "wcag2aa", "wcag21aa"},
			expected: []string{"wcag2a", "wcag2aa", "wcag21aa"},
		},
		{
			name:     "empty tags",
			tags:     []string{},
			expected: []string{},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := extractWCAGTags(tc.tags)
			if len(got) != len(tc.expected) {
				t.Errorf("extractWCAGTags(%v) returned %d tags, want %d", tc.tags, len(got), len(tc.expected))
				return
			}
			for i := range got {
				if got[i] != tc.expected[i] {
					t.Errorf("extractWCAGTags(%v)[%d] = %q, want %q", tc.tags, i, got[i], tc.expected[i])
				}
			}
		})
	}
}

func TestPassesIncludedWhenRequested(t *testing.T) {
	a11yResult := json.RawMessage(`{
		"violations": [],
		"passes": [{
			"id": "button-name",
			"impact": "critical",
			"description": "Buttons must have discernible text",
			"help": "Buttons must have discernible text",
			"helpUrl": "https://dequeuniversity.com/rules/axe/4.10/button-name",
			"tags": ["wcag2a", "wcag412"],
			"nodes": [{
				"html": "<button>Submit</button>",
				"target": ["button.submit"],
				"impact": "critical"
			}]
		}],
		"incomplete": [],
		"inapplicable": []
	}`)

	opts := SARIFExportOptions{IncludePasses: true}
	log, err := ExportSARIF(a11yResult, opts)
	if err != nil {
		t.Fatalf("ExportSARIF failed: %v", err)
	}

	if len(log.Runs[0].Results) != 1 {
		t.Fatalf("Expected 1 result from passes, got %d", len(log.Runs[0].Results))
	}

	result := log.Runs[0].Results[0]
	if result.Level != "none" {
		t.Errorf("Expected level 'none' for passes, got %q", result.Level)
	}
	if result.RuleID != "button-name" {
		t.Errorf("Expected ruleId 'button-name', got %q", result.RuleID)
	}
}

func TestPassesExcludedByDefault(t *testing.T) {
	a11yResult := json.RawMessage(`{
		"violations": [],
		"passes": [{
			"id": "button-name",
			"impact": "critical",
			"description": "Buttons must have discernible text",
			"help": "Buttons must have discernible text",
			"helpUrl": "https://dequeuniversity.com/rules/axe/4.10/button-name",
			"tags": ["wcag2a"],
			"nodes": [{
				"html": "<button>Submit</button>",
				"target": ["button.submit"],
				"impact": "critical"
			}]
		}],
		"incomplete": [],
		"inapplicable": []
	}`)

	opts := SARIFExportOptions{IncludePasses: false}
	log, err := ExportSARIF(a11yResult, opts)
	if err != nil {
		t.Fatalf("ExportSARIF failed: %v", err)
	}

	if len(log.Runs[0].Results) != 0 {
		t.Errorf("Expected 0 results when passes excluded, got %d", len(log.Runs[0].Results))
	}
}

// ============================================
// Full Export Tests
// ============================================

func TestExportSARIF_EmptyViolations(t *testing.T) {
	a11yResult := json.RawMessage(`{
		"violations": [],
		"passes": [],
		"incomplete": [],
		"inapplicable": []
	}`)

	opts := SARIFExportOptions{}
	log, err := ExportSARIF(a11yResult, opts)
	if err != nil {
		t.Fatalf("ExportSARIF failed: %v", err)
	}

	if len(log.Runs) != 1 {
		t.Fatalf("Expected 1 run, got %d", len(log.Runs))
	}
	if len(log.Runs[0].Results) != 0 {
		t.Errorf("Expected 0 results for empty violations, got %d", len(log.Runs[0].Results))
	}

	// Should still be valid SARIF
	data, err := json.Marshal(log)
	if err != nil {
		t.Fatalf("Failed to marshal SARIF: %v", err)
	}
	if !json.Valid(data) {
		t.Error("Expected valid JSON output")
	}
}

func TestExportSARIF_MultipleViolations(t *testing.T) {
	a11yResult := json.RawMessage(`{
		"violations": [
			{
				"id": "color-contrast",
				"impact": "serious",
				"description": "Color contrast",
				"help": "Must have contrast",
				"helpUrl": "https://example.com/color",
				"tags": ["wcag2aa"],
				"nodes": [
					{"html": "<span>a</span>", "target": ["span.a"], "impact": "serious"},
					{"html": "<span>b</span>", "target": ["span.b"], "impact": "serious"}
				]
			},
			{
				"id": "image-alt",
				"impact": "critical",
				"description": "Image alt text",
				"help": "Images must have alt",
				"helpUrl": "https://example.com/img",
				"tags": ["wcag2a"],
				"nodes": [
					{"html": "<img>", "target": ["img.x"], "impact": "critical"}
				]
			}
		],
		"passes": [],
		"incomplete": [],
		"inapplicable": []
	}`)

	opts := SARIFExportOptions{}
	log, err := ExportSARIF(a11yResult, opts)
	if err != nil {
		t.Fatalf("ExportSARIF failed: %v", err)
	}

	// 2 nodes from first violation + 1 from second = 3 results
	if len(log.Runs[0].Results) != 3 {
		t.Errorf("Expected 3 results, got %d", len(log.Runs[0].Results))
	}

	// 2 unique rules
	if len(log.Runs[0].Tool.Driver.Rules) != 2 {
		t.Errorf("Expected 2 rules, got %d", len(log.Runs[0].Tool.Driver.Rules))
	}
}

func TestExportSARIF_Schema(t *testing.T) {
	a11yResult := json.RawMessage(`{
		"violations": [],
		"passes": [],
		"incomplete": [],
		"inapplicable": []
	}`)

	opts := SARIFExportOptions{}
	log, err := ExportSARIF(a11yResult, opts)
	if err != nil {
		t.Fatalf("ExportSARIF failed: %v", err)
	}

	if log.Version != "2.1.0" {
		t.Errorf("Expected version '2.1.0', got %q", log.Version)
	}
	if log.Schema != "https://raw.githubusercontent.com/oasis-tcs/sarif-spec/main/sarif-2.1/schema/sarif-schema-2.1.0.json" {
		t.Errorf("Unexpected $schema: %q", log.Schema)
	}
	if log.Runs[0].Tool.Driver.Name != "Gasoline" {
		t.Errorf("Expected tool name 'Gasoline', got %q", log.Runs[0].Tool.Driver.Name)
	}
	if log.Runs[0].Tool.Driver.Version != version {
		t.Errorf("Expected tool version %q, got %q", version, log.Runs[0].Tool.Driver.Version)
	}
	if log.Runs[0].Tool.Driver.InformationURI != "https://github.com/anthropics/gasoline" {
		t.Errorf("Unexpected informationUri: %q", log.Runs[0].Tool.Driver.InformationURI)
	}
}

func TestExportSARIF_RulesDeduplication(t *testing.T) {
	// Same rule ID appearing in multiple results should only produce 1 rule entry
	a11yResult := json.RawMessage(`{
		"violations": [{
			"id": "color-contrast",
			"impact": "serious",
			"description": "Color contrast",
			"help": "Must have contrast",
			"helpUrl": "https://example.com/color",
			"tags": ["wcag2aa"],
			"nodes": [
				{"html": "<span>a</span>", "target": ["span.a"], "impact": "serious"},
				{"html": "<span>b</span>", "target": ["span.b"], "impact": "serious"},
				{"html": "<span>c</span>", "target": ["span.c"], "impact": "serious"}
			]
		}],
		"passes": [],
		"incomplete": [],
		"inapplicable": []
	}`)

	opts := SARIFExportOptions{}
	log, err := ExportSARIF(a11yResult, opts)
	if err != nil {
		t.Fatalf("ExportSARIF failed: %v", err)
	}

	if len(log.Runs[0].Tool.Driver.Rules) != 1 {
		t.Errorf("Expected 1 deduplicated rule, got %d", len(log.Runs[0].Tool.Driver.Rules))
	}
	if len(log.Runs[0].Results) != 3 {
		t.Errorf("Expected 3 results, got %d", len(log.Runs[0].Results))
	}
}

// ============================================
// File Save Tests
// ============================================

func TestExportSARIF_SaveToFile(t *testing.T) {
	a11yResult := json.RawMessage(`{
		"violations": [{
			"id": "image-alt",
			"impact": "critical",
			"description": "Images must have alt text",
			"help": "Add alt attribute",
			"helpUrl": "https://example.com",
			"tags": ["wcag2a"],
			"nodes": [{"html": "<img>", "target": ["img"], "impact": "critical"}]
		}],
		"passes": [],
		"incomplete": [],
		"inapplicable": []
	}`)

	tmpDir := t.TempDir()
	savePath := filepath.Join(tmpDir, "results.sarif")

	opts := SARIFExportOptions{SaveTo: savePath}
	log, err := ExportSARIF(a11yResult, opts)
	if err != nil {
		t.Fatalf("ExportSARIF failed: %v", err)
	}

	// Verify the file was written
	data, err := os.ReadFile(savePath)
	if err != nil {
		t.Fatalf("Failed to read saved file: %v", err)
	}

	if len(data) == 0 {
		t.Error("Saved file is empty")
	}

	// Verify it's valid JSON
	var parsed SARIFLog
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Saved file is not valid SARIF JSON: %v", err)
	}

	// The returned log should still be valid
	if log.Version != "2.1.0" {
		t.Errorf("Expected version '2.1.0', got %q", log.Version)
	}
}

func TestExportSARIF_InvalidPath(t *testing.T) {
	a11yResult := json.RawMessage(`{
		"violations": [],
		"passes": [],
		"incomplete": [],
		"inapplicable": []
	}`)

	opts := SARIFExportOptions{SaveTo: "/nonexistent/path/results.sarif"}
	_, err := ExportSARIF(a11yResult, opts)
	if err == nil {
		t.Error("Expected error for invalid path, got nil")
	}
}

func TestExportSARIF_PathTraversal(t *testing.T) {
	a11yResult := json.RawMessage(`{
		"violations": [],
		"passes": [],
		"incomplete": [],
		"inapplicable": []
	}`)

	// Path traversal attempts should be rejected
	badPaths := []string{
		"../../etc/passwd",
		"/etc/passwd",
	}

	for _, path := range badPaths {
		opts := SARIFExportOptions{SaveTo: path}
		_, err := ExportSARIF(a11yResult, opts)
		// Either error or the path should be rejected
		// We allow /tmp and cwd, reject everything else
		if err == nil && !strings.HasPrefix(path, "/tmp") {
			// Check if it's under cwd
			cwd, _ := os.Getwd()
			absPath, _ := filepath.Abs(path)
			if !strings.HasPrefix(absPath, cwd) {
				t.Errorf("Expected error for path traversal %q, got nil", path)
			}
		}
	}
}

// ============================================
// MCP Integration Test
// ============================================

func TestExportSARIFTool(t *testing.T) {
	capture := setupTestCapture(t)

	// Pre-populate the a11y cache with a result
	cacheKey := capture.a11yCacheKey("", nil)
	a11yResult := json.RawMessage(`{
		"violations": [{
			"id": "link-name",
			"impact": "serious",
			"description": "Links must have discernible text",
			"help": "Links must have discernible text",
			"helpUrl": "https://dequeuniversity.com/rules/axe/4.10/link-name",
			"tags": ["wcag2a", "wcag412"],
			"nodes": [{
				"html": "<a href=\"/page\"></a>",
				"target": ["a.nav-link"],
				"impact": "serious"
			}]
		}],
		"passes": [],
		"incomplete": [],
		"inapplicable": []
	}`)
	capture.setA11yCacheEntry(cacheKey, a11yResult)

	server, _ := NewServer(filepath.Join(t.TempDir(), "test.jsonl"), 100)
	handler := &ToolHandler{
		MCPHandler: NewMCPHandler(server),
		capture:    capture,
	}

	// Call the export_sarif tool
	req := JSONRPCRequest{JSONRPC: "2.0", ID: float64(1)}
	args := json.RawMessage(`{}`)

	resp := handler.toolExportSARIF(req, args)
	if resp.Error != nil {
		t.Fatalf("Tool returned error: %s", resp.Error.Message)
	}

	// Parse the MCP response to get the SARIF content
	var toolResult MCPToolResult
	if err := json.Unmarshal(resp.Result, &toolResult); err != nil {
		t.Fatalf("Failed to parse tool result: %v", err)
	}

	if toolResult.IsError {
		t.Fatalf("Tool result is an error: %s", toolResult.Content[0].Text)
	}

	if len(toolResult.Content) == 0 {
		t.Fatal("Expected content in tool result")
	}

	// Parse the SARIF JSON from the response
	var sarifLog SARIFLog
	if err := json.Unmarshal([]byte(toolResult.Content[0].Text), &sarifLog); err != nil {
		t.Fatalf("Tool response is not valid SARIF JSON: %v", err)
	}

	if sarifLog.Version != "2.1.0" {
		t.Errorf("Expected SARIF version '2.1.0', got %q", sarifLog.Version)
	}
	if len(sarifLog.Runs[0].Results) != 1 {
		t.Errorf("Expected 1 result, got %d", len(sarifLog.Runs[0].Results))
	}
}

func TestExportSARIFTool_NoCachedResult(t *testing.T) {
	capture := setupTestCapture(t)

	server, _ := NewServer(filepath.Join(t.TempDir(), "test.jsonl"), 100)
	handler := &ToolHandler{
		MCPHandler: NewMCPHandler(server),
		capture:    capture,
	}

	req := JSONRPCRequest{JSONRPC: "2.0", ID: float64(1)}
	args := json.RawMessage(`{}`)

	resp := handler.toolExportSARIF(req, args)
	if resp.Error != nil {
		t.Fatalf("Tool returned JSON-RPC error: %s", resp.Error.Message)
	}

	// Should be an MCP error response (isError: true)
	var toolResult MCPToolResult
	if err := json.Unmarshal(resp.Result, &toolResult); err != nil {
		t.Fatalf("Failed to parse tool result: %v", err)
	}

	if !toolResult.IsError {
		t.Error("Expected isError=true when no cached result available")
	}

	if !strings.Contains(toolResult.Content[0].Text, "No accessibility audit results available") {
		t.Errorf("Expected error message about no results, got %q", toolResult.Content[0].Text)
	}
}
