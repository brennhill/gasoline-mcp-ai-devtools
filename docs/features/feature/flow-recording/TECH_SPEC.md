---
feature: Flow Recording & Playback
status: in-progress
version: v6.0
---

# Tech Spec: Flow Recording & Playback (Regression Testing)

## Overview

This spec defines the technical architecture for Flow Recording & Playback. The feature captures browser interactions, stores them as JSON action sequences, replays them to detect regressions, and provides root cause analysis via a Claude skill.

**Key Components:**
1. **Extension Recording** (TypeScript/JS) — Captures interactions, sends to server
2. **Server Storage** (Go) — Ring buffers + disk storage for recordings/screenshots
3. **Playback Engine** (Go + Extension) — Executes action sequences, captures logs
4. **Root Cause Analysis Skill** (Claude prompt) — Analyzes regression logs, suggests fixes

---

## Architecture

### 1. Recording Flow

```
Extension Records                Server Stores                LLM Queries
─────────────────              ─────────────                 ──────────

[Click event] → POST /query    Recording buffer         observe({what:
  {type: 'click',               (memory ring)            'recordings'})
   selector: '.btn',
   x: 500, y: 200}      ← Buffer N entries              [recording list]
                         ← Screenshot to disk
[Type event] → POST /query
  {type: 'type',
   text: 'alice@...'}   → GET /pending-queries    observe({what:
                         ← Return queued actions   'recording_actions',
[Navigate] → POST /query                          recording_id: 'X'})
  {type: 'navigate',
   url: 'https://...'}  ← Return action sequence
                                                  [action list in JSON]
```

### 2. Playback Flow

```
LLM Invokes                   Server Executes              Results
───────────────               ──────────────               ────────

interact({action:        start_boundary('replay-X')
  'playback',      →           ↓
  recording:         replay action sequence
  'checkout-X',        ├─ navigate → wait for load
  test_id:           ├─ click (find by selector)
  'replay-checkout'})  ├─ type (verify input)
                       ├─ screenshot (on error)
                       └─ log all events with test_id
                            ↓
                       end_boundary('replay-X')
                            ↓
                       {status, errors, duration}
                              ↓
                        observe({what: 'logs',
                         test_id: 'replay-X'})
                              ↓
                        [compare to original logs]
```

### 3. Root Cause Analysis Flow

```
LLM Detects Regression         Gasoline Analyzes            LLM Reviews
──────────────────────         ──────────────────           ──────────

observe({what: 'logs',    Diff logs:
  test_id: 'original'})    ├─ Extract errors
         ↓                 ├─ Identify types (404, timeout, etc.)
[logs A: clean]            ├─ Find git commits touched files
         ↓                 ├─ Analyze error patterns
observe({what: 'logs',     └─ Rank confidence (HIGH/MED/LOW)
  test_id: 'replay'})            ↓
         ↓                 /gasoline-fix response:
[logs B: 404 errors]        {root_cause, confidence,
         ↓                   suggested_fixes, related_commits,
/gasoline-fix ...            affected_files}
  original='logs A'               ↓
  replay='logs B'           LLM reviews & applies
  git_repo='/app'           → Implements fix
                           → Re-runs playback
                           → ✓ Verified
```

---

## Data Structures

### 1. Recording Storage

**In-Memory (Runtime):**
```go
type Recording struct {
    ID           string    // "shopping-checkout-20260130T143022Z"
    Name         string    // "shopping-checkout" (user-provided or auto-generated)
    CreatedAt    time.Time // RFC3339 timestamp
    Duration     int       // milliseconds
    ActionCount  int       // number of actions recorded
    StartURL     string    // initial page URL
    Actions      []Action  // action sequence (in memory during recording)
    Metadata     RecordingMeta
    Status       string    // "recording" | "complete" | "archived"
}

type RecordingMeta struct {
    ViewportWidth  int       // page width when recording started
    ViewportHeight int       // page height when recording started
    UserAgent      string    // browser user agent
    TabID          int       // which browser tab was recorded
    ScreenshotDir  string    // path to screenshots directory
}

type Action struct {
    Type      string    // "navigate" | "click" | "type" | "screenshot"
    Timestamp int64     // Unix milliseconds
    Selector  string    // CSS selector (empty for navigate)
    Text      string    // typed text (empty for click/navigate)
    X         int       // x coordinate
    Y         int       // y coordinate
    URL       string    // for navigate actions

    // Element matching (priority order when replaying)
    DataTestID string   // data-testid attribute value
    AriaLabel  string   // aria-label attribute value

    // Metadata
    ScreenshotPath string // relative path to screenshot
    ErrorMessage   string // if action failed during recording
}
```

