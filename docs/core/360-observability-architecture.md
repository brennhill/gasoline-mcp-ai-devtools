---
status: proposed
scope: v6-v7-architecture
ai-priority: critical
tags: [v6, v7, architecture, system-design, data-flow, storage, api]
relates-to: [360-observability-roadmap.md, backend-frontend-unification.md]
last-verified: 2026-01-31
---

# Gasoline 360° Observability: Target Architecture

**Comprehensive system design for v6 and v7 that enables autonomous feature development and test automation.**

---

## System Overview

### High-Level Architecture

```
┌─────────────────────────────────────────────────────────────────────┐
│                      GASOLINE 360 PLATFORM                         │
├──────────────────┬──────────────────┬──────────────────┬────────────┤
│  DATA SOURCES    │  INGESTION       │  PROCESSING      │  OUTPUT    │
├──────────────────┼──────────────────┼──────────────────┼────────────┤
│                  │                  │                  │            │
│ 📱 Browser       │                  │                  │            │
│ ├─ Console logs  │ ┌──────────────┐ │ ┌──────────────┐ │            │
│ ├─ Network       │ │ Ingest Layer │ │ │ Normalize    │ │ 🔍 Query   │
│ ├─ DOM/DOM       │ │              │ │ │ Schema       │ │ Service    │
│ ├─ Screenshots   │ ├─ Tail logs   │ │ │              │ │ ├─ Timeline│
│ └─ Events        │ ├─ Parse JSON  │ │ ├─ Dedupe      │ │ ├─ Search  │
│                  │ ├─ Extract IDs │ │ ├─ Correlate   │ │ └─ Analyze │
│ 🖥️  Backend      │ └──────────────┘ │ ├─ Enrich      │ │            │
│ ├─ App logs      │                  │ └──────────────┘ │ 📊 Reports │
│ ├─ DB queries    │ ┌──────────────┐ │                  │            │
│ ├─ Services      │ │ Ring Buffers │ │ ┌──────────────┐ │ 🤖 LLM     │
│ ├─ Errors        │ │ (24h TTL)    │ │ │ Correlation  │ │ Context    │
│ └─ Metrics       │ │ Per stream   │ │ │ Engine       │ │            │
│                  │ │ (circular)   │ │ │              │ │            │
│ 🧪 Tests         │ └──────────────┘ │ ├─ Link events │ │            │
│ ├─ Pass/fail     │                  │ ├─ Build       │ │            │
│ ├─ Duration      │ ┌──────────────┐ │ │   timeline   │ │            │
│ ├─ Coverage      │ │ Parser Pool  │ │ ├─ Causality  │ │            │
│ └─ Output        │ │              │ │ └──────────────┘ │            │
│                  │ ├─ Logs        │ │                  │            │
│ 📝 Git           │ ├─ Test output │ │ ┌──────────────┐ │            │
│ ├─ Commits       │ ├─ Network     │ │ │ Regression   │ │            │
│ ├─ Diffs         │ └──────────────┘ │ │ Detection    │ │            │
│ └─ Changes       │                  │ │              │ │            │
│                  │                  │ ├─ Compare     │ │            │
│ 🎯 Custom        │                  │ ├─ Baseline    │ │            │
│ └─ Business      │                  │ └──────────────┘ │            │
│    events        │                  │                  │            │
└──────────────────┴──────────────────┴──────────────────┴────────────┘
                              ↓
                      ┌───────────────┐
                      │  MCP SERVER   │
                      │  (stdio bridge)│
                      └───────────────┘
                              ↓
                    🧠 Claude / GPT / etc.
```

---

## Component Architecture

### 1. Ingestion Layer

**Purpose:** Collect data from all sources without blocking or losing events.

#### Browser Extension (Existing, Enhanced)

```go
// v5 capabilities (keep)
- Console logs
- Network requests/responses
- WebSocket events
- DOM mutations
- User actions
- Screenshots
- Web Vitals

// v6 additions
- Performance timing per action
- DOM state snapshots
- Accessibility violations
- Custom event API
- Correlation ID injection

// v7 additions
- Trace ID propagation (W3C)
- Request correlation headers
- Source mapping for minified code
```

**Architecture:**

