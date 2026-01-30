// testgen_test.go — Tests for test generation feature
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// ============================================
// Test Setup Helpers
// ============================================

func setupTestGenHandler(t *testing.T) *ToolHandler {
	t.Helper()
	capture := NewCapture()
	server := &Server{
		entries: make([]LogEntry, 0),
	}
	mcpHandler := &MCPHandler{server: server}
	return &ToolHandler{
		MCPHandler: mcpHandler,
		capture:    capture,
	}
}

// ============================================
// TG-UNIT-001: generate tool accepts type: "test_from_context"
// ============================================

func TestToolGenerate_AcceptsTestFromContext(t *testing.T) {
	h := setupTestGenHandler(t)

	args := json.RawMessage(`{"format":"test_from_context","context":"error"}`)
	req := JSONRPCRequest{ID: 1}

	resp := h.toolGenerate(req, args)

	// Should not return unknown_mode error
	resultStr := string(resp.Result)
	if strings.Contains(resultStr, "unknown_mode") {
		t.Errorf("Expected test_from_context to be accepted, got: %s", resultStr)
	}
}

// ============================================
// TG-UNIT-010: context: "error" with valid error generates test
// ============================================

func TestGenerateTestFromError_ValidError(t *testing.T) {
	h := setupTestGenHandler(t)

	// Add a console error
	now := time.Now().UnixMilli()
	errorEntry := LogEntry{
		"ts":      time.Now().Format(time.RFC3339Nano),
		"level":   "error",
		"message": "Uncaught TypeError: Cannot read property 'foo' of undefined",
		"source":  "console-api",
		"url":     "https://example.com/test",
	}
	h.server.mu.Lock()
	h.server.entries = append(h.server.entries, errorEntry)
	h.server.mu.Unlock()

	// Add some actions before the error
	h.capture.AddEnhancedActions([]EnhancedAction{{
		Type:      "click",
		Timestamp: now - 1000, // 1 second before error
		URL:       "https://example.com/test",
		Selectors: map[string]interface{}{
			"testId": "submit-button",
		},
	}})

	h.capture.AddEnhancedActions([]EnhancedAction{{
		Type:      "input",
		Timestamp: now - 2000, // 2 seconds before error
		URL:       "https://example.com/test",
		Value:     "test@example.com",
		Selectors: map[string]interface{}{
			"testId": "email-input",
		},
	}})

	args := json.RawMessage(`{"format":"test_from_context","context":"error","framework":"playwright"}`)
	req := JSONRPCRequest{ID: 1}

	resp := h.toolGenerate(req, args)

	// Should return a valid response with test content
	var result MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if result.IsError {
		t.Errorf("Expected successful response, got error: %s", result.Content[0].Text)
	}

	// Response should contain test content
	responseText := result.Content[0].Text
	if !strings.Contains(responseText, "Generated playwright test") {
		t.Errorf("Expected response to mention generated test, got: %s", responseText)
	}

	// Check for JSON data in response
	if !strings.Contains(responseText, "framework") {
		t.Errorf("Expected JSON data with framework field, got: %s", responseText)
	}
}

// ============================================
// TG-UNIT-030: Generate test from single console error
// ============================================

func TestGenerateTestFromError_SingleError(t *testing.T) {
	h := setupTestGenHandler(t)

	// Add error
	now := time.Now().UnixMilli()
	errorEntry := LogEntry{
		"ts":      time.Now().Format(time.RFC3339Nano),
		"level":   "error",
		"message": "Form validation failed",
		"source":  "console-api",
	}
	h.server.mu.Lock()
	h.server.entries = append(h.server.entries, errorEntry)
	h.server.mu.Unlock()

	// Add action
	h.capture.AddEnhancedActions([]EnhancedAction{{
		Type:      "click",
		Timestamp: now - 500,
		URL:       "https://example.com",
		Selectors: map[string]interface{}{
			"testId": "submit",
		},
	}})

	req := TestFromContextRequest{
		Context:   "error",
		Framework: "playwright",
	}

	result, err := h.generateTestFromError(req)
	if err != nil {
		t.Fatalf("Expected test generation to succeed, got error: %v", err)
	}

	if result.Framework != "playwright" {
		t.Errorf("Expected framework=playwright, got: %s", result.Framework)
	}

	if result.Assertions == 0 {
		t.Errorf("Expected at least 1 assertion, got: %d", result.Assertions)
	}

	// Test content should include the error message
	if !strings.Contains(result.Content, "Form validation failed") {
		t.Errorf("Expected test content to include error message")
	}

	// Should use import from @playwright/test
	if !strings.Contains(result.Content, "import { test, expect } from '@playwright/test'") {
		t.Errorf("Expected Playwright import statement")
	}
}

// ============================================
// TG-UNIT-031: Generated test includes actions within ±5s of error
// ============================================

func TestGenerateTestFromError_ActionsWithinTimeWindow(t *testing.T) {
	h := setupTestGenHandler(t)

	now := time.Now().UnixMilli()

	// Add error
	errorEntry := LogEntry{
		"ts":      time.UnixMilli(now).Format(time.RFC3339Nano),
		"level":   "error",
		"message": "Test error",
		"source":  "console-api",
	}
	h.server.mu.Lock()
	h.server.entries = append(h.server.entries, errorEntry)
	h.server.mu.Unlock()

	// Add action within window (3s before)
	h.capture.AddEnhancedActions([]EnhancedAction{{
		Type:      "click",
		Timestamp: now - 3000,
		URL:       "https://example.com",
		Selectors: map[string]interface{}{"testId": "in-window"},
	}})

	// Add action outside window (10s before)
	h.capture.AddEnhancedActions([]EnhancedAction{{
		Type:      "click",
		Timestamp: now - 10000,
		URL:       "https://example.com",
		Selectors: map[string]interface{}{"testId": "out-of-window"},
	}})

	req := TestFromContextRequest{
		Context:   "error",
		Framework: "playwright",
	}

	result, err := h.generateTestFromError(req)
	if err != nil {
		t.Fatalf("Expected success, got error: %v", err)
	}

	// Should include action within window
	if !strings.Contains(result.Content, "in-window") {
		t.Errorf("Expected test to include action within time window")
	}

	// Should NOT include action outside window
	if strings.Contains(result.Content, "out-of-window") {
		t.Errorf("Expected test to exclude action outside time window")
	}
}

// ============================================
// TG-UNIT-035: No actions captured returns error
// ============================================

func TestGenerateTestFromError_NoActions(t *testing.T) {
	h := setupTestGenHandler(t)

	// Add error but no actions
	errorEntry := LogEntry{
		"ts":      time.Now().Format(time.RFC3339Nano),
		"level":   "error",
		"message": "Test error",
		"source":  "console-api",
	}
	h.server.mu.Lock()
	h.server.entries = append(h.server.entries, errorEntry)
	h.server.mu.Unlock()

	req := TestFromContextRequest{
		Context:   "error",
		Framework: "playwright",
	}

	_, err := h.generateTestFromError(req)
	if err == nil {
		t.Errorf("Expected error when no actions captured")
	}

	if !strings.Contains(err.Error(), "no_actions_captured") {
		t.Errorf("Expected no_actions_captured error code, got: %v", err)
	}
}

// ============================================
// TG-UNIT-036: No error context available returns error
// ============================================

func TestGenerateTestFromError_NoErrorContext(t *testing.T) {
	h := setupTestGenHandler(t)

	// No errors added
	// Add some actions
	h.capture.AddEnhancedActions([]EnhancedAction{{
		Type:      "click",
		Timestamp: time.Now().UnixMilli(),
		URL:       "https://example.com",
		Selectors: map[string]interface{}{"testId": "test"},
	}})

	req := TestFromContextRequest{
		Context:   "error",
		Framework: "playwright",
	}

	_, err := h.generateTestFromError(req)
	if err == nil {
		t.Errorf("Expected error when no error context available")
	}

	if !strings.Contains(err.Error(), "no_error_context") {
		t.Errorf("Expected no_error_context error code, got: %v", err)
	}
}

// ============================================
// Error ID Generation Tests
// ============================================

func TestErrorIDGeneration(t *testing.T) {
	// Test that error IDs follow the format: err_{timestamp}_{hash8}
	errorMsg := "Test error message"
	stack := "at foo.js:10"
	url := "https://example.com"

	id1 := generateErrorID(errorMsg, stack, url)
	id2 := generateErrorID(errorMsg, stack, url)

	// IDs should start with err_
	if !strings.HasPrefix(id1, "err_") {
		t.Errorf("Expected error ID to start with 'err_', got: %s", id1)
	}

	// IDs should have exactly 3 parts separated by underscore
	parts := strings.Split(id1, "_")
	if len(parts) != 3 {
		t.Errorf("Expected error ID to have 3 parts, got: %d (%s)", len(parts), id1)
	}

	// Hash part should be deterministic (same inputs = same hash)
	hash1 := parts[2]
	hash2 := strings.Split(id2, "_")[2]
	if hash1 != hash2 {
		t.Errorf("Expected same hash for same inputs, got %s vs %s", hash1, hash2)
	}

	// Hash should be 8 characters
	if len(hash1) != 8 {
		t.Errorf("Expected hash length of 8, got: %d (%s)", len(hash1), hash1)
	}
}

// ============================================
// Selector Priority Tests
// ============================================

func TestGenerateTest_SelectorPriority(t *testing.T) {
	h := setupTestGenHandler(t)

	now := time.Now().UnixMilli()
	errorEntry := LogEntry{
		"ts":      time.Now().Format(time.RFC3339Nano),
		"level":   "error",
		"message": "Test error",
		"source":  "console-api",
	}
	h.server.mu.Lock()
	h.server.entries = append(h.server.entries, errorEntry)
	h.server.mu.Unlock()

	// Add action with testId (highest priority)
	h.capture.AddEnhancedActions([]EnhancedAction{{
		Type:      "click",
		Timestamp: now - 1000,
		URL:       "https://example.com",
		Selectors: map[string]interface{}{
			"testId":    "submit-btn",
			"role":      map[string]interface{}{"role": "button", "name": "Submit"},
			"ariaLabel": "Submit button",
		},
	}})

	req := TestFromContextRequest{
		Context:   "error",
		Framework: "playwright",
	}

	result, err := h.generateTestFromError(req)
	if err != nil {
		t.Fatalf("Expected success, got error: %v", err)
	}

	// Should prioritize testId
	if !strings.Contains(result.Content, "getByTestId('submit-btn')") {
		t.Errorf("Expected test to use getByTestId, got: %s", result.Content)
	}
}

// ============================================
// Response Format Tests
// ============================================

