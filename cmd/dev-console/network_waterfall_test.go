package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// ============================================
// Network Waterfall Tests (TDD Phase 2)
// ============================================
// These tests verify the network waterfall capture system for complete
// CSP generation and security flagging.

// ============================================
// Basic Functionality Tests
// ============================================

func TestHandleNetworkWaterfall_AcceptsValidPayload(t *testing.T) {
	t.Parallel()
	capture := NewCapture()

	payload := NetworkWaterfallPayload{
		PageURL: "https://example.com",
		Entries: []NetworkWaterfallEntry{
			{
				Name:            "https://example.com/app.js",
				URL:             "https://example.com/app.js",
				InitiatorType:   "script",
				Duration:        50.5,
				TransferSize:    1024,
				DecodedBodySize: 2048,
			},
		},
	}

	body, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/network-waterfall", bytes.NewReader(body))
	w := httptest.NewRecorder()

	capture.HandleNetworkWaterfall(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Verify entry was stored
	capture.mu.RLock()
	count := len(capture.networkWaterfall)
	capture.mu.RUnlock()

	if count != 1 {
		t.Errorf("Expected 1 waterfall entry, got %d", count)
	}
}

func TestHandleNetworkWaterfall_RejectsMalformedJSON(t *testing.T) {
	t.Parallel()
	capture := NewCapture()

	req := httptest.NewRequest("POST", "/network-waterfall", strings.NewReader("{invalid json"))
	w := httptest.NewRecorder()

	capture.HandleNetworkWaterfall(w, req)

	if w.Code == http.StatusOK {
		t.Error("Should reject malformed JSON")
	}
}

func TestHandleNetworkWaterfall_StoresTimestamp(t *testing.T) {
	t.Parallel()
	capture := NewCapture()

	payload := NetworkWaterfallPayload{
		PageURL: "https://example.com",
		Entries: []NetworkWaterfallEntry{
			{
				Name:          "https://example.com/app.js",
				URL:           "https://example.com/app.js",
				InitiatorType: "script",
			},
		},
	}

	body, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/network-waterfall", bytes.NewReader(body))
	w := httptest.NewRecorder()

	before := time.Now()
	capture.HandleNetworkWaterfall(w, req)
	after := time.Now()

	capture.mu.RLock()
	entry := capture.networkWaterfall[0]
	capture.mu.RUnlock()

	if entry.Timestamp.Before(before) || entry.Timestamp.After(after) {
		t.Error("Timestamp should be set to current time")
	}
}

func TestHandleNetworkWaterfall_StoresPageURL(t *testing.T) {
	t.Parallel()
	capture := NewCapture()

	pageURL := "https://example.com/page"
	payload := NetworkWaterfallPayload{
		PageURL: pageURL,
		Entries: []NetworkWaterfallEntry{
			{
				Name:          "https://cdn.com/lib.js",
				URL:           "https://cdn.com/lib.js",
				InitiatorType: "script",
			},
		},
	}

	body, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/network-waterfall", bytes.NewReader(body))
	w := httptest.NewRecorder()

	capture.HandleNetworkWaterfall(w, req)

	capture.mu.RLock()
	entry := capture.networkWaterfall[0]
	capture.mu.RUnlock()

	if entry.PageURL != pageURL {
		t.Errorf("Expected PageURL %s, got %s", pageURL, entry.PageURL)
	}
}

// ============================================
// Ring Buffer Tests
// ============================================

func TestNetworkWaterfall_RingBufferEviction(t *testing.T) {
	t.Parallel()
	// Create capture with small capacity
	capture := NewCapture()
	capture.networkWaterfallCapacity = 3

	// Add 5 entries (should keep only last 3)
	for i := 1; i <= 5; i++ {
		payload := NetworkWaterfallPayload{
			PageURL: "https://example.com",
			Entries: []NetworkWaterfallEntry{
				{
					Name:          "https://example.com/file" + string(rune('0'+i)) + ".js",
					URL:           "https://example.com/file" + string(rune('0'+i)) + ".js",
					InitiatorType: "script",
				},
			},
		}

		body, _ := json.Marshal(payload)
		req := httptest.NewRequest("POST", "/network-waterfall", bytes.NewReader(body))
		w := httptest.NewRecorder()
		capture.HandleNetworkWaterfall(w, req)
	}

	capture.mu.RLock()
	count := len(capture.networkWaterfall)
	firstURL := capture.networkWaterfall[0].URL
	capture.mu.RUnlock()

	if count != 3 {
		t.Errorf("Expected 3 entries (capacity), got %d", count)
	}

	// Should have entries 3, 4, 5 (oldest evicted)
	if !strings.Contains(firstURL, "file3") {
		t.Errorf("Expected oldest entry to be file3, got %s", firstURL)
	}
}

func TestNetworkWaterfall_MultipleEntriesInSinglePayload(t *testing.T) {
	t.Parallel()
	capture := NewCapture()

	payload := NetworkWaterfallPayload{
		PageURL: "https://example.com",
		Entries: []NetworkWaterfallEntry{
			{Name: "https://example.com/a.js", URL: "https://example.com/a.js", InitiatorType: "script"},
			{Name: "https://example.com/b.css", URL: "https://example.com/b.css", InitiatorType: "stylesheet"},
			{Name: "https://example.com/c.png", URL: "https://example.com/c.png", InitiatorType: "img"},
		},
	}

	body, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/network-waterfall", bytes.NewReader(body))
	w := httptest.NewRecorder()

	capture.HandleNetworkWaterfall(w, req)

	capture.mu.RLock()
	count := len(capture.networkWaterfall)
	capture.mu.RUnlock()

	if count != 3 {
		t.Errorf("Expected 3 entries, got %d", count)
	}
}

// ============================================
// CSP Integration Tests
// ============================================

func TestNetworkWaterfall_FeedsCSPGenerator(t *testing.T) {
	t.Parallel()
	capture := NewCapture()

	// Send 3 requests to cdn.example.com on 2 different pages to ensure high confidence
	payloads := []NetworkWaterfallPayload{
		{
			PageURL: "https://myapp.com/page1",
			Entries: []NetworkWaterfallEntry{
				{
					Name:          "https://cdn.example.com/library.js",
					URL:           "https://cdn.example.com/library.js",
					InitiatorType: "script",
				},
			},
		},
		{
			PageURL: "https://myapp.com/page2",
			Entries: []NetworkWaterfallEntry{
				{
					Name:          "https://cdn.example.com/library.js",
					URL:           "https://cdn.example.com/library.js",
					InitiatorType: "script",
				},
			},
		},
		{
			PageURL: "https://myapp.com/page1",
			Entries: []NetworkWaterfallEntry{
				{
					Name:          "https://cdn.example.com/library.js",
					URL:           "https://cdn.example.com/library.js",
					InitiatorType: "script",
				},
			},
		},
	}

	for _, payload := range payloads {
		body, _ := json.Marshal(payload)
		req := httptest.NewRequest("POST", "/network-waterfall", bytes.NewReader(body))
		w := httptest.NewRecorder()
		capture.HandleNetworkWaterfall(w, req)
	}

	// Verify CSP generator received the origin and included it (high confidence: 3+ observations on 2+ pages)
	csp := capture.cspGen.GenerateCSP(CSPParams{})

	if !strings.Contains(csp.CSPHeader, "cdn.example.com") {
		t.Errorf("CSP should include origin from waterfall entry\nGot: %s", csp.CSPHeader)
	}
}

// ============================================
// Origin Extraction Tests
// ============================================

func TestExtractOrigin_StandardHTTPS(t *testing.T) {
	t.Parallel()
	origin := extractOrigin("https://example.com/path/to/resource")
	if origin != "https://example.com" {
		t.Errorf("Expected 'https://example.com', got '%s'", origin)
	}
}

func TestExtractOrigin_WithPort(t *testing.T) {
	t.Parallel()
	origin := extractOrigin("http://localhost:3000/api/data")
	if origin != "http://localhost:3000" {
		t.Errorf("Expected 'http://localhost:3000', got '%s'", origin)
	}
}

func TestExtractOrigin_DataURL(t *testing.T) {
	t.Parallel()
	origin := extractOrigin("data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAUA")
	if origin != "" {
		t.Errorf("Data URLs should return empty string, got '%s'", origin)
	}
}

func TestExtractOrigin_BlobURL(t *testing.T) {
	t.Parallel()
	origin := extractOrigin("blob:https://example.com/550e8400-e29b-41d4-a716-446655440000")
	if origin != "https://example.com" {
		t.Errorf("Expected 'https://example.com', got '%s'", origin)
	}
}

func TestExtractOrigin_MalformedURL(t *testing.T) {
	t.Parallel()
	origin := extractOrigin("not-a-valid-url")
	if origin != "" {
		t.Errorf("Malformed URLs should return empty string, got '%s'", origin)
	}
}

// ============================================
// Capacity Configuration Tests
// ============================================

func TestNetworkWaterfall_DefaultCapacity(t *testing.T) {
	t.Parallel()
	capture := NewCapture()

	// Default should be 1000
	if capture.networkWaterfallCapacity != DefaultNetworkWaterfallCapacity {
		t.Errorf("Expected default capacity %d, got %d",
			DefaultNetworkWaterfallCapacity, capture.networkWaterfallCapacity)
	}
}

func TestNetworkWaterfall_CustomCapacity(t *testing.T) {
	t.Parallel()
	capture := NewCapture()
	capture.networkWaterfallCapacity = 500

	if capture.networkWaterfallCapacity != 500 {
		t.Errorf("Expected capacity 500, got %d", capture.networkWaterfallCapacity)
	}
}

// ============================================
// Concurrent Access Tests
// ============================================

func TestNetworkWaterfall_ConcurrentWrites(t *testing.T) {
	t.Parallel()
	capture := NewCapture()

	// Write 100 entries concurrently
	done := make(chan bool, 100)
	for i := 0; i < 100; i++ {
		go func(index int) {
			payload := NetworkWaterfallPayload{
				PageURL: "https://example.com",
				Entries: []NetworkWaterfallEntry{
					{
						Name:          "https://example.com/file.js",
						URL:           "https://example.com/file.js",
						InitiatorType: "script",
					},
				},
			}

			body, _ := json.Marshal(payload)
			req := httptest.NewRequest("POST", "/network-waterfall", bytes.NewReader(body))
			w := httptest.NewRecorder()
			capture.HandleNetworkWaterfall(w, req)
			done <- true
		}(i)
	}

	// Wait for all to complete
	for i := 0; i < 100; i++ {
		<-done
	}

	capture.mu.RLock()
	count := len(capture.networkWaterfall)
	capture.mu.RUnlock()

	if count != 100 {
		t.Errorf("Expected 100 entries, got %d", count)
	}
}

// ============================================
// MCP Tool Handler Tests (BUG-001 Fix)
// ============================================
// These tests verify the MCP handler that retrieves waterfall data

// Test 1: Empty buffer returns helpful message
func TestToolGetNetworkWaterfall_EmptyBuffer(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`1`),
		Method:  "tools/call",
	}
	args := json.RawMessage(`{}`)
	resp := mcp.toolHandler.toolGetNetworkWaterfall(req, args)

	var content struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	json.Unmarshal(resp.Result, &content)

	if len(content.Content) == 0 {
		t.Fatal("Expected content in response")
	}

	text := content.Content[0].Text

	// Parse JSON response
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(text), &data); err != nil {
		t.Fatalf("Expected JSON response, got: %s", text)
	}

	if count, ok := data["count"].(float64); !ok || count != 0 {
		t.Errorf("Expected count=0, got: %v", data["count"])
	}

	if entries, ok := data["entries"].([]interface{}); !ok || len(entries) != 0 {
		t.Errorf("Expected empty entries array, got: %v", data["entries"])
	}
}

