package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestV4NetworkBodiesBuffer(t *testing.T) {
	v4 := setupV4TestServer(t)

	bodies := []NetworkBody{
		{
			Timestamp:    "2024-01-15T10:30:00.000Z",
			Method:       "POST",
			URL:          "/api/users",
			Status:       201,
			RequestBody:  `{"name":"Alice"}`,
			ResponseBody: `{"id":1,"name":"Alice"}`,
			ContentType:  "application/json",
			Duration:     142,
		},
	}

	v4.AddNetworkBodies(bodies)

	if v4.GetNetworkBodyCount() != 1 {
		t.Errorf("Expected 1 body, got %d", v4.GetNetworkBodyCount())
	}
}

func TestV4NetworkBodiesBufferRotation(t *testing.T) {
	v4 := setupV4TestServer(t)

	// Add more than max (100) entries
	bodies := make([]NetworkBody, 120)
	for i := range bodies {
		bodies[i] = NetworkBody{Method: "GET", URL: "/api/test", Status: 200}
	}

	v4.AddNetworkBodies(bodies)

	if v4.GetNetworkBodyCount() != 100 {
		t.Errorf("Expected 100 bodies after rotation, got %d", v4.GetNetworkBodyCount())
	}
}

func TestV4NetworkBodiesFilterByURL(t *testing.T) {
	v4 := setupV4TestServer(t)

	v4.AddNetworkBodies([]NetworkBody{
		{URL: "/api/users", Status: 200},
		{URL: "/api/products", Status: 200},
		{URL: "/api/users/1", Status: 404},
	})

	filtered := v4.GetNetworkBodies(NetworkBodyFilter{URLFilter: "users"})

	if len(filtered) != 2 {
		t.Errorf("Expected 2 bodies matching 'users', got %d", len(filtered))
	}
}

func TestV4NetworkBodiesFilterByMethod(t *testing.T) {
	v4 := setupV4TestServer(t)

	v4.AddNetworkBodies([]NetworkBody{
		{URL: "/api/test", Method: "GET", Status: 200},
		{URL: "/api/test", Method: "POST", Status: 201},
		{URL: "/api/test", Method: "GET", Status: 200},
	})

	filtered := v4.GetNetworkBodies(NetworkBodyFilter{Method: "POST"})

	if len(filtered) != 1 {
		t.Errorf("Expected 1 POST body, got %d", len(filtered))
	}
}

func TestV4NetworkBodiesFilterByStatus(t *testing.T) {
	v4 := setupV4TestServer(t)

	v4.AddNetworkBodies([]NetworkBody{
		{URL: "/api/test", Status: 200},
		{URL: "/api/test", Status: 404},
		{URL: "/api/test", Status: 500},
		{URL: "/api/test", Status: 201},
	})

	// Filter for errors only (>= 400)
	filtered := v4.GetNetworkBodies(NetworkBodyFilter{StatusMin: 400})

	if len(filtered) != 2 {
		t.Errorf("Expected 2 error bodies, got %d", len(filtered))
	}

	// Filter for range 400-499
	filtered = v4.GetNetworkBodies(NetworkBodyFilter{StatusMin: 400, StatusMax: 499})

	if len(filtered) != 1 {
		t.Errorf("Expected 1 client error body, got %d", len(filtered))
	}
}

func TestV4NetworkBodiesFilterWithLimit(t *testing.T) {
	v4 := setupV4TestServer(t)

	for i := 0; i < 50; i++ {
		v4.AddNetworkBodies([]NetworkBody{
			{URL: "/api/test", Status: 200},
		})
	}

	filtered := v4.GetNetworkBodies(NetworkBodyFilter{Limit: 10})

	if len(filtered) != 10 {
		t.Errorf("Expected 10 bodies with limit, got %d", len(filtered))
	}
}

func TestV4NetworkBodiesDefaultLimit(t *testing.T) {
	v4 := setupV4TestServer(t)

	for i := 0; i < 50; i++ {
		v4.AddNetworkBodies([]NetworkBody{
			{URL: "/api/test", Status: 200},
		})
	}

	// Default limit is 20
	filtered := v4.GetNetworkBodies(NetworkBodyFilter{})

	if len(filtered) != 20 {
		t.Errorf("Expected 20 bodies with default limit, got %d", len(filtered))
	}
}

func TestV4NetworkBodiesNewestFirst(t *testing.T) {
	v4 := setupV4TestServer(t)

	v4.AddNetworkBodies([]NetworkBody{
		{URL: "/api/first", Timestamp: "2024-01-15T10:30:00.000Z"},
		{URL: "/api/last", Timestamp: "2024-01-15T10:30:05.000Z"},
	})

	filtered := v4.GetNetworkBodies(NetworkBodyFilter{})

	if filtered[0].URL != "/api/last" {
		t.Errorf("Expected newest first, got URL %s", filtered[0].URL)
	}
}