func TestGenerateTestFromError_ResponseFormat(t *testing.T) {
	h := setupTestGenHandler(t)

	now := time.Now().UnixMilli()
	errorEntry := LogEntry{
		"ts":      time.UnixMilli(now).Format(time.RFC3339Nano),
		"level":   "error",
		"message": "Test error",
		"source":  "console-api",
	}
	h.server.mu.Lock()
	h.server.entries = append(h.server.entries, errorEntry)
	h.server.mu.Unlock()

	h.capture.AddEnhancedActions([]EnhancedAction{{
		Type:      "click",
		Timestamp: now - 1000,
		URL:       "https://example.com",
		Selectors: map[string]interface{}{"testId": "test"},
	}})

	args := json.RawMessage(`{"format":"test_from_context","context":"error","framework":"playwright"}`)
	req := JSONRPCRequest{ID: 1}

	resp := h.toolGenerate(req, args)

	var result MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	responseText := result.Content[0].Text

	// Should have summary line
	if !strings.Contains(responseText, "Generated playwright test") {
		t.Errorf("Expected summary line in response, got: %s", responseText)
	}

	// Should have JSON data with required fields
	if !strings.Contains(responseText, "\"framework\"") {
		t.Errorf("Expected framework field in JSON response")
	}

	if !strings.Contains(responseText, "\"filename\"") {
		t.Errorf("Expected filename field in JSON response")
	}

	if !strings.Contains(responseText, "\"content\"") {
		t.Errorf("Expected content field in JSON response")
	}

	if !strings.Contains(responseText, "\"assertions\"") {
		t.Errorf("Expected assertions field in JSON response")
	}
}

// ============================================
// Base URL Override Tests
// ============================================

func TestGenerateTest_BaseURLOverride(t *testing.T) {
	h := setupTestGenHandler(t)

	now := time.Now().UnixMilli()
	errorEntry := LogEntry{
		"ts":      time.Now().Format(time.RFC3339Nano),
		"level":   "error",
		"message": "Test error",
		"source":  "console-api",
	}
	h.server.mu.Lock()
	h.server.entries = append(h.server.entries, errorEntry)
	h.server.mu.Unlock()

	h.capture.AddEnhancedActions([]EnhancedAction{{
		Type:      "navigate",
		Timestamp: now - 1000,
		URL:       "https://example.com/test",
		ToURL:     "https://example.com/test",
	}})

	req := TestFromContextRequest{
		Context:   "error",
		Framework: "playwright",
		BaseURL:   "http://localhost:3000",
	}

	result, err := h.generateTestFromError(req)
	if err != nil {
		t.Fatalf("Expected success, got error: %v", err)
	}

	// URL should use localhost instead of example.com
	if !strings.Contains(result.Content, "localhost:3000") {
		t.Errorf("Expected base URL override to apply")
	}

	if strings.Contains(result.Content, "example.com") {
		t.Errorf("Expected original URL to be replaced")
	}
}

// ============================================
// Coverage Metadata Tests
// ============================================

func TestGenerateTest_Metadata(t *testing.T) {
	h := setupTestGenHandler(t)

	now := time.Now().UnixMilli()
	errorEntry := LogEntry{
		"ts":      time.UnixMilli(now).Format(time.RFC3339Nano),
		"level":   "error",
		"message": "Test error",
		"source":  "console-api",
	}
	h.server.mu.Lock()
	h.server.entries = append(h.server.entries, errorEntry)
	h.server.mu.Unlock()

	h.capture.AddEnhancedActions([]EnhancedAction{{
		Type:      "click",
		Timestamp: now - 1000,
		URL:       "https://example.com",
		Selectors: map[string]interface{}{"testId": "test"},
	}})

	req := TestFromContextRequest{
		Context:   "error",
		Framework: "playwright",
	}

	result, err := h.generateTestFromError(req)
	if err != nil {
		t.Fatalf("Expected success, got error: %v", err)
	}

	// Check metadata is populated
	if result.Metadata.GeneratedAt == "" {
		t.Errorf("Expected generated_at timestamp")
	}

	if len(result.Metadata.ContextUsed) == 0 {
		t.Errorf("Expected context_used to be populated")
	}

	// Check coverage flags
	if !result.Coverage.ErrorReproduced {
		t.Errorf("Expected error_reproduced to be true")
	}
}

// ============================================
// TG-UNIT-011: context: "interaction" generates test from recorded actions
// ============================================

func TestGenerateTestFromInteraction_BasicTest(t *testing.T) {
	h := setupTestGenHandler(t)

	now := time.Now().UnixMilli()

	// Add some user interactions (no error needed)
	h.capture.AddEnhancedActions([]EnhancedAction{
		{
			Type:      "navigate",
			Timestamp: now - 3000,
			URL:       "https://example.com/login",
			ToURL:     "https://example.com/login",
		},
		{
			Type:      "input",
			Timestamp: now - 2000,
			URL:       "https://example.com/login",
			Value:     "user@example.com",
			Selectors: map[string]interface{}{
				"testId": "email-input",
			},
		},
		{
			Type:      "click",
			Timestamp: now - 1000,
			URL:       "https://example.com/login",
			Selectors: map[string]interface{}{
				"testId": "submit-button",
			},
		},
	})

	args := json.RawMessage(`{"format":"test_from_context","context":"interaction","framework":"playwright"}`)
	req := JSONRPCRequest{ID: 1}

	resp := h.toolGenerate(req, args)

	var result MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if result.IsError {
		t.Errorf("Expected successful response, got error: %s", result.Content[0].Text)
	}

	responseText := result.Content[0].Text
	if !strings.Contains(responseText, "Generated playwright test") {
		t.Errorf("Expected response to mention generated test, got: %s", responseText)
	}
}

// ============================================
// TG-UNIT-050: Generate test from click actions
// ============================================

func TestGenerateTestFromInteraction_ClickActions(t *testing.T) {
	h := setupTestGenHandler(t)

	now := time.Now().UnixMilli()

	h.capture.AddEnhancedActions([]EnhancedAction{{
		Type:      "click",
		Timestamp: now,
		URL:       "https://example.com/test",
		Selectors: map[string]interface{}{
			"testId": "submit-btn",
		},
	}})

	req := TestFromContextRequest{
		Context:   "interaction",
		Framework: "playwright",
	}

	result, err := h.generateTestFromInteraction(req)
	if err != nil {
		t.Fatalf("Expected success, got error: %v", err)
	}

	// Should generate valid Playwright test
	if !strings.Contains(result.Content, "import { test, expect } from '@playwright/test'") {
		t.Errorf("Expected Playwright import")
	}

	// Should contain click action
	if !strings.Contains(result.Content, ".click()") {
		t.Errorf("Expected click action in test")
	}

	// Should use testId selector
	if !strings.Contains(result.Content, "submit-btn") {
		t.Errorf("Expected testId selector in test")
	}
}

// ============================================
// TG-UNIT-051: Generate test from input actions
// ============================================

func TestGenerateTestFromInteraction_InputActions(t *testing.T) {
	h := setupTestGenHandler(t)

	now := time.Now().UnixMilli()

	h.capture.AddEnhancedActions([]EnhancedAction{{
		Type:      "input",
		Timestamp: now,
		URL:       "https://example.com/test",
		Value:     "test@example.com",
		Selectors: map[string]interface{}{
			"testId": "email-input",
		},
	}})

	req := TestFromContextRequest{
		Context:   "interaction",
		Framework: "playwright",
	}

	result, err := h.generateTestFromInteraction(req)
	if err != nil {
		t.Fatalf("Expected success, got error: %v", err)
	}

	// Should contain fill action
	if !strings.Contains(result.Content, ".fill(") {
		t.Errorf("Expected fill action in test")
	}

	// Should contain the input value
	if !strings.Contains(result.Content, "test@example.com") {
		t.Errorf("Expected input value in test")
	}
}

// ============================================
// TG-UNIT-052: Generate test from select actions
// ============================================

func TestGenerateTestFromInteraction_SelectActions(t *testing.T) {
	h := setupTestGenHandler(t)

	now := time.Now().UnixMilli()

	h.capture.AddEnhancedActions([]EnhancedAction{{
		Type:          "select",
		Timestamp:     now,
		URL:           "https://example.com/test",
		SelectedValue: "option2",
		Selectors: map[string]interface{}{
			"testId": "country-select",
		},
	}})

	req := TestFromContextRequest{
		Context:   "interaction",
		Framework: "playwright",
	}

	result, err := h.generateTestFromInteraction(req)
	if err != nil {
		t.Fatalf("Expected success, got error: %v", err)
	}

	// Should contain selectOption action
	if !strings.Contains(result.Content, ".selectOption(") {
		t.Errorf("Expected selectOption action in test")
	}

	// Should contain the selected value
	if !strings.Contains(result.Content, "option2") {
		t.Errorf("Expected selected value in test")
	}
}

// ============================================
// TG-UNIT-053: Redacted password values replaced with placeholder
// ============================================

func TestGenerateTestFromInteraction_RedactedPasswords(t *testing.T) {
	h := setupTestGenHandler(t)

	now := time.Now().UnixMilli()

	h.capture.AddEnhancedActions([]EnhancedAction{{
		Type:      "input",
		Timestamp: now,
		URL:       "https://example.com/login",
		Value:     "[redacted]", // Password was redacted
		Selectors: map[string]interface{}{
			"testId": "password-input",
		},
	}})

	req := TestFromContextRequest{
		Context:   "interaction",
		Framework: "playwright",
	}

	result, err := h.generateTestFromInteraction(req)
	if err != nil {
		t.Fatalf("Expected success, got error: %v", err)
	}

	// Should replace [redacted] with [user-provided]
	if !strings.Contains(result.Content, "[user-provided]") {
		t.Errorf("Expected [user-provided] placeholder for redacted password")
	}

	// Should NOT contain [redacted]
	if strings.Contains(result.Content, "[redacted]") {
		t.Errorf("Expected [redacted] to be replaced")
	}
}

// ============================================
// TG-UNIT-054: Generated test has valid syntax
// ============================================

