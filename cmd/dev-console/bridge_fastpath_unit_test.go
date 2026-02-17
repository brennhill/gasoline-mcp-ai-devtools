package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	statecfg "github.com/dev-console/dev-console/internal/state"
)

func contentLengthFrame(payload string) string {
	return fmt.Sprintf("Content-Length: %d\r\nContent-Type: application/json\r\n\r\n%s", len(payload), payload)
}

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

func captureBridgeIOWithStderr(t *testing.T, input string, fn func()) (string, string) {
	t.Helper()

	inR, inW, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe(stdin) error = %v", err)
	}
	outR, outW, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe(stdout) error = %v", err)
	}
	errR, errW, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe(stderr) error = %v", err)
	}

	oldIn := os.Stdin
	oldOut := os.Stdout
	oldErr := os.Stderr
	os.Stdin = inR
	os.Stdout = outW
	os.Stderr = errW

	_, _ = io.WriteString(inW, input)
	_ = inW.Close()

	fn()

	os.Stdin = oldIn
	os.Stdout = oldOut
	os.Stderr = oldErr
	_ = inR.Close()
	_ = outW.Close()
	_ = errW.Close()

	stdout, readErr := io.ReadAll(outR)
	if readErr != nil {
		t.Fatalf("ReadAll(stdout) error = %v", readErr)
	}
	_ = outR.Close()
	stderr, readErr := io.ReadAll(errR)
	if readErr != nil {
		t.Fatalf("ReadAll(stderr) error = %v", readErr)
	}
	_ = errR.Close()

	return string(stdout), string(stderr)
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
	resetFastPathResourceReadCounters()
	state := &daemonState{readyCh: make(chan struct{}), failedCh: make(chan struct{})}
	input := strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,`,
		`{"jsonrpc":"2.0","id":2,"method":"initialize","params":{}}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/list","params":{}}`,
		`{"jsonrpc":"2.0","id":4,"method":"resources/list","params":{}}`,
		`{"jsonrpc":"2.0","id":5,"method":"resources/read","params":{"uri":"gasoline://capabilities"}}`,
		`{"jsonrpc":"2.0","id":6,"method":"unknown/method","params":{}}`,
		`{"jsonrpc":"2.0","id":7,"method":"tools/call","params":{"name":"observe","arguments":{"what":"errors"}}}`,
	}, "\n") + "\n"

	output := captureBridgeIO(t, input, func() {
		bridgeStdioToHTTPFast("http://127.0.0.1:1/mcp", state, 7890)
	})

	responses := parseJSONLines(t, output)
	if len(responses) != 7 {
		t.Fatalf("response count = %d, want 7", len(responses))
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

	// resources/read should be served by fast path while daemon is still starting.
	var resourcesReadResult map[string]any
	if err := json.Unmarshal(responses[4].Result, &resourcesReadResult); err != nil {
		t.Fatalf("resources/read result unmarshal error = %v", err)
	}
	contents, _ := resourcesReadResult["contents"].([]any)
	if len(contents) != 1 {
		t.Fatalf("resources/read contents = %v, want one content item", resourcesReadResult["contents"])
	}

	// unknown method should return method-not-found protocol error.
	if responses[5].Error == nil || responses[5].Error.Code != -32601 {
		t.Fatalf("unknown method response = %+v, want -32601", responses[5])
	}

	// tools/call while not ready should be a soft tool error.
	if responses[6].Error != nil {
		t.Fatalf("tools/call startup response should be soft error, got protocol error %+v", responses[6].Error)
	}
	var startupResult map[string]any
	if err := json.Unmarshal(responses[6].Result, &startupResult); err != nil {
		t.Fatalf("startup result unmarshal error = %v", err)
	}
	if startupResult["isError"] != true {
		t.Fatalf("startup result isError = %v, want true", startupResult["isError"])
	}

	success, failure := snapshotFastPathResourceReadCounters()
	if success != 1 || failure != 0 {
		t.Fatalf("fast-path resources/read counters = (%d,%d), want (1,0)", success, failure)
	}
}