```typescript
// extension/content.js
class GasolineSensor {
  private buffer = new CircularBuffer(maxSize: 10_000_events);
  private correlationId = generateUUID();

  // Intercept and buffer all events
  captureEvent(event: GasolineEvent) {
    event.correlation_id = this.correlationId;
    event.timestamp = performance.now();
    this.buffer.push(event);
    this.sendToDaemon(event); // Real-time stream
  }

  // v7: Inject trace IDs into network requests
  interceptFetch() {
    const original = window.fetch;
    window.fetch = (url, options) => {
      options.headers = {
        ...options.headers,
        'x-correlation-id': this.correlationId,
        'x-trace-id': generateTraceId(),
        'x-timestamp': Date.now()
      };
      return original(url, options);
    };
  }
}
```

#### Backend Log Streaming (v6/v7)

**Input sources:**
- Dev server stdout (Node.js, Python, Go, etc.)
- Docker container logs
- Log files (tail -f)
- Structured logging (JSON)
- Syslog

**Implementation:**

```go
// server/ingestion/log_streamer.go
type LogStreamer struct {
  source LogSource // "dev_server", "docker", "file", "syslog"
  parser Parser     // Parse JSON, plain text, etc.
  buffer *RingBuffer
}

func (ls *LogStreamer) Stream(ctx context.Context) {
  scanner := NewLogScanner(ls.source)
  for scanner.Scan() {
    line := scanner.Text()
    event := ls.parser.Parse(line)
    event.correlation_id = ExtractCorrelationID(line)
    event.source = "backend"
    event.timestamp = time.Now()
    ls.buffer.Push(event)
    ls.publishEvent(event) // Real-time
  }
}
```

#### Test Execution Capture (v6/v7)

**Intercept test runners:**

```go
// server/ingestion/test_capturer.go
type TestCapturer struct {
  framework string // "jest", "pytest", "mocha", "go test"
  buffer    *RingBuffer
}

// Hook into test runner output
func (tc *TestCapturer) CaptureJest(ctx context.Context) {
  // Intercept Jest reporter
  // Extract: test name, pass/fail, duration, stack trace

  event := &TestEvent{
    name:        "User can add item to cart",
    status:      "passed",
    duration_ms: 1250,
    file:        "tests/checkout.test.js:45",
  }
  tc.buffer.Push(event)
}
```

#### Git Event Tracking (v7)

```go
// server/ingestion/git_tracker.go
type GitTracker struct {
  repoPath string
  buffer   *RingBuffer
}

func (gt *GitTracker) TrackChanges(ctx context.Context) {
  watcher := NewGitWatcher(gt.repoPath)
  for change := range watcher.Changes() {
    event := &GitEvent{
      type:           change.Type, // "modified", "created", "deleted"
      files:          change.Files,
      commit:         change.Commit,
      timestamp:      change.Timestamp,
      author:         change.Author,
      correlation_id: GenerateCorrelationID(), // Link to related events
    }
    gt.buffer.Push(event)
  }
}
```

---

### 2. Storage Architecture

**Goals:**
- Never lose events
- Query efficiently
- TTL-based cleanup (24h default)
- Memory bounded

#### Ring Buffer Design (Per Stream)

```go
// server/storage/ring_buffer.go
type RingBuffer struct {
  data       []Event
  head       int64         // Next write position
  tail       int64         // Oldest event position
  size       int           // Capacity
  mu         sync.RWMutex
  ttl        time.Duration // Auto-cleanup old entries
}

// Example: Browser buffer
browserBuffer := NewRingBuffer(
  capacity: 10_000_events,
  ttl:      24 * time.Hour,
)

// Example: Backend buffer (separate)
backendBuffer := NewRingBuffer(
  capacity: 50_000_events,  // More events from backend
  ttl:      24 * time.Hour,
)

// Example: Network buffer (separate)
networkBuffer := NewRingBuffer(
  capacity: 5_000_events,
  ttl:      24 * time.Hour,
)
```

**Per-buffer breakdown:**

| Buffer | Capacity | TTL | Reason |
|--------|----------|-----|--------|
| Browser logs | 10K | 24h | Console-heavy, deduplicated |
| Browser network | 5K | 24h | Compressed (headers + summary) |
| Browser DOM | 2K | 24h | Snapshots only, not every mutation |
| Backend logs | 50K | 24h | Verbose, many services |
| Tests | 5K | 24h | Metadata only |
| Git events | 1K | 24h | Small events |
| Custom events | 10K | 24h | App-injected |