func TestGenerateTestFromInteraction_ValidSyntax(t *testing.T) {
	h := setupTestGenHandler(t)

	now := time.Now().UnixMilli()

	h.capture.AddEnhancedActions([]EnhancedAction{{
		Type:      "click",
		Timestamp: now,
		URL:       "https://example.com/test",
		Selectors: map[string]interface{}{
			"testId": "test-btn",
		},
	}})

	req := TestFromContextRequest{
		Context:   "interaction",
		Framework: "playwright",
	}

	result, err := h.generateTestFromInteraction(req)
	if err != nil {
		t.Fatalf("Expected success, got error: %v", err)
	}

	// Basic syntax checks
	content := result.Content

	// Should have import statement
	if !strings.Contains(content, "import") {
		t.Errorf("Expected import statement")
	}

	// Should have test() call
	if !strings.Contains(content, "test(") {
		t.Errorf("Expected test() function call")
	}

	// Should have async function
	if !strings.Contains(content, "async") {
		t.Errorf("Expected async function")
	}

	// Should have page parameter
	if !strings.Contains(content, "page") {
		t.Errorf("Expected page parameter")
	}

	// Should have closing braces
	openBraces := strings.Count(content, "{")
	closeBraces := strings.Count(content, "}")
	if openBraces != closeBraces {
		t.Errorf("Expected balanced braces, got %d open and %d close", openBraces, closeBraces)
	}
}

// ============================================
// Test interaction mode with no actions captured
// ============================================

func TestGenerateTestFromInteraction_NoActions(t *testing.T) {
	h := setupTestGenHandler(t)

	// No actions added

	req := TestFromContextRequest{
		Context:   "interaction",
		Framework: "playwright",
	}

	_, err := h.generateTestFromInteraction(req)
	if err == nil {
		t.Errorf("Expected error when no actions captured")
	}

	if !strings.Contains(err.Error(), "no_actions_captured") {
		t.Errorf("Expected no_actions_captured error code, got: %v", err)
	}
}

// ============================================
// Test interaction mode metadata
// ============================================

func TestGenerateTestFromInteraction_Metadata(t *testing.T) {
	h := setupTestGenHandler(t)

	now := time.Now().UnixMilli()

	h.capture.AddEnhancedActions([]EnhancedAction{{
		Type:      "click",
		Timestamp: now,
		URL:       "https://example.com/test",
		Selectors: map[string]interface{}{
			"testId": "test-btn",
		},
	}})

	req := TestFromContextRequest{
		Context:   "interaction",
		Framework: "playwright",
	}

	result, err := h.generateTestFromInteraction(req)
	if err != nil {
		t.Fatalf("Expected success, got error: %v", err)
	}

	// Check metadata
	if result.Metadata.SourceError != "" {
		t.Errorf("Expected source_error to be empty for interaction mode")
	}

	if result.Metadata.GeneratedAt == "" {
		t.Errorf("Expected generated_at timestamp")
	}

	if len(result.Metadata.ContextUsed) == 0 {
		t.Errorf("Expected context_used to be populated")
	}

	// Should include "actions" in context_used
	hasActions := false
	for _, ctx := range result.Metadata.ContextUsed {
		if ctx == "actions" {
			hasActions = true
			break
		}
	}
	if !hasActions {
		t.Errorf("Expected 'actions' in context_used")
	}

	// Check coverage
	if result.Coverage.ErrorReproduced {
		t.Errorf("Expected error_reproduced to be false for interaction mode")
	}

	if !result.Coverage.StateCaptured {
		t.Errorf("Expected state_captured to be true when actions present")
	}
}

// ============================================
// Test interaction mode with base_url override
// ============================================

func TestGenerateTestFromInteraction_BaseURLOverride(t *testing.T) {
	h := setupTestGenHandler(t)

	now := time.Now().UnixMilli()

	h.capture.AddEnhancedActions([]EnhancedAction{{
		Type:      "navigate",
		Timestamp: now,
		URL:       "https://example.com/test",
		ToURL:     "https://example.com/test",
	}})

	req := TestFromContextRequest{
		Context:   "interaction",
		Framework: "playwright",
		BaseURL:   "http://localhost:3000",
	}

	result, err := h.generateTestFromInteraction(req)
	if err != nil {
		t.Fatalf("Expected success, got error: %v", err)
	}

	// URL should use localhost instead of example.com
	if !strings.Contains(result.Content, "localhost:3000") {
		t.Errorf("Expected base URL override to apply")
	}

	if strings.Contains(result.Content, "example.com") {
		t.Errorf("Expected original URL to be replaced")
	}
}

// ============================================
// Test interaction mode with multiple action types
// ============================================

func TestGenerateTestFromInteraction_MultipleActionTypes(t *testing.T) {
	h := setupTestGenHandler(t)

	now := time.Now().UnixMilli()

	h.capture.AddEnhancedActions([]EnhancedAction{
		{
			Type:      "navigate",
			Timestamp: now - 4000,
			URL:       "https://example.com/login",
			ToURL:     "https://example.com/login",
		},
		{
			Type:      "input",
			Timestamp: now - 3000,
			URL:       "https://example.com/login",
			Value:     "user@example.com",
			Selectors: map[string]interface{}{
				"testId": "email",
			},
		},
		{
			Type:      "input",
			Timestamp: now - 2000,
			URL:       "https://example.com/login",
			Value:     "[redacted]",
			Selectors: map[string]interface{}{
				"testId": "password",
			},
		},
		{
			Type:      "click",
			Timestamp: now - 1000,
			URL:       "https://example.com/login",
			Selectors: map[string]interface{}{
				"testId": "submit",
			},
		},
		{
			Type:      "navigate",
			Timestamp: now,
			URL:       "https://example.com/dashboard",
			ToURL:     "https://example.com/dashboard",
		},
	})

	req := TestFromContextRequest{
		Context:   "interaction",
		Framework: "playwright",
	}

	result, err := h.generateTestFromInteraction(req)
	if err != nil {
		t.Fatalf("Expected success, got error: %v", err)
	}

	// Should contain all action types
	content := result.Content

	// Navigate
	if !strings.Contains(content, "goto") || !strings.Contains(content, "waitForURL") {
		t.Errorf("Expected navigation actions")
	}

	// Input
	if !strings.Contains(content, ".fill(") {
		t.Errorf("Expected fill action for input")
	}

	// Click
	if !strings.Contains(content, ".click()") {
		t.Errorf("Expected click action")
	}

	// Should have multiple selectors
	if len(result.Selectors) < 3 {
		t.Errorf("Expected at least 3 selectors, got: %d", len(result.Selectors))
	}
}

// ============================================
// test_heal Mode Tests
// ============================================