func TestV4NetworkBodiesTruncation(t *testing.T) {
	v4 := setupV4TestServer(t)

	// Request body > 8KB should be truncated
	largeBody := strings.Repeat("x", 10000)
	v4.AddNetworkBodies([]NetworkBody{
		{URL: "/api/test", RequestBody: largeBody, Status: 200},
	})

	filtered := v4.GetNetworkBodies(NetworkBodyFilter{})

	if len(filtered[0].RequestBody) > 8192 {
		t.Errorf("Expected request body truncated to 8KB, got %d bytes", len(filtered[0].RequestBody))
	}

	if !filtered[0].RequestTruncated {
		t.Error("Expected RequestTruncated flag to be true")
	}
}

func TestV4NetworkBodiesResponseTruncation(t *testing.T) {
	v4 := setupV4TestServer(t)

	// Response body > 16KB should be truncated
	largeBody := strings.Repeat("y", 20000)
	v4.AddNetworkBodies([]NetworkBody{
		{URL: "/api/test", ResponseBody: largeBody, Status: 200},
	})

	filtered := v4.GetNetworkBodies(NetworkBodyFilter{})

	if len(filtered[0].ResponseBody) > 16384 {
		t.Errorf("Expected response body truncated to 16KB, got %d bytes", len(filtered[0].ResponseBody))
	}

	if !filtered[0].ResponseTruncated {
		t.Error("Expected ResponseTruncated flag to be true")
	}
}

func TestV4PostNetworkBodiesEndpoint(t *testing.T) {
	v4 := setupV4TestServer(t)

	body := `{"bodies":[{"ts":"2024-01-15T10:30:00.000Z","method":"GET","url":"/api/test","status":200,"responseBody":"{}","contentType":"application/json"}]}`
	req := httptest.NewRequest("POST", "/network-bodies", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	v4.HandleNetworkBodies(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", rec.Code)
	}

	if v4.GetNetworkBodyCount() != 1 {
		t.Errorf("Expected 1 body stored, got %d", v4.GetNetworkBodyCount())
	}
}

func TestV4PostNetworkBodiesInvalidJSON(t *testing.T) {
	v4 := setupV4TestServer(t)

	req := httptest.NewRequest("POST", "/network-bodies", bytes.NewBufferString("garbage"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	v4.HandleNetworkBodies(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Expected 400, got %d", rec.Code)
	}
}

func TestMCPGetNetworkBodies(t *testing.T) {
	server, _ := setupTestServer(t)
	v4 := setupV4TestServer(t)
	mcp := NewMCPHandlerV4(server, v4)

	v4.AddNetworkBodies([]NetworkBody{
		{URL: "/api/users", Method: "GET", Status: 200, ResponseBody: `[{"id":1}]`},
		{URL: "/api/users", Method: "POST", Status: 201, RequestBody: `{"name":"Alice"}`},
	})

	mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "initialize",
		Params: json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}`),
	})

	resp := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 2, Method: "tools/call",
		Params: json.RawMessage(`{"name":"get_network_bodies","arguments":{}}`),
	})

	if resp.Error != nil {
		t.Fatalf("Expected no error, got: %v", resp.Error)
	}

	var result struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	json.Unmarshal(resp.Result, &result)

	var bodies []NetworkBody
	json.Unmarshal([]byte(result.Content[0].Text), &bodies)

	if len(bodies) != 2 {
		t.Errorf("Expected 2 bodies, got %d", len(bodies))
	}
}

func TestMCPGetNetworkBodiesWithFilter(t *testing.T) {
	server, _ := setupTestServer(t)
	v4 := setupV4TestServer(t)
	mcp := NewMCPHandlerV4(server, v4)

	v4.AddNetworkBodies([]NetworkBody{
		{URL: "/api/users", Method: "GET", Status: 200},
		{URL: "/api/users", Method: "GET", Status: 500},
	})

	mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "initialize",
		Params: json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}`),
	})

	resp := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 2, Method: "tools/call",
		Params: json.RawMessage(`{"name":"get_network_bodies","arguments":{"status_min":400}}`),
	})

	var result struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	json.Unmarshal(resp.Result, &result)

	var bodies []NetworkBody
	json.Unmarshal([]byte(result.Content[0].Text), &bodies)

	if len(bodies) != 1 {
		t.Errorf("Expected 1 error body, got %d", len(bodies))
	}
}

func TestMCPGetNetworkBodiesEmpty(t *testing.T) {
	server, _ := setupTestServer(t)
	v4 := setupV4TestServer(t)
	mcp := NewMCPHandlerV4(server, v4)

	mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "initialize",
		Params: json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}`),
	})

	resp := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 2, Method: "tools/call",
		Params: json.RawMessage(`{"name":"get_network_bodies","arguments":{}}`),
	})

	var result struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	json.Unmarshal(resp.Result, &result)

	if result.Content[0].Text != "No network bodies captured" {
		var bodies []NetworkBody
		if err := json.Unmarshal([]byte(result.Content[0].Text), &bodies); err == nil {
			if len(bodies) != 0 {
				t.Error("Expected empty bodies or message")
			}
		}
	}
}
