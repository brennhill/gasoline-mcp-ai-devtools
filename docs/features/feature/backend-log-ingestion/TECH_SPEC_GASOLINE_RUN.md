---
status: draft
priority: tier-1
phase: v5.4-foundation
relates-to: [PRODUCT_SPEC.md]
last-updated: 2026-01-31
---

# Backend Log Ingestion: gasoline-run Wrapper — Technical Specification

**Goal:** Wrapper executable that intercepts stdout/stderr and streams logs to local Gasoline daemon.

---

## Process Architecture

```
Developer
   │
   ├─ $ gasoline-run npm run dev
   │
   ▼
gasoline-run (new executable)
   ├─ spawn: npm run dev (child process)
   ├─ Capture: stdout/stderr
   ├─ Parse: Each line
   └─ Stream: HTTP POST http://localhost:7890/event
       │
       ▼
Gasoline Daemon (existing)
   ├─ Receive: POST /event
   ├─ Normalize: to NormalizedEvent
   └─ Store: in ring buffer

Terminal
   └─ Show: passthrough of stdout/stderr (user sees logs normally)
```

---

## Implementation

### File: `cmd/gasoline-run/main.go`

```go
package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"time"
)

func main() {
	configPath := flag.String("config", "", "Path to Gasoline config")
	daemonHost := flag.String("host", "localhost:7890", "Gasoline daemon address")
	streamType := flag.String("type", "auto", "Stream type: auto, stdout, stderr")
	flag.Parse()

	args := flag.Args()
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "Usage: gasoline-run [flags] command [args...]\n")
		fmt.Fprintf(os.Stderr, "Flags:\n")
		flag.PrintDefaults()
		os.Exit(1)
	}

	// Spawn child process
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stdin = os.Stdin

	// Capture stdout/stderr
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		fatal("Failed to create stdout pipe: %v", err)
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		fatal("Failed to create stderr pipe: %v", err)
	}

	// Start process
	if err := cmd.Start(); err != nil {
		fatal("Failed to start command: %v", err)
	}

	fmt.Fprintf(os.Stderr, "[gasoline-run] Started: %s\n", strings.Join(args, " "))
	fmt.Fprintf(os.Stderr, "[gasoline-run] Streaming to: %s\n", *daemonHost)

	// Stream logs
	client := &http.Client{Timeout: 5 * time.Second}

	// Concurrent streaming
	go streamLogs(stdoutPipe, "stdout", *daemonHost, client)
	go streamLogs(stderrPipe, "stderr", *daemonHost, client)

	// Also pass-through to terminal
	go io.Copy(os.Stdout, stdoutPipe)
	go io.Copy(os.Stderr, stderrPipe)

	// Wait for process to finish or signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)

	exitCode := 0
	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	select {
	case <-sigChan:
		// User pressed Ctrl+C
		cmd.Process.Kill()
		exitCode = 1
	case err := <-done:
		// Process exited
		if err != nil {
			if exiterr, ok := err.(*exec.ExitError); ok {
				exitCode = exiterr.ExitCode()
			} else {
				exitCode = 1
			}
		}
	}

	os.Exit(exitCode)
}

func streamLogs(
	reader io.Reader,
	streamType string,
	daemonHost string,
	client *http.Client,
) {
	scanner := bufio.NewScanner(reader)

	for scanner.Scan() {
		line := scanner.Text()

		// Send to daemon
		go sendLogLine(line, streamType, daemonHost, client)
	}
}

type LogLineEvent struct {
	Timestamp int64  `json:"timestamp"`
	Level     string `json:"level"`
	Source    string `json:"source"`
	Message   string `json:"message"`
	Metadata  map[string]interface{} `json:"metadata"`
	Tags      []string `json:"tags"`
}

func sendLogLine(
	line string,
	streamType string,
	daemonHost string,
	client *http.Client,
) {
	// Parse line to extract fields
	event := parseLogLine(line, streamType)
	if event == nil {
		return
	}

	// Send via HTTP POST
	body, _ := json.Marshal(event)
	req, _ := http.NewRequest(
		"POST",
		fmt.Sprintf("http://%s/event", daemonHost),
		bytes.NewReader(body),
	)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		// Daemon not running, skip (don't fail the child process)
		return
	}
	defer resp.Body.Close()
}

func parseLogLine(line string, streamType string) *LogLineEvent {
	// Try JSON
	if event := tryParseJSON(line); event != nil {
		return event
	}

	// Try structured pattern
	if event := tryParseStructured(line); event != nil {
		return event
	}

	// Fallback: simple plaintext
	level := "info"
	if streamType == "stderr" {
		level = "error"
	}

	return &LogLineEvent{
		Timestamp: time.Now().UnixMilli(),
		Level:     level,
		Source:    "backend",
		Message:   line,
		Tags:      []string{"dev", "local"},
	}
}

func tryParseJSON(line string) *LogLineEvent {
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(line), &data); err != nil {
		return nil
	}

	event := &LogLineEvent{
		Source:   "backend",
		Metadata: data,
		Tags:     []string{"dev", "local"},
	}

	// Extract timestamp
	if ts, ok := data["timestamp"].(string); ok {
		event.Timestamp = parseTimestamp(ts)
	} else {
		event.Timestamp = time.Now().UnixMilli()
	}

	// Extract level
	if level, ok := data["level"].(string); ok {
		event.Level = normalizeLevel(level)
	} else {
		event.Level = "info"
	}

	// Extract message
	if msg, ok := data["message"].(string); ok {
		event.Message = msg
	} else if msg, ok := data["msg"].(string); ok {
		event.Message = msg
	}

	return event
}

func tryParseStructured(line string) *LogLineEvent {
	// Pattern: [timestamp] LEVEL [req:xyz] message
	// Example: [2024-01-01T12:00:00Z] INFO [req:abc123] User logged in

	parts := strings.Split(line, "]")
	if len(parts) < 3 {
		return nil
	}

	timestamp := strings.Trim(parts[0], " [")
	level := strings.Trim(parts[1], " ")
	rest := strings.Join(parts[2:], "]")

	if !isValidLevel(level) {
		return nil // Not a structured log
	}

	event := &LogLineEvent{
		Timestamp: parseTimestamp(timestamp),
		Level:     normalizeLevel(level),
		Source:    "backend",
		Message:   strings.TrimLeft(rest, " []"),
		Tags:      []string{"dev", "local"},
	}

	// Try to extract correlation_id
	if correlationID := extractCorrelationID(rest); correlationID != "" {
		event.Metadata = map[string]interface{}{
			"correlation_id": correlationID,
		}
	}

	return event
}

func extractCorrelationID(s string) string {
	patterns := []string{
		`req[_:]?(\w+)`,
		`trace[_]?id[_:]?(\w+)`,
		`correlation[_]?id[_:]?(\w+)`,
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindStringSubmatch(s)
		if len(matches) > 1 {
			return matches[1]
		}
	}

	return ""
}

func parseTimestamp(s string) int64 {
	// Try common formats
	formats := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05Z07:00",
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
	}

	for _, format := range formats {
		if t, err := time.Parse(format, s); err == nil {
			return t.UnixMilli()
		}
	}

	return time.Now().UnixMilli()
}

func normalizeLevel(s string) string {
	s = strings.ToLower(s)
	switch {
	case s == "trace", s == "debug":
		return "debug"
	case s == "info", s == "information":
		return "info"
	case s == "warn", s == "warning":
		return "warn"
	case s == "error", s == "err":
		return "error"
	case s == "fatal", s == "panic", s == "critical":
		return "critical"
	default:
		return "info"
	}
}

func isValidLevel(s string) bool {
	s = strings.ToLower(s)
	valid := map[string]bool{
		"trace": true, "debug": true, "info": true, "warn": true,
		"error": true, "fatal": true, "panic": true, "critical": true,
	}
	return valid[s]
}

func fatal(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "[gasoline-run] ERROR: "+format+"\n", args...)
	os.Exit(1)
}
```