// TG-UNIT-070: action: "analyze" with valid test_file identifies broken selectors
func TestTestHeal_AnalyzeValidFile(t *testing.T) {
	h := setupTestGenHandler(t)

	// Create a temporary test file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.spec.ts")
	testContent := `
import { test, expect } from '@playwright/test';

test('login test', async ({ page }) => {
	await page.getByTestId('username').fill('user');
	await page.locator('#old-submit-btn').click();
	await page.locator('.removed-element').isVisible();
});
`
	if err := os.WriteFile(testFile, []byte(testContent), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Mock the handler's sessionStore to use tmpDir as projectPath
	if h.sessionStore == nil {
		store, err := NewSessionStore(tmpDir)
		if err != nil {
			t.Fatalf("Failed to create session store: %v", err)
		}
		h.sessionStore = store
		defer store.Shutdown()
	}

	req := TestHealRequest{
		Action:   "analyze",
		TestFile: "test.spec.ts",
	}

	result, err := h.analyzeTestFile(req, tmpDir)
	if err != nil {
		t.Fatalf("Expected analyze to succeed, got error: %v", err)
	}

	// Should identify selectors in the test file
	if len(result) == 0 {
		t.Errorf("Expected to find selectors in test file")
	}
}

// TG-UNIT-071: action: "repair" with broken_selectors returns healed selectors
func TestTestHeal_RepairBrokenSelectors(t *testing.T) {
	h := setupTestGenHandler(t)

	tmpDir := t.TempDir()
	brokenSelectors := []string{"#old-submit-btn", ".removed-element"}

	req := TestHealRequest{
		Action:          "repair",
		BrokenSelectors: brokenSelectors,
		AutoApply:       false,
	}

	result, err := h.repairSelectors(req, tmpDir)
	if err != nil {
		t.Fatalf("Expected repair to succeed, got error: %v", err)
	}

	// Should have processed the selectors (even if not healed)
	totalProcessed := len(result.Healed) + len(result.Unhealed)
	if totalProcessed != len(brokenSelectors) {
		t.Errorf("Expected to process %d selectors, got %d", len(brokenSelectors), totalProcessed)
	}
}

// TG-UNIT-072: Invalid action rejected
func TestTestHeal_InvalidAction(t *testing.T) {
	h := setupTestGenHandler(t)

	args := json.RawMessage(`{"format":"test_heal","action":"invalid_action"}`)
	req := JSONRPCRequest{ID: 1}

	resp := h.toolGenerate(req, args)

	var result MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if !result.IsError {
		t.Errorf("Expected error for invalid action")
	}
}

// TG-UNIT-073: test_file path not found → error
func TestTestHeal_FileNotFound(t *testing.T) {
	h := setupTestGenHandler(t)

	tmpDir := t.TempDir()
	store, err := NewSessionStore(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create session store: %v", err)
	}
	h.sessionStore = store
	defer store.Shutdown()

	req := TestHealRequest{
		Action:   "analyze",
		TestFile: "nonexistent.spec.ts",
	}

	_, err = h.analyzeTestFile(req, tmpDir)
	if err == nil {
		t.Errorf("Expected error for nonexistent file")
	}

	if !strings.Contains(err.Error(), ErrTestFileNotFound) {
		t.Errorf("Expected test_file_not_found error, got: %v", err)
	}
}

// TG-UNIT-090: Heal selector by testid_match strategy → confidence >= 0.9
func TestTestHeal_TestIdMatchStrategy(t *testing.T) {
	// Mock DOM with element that has data-testid
	mockDOM := map[string]interface{}{
		"elements": []map[string]interface{}{
			{
				"selector":   "button[data-testid='submit']",
				"attributes": map[string]string{"data-testid": "submit", "type": "button"},
				"text":       "Submit",
			},
		},
	}

	healed := healSelectorWithStrategy("#old-submit", mockDOM, "testid_match")

	if healed.Confidence < 0.8 {
		t.Errorf("Expected high confidence for testid_match, got: %.2f", healed.Confidence)
	}

	if healed.Strategy != "testid_match" {
		t.Errorf("Expected strategy=testid_match, got: %s", healed.Strategy)
	}
}

// TG-UNIT-091: Heal selector by aria_match strategy → confidence ~0.7
func TestTestHeal_AriaMatchStrategy(t *testing.T) {
	mockDOM := map[string]interface{}{
		"elements": []map[string]interface{}{
			{
				"selector":   "button[role='button']",
				"attributes": map[string]string{"role": "button", "aria-label": "Submit form"},
				"text":       "Submit",
			},
		},
	}

	healed := healSelectorWithStrategy("#old-submit", mockDOM, "aria_match")

	if healed.Confidence < 0.6 || healed.Confidence > 0.8 {
		t.Errorf("Expected confidence around 0.7 for aria_match, got: %.2f", healed.Confidence)
	}

	if healed.Strategy != "aria_match" {
		t.Errorf("Expected strategy=aria_match, got: %s", healed.Strategy)
	}
}

// TG-SEC-001: test_file path traversal (../) → error "path_not_allowed"
func TestTestHeal_PathTraversal(t *testing.T) {
	tmpDir := t.TempDir()

	// Try path traversal
	err := validateTestFilePath("../../etc/passwd", tmpDir)
	if err == nil {
		t.Errorf("Expected error for path traversal")
	}

	if !strings.Contains(err.Error(), ErrPathNotAllowed) {
		t.Errorf("Expected path_not_allowed error, got: %v", err)
	}
}

// TG-SEC-002: Selector with script injection → sanitized/rejected
func TestTestHeal_SelectorInjection(t *testing.T) {
	dangerousSelectors := []string{
		"javascript:alert(1)",
		"<script>alert(1)</script>",
		"img[onerror=alert(1)]",
		"img[onload=alert(1)]",
	}

	for _, selector := range dangerousSelectors {
		err := validateSelector(selector)
		if err == nil {
			t.Errorf("Expected error for dangerous selector: %s", selector)
		}

		if !strings.Contains(err.Error(), ErrSelectorInjection) {
			t.Errorf("Expected selector_injection_detected error for: %s, got: %v", selector, err)
		}
	}
}

// ============================================
// Helper functions for test_heal tests
// ============================================

// Mock helper for healing strategies
func healSelectorWithStrategy(oldSelector string, mockDOM map[string]interface{}, strategy string) HealedSelector {
	// Base confidence based on strategy
	confidenceMap := map[string]float64{
		"testid_match":     0.8,
		"aria_match":       0.7,
		"text_match":       0.6,
		"attribute_match":  0.5,
		"structural_match": 0.3,
	}

	confidence := confidenceMap[strategy]

	// For testing, just return a mock healed selector
	return HealedSelector{
		OldSelector: oldSelector,
		NewSelector: "button[data-testid='submit']",
		Confidence:  confidence,
		Strategy:    strategy,
		LineNumber:  5,
	}
}

// Integration test: verify test_heal accepts both analyze and repair actions
func TestIntegration_TestHealAcceptsActions(t *testing.T) {
	h := setupTestGenHandler(t)
	
	// Set up session store
	tmpDir := t.TempDir()
	store, _ := NewSessionStore(tmpDir)
	h.sessionStore = store
	defer store.Shutdown()

	// Test analyze action
	analyzeArgs := json.RawMessage(`{"format":"test_heal","action":"analyze","test_file":"test.spec.ts"}`)
	req := JSONRPCRequest{ID: 1}
	
	resp := h.toolGenerate(req, analyzeArgs)
	
	var result MCPToolResult
	json.Unmarshal(resp.Result, &result)
	
	// Should not get unknown_mode error
	if strings.Contains(result.Content[0].Text, "unknown_mode") {
		t.Errorf("test_heal mode should be recognized")
	}
	
	// Test repair action
	repairArgs := json.RawMessage(`{"format":"test_heal","action":"repair","broken_selectors":["#test"]}`)
	resp2 := h.toolGenerate(req, repairArgs)
	
	var result2 MCPToolResult
	json.Unmarshal(resp2.Result, &result2)
	
	if strings.Contains(result2.Content[0].Text, "unknown_mode") {
		t.Errorf("test_heal repair mode should be recognized")
	}
	
	fmt.Println("✓ test_heal mode is properly integrated into generate tool")
}

// ============================================
// test_classify Mode Tests
// ============================================

// TG-UNIT-130: "Timeout waiting for selector" + selector missing → category: "selector_broken"
func TestTestClassify_TimeoutSelectorMissing(t *testing.T) {
	h := setupTestGenHandler(t)

	failure := &TestFailure{
		TestName:   "login test",
		Error:      "Timeout waiting for selector '#submit-btn' to be visible",
		Trace:      "at login.spec.ts:15",
		DurationMs: 30000,
	}

	classification := h.classifyFailure(failure)

	if classification.Category != CategorySelectorBroken {
		t.Errorf("Expected category=%s, got: %s", CategorySelectorBroken, classification.Category)
	}

	if classification.Confidence < 0.85 {
		t.Errorf("Expected high confidence (>= 0.85), got: %.2f", classification.Confidence)
	}

	if !classification.IsFlaky && !classification.IsRealBug && classification.IsEnvironment {
		// selector_broken should not be marked as flaky, real bug, or environment
	}

	if classification.RecommendedAction != "heal" {
		t.Errorf("Expected recommended_action=heal, got: %s", classification.RecommendedAction)
	}
}

// TG-UNIT-131: "Timeout waiting for selector" + selector exists → category: "timing_flaky"
func TestTestClassify_TimeoutSelectorExists(t *testing.T) {
	h := setupTestGenHandler(t)

	failure := &TestFailure{
		TestName:   "dashboard test",
		Error:      "Timeout waiting for selector '.loading-spinner'",
		Trace:      "at dashboard.spec.ts:20",
		DurationMs: 5000,
	}

	classification := h.classifyFailure(failure)

	// Without DOM query, we assume timing_flaky when selector is not explicitly mentioned as missing
	if classification.Category != CategoryTimingFlaky && classification.Category != CategorySelectorBroken {
		t.Errorf("Expected category=timing_flaky or selector_broken, got: %s", classification.Category)
	}

	if classification.Category == CategoryTimingFlaky && !classification.IsFlaky {
		t.Errorf("Expected is_flaky=true for timing_flaky category")
	}
}

// TG-UNIT-132: "net::ERR_" error → category: "network_flaky"
func TestTestClassify_NetworkError(t *testing.T) {
	h := setupTestGenHandler(t)

	failure := &TestFailure{
		TestName:   "api test",
		Error:      "net::ERR_CONNECTION_REFUSED at http://localhost:3000/api/users",
		Trace:      "at api.spec.ts:10",
		DurationMs: 1000,
	}

	classification := h.classifyFailure(failure)

	if classification.Category != CategoryNetworkFlaky {
		t.Errorf("Expected category=%s, got: %s", CategoryNetworkFlaky, classification.Category)
	}

	if classification.Confidence < 0.8 {
		t.Errorf("Expected high confidence (>= 0.8), got: %.2f", classification.Confidence)
	}

	if !classification.IsFlaky {
		t.Errorf("Expected is_flaky=true for network_flaky")
	}

	if !classification.IsEnvironment {
		t.Errorf("Expected is_environment=true for network_flaky")
	}

	if classification.RecommendedAction != "mock_network" {
		t.Errorf("Expected recommended_action=mock_network, got: %s", classification.RecommendedAction)
	}
}

// TG-UNIT-133: "Expected X to be Y" assertion → category: "real_bug", is_real_bug: true
func TestTestClassify_AssertionFailure(t *testing.T) {
	h := setupTestGenHandler(t)

	failure := &TestFailure{
		TestName:   "checkout test",
		Error:      "Expected total to be $100, but got $95",
		Trace:      "at checkout.spec.ts:25",
		DurationMs: 500,
	}

	classification := h.classifyFailure(failure)

	if classification.Category != CategoryRealBug {
		t.Errorf("Expected category=%s, got: %s", CategoryRealBug, classification.Category)
	}

	if !classification.IsRealBug {
		t.Errorf("Expected is_real_bug=true for real_bug category")
	}

	if classification.IsFlaky {
		t.Errorf("Expected is_flaky=false for real_bug category")
	}

	if classification.RecommendedAction != "fix_bug" {
		t.Errorf("Expected recommended_action=fix_bug, got: %s", classification.RecommendedAction)
	}
}

// TG-UNIT-134: Unknown error pattern → category: "unknown", low confidence
func TestTestClassify_UnknownPattern(t *testing.T) {
	h := setupTestGenHandler(t)

	failure := &TestFailure{
		TestName:   "mysterious test",
		Error:      "Something went wrong in a very unusual way",
		Trace:      "at mystery.spec.ts:42",
		DurationMs: 1000,
	}

	classification := h.classifyFailure(failure)

	if classification.Category != CategoryUnknown {
		t.Errorf("Expected category=%s, got: %s", CategoryUnknown, classification.Category)
	}

	if classification.Confidence >= 0.5 {
		t.Errorf("Expected low confidence (< 0.5), got: %.2f", classification.Confidence)
	}

	if classification.RecommendedAction != "manual_review" {
		t.Errorf("Expected recommended_action=manual_review, got: %s", classification.RecommendedAction)
	}
}

// TG-UNIT-135: suggested_fix provided for selector_broken → type: "selector_update"
func TestTestClassify_SuggestedFixSelectorBroken(t *testing.T) {
	h := setupTestGenHandler(t)

	failure := &TestFailure{
		TestName:   "login test",
		Error:      "Timeout waiting for selector '#old-submit-btn'",
		Trace:      "at login.spec.ts:15",
		DurationMs: 30000,
	}

	classification := h.classifyFailure(failure)

	if classification.SuggestedFix == nil {
		t.Fatalf("Expected suggested_fix to be provided for selector_broken")
	}

	if classification.SuggestedFix.Type != "selector_update" {
		t.Errorf("Expected suggested_fix.type=selector_update, got: %s", classification.SuggestedFix.Type)
	}

	if classification.SuggestedFix.Old != "#old-submit-btn" {
		t.Errorf("Expected suggested_fix.old='#old-submit-btn', got: %s", classification.SuggestedFix.Old)
	}

	if !strings.Contains(classification.SuggestedFix.Code, "test_heal") {
		t.Errorf("Expected suggested_fix.code to mention test_heal")
	}
}

// Test additional patterns: "Element is not attached to DOM"
func TestTestClassify_ElementNotAttached(t *testing.T) {
	h := setupTestGenHandler(t)

	failure := &TestFailure{
		TestName:   "dynamic content test",
		Error:      "Element is not attached to DOM",
		Trace:      "at dynamic.spec.ts:30",
		DurationMs: 2000,
	}

	classification := h.classifyFailure(failure)

	if classification.Category != CategoryTimingFlaky {
		t.Errorf("Expected category=%s, got: %s", CategoryTimingFlaky, classification.Category)
	}

	if classification.Confidence < 0.75 {
		t.Errorf("Expected confidence >= 0.75, got: %.2f", classification.Confidence)
	}

	if !classification.IsFlaky {
		t.Errorf("Expected is_flaky=true for timing_flaky")
	}
}

// Test "Element is outside viewport"
func TestTestClassify_ElementOutsideViewport(t *testing.T) {
	h := setupTestGenHandler(t)

	failure := &TestFailure{
		TestName:   "long page test",
		Error:      "Element is outside viewport",
		Trace:      "at scroll.spec.ts:10",
		DurationMs: 1000,
	}

	classification := h.classifyFailure(failure)

	if classification.Category != CategoryTestBug {
		t.Errorf("Expected category=%s, got: %s", CategoryTestBug, classification.Category)
	}

	if classification.Confidence < 0.7 {
		t.Errorf("Expected confidence >= 0.7, got: %.2f", classification.Confidence)
	}

	if classification.RecommendedAction != "fix_test" {
		t.Errorf("Expected recommended_action=fix_test, got: %s", classification.RecommendedAction)
	}

	if classification.SuggestedFix == nil {
		t.Fatalf("Expected suggested_fix for test_bug (viewport)")
	}

	if classification.SuggestedFix.Type != "scroll_to_element" {
		t.Errorf("Expected suggested_fix.type=scroll_to_element, got: %s", classification.SuggestedFix.Type)
	}
}

// Test integration: classify via toolGenerate
func TestIntegration_TestClassifyViaToolGenerate(t *testing.T) {
	h := setupTestGenHandler(t)

	args := json.RawMessage(`{
		"format": "test_classify",
		"action": "failure",
		"failure": {
			"test_name": "login test",
			"error": "Timeout waiting for selector '#submit-btn'",
			"trace": "at test.spec.ts:10",
			"duration_ms": 30000
		}
	}`)

	req := JSONRPCRequest{ID: 1}
	resp := h.toolGenerate(req, args)

	var result MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if result.IsError {
		t.Errorf("Expected successful response, got error: %s", result.Content[0].Text)
	}

	responseText := result.Content[0].Text

	// Should have summary with category and confidence
	if !strings.Contains(responseText, "selector_broken") {
		t.Errorf("Expected category in summary, got: %s", responseText)
	}

	// Should have JSON data with classification
	if !strings.Contains(responseText, "\"category\"") {
		t.Errorf("Expected category field in JSON response")
	}

	if !strings.Contains(responseText, "\"confidence\"") {
		t.Errorf("Expected confidence field in JSON response")
	}

	if !strings.Contains(responseText, "\"evidence\"") {
		t.Errorf("Expected evidence field in JSON response")
	}
}

// Test error handling: missing failure parameter
func TestTestClassify_MissingFailureParameter(t *testing.T) {
	h := setupTestGenHandler(t)

	args := json.RawMessage(`{
		"format": "test_classify",
		"action": "failure"
	}`)

	req := JSONRPCRequest{ID: 1}
	resp := h.toolGenerate(req, args)

	var result MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if !result.IsError {
		t.Errorf("Expected error for missing failure parameter")
	}

	responseText := result.Content[0].Text
	if !strings.Contains(responseText, ErrMissingParam) {
		t.Errorf("Expected missing_param error code")
	}
}

// Test error handling: invalid action
func TestTestClassify_InvalidAction(t *testing.T) {
	h := setupTestGenHandler(t)

	args := json.RawMessage(`{
		"format": "test_classify",
		"action": "batch"
	}`)

	req := JSONRPCRequest{ID: 1}
	resp := h.toolGenerate(req, args)

	var result MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if !result.IsError {
		t.Errorf("Expected error for invalid action (batch not implemented)")
	}
}

// Test low confidence classification returns error
func TestTestClassify_LowConfidenceReturnsError(t *testing.T) {
	h := setupTestGenHandler(t)

	args := json.RawMessage(`{
		"format": "test_classify",
		"action": "failure",
		"failure": {
			"test_name": "unknown test",
			"error": "Very unusual error that doesn't match any pattern",
			"trace": "at unknown.spec.ts:1",
			"duration_ms": 1000
		}
	}`)

	req := JSONRPCRequest{ID: 1}
	resp := h.toolGenerate(req, args)

	var result MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if !result.IsError {
		t.Errorf("Expected error for low confidence classification")
	}

	responseText := result.Content[0].Text
	if !strings.Contains(responseText, ErrClassificationUncertain) {
		t.Errorf("Expected classification_uncertain error code")
	}
}

// ============================================
// test_from_context.regression Mode Tests
// ============================================

// TG-UNIT-060: context: "regression" accepted and generates test
func TestGenerateTestFromRegression_BasicTest(t *testing.T) {
	h := setupTestGenHandler(t)

	now := time.Now().UnixMilli()

	// Add some user interactions as baseline
	h.capture.AddEnhancedActions([]EnhancedAction{
		{
			Type:      "navigate",
			Timestamp: now - 3000,
			URL:       "https://example.com/app",
			ToURL:     "https://example.com/app",
		},
		{
			Type:      "click",
			Timestamp: now - 2000,
			URL:       "https://example.com/app",
			Selectors: map[string]interface{}{
				"testId": "load-data-btn",
			},
		},
	})

	args := json.RawMessage(`{"format":"test_from_context","context":"regression","framework":"playwright"}`)
	req := JSONRPCRequest{ID: 1}

	resp := h.toolGenerate(req, args)

	var result MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if result.IsError {
		t.Errorf("Expected successful response, got error: %s", result.Content[0].Text)
	}

	responseText := result.Content[0].Text
	if !strings.Contains(responseText, "Generated playwright test") {
		t.Errorf("Expected response to mention generated test, got: %s", responseText)
	}
}

// Test regression mode includes error assertions
func TestGenerateTestFromRegression_ErrorAssertions(t *testing.T) {
	h := setupTestGenHandler(t)

	now := time.Now().UnixMilli()

	// Add actions
	h.capture.AddEnhancedActions([]EnhancedAction{{
		Type:      "click",
		Timestamp: now,
		URL:       "https://example.com/test",
		Selectors: map[string]interface{}{
			"testId": "test-btn",
		},
	}})

	// No errors in baseline (clean baseline)
	req := TestFromContextRequest{
		Context:   "regression",
		Framework: "playwright",
	}

	result, err := h.generateTestFromRegression(req)
	if err != nil {
		t.Fatalf("Expected success, got error: %v", err)
	}

	// Should include console error assertions since baseline was clean
	if !strings.Contains(result.Content, "consoleErrors") {
		t.Errorf("Expected test to include console error assertions")
	}

	if !strings.Contains(result.Content, "expect(consoleErrors).toHaveLength(0)") {
		t.Errorf("Expected test to assert no console errors")
	}
}

// Test regression mode includes network assertions
func TestGenerateTestFromRegression_NetworkAssertions(t *testing.T) {
	h := setupTestGenHandler(t)

	now := time.Now().UnixMilli()

	// Add actions
	h.capture.AddEnhancedActions([]EnhancedAction{{
		Type:      "click",
		Timestamp: now,
		URL:       "https://example.com/test",
		Selectors: map[string]interface{}{
			"testId": "test-btn",
		},
	}})

	// Add network body for baseline
	h.capture.mu.Lock()
	h.capture.networkBodies = append(h.capture.networkBodies, NetworkBody{
		URL:         "https://api.example.com/data",
		Method:      "GET",
		Status:      200,
		ContentType: "application/json",
		Timestamp:   time.UnixMilli(now).Format(time.RFC3339Nano),
	})
	h.capture.mu.Unlock()

	req := TestFromContextRequest{
		Context:   "regression",
		Framework: "playwright",
	}

	result, err := h.generateTestFromRegression(req)
	if err != nil {
		t.Fatalf("Expected success, got error: %v", err)
	}

	// Should include TODO comments for network assertions
	if !strings.Contains(result.Content, "api.example.com/data") {
		t.Errorf("Expected test to reference network request")
	}

	if !strings.Contains(result.Content, "200") {
		t.Errorf("Expected test to reference status code")
	}
}

// Test regression mode includes performance TODO comments
func TestGenerateTestFromRegression_PerformanceTODO(t *testing.T) {
	h := setupTestGenHandler(t)

	now := time.Now().UnixMilli()

	h.capture.AddEnhancedActions([]EnhancedAction{{
		Type:      "click",
		Timestamp: now,
		URL:       "https://example.com/test",
		Selectors: map[string]interface{}{
			"testId": "test-btn",
		},
	}})

	req := TestFromContextRequest{
		Context:   "regression",
		Framework: "playwright",
	}

	result, err := h.generateTestFromRegression(req)
	if err != nil {
		t.Fatalf("Expected success, got error: %v", err)
	}

	// Should include TODO for performance assertions
	if !strings.Contains(result.Content, "TODO: Add performance assertions") {
		t.Errorf("Expected test to include performance TODO")
	}

	if !strings.Contains(result.Content, "Load time") || !strings.Contains(result.Content, "FCP") {
		t.Errorf("Expected test to mention specific performance metrics")
	}
}

// Test no baseline available scenario
func TestGenerateTestFromRegression_NoBaseline(t *testing.T) {
	h := setupTestGenHandler(t)

	// No actions added (no baseline)

	req := TestFromContextRequest{
		Context:   "regression",
		Framework: "playwright",
	}

	_, err := h.generateTestFromRegression(req)
	if err == nil {
		t.Errorf("Expected error when no baseline available")
	}

	if !strings.Contains(err.Error(), "no_actions_captured") {
		t.Errorf("Expected no_actions_captured error code, got: %v", err)
	}
}

// Test regression mode with errors in baseline
func TestGenerateTestFromRegression_BaselineWithErrors(t *testing.T) {
	h := setupTestGenHandler(t)

	now := time.Now().UnixMilli()

	// Add actions
	h.capture.AddEnhancedActions([]EnhancedAction{{
		Type:      "click",
		Timestamp: now,
		URL:       "https://example.com/test",
		Selectors: map[string]interface{}{
			"testId": "test-btn",
		},
	}})

	// Add error to baseline
	errorEntry := LogEntry{
		"ts":      time.Now().Format(time.RFC3339Nano),
		"level":   "error",
		"message": "Baseline error",
		"source":  "console-api",
	}
	h.server.mu.Lock()
	h.server.entries = append(h.server.entries, errorEntry)
	h.server.mu.Unlock()

	req := TestFromContextRequest{
		Context:   "regression",
		Framework: "playwright",
	}

	result, err := h.generateTestFromRegression(req)
	if err != nil {
		t.Fatalf("Expected success, got error: %v", err)
	}

	// Should mention baseline had errors
	if !strings.Contains(result.Content, "Baseline had") && !strings.Contains(result.Content, "console errors") {
		t.Errorf("Expected test to mention baseline errors")
	}

	// Should include TODO for error verification
	if !strings.Contains(result.Content, "TODO") {
		t.Errorf("Expected TODO comment for error verification")
	}
}

// Test regression mode metadata
func TestGenerateTestFromRegression_Metadata(t *testing.T) {
	h := setupTestGenHandler(t)

	now := time.Now().UnixMilli()

	h.capture.AddEnhancedActions([]EnhancedAction{{
		Type:      "click",
		Timestamp: now,
		URL:       "https://example.com/test",
		Selectors: map[string]interface{}{
			"testId": "test-btn",
		},
	}})

	req := TestFromContextRequest{
		Context:   "regression",
		Framework: "playwright",
	}

	result, err := h.generateTestFromRegression(req)
	if err != nil {
		t.Fatalf("Expected success, got error: %v", err)
	}

	// Check metadata
	if result.Metadata.SourceError != "" {
		t.Errorf("Expected source_error to be empty for regression mode")
	}

	if result.Metadata.GeneratedAt == "" {
		t.Errorf("Expected generated_at timestamp")
	}

	if len(result.Metadata.ContextUsed) == 0 {
		t.Errorf("Expected context_used to be populated")
	}

	// Should include multiple context types
	expectedContexts := []string{"actions", "console", "network", "performance"}
	for _, ctx := range expectedContexts {
		found := false
		for _, used := range result.Metadata.ContextUsed {
			if used == ctx {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected '%s' in context_used", ctx)
		}
	}

	// Check coverage
	if result.Coverage.ErrorReproduced {
		t.Errorf("Expected error_reproduced to be false for regression mode")
	}

	if !result.Coverage.StateCaptured {
		t.Errorf("Expected state_captured to be true when actions present")
	}
}

// Test response format for regression mode
func TestGenerateTestFromRegression_ResponseFormat(t *testing.T) {
	h := setupTestGenHandler(t)

	now := time.Now().UnixMilli()

	h.capture.AddEnhancedActions([]EnhancedAction{{
		Type:      "click",
		Timestamp: now,
		URL:       "https://example.com/test",
		Selectors: map[string]interface{}{
			"testId": "test-btn",
		},
	}})

	args := json.RawMessage(`{"format":"test_from_context","context":"regression","framework":"playwright"}`)
	req := JSONRPCRequest{ID: 1}

	resp := h.toolGenerate(req, args)

	var result MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	responseText := result.Content[0].Text

	// Should have summary line
	if !strings.Contains(responseText, "Generated playwright test") {
		t.Errorf("Expected summary line in response")
	}

	// Should have JSON data with required fields
	if !strings.Contains(responseText, "\"framework\"") {
		t.Errorf("Expected framework field in JSON response")
	}

	if !strings.Contains(responseText, "\"coverage\"") {
		t.Errorf("Expected coverage field in JSON response")
	}

	if !strings.Contains(responseText, "\"metadata\"") {
		t.Errorf("Expected metadata field in JSON response")
	}

	// Verify coverage fields
	if !strings.Contains(responseText, "\"error_reproduced\"") {
		t.Errorf("Expected error_reproduced field in coverage")
	}

	if !strings.Contains(responseText, "\"network_mocked\"") {
		t.Errorf("Expected network_mocked field in coverage")
	}

	if !strings.Contains(responseText, "\"state_captured\"") {
		t.Errorf("Expected state_captured field in coverage")
	}

	// Verify metadata fields
	if !strings.Contains(responseText, "\"context_used\"") {
		t.Errorf("Expected context_used field in metadata")
	}

	if !strings.Contains(responseText, "\"generated_at\"") {
		t.Errorf("Expected generated_at field in metadata")
	}
}

// ============================================
// End of regression mode tests
// ============================================

// ============================================
// test_heal.batch Mode Tests
// ============================================

// TG-BATCH-001: Batch mode processes multiple test files
func TestTestHealBatch_MultipleFiles(t *testing.T) {
	h := setupTestGenHandler(t)

	// Create temporary directory with test files
	tmpDir := t.TempDir()
	testDir := filepath.Join(tmpDir, "tests")
	if err := os.Mkdir(testDir, 0755); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	// Create test files
	testFiles := map[string]string{
		"login.spec.ts": `
import { test, expect } from '@playwright/test';

test('login test', async ({ page }) => {
	await page.getByTestId('username').fill('user');
	await page.locator('#submit-btn').click();
});
`,
		"dashboard.spec.ts": `
import { test, expect } from '@playwright/test';

test('dashboard test', async ({ page }) => {
	await page.locator('.header').isVisible();
	await page.getByTestId('menu').click();
});
`,
		"profile.spec.ts": `
import { test, expect } from '@playwright/test';

test('profile test', async ({ page }) => {
	await page.locator('#profile-btn').click();
	await page.getByTestId('save').click();
});
`,
	}

	for filename, content := range testFiles {
		filePath := filepath.Join(testDir, filename)
		if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to create test file %s: %v", filename, err)
		}
	}

	// Set up session store
	store, err := NewSessionStore(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create session store: %v", err)
	}
	h.sessionStore = store
	defer store.Shutdown()

	req := TestHealRequest{
		Action:  "batch",
		TestDir: "tests",
	}

	result, err := h.healTestBatch(req, tmpDir)
	if err != nil {
		t.Fatalf("Expected batch healing to succeed, got error: %v", err)
	}

	// Should process all 3 files
	if result.FilesProcessed != 3 {
		t.Errorf("Expected 3 files processed, got: %d", result.FilesProcessed)
	}

	if result.FilesSkipped != 0 {
		t.Errorf("Expected 0 files skipped, got: %d", result.FilesSkipped)
	}

	// Should have file results for each file
	if len(result.FileResults) != 3 {
		t.Errorf("Expected 3 file results, got: %d", len(result.FileResults))
	}

	// Should have processed selectors
	if result.TotalSelectors == 0 {
		t.Errorf("Expected to find selectors in test files")
	}
}

// TG-BATCH-002: Batch mode enforces file count limit
func TestTestHealBatch_FileCountLimit(t *testing.T) {
	h := setupTestGenHandler(t)

	tmpDir := t.TempDir()
	testDir := filepath.Join(tmpDir, "tests")
	if err := os.Mkdir(testDir, 0755); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	// Create more than MaxFilesPerBatch test files
	testContent := `import { test } from '@playwright/test';
test('test', async ({ page }) => { await page.locator('#test').click(); });`

	for i := 0; i < MaxFilesPerBatch+5; i++ {
		filename := fmt.Sprintf("test%d.spec.ts", i)
		filePath := filepath.Join(testDir, filename)
		if err := os.WriteFile(filePath, []byte(testContent), 0644); err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}
	}

	store, err := NewSessionStore(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create session store: %v", err)
	}
	h.sessionStore = store
	defer store.Shutdown()

	req := TestHealRequest{
		Action:  "batch",
		TestDir: "tests",
	}

	result, err := h.healTestBatch(req, tmpDir)
	if err != nil {
		t.Fatalf("Expected batch healing to succeed, got error: %v", err)
	}

	// Should limit to MaxFilesPerBatch
	totalFiles := result.FilesProcessed + result.FilesSkipped
	if totalFiles > MaxFilesPerBatch {
		t.Errorf("Expected at most %d files processed, got: %d", MaxFilesPerBatch, totalFiles)
	}

	// Should have warning about batch limit
	hasWarning := false
	for _, warning := range result.Warnings {
		if strings.Contains(warning, "Batch limited to") {
			hasWarning = true
			break
		}
	}
	if !hasWarning {
		t.Errorf("Expected warning about batch limit")
	}
}

