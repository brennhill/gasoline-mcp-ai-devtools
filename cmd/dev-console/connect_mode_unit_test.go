// connect_mode_unit_test.go — Unit tests for connect mode helpers.
package main

import (
	"bufio"
	"encoding/json"
	"io"
	"os"
	"strings"
	"sync"
	"testing"
)

func TestSendMCPError(t *testing.T) {
	// Cannot use t.Parallel() — redirects os.Stdout

	t.Run("writes valid JSON-RPC error", func(t *testing.T) {
		r, w, err := os.Pipe()
		if err != nil {
			t.Fatalf("os.Pipe: %v", err)
		}
		oldStdout := os.Stdout
		os.Stdout = w

		sendMCPError(1, -32600, "Invalid Request")

		os.Stdout = oldStdout
		w.Close()

		out, err := io.ReadAll(r)
		if err != nil {
			t.Fatalf("ReadAll: %v", err)
		}
		r.Close()

		var resp JSONRPCResponse
		if err := json.Unmarshal([]byte(strings.TrimSpace(string(out))), &resp); err != nil {
			t.Fatalf("output is not valid JSON: %v\ngot: %s", err, out)
		}

		if resp.JSONRPC != "2.0" {
			t.Errorf("jsonrpc = %q, want 2.0", resp.JSONRPC)
		}
		if resp.Error == nil {
			t.Fatal("expected error field")
		}
		if resp.Error.Code != -32600 {
			t.Errorf("error code = %d, want -32600", resp.Error.Code)
		}
		if resp.Error.Message != "Invalid Request" {
			t.Errorf("error message = %q, want 'Invalid Request'", resp.Error.Message)
		}
	})

	t.Run("handles nil id", func(t *testing.T) {
		r, w, err := os.Pipe()
		if err != nil {
			t.Fatalf("os.Pipe: %v", err)
		}
		oldStdout := os.Stdout
		os.Stdout = w

		sendMCPError(nil, -32700, "Parse error")

		os.Stdout = oldStdout
		w.Close()

		out, err := io.ReadAll(r)
		if err != nil {
			t.Fatalf("ReadAll: %v", err)
		}
		r.Close()

		var resp JSONRPCResponse
		if err := json.Unmarshal([]byte(strings.TrimSpace(string(out))), &resp); err != nil {
			t.Fatalf("output is not valid JSON: %v\ngot: %s", err, out)
		}
		if resp.Error == nil || resp.Error.Code != -32700 {
			t.Fatalf("expected parse error code, got %+v", resp.Error)
		}
	})
}

// ============================================
// CR-13: stdout writes in connect mode must use mcpStdoutMu
// ============================================

func TestCR13_SendMCPError_ConcurrentWritesProduceValidJSON(t *testing.T) {
	// Cannot use t.Parallel() — redirects os.Stdout
	//
	// Launches multiple goroutines calling sendMCPError concurrently.
	// Each output line must be a complete, valid JSON-RPC response.
	// Without mutex protection, writes could interleave on large payloads.

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	oldStdout := os.Stdout
	os.Stdout = w

	const goroutines = 20
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func(id int) {
			defer wg.Done()
			sendMCPError(id, -32603, "concurrent error test")
		}(i)
	}
	wg.Wait()

	os.Stdout = oldStdout
	w.Close()

	scanner := bufio.NewScanner(r)
	lineCount := 0
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		lineCount++
		var resp JSONRPCResponse
		if err := json.Unmarshal([]byte(line), &resp); err != nil {
			t.Fatalf("line %d is not valid JSON: %v\ngot: %q", lineCount, err, line)
		}
		if resp.JSONRPC != "2.0" {
			t.Errorf("line %d: jsonrpc = %q, want 2.0", lineCount, resp.JSONRPC)
		}
		if resp.Error == nil || resp.Error.Code != -32603 {
			t.Errorf("line %d: unexpected error: %+v", lineCount, resp.Error)
		}
	}
	r.Close()

	if lineCount != goroutines {
		t.Fatalf("expected %d JSON lines, got %d", goroutines, lineCount)
	}
}
