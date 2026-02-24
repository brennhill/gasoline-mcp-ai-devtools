---
status: draft
scope: layer-1-backend-observability
relates-to: [target-architecture.md, 360-observability-architecture.md]
last-updated: 2026-01-31
last_reviewed: 2026-02-16
---

# Layer 1: Backend Observability

**Goal:** Capture backend logs (local dev and production) and merge with FE telemetry for unified request tracing.

---

## Philosophy

Backend services already log to stdout. We don't ask developers to change their code. We intercept:

1. **Local dev:** Wrap the dev command (`gasoline-run npm run dev`)
2. **Production:** Tail the log file or fetch via SSH
3. **Logging format:** Any format (plaintext, JSON, custom regex)
4. **Request correlation:** Extract request ID from log to link FE + BE

---

## Architecture

```
FE: User clicks button (trace_id = abc123)
    ├─ Extension captures: timestamp, action, trace_id
    └─ Sends to: localhost:7890 via HTTPS

BE: Service receives request
    ├─ Logs to stdout: "[timestamp] INFO [req:abc123] Processing"
    ├─ Gasoline-run captures: stdout → parses → normalizes
    └─ Sends to: localhost:7890 via HTTP

Daemon (localhost:7890)
    ├─ Ring Buffer:
    │  ├─ FE event (trace_id=abc123)
    │  └─ BE event (correlation_id=abc123)
    └─ Query: Show all events for abc123
       ├─ FE: User clicked button (12:00:00)
       ├─ BE: Received request (12:00:00.002)
       ├─ BE: DB query (12:00:00.050)
       └─ BE: Response sent (12:00:00.320)
```

---

## Implementation: gasoline-run (Local Dev)

**File:** `cmd/gasoline-run/main.go`

### Usage

```bash
# Instead of: npm run dev
# Use:        gasoline-run npm run dev

gasoline-run npm run dev
gasoline-run python -m flask run
gasoline-run go run main.go
gasoline-run ruby rails server
```

### How It Works

```go
package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"time"
)

func main() {
	// Parse: gasoline-run [--config ...] [command] [args...]
	args := os.Args[1:]
	configFile := ""

	if len(args) > 0 && args[0] == "--config" {
		configFile = args[1]
		args = args[2:]
	}

	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "Usage: gasoline-run [--config file] command [args...]\n")
		os.Exit(1)
	}

	// Start daemon if not running
	daemon := NewDaemonClient("localhost:7890")
	daemon.EnsureRunning()

	// Spawn child process
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stdin = os.Stdin

	// Capture stdout/stderr
	stdout, _ := cmd.StdoutPipe()
	stderr, _ := cmd.StderrPipe()

	// Start process
	if err := cmd.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to start: %v\n", err)
		os.Exit(1)
	}

	// Stream stdout to both: user terminal + daemon
	go streamLogs(stdout, "stdout", daemon)
	go streamLogs(stderr, "stderr", daemon)

	// Pass-through to user's terminal
	go io.Copy(os.Stdout, stdout)
	go io.Copy(os.Stderr, stderr)

	// Wait for process
	if err := cmd.Wait(); err != nil {
		os.Exit(1)
	}
}

func streamLogs(pipe io.Reader, streamType string, daemon *DaemonClient) {
	scanner := bufio.NewScanner(pipe)

	for scanner.Scan() {
		line := scanner.Text()

		// Send to daemon
		event := ParseLogLine(line, streamType)
		if event != nil {
			daemon.SendEvent(event)
		}
	}
}
```

### Log Line Parsing