**On Disk (Persistent):**

```
~/.gasoline/recordings/
├── shopping-checkout-20260130T143022Z/
│   ├── metadata.json          # Recording metadata + action list (JSON)
│   └── screenshots/
│       ├── 20260130-001-click.jpg
│       ├── 20260130-002-type.jpg
│       └── 20260130-003-error.jpg
└── [other recordings...]
```

**metadata.json Format:**
```json
{
  "id": "shopping-checkout-20260130T143022Z",
  "name": "shopping-checkout",
  "created_at": "2026-01-30T14:30:22Z",
  "duration_ms": 45000,
  "action_count": 8,
  "start_url": "https://example.com/shop",
  "viewport": {"width": 1920, "height": 1080},
  "actions": [
    {
      "type": "navigate",
      "timestamp_ms": 0,
      "url": "https://example.com/shop"
    },
    {
      "type": "click",
      "timestamp_ms": 1200,
      "selector": "[data-testid=product-card-1]",
      "data_testid": "product-card-1",
      "x": 500,
      "y": 300,
      "screenshot_path": "screenshots/20260130-001-click.jpg"
    },
    {
      "type": "type",
      "timestamp_ms": 3400,
      "selector": "input#quantity",
      "text": "3",
      "x": 523,
      "y": 456,
      "screenshot_path": "screenshots/20260130-002-type.jpg"
    }
  ]
}
```

### 2. Extension Recording Message

**POST /query body (from extension):**
```json
{
  "type": "recording_action",
  "action": {
    "type": "click",
    "selector": ".add-to-cart-btn",
    "data_testid": "add-to-cart",
    "x": 600,
    "y": 300,
    "timestamp_ms": 1647302200000
  }
}
```

### 3. Test Boundaries Integration

All recorded actions are tagged with `test_ids` for correlation:

```go
type EnhancedAction struct {
    // ... existing fields ...
    TestIDs []string // e.g., ["recording-shopping-checkout", "replay-shopping-checkout"]
}
```

**Usage:**
```javascript
// Record original flow
configure({action: 'test_boundary_start', test_id: 'original-checkout'})
// ... user performs flow ...
configure({action: 'test_boundary_end', test_id: 'original-checkout'})

// Replay same flow
interact({action: 'playback', recording: 'shopping-checkout-20260130T...', test_id: 'replay-checkout'})

// Query and compare logs
observe({what: 'logs', test_id: 'original-checkout'})
observe({what: 'logs', test_id: 'replay-checkout'})
```

---

## MCP API Design

### A. Recording Control

**Start Recording:**
```javascript
configure({
  action: 'recording_start',
  name?: 'shopping-checkout',        // optional, auto-generates if empty
  url?: 'https://example.com/shop'   // optional, auto-navigates if provided
})
```

**Response:**
```json
{
  "status": "ok",
  "recording_id": "shopping-checkout-20260130T143022Z",
  "message": "Recording started. Auto-generated ID: shopping-checkout-20260130T143022Z"
}
```

**Stop Recording:**
```javascript
configure({
  action: 'recording_stop',
  recording_id?: 'shopping-checkout-20260130T143022Z'  // optional, stops current if omitted
})
```

**Response:**
```json
{
  "status": "ok",
  "recording_id": "shopping-checkout-20260130T143022Z",
  "duration_ms": 45000,
  "action_count": 8,
  "screenshot_count": 3
}
```

### B. Recording Query

**List Recordings:**
```javascript
observe({
  what: 'recordings',
  limit?: 10
})
```

**Response:**
```json
{
  "recordings": [
    {
      "id": "shopping-checkout-20260130T143022Z",
      "name": "shopping-checkout",
      "created_at": "2026-01-30T14:30:22Z",
      "duration_ms": 45000,
      "action_count": 8,
      "url": "https://example.com/shop"
    }
  ]
}
```

