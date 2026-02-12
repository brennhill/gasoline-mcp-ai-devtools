package main

import (
	"bufio"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"
)

func captureBridgeIO(t *testing.T, input string, fn func()) string {
	t.Helper()

	inR, inW, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe(stdin) error = %v", err)
	}
	outR, outW, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe(stdout) error = %v", err)
	}

	oldIn := os.Stdin
	oldOut := os.Stdout
	os.Stdin = inR
	os.Stdout = outW

	_, _ = io.WriteString(inW, input)
	_ = inW.Close()

	fn()

	os.Stdin = oldIn
	os.Stdout = oldOut
	_ = inR.Close()
	_ = outW.Close()

	out, err := io.ReadAll(outR)
	if err != nil {
		t.Fatalf("ReadAll(stdout) error = %v", err)
	}
	_ = outR.Close()
	return string(out)
}

func parseJSONLines(t *testing.T, output string) []JSONRPCResponse {
	t.Helper()
	var responses []JSONRPCResponse
	sc := bufio.NewScanner(strings.NewReader(output))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var resp JSONRPCResponse
		if err := json.Unmarshal([]byte(line), &resp); err != nil {
			t.Fatalf("invalid JSON line %q: %v", line, err)
		}
		responses = append(responses, resp)
	}
	if err := sc.Err(); err != nil {
		t.Fatalf("scanner error: %v", err)
	}
	return responses
}

func TestBridgeFastPathCoreMethods(t *testing.T) {
	// Do not run in parallel; test redirects process stdio.
	state := &daemonState{readyCh: make(chan struct{}), failedCh: make(chan struct{})}
	input := strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,`,
		`{"jsonrpc":"2.0","id":2,"method":"initialize","params":{}}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/list","params":{}}`,
		`{"jsonrpc":"2.0","id":4,"method":"resources/list","params":{}}`,
		`{"jsonrpc":"2.0","id":5,"method":"unknown/method","params":{}}`,
		`{"jsonrpc":"2.0","id":6,"method":"tools/call","params":{"name":"observe","arguments":{"what":"errors"}}}`,
	}, "\n") + "\n"

	output := captureBridgeIO(t, input, func() {
		bridgeStdioToHTTPFast("http://127.0.0.1:1/mcp", state, 7890)
	})

	responses := parseJSONLines(t, output)
	if len(responses) != 6 {
		t.Fatalf("response count = %d, want 6", len(responses))
	}

	// Parse error response.
	if responses[0].Error == nil || responses[0].Error.Code != -32700 {
		t.Fatalf("parse-error response = %+v, want JSON parse error", responses[0])
	}

	// initialize response should include protocolVersion.
	if responses[1].Error != nil {
		t.Fatalf("initialize response has error: %+v", responses[1].Error)
	}
	var initResult map[string]any
	if err := json.Unmarshal(responses[1].Result, &initResult); err != nil {
		t.Fatalf("initialize result unmarshal error = %v", err)
	}
	if initResult["protocolVersion"] != "2024-11-05" {
		t.Fatalf("protocolVersion = %v, want 2024-11-05", initResult["protocolVersion"])
	}

	// tools/list fast path.
	var toolsResult map[string]any
	if err := json.Unmarshal(responses[2].Result, &toolsResult); err != nil {
		t.Fatalf("tools/list result unmarshal error = %v", err)
	}
	if _, ok := toolsResult["tools"]; !ok {
		t.Fatalf("tools/list result missing tools: %v", toolsResult)
	}

	// unknown method should return method-not-found protocol error.
	if responses[4].Error == nil || responses[4].Error.Code != -32601 {
		t.Fatalf("unknown method response = %+v, want -32601", responses[4])
	}

	// tools/call while not ready should be a soft tool error.
	if responses[5].Error != nil {
		t.Fatalf("tools/call startup response should be soft error, got protocol error %+v", responses[5].Error)
	}
	var startupResult map[string]any
	if err := json.Unmarshal(responses[5].Result, &startupResult); err != nil {
		t.Fatalf("startup result unmarshal error = %v", err)
	}
	if startupResult["isError"] != true {
		t.Fatalf("startup result isError = %v, want true", startupResult["isError"])
	}
}