**Memory calculation:**
- Average event: ~500 bytes (compressed)
- 10K events × 500 bytes = 5MB per buffer
- ~10 buffers = ~50MB total
- With OS overhead: ~100-200MB target

#### Normalized Event Schema

```typescript
// Common schema for all sources
interface NormalizedEvent {
  // Always present
  id: string;                    // UUID
  timestamp: number;             // Unix milliseconds
  source: "browser" | "backend" | "test" | "git" | "custom";
  level: "debug" | "info" | "warn" | "error" | "critical";

  // v6: Correlation
  correlation_id?: string;       // Browser session or trace ID

  // v7: Full tracing
  trace_id?: string;             // W3C Trace Context
  span_id?: string;
  parent_span_id?: string;

  // Content (varies by source)
  message: string;
  metadata: Record<string, any>;

  // v6: Indexing
  tags: string[];               // For filtering

  // Source-specific
  source_details: {
    browser?: { url: string, origin: string };
    backend?: { service: string, host: string };
    test?: { framework: string, file: string };
    git?: { repo: string, commit: string };
  };
}
```

**Benefits:**
- Single schema for querying
- Efficient filtering (all events in same format)
- LLM sees consistent structure
- Easy to add new sources

---

### 3. Processing Pipeline

**Architecture:**

```
┌─────────────────┐
│  Ingest Layer   │
│  (Ring Buffers) │
└────────┬────────┘
         │
         ↓ Stream of events
┌─────────────────────┐
│  Normalization      │  Ensure consistent schema
│  (Parser Pool)      │  Extract correlation IDs
└────────┬────────────┘
         │
         ↓ Normalized events
┌─────────────────────┐
│  Correlation        │  Link browser ↔ backend
│  Engine             │  Match by: trace ID, timestamp, user session
│  (v7)               │
└────────┬────────────┘
         │
         ↓ Correlated events
┌─────────────────────┐
│  Enrichment         │  Add context
│  (v7)               │  Link to code, tests, git
└────────┬────────────┘
         │
         ↓ Enriched events
┌─────────────────────┐
│  Regression         │  Compare to baseline
│  Detection          │  Detect unexpected changes
│  (v6)               │
└────────┬────────────┘
         │
         ↓ Analysis results
┌─────────────────────┐
│  Query Service      │  Make queryable
│  (Indexing)         │  Timeline, search, filter
└─────────────────────┘
```

#### Step 1: Normalization

```go
// server/processing/normalizer.go
func NormalizeEvent(raw RawEvent) NormalizedEvent {
  switch raw.Source {
  case "browser":
    return normalizeBrowserEvent(raw)
  case "backend":
    return normalizeBackendEvent(raw)
  case "test":
    return normalizeTestEvent(raw)
  case "git":
    return normalizeGitEvent(raw)
  }
}

func normalizeBrowserEvent(raw RawEvent) NormalizedEvent {
  // Extract correlation ID from source or generate
  correlationID := raw.Metadata["correlation_id"]
  if correlationID == "" {
    correlationID = raw.SessionID
  }

  return NormalizedEvent{
    id:              generateUUID(),
    timestamp:       raw.Timestamp,
    source:          "browser",
    level:           raw.Level,
    message:         raw.Message,
    correlation_id:  correlationID,
    tags:            extractTags(raw),
  }
}
```

#### Step 2: Correlation (v7)

