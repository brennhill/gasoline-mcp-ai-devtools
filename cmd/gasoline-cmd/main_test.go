// main_test.go — Tests for CLI arg parsing, routing, and end-to-end flows.
package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunNoArgs(t *testing.T) {
	code := run([]string{})
	if code != 2 {
		t.Errorf("expected exit code 2 for no args, got %d", code)
	}
}

func TestRunVersion(t *testing.T) {
	code := run([]string{"--version"})
	if code != 0 {
		t.Errorf("expected exit code 0 for --version, got %d", code)
	}
}

func TestRunHelp(t *testing.T) {
	code := run([]string{"--help"})
	if code != 0 {
		t.Errorf("expected exit code 0 for --help, got %d", code)
	}
}

func TestRunHelpCommand(t *testing.T) {
	code := run([]string{"help"})
	if code != 0 {
		t.Errorf("expected exit code 0 for help command, got %d", code)
	}
}

func TestRunMissingAction(t *testing.T) {
	code := run([]string{"interact"})
	if code != 2 {
		t.Errorf("expected exit code 2 for missing action, got %d", code)
	}
}

func TestRunUnknownTool(t *testing.T) {
	code := run([]string{"unknown", "something"})
	if code != 2 {
		t.Errorf("expected exit code 2 for unknown tool, got %d", code)
	}
}

func TestRunInteractMissingSelector(t *testing.T) {
	// click requires --selector, so this should be a usage error
	code := run([]string{"interact", "click"})
	if code != 2 {
		t.Errorf("expected exit code 2 for missing selector, got %d", code)
	}
}

func TestExtractGlobalFlags(t *testing.T) {
	t.Parallel()

	args := []string{"--format", "json", "--server-port", "9224", "--timeout", "30000", "--stream", "--selector", "#btn"}
	flags, remaining := extractGlobalFlags(args)

	if flags.Format == nil || *flags.Format != "json" {
		t.Error("expected format=json")
	}
	if flags.ServerPort == nil || *flags.ServerPort != 9224 {
		t.Error("expected server-port=9224")
	}
	if flags.Timeout == nil || *flags.Timeout != 30000 {
		t.Error("expected timeout=30000")
	}
	if flags.Stream == nil || !*flags.Stream {
		t.Error("expected stream=true")
	}

	// --selector and #btn should remain
	if len(remaining) != 2 {
		t.Errorf("expected 2 remaining args, got %d: %v", len(remaining), remaining)
	}
}

func TestExtractGlobalFlagsNoAutoStart(t *testing.T) {
	t.Parallel()

	args := []string{"--no-auto-start", "--selector", "#btn"}
	flags, remaining := extractGlobalFlags(args)

	if flags.AutoStartServer == nil || *flags.AutoStartServer != false {
		t.Error("expected auto-start=false")
	}
	if len(remaining) != 2 {
		t.Errorf("expected 2 remaining args, got %d: %v", len(remaining), remaining)
	}
}

func TestParseInt(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input    string
		expected int
	}{
		{"123", 123},
		{"0", 0},
		{"9999", 9999},
		{"abc", 0},
		{"12abc", 0},
		{"", 0},
	}

	for _, tt := range tests {
		got := parseInt(tt.input)
		if got != tt.expected {
			t.Errorf("parseInt(%q) = %d, want %d", tt.input, got, tt.expected)
		}
	}
}