func TestBridgeFastPathFailedDaemonMessage(t *testing.T) {
	// Do not run in parallel; test redirects process stdio.
	state := &daemonState{
		failed:   true,
		err:      "bind: address already in use",
		readyCh:  make(chan struct{}),
		failedCh: make(chan struct{}),
	}

	output := captureBridgeIO(t, `{"jsonrpc":"2.0","id":7,"method":"tools/call","params":{"name":"observe","arguments":{"what":"errors"}}}`+"\n", func() {
		bridgeStdioToHTTPFast("http://127.0.0.1:1/mcp", state, 7890)
	})
	responses := parseJSONLines(t, output)
	if len(responses) != 1 {
		t.Fatalf("response count = %d, want 1", len(responses))
	}

	var result map[string]any
	if err := json.Unmarshal(responses[0].Result, &result); err != nil {
		t.Fatalf("unmarshal soft error result: %v", err)
	}
	content, _ := result["content"].([]any)
	if len(content) == 0 {
		t.Fatalf("content = %v, want non-empty", result["content"])
	}
	first, _ := content[0].(map[string]any)
	text, _ := first["text"].(string)
	if !strings.Contains(text, "Port may be in use") {
		t.Fatalf("daemon failure message = %q, expected port suggestion", text)
	}
}

func TestSendJSONResponseFallbackOnMarshalError(t *testing.T) {
	// Do not run in parallel; test redirects process stdio.
	output := captureBridgeIO(t, "", func() {
		sendJSONResponse(map[string]any{"bad": make(chan int)}, 11)
	})
	responses := parseJSONLines(t, output)
	if len(responses) != 1 {
		t.Fatalf("response count = %d, want 1", len(responses))
	}
	if responses[0].Error == nil || responses[0].Error.Code != -32603 {
		t.Fatalf("sendJSONResponse fallback = %+v, want -32603 bridge error", responses[0])
	}
}

func TestBridgeServerHealthHelpers(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"status":"ok"}`)
	})

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen error = %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	srv := &http.Server{Handler: mux}
	go func() { _ = srv.Serve(ln) }()
	t.Cleanup(func() {
		_ = srv.Close()
	})

	if !isServerRunning(port) {
		t.Fatalf("isServerRunning(%d) = false, want true", port)
	}
	if !waitForServer(port, 750*time.Millisecond) {
		t.Fatalf("waitForServer(%d) = false, want true", port)
	}

	_ = srv.Close()
	time.Sleep(50 * time.Millisecond)
	if isServerRunning(port) {
		t.Fatalf("isServerRunning(%d) = true after shutdown, want false", port)
	}
}

func TestFlushStdoutNoPanic(t *testing.T) {
	// Do not run in parallel; test redirects process stdio.
	outR, outW, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe error = %v", err)
	}
	oldOut := os.Stdout
	os.Stdout = outW
	defer func() {
		os.Stdout = oldOut
		_ = outW.Close()
		_ = outR.Close()
	}()

	flushStdout()
}

func TestSendJSONResponseSuccess(t *testing.T) {
	// Do not run in parallel; test redirects process stdio.
	output := captureBridgeIO(t, "", func() {
		sendJSONResponse(JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      1,
			Result:  json.RawMessage(`{"ok":true}`),
		}, 1)
	})
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) == 0 {
		t.Fatal("expected JSON response line")
	}
	if !json.Valid([]byte(lines[0])) {
		t.Fatalf("invalid JSON output: %q", lines[0])
	}
}

func TestBridgeStdioLegacyParseError(t *testing.T) {
	// Do not run in parallel; test redirects process stdio.
	output := captureBridgeIO(t, `{"jsonrpc":"2.0","id":1,`+"\n", func() {
		bridgeStdioToHTTP("http://127.0.0.1:1/mcp")
	})
	if !strings.Contains(output, `"code":-32700`) {
		t.Fatalf("legacy bridge output missing parse error: %q", output)
	}
}

func TestBridgeStdioLegacyHTTPError(t *testing.T) {
	// Do not run in parallel; test redirects process stdio.
	mux := http.NewServeMux()
	mux.HandleFunc("/mcp", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = io.WriteString(w, "boom")
	})
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen error = %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	srv := &http.Server{Handler: mux}
	go func() { _ = srv.Serve(ln) }()
	defer func() { _ = srv.Close() }()

	input := `{"jsonrpc":"2.0","id":9,"method":"ping","params":{}}` + "\n"
	output := captureBridgeIO(t, input, func() {
		bridgeStdioToHTTP("http://127.0.0.1:" + strconv.Itoa(port) + "/mcp")
	})
	if !strings.Contains(output, `"code":-32603`) {
		t.Fatalf("expected bridge protocol error in output, got %q", output)
	}
}
