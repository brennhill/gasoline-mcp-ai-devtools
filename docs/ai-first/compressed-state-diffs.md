# Compressed State Diffs (`get_changes_since`)

## Status: Specification

---

## Justification

### The Problem

AI coding agents operate in tight edit-verify loops: make a change, check if it worked, fix if broken, repeat. Each verification step requires understanding browser state. Today, Gasoline exposes raw buffers:

- `get_browser_logs` → up to 1000 log entries
- `get_network_bodies` → up to 100 entries with full request/response bodies
- `get_websocket_events` → up to 500 events
- `get_enhanced_actions` → up to 50 actions

For an agent making 50+ edits per session, re-reading full state each time is:
- **Token-expensive:** 10-20K tokens per full state read × 50 checks = 500K-1M tokens/session wasted on state reads
- **Slow:** Each full read adds latency to the feedback loop
- **Noisy:** Most of the state hasn't changed since the last check

### The Insight

After an edit, the agent needs to know only **what changed** — the delta. If the agent checked state at timestamp T1 and now checks at T2, only events/changes between T1 and T2 matter.

### Why This is AI-Critical

Humans have peripheral awareness — they notice new console errors appearing in DevTools without actively reading every log. AI agents have no peripheral awareness. They must explicitly poll for state. Compressed diffs give agents the equivalent of peripheral awareness: "here's what's new since you last looked."

---

## MCP Tool Interface

### Tool Name
`get_changes_since`

### Parameters

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `checkpoint` | string | No | Last call timestamp | Named checkpoint or ISO timestamp to diff from |
| `include` | string[] | No | All categories | Which categories: `console`, `network`, `websocket`, `dom`, `actions` |
| `severity` | string | No | `"all"` | Minimum severity: `"all"`, `"warnings"`, `"errors_only"` |

### Response

```json
{
  "checkpoint_from": "2026-01-23T10:30:00.000Z",
  "checkpoint_to": "2026-01-23T10:30:45.123Z",
  "duration_ms": 45123,

  "console": {
    "new_errors": [
      {"message": "TypeError: Cannot read property 'id' of undefined", "source": "app.js:42", "count": 1}
    ],
    "new_warnings": [],
    "total_new_entries": 3
  },

  "network": {
    "failures": [
      {"method": "POST", "url": "/api/users", "status": 500, "previous_status": 200}
    ],
    "new_endpoints": [],
    "degraded": [
      {"method": "GET", "url": "/api/projects", "avg_ms": 2100, "previous_avg_ms": 150}
    ],
    "total_new_requests": 5
  },

  "websocket": {
    "disconnections": [],
    "new_connections": [],
    "error_messages": [],
    "total_new_messages": 12
  },

  "dom": {
    "new_error_elements": [
      {"selector": ".error-banner", "text": "Something went wrong", "appeared_at": "2026-01-23T10:30:42.000Z"}
    ],
    "disappeared_elements": [],
    "url_changed": null
  },

  "summary": "1 new console error, 1 network failure (POST /api/users 500), error banner visible",
  "severity": "error",
  "token_count": 287
}
```

### Checkpoint Management

```
// First call — establishes checkpoint
get_changes_since() → returns full current state summary + sets checkpoint "auto_1"

// Subsequent calls — returns only delta
get_changes_since() → returns changes since "auto_1", sets checkpoint "auto_2"

// Named checkpoints
get_changes_since(checkpoint: "before_refactor") → diff from named point

// Explicit timestamp
get_changes_since(checkpoint: "2026-01-23T10:25:00Z") → diff from that time
```

---

## Implementation

### Architecture

```
┌─────────────────────────────────────────────┐
│              Gasoline Server                  │
│                                              │
│  Existing buffers (unchanged):               │
│    logBuffer[]                                │
│    networkBodies[]                            │
│    wsEvents[]                                 │
│    enhancedActions[]                          │
│                                              │
│  New: Checkpoint Registry                    │
│    checkpoints map[string]CheckpointState    │
│    autoCheckpoint *CheckpointState           │
│                                              │
│  New: Diff Engine                            │
│    diffConsole(from, to) → ConsoleDiff       │
│    diffNetwork(from, to) → NetworkDiff       │
│    diffWebSocket(from, to) → WSDiff          │
│    diffDOM(from, to) → DOMDiff               │
│                                              │
└─────────────────────────────────────────────┘
```

### Types