```go
// server/processing/correlator.go
type Correlator struct {
  browserEvents  *RingBuffer
  backendEvents  *RingBuffer
  correlations   map[string]*CorrelationChain
}

func (c *Correlator) CorrelateByTraceID(traceID string) []*NormalizedEvent {
  // Find all events with this trace ID
  events := []*NormalizedEvent{}

  // Browser events
  for event := range c.browserEvents.Query(
    Query{trace_id: traceID}) {
    events = append(events, event)
  }

  // Backend events
  for event := range c.backendEvents.Query(
    Query{trace_id: traceID}) {
    events = append(events, event)
  }

  // Sort by timestamp
  sort.Slice(events, func(i, j int) bool {
    return events[i].Timestamp < events[j].Timestamp
  })

  return events
}

func (c *Correlator) CorrelateByTimestamp(
  browserEvent *NormalizedEvent,
  timeWindow time.Duration) []*NormalizedEvent {

  // Find backend events within N milliseconds
  result := []*NormalizedEvent{browserEvent}

  for event := range c.backendEvents.Query(
    Query{
      min_timestamp: browserEvent.Timestamp,
      max_timestamp: browserEvent.Timestamp + timeWindow.Milliseconds(),
    }) {
    result = append(result, event)
  }

  return result
}
```

#### Step 3: Enrichment (v7)

```go
// server/processing/enricher.go
type Enricher struct {
  codebase *Codebase  // Index of code + tests
  gitRepo  *GitRepo   // Git history
}

func (e *Enricher) EnrichEvent(event *NormalizedEvent) {
  // Add code context
  if event.source == "backend" {
    if stackTrace := event.metadata["stack_trace"]; stackTrace != "" {
      frames := parseStackTrace(stackTrace)
      for _, frame := range frames {
        code, _ := e.codebase.Read(frame.file, frame.line)
        event.metadata["code_context"] = code
      }
    }
  }

  // Add git context
  if filename := event.metadata["filename"]; filename != "" {
    commit, _ := e.gitRepo.LastCommit(filename)
    event.metadata["last_change"] = commit
  }

  // Add test coverage
  if filename := event.metadata["filename"]; filename != "" {
    tests, _ := e.codebase.TestsFor(filename)
    event.metadata["related_tests"] = tests
  }
}
```

#### Step 4: Regression Detection (v6)

```go
// server/processing/regression_detector.go
type RegressionDetector struct {
  baselineCheckpoint *Checkpoint
  currentEvents      []*NormalizedEvent
}

func (rd *RegressionDetector) Detect() []Regression {
  regressions := []Regression{}

  // Compare current events to baseline
  for _, event := range rd.currentEvents {
    baseline := rd.baselineCheckpoint.EventsLike(event)

    if baseline == nil {
      // New event, might be bug
      regressions = append(regressions, Regression{
        type:     "new_event",
        event:    event,
        severity: "low",
      })
    } else if event.Message != baseline.Message {
      // Changed behavior
      regressions = append(regressions, Regression{
        type:     "behavior_change",
        event:    event,
        baseline: baseline,
        severity: "high",
      })
    }
  }

  return regressions
}
```

---

### 4. Query Service

**Purpose:** Expose queryable APIs for LLM and CLI

#### Query API

```typescript
// /observe endpoint (MCP tool: observe)
interface ObserveQuery {
  what: "logs" | "network_waterfall" | "websocket_events" | "actions"
        | "timeline" | "correlation" | "causality";
  filter?: {
    url?: string;
    method?: string;
    status?: number;
    level?: "debug" | "info" | "warn" | "error";
    source?: string;
    tags?: string[];
  };
  limit?: number;
  offset?: number;
  after_cursor?: string;
  before_cursor?: string;
}

// Response
interface ObserveResponse {
  events: NormalizedEvent[];
  metadata: {
    total_count: number;
    cursor_next?: string;
    cursor_prev?: string;
    query_time_ms: number;
  };
}
```

**Implementation:**

```go
// server/api/query_service.go
func (qs *QueryService) Execute(query ObserveQuery) ObserveResponse {
  var events []*NormalizedEvent

  switch query.what {
  case "timeline":
    events = qs.queryTimeline(query)
  case "correlation":
    events = qs.queryCorrelation(query)
  case "causality":
    events = qs.queryCausality(query)
  default:
    events = qs.queryLogs(query)
  }

  // Apply pagination
  start := query.offset
  end := start + query.limit
  paginated := events[start:end]

  return ObserveResponse{
    events: paginated,
    metadata: Metadata{
      total_count: len(events),
      cursor_next: generateCursor(end),
    },
  }
}
```

#### Timeline Query