func TestBridgeFastPathResourcesReadCanonicalizesPlaybookAliases(t *testing.T) {
	// Do not run in parallel; test redirects process stdio.
	resetFastPathResourceReadCounters()
	state := &daemonState{readyCh: make(chan struct{}), failedCh: make(chan struct{})}
	input := strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"resources/read","params":{"uri":"gasoline://playbook/security"}}`,
		`{"jsonrpc":"2.0","id":2,"method":"resources/read","params":{"uri":"gasoline://playbook/security_audit/quick"}}`,
	}, "\n") + "\n"

	output := captureBridgeIO(t, input, func() {
		bridgeStdioToHTTPFast("http://127.0.0.1:1/mcp", state, 7890)
	})

	responses := parseJSONLines(t, output)
	if len(responses) != 2 {
		t.Fatalf("response count = %d, want 2", len(responses))
	}
	for i, resp := range responses {
		if resp.Error != nil {
			t.Fatalf("response[%d] error = %+v, want success", i, resp.Error)
		}
		var result map[string]any
		if err := json.Unmarshal(resp.Result, &result); err != nil {
			t.Fatalf("response[%d] unmarshal error = %v", i, err)
		}
		contents, _ := result["contents"].([]any)
		if len(contents) != 1 {
			t.Fatalf("response[%d] contents = %v, want one content", i, result["contents"])
		}
		content, _ := contents[0].(map[string]any)
		if content["uri"] != "gasoline://playbook/security/quick" {
			t.Fatalf("response[%d] content uri = %v, want gasoline://playbook/security/quick", i, content["uri"])
		}
	}

	success, failure := snapshotFastPathResourceReadCounters()
	if success != 2 || failure != 0 {
		t.Fatalf("fast-path resources/read counters = (%d,%d), want (2,0)", success, failure)
	}
}

func TestBridgeFastPathResourcesReadFailureTelemetry(t *testing.T) {
	// Do not run in parallel; test redirects process stdio.
	resetFastPathResourceReadCounters()
	state := &daemonState{readyCh: make(chan struct{}), failedCh: make(chan struct{})}
	input := strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"resources/read","params":{"uri":"gasoline://playbook/nonexistent/quick"}}`,
		`{"jsonrpc":"2.0","id":2,"method":"resources/read","params":[]}`,
	}, "\n") + "\n"

	output := captureBridgeIO(t, input, func() {
		bridgeStdioToHTTPFast("http://127.0.0.1:1/mcp", state, 7890)
	})
	responses := parseJSONLines(t, output)
	if len(responses) != 2 {
		t.Fatalf("response count = %d, want 2", len(responses))
	}
	if responses[0].Error == nil || responses[0].Error.Code != -32002 {
		t.Fatalf("response[0] error = %+v, want -32002", responses[0].Error)
	}
	if responses[1].Error == nil || responses[1].Error.Code != -32602 {
		t.Fatalf("response[1] error = %+v, want -32602", responses[1].Error)
	}

	success, failure := snapshotFastPathResourceReadCounters()
	if success != 0 || failure != 2 {
		t.Fatalf("fast-path resources/read counters = (%d,%d), want (0,2)", success, failure)
	}
}

func TestBridgeFastPathResourcesReadTelemetryPersistsToStateLogs(t *testing.T) {
	// Do not run in parallel; test redirects process stdio and env.
	resetFastPathResourceReadCounters()
	t.Setenv(statecfg.StateDirEnv, t.TempDir())

	state := &daemonState{readyCh: make(chan struct{}), failedCh: make(chan struct{})}
	input := strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"resources/read","params":{"uri":"gasoline://capabilities"}}`,
		`{"jsonrpc":"2.0","id":2,"method":"resources/read","params":{"uri":"gasoline://playbook/nonexistent/quick"}}`,
	}, "\n") + "\n"

	_ = captureBridgeIO(t, input, func() {
		bridgeStdioToHTTPFast("http://127.0.0.1:1/mcp", state, 7890)
	})

	path, err := fastPathResourceReadLogPath()
	if err != nil {
		t.Fatalf("fastPathResourceReadLogPath() error = %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("os.ReadFile(%q) error = %v", path, err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 2 {
		t.Fatalf("telemetry log lines = %d, want 2; log=%q", len(lines), string(data))
	}

	var first map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &first); err != nil {
		t.Fatalf("first telemetry line parse error = %v", err)
	}
	if first["event"] != "bridge_fastpath_resources_read" {
		t.Fatalf("first telemetry event = %v, want bridge_fastpath_resources_read", first["event"])
	}
	if first["success"] != true {
		t.Fatalf("first telemetry success = %v, want true", first["success"])
	}

	var second map[string]any
	if err := json.Unmarshal([]byte(lines[1]), &second); err != nil {
		t.Fatalf("second telemetry line parse error = %v", err)
	}
	if second["success"] != false {
		t.Fatalf("second telemetry success = %v, want false", second["success"])
	}
	if code, ok := second["error_code"].(float64); !ok || int(code) != -32002 {
		t.Fatalf("second telemetry error_code = %v, want -32002", second["error_code"])
	}
}

func TestBridgeFastPathResourcesReadTelemetry(t *testing.T) {
	// Do not run in parallel; test redirects process stdio and uses Setenv.
	t.Setenv(statecfg.StateDirEnv, t.TempDir())
	resetFastPathCounters()

	state := &daemonState{readyCh: make(chan struct{}), failedCh: make(chan struct{})}
	input := strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`,
		`{"jsonrpc":"2.0","id":2,"method":"resources/read","params":{"uri":"gasoline://capabilities"}}`,
		`{"jsonrpc":"2.0","id":3,"method":"resources/read","params":{"uri":"gasoline://playbook/not-a-thing/quick"}}`,
	}, "\n") + "\n"

	output := captureBridgeIO(t, input, func() {
		bridgeStdioToHTTPFast("http://127.0.0.1:1/mcp", state, 7890)
	})

	responses := parseJSONLines(t, output)
	if len(responses) != 3 {
		t.Fatalf("response count = %d, want 3", len(responses))
	}
	if responses[1].Error != nil {
		t.Fatalf("resources/read success response has protocol error: %+v", responses[1].Error)
	}
	if responses[2].Error == nil || responses[2].Error.Code != -32002 {
		t.Fatalf("resources/read miss response = %+v, want -32002", responses[2])
	}

	path, err := fastPathTelemetryLogPath()
	if err != nil {
		t.Fatalf("fastPathTelemetryLogPath error = %v", err)
	}
	summary := summarizeFastPathTelemetryLog(path, 100)
	if summary.total < 3 {
		t.Fatalf("telemetry total = %d, want >= 3", summary.total)
	}
	if summary.methods["resources/read"] < 2 {
		t.Fatalf("resources/read telemetry count = %d, want >= 2", summary.methods["resources/read"])
	}
	if summary.failure < 1 {
		t.Fatalf("failure count = %d, want >= 1", summary.failure)
	}
	if summary.errorCodes[-32002] < 1 {
		t.Fatalf("error code -32002 count = %d, want >= 1", summary.errorCodes[-32002])
	}
}

