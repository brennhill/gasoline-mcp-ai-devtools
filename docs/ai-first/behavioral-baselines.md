# Behavioral Baselines (`save_baseline`, `compare_baseline`)

## Status: Specification

---

## Justification

### The Problem

An AI agent completes a feature. It works. The agent moves to the next task. Three edits later, the first feature silently breaks — a shared dependency changed, a CSS class was renamed, an API endpoint was refactored.

Without baselines, the agent has no reference for "what did correct behavior look like?" It can detect new errors (via `get_changes_since`), but it cannot detect *regressions* — things that used to work but no longer do.

### The Analogy

Baselines are to AI agents what **snapshot tests** are to code — but for runtime behavior rather than rendered output. They capture the full behavioral fingerprint of a working feature:

- Which network endpoints were called and what they returned
- What the console state was (clean, or known acceptable warnings)
- What DOM elements were visible and interactive
- What WebSocket messages were exchanged
- How long things took

### Why This is AI-Critical

- **Humans remember:** "The login page used to redirect to /dashboard after submit." An AI agent cannot remember across context windows.
- **Tests are late:** `generate_test` locks in behavior as a Playwright test, but tests run in CI — not during the active development session. Baselines provide *immediate* regression detection during development.
- **Compound coverage:** Each saved baseline adds to the regression surface the agent monitors. Over time, the agent accumulates understanding of what "correct" means for the entire application.

### The Difference from Tests

| | Behavioral Baselines | Generated Tests |
|---|---|---|
| When checked | During development (every edit) | In CI (after commit) |
| Effort to create | Zero (just `save_baseline` when it works) | Requires `generate_test` + `validate_test` |
| What they verify | "Same network/console/DOM pattern" | "Full user flow with assertions" |
| Granularity | Per-feature behavioral snapshot | Per-flow action sequence |
| Persistence | Across sessions (disk) | Across deploys (source control) |

They're complementary: baselines catch regressions during development, tests catch them in CI.

---

## MCP Tool Interface

### Tool: `save_baseline`

Captures current browser behavioral state as a named baseline.

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `name` | string | Yes | — | Descriptive name (e.g., `"login-flow"`, `"dashboard-load"`) |
| `description` | string | No | — | What this baseline represents |
| `capture` | object | No | All enabled | Which signals to include (see below) |
| `url_scope` | string | No | Current URL | URL pattern this baseline applies to |
| `overwrite` | bool | No | `false` | Replace existing baseline with same name |

### `capture` Object

```json
{
  "network": true,
  "console": true,
  "dom_structure": true,
  "websocket": true,
  "timing": true
}
```

### Tool: `compare_baseline`

Compares current browser state against a saved baseline.

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `name` | string | Yes | — | Baseline name to compare against |
| `tolerance` | object | No | Defaults below | Acceptable deviation thresholds |

### `tolerance` Object

```json
{
  "timing_factor": 3.0,
  "allow_additional_network": true,
  "allow_additional_console_info": true,
  "ignore_dynamic_values": true
}
```

### Tool: `list_baselines`

Lists all saved baselines with metadata.

No parameters.

### Tool: `delete_baseline`

Removes a saved baseline.

| Parameter | Type | Required |
|-----------|------|----------|
| `name` | string | Yes |

---

## Baseline Structure

### What Gets Captured