// Test 2: Populated buffer returns markdown table
func TestToolGetNetworkWaterfall_PopulatedBuffer(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	// Add waterfall entries
	capture.mu.Lock()
	capture.networkWaterfall = []NetworkWaterfallEntry{
		{
			Name:            "https://example.com/api/users",
			URL:             "https://example.com/api/users",
			InitiatorType:   "fetch",
			Duration:        123.45,
			TransferSize:    1234,
			DecodedBodySize: 2468,
			EncodedBodySize: 1234,
			PageURL:         "https://example.com",
			Timestamp:       time.Now(),
		},
		{
			Name:            "https://cdn.example.com/script.js",
			URL:             "https://cdn.example.com/script.js",
			InitiatorType:   "script",
			Duration:        89.12,
			TransferSize:    0, // Cached
			DecodedBodySize: 11356,
			EncodedBodySize: 5678,
			PageURL:         "https://example.com",
			Timestamp:       time.Now(),
		},
	}
	capture.mu.Unlock()

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`1`),
		Method:  "tools/call",
	}
	args := json.RawMessage(`{}`)
	resp := mcp.toolHandler.toolGetNetworkWaterfall(req, args)

	var content struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	json.Unmarshal(resp.Result, &content)

	text := content.Content[0].Text

	// Parse JSON response (skip summary line if present)
	lines := strings.Split(text, "\n")
	jsonText := text
	if len(lines) > 1 && !strings.HasPrefix(text, "{") {
		jsonText = strings.Join(lines[1:], "\n")
	}

	var data map[string]interface{}
	if err := json.Unmarshal([]byte(jsonText), &data); err != nil {
		t.Fatalf("Expected JSON response, got: %s", text)
	}

	// Verify count
	if count, ok := data["count"].(float64); !ok || count != 2 {
		t.Errorf("Expected count=2, got: %v", data["count"])
	}

	// Verify entries array
	entries, ok := data["entries"].([]interface{})
	if !ok {
		t.Fatal("Expected entries array")
	}
	if len(entries) != 2 {
		t.Errorf("Expected 2 entries, got %d", len(entries))
	}

	// Verify first entry
	entry1 := entries[0].(map[string]interface{})
	if !strings.Contains(entry1["url"].(string), "example.com/api/users") {
		t.Error("Expected first URL to contain example.com/api/users")
	}
	if entry1["initiatorType"].(string) != "fetch" {
		t.Error("Expected first entry initiatorType=fetch")
	}

	// Verify second entry (cached)
	entry2 := entries[1].(map[string]interface{})
	if !strings.Contains(entry2["url"].(string), "cdn.example.com/script.js") {
		t.Error("Expected second URL to contain cdn.example.com/script.js")
	}
	if entry2["initiatorType"].(string) != "script" {
		t.Error("Expected second entry initiatorType=script")
	}
	if cached, ok := entry2["cached"].(bool); !ok || !cached {
		t.Error("Expected second entry to be cached (transferSize=0)")
	}
}