**Get Recording Actions:**
```javascript
observe({
  what: 'recording_actions',
  recording_id: 'shopping-checkout-20260130T143022Z'
})
```

**Response:**
```json
{
  "recording_id": "shopping-checkout-20260130T143022Z",
  "actions": [
    {
      "type": "navigate",
      "timestamp_ms": 0,
      "url": "https://example.com/shop"
    },
    {
      "type": "click",
      "timestamp_ms": 1200,
      "selector": "[data-testid=product-card-1]",
      "x": 500,
      "y": 300,
      "screenshot_path": "20260130-shopping-checkout-001-click.jpg"
    }
  ]
}
```

### C. Playback

**Execute Playback:**
```javascript
interact({
  action: 'playback',
  recording: 'shopping-checkout-20260130T143022Z',
  test_id?: 'replay-shopping-checkout',          // optional test boundary
  sequence_mode?: true,                          // true: fast-forward, false: timed
  timeout_ms?: 30000                             // max wait per action
})
```

**Response:**
```json
{
  "status": "ok",
  "recording_id": "shopping-checkout-20260130T143022Z",
  "test_id": "replay-shopping-checkout",
  "actions_executed": 8,
  "errors": [
    {
      "action_index": 3,
      "type": "selector_not_found",
      "selector": "[data-testid=product-card-1]",
      "message": "Element not found. Tried CSS selector, then coordinates [500, 300]."
    }
  ],
  "duration_ms": 12000,
  "screenshot_count": 2
}
```

---

## Git Integration (Root Cause Analysis)

### Read-Only Operations

The skill performs **read-only analysis only**:

```go
// Safe operations (read-only)
✅ git log --oneline --all <file>     // Find commits touching file
✅ git show <commit>:<file>            // View file at commit
✅ git log --grep=<pattern>            // Find commits by message
✅ git diff <commit1>..<commit2>       // Show changes between commits
✅ git blame <file>                    // Show who changed what line

// Blocked operations (destructive)
❌ git commit
❌ git push
❌ git reset
❌ git rebase
❌ git checkout
```

### Error Pattern Analysis

When `/gasoline-fix` skill analyzes logs, it detects:

```go
type ErrorType string

const (
    ErrorNetwork    ErrorType = "network"       // 4xx, 5xx, timeout, connection refused
    ErrorDOM        ErrorType = "dom"           // selector not found, moved elements
    ErrorAssertion  ErrorType = "assertion"     // expected text missing
    ErrorTiming     ErrorType = "timing"        // load timeout, slow response
    ErrorUnknown    ErrorType = "unknown"       // can't categorize
)

type RegressionAnalysis struct {
    ErrorType       ErrorType
    ErrorMessage    string
    AffectedURL     string
    AffectedAction  int
    ConfidenceLevel string    // "HIGH" | "MEDIUM" | "LOW"

    SuggestedFixes []FixSuggestion
    RelatedCommits []CommitInfo
    AffectedFiles  []string
}

type FixSuggestion struct {
    File         string // "src/api/checkout.ts"
    LineNumber   int
    Description  string // "Change endpoint from /api/order to /api/orders"
    Confidence   string // "HIGH" | "MEDIUM" | "LOW"
}

type CommitInfo struct {
    Hash      string // "abc123def456"
    Message   string // "Refactor API endpoints"
    Author    string // "alice@company.com"
    Date      string // RFC3339
    Touch     bool   // true if this commit touched affected file
    Candidate bool   // true if likely introduced the issue
}
```

### Implementation: Commit Analysis Algorithm

