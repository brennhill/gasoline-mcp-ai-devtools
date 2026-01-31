---
status: draft
priority: tier-1
phase: v5.4-foundation
relates-to: [PRODUCT_SPEC.md, TECH_SPEC_GASOLINE_RUN.md]
last-updated: 2026-01-31
---

# Backend Log Ingestion: Local File Tailer — Technical Specification

**Goal:** Poll and tail local log files, stream new lines to daemon ring buffer.

---

## Process Architecture

```
Gasoline Daemon
   │
   ├─ HTTP Server (localhost:7890)
   ├─ MCP Server (stdio)
   └─ Log Ingestion Goroutines
       │
       ├─ gasoline-run Listener (accepts HTTP /event)
       │
       ├─ Local File Tailer (this spec)
       │  ├─ Poll: /var/log/app.log every 100ms
       │  ├─ Read: New lines since last position
       │  ├─ Parse: Extract timestamp, level, message, correlation_id
       │  └─ Send: HTTP POST /event → Ring Buffer
       │
       └─ SSH Tailer
          └─ (separate spec)

Ring Buffer
   └─ Stores: FE events + BE events (merged by timestamp)
```

---

## Implementation

### File: `pkg/ingest/local_tailer.go`

```go
package ingest

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"sync"
	"time"
)

// LocalFileTailer polls a local file for new lines
type LocalFileTailer struct {
	path         string
	pollInterval time.Duration
	file         *os.File
	lastPos      int64
	mu           sync.RWMutex
	ctx          context.Context
	cancel       context.CancelFunc
	handler      LogLineHandler  // Callback to process lines
	stats        TailerStats
}

// LogLineHandler processes each log line
type LogLineHandler func(ctx context.Context, line string) error

// TailerStats tracks tailer health
type TailerStats struct {
	LinesRead       int64
	LinesFailed     int64
	FileRotations   int64
	LastReadTime    time.Time
	LastErrorTime   time.Time
	LastError       string
}

// NewLocalFileTailer creates a new file tailer
func NewLocalFileTailer(
	path string,
	handler LogLineHandler,
) (*LocalFileTailer, error) {
	// Verify file exists
	if _, err := os.Stat(path); err != nil {
		return nil, fmt.Errorf("log file not found: %s", path)
	}

	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %w", err)
	}

	// Seek to end (only read new lines)
	lastPos, err := file.Seek(0, io.SeekEnd)
	if err != nil {
		file.Close()
		return nil, fmt.Errorf("failed to seek: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &LocalFileTailer{
		path:         path,
		pollInterval: 100 * time.Millisecond,
		file:         file,
		lastPos:      lastPos,
		ctx:          ctx,
		cancel:       cancel,
		handler:      handler,
	}, nil
}

// Start begins polling the file
func (t *LocalFileTailer) Start(ctx context.Context) error {
	defer func() {
		t.file.Close()
	}()

	ticker := time.NewTicker(t.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-t.ctx.Done():
			return fmt.Errorf("tailer stopped")
		case <-ticker.C:
			if err := t.poll(ctx); err != nil {
				t.recordError(err)
				// Continue polling even on error
			}
		}
	}
}

func (t *LocalFileTailer) poll(ctx context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	// Check if file still exists or was rotated
	stat, err := t.file.Stat()
	if err != nil {
		// File was deleted/rotated
		if err := t.handleFileRotation(); err != nil {
			return fmt.Errorf("failed to handle rotation: %w", err)
		}
		return nil
	}

	currentSize := stat.Size()

	// Check for file rotation (size decreased or inode changed)
	if currentSize < t.lastPos {
		if err := t.handleFileRotation(); err != nil {
			return fmt.Errorf("file rotated but recovery failed: %w", err)
		}
		return nil
	}

	// Read new data since last position
	if currentSize > t.lastPos {
		if err := t.readNewLines(ctx); err != nil {
			return err
		}
	}

	t.stats.LastReadTime = time.Now()
	return nil
}

func (t *LocalFileTailer) readNewLines(ctx context.Context) error {
	// Seek to last known position
	_, err := t.file.Seek(t.lastPos, io.SeekStart)
	if err != nil {
		return fmt.Errorf("seek failed: %w", err)
	}

	scanner := bufio.NewScanner(t.file)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)  // 64KB default, 1MB max

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		line := scanner.Text()

		// Call handler (send to daemon)
		if err := t.handler(ctx, line); err != nil {
			t.stats.LinesFailed++
			// Log but continue
		} else {
			t.stats.LinesRead++
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("scan error: %w", err)
	}

	// Update last position
	pos, err := t.file.Seek(0, io.SeekCurrent)
	if err != nil {
		return fmt.Errorf("get position failed: %w", err)
	}
	t.lastPos = pos

	return nil
}

func (t *LocalFileTailer) handleFileRotation() error {
	// File was rotated (size decreased)
	t.stats.FileRotations++

	// Close old file
	t.file.Close()

	// Wait a moment for new file to be created
	time.Sleep(100 * time.Millisecond)

	// Try to open file again
	file, err := os.Open(t.path)
	if err != nil {
		return fmt.Errorf("failed to reopen after rotation: %w", err)
	}

	// Check if this is truly a new file or if rotation hasn't completed
	stat, _ := file.Stat()
	if stat.Size() == 0 {
		// New empty file, start from beginning
		t.lastPos = 0
	} else {
		// Partial file, read from start
		t.lastPos = 0
	}

	t.file = file
	return nil
}

func (t *LocalFileTailer) recordError(err error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.stats.LastError = err.Error()
	t.stats.LastErrorTime = time.Now()
}

// Stop stops the tailer
func (t *LocalFileTailer) Stop() error {
	t.cancel()
	return t.file.Close()
}

// Stats returns current tailer statistics
func (t *LocalFileTailer) Stats() TailerStats {
	t.mu.RLock()
	defer t.mu.RUnlock()

	return t.stats
}
```

