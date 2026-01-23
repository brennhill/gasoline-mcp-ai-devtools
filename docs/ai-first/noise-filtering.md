# Noise Filtering (`configure_noise`, `dismiss_noise`)

## Status: Specification

---

## Justification

### The Problem

Real browser environments are noisy. A typical development page load produces:

| Source | Example | Relevance |
|--------|---------|-----------|
| Browser extensions | `chrome-extension://... error` | Never relevant |
| Favicon | `GET /favicon.ico → 404` | Cosmetic only |
| React DevTools | `Download the React DevTools...` | Development noise |
| Analytics/tracking | `POST /analytics → network error` | Third-party noise |
| Hot Module Reload | `[HMR] connected`, `[vite] hot updated` | Framework noise |
| CORS preflight | `OPTIONS /api/... → 204` | Infrastructure noise |
| Source maps | `GET /*.map → 404` | Development artifact |
| Service workers | `SW registration failed` | Often expected during dev |

A human developer instantly ignores these. An AI agent cannot distinguish "favicon 404" from "critical API 404" without explicit classification.

### The Cost of Noise

Without filtering:
- **False investigations:** Agent spends tokens investigating irrelevant errors (measured: 30-50% of `diagnose_error` calls are noise-triggered in unfiltered sessions)
- **Alert fatigue:** Agent learns to ignore ALL errors if too many are noise → misses real regressions
- **Token waste:** Noise entries consume diff/health-check token budgets, crowding out real signals
- **Wrong fixes:** Agent may attempt to "fix" framework warnings or extension errors, introducing bugs

### Why This is AI-Critical

Humans develop noise immunity through experience. A senior developer ignores HMR logs reflexively; a junior developer learns which errors matter over weeks. An AI agent has neither experience nor learning — it needs explicit classification, either auto-detected or user/agent-configured.

---

## MCP Tool Interface

### Tool: `configure_noise`

Sets up noise classification rules.

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `action` | string | Yes | — | `"add"`, `"remove"`, `"list"`, `"reset"`, `"auto_detect"` |
| `rules` | NoiseRule[] | For `add` | — | Rules to add |

### Tool: `dismiss_noise`

Quick single-entry dismissal (convenience wrapper).

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `pattern` | string | Yes | — | Message pattern or URL pattern to dismiss |
| `category` | string | No | `"console"` | `"console"`, `"network"`, `"websocket"` |
| `reason` | string | No | — | Why this is noise (for audit trail) |

### NoiseRule Type

```json
{
  "id": "rule_001",
  "category": "console",
  "match": {
    "message_pattern": "Download the React DevTools.*",
    "source_pattern": null,
    "url_pattern": null,
    "level": null
  },
  "classification": "framework_noise",
  "auto_detected": true,
  "created_at": "2026-01-23T10:00:00Z"
}
```

### Response (for `list` action)

```json
{
  "rules": [
    {"id": "auto_1", "category": "console", "match": {"message_pattern": ".*react-devtools.*"}, "classification": "extension", "auto_detected": true},
    {"id": "auto_2", "category": "network", "match": {"url_pattern": ".*favicon\\.ico$"}, "classification": "cosmetic", "auto_detected": true},
    {"id": "user_1", "category": "console", "match": {"source_pattern": "^chrome-extension://.*"}, "classification": "extension", "auto_detected": false}
  ],
  "stats": {
    "total_entries_filtered": 247,
    "entries_that_would_have_been_noise": 189,
    "noise_percentage": 76.5,
    "last_real_signal": "TypeError at app.js:42 (2 minutes ago)"
  }
}
```

### Auto-Detection (`action: "auto_detect"`)

Scans current buffers and proposes noise rules:

```json
{
  "proposed_rules": [
    {
      "rule": {"category": "console", "match": {"source_pattern": "^chrome-extension://.*"}},
      "evidence": "12 entries from 3 different extensions, none application-related",
      "confidence": 0.99
    },
    {
      "rule": {"category": "network", "match": {"url_pattern": ".*/hot-update\\..*"}},
      "evidence": "34 HMR update requests, all 200 status, framework infrastructure",
      "confidence": 0.95
    },
    {
      "rule": {"category": "console", "match": {"message_pattern": "^\\[vite\\].*"}},
      "evidence": "8 Vite HMR log messages, development tooling",
      "confidence": 0.97
    }
  ],
  "auto_applied": true
}
```

---

## Implementation

### Auto-Detection Heuristics