1. **Identify Affected Files** (from error context)
   - API endpoint changed → affects src/api/*, src/handlers/*
   - DOM element moved → affects src/components/*, src/views/*
   - Field validation failed → affects src/models/*, src/schemas/*

2. **Find Related Commits**
   ```bash
   git log --oneline --all -- <affected_file> | head -20
   ```

3. **Analyze Commit Messages** (pattern matching)
   - "Refactor" + file touched → candidate for issue
   - "Fix" + similar error type → candidate for fix
   - "Remove" + API endpoint → direct match

4. **Show Context**
   ```bash
   git show <commit> -- <file> | head -50
   ```

5. **Rank Confidence**
   - HIGH: Direct match (404 on endpoint X, commit renamed X)
   - MEDIUM: Pattern match (timeout, recent performance commit touched DB)
   - LOW: Speculative (DOM moved, could be many causes)

---

## WebSocket Migration (Log Streaming)

### Current State (Polling)

```javascript
Extension                        Server
─────────                        ──────
  ↓
[poll GET /pending-queries every 100ms]
  ↓ (if data available)
← [return queued actions]
  ↓
[process action]
  ↓
[poll GET /pending-queries again]
```

**Problem:** Polling introduces ≥100ms latency between log entry and poll cycle. Timestamps are inaccurate by up to 100ms.

### Target State (WebSocket)

```javascript
Extension                        Server
─────────                        ──────
[open WebSocket connection]
      ↓
      ↕ [persistent connection]
      ↓
[stream logs in real-time]
  ← {type: 'log', timestamp_ms: 1647302200543, ...}
  ← {type: 'log', timestamp_ms: 1647302200612, ...}
  ← {type: 'action', type: 'click', ...}
```

**Benefit:** Sub-millisecond accuracy, streaming architecture ready for Phase 2 features.

### Implementation

**Server Changes (Go):**
```go
// Upgrade HTTP connection to WebSocket
GET /ws → Upgrade(conn) → handle messages in loop

// Stream logs to all connected clients
for each log entry {
    for each connected WebSocket {
        write(entry)
    }
}

// Buffer management
- Keep 10 seconds of logs in memory
- On overflow: drop oldest, log warning
- On disconnect: clean up connection
```

**Extension Changes (TypeScript):**
```typescript
// Open persistent WebSocket
ws = new WebSocket('ws://localhost:9090/ws')

// Listen for streamed logs
ws.onmessage = (event) => {
  const entry = JSON.parse(event.data)
  addToBuffer(entry)  // add to local ring buffer
}

// Fallback to polling if WebSocket stale > 3s
if (lastMessageAt + 3000 < now()) {
  ws.close()
  switchToPolling()
}
```

---

## Implementation Phases

### Phase 1: Basic Recording & Playback (Weeks 1-2)

**Goals:** Users can record flows and replay them to detect regressions.

**Deliverables:**
1. ✅ Extension records clicks, typing, navigation
2. ✅ Server stores recordings as JSON + screenshots
3. ✅ Playback engine replays action sequences
4. ✅ Test boundary integration (tag logs)
5. ✅ MCP API (record/playback/query)

**Files:**

| File | Changes | LOC |
|------|---------|-----|
| `extension/src/recording.ts` | NEW: Recording controller | 300 |
| `extension/src/element-matcher.ts` | NEW: Selector matching logic | 200 |
| `extension/src/screenshot-manager.ts` | NEW: Screenshot capture | 150 |
| `cmd/dev-console/recording.go` | NEW: Server recording storage | 200 |
| `cmd/dev-console/playback.go` | NEW: Playback engine | 250 |
| `cmd/dev-console/tools.go` | Add recording_start, recording_stop, playback actions | 150 |
| `cmd/dev-console/queries.go` | Add 'recordings', 'recording_actions' handlers | 100 |
| `tests/recording/*.test.ts` | Extension tests | 300 |
| `cmd/dev-console/recording_test.go` | Go tests | 400 |

**Total Phase 1: ~2000 LOC**

### Phase 2: Root Cause Analysis & Git Integration (Weeks 3-4)

**Goals:** Gasoline suggests fixes when regressions are detected.

**Deliverables:**
1. ✅ Log diffing engine (compare original vs replay)
2. ✅ Error pattern analysis
3. ✅ Git commit analysis (read-only)
4. ✅ Claude skill (`/gasoline-fix`)
5. ✅ Test generation (LLM synthesizes variations)

**Files:**

| File | Changes | LOC |
|------|---------|-----|
| `cmd/dev-console/rca.go` | NEW: Root cause analysis engine | 400 |
| `cmd/dev-console/git-analyzer.go` | NEW: Git commit analysis | 200 |
| `cmd/dev-console/skill-gasoline-fix.md` | NEW: Claude skill definition | 150 |
| `cmd/dev-console/tools.go` | Add /gasoline-fix skill handler | 100 |
| `cmd/dev-console/rca_test.go` | Root cause analysis tests | 300 |
| `cmd/dev-console/git_analyzer_test.go` | Git analysis tests | 250 |

**Total Phase 2: ~1400 LOC**

### Phase 3: Large-Scale Storage & Performance (Weeks 5-6)

**Goals:** Handle long-running recordings, archival, and cloud storage.

**Deliverables:**
1. ✅ SQLite database for recording metadata
2. ✅ Recording archival (compress old recordings)
3. ✅ Cloud storage integration (S3/GCS)
4. ✅ Recording browser UI (list, delete, export)

**Defer to later** — Not critical for MVP.

---

## Critical Implementation Details

### 1. Element Selector Matching (Playback)

When replaying a click, the engine tries selectors in priority order:

```go
func findElement(action Action) (*Element, error) {
    // 1. Try data-testid (most reliable)
    if action.DataTestID != "" {
        elem := browser.querySelector(`[data-testid="${action.DataTestID}"]`)
        if elem != nil {
            return elem, nil
        }
    }

    // 2. Try CSS selector from recording
    if action.Selector != "" {
        elem := browser.querySelector(action.Selector)
        if elem != nil {
            return elem, nil
        }
    }

    // 3. Try aria-label
    if action.AriaLabel != "" {
        elem := browser.querySelector(`[aria-label="${action.AriaLabel}"]`)
        if elem != nil {
            return elem, nil
        }
    }

    // 4. Fall back to x/y coordinates
    elem := browser.elementAtPoint(action.X, action.Y)
    if elem != nil {
        logWarning("Element moved from [%d, %d] to [%d, %d]",
            action.X, action.Y, elem.rect.X, elem.rect.Y)
        takeScreenshot("moved-selector")
        return elem, nil
    }

    return nil, fmt.Errorf("Element not found: %s", action.Selector)
}
```

### 2. Screenshot Management

```typescript
// Capture screenshot with metadata
async function captureScreenshot(
  recordingId: string,
  actionIndex: number,
  issueType: string  // "click", "type", "navigate", "error", "moved-selector"
) {
  const canvas = await html2canvas(document.body)
  const blob = canvas.toBlob('image/jpeg', 0.85)  // 85% compression

  const filename = `${date}-${recordingId}-${actionIndex}-${issueType}.jpg`

  // Send to server via POST /recordings/{id}/screenshot
  const formData = new FormData()
  formData.append('file', blob, filename)
  formData.append('issue_type', issueType)

  await fetch(`/recordings/${recordingId}/screenshot`, {
    method: 'POST',
    body: formData
  })
}
```

### 3. Recording Ring Buffer (Server)

```go
type RecordingBuffer struct {
    mu        sync.Mutex
    recordings []*Recording      // ring buffer, max 100 recordings
    index     int                // current position
    maxSize   int                // 100
}

func (rb *RecordingBuffer) Add(r *Recording) {
    rb.mu.Lock()
    defer rb.mu.Unlock()

    rb.recordings[rb.index%rb.maxSize] = r
    rb.index++

    // Check disk space
    if rb.totalDiskUsage() > 1*1024*1024*1024 {  // 1GB
        rb.archiveOldestRecording()
    }
}

func (rb *RecordingBuffer) Query() []*Recording {
    // Most recent 10
    rb.mu.Lock()
    defer rb.mu.Unlock()

    return rb.recordings[max(0, rb.index-10):rb.index]
}
```

### 4. Log Diffing (Root Cause Analysis)

```go
func diffLogs(originalLogs, replayLogs []LogEntry) *RegressionReport {
    // Find errors in replay that weren't in original
    originalErrors := extractErrors(originalLogs)
    replayErrors := extractErrors(replayLogs)

    newErrors := difference(replayErrors, originalErrors)

    report := &RegressionReport{
        NewErrors: newErrors,
        ErrorCount: len(newErrors),
    }

    for _, err := range newErrors {
        report.ErrorTypes[err.Type]++

        // Analyze error pattern
        if err.Type == ErrorNetwork && err.Status == 404 {
            report.Candidate = "API endpoint missing or renamed"
            report.Confidence = "HIGH"
        }

        if err.Type == ErrorDOM && strings.Contains(err.Message, "not found") {
            report.Candidate = "DOM structure changed"
            report.Confidence = "MEDIUM"
        }
    }

    return report
}
```

---

## Performance Targets

| Component | Target | Notes |
|-----------|--------|-------|
| Extension recording overhead | < 5% CPU | Non-intrusive |
| Playback speed (sequence mode) | 10+ actions/sec | Fast-forward |
| Screenshot capture | < 100ms per image | Async, non-blocking |
| Log streaming (WebSocket) | < 1ms latency | Sub-millisecond accuracy |
| Recording query response | < 500ms | List 100 recordings |
| Playback engine startup | < 1s | Navigate + setup |
| Disk storage per 1hr recording | < 500MB | With screenshots |

---

## Testing Strategy

### Phase 1 Unit Tests

**Extension (TypeScript):**
- Recording start/stop
- Selector matching (data-testid → CSS → x/y)
- Screenshot capture (compression, size limits)
- Action serialization/deserialization

**Server (Go):**
- Recording storage (add, query, cleanup)
- Playback execution (navigate, click, type, error handling)
- Element matching (priority order, fallbacks)
- Test boundary tagging

### Phase 2 Integration Tests

- End-to-end: record → playback → compare logs
- Error detection (404, timeout, selector not found)
- Git integration (find commits, analyze patterns)
- Claude skill execution

### Performance Tests

- Recording 500+ actions, verify memory < 20MB
- Playback 100 actions in sequence mode, verify time < 10s
- Screenshot batch capture, verify throughput > 10 img/sec

---

## Error Handling

### Recording Errors

```
Selector no longer valid
→ Log warning
→ Take screenshot with issue_type: "moved-selector"
→ Continue recording (non-blocking)

Screenshot exceeds 500KB
→ Reduce compression to 75%
→ If still > 500KB, skip screenshot, log warning

Recording exceeds 30 minutes
→ Auto-stop recording
→ Log message: "Recording auto-stopped after 30 minutes"

Recording exceeds 500 actions
→ Auto-stop recording
→ Log message: "Recording auto-stopped after 500 actions"
```

### Playback Errors

```
Selector not found
→ Try fallback (x/y coordinates)
→ If still not found, log error, take screenshot
→ Continue playback (non-blocking)

Navigation timeout (5s)
→ Log error: "Navigation to <URL> timed out"
→ Continue playback (non-blocking)

Element click outside viewport
→ Scroll element into view
→ Try click again
```

---

## Security & Privacy

### Sensitive Data Handling

Recording can capture passwords, credit cards, API keys. By default:
- ✅ **DO capture** full text (necessary for regression testing)
- ⚠️ **Warn user** if sensitive patterns detected
- 🔧 **Allow user setting** to redact sensitive fields (Phase 2)

Example warning:
```
⚠️ Recording contains sensitive patterns:
   - Password field (type=password): text captured
   - CVV field (name=cvv): text captured

Recommendation: Disable this setting if testing login flows with real credentials.
See: chrome://extensions/... > Gasoline > Options
```

### Git Access

When analyzing git commits:
- ✅ Read-only operations (log, show, diff)
- ❌ No write access (no commit, push, reset)
- ✅ Limit to repository provided by user
- ❌ No access to parent directories or other repos

---

## Success Criteria (Tech Spec Gate)

✅ **Architecture & Design**
- Clear separation between recording, playback, RCA components
- MCP API consistent with observe/interact/configure patterns
- Test boundary integration works correctly
- Git operations safe (read-only)

✅ **Data Structures**
- Action format supports all interaction types
- JSON serialization roundtrips correctly
- Screenshots on disk with proper naming/cleanup

✅ **Performance**
- Recording overhead < 5% CPU
- Playback 10+ actions/sec
- WebSocket streaming < 1ms latency

✅ **Error Handling**
- Non-blocking error recovery
- Selector fallbacks (data-testid → CSS → x/y)
- Comprehensive logging

✅ **Testing**
- Unit tests cover happy path + error paths
- Edge cases (moved elements, long recordings, missing selectors)
- Zero regressions (existing Gasoline tests still pass)

---

## Next Steps

**Gate 3: Principal Review** → Approve architecture and design

**Gate 4: QA Plan** → Define test cases before implementation

**Gate 5: Implementation** → Phase 1 basic recording/playback, Phase 2 RCA + git