```go
// pkg/ingest/parser.go

type LogEvent struct {
	Timestamp     int64
	Level         string  // debug, info, warn, error
	Message       string
	CorrelationID string  // req_123, trace_id, etc.
	Logger        string  // which service/module
	Metadata      map[string]interface{}
}

func ParseLogLine(line string, streamType string) *LogEvent {
	// Try each parser in order

	// 1. JSON (most common in modern services)
	if event := parseJSON(line); event != nil {
		return event
	}

	// 2. Structured plaintext (common pattern)
	//    [2024-01-01T12:00:00.123Z] INFO [req:abc123] [auth] User logged in
	if event := parseStructured(line); event != nil {
		return event
	}

	// 3. Simple plaintext
	//    Processing user request
	if event := parseSimple(line, streamType); event != nil {
		return event
	}

	return nil
}

// parseJSON: Assumes JSON with standard fields
func parseJSON(line string) *LogEvent {
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(line), &data); err != nil {
		return nil // Not JSON
	}

	// Extract standard fields (varies by framework/library)
	event := &LogEvent{
		Timestamp: extractTimestamp(data),
		Level:     extractLevel(data),
		Message:   extractMessage(data),
		CorrelationID: extractCorrelationID(data),
		Logger:    extractLogger(data),
		Metadata:  data,
	}

	return event
}

// parseStructured: Pattern: [timestamp] LEVEL [req:xyz] [logger] message
func parseStructured(line string) *LogEvent {
	// Regex: ^\[(.*?)\]\s+(\w+)\s+\[(.*?)\]\s+\[(.*?)\]\s+(.*)$

	re := regexp.MustCompile(
		`^\[(.*?)\]\s+(\w+)\s+\[(.*?)\]\s+\[(.*?)\]\s+(.*)$`,
	)

	matches := re.FindStringSubmatch(line)
	if matches == nil {
		return nil
	}

	timestamp, _ := parseTimestamp(matches[1])
	level := normalizeLevel(matches[2])        // INFO → info
	context := matches[3]                       // req:abc123
	logger := matches[4]                        // auth
	message := matches[5]

	correlationID := extractCorrelationID(context)

	return &LogEvent{
		Timestamp:     timestamp,
		Level:         level,
		Message:       message,
		CorrelationID: correlationID,
		Logger:        logger,
	}
}

// parseSimple: Just a string
func parseSimple(line string, streamType string) *LogEvent {
	return &LogEvent{
		Timestamp: time.Now().UnixMilli(),
		Level:     inferLevel(line, streamType),
		Message:   line,
	}
}

// Helper: Extract correlation ID from various formats
func extractCorrelationID(s string) string {
	patterns := []string{
		`req[_:]?(\w+)`,
		`trace[_]?id[_:]?(\w+)`,
		`request[_]?id[_:]?(\w+)`,
		`correlation[_]?id[_:]?(\w+)`,
		`\[(\w{32})\]`, // UUID-like
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

// Helper: Normalize log level
func normalizeLevel(s string) string {
	lower := strings.ToLower(s)
	switch lower {
	case "trace", "debug":
		return "debug"
	case "info", "information":
		return "info"
	case "warn", "warning":
		return "warn"
	case "error", "err":
		return "error"
	case "fatal", "panic", "critical":
		return "critical"
	default:
		return "info"
	}
}
```

---

## Implementation: Remote Log Tailer (Production)

**File:** `pkg/ingest/tailer.go`

### Supported Backends

```go
type LogSource interface {
	// ReadNewLines returns lines added since last call
	ReadNewLines(ctx context.Context) ([]string, error)

	// Close closes the source
	Close() error
}

// Local file tailer
type LocalFileTailer struct {
	path     string
	file     *os.File
	lastPos  int64
	pollRate time.Duration
}

// SSH remote tailer
type SSHTailer struct {
	host     string
	user     string
	path     string
	client   *ssh.Client
	session  *ssh.Session
	pollRate time.Duration
}

// HTTP API tailer (future: for log aggregation APIs)
type APITailer struct {
	url      string
	apiKey   string
	filter   string
	pollRate time.Duration
}
```

### Local File Tailer

```go
// pkg/ingest/local_tailer.go

func NewLocalFileTailer(path string) (*LocalFileTailer, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	// Seek to end (only read new lines)
	lastPos, err := file.Seek(0, io.SeekEnd)
	if err != nil {
		return nil, err
	}

	return &LocalFileTailer{
		path:     path,
		file:     file,
		lastPos:  lastPos,
		pollRate: 100 * time.Millisecond,
	}, nil
}

func (t *LocalFileTailer) ReadNewLines(ctx context.Context) ([]string, error) {
	var lines []string

	for {
		select {
		case <-ctx.Done():
			return lines, ctx.Err()
		default:
		}

		// Check for new data
		stat, err := t.file.Stat()
		if err != nil {
			return lines, err
		}

		if stat.Size() > t.lastPos {
			// Read new data
			t.file.Seek(t.lastPos, io.SeekStart)
			scanner := bufio.NewScanner(t.file)

			for scanner.Scan() {
				lines = append(lines, scanner.Text())
			}

			t.lastPos = stat.Size()
		}

		if len(lines) > 0 {
			return lines, nil
		}

		// Poll again
		time.Sleep(t.pollRate)
	}
}

func (t *LocalFileTailer) Close() error {
	return t.file.Close()
}
```