```go
type CheckpointState struct {
    Timestamp     time.Time
    Name          string
    ConsoleCount  int    // Index into log buffer at checkpoint time
    NetworkCount  int    // Index into network buffer
    WSCount       int    // Index into WebSocket buffer
    ActionCount   int    // Index into action buffer
    LastKnownURL  string // Page URL at checkpoint time
    ErrorElements []string // Known error-class elements visible
}

type ChangesSinceResponse struct {
    CheckpointFrom string          `json:"checkpoint_from"`
    CheckpointTo   string          `json:"checkpoint_to"`
    DurationMs     int64           `json:"duration_ms"`
    Console        ConsoleDiff     `json:"console"`
    Network        NetworkDiff     `json:"network"`
    WebSocket      WSDiff          `json:"websocket"`
    DOM            DOMDiff         `json:"dom"`
    Summary        string          `json:"summary"`
    Severity       string          `json:"severity"`  // "clean", "warning", "error"
    TokenCount     int             `json:"token_count"`
}

type ConsoleDiff struct {
    NewErrors      []ConsoleEntry `json:"new_errors"`
    NewWarnings    []ConsoleEntry `json:"new_warnings"`
    TotalNew       int            `json:"total_new_entries"`
}

type NetworkDiff struct {
    Failures       []NetworkChange `json:"failures"`      // Status went to 4xx/5xx
    NewEndpoints   []NetworkEntry  `json:"new_endpoints"` // Never-before-seen URLs
    Degraded       []NetworkChange `json:"degraded"`      // Latency > 3x previous
    TotalNew       int             `json:"total_new_requests"`
}

type NetworkChange struct {
    Method         string `json:"method"`
    URL            string `json:"url"`
    Status         int    `json:"status"`
    PreviousStatus int    `json:"previous_status,omitempty"`
    AvgMs          int    `json:"avg_ms,omitempty"`
    PreviousAvgMs  int    `json:"previous_avg_ms,omitempty"`
}

type WSDiff struct {
    Disconnections []WSConnectionChange `json:"disconnections"`
    NewConnections []WSConnectionChange `json:"new_connections"`
    ErrorMessages  []WSEvent            `json:"error_messages"`
    TotalNew       int                  `json:"total_new_messages"`
}

type DOMDiff struct {
    NewErrorElements     []DOMChange `json:"new_error_elements"`
    DisappearedElements  []DOMChange `json:"disappeared_elements"`
    URLChanged           *URLChange  `json:"url_changed"`
}
```

### Diff Algorithm

For each category, the diff engine:

1. **Console:** Scan log entries after checkpoint index. Classify by level. Deduplicate by message (count occurrences).
2. **Network:** Compare new requests against checkpoint's known endpoints. Flag status changes (was 200, now 500). Flag latency changes (> 3x previous average for same endpoint).
3. **WebSocket:** Report new connection/disconnection lifecycle events. Flag error-type messages.
4. **DOM:** Use pending query mechanism to check for `.error`, `.alert`, `[role=alert]` elements that weren't present at checkpoint. Report URL changes.

### Compression Strategy

The diff is compressed by:
- **Deduplication:** Same error message × 5 → one entry with `count: 5`
- **Aggregation:** 10 network requests to same endpoint → one summary entry
- **Elision:** Unchanged categories return empty objects (omitted from summary)
- **Token counting:** Response includes `token_count` field so the agent can decide if it needs more detail

### Performance Budget

| Operation | Budget | Rationale |
|-----------|--------|-----------|
| Checkpoint creation | < 1ms | Just stores buffer indices |
| Diff computation | < 20ms | Linear scan of buffer slices |
| DOM check (if included) | < 100ms | Uses existing pending query mechanism |
| Total response time | < 150ms | Dominated by optional DOM check |
| Response size | < 2KB typical | Compressed diff, not raw data |

---

## Proving Improvements

### Metrics to Track

| Metric | Baseline (without feature) | Target (with feature) | How to measure |
|--------|---------------------------|----------------------|----------------|
| Tokens per health check | 10-20K (full buffer read) | 200-500 (compressed diff) | Count response tokens |
| Feedback loop latency | 2-5s (read + parse full state) | < 200ms (diff only) | Time from tool call to agent's next action |
| False positive rate | High (agent investigates noise) | < 5% (only new, real signals) | Track agent investigations that lead to no code change |
| Agent edits per session | ~10-20 (slow feedback) | 50+ (fast feedback) | Count successful edit-verify cycles |

### Benchmarks

1. **Token efficiency benchmark:**
   - Simulate 50-edit session
   - Compare total tokens consumed: full-read strategy vs. diff strategy
   - Target: 95% token reduction for state verification

2. **Latency benchmark:**
   - Measure time from `get_changes_since` call to response
   - Compare against `get_browser_logs` + `get_network_bodies` + `get_websocket_events` combined
   - Target: 10x faster response

3. **Signal-to-noise benchmark:**
   - Inject known regressions into a running app
   - Measure detection rate (true positives) and false alarm rate
   - Target: > 95% detection, < 5% false alarms

### A/B Test Design

Run two AI agents on the same set of coding tasks:
- **Agent A:** Uses `get_browser_logs` + `get_network_bodies` after each edit
- **Agent B:** Uses `get_changes_since` after each edit

Measure:
- Task completion rate
- Total tokens consumed
- Time to detect introduced regressions
- Number of unnecessary investigations (false positives)

---

## Integration with Other Features

| Feature | Relationship |
|---------|-------------|
| Noise Filtering | Diffs exclude noise-classified entries |
| Behavioral Baselines | Diffs can reference baseline as comparison point |
| Persistent Memory | Checkpoints can persist across sessions |
| `get_session_health` | Health is a boolean summary; diffs are the structural detail |
| `diagnose_error` | Agent calls `get_changes_since` → sees error → calls `diagnose_error` for causal chain |

---

## Edge Cases

| Case | Handling |
|------|---------|
| Buffer wrapped since checkpoint | Return `"buffer_overflow": true` + best-effort diff from earliest available |
| No changes since checkpoint | Return empty diff with `"severity": "clean"` |
| Checkpoint not found | Return error with available checkpoint list |
| DOM check times out | Return diff without DOM section, note `"dom_unavailable": true` |
| Very long gap (hours) | Cap diff at last 100 entries per category, note truncation |
| Concurrent checkpoints | Each checkpoint is independent; multiple can coexist |
