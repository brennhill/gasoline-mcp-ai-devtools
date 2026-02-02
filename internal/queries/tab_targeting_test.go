// tab_targeting_test.go - Tests for Tab Targeting feature (Phase 0)
// Tests for: observe {what: "tabs"}, tab_id parameter on pending queries,
// and interact {action: "navigate", url: "..."}.
package queries

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// ============================================
// Test: observe {what: "tabs"} mode
// ============================================

func TestObserveTabsInToolsList(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "initialize",
		Params: json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}`),
	})

	resp := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 2, Method: "tools/list",
	})

	var result struct {
		Tools []struct {
			Name        string                 `json:"name"`
			InputSchema map[string]interface{} `json:"inputSchema"`
		} `json:"tools"`
	}

	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("Failed to parse tools list: %v", err)
	}

	// Find observe tool
	var observeTool struct {
		Name        string                 `json:"name"`
		InputSchema map[string]interface{} `json:"inputSchema"`
	}
	for _, tool := range result.Tools {
		if tool.Name == "observe" {
			observeTool = tool
			break
		}
	}

	if observeTool.Name == "" {
		t.Fatal("observe tool not found")
	}

	// Check that "tabs" is in the 'what' enum
	props, ok := observeTool.InputSchema["properties"].(map[string]interface{})
	if !ok {
		t.Fatal("observe should have properties")
	}

	what, ok := props["what"].(map[string]interface{})
	if !ok {
		t.Fatal("observe should have 'what' property")
	}

	enum, ok := what["enum"].([]interface{})
	if !ok {
		t.Fatal("'what' should have enum values")
	}

	tabsFound := false
	for _, v := range enum {
		if v == "tabs" {
			tabsFound = true
			break
		}
	}

	if !tabsFound {
		t.Error("Expected 'tabs' in observe what enum")
	}
}

func TestObserveTabsCreatesQuery(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "initialize",
		Params: json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}`),
	})

	// Start observe {what: "tabs"} in a goroutine
	done := make(chan JSONRPCResponse)
	go func() {
		resp := mcp.HandleRequest(JSONRPCRequest{
			JSONRPC: "2.0", ID: 2, Method: "tools/call",
			Params: json.RawMessage(`{"name":"observe","arguments":{"what":"tabs"}}`),
		})
		done <- resp
	}()

	// Check pending query is created
	time.Sleep(50 * time.Millisecond)
	pending := capture.GetPendingQueries()
	if len(pending) == 0 {
		t.Fatal("Expected pending query for observe tabs")
	}

	if pending[0].Type != "tabs" {
		t.Errorf("Expected query type 'tabs', got %s", pending[0].Type)
	}

	// Simulate extension response with tab list
	tabsResponse := `{"tabs":[{"id":1,"url":"https://example.com","title":"Example","active":true}]}`
	capture.SetQueryResult(pending[0].ID, json.RawMessage(tabsResponse))

	resp := <-done

	if resp.Error != nil {
		t.Fatalf("Expected no error, got: %v", resp.Error)
	}

	var result MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("Failed to parse result: %v", err)
	}

	if len(result.Content) == 0 {
		t.Fatal("Expected content in response")
	}

	if !strings.Contains(result.Content[0].Text, "example.com") {
		t.Errorf("Expected result to contain tab info, got: %s", result.Content[0].Text)
	}
}

// ============================================
// Test: tab_id field on PendingQuery
// ============================================

func TestPendingQueryWithTabID(t *testing.T) {
	t.Parallel()
	capture := setupTestCapture(t)

	// Create query targeting a specific tab
	id := capture.CreatePendingQuery(PendingQuery{
		Type:   "dom",
		Params: json.RawMessage(`{"selector":".test"}`),
		TabID:  42,
	})

	if id == "" {
		t.Error("Expected non-empty query ID")
	}

	pending := capture.GetPendingQueries()
	if len(pending) != 1 {
		t.Fatalf("Expected 1 pending query, got %d", len(pending))
	}

	if pending[0].TabID != 42 {
		t.Errorf("Expected TabID 42, got %d", pending[0].TabID)
	}
}