### SSH Remote Tailer

```go
// pkg/ingest/ssh_tailer.go

func NewSSHTailer(host, user, path string) (*SSHTailer, error) {
	// Parse SSH key from ~/.ssh/id_rsa
	key, _ := ioutil.ReadFile(os.ExpandUser("~/.ssh/id_rsa"))
	signer, _ := ssh.ParsePrivateKey(key)

	config := &ssh.ClientConfig{
		User: user,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // ⚠️ For dev only
	}

	client, err := ssh.Dial("tcp", host+":22", config)
	if err != nil {
		return nil, err
	}

	return &SSHTailer{
		host:     host,
		user:     user,
		path:     path,
		client:   client,
		pollRate: 100 * time.Millisecond,
	}, nil
}

func (t *SSHTailer) ReadNewLines(ctx context.Context) ([]string, error) {
	// Execute: tail -F /path/to/log
	session, err := t.client.NewSession()
	if err != nil {
		return nil, err
	}
	defer session.Close()

	stdout, err := session.StdoutPipe()
	if err != nil {
		return nil, err
	}

	// Start tail command (blocks until data available)
	err = session.Start(fmt.Sprintf("tail -f %s", t.path))
	if err != nil {
		return nil, err
	}

	var lines []string
	scanner := bufio.NewScanner(stdout)

	// Read available lines (with timeout)
	ctx2, cancel := context.WithTimeout(ctx, 1*time.Second)
	defer cancel()

	for scanner.Scan() {
		select {
		case <-ctx2.Done():
			return lines, nil
		default:
			lines = append(lines, scanner.Text())
		}
	}

	return lines, nil
}

func (t *SSHTailer) Close() error {
	return t.client.Close()
}
```

---

## Configuration

**File:** `~/.gasoline/config.yaml`

### Local Dev (gasoline-run)

```yaml
# Auto-configured by gasoline-run
# No config needed, just use: gasoline-run npm run dev
```

### Production Layer 1

```yaml
backend:
  logs:
    # Option 1: Local file
    - type: file
      path: /var/log/app.log
      format: plaintext  # or: json, structured
      poll_interval_ms: 100

    # Option 2: Remote SSH
    - type: ssh
      host: prod.example.com
      user: ubuntu
      path: /var/log/app.log
      format: json
      poll_interval_ms: 500
      auth: ~/.ssh/id_rsa  # or: ~/.ssh/config

    # Option 3: Multiple services (Layer 2 preview)
    - type: ssh
      host: api-1.internal
      user: deploy
      path: /var/log/service.log
      format: structured
      correlation_id_field: request_id
```

### Log Format Hints

```yaml
backend:
  logs:
    - type: file
      path: /var/log/app.log
      format: plaintext

      # Help parser find key fields
      patterns:
        timestamp: '^\[(.*?)\]'
        level: '\[(INFO|WARN|ERROR)\]'
        correlation_id: '\[req:(\w+)\]'
        message: '\] (.*)'
```

---

## Merging FE + BE Events

### Before Merge (Ring Buffers)

```
Browser Buffer:
├─ timestamp=1704067200000, source=browser, message="Click add-to-cart", correlation_id=abc123
└─ timestamp=1704067200005, source=browser, message="POST /api/checkout 200 OK", correlation_id=abc123

Backend Buffer:
├─ timestamp=1704067200002, source=backend, message="Received POST /checkout", correlation_id=abc123
├─ timestamp=1704067200050, source=backend, message="DB query: INSERT order", correlation_id=abc123
└─ timestamp=1704067200100, source=backend, message="Response sent: 200", correlation_id=abc123
```

### After Merge (Query)

```
Query: GET /buffers/timeline?filter=correlation_id:abc123

Response (sorted by timestamp):
[
  {timestamp: 1704067200000, source: "browser", message: "Click add-to-cart"},
  {timestamp: 1704067200002, source: "backend", message: "Received POST /checkout"},
  {timestamp: 1704067200005, source: "browser", message: "POST /api/checkout 200 OK"},
  {timestamp: 1704067200050, source: "backend", message: "DB query: INSERT order"},
  {timestamp: 1704067200100, source: "backend", message: "Response sent: 200"},
]

AI sees: Complete request flow from click to response
```