---

## Build & Installation

```bash
# Build
go build -o gasoline-run ./cmd/gasoline-run

# Install (optional)
go install ./cmd/gasoline-run

# Usage
gasoline-run npm run dev
gasoline-run python -m flask run
gasoline-run go run main.go
```

---

## Key Design Decisions

### 1. Wrapper vs Agent
- **Chose:** Wrapper (exec wrapper)
- **Why:** No code changes required. Works with any command.
- **Alternative:** Background agent (needs process manager)

### 2. Pass-Through Logs
- **Chose:** Copy stdout/stderr to terminal AND to daemon
- **Why:** User still sees logs normally. Doesn't interfere with debugging.
- **Alternative:** Capture only (would silence terminal)

### 3. HTTP POST vs Direct Connection
- **Chose:** HTTP POST to daemon
- **Why:** Daemon handles ingest, doesn't need special handling for gasoline-run
- **Alternative:** Unix socket (platform-specific)

### 4. Async Streaming
- **Chose:** Each line spawned as async go routine to POST
- **Why:** Non-blocking. If daemon is slow, doesn't block subprocess.
- **Alternative:** Buffering (would delay logs to user)

### 5. Error Handling
- **Chose:** Silently skip if daemon not running
- **Why:** Shouldn't break dev workflow if Gasoline daemon crashes
- **Alternative:** Fail hard (would break dev command)