func TestPendingQueryWithoutTabID(t *testing.T) {
	t.Parallel()
	capture := setupTestCapture(t)

	// Create query without tab_id (falls back to active tab)
	id := capture.CreatePendingQuery(PendingQuery{
		Type:   "dom",
		Params: json.RawMessage(`{"selector":".test"}`),
	})

	if id == "" {
		t.Error("Expected non-empty query ID")
	}

	pending := capture.GetPendingQueries()
	if len(pending) != 1 {
		t.Fatalf("Expected 1 pending query, got %d", len(pending))
	}

	// TabID should be 0 (default, meaning "use active tab")
	if pending[0].TabID != 0 {
		t.Errorf("Expected TabID 0 for default, got %d", pending[0].TabID)
	}
}

func TestPendingQueryTabIDInResponse(t *testing.T) {
	t.Parallel()
	capture := setupTestCapture(t)

	// Create query with specific tab_id
	capture.CreatePendingQuery(PendingQuery{
		Type:   "dom",
		Params: json.RawMessage(`{"selector":".test"}`),
		TabID:  123,
	})

	// Get pending queries (as the extension would)
	pending := capture.GetPendingQueries()
	if len(pending) != 1 {
		t.Fatalf("Expected 1 pending query, got %d", len(pending))
	}

	// Serialize and verify tab_id is included
	responseJSON, err := json.Marshal(pending[0])
	if err != nil {
		t.Fatalf("Failed to marshal pending query: %v", err)
	}

	if !strings.Contains(string(responseJSON), `"tab_id":123`) {
		t.Errorf("Expected tab_id in serialized response, got: %s", string(responseJSON))
	}
}

// ============================================
// Test: interact {action: "navigate", url: "..."}
// ============================================

func TestBrowserActionOpenSchema(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "initialize",
		Params: json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}`),
	})

	resp := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 2, Method: "tools/list",
	})

	var result struct {
		Tools []struct {
			Name        string                 `json:"name"`
			InputSchema map[string]interface{} `json:"inputSchema"`
		} `json:"tools"`
	}

	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("Failed to parse tools list: %v", err)
	}

	// Find interact tool
	var interactTool struct {
		Name        string                 `json:"name"`
		InputSchema map[string]interface{} `json:"inputSchema"`
	}
	for _, tool := range result.Tools {
		if tool.Name == "interact" {
			interactTool = tool
			break
		}
	}

	if interactTool.Name == "" {
		t.Fatal("interact tool not found")
	}

	// Check that "navigate" is in the action enum
	props, ok := interactTool.InputSchema["properties"].(map[string]interface{})
	if !ok {
		t.Fatal("interact should have properties")
	}

	action, ok := props["action"].(map[string]interface{})
	if !ok {
		t.Fatal("interact should have 'action' property")
	}

	enum, ok := action["enum"].([]interface{})
	if !ok {
		t.Fatal("'action' should have enum values")
	}

	navigateFound := false
	for _, v := range enum {
		if v == "navigate" {
			navigateFound = true
			break
		}
	}

	if !navigateFound {
		t.Error("Expected 'navigate' in interact action enum")
	}
}

func TestBrowserActionOpenCreatesQuery(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "initialize",
		Params: json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}`),
	})

	// Simulate extension connection with pilot enabled
	capture.lastPollAt = time.Now()
	capture.pilotEnabled = true
	capture.pilotUpdatedAt = time.Now()

	// Start interact {action: "navigate", url: "..."} in a goroutine
	done := make(chan JSONRPCResponse)
	go func() {
		resp := mcp.HandleRequest(JSONRPCRequest{
			JSONRPC: "2.0", ID: 2, Method: "tools/call",
			Params: json.RawMessage(`{"name":"interact","arguments":{"action":"navigate","url":"https://example.com/test"}}`),
		})
		done <- resp
	}()

	// Check pending query is created
	time.Sleep(50 * time.Millisecond)
	pending := capture.GetPendingQueries()
	if len(pending) == 0 {
		t.Fatal("Expected pending query for interact navigate")
	}

	if pending[0].Type != "browser_action" {
		t.Errorf("Expected query type 'browser_action', got %s", pending[0].Type)
	}

	// Verify params include action and url
	var params map[string]interface{}
	if err := json.Unmarshal(pending[0].Params, &params); err != nil {
		t.Fatalf("Failed to unmarshal params: %v", err)
	}

	if params["action"] != "navigate" {
		t.Errorf("Expected action 'navigate', got %v", params["action"])
	}
	if params["url"] != "https://example.com/test" {
		t.Errorf("Expected url 'https://example.com/test', got %v", params["url"])
	}

	// Simulate extension response with new tab_id
	openResponse := `{"success":true,"action":"navigate","tab_id":999,"url":"https://example.com/test"}`
	capture.SetQueryResult(pending[0].ID, json.RawMessage(openResponse))

	resp := <-done

	if resp.Error != nil {
		t.Fatalf("Expected no error, got: %v", resp.Error)
	}

	var result MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("Failed to parse result: %v", err)
	}

	// Response should include the new tab_id
	if !strings.Contains(result.Content[0].Text, "tab_id") {
		t.Errorf("Expected result to contain tab_id, got: %s", result.Content[0].Text)
	}
	if !strings.Contains(result.Content[0].Text, "999") {
		t.Errorf("Expected result to contain tab_id 999, got: %s", result.Content[0].Text)
	}
}