// TestEndToEndWithMockServer tests the full CLI flow against a mock MCP server.
func TestEndToEndWithMockServer(t *testing.T) {
	// Create a mock MCP server
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status":"ok"}`))
			return
		}

		var req struct {
			JSONRPC string          `json:"jsonrpc"`
			ID      any             `json:"id"`
			Method  string          `json:"method"`
			Params  json.RawMessage `json:"params,omitempty"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)

		switch req.Method {
		case "initialize":
			resp := map[string]any{
				"jsonrpc": "2.0",
				"id":      req.ID,
				"result": map[string]any{
					"protocolVersion": "2024-11-05",
					"serverInfo":      map[string]any{"name": "gasoline", "version": "5.8.0"},
					"capabilities":    map[string]any{"tools": map[string]any{}, "resources": map[string]any{}},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
		case "notifications/initialized":
			w.WriteHeader(http.StatusNoContent)
		case "tools/call":
			result := map[string]any{
				"content": []map[string]any{
					{"type": "text", "text": `{"entries":[],"total":0}`},
				},
			}
			resultJSON, _ := json.Marshal(result)
			resp := map[string]any{
				"jsonrpc": "2.0",
				"id":      req.ID,
				"result":  json.RawMessage(resultJSON),
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
		}
	}))
	defer srv.Close()

	// Extract port from mock server URL
	port := strings.TrimPrefix(srv.URL, "http://127.0.0.1:")

	code := run([]string{
		"observe", "logs",
		"--limit", "10",
		"--server-port", port,
		"--no-auto-start",
		"--format", "json",
	})

	if code != 0 {
		t.Errorf("expected exit code 0, got %d", code)
	}
}

// TestEndToEndInteractClick tests interact click against a mock server.
func TestEndToEndInteractClick(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status":"ok"}`))
			return
		}

		var req struct {
			JSONRPC string          `json:"jsonrpc"`
			ID      any             `json:"id"`
			Method  string          `json:"method"`
			Params  json.RawMessage `json:"params,omitempty"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)

		switch req.Method {
		case "initialize":
			resp := map[string]any{
				"jsonrpc": "2.0",
				"id":      req.ID,
				"result": map[string]any{
					"protocolVersion": "2024-11-05",
					"serverInfo":      map[string]any{"name": "gasoline", "version": "5.8.0"},
					"capabilities":    map[string]any{"tools": map[string]any{}, "resources": map[string]any{}},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
		case "notifications/initialized":
			w.WriteHeader(http.StatusNoContent)
		case "tools/call":
			// Verify the args
			var params struct {
				Name      string          `json:"name"`
				Arguments json.RawMessage `json:"arguments"`
			}
			_ = json.Unmarshal(req.Params, &params)

			var args map[string]any
			_ = json.Unmarshal(params.Arguments, &args)

			if args["action"] != "click" {
				t.Errorf("expected action 'click', got %v", args["action"])
			}
			if args["selector"] != "#submit-btn" {
				t.Errorf("expected selector '#submit-btn', got %v", args["selector"])
			}

			result := map[string]any{
				"content": []map[string]any{
					{"type": "text", "text": `{"clicked":true}`},
				},
			}
			resultJSON, _ := json.Marshal(result)
			resp := map[string]any{
				"jsonrpc": "2.0",
				"id":      req.ID,
				"result":  json.RawMessage(resultJSON),
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
		}
	}))
	defer srv.Close()

	port := strings.TrimPrefix(srv.URL, "http://127.0.0.1:")

	code := run([]string{
		"interact", "click",
		"--selector", "#submit-btn",
		"--server-port", port,
		"--no-auto-start",
	})

	if code != 0 {
		t.Errorf("expected exit code 0, got %d", code)
	}
}

// TestEndToEndToolCallError tests error handling for failed tool calls.
func TestEndToEndToolCallError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status":"ok"}`))
			return
		}

		var req struct {
			JSONRPC string          `json:"jsonrpc"`
			ID      any             `json:"id"`
			Method  string          `json:"method"`
			Params  json.RawMessage `json:"params,omitempty"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)

		switch req.Method {
		case "initialize":
			resp := map[string]any{
				"jsonrpc": "2.0",
				"id":      req.ID,
				"result": map[string]any{
					"protocolVersion": "2024-11-05",
					"serverInfo":      map[string]any{"name": "gasoline", "version": "5.8.0"},
					"capabilities":    map[string]any{"tools": map[string]any{}, "resources": map[string]any{}},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
		case "notifications/initialized":
			w.WriteHeader(http.StatusNoContent)
		case "tools/call":
			result := map[string]any{
				"content": []map[string]any{
					{"type": "text", "text": "Element not found: #bad-selector"},
				},
				"isError": true,
			}
			resultJSON, _ := json.Marshal(result)
			resp := map[string]any{
				"jsonrpc": "2.0",
				"id":      req.ID,
				"result":  json.RawMessage(resultJSON),
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
		}
	}))
	defer srv.Close()

	port := strings.TrimPrefix(srv.URL, "http://127.0.0.1:")

	code := run([]string{
		"interact", "click",
		"--selector", "#bad-selector",
		"--server-port", port,
		"--no-auto-start",
	})

	if code != 1 {
		t.Errorf("expected exit code 1 for tool error, got %d", code)
	}
}

// TestEndToEndBulkCSV tests bulk CSV processing.
func TestEndToEndBulkCSV(t *testing.T) {
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status":"ok"}`))
			return
		}

		var req struct {
			JSONRPC string          `json:"jsonrpc"`
			ID      any             `json:"id"`
			Method  string          `json:"method"`
			Params  json.RawMessage `json:"params,omitempty"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)

		switch req.Method {
		case "initialize":
			resp := map[string]any{
				"jsonrpc": "2.0",
				"id":      req.ID,
				"result": map[string]any{
					"protocolVersion": "2024-11-05",
					"serverInfo":      map[string]any{"name": "gasoline", "version": "5.8.0"},
					"capabilities":    map[string]any{"tools": map[string]any{}, "resources": map[string]any{}},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
		case "notifications/initialized":
			w.WriteHeader(http.StatusNoContent)
		case "tools/call":
			callCount++
			result := map[string]any{
				"content": []map[string]any{
					{"type": "text", "text": `{"status":"success"}`},
				},
			}
			resultJSON, _ := json.Marshal(result)
			resp := map[string]any{
				"jsonrpc": "2.0",
				"id":      req.ID,
				"result":  json.RawMessage(resultJSON),
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
		}
	}))
	defer srv.Close()

	// Create temp CSV file
	dir := t.TempDir()
	csvPath := filepath.Join(dir, "test.csv")
	err := os.WriteFile(csvPath, []byte("selector,text\n#title,Video 1\n#title,Video 2\n"), 0644)
	if err != nil {
		t.Fatalf("write CSV: %v", err)
	}

	port := strings.TrimPrefix(srv.URL, "http://127.0.0.1:")

	code := run([]string{
		"interact", "type",
		"--selector", "#title",
		"--csv-file", csvPath,
		"--server-port", port,
		"--no-auto-start",
		"--format", "csv",
	})

	if code != 0 {
		t.Errorf("expected exit code 0, got %d", code)
	}

	if callCount != 2 {
		t.Errorf("expected 2 tool calls for 2 CSV rows, got %d", callCount)
	}
}

func TestExtractFlag(t *testing.T) {
	t.Parallel()

	args := []string{"--selector", "#btn", "--format", "json", "--limit", "10"}

	val, remaining := extractFlag(args, "--format")
	if val != "json" {
		t.Errorf("expected 'json', got %q", val)
	}
	if len(remaining) != 4 {
		t.Errorf("expected 4 remaining, got %d", len(remaining))
	}

	// Flag not found
	val2, remaining2 := extractFlag(args, "--missing")
	if val2 != "" {
		t.Errorf("expected empty for missing flag, got %q", val2)
	}
	if len(remaining2) != len(args) {
		t.Errorf("expected same args for missing flag")
	}
}

// TestEndToEndServerNotRunning tests behavior when server is not running.
func TestEndToEndServerNotRunning(t *testing.T) {
	code := run([]string{
		"observe", "logs",
		"--server-port", "19999",
		"--no-auto-start",
	})

	if code != 1 {
		t.Errorf("expected exit code 1 for unreachable server, got %d", code)
	}
}