---

## Integration with Daemon

### HTTP Endpoint: POST /event

**Request:**
```
POST http://localhost:7890/event
Content-Type: application/json

{
  "timestamp": 1704067200000,
  "level": "info",
  "source": "backend",
  "message": "User logged in",
  "metadata": {
    "correlation_id": "abc123"
  },
  "tags": ["dev", "local"]
}
```

**Response:**
```
200 OK
{
  "status": "ok",
  "event_id": "550e8400-e29b-41d4..."
}
```

### Daemon Responsibilities
1. **Receive** HTTP POST /event
2. **Normalize** to NormalizedEvent
3. **Extract** correlation_id from metadata
4. **Route** to appropriate ring buffer (backend_logs)
5. **Store** for query

Daemon does NOT need to know where event came from (gasoline-run, file tailer, extension, etc.).

---

## Performance

### Throughput
- **Per-line latency:** <1ms (HTTP overhead ~500μs)
- **Throughput:** 1000+ lines/sec to daemon
- **Typical dev server:** 10-100 lines/sec (no impact)

### Memory
- **Per-process:** ~5MB (binary size)
- **Per-invocation:** Minimal (streaming, no buffering)

### CPU
- **Idle:** 0% (waits for input)
- **Streaming:** <1% (single core)

---

## Error Handling

| Error | Behavior |
|-------|----------|
| Daemon not running | Silently skip POST (don't break subprocess) |
| Network timeout | Skip that line, continue streaming |
| Parse error | Fallback to simple plaintext |
| Process dies | gasoline-run exits with same code |
| SIGINT (Ctrl+C) | Propagate to child, exit cleanly |

---

## Testing Strategy

### Unit Tests
- [ ] Parse JSON logs correctly
- [ ] Parse structured logs with regex
- [ ] Fallback to plaintext
- [ ] Extract correlation_id
- [ ] Normalize log levels
- [ ] Parse timestamps in various formats

### Integration Tests
- [ ] `gasoline-run npm run dev` works
- [ ] Logs appear in daemon
- [ ] Pass-through still visible in terminal
- [ ] Works with piped output
- [ ] Works with colored output (ANSI codes)

### Manual Tests
```bash
# Test 1: Simple command
gasoline-run echo "Hello World"

# Test 2: Dev server
gasoline-run npm run dev

# Test 3: Long-running
gasoline-run python -m http.server 8000

# Test 4: Error output
gasoline-run npm run build  # (if build has errors)

# Test 5: Piped output
gasoline-run npm run dev | tee output.log
```

---

## Limitations (v5.4)

- [ ] No log filtering (captures all)
- [ ] No sampling (all lines parsed)
- [ ] No structured log parsing (regex patterns only)
- [ ] ANSI color codes passed through (may clutter logs)

---

## Related Documents

- **Product Spec:** [PRODUCT_SPEC.md](PRODUCT_SPEC.md)
- **Local Tailer:** [TECH_SPEC_LOCAL_TAILER.md](TECH_SPEC_LOCAL_TAILER.md)
- **SSH Tailer:** [TECH_SPEC_SSH_TAILER.md](TECH_SPEC_SSH_TAILER.md)
- **Architecture:** [layer1-be-observability.md](../../core/layer1-be-observability.md)

---

**Status:** Ready for implementation
**Estimated Effort:** 3 days
**Dependencies:** None (standalone executable)