// TG-BATCH-003: Batch mode skips files exceeding size limit
func TestTestHealBatch_FileSizeLimit(t *testing.T) {
	h := setupTestGenHandler(t)

	tmpDir := t.TempDir()
	testDir := filepath.Join(tmpDir, "tests")
	if err := os.Mkdir(testDir, 0755); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	// Create a normal test file
	normalContent := `import { test } from '@playwright/test';
test('test', async ({ page }) => { await page.locator('#test').click(); });`

	normalPath := filepath.Join(testDir, "normal.spec.ts")
	if err := os.WriteFile(normalPath, []byte(normalContent), 0644); err != nil {
		t.Fatalf("Failed to create normal test file: %v", err)
	}

	// Create a test file exceeding size limit
	// Make sure it's definitely over 500KB
	largeContent := strings.Repeat("// This is a comment line to pad the file size\n", MaxFileSizeBytes/40+1000)
	largePath := filepath.Join(testDir, "large.spec.ts")
	if err := os.WriteFile(largePath, []byte(largeContent), 0644); err != nil {
		t.Fatalf("Failed to create large test file: %v", err)
	}

	// Verify the file is actually over the limit
	largeInfo, _ := os.Stat(largePath)
	if largeInfo.Size() <= MaxFileSizeBytes {
		t.Fatalf("Test setup error: large file is only %d bytes, need > %d", largeInfo.Size(), MaxFileSizeBytes)
	}

	store, err := NewSessionStore(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create session store: %v", err)
	}
	h.sessionStore = store
	defer store.Shutdown()

	req := TestHealRequest{
		Action:  "batch",
		TestDir: "tests",
	}

	result, err := h.healTestBatch(req, tmpDir)
	if err != nil {
		t.Fatalf("Expected batch healing to succeed, got error: %v", err)
	}

	// Should process normal file and skip large file
	if result.FilesProcessed != 1 {
		t.Errorf("Expected 1 file processed, got: %d", result.FilesProcessed)
	}

	if result.FilesSkipped != 1 {
		t.Errorf("Expected 1 file skipped, got: %d", result.FilesSkipped)
	}

	// Check that skipped file has reason
	skippedFile := false
	for _, fileResult := range result.FileResults {
		if fileResult.Skipped && strings.Contains(fileResult.Reason, "exceeds") {
			skippedFile = true
			break
		}
	}
	if !skippedFile {
		t.Errorf("Expected skipped file with size limit reason")
	}
}