```go
type NoiseDetector struct {
    rules []NoiseHeuristic
}

// Built-in heuristics (not configurable, always active)
var builtinHeuristics = []NoiseHeuristic{
    // Browser extensions
    {Category: "console", SourcePattern: `^chrome-extension://`, Classification: "extension"},
    {Category: "console", SourcePattern: `^moz-extension://`, Classification: "extension"},

    // Favicon
    {Category: "network", URLPattern: `favicon\.ico$`, Classification: "cosmetic"},

    // Source maps
    {Category: "network", URLPattern: `\.map$`, StatusCode: 404, Classification: "development"},

    // HMR / Hot reload
    {Category: "console", MessagePattern: `^\[(HMR|vite|webpack)\]`, Classification: "framework"},
    {Category: "network", URLPattern: `hot-update\.|__webpack_hmr|/_next/webpack`, Classification: "framework"},

    // React DevTools
    {Category: "console", MessagePattern: `Download the React DevTools`, Classification: "framework"},
    {Category: "console", MessagePattern: `react-devtools`, Classification: "extension"},

    // CORS preflight
    {Category: "network", Method: "OPTIONS", StatusRange: [200, 299], Classification: "infrastructure"},

    // Analytics/tracking (common patterns)
    {Category: "network", URLPattern: `(google-analytics|gtag|segment|mixpanel|hotjar|amplitude)`, Classification: "analytics"},

    // Service worker lifecycle
    {Category: "console", MessagePattern: `^(ServiceWorker|SW).*`, Level: "info", Classification: "infrastructure"},
}
```

### Statistical Detection

Beyond built-in heuristics, auto-detection uses statistical analysis:

1. **Frequency analysis:** Entries that repeat > 10 times with identical message are likely noise
2. **Source analysis:** Entries from non-application origins (extensions, CDN, framework internals)
3. **Correlation analysis:** Entries that appear on every page load regardless of user action are environmental
4. **Timing analysis:** Entries that appear before user interaction (during page setup) are likely framework initialization

```go
func (d *NoiseDetector) statisticalDetect(entries []LogEntry) []ProposedRule {
    // Group by message fingerprint (normalize numbers, UUIDs)
    fingerprints := groupByFingerprint(entries)

    proposed := []ProposedRule{}
    for fp, group := range fingerprints {
        // High-frequency, non-error entries are likely noise
        if len(group) > 10 && group[0].Level != "error" {
            proposed = append(proposed, ProposedRule{
                Pattern:    fp,
                Evidence:   fmt.Sprintf("%d identical entries, level=%s", len(group), group[0].Level),
                Confidence: min(0.99, 0.7 + float64(len(group))/100.0),
            })
        }
    }
    return proposed
}
```

### Fingerprinting

Messages are fingerprinted by normalizing dynamic content:

```
"Failed to load resource: /api/users/123"     → "Failed to load resource: /api/users/{id}"
"[HMR] Updated module: ./src/App.tsx (hash: abc123)" → "[HMR] Updated module: {path} (hash: {hash})"
"WebSocket connected at 1706012345678"        → "WebSocket connected at {timestamp}"
```

This groups semantically-identical messages for frequency analysis.

### Storage

```go
type NoiseConfig struct {
    Rules       []NoiseRule       `json:"rules"`
    Stats       NoiseStats        `json:"stats"`
    AutoApplied bool              `json:"auto_applied"`
    LastUpdate  time.Time         `json:"last_update"`
}

type NoiseStats struct {
    TotalFiltered    int64  `json:"total_filtered"`
    FilteredByRule   map[string]int64 `json:"filtered_by_rule"` // rule_id → count
    LastRealSignal   *time.Time `json:"last_real_signal"`
}
```

Noise config is stored in-memory and optionally persisted to disk (see Persistent Memory spec).

### Integration Points

All existing and new tools respect noise filtering:

| Tool | How noise is filtered |
|------|----------------------|
| `get_changes_since` | Noise entries excluded from diffs |
| `get_session_health` | Noise entries excluded from health assessment |
| `get_browser_logs` | Optional `exclude_noise: true` parameter (default: true for AI, false for raw access) |
| `get_network_bodies` | Noise URLs excluded unless explicitly requested |
| `diagnose_error` | Noise entries excluded from causal chain |
| `generate_test` | Noise patterns added to test's ignore list automatically |

---

## Proving Improvements

### Metrics

| Metric | Baseline | Target | Measurement |
|--------|----------|--------|-------------|
| Signal-to-noise ratio in diffs | ~30% signal (rest is noise) | > 90% signal | Classify each diff entry as signal/noise by human audit |
| False investigation rate | ~40% of agent investigations are noise-triggered | < 5% | Track investigations that result in no code change |
| Tokens wasted on noise | ~5K tokens/session reading noise | < 200 tokens/session | Count tokens for noise-classified entries |
| Time to detect real regression | Delayed by noise investigations | No delay (noise pre-filtered) | Time from regression introduction to agent's fix attempt |
| Auto-detection accuracy | N/A | > 90% precision, > 80% recall | Compare auto-detected rules against human-labeled noise |

### Benchmark: Noise Injection Test

1. Set up a real development environment (React + Vite + Chrome extensions)
2. Count total console/network entries during a 5-minute development session
3. Manually label each entry as signal or noise
4. Run auto-detection
5. Measure precision (correct noise classifications / total classifications) and recall (detected noise / actual noise)

**Target:** 90% precision (don't accidentally filter real errors), 80% recall (catch most noise).

### Benchmark: Agent Efficiency

1. Run AI agent on 10 coding tasks with noise filtering OFF
2. Run same agent on same tasks with noise filtering ON
3. Measure:
   - Number of wasted `diagnose_error` calls (investigating noise)
   - Total tokens consumed on state reading
   - Time to complete each task
   - Regression detection latency

---

## Edge Cases

| Case | Handling |
|------|---------|
| Real error matches noise pattern | `severity: "error"` entries have higher bar: only filtered if source is definitively non-application |
| New framework not in heuristics | Statistical detection catches high-frequency patterns; user can add rules manually |
| Noise rule filters real regression | Each filtered entry is counted; sudden spike in filtered entries triggers a "noise rule may be stale" warning |
| Too many rules (performance) | Max 100 rules; compiled to single regex per category for O(1) matching |
| Rule conflicts (both match and don't match) | Most specific rule wins (URL+message > URL-only > message-only) |

---

## Security Considerations

- Noise rules cannot be configured to hide security-relevant events
- `security_audit` tool (v6 Phase 1) ignores noise filtering — it always scans raw data
- Auth-related network failures (401, 403) are never auto-classified as noise
- Rules are logged for audit trail (who added, when, why)