```go
type Baseline struct {
    Name        string          `json:"name"`
    Description string          `json:"description,omitempty"`
    CreatedAt   time.Time       `json:"created_at"`
    UpdatedAt   time.Time       `json:"updated_at"`
    URLScope    string          `json:"url_scope"`
    Version     int             `json:"version"` // Incremented on overwrite

    Network     NetworkBaseline `json:"network"`
    Console     ConsoleBaseline `json:"console"`
    DOM         DOMBaseline     `json:"dom"`
    WebSocket   WSBaseline      `json:"websocket"`
    Timing      TimingBaseline  `json:"timing"`
}

type NetworkBaseline struct {
    Endpoints []EndpointSnapshot `json:"endpoints"`
}

type EndpointSnapshot struct {
    Method        string            `json:"method"`
    URLPattern    string            `json:"url_pattern"`    // Path with IDs normalized
    Status        int               `json:"status"`
    ResponseShape map[string]string `json:"response_shape"` // top-level property → type
    AvgLatencyMs  int               `json:"avg_latency_ms"`
    ContentType   string            `json:"content_type"`
}

type ConsoleBaseline struct {
    ErrorCount   int      `json:"error_count"`   // Expected errors (usually 0)
    WarningCount int      `json:"warning_count"` // Expected warnings
    KnownMessages []string `json:"known_messages"` // Fingerprints of acceptable messages
}

type DOMBaseline struct {
    URL              string   `json:"url"`
    Title            string   `json:"title"`
    InteractiveElements []DOMElement `json:"interactive_elements"`
    Headings         []string `json:"headings"`
    ErrorElements    []string `json:"error_elements"` // Usually empty
    FormCount        int      `json:"form_count"`
    LinkCount        int      `json:"link_count"`
}

type WSBaseline struct {
    Connections []WSConnectionSnapshot `json:"connections"`
}

type WSConnectionSnapshot struct {
    URLPattern    string            `json:"url_pattern"`
    MessageShapes map[string][]string `json:"message_shapes"` // direction → top-level keys
    ExpectedOpen  bool              `json:"expected_open"`
}

type TimingBaseline struct {
    PageLoadMs    int `json:"page_load_ms"`
    DOMReadyMs    int `json:"dom_ready_ms"`
    FirstPaintMs  int `json:"first_paint_ms"`
    NetworkP95Ms  int `json:"network_p95_ms"`
}
```

### URL Normalization

Dynamic segments in URLs are normalized for stable comparison:

```
/api/users/550e8400-e29b-41d4-a716-446655440000 → /api/users/{uuid}
/api/projects/42/tasks/7                          → /api/projects/{id}/tasks/{id}
/api/search?q=hello&page=2                        → /api/search (query params stripped)
```

---

## Comparison Algorithm

### `compare_baseline` Response

```json
{
  "baseline": "login-flow",
  "status": "regression",
  "regressions": [
    {
      "category": "network",
      "severity": "error",
      "description": "POST /api/auth now returns 500 (baseline: 200)",
      "baseline_value": {"status": 200, "response_shape": ["token", "user"]},
      "current_value": {"status": 500, "response_shape": ["error", "message"]}
    },
    {
      "category": "dom",
      "severity": "warning",
      "description": "Missing interactive element: button 'Sign In'",
      "baseline_value": {"element": "button", "text": "Sign In", "visible": true},
      "current_value": null
    }
  ],
  "improvements": [
    {
      "category": "timing",
      "description": "Page load 40% faster (was 1200ms, now 720ms)"
    }
  ],
  "unchanged": ["console", "websocket"],
  "summary": "2 regressions (1 error, 1 warning), 1 improvement"
}
```

### Comparison Rules

| Category | Regression if... | Tolerance |
|----------|-----------------|-----------|
| Network status | Status code changed to 4xx/5xx | `allow_additional_network` permits new endpoints |
| Network shape | Top-level properties disappeared | New properties are OK (additive is fine) |
| Network timing | Latency > `timing_factor` × baseline | Default 3x (e.g., 150ms baseline → regression at 450ms) |
| Console errors | Error count increased | `allow_additional_console_info` permits new info-level |
| DOM elements | Expected interactive elements missing | New elements are OK |
| DOM errors | Error elements appeared (were absent in baseline) | — |
| WebSocket | Expected connection not open | — |
| WS messages | Expected message shape keys missing | New keys are OK |

### Dynamic Value Handling

When `ignore_dynamic_values: true` (default):
- UUIDs, timestamps, session tokens in URLs are normalized
- Response body *values* are not compared (only *shape*)
- DOM text content with numbers/dates is pattern-matched, not exact-matched

---