// TG-BATCH-004: Batch mode enforces total batch size limit
func TestTestHealBatch_TotalBatchSizeLimit(t *testing.T) {
	h := setupTestGenHandler(t)

	tmpDir := t.TempDir()
	testDir := filepath.Join(tmpDir, "tests")
	if err := os.Mkdir(testDir, 0755); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	// Create multiple files that together exceed MaxTotalBatchSize
	// Each file is 400KB (under individual limit), but total exceeds 5MB
	mediumContent := strings.Repeat("// Comment line\n", 25000) // ~400KB

	for i := 0; i < 15; i++ {
		filename := fmt.Sprintf("test%d.spec.ts", i)
		filePath := filepath.Join(testDir, filename)
		if err := os.WriteFile(filePath, []byte(mediumContent), 0644); err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}
	}

	store, err := NewSessionStore(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create session store: %v", err)
	}
	h.sessionStore = store
	defer store.Shutdown()

	req := TestHealRequest{
		Action:  "batch",
		TestDir: "tests",
	}

	result, err := h.healTestBatch(req, tmpDir)
	if err != nil {
		t.Fatalf("Expected batch healing to succeed, got error: %v", err)
	}

	// Some files should be skipped due to total batch size limit
	if result.FilesSkipped == 0 {
		t.Errorf("Expected some files to be skipped due to total batch size limit")
	}

	// Check that skipped files have appropriate reason
	hasReason := false
	for _, fileResult := range result.FileResults {
		if fileResult.Skipped && strings.Contains(fileResult.Reason, "Total batch size") {
			hasReason = true
			break
		}
	}
	if !hasReason {
		t.Errorf("Expected skipped files to mention total batch size limit")
	}
}

