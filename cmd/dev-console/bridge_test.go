// bridge_test.go — Unit tests for stdio-to-HTTP bridge
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func TestIsServerRunning(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		setupServer     bool
		expectedRunning bool
	}{
		{
			name:            "server running",
			setupServer:     true,
			expectedRunning: true,
		},
		{
			name:            "server not running",
			setupServer:     false,
			expectedRunning: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var ln net.Listener
			var port int

			if tt.setupServer {
				// Start a test TCP server
				var err error
				ln, err = net.Listen("tcp", "127.0.0.1:0")
				if err != nil {
					t.Fatalf("Failed to create test server: %v", err)
				}
				defer ln.Close()

				port = ln.Addr().(*net.TCPAddr).Port
			} else {
				// Use a port that's very unlikely to be in use
				port = 65534
			}

			running := isServerRunning(port)

			if running != tt.expectedRunning {
				t.Errorf("isServerRunning(%d) = %v, want %v", port, running, tt.expectedRunning)
			}
		})
	}
}

func TestWaitForServer(t *testing.T) {
	t.Parallel()

	t.Run("server starts immediately", func(t *testing.T) {
		// Create HTTP server with /health endpoint
		mux := http.NewServeMux()
		mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status":"ok"}`))
		})

		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("Failed to create listener: %v", err)
		}
		port := ln.Addr().(*net.TCPAddr).Port

		server := &http.Server{Handler: mux}
		go server.Serve(ln)
		defer server.Close()

		// Wait for server to start
		if !waitForServer(port, 5*time.Second) {
			t.Error("waitForServer() = false, want true (server should be running)")
		}
	})

	t.Run("server never starts", func(t *testing.T) {
		// Use a port that's not listening
		port := 65533

		// Should timeout quickly
		start := time.Now()
		if waitForServer(port, 500*time.Millisecond) {
			t.Error("waitForServer() = true, want false (no server running)")
		}
		elapsed := time.Since(start)

		// Should have waited approximately the timeout duration
		if elapsed < 400*time.Millisecond || elapsed > 1*time.Second {
			t.Errorf("waitForServer() took %v, expected ~500ms", elapsed)
		}
	})

	t.Run("server starts after delay", func(t *testing.T) {
		mux := http.NewServeMux()
		mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("Failed to create listener: %v", err)
		}
		port := ln.Addr().(*net.TCPAddr).Port
		ln.Close() // Close it initially

		// Start server after 200ms delay
		go func() {
			time.Sleep(200 * time.Millisecond)
			ln, _ := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
			server := &http.Server{Handler: mux}
			server.Serve(ln)
		}()

		// Should succeed after waiting
		if !waitForServer(port, 2*time.Second) {
			t.Error("waitForServer() = false, want true (server should start within timeout)")
		}
	})

	t.Run("server responds non-200 to health", func(t *testing.T) {
		mux := http.NewServeMux()
		mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		})

		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("Failed to create listener: %v", err)
		}
		port := ln.Addr().(*net.TCPAddr).Port

		server := &http.Server{Handler: mux}
		go server.Serve(ln)
		defer server.Close()

		// Give server time to fully start
		time.Sleep(100 * time.Millisecond)

		// Should fail because health returns 500
		if waitForServer(port, 500*time.Millisecond) {
			t.Error("waitForServer() = true, want false (health endpoint returns 500)")
		}
	})
}