```go
// Returns all events, sorted by timestamp, with trace context
func (qs *QueryService) queryTimeline(query ObserveQuery) []*NormalizedEvent {
  events := []*NormalizedEvent{}

  // Gather from all buffers
  for event := range qs.browserBuffer.All() {
    if qs.matches(event, query.filter) {
      events = append(events, event)
    }
  }
  for event := range qs.backendBuffer.All() {
    if qs.matches(event, query.filter) {
      events = append(events, event)
    }
  }

  // Sort by timestamp
  sort.Slice(events, func(i, j int) bool {
    return events[i].Timestamp < events[j].Timestamp
  })

  return events
}
```

#### Correlation Query (v7)

```go
// Returns all events linked by trace/correlation ID
func (qs *QueryService) queryCorrelation(query ObserveQuery) []*NormalizedEvent {
  traceID := query.filter["trace_id"]

  return qs.correlator.CorrelateByTraceID(traceID)
}
```

#### Causality Query (v7)

```go
// Returns causality chain: root cause → impact → symptom
func (qs *QueryService) queryCausality(query ObserveQuery) []*NormalizedEvent {
  symptoms := qs.queryLogs(query) // Find symptoms
  chains := []*NormalizedEvent{}

  for _, symptom := range symptoms {
    root := qs.findRootCause(symptom)
    chain := qs.correlator.TraceChain(root, symptom)
    chains = append(chains, chain...)
  }

  return chains
}
```

---

### 5. MCP Tool API

**Five canonical tools** (unchanged from v6 design):

#### observe

```typescript
// Query captured data
observe({
  what: "timeline",
  filter: { level: "error" },
  limit: 100
})
→ [events with causality context]
```

#### generate

```typescript
// Generate tests, fixes, etc.
generate({
  format: "test",
  test_name: "User can add to cart",
  context: /* from observe */
})
→ Playwright test code
```

#### configure

```typescript
// Save checkpoints, set baselines, etc.
configure({
  action: "save_checkpoint",
  name: "happy_path_checkout",
  events: /* from observe */
})
→ Checkpoint stored
```

#### interact

```typescript
// Explore, record, replay
interact({
  action: "explore",
  steps: [{action: "click", selector: "button"}]
})
→ Observations captured
```

#### analyze

```typescript
// Infer, detect loops, etc.
analyze({
  type: "infer",
  context: /* from observe */,
  question: "Why did this fail?"
})
→ Analysis result
```

---

### 6. Persistence Layer

#### Checkpoint Format

```json
{
  "name": "happy_path_checkout",
  "timestamp": "2026-01-31T10:00:00Z",
  "events": [
    {
      "id": "uuid",
      "timestamp": 0,
      "source": "browser",
      "level": "info",
      "message": "User navigated to /checkout",
      "correlation_id": "session-123"
    },
    // ... more events
  ],
  "metadata": {
    "duration_ms": 5000,
    "actions_count": 8,
    "network_requests": 12,
    "errors": 0
  }
}
```

#### Baseline Storage

```
.gasoline/
├── checkpoints/
│   ├── happy_path_checkout.json
│   ├── add_to_cart.json
│   └── spec_validation.json
├── contracts/
│   ├── auth-service.json
│   └── payment-service.json
├── dependencies.json
└── edge-cases.json
```

---

## Data Flow Examples

### Example 1: Spec-Driven Validation (v6)

```
1. Developer: "Validate signup form"
   ↓
2. LLM: observe({what: "page"})
   → Browser state at /signup
   ↓
3. LLM: interact({action: "explore", steps: [...]})
   → User fills form, submits
   ↓
4. Gasoline captures:
   - DOM before/after
   - Console logs
   - Network requests
   - User actions
   ↓
5. LLM: analyze({type: "infer", ...})
   → "Password field accepts 6 chars, spec requires 8+"
   ↓
6. Result: Bug identified, ready to fix
```

**Data flow:**
```
Browser Events → Ring Buffer → Normalize → Timeline Query → LLM Context
Network Events → Ring Buffer → Normalize ↗
```

### Example 2: Production Error Reproduction (v6→v7)

```
1. Developer: Error in checkout (prod), here's recording
   ↓
2. LLM: analyze({type: "infer", context: recording})
   → "HTTP 500 on payment API"
   ↓
3. LLM (v7): analyze({type: "correlate", trace_id: "abc123"})
   → Backend logs show: DB timeout
   ↓
4. LLM: analyze({type: "causality", symptom: "payment_failed"})
   → Chain: [User clicked] → [API called] → [DB slow] → [Timeout]
   ↓
5. Result: Root cause identified (DB timeout), fix clear
```