// Test 3: limit parameter returns last N entries
func TestToolGetNetworkWaterfall_LimitParameter(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	// Add 5 entries
	capture.mu.Lock()
	for i := 1; i <= 5; i++ {
		capture.networkWaterfall = append(capture.networkWaterfall, NetworkWaterfallEntry{
			Name:          fmt.Sprintf("https://example.com/resource%d", i),
			URL:           fmt.Sprintf("https://example.com/resource%d", i),
			InitiatorType: "fetch",
			Duration:      100.0,
			PageURL:       "https://example.com",
			Timestamp:     time.Now(),
		})
	}
	capture.mu.Unlock()

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`1`),
		Method:  "tools/call",
	}
	args := json.RawMessage(`{"limit": 2}`)
	resp := mcp.toolHandler.toolGetNetworkWaterfall(req, args)

	var content struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	json.Unmarshal(resp.Result, &content)

	text := content.Content[0].Text

	// Parse JSON response (skip summary line if present)
	lines := strings.Split(text, "\n")
	jsonText := text
	if len(lines) > 1 && !strings.HasPrefix(text, "{") {
		jsonText = strings.Join(lines[1:], "\n")
	}

	var data map[string]interface{}
	if err := json.Unmarshal([]byte(jsonText), &data); err != nil {
		t.Fatalf("Expected JSON response, got: %s", text)
	}

	// Verify count
	if count, ok := data["count"].(float64); !ok || count != 2 {
		t.Errorf("Expected count=2, got: %v", data["count"])
	}

	// Verify only 2 entries returned (last 2: resource4 and resource5)
	entries, ok := data["entries"].([]interface{})
	if !ok {
		t.Fatal("Expected entries array")
	}
	if len(entries) != 2 {
		t.Errorf("Expected 2 entries due to limit, got %d", len(entries))
	}
}