## Storage

### On-Disk Format

```
~/.gasoline/baselines/
├── login-flow.json
├── dashboard-load.json
├── checkout-flow.json
└── _index.json          # Baseline registry with metadata
```

### Size Limits

| Limit | Value | Rationale |
|-------|-------|-----------|
| Max baselines | 50 | Prevent unbounded disk growth |
| Max baseline size | 100KB | Network shapes can be large |
| Total storage | 5MB | Reasonable for development tooling |
| Eviction | LRU (least recently compared) | Keep actively-used baselines |

---

## Proving Improvements

### Metrics

| Metric | Baseline (no baselines) | Target (with baselines) | Measurement |
|--------|------------------------|------------------------|-------------|
| Regression detection time | Discovered when feature is re-tested manually | < 30s after regression-introducing edit | Time from breaking edit to `compare_baseline` alert |
| Regressions shipped to CI | ~30% of commits have test failures | < 5% (caught during dev) | CI test failure rate on commits from agent |
| Agent self-correction rate | Low (doesn't know something broke) | > 80% (detects and fixes before commit) | Ratio of regressions caught/fixed during session |
| Baseline creation overhead | N/A | < 50ms per baseline save | Benchmark `save_baseline` response time |

### Benchmark: Regression Detection

1. Agent builds 5 features, saving a baseline after each
2. Introduce 10 regressions across the features (API changes, DOM removals, new errors)
3. Agent calls `compare_baseline` for each
4. Measure:
   - Detection rate (how many of 10 caught?)
   - False positive rate (how many non-regressions flagged?)
   - Time from regression to detection

**Target:** > 90% detection rate, < 10% false positive rate, < 1s detection time.

### Benchmark: Compound Value

1. Simulate 10-session development arc (agent works on app over "days")
2. Accumulate baselines across sessions
3. In session 10, make breaking changes
4. Compare: agent with 10 sessions of baselines vs. agent with no history

**Target:** Agent with baselines catches 5x more regressions than agent without.

---

## Workflow Examples

### Basic: Save and Compare

```
Agent builds user registration:
  → Page loads, form renders, POST /api/users returns 201
  → save_baseline("user-registration")

Agent later refactors auth module:
  → compare_baseline("user-registration")
  → "POST /api/users now returns 500 (baseline: 201)"
  → Agent investigates and fixes the regression
```

### Advanced: Multi-Feature Safety Net

```
Session 1: Agent builds login      → save_baseline("login")
Session 2: Agent builds dashboard  → save_baseline("dashboard")
Session 3: Agent builds checkout   → save_baseline("checkout")
Session 4: Agent refactors shared auth module
  → compare_baseline("login")     → PASS
  → compare_baseline("dashboard") → REGRESSION: missing user menu
  → compare_baseline("checkout")  → REGRESSION: 401 on payment endpoint
  → Agent fixes both before committing
```

### Integration with generate_test

```
Agent builds feature → save_baseline("feature-x")
Agent: "Now lock this in as a test"
  → generate_test(last_n_actions: 10) → Playwright test
  → validate_test("feature-x.spec.ts") → PASS
  → Commit test + code

Later:
  → compare_baseline("feature-x") → catches runtime regressions (dev-time)
  → CI runs feature-x.spec.ts     → catches regressions (CI-time)
  → Double coverage: baselines are fast/cheap, tests are thorough/portable
```

---

## Edge Cases

| Case | Handling |
|------|---------|
| App intentionally changed (baseline stale) | Agent calls `save_baseline` with `overwrite: true` to update |
| Multiple pages share a baseline name | `url_scope` restricts where comparison is valid |
| Baseline captures transient state | Agent should save baseline after page is fully loaded (wait for network idle) |
| API returns different valid data | Shape comparison (not value) handles this; only structural changes flag |
| Baseline for deleted feature | `list_baselines` shows last-compared date; agent or user prunes stale ones |
| Network endpoint has variable latency | `timing_factor` tolerance (default 3x) accommodates variance |