**Data flow:**
```
Browser Recording ──→ Normalize ──→ Correlation ──→ Enrichment ──→ Causality Analysis
Backend Logs ─────→ Normalize ──↗
Git History ──────→ Normalize ──→ Enrichment ──→ (shows change 3 days ago)
```

### Example 3: Feature Implementation (v6→v7)

```
1. LLM: Read spec "Add price filter"
   ↓
2. LLM: observe({what: "code"})
   → Current product list implementation
   ↓
3. LLM: generate({format: "implementation_plan"})
   → Files to modify, tests to add
   ↓
4. LLM: interact({action: "apply_changes", ...})
   → Modify code
   ↓
5. LLM: interact({action: "explore", steps: [use_filter]})
   → Test new feature
   ↓
6. Gasoline captures new behavior
   ↓
7. LLM: analyze({type: "regression"})
   → Compare to checkpoint
   ↓
8. Result: Feature works, no regression
```

**Data flow:**
```
Code (v7) ──────────→ Enrich ──→ Implementation Plan
Checkpoints (v6) ──→ Normalize → Regression Detection
New behavior ──────→ Normalize → Compare → Result
```

---

## Performance Targets

| Operation | Target | Notes |
|-----------|--------|-------|
| Event ingestion latency | <1ms | Per event, no blocking |
| Timeline query | <500ms | 10K events, all buffers |
| Correlation query | <200ms | Trace ID lookup, v7 |
| Causality analysis | <2s | LLM reasoning, v7 |
| Memory overhead | <200MB | All buffers + indexes |
| Network traffic (to LLM) | <10KB/query | Compressed context |
| Browser extension overhead | <0.1ms | Per console.log |
| Test capture latency | <10ms | Per test result |

---

## API Contracts

### v6 Contracts

```typescript
// observe
type ObserveInput = {
  what: "logs" | "network_waterfall" | "network_bodies" |
        "websocket_events" | "actions" | "timeline";
  limit?: number;
  offset?: number;
  after_cursor?: string;
};

// generate
type GenerateInput = {
  format: "test" | "reproduction" | "pr_summary";
  test_name?: string;
  context?: any;
};

// configure
type ConfigureInput = {
  action: "save_checkpoint" | "set_baseline" |
          "clear" | "noise_rule";
  name?: string;
  buffer?: string;
};

// interact
type InteractInput = {
  action: "explore" | "record" | "replay";
  steps?: any[];
  snapshot_name?: string;
};

// analyze
type AnalyzeInput = {
  type: "infer" | "detect_loop" | "regression";
  context?: any;
  question?: string;
};
```

### v7 Extensions

```typescript
// observe (v7 additions)
what: "correlations" | "causality" | "impact_analysis";

// analyze (v7 additions)
type: "correlate" | "causality_chain" | "impact_analysis" |
      "contract_validation" | "edge_case_check";
```

---

## Deployment Architecture

### Monolithic (v6)

```
┌──────────────────────┐
│  Single Go Binary    │
├──────────────────────┤
│ Ingestion            │
│ Processing           │
│ Storage              │
│ Query                │
│ MCP Server           │
└──────────────────────┘
     localhost:7890
```

### Distributed (v7+, Optional)

```
┌──────────┐     ┌──────────┐     ┌──────────┐
│ Ingest   │────→│Processing│────→│ Query    │
│Service   │     │Service   │     │Service   │
└──────────┘     └──────────┘     └──────────┘
       ↓               ↓                ↓
  Ring Buffers   Normalized Schema   Indexes
```

**Not planned for v6/v7**, but architecture allows:
- Scaling ingestion
- Scaling processing
- Scaling queries
- Shared storage (optional)

---

## Technology Choices & Rationale