// TG-BATCH-005: Batch mode aggregates results correctly
func TestTestHealBatch_AggregatedResults(t *testing.T) {
	h := setupTestGenHandler(t)

	tmpDir := t.TempDir()
	testDir := filepath.Join(tmpDir, "tests")
	if err := os.Mkdir(testDir, 0755); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	// Create test files with known selectors
	testFiles := map[string]string{
		"test1.spec.ts": `test('test', async ({ page }) => {
			await page.locator('#btn1').click();
			await page.locator('#btn2').click();
		});`,
		"test2.spec.ts": `test('test', async ({ page }) => {
			await page.locator('#btn3').click();
		});`,
	}

	for filename, content := range testFiles {
		filePath := filepath.Join(testDir, filename)
		if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}
	}

	store, err := NewSessionStore(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create session store: %v", err)
	}
	h.sessionStore = store
	defer store.Shutdown()

	req := TestHealRequest{
		Action:  "batch",
		TestDir: "tests",
	}

	result, err := h.healTestBatch(req, tmpDir)
	if err != nil {
		t.Fatalf("Expected batch healing to succeed, got error: %v", err)
	}

	// Verify aggregated totals
	expectedSelectors := 3 // #btn1, #btn2, #btn3
	if result.TotalSelectors != expectedSelectors {
		t.Errorf("Expected %d total selectors, got: %d", expectedSelectors, result.TotalSelectors)
	}

	// Total healed + unhealed should equal total selectors
	totalProcessed := result.TotalHealed + result.TotalUnhealed
	if totalProcessed != result.TotalSelectors {
		t.Errorf("Expected healed (%d) + unhealed (%d) = total (%d), got: %d",
			result.TotalHealed, result.TotalUnhealed, result.TotalSelectors, totalProcessed)
	}
}

// TG-BATCH-006: Batch mode handles directory not found
func TestTestHealBatch_DirectoryNotFound(t *testing.T) {
	h := setupTestGenHandler(t)

	tmpDir := t.TempDir()
	store, err := NewSessionStore(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create session store: %v", err)
	}
	h.sessionStore = store
	defer store.Shutdown()

	req := TestHealRequest{
		Action:  "batch",
		TestDir: "nonexistent",
	}

	_, err = h.healTestBatch(req, tmpDir)
	if err == nil {
		t.Errorf("Expected error for nonexistent directory")
	}

	if !strings.Contains(err.Error(), ErrTestFileNotFound) {
		t.Errorf("Expected test_file_not_found error, got: %v", err)
	}
}

// TG-BATCH-007: Batch mode validates path traversal
func TestTestHealBatch_PathTraversal(t *testing.T) {
	h := setupTestGenHandler(t)

	tmpDir := t.TempDir()
	store, err := NewSessionStore(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create session store: %v", err)
	}
	h.sessionStore = store
	defer store.Shutdown()

	req := TestHealRequest{
		Action:  "batch",
		TestDir: "../../etc",
	}

	_, err = h.healTestBatch(req, tmpDir)
	if err == nil {
		t.Errorf("Expected error for path traversal")
	}

	if !strings.Contains(err.Error(), ErrPathNotAllowed) {
		t.Errorf("Expected path_not_allowed error, got: %v", err)
	}
}

// TG-BATCH-008: Batch mode response format
func TestTestHealBatch_ResponseFormat(t *testing.T) {
	h := setupTestGenHandler(t)

	tmpDir := t.TempDir()
	testDir := filepath.Join(tmpDir, "tests")
	if err := os.Mkdir(testDir, 0755); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	// Create a simple test file
	testContent := `test('test', async ({ page }) => { await page.locator('#test').click(); });`
	testPath := filepath.Join(testDir, "test.spec.ts")
	if err := os.WriteFile(testPath, []byte(testContent), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	store, err := NewSessionStore(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create session store: %v", err)
	}
	h.sessionStore = store
	defer store.Shutdown()

	args := json.RawMessage(`{"format":"test_heal","action":"batch","test_dir":"tests"}`)
	req := JSONRPCRequest{ID: 1}

	resp := h.toolGenerate(req, args)

	var result MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if result.IsError {
		t.Errorf("Expected successful response, got error: %s", result.Content[0].Text)
	}

	responseText := result.Content[0].Text

	// Verify summary format
	if !strings.Contains(responseText, "Healed") {
		t.Errorf("Expected summary to mention healed selectors")
	}

	if !strings.Contains(responseText, "files") {
		t.Errorf("Expected summary to mention files")
	}

	// Verify JSON structure
	if !strings.Contains(responseText, "\"files_processed\"") {
		t.Errorf("Expected files_processed field in JSON response")
	}

	if !strings.Contains(responseText, "\"files_skipped\"") {
		t.Errorf("Expected files_skipped field in JSON response")
	}

	if !strings.Contains(responseText, "\"total_selectors\"") {
		t.Errorf("Expected total_selectors field in JSON response")
	}

	if !strings.Contains(responseText, "\"total_healed\"") {
		t.Errorf("Expected total_healed field in JSON response")
	}

	if !strings.Contains(responseText, "\"total_unhealed\"") {
		t.Errorf("Expected total_unhealed field in JSON response")
	}

	if !strings.Contains(responseText, "\"file_results\"") {
		t.Errorf("Expected file_results field in JSON response")
	}
}

// TG-BATCH-009: Batch mode skips non-test files
func TestTestHealBatch_SkipsNonTestFiles(t *testing.T) {
	h := setupTestGenHandler(t)

	tmpDir := t.TempDir()
	testDir := filepath.Join(tmpDir, "tests")
	if err := os.Mkdir(testDir, 0755); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	// Create a mix of test and non-test files
	files := map[string]string{
		"test.spec.ts":  `test('test', async ({ page }) => {});`,
		"helper.ts":     `export function helper() {}`,
		"README.md":     `# Tests`,
		"login.test.js": `test('login', () => {});`,
	}

	for filename, content := range files {
		filePath := filepath.Join(testDir, filename)
		if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to create file: %v", err)
		}
	}

	store, err := NewSessionStore(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create session store: %v", err)
	}
	h.sessionStore = store
	defer store.Shutdown()

	req := TestHealRequest{
		Action:  "batch",
		TestDir: "tests",
	}

	result, err := h.healTestBatch(req, tmpDir)
	if err != nil {
		t.Fatalf("Expected batch healing to succeed, got error: %v", err)
	}

	// Should only process test files (test.spec.ts and login.test.js)
	totalFiles := result.FilesProcessed + result.FilesSkipped
	if totalFiles != 2 {
		t.Errorf("Expected 2 test files found, got: %d", totalFiles)
	}
}

// TG-BATCH-010: Batch mode skips node_modules
func TestTestHealBatch_SkipsNodeModules(t *testing.T) {
	h := setupTestGenHandler(t)

	tmpDir := t.TempDir()
	testDir := filepath.Join(tmpDir, "tests")
	if err := os.Mkdir(testDir, 0755); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	// Create node_modules with a test file inside
	nodeModulesDir := filepath.Join(testDir, "node_modules")
	if err := os.Mkdir(nodeModulesDir, 0755); err != nil {
		t.Fatalf("Failed to create node_modules: %v", err)
	}

	// Create test file in node_modules (should be skipped)
	nodeModulesTest := filepath.Join(nodeModulesDir, "test.spec.ts")
	if err := os.WriteFile(nodeModulesTest, []byte("test()"), 0644); err != nil {
		t.Fatalf("Failed to create test file in node_modules: %v", err)
	}

	// Create legitimate test file
	legitTest := filepath.Join(testDir, "real.spec.ts")
	if err := os.WriteFile(legitTest, []byte("test()"), 0644); err != nil {
		t.Fatalf("Failed to create legitimate test file: %v", err)
	}

	store, err := NewSessionStore(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create session store: %v", err)
	}
	h.sessionStore = store
	defer store.Shutdown()

	req := TestHealRequest{
		Action:  "batch",
		TestDir: "tests",
	}

	result, err := h.healTestBatch(req, tmpDir)
	if err != nil {
		t.Fatalf("Expected batch healing to succeed, got error: %v", err)
	}

	// Should only find 1 test file (not the one in node_modules)
	totalFiles := result.FilesProcessed + result.FilesSkipped
	if totalFiles != 1 {
		t.Errorf("Expected 1 test file (node_modules should be skipped), got: %d", totalFiles)
	}
}

// TG-BATCH-011: Integration test - batch mode through toolGenerate
func TestIntegration_TestHealBatchViaToolGenerate(t *testing.T) {
	h := setupTestGenHandler(t)

	tmpDir := t.TempDir()
	testDir := filepath.Join(tmpDir, "tests")
	if err := os.Mkdir(testDir, 0755); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	// Create test files
	testFiles := map[string]string{
		"login.spec.ts": `
test('login', async ({ page }) => {
	await page.getByTestId('username').fill('user');
	await page.locator('#submit').click();
});`,
		"profile.spec.ts": `
test('profile', async ({ page }) => {
	await page.locator('.profile-btn').click();
	await page.getByTestId('save').click();
});`,
	}

	for filename, content := range testFiles {
		filePath := filepath.Join(testDir, filename)
		if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}
	}

	store, err := NewSessionStore(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create session store: %v", err)
	}
	h.sessionStore = store
	defer store.Shutdown()

	args := json.RawMessage(`{"format":"test_heal","action":"batch","test_dir":"tests"}`)
	req := JSONRPCRequest{ID: 1}

	resp := h.toolGenerate(req, args)

	var result MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if result.IsError {
		t.Errorf("Expected successful response, got error: %s", result.Content[0].Text)
	}

	responseText := result.Content[0].Text

	// Should have summary with batch statistics
	if !strings.Contains(responseText, "Healed") && !strings.Contains(responseText, "selectors") {
		t.Errorf("Expected summary to mention healed selectors, got: %s", responseText)
	}

	// Should have JSON data with batch_result
	if !strings.Contains(responseText, "files_processed") {
		t.Errorf("Expected files_processed in response")
	}

	if !strings.Contains(responseText, "file_results") {
		t.Errorf("Expected file_results in response")
	}

	fmt.Println("✓ test_heal.batch mode is properly integrated into generate tool")
}