func TestBridgeFastPathFramedInitializeAndToolsList(t *testing.T) {
	// Do not run in parallel; test redirects process stdio.
	state := &daemonState{readyCh: make(chan struct{}), failedCh: make(chan struct{})}
	initReq := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"codex","version":"1"}}}`
	toolsReq := `{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`
	input := contentLengthFrame(initReq) + contentLengthFrame(toolsReq)

	setStderrSink(io.Discard)
	stdout, stderr := captureBridgeIOWithStderr(t, input, func() {
		bridgeStdioToHTTPFast("http://127.0.0.1:1/mcp", state, 7890)
	})
	setStderrSink(os.Stderr)

	if strings.TrimSpace(stderr) != "" {
		t.Fatalf("framed startup emitted stderr output: %q", stderr)
	}
	if strings.Contains(stdout, `"code":-32700`) {
		t.Fatalf("framed startup emitted parse error: %q", stdout)
	}

	responses := parseJSONLines(t, stdout)
	if len(responses) != 2 {
		t.Fatalf("response count = %d, want 2", len(responses))
	}
	if responses[0].Error != nil {
		t.Fatalf("initialize returned protocol error: %+v", responses[0].Error)
	}
	if responses[1].Error != nil {
		t.Fatalf("tools/list returned protocol error: %+v", responses[1].Error)
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

func TestCheckDaemonStatus_HealsReadyFlagFromHealth(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"status":"ok","service-name":"gasoline","version":"1.0.0"}`)
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

	state := &daemonState{
		port:     port,
		ready:    false,
		failed:   true,
		err:      "stale failure",
		readyCh:  make(chan struct{}),
		failedCh: make(chan struct{}),
	}
	req := JSONRPCRequest{Method: "tools/call"}

	status := checkDaemonStatus(state, req, 7890)
	if status != "" {
		t.Fatalf("checkDaemonStatus() = %q, want empty status", status)
	}

	state.mu.Lock()
	defer state.mu.Unlock()
	if !state.ready {
		t.Fatal("expected state.ready to be healed to true")
	}
	if state.failed {
		t.Fatal("expected state.failed to be cleared")
	}
	if state.err != "" {
		t.Fatalf("expected state.err to be cleared, got %q", state.err)
	}
}

func TestRunningServerVersionCompatible(t *testing.T) {
	t.Parallel()

	oldVersion := version
	version = "9.9.9"
	t.Cleanup(func() { version = oldVersion })
	healthVersion := "9.9.9"

	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"status":"ok","service-name":"gasoline","version":"`+healthVersion+`"}`)
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

	compatible, gotVersion, gotService := runningServerVersionCompatible(port)
	if !compatible || gotVersion != "9.9.9" || gotService != "gasoline" {
		t.Fatalf("runningServerVersionCompatible() = (%v, %q, %q), want (true, %q, %q)", compatible, gotVersion, gotService, "9.9.9", "gasoline")
	}

	healthVersion = "1.0.0"
	compatible, gotVersion, gotService = runningServerVersionCompatible(port)
	if compatible || gotVersion != "1.0.0" || gotService != "gasoline" {
		t.Fatalf("runningServerVersionCompatible() = (%v, %q, %q), want (false, %q, %q)", compatible, gotVersion, gotService, "1.0.0", "gasoline")
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