| Component | Technology | Why |
|-----------|-----------|-----|
| **Language** | Go (binary) | Single binary, no runtime, fast |
| **Storage** | In-memory + disk (optional) | Bounded memory, no DB dependency |
| **Serialization** | JSON | Human-readable, debuggable |
| **Parsing** | Custom (not regex) | Performance, correctness |
| **Concurrency** | goroutines + channels | Lightweight, efficient |
| **Buffers** | Ring buffers | O(1) operations, bounded memory |
| **Indexing** | Hash maps | Fast lookups, simple |
| **Network** | HTTP + stdio (MCP) | Standard, no new protocols |

---

## Security & Privacy

### Data Isolation

- **Local-only:** All data stays on localhost:7890
- **Ephemeral:** Data cleared on process exit (by default)
- **Filtered:** Auth tokens, API keys auto-redacted
- **Optional persistence:** Developer can opt-in to save checkpoints

### Authentication

```go
// Optional: Require API key for HTTP access
// (MCP uses stdio, so no auth needed there)
if os.Getenv("GASOLINE_API_KEY") != "" {
  apiKey := os.Getenv("GASOLINE_API_KEY")
  // Validate key on all HTTP requests
}
```

---

## Scalability Limits

**Single instance limits:**
- Events: ~200K/day (with 24h TTL)
- Buffer size: ~200MB memory
- Query latency: <1s for any query
- Concurrent clients: 1 (MCP) + web UI
- Checkpoint size: ~10MB

**For higher volumes:**
- Option 1: Multiple instances (separate contexts)
- Option 2: Distributed architecture (v7.1+)
- Option 3: Streaming to external storage (future)

---

## Testing Strategy

### Unit Tests

```
✅ Normalization (convert raw events to schema)
✅ Correlation (link events by trace ID)
✅ Ring buffer (insertion, TTL, queries)
✅ Query service (all query types)
✅ Regression detection (checkpoint comparison)
```

### Integration Tests

```
✅ End-to-end: Browser → Ingestion → Storage → Query
✅ Real browser extension → Daemon
✅ Real test runner capture
✅ Real backend logs
```

### Load Tests

```
✅ 1000 events/sec ingestion
✅ 10K events in memory
✅ 1MB checkpoint serialization
```

---

## Implementation Sequencing

**Based on this architecture, the work sequences as:**

1. **Foundation (v6 Phase 1):** Ingestion + Normalization
2. **Queryability (v6 Phase 2):** Storage + Query Service
3. **Correlation (v7 Phase 1):** Correlation Engine
4. **Enrichment (v7 Phase 2):** Context Addition
5. **Advanced (v7+):** Causality, Impact Analysis, etc.

See `360-observability-roadmap.md` for detailed sequencing.

---

## Appendix: Data Schema Examples

### Browser Event

```json
{
  "id": "uuid-1",
  "timestamp": 1706700000000,
  "source": "browser",
  "level": "info",
  "message": "User clicked checkout button",
  "correlation_id": "session-123",
  "tags": ["user_action", "checkout"],
  "metadata": {
    "action": "click",
    "selector": "button.checkout",
    "url": "https://shop.local/cart",
    "user_agent": "Chrome/...",
    "session_id": "session-123"
  }
}
```

### Backend Event

```json
{
  "id": "uuid-2",
  "timestamp": 1706700000050,
  "source": "backend",
  "level": "info",
  "message": "Payment API called",
  "trace_id": "trace-abc123",
  "tags": ["api", "payment", "external"],
  "metadata": {
    "service": "payment-service",
    "endpoint": "POST /api/charges",
    "duration_ms": 1500,
    "status": 200,
    "external_api": "stripe",
    "request_id": "req-123"
  }
}
```

### Correlated Timeline

```json
{
  "correlation_id": "session-123",
  "trace_id": "trace-abc123",
  "events": [
    {"timestamp": 0, "source": "browser", "message": "Click checkout"},
    {"timestamp": 50, "source": "backend", "message": "POST /api/charges"},
    {"timestamp": 1550, "source": "backend", "message": "Payment complete"},
    {"timestamp": 1555, "source": "browser", "message": "Show success"}
  ],
  "total_time_ms": 1555,
  "latency_breakdown": {
    "browser_to_network": 50,
    "network_roundtrip": 1500,
    "backend_processing": 1450
  }
}
```

---

**Document Status:** Target Architecture v1
**Last Updated:** 2026-01-31
**Ready For:** Implementation sequencing and sprint planning