// Test 4: url filter parameter filters entries
func TestToolGetNetworkWaterfall_URLFilter(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	capture.mu.Lock()
	capture.networkWaterfall = []NetworkWaterfallEntry{
		{URL: "https://api.example.com/users", InitiatorType: "fetch", Timestamp: time.Now()},
		{URL: "https://cdn.example.com/style.css", InitiatorType: "stylesheet", Timestamp: time.Now()},
		{URL: "https://api.example.com/posts", InitiatorType: "fetch", Timestamp: time.Now()},
	}
	capture.mu.Unlock()

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`1`),
		Method:  "tools/call",
	}
	args := json.RawMessage(`{"url": "api.example.com"}`)
	resp := mcp.toolHandler.toolGetNetworkWaterfall(req, args)

	var content struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	json.Unmarshal(resp.Result, &content)

	text := content.Content[0].Text

	// Parse JSON response (skip summary line if present)
	lines := strings.Split(text, "\n")
	jsonText := text
	if len(lines) > 1 && !strings.HasPrefix(text, "{") {
		jsonText = strings.Join(lines[1:], "\n")
	}

	var data map[string]interface{}
	if err := json.Unmarshal([]byte(jsonText), &data); err != nil {
		t.Fatalf("Expected JSON response, got: %s", text)
	}

	// Verify only API URLs are included (filtered by "api.example.com")
	entries, ok := data["entries"].([]interface{})
	if !ok {
		t.Fatal("Expected entries array")
	}
	if len(entries) != 2 {
		t.Errorf("Expected 2 filtered entries, got %d", len(entries))
	}

	for _, e := range entries {
		entry := e.(map[string]interface{})
		url := entry["url"].(string)
		if !strings.Contains(url, "api.example.com") {
			t.Errorf("Expected URL to contain 'api.example.com', got: %s", url)
		}
		if strings.Contains(url, "style.css") {
			t.Error("Should not contain non-API URLs")
		}
	}
}

// Test 5: Concurrent access safety
func TestToolGetNetworkWaterfall_ConcurrentAccessSafety(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`1`),
		Method:  "tools/call",
	}

	done := make(chan bool, 2)

	// Writer goroutine
	go func() {
		for i := 0; i < 100; i++ {
			capture.mu.Lock()
			capture.networkWaterfall = append(capture.networkWaterfall, NetworkWaterfallEntry{
				URL:       fmt.Sprintf("https://example.com/%d", i),
				Timestamp: time.Now(),
			})
			capture.mu.Unlock()
			time.Sleep(1 * time.Millisecond)
		}
		done <- true
	}()

	// Reader goroutine
	go func() {
		for i := 0; i < 50; i++ {
			args := json.RawMessage(`{}`)
			mcp.toolHandler.toolGetNetworkWaterfall(req, args)
			time.Sleep(2 * time.Millisecond)
		}
		done <- true
	}()

	<-done
	<-done
	// Should not panic or race
}