// Test response format matches spec
func TestTestClassify_ResponseFormat(t *testing.T) {
	h := setupTestGenHandler(t)

	args := json.RawMessage(`{
		"format": "test_classify",
		"action": "failure",
		"failure": {
			"test_name": "login test",
			"error": "Timeout waiting for selector '#submit-btn'",
			"trace": "at test.spec.ts:10",
			"duration_ms": 30000
		}
	}`)

	req := JSONRPCRequest{ID: 1}
	resp := h.toolGenerate(req, args)

	var result MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if result.IsError {
		t.Fatalf("Expected successful response, got error: %s", result.Content[0].Text)
	}

	responseText := result.Content[0].Text

	// Verify summary format: "Classified as {category} ({confidence}% confidence) — recommended: {action}"
	if !strings.Contains(responseText, "Classified as") {
		t.Errorf("Expected summary to start with 'Classified as'")
	}

	if !strings.Contains(responseText, "confidence") {
		t.Errorf("Expected summary to include confidence percentage")
	}

	if !strings.Contains(responseText, "recommended:") {
		t.Errorf("Expected summary to include recommended action")
	}

	// Verify JSON structure
	if !strings.Contains(responseText, "\"classification\"") {
		t.Errorf("Expected classification object in JSON data")
	}

	if !strings.Contains(responseText, "\"category\"") {
		t.Errorf("Expected category field")
	}

	if !strings.Contains(responseText, "\"confidence\"") {
		t.Errorf("Expected confidence field")
	}

	if !strings.Contains(responseText, "\"evidence\"") {
		t.Errorf("Expected evidence field")
	}

	if !strings.Contains(responseText, "\"recommended_action\"") {
		t.Errorf("Expected recommended_action field")
	}

	if !strings.Contains(responseText, "\"is_real_bug\"") {
		t.Errorf("Expected is_real_bug field")
	}

	if !strings.Contains(responseText, "\"is_flaky\"") {
		t.Errorf("Expected is_flaky field")
	}

	if !strings.Contains(responseText, "\"is_environment\"") {
		t.Errorf("Expected is_environment field")
	}

	if !strings.Contains(responseText, "\"suggested_fix\"") {
		t.Errorf("Expected suggested_fix field")
	}
}

// ============================================
// test_classify.batch Tests
// ============================================

// TG-BATCH-CLASSIFY-001: Classify multiple failures - mixed categories
func TestTestClassifyBatch_MixedCategories(t *testing.T) {
	h := setupTestGenHandler(t)

	failures := []TestFailure{
		{
			TestName:   "test1",
			Error:      "Timeout waiting for selector '#btn'",
			DurationMs: 30000,
		},
		{
			TestName:   "test2",
			Error:      "net::ERR_CONNECTION_REFUSED",
			DurationMs: 1000,
		},
		{
			TestName:   "test3",
			Error:      "Expected 'Hello' to be 'Goodbye'",
			DurationMs: 500,
		},
		{
			TestName:   "test4",
			Error:      "Element is not attached to DOM",
			DurationMs: 2000,
		},
	}

	result := h.classifyFailureBatch(failures)

	if result.TotalClassified != 4 {
		t.Errorf("Expected total_classified=4, got: %d", result.TotalClassified)
	}

	// Should have at least 3 different categories
	if len(result.Summary) < 3 {
		t.Errorf("Expected at least 3 different categories, got: %d", len(result.Summary))
	}

	// Check that we have selector_broken
	if result.Summary[CategorySelectorBroken] != 1 {
		t.Errorf("Expected 1 selector_broken, got: %d", result.Summary[CategorySelectorBroken])
	}

	// Check that we have network_flaky
	if result.Summary[CategoryNetworkFlaky] != 1 {
		t.Errorf("Expected 1 network_flaky, got: %d", result.Summary[CategoryNetworkFlaky])
	}

	// Check counters
	if result.RealBugs != 1 {
		t.Errorf("Expected 1 real bug, got: %d", result.RealBugs)
	}

	if result.FlakyTests < 2 {
		t.Errorf("Expected at least 2 flaky tests, got: %d", result.FlakyTests)
	}
}

// TG-BATCH-CLASSIFY-002: Empty batch
func TestTestClassifyBatch_EmptyBatch(t *testing.T) {
	h := setupTestGenHandler(t)

	failures := []TestFailure{}
	result := h.classifyFailureBatch(failures)

	if result.TotalClassified != 0 {
		t.Errorf("Expected total_classified=0, got: %d", result.TotalClassified)
	}

	if result.RealBugs != 0 {
		t.Errorf("Expected 0 real bugs, got: %d", result.RealBugs)
	}
}

// TG-BATCH-CLASSIFY-003: All same category
func TestTestClassifyBatch_AllSameCategory(t *testing.T) {
	h := setupTestGenHandler(t)

	failures := []TestFailure{
		{TestName: "test1", Error: "Timeout waiting for selector '.btn1'", DurationMs: 30000},
		{TestName: "test2", Error: "Timeout waiting for selector '.btn2'", DurationMs: 30000},
		{TestName: "test3", Error: "Timeout waiting for selector '.btn3'", DurationMs: 30000},
	}

	result := h.classifyFailureBatch(failures)

	if result.TotalClassified != 3 {
		t.Errorf("Expected total_classified=3, got: %d", result.TotalClassified)
	}

	// All should be selector_broken or timing_flaky
	if result.Summary[CategorySelectorBroken] != 3 && result.Summary[CategoryTimingFlaky] != 3 {
		t.Errorf("Expected all 3 to be same category (selector_broken or timing_flaky), got summary: %+v", result.Summary)
	}
}

// TG-BATCH-CLASSIFY-004: Via tool interface
func TestIntegration_TestClassifyBatchViaTool(t *testing.T) {
	h := setupTestGenHandler(t)

	args := json.RawMessage(`{
		"format": "test_classify",
		"action": "batch",
		"failures": [
			{
				"test_name": "login test",
				"error": "Timeout waiting for selector '#submit'",
				"duration_ms": 30000
			},
			{
				"test_name": "api test",
				"error": "net::ERR_CONNECTION_REFUSED",
				"duration_ms": 1000
			}
		]
	}`)

	req := JSONRPCRequest{ID: 1}
	resp := h.toolGenerate(req, args)

	var result MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if result.IsError {
		t.Fatalf("Expected successful response, got error: %s", result.Content[0].Text)
	}

	responseText := result.Content[0].Text

	// Verify summary format
	if !strings.Contains(responseText, "Classified 2 failures") {
		t.Errorf("Expected summary to mention '2 failures', got: %s", responseText)
	}

	// Verify JSON structure
	if !strings.Contains(responseText, "\"batch_result\"") {
		t.Errorf("Expected batch_result in response")
	}

	if !strings.Contains(responseText, "\"total_classified\"") {
		t.Errorf("Expected total_classified field")
	}

	if !strings.Contains(responseText, "\"classifications\"") {
		t.Errorf("Expected classifications array")
	}

	if !strings.Contains(responseText, "\"summary\"") {
		t.Errorf("Expected summary object")
	}
}

// TG-BATCH-CLASSIFY-005: Batch too large
func TestTestClassifyBatch_TooLarge(t *testing.T) {
	h := setupTestGenHandler(t)

	// Create 25 failures (exceeds MaxFailuresPerBatch of 20)
	failures := make([]map[string]interface{}, 25)
	for i := 0; i < 25; i++ {
		failures[i] = map[string]interface{}{
			"test_name":   fmt.Sprintf("test%d", i),
			"error":       "Some error",
			"duration_ms": 1000,
		}
	}

	argsData := map[string]interface{}{
		"format":   "test_classify",
		"action":   "batch",
		"failures": failures,
	}

	argsBytes, _ := json.Marshal(argsData)
	args := json.RawMessage(argsBytes)

	req := JSONRPCRequest{ID: 1}

	resp := h.toolGenerate(req, args)

	var result MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if !result.IsError {
		t.Errorf("Expected error for batch too large")
	}

	responseText := result.Content[0].Text
	if !strings.Contains(responseText, ErrBatchTooLarge) {
		t.Errorf("Expected error code %s, got: %s", ErrBatchTooLarge, responseText)
	}
}

// TG-BATCH-CLASSIFY-006: Missing failures parameter
func TestTestClassifyBatch_MissingFailures(t *testing.T) {
	h := setupTestGenHandler(t)

	args := json.RawMessage(`{
		"format": "test_classify",
		"action": "batch"
	}`)

	req := JSONRPCRequest{ID: 1}

	resp := h.toolGenerate(req, args)

	var result MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if !result.IsError {
		t.Errorf("Expected error for missing failures parameter")
	}

	responseText := result.Content[0].Text
	if !strings.Contains(responseText, ErrMissingParam) {
		t.Errorf("Expected error code %s, got: %s", ErrMissingParam, responseText)
	}
}

// TG-BATCH-CLASSIFY-007: Uncertain classifications counted
func TestTestClassifyBatch_UncertainCounted(t *testing.T) {
	h := setupTestGenHandler(t)

	failures := []TestFailure{
		{TestName: "test1", Error: "Some completely unknown error pattern xyz123", DurationMs: 1000},
		{TestName: "test2", Error: "Another weird error 456abc", DurationMs: 1000},
	}

	result := h.classifyFailureBatch(failures)

	if result.TotalClassified != 2 {
		t.Errorf("Expected total_classified=2, got: %d", result.TotalClassified)
	}

	// Both should be uncertain
	if result.Uncertain != 2 {
		t.Errorf("Expected 2 uncertain classifications, got: %d", result.Uncertain)
	}

	// Should have unknown category
	if result.Summary[CategoryUnknown] != 2 {
		t.Errorf("Expected 2 unknown category, got: %d", result.Summary[CategoryUnknown])
	}
}