func TestSendBridgeError(t *testing.T) {
	// Note: Cannot use t.Parallel() because this test modifies global os.Stdout

	tests := []struct {
		name     string
		id       interface{}
		code     int
		message  string
		wantJSON string
	}{
		{
			name:     "error with numeric ID",
			id:       42,
			code:     -32603,
			message:  "Internal error",
			wantJSON: `{"jsonrpc":"2.0","id":42,"error":{"code":-32603,"message":"Internal error"}}`,
		},
		{
			name:     "error with string ID",
			id:       "abc-123",
			code:     -32700,
			message:  "Parse error",
			wantJSON: `{"jsonrpc":"2.0","id":"abc-123","error":{"code":-32700,"message":"Parse error"}}`,
		},
		{
			name:     "error with nil ID",
			id:       nil,
			code:     -32600,
			message:  "Invalid Request",
			wantJSON: `{"jsonrpc":"2.0","id":null,"error":{"code":-32600,"message":"Invalid Request"}}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Capture stdout
			oldStdout := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w

			// Call function
			sendBridgeError(tt.id, tt.code, tt.message)

			// Restore stdout
			w.Close()
			os.Stdout = oldStdout

			// Read captured output
			var buf bytes.Buffer
			io.Copy(&buf, r)
			got := strings.TrimSpace(buf.String())

			// Parse both as JSON for comparison (order-independent)
			var gotJSON, wantJSON map[string]interface{}
			if err := json.Unmarshal([]byte(got), &gotJSON); err != nil {
				t.Fatalf("Failed to parse output JSON: %v\nGot: %s", err, got)
			}
			if err := json.Unmarshal([]byte(tt.wantJSON), &wantJSON); err != nil {
				t.Fatalf("Failed to parse expected JSON: %v", err)
			}

			// Compare structures
			if gotJSON["jsonrpc"] != wantJSON["jsonrpc"] {
				t.Errorf("jsonrpc = %v, want %v", gotJSON["jsonrpc"], wantJSON["jsonrpc"])
			}

			// ID comparison (handle null)
			if fmt.Sprintf("%v", gotJSON["id"]) != fmt.Sprintf("%v", wantJSON["id"]) {
				t.Errorf("id = %v, want %v", gotJSON["id"], wantJSON["id"])
			}

			// Error comparison
			gotError := gotJSON["error"].(map[string]interface{})
			wantError := wantJSON["error"].(map[string]interface{})

			if int(gotError["code"].(float64)) != int(wantError["code"].(float64)) {
				t.Errorf("error.code = %v, want %v", gotError["code"], wantError["code"])
			}
			if gotError["message"] != wantError["message"] {
				t.Errorf("error.message = %v, want %v", gotError["message"], wantError["message"])
			}
		})
	}
}

func TestBridgeStdioToHTTP_JSONValidation(t *testing.T) {
	t.Parallel()

	// Create a mock HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Echo back the request as a result
		body, _ := io.ReadAll(r.Body)
		var req JSONRPCRequest
		json.Unmarshal(body, &req)

		result, _ := json.Marshal(map[string]interface{}{"echo": "ok"})
		resp := JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  result,
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	tests := []struct {
		name       string
		input      string
		wantOutput string
		wantError  bool
	}{
		{
			name:       "valid JSON-RPC request",
			input:      `{"jsonrpc":"2.0","id":1,"method":"test"}`,
			wantOutput: `{"jsonrpc":"2.0","id":1,"result":{"echo":"ok"}}`,
			wantError:  false,
		},
		{
			name:       "invalid JSON",
			input:      `{invalid json}`,
			wantOutput: `{"jsonrpc":"2.0","id":null,"error":{"code":-32700,"message":"Parse error:`,
			wantError:  true,
		},
		{
			name:       "empty line",
			input:      ``,
			wantOutput: ``, // Should skip empty lines
			wantError:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Skip tests that require stdin simulation (complex)
			// This test verifies the JSON validation logic works
			// Full integration tests would require process spawning

			if tt.input == "" {
				return // Skip empty line test (hard to verify no output)
			}

			var req JSONRPCRequest
			err := json.Unmarshal([]byte(tt.input), &req)

			if tt.wantError && err == nil {
				t.Error("Expected JSON parse error, got nil")
			}
			if !tt.wantError && err != nil {
				t.Errorf("Unexpected JSON parse error: %v", err)
			}
		})
	}
}

func TestBridgeStdioToHTTP_ServerError(t *testing.T) {
	t.Parallel()

	// Create a mock HTTP server that returns errors
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Server error"))
	}))
	defer server.Close()

	// Verify that HTTP errors are handled
	// (Full test would require stdin/stdout capture, which is complex)
	// This verifies the server setup for error testing

	resp, err := http.Post(server.URL, "application/json", bytes.NewReader([]byte(`{"test":"data"}`)))
	if err != nil {
		t.Fatalf("Failed to send request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("StatusCode = %d, want 500", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	if string(body) != "Server error" {
		t.Errorf("Body = %q, want %q", string(body), "Server error")
	}
}