func TestBrowserActionOpenRequiresURL(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "initialize",
		Params: json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}`),
	})

	// Call interact {action: "navigate"} without URL
	resp := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 2, Method: "tools/call",
		Params: json.RawMessage(`{"name":"interact","arguments":{"action":"navigate"}}`),
	})

	var result MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("Failed to parse result: %v", err)
	}

	// Should return an error about missing URL
	if !result.IsError {
		t.Error("Expected isError to be true when URL is missing for navigate action")
	}

	if !strings.Contains(result.Content[0].Text, "URL") {
		t.Errorf("Expected error message about URL, got: %s", result.Content[0].Text)
	}
}

// ============================================
// Test: tab_id parameter on AI Web Pilot tools
// ============================================

func TestQueryDOMWithTabID(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "initialize",
		Params: json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}`),
	})

	// Call configure with action query_dom and tab_id
	done := make(chan JSONRPCResponse)
	go func() {
		resp := mcp.HandleRequest(JSONRPCRequest{
			JSONRPC: "2.0", ID: 2, Method: "tools/call",
			Params: json.RawMessage(`{"name":"configure","arguments":{"action":"query_dom","selector":"h1","tab_id":42}}`),
		})
		done <- resp
	}()

	// Check pending query has tab_id
	time.Sleep(50 * time.Millisecond)
	pending := capture.GetPendingQueries()
	if len(pending) == 0 {
		t.Fatal("Expected pending query")
	}

	if pending[0].TabID != 42 {
		t.Errorf("Expected TabID 42 on pending query, got %d", pending[0].TabID)
	}

	capture.SetQueryResult(pending[0].ID, json.RawMessage(`{"matches":[],"match_count":0}`))
	<-done
}

func TestExecuteJavaScriptWithTabID(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "initialize",
		Params: json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}`),
	})

	// Simulate extension connection with pilot enabled
	capture.lastPollAt = time.Now()
	capture.pilotEnabled = true
	capture.pilotUpdatedAt = time.Now()

	// Call interact with action execute_js and tab_id
	done := make(chan JSONRPCResponse)
	go func() {
		resp := mcp.HandleRequest(JSONRPCRequest{
			JSONRPC: "2.0", ID: 2, Method: "tools/call",
			Params: json.RawMessage(`{"name":"interact","arguments":{"action":"execute_js","script":"return 1","tab_id":55}}`),
		})
		done <- resp
	}()

	// Check pending query has tab_id
	time.Sleep(50 * time.Millisecond)
	pending := capture.GetPendingQueries()
	if len(pending) == 0 {
		t.Fatal("Expected pending query")
	}

	if pending[0].TabID != 55 {
		t.Errorf("Expected TabID 55 on pending query, got %d", pending[0].TabID)
	}

	capture.SetQueryResult(pending[0].ID, json.RawMessage(`{"success":true,"result":1}`))
	<-done
}

func TestHighlightElementWithTabID(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "initialize",
		Params: json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}`),
	})

	// Simulate extension connection with pilot enabled
	capture.lastPollAt = time.Now()
	capture.pilotEnabled = true
	capture.pilotUpdatedAt = time.Now()

	// Call interact with action highlight and tab_id
	done := make(chan JSONRPCResponse)
	go func() {
		resp := mcp.HandleRequest(JSONRPCRequest{
			JSONRPC: "2.0", ID: 2, Method: "tools/call",
			Params: json.RawMessage(`{"name":"interact","arguments":{"action":"highlight","selector":"#test","tab_id":77}}`),
		})
		done <- resp
	}()

	// Check pending query has tab_id
	time.Sleep(50 * time.Millisecond)
	pending := capture.GetPendingQueries()
	if len(pending) == 0 {
		t.Fatal("Expected pending query")
	}

	if pending[0].TabID != 77 {
		t.Errorf("Expected TabID 77 on pending query, got %d", pending[0].TabID)
	}

	capture.SetQueryResult(pending[0].ID, json.RawMessage(`{"success":true,"selector":"#test"}`))
	<-done
}