---

## Correlation ID Strategy

### How It Works

1. **Browser generates trace ID** (UUID, v6.0)
   ```
   const traceID = crypto.randomUUID()  // e.g., "550e8400-e29b-41d4..."
   ```

2. **Extension sends with every request**
   ```
   POST /api/checkout
   Headers: X-Trace-ID: 550e8400-e29b-41d4...
   Body: {...}
   ```

3. **Backend receives trace ID**
   ```
   app.use((req, res, next) => {
     req.traceID = req.headers['x-trace-id']
     logger.info(`Request ${req.method} ${req.path}`, {
       trace_id: req.traceID,
     })
     next()
   })
   ```

4. **Logs include trace ID**
   ```
   [2024-01-01T12:00:00Z] INFO [trace:550e8400-e29b-41d4] Received checkout
   ```

5. **Parser extracts and stores**
   ```
   correlation_id: "550e8400-e29b-41d4"
   ```

6. **Query merges**
   ```
   All events with correlation_id="550e8400-e29b-41d4" → One timeline
   ```

### Fallback (If No Trace ID in Logs)

If backend doesn't log trace ID, match by timestamp + request path:

```
FE: timestamp=1704067200005, "POST /api/checkout", status=200
BE: timestamp=1704067200002-100, "POST /checkout"

Match: Same endpoint, overlapping timestamps → Likely same request
Confidence: Medium (could be concurrent requests)
```

---

## Local Dev Flow

```bash
$ gasoline-run npm run dev

[Gasoline] Starting daemon (localhost:7890)
[Gasoline] Listening for events
[App] Server running on http://localhost:3000

# User opens browser, clicks button
[App] [2024-01-01T12:00:00.000Z] INFO [req:abc123] POST /checkout received
[App] [2024-01-01T12:00:00.050Z] INFO [req:abc123] Database query: INSERT order
[App] [2024-01-01T12:00:00.100Z] INFO [req:abc123] Response sent

# AI queries: "Show me what happened"
# Query: GET http://localhost:7890/buffers/timeline

FE + BE events merged:
1. Click "Checkout" (FE, 12:00:00.000)
2. POST /api/checkout 200 (FE, 12:00:00.005)
3. Received POST /checkout (BE, 12:00:00.002)
4. Database query (BE, 12:00:00.050)
5. Response sent (BE, 12:00:00.100)
```

---

## Production Flow (Layer 1)

```bash
# Production server: normal operations
$ npm run start > /var/log/app.log 2>&1

# Developer's machine: configure Gasoline
$ cat ~/.gasoline/config.yaml
backend:
  logs:
    - type: ssh
      host: prod.example.com
      user: ubuntu
      path: /var/log/app.log

# Developer starts debugging on production
$ gasoline --config ~/.gasoline/config.yaml

[Gasoline] SSH connected to prod.example.com
[Gasoline] Tailing /var/log/app.log
[Gasoline] Daemon ready on http://localhost:7890

# Developer opens prod site in browser
# User clicks button, sees error
# Extension captures: FE events + error

# AI queries: "Why did checkout fail?"
# Daemon pulls recent logs from prod via SSH
# Correlates FE + BE by timestamp/request_id
# Shows full flow: FE → BE → error

Timeline:
1. Click "Checkout" (FE)
2. POST /checkout (FE)
3. Received POST /checkout (BE) ← from SSH tail
4. Database connection timeout (BE) ← from SSH tail
5. 500 error response (BE) ← from SSH tail
```

---

## Known Limitations (v6.0)

- [ ] Requires trace ID in logs (or timestamp matching fallback)
- [ ] SSH key auth only (no password auth)
- [ ] Logs parsed on best-effort basis (custom formats need regex config)
- [ ] No log filtering/sampling (all logs captured)
- [ ] No log aggregation platform integration yet (BigQuery, Datadog, etc. → v7.0)

---

## Related Documents

- **Target Architecture:** [target-architecture.md](target-architecture.md)
- **360 Observability Architecture:** [360-observability-architecture.md](360-observability-architecture.md)

---

**Status:** Architecture for Layer 1 BE observability
**Next:** Implementation specs for gasoline-run and log tailer