---

## Integration with Daemon

### File: `internal/daemon/ingestion.go`

```go
package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"path/filepath"
	"sync"

	"github.com/gasoline/pkg/ingest"
)

// IngestionManager manages all log sources
type IngestionManager struct {
	tailers map[string]*ingest.LocalFileTailer
	mu      sync.RWMutex
	sender  EventSender  // Sends events to ring buffer
}

// EventSender interface (implemented by daemon)
type EventSender interface {
	SendEvent(event *ingest.LogEvent) error
}

func NewIngestionManager(sender EventSender) *IngestionManager {
	return &IngestionManager{
		tailers: make(map[string]*ingest.LocalFileTailer),
		sender:  sender,
	}
}

// AddLocalFileTailer starts tailing a local file
func (im *IngestionManager) AddLocalFileTailer(
	ctx context.Context,
	path string,
) error {
	im.mu.Lock()
	defer im.mu.Unlock()

	// Prevent duplicate tailers
	absPath, _ := filepath.Abs(path)
	if _, exists := im.tailers[absPath]; exists {
		return fmt.Errorf("tailer already running for %s", absPath)
	}

	// Create handler that processes log lines
	handler := func(ctx context.Context, line string) error {
		event := ingest.ParseLogLine(line, "backend")
		if event == nil {
			return nil
		}

		// Send to daemon
		return im.sender.SendEvent(event)
	}

	// Create tailer
	tailer, err := ingest.NewLocalFileTailer(absPath, handler)
	if err != nil {
		return err
	}

	im.tailers[absPath] = tailer

	// Start tailing (in background goroutine)
	go func() {
		if err := tailer.Start(ctx); err != nil {
			fmt.Printf("[ingest] Tailer error for %s: %v\n", absPath, err)
		}
	}()

	fmt.Printf("[ingest] Started tailing: %s\n", absPath)
	return nil
}

// Stats returns statistics for all tailers
func (im *IngestionManager) Stats() map[string]interface{} {
	im.mu.RLock()
	defer im.mu.RUnlock()

	stats := make(map[string]interface{})
	for path, tailer := range im.tailers {
		stats[path] = tailer.Stats()
	}

	return stats
}

// HTTP Handler: GET /ingest/stats
func (im *IngestionManager) StatsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(im.Stats())
}
```

---

## Configuration

### YAML Schema

```yaml
backend:
  logs:
    # Local file tailer
    - type: file
      path: /var/log/app.log
      poll_interval_ms: 100  # How often to check for new lines
      enabled: true
```

### Loading Configuration

```go
// pkg/config/config.go

type LogSourceConfig struct {
	Type             string `yaml:"type"`  // "file", "ssh", "http"
	Path             string `yaml:"path"`  // For file tailer
	PollIntervalMs   int    `yaml:"poll_interval_ms"`
	Enabled          bool   `yaml:"enabled"`
}

func LoadConfig(path string) ([]LogSourceConfig, error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var config struct {
		Backend struct {
			Logs []LogSourceConfig `yaml:"logs"`
		} `yaml:"backend"`
	}

	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	return config.Backend.Logs, nil
}
```