func TestManageStateWithTabID(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "initialize",
		Params: json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}`),
	})

	// Simulate extension connection with pilot enabled
	capture.lastPollAt = time.Now()
	capture.pilotEnabled = true
	capture.pilotUpdatedAt = time.Now()

	// Call interact with action save_state and tab_id
	done := make(chan JSONRPCResponse)
	go func() {
		resp := mcp.HandleRequest(JSONRPCRequest{
			JSONRPC: "2.0", ID: 2, Method: "tools/call",
			Params: json.RawMessage(`{"name":"interact","arguments":{"action":"save_state","tab_id":88}}`),
		})
		done <- resp
	}()

	// Check pending query has tab_id
	time.Sleep(50 * time.Millisecond)
	pending := capture.GetPendingQueries()
	if len(pending) == 0 {
		t.Fatal("Expected pending query")
	}

	if pending[0].TabID != 88 {
		t.Errorf("Expected TabID 88 on pending query, got %d", pending[0].TabID)
	}

	capture.SetQueryResult(pending[0].ID, json.RawMessage(`{"success":true,"action":"capture"}`))
	<-done
}

func TestBrowserActionNavigateWithTabID(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "initialize",
		Params: json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}`),
	})

	// Simulate extension connection with pilot enabled
	capture.lastPollAt = time.Now()
	capture.pilotEnabled = true
	capture.pilotUpdatedAt = time.Now()

	// Call interact navigate with tab_id
	done := make(chan JSONRPCResponse)
	go func() {
		resp := mcp.HandleRequest(JSONRPCRequest{
			JSONRPC: "2.0", ID: 2, Method: "tools/call",
			Params: json.RawMessage(`{"name":"interact","arguments":{"action":"navigate","url":"https://test.com","tab_id":99}}`),
		})
		done <- resp
	}()

	// Check pending query has tab_id
	time.Sleep(50 * time.Millisecond)
	pending := capture.GetPendingQueries()
	if len(pending) == 0 {
		t.Fatal("Expected pending query")
	}

	if pending[0].TabID != 99 {
		t.Errorf("Expected TabID 99 on pending query, got %d", pending[0].TabID)
	}

	capture.SetQueryResult(pending[0].ID, json.RawMessage(`{"success":true,"action":"navigate"}`))
	<-done
}

// ============================================
// Test: tab_id parameter in tool schemas
// ============================================

func TestTabIDInQueryDOMSchema(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "initialize",
		Params: json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}`),
	})

	resp := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 2, Method: "tools/list",
	})

	var result struct {
		Tools []struct {
			Name        string                 `json:"name"`
			InputSchema map[string]interface{} `json:"inputSchema"`
		} `json:"tools"`
	}

	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("Failed to parse tools list: %v", err)
	}

	// Check configure has tab_id parameter
	for _, tool := range result.Tools {
		if tool.Name == "configure" {
			props, ok := tool.InputSchema["properties"].(map[string]interface{})
			if !ok {
				t.Fatal("configure should have properties")
			}
			if _, ok := props["tab_id"]; !ok {
				t.Error("configure should have 'tab_id' parameter")
			}
			return
		}
	}
	t.Fatal("configure tool not found")
}

func TestTabIDInExecuteJavaScriptSchema(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "initialize",
		Params: json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}`),
	})

	resp := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 2, Method: "tools/list",
	})

	var result struct {
		Tools []struct {
			Name        string                 `json:"name"`
			InputSchema map[string]interface{} `json:"inputSchema"`
		} `json:"tools"`
	}

	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("Failed to parse tools list: %v", err)
	}

	// Check interact has tab_id parameter
	for _, tool := range result.Tools {
		if tool.Name == "interact" {
			props, ok := tool.InputSchema["properties"].(map[string]interface{})
			if !ok {
				t.Fatal("interact should have properties")
			}
			if _, ok := props["tab_id"]; !ok {
				t.Error("interact should have 'tab_id' parameter")
			}
			return
		}
	}
	t.Fatal("interact tool not found")
}