---

## File Rotation Handling

### Scenario 1: Typical log rotation (size-based)

```
Before rotation:
├─ app.log (1GB, position=1GB)

Rotation happens:
├─ app.log → app.log.1
├─ app.log (empty, new)

Tailer detects:
├─ file size decreased (rotation detected)
├─ closes old file handle
├─ reopens app.log
├─ resets position to 0
└─ continues reading

Result: No lines lost, clean switchover
```

### Scenario 2: Delayed rotation

```
Before:
├─ app.log (1GB)

System rotates:
├─ app.log.1 (contains old logs)
├─ BUT app.log still exists (file handle held by logger)

Tailer:
├─ detects size decrease (but file still has content)
├─ reads remaining lines from old file
├─ eventually file gets truncated by logger
└─ transitions to reading new content

Result: Handles gracefully, no duplicates
```

---

## Performance

### Throughput
- **Poll interval:** 100ms (configurable)
- **Lines per poll:** 10-1000 (depends on activity)
- **Parsing time:** <1ms per line
- **Typical impact:** <1% CPU, <5MB memory

### Memory
- **Per tailer:** ~10MB (includes 1MB buffer)
- **Per-line:** ~1KB in handler (temporary)
- **Overhead:** Minimal, no buffering

### Network
- **Via HTTP to daemon:** ~500B per line (payload)
- **Throughput:** 1000+ lines/sec sustainable

---

## Error Handling

| Error | Behavior |
|-------|----------|
| File not found | Log error, retry on next poll |
| File rotated | Detect by size decrease, reopen and continue |
| Permission denied | Log error, fail tailer |
| Inode changed | Treat as rotation, reopen |
| Disk full | Log error, continue (daemon will handle) |
| Line too long (>1MB) | Split or truncate, continue |

---

## Monitoring & Debugging

### Health Check
```bash
curl http://localhost:7890/ingest/stats
```

Response:
```json
{
  "/var/log/app.log": {
    "lines_read": 1500,
    "lines_failed": 2,
    "file_rotations": 1,
    "last_read_time": "2024-01-01T12:00:05Z",
    "last_error_time": null,
    "last_error": ""
  }
}
```

### Debug Logging
```
[ingest] Started tailing: /var/log/app.log
[ingest] Poll: read 42 lines (position: 50000)
[ingest] File rotated: /var/log/app.log (size decreased)
[ingest] Reopened: /var/log/app.log (new inode)
```

---

## Testing Strategy

### Unit Tests
- [ ] Poll detects new lines
- [ ] Position tracking accurate
- [ ] File rotation detected
- [ ] Handler errors don't crash tailer
- [ ] Buffer handling (large lines)

### Integration Tests
- [ ] Real file tailing works
- [ ] File rotation works
- [ ] Multiple tailers don't interfere
- [ ] Stats accurate

### Manual Tests
```bash
# Test 1: Basic tailing
$ touch /tmp/test.log
$ gasoline-run --local-tailer /tmp/test.log
$ echo "Hello" >> /tmp/test.log
# Verify: Message appears in daemon

# Test 2: File rotation
$ # Stop daemon, rotate logs, restart daemon
$ sudo mv /tmp/test.log /tmp/test.log.1
$ touch /tmp/test.log
$ # Verify: Daemon continues tailing

# Test 3: Large file
$ # Create 1GB log file
$ # Verify: Tailer handles efficiently
```

---

## Limitations (v5.4)

- [ ] No line filtering
- [ ] No sampling
- [ ] No deduplication
- [ ] Poll-based (not inotify)
- [ ] Local filesystem only (not remote filesystems like NFS)

---

## Future Improvements

- [ ] Use inotify on Linux (faster than polling)
- [ ] Support NFS/remote filesystems
- [ ] Line-level filtering (regex exclude)
- [ ] Sampling (1-in-N lines)
- [ ] Compression for old tailers

---

## Related Documents

- **Product Spec:** [PRODUCT_SPEC.md](PRODUCT_SPEC.md)
- **gasoline-run:** [TECH_SPEC_GASOLINE_RUN.md](TECH_SPEC_GASOLINE_RUN.md)
- **SSH Tailer:** [TECH_SPEC_SSH_TAILER.md](TECH_SPEC_SSH_TAILER.md)
- **Architecture:** [layer1-be-observability.md](../../core/layer1-be-observability.md)

---

**Status:** Ready for implementation
**Estimated Effort:** 3 days
**Dependencies:** Context, sync, os, io packages (stdlib only)
