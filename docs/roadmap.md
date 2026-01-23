---
title: "Roadmap — AI-First Features"
description: "Upcoming Gasoline features built for AI-native development: compressed state diffs, noise filtering, behavioral baselines, persistent memory, API schema inference, and DOM fingerprinting."
keywords: "AI-first debugging, AI coding agent features, compressed state diffs, noise filtering, behavioral baselines, persistent memory, API schema inference, DOM fingerprinting, token-efficient debugging"
permalink: /roadmap/
header:
  overlay_image: /assets/images/hero-banner.png
  overlay_filter: 0.85
  excerpt: "What's next: features designed for a world where AI agents are the primary coders."
toc: true
toc_sticky: true
---

These features are designed for the next generation of AI coding — where agents run tight edit-verify loops, need peripheral awareness, and accumulate understanding over time.

## <i class="fas fa-bolt"></i> Compressed State Diffs

**Status:** Specification Complete
{: .notice--info}

### The Problem

AI agents in edit-verify loops waste 10-20K tokens per state check, re-reading entire log buffers to find what changed. Over a 50-edit session, that's 500K-1M tokens wasted on state reads.

### The Solution

`get_changes_since` returns only what changed since the last check — token-efficient deltas instead of full state dumps.

```json
{
  "summary": "1 new console error, 1 network failure (POST /api/users 500), error banner visible",
  "severity": "error",
  "token_count": 287
}
```

**Target:** 95% token reduction for state verification. 10x faster response. < 5% false alarm rate.

---

## <i class="fas fa-filter"></i> Noise Filtering

**Status:** Specification Complete
{: .notice--info}

### The Problem

A typical page load produces dozens of irrelevant entries — extension errors, favicon 404s, HMR logs, analytics failures. Humans ignore these reflexively. AI agents can't distinguish "favicon 404" from "critical API 404" without explicit classification.

### The Solution

`configure_noise` and `dismiss_noise` classify noise automatically. Built-in heuristics catch common patterns (extensions, HMR, analytics). Statistical detection catches the rest.

Auto-detection proposes rules with confidence scores:

```json
{
  "rule": {"category": "console", "match": {"source_pattern": "^chrome-extension://.*"}},
  "evidence": "12 entries from 3 extensions, none application-related",
  "confidence": 0.99
}
```

**Target:** 90% precision (don't filter real errors), 80% recall (catch most noise). < 5% false investigation rate.

---

## <i class="fas fa-chart-bar"></i> Behavioral Baselines

**Status:** Specification Complete
{: .notice--info}

### The Problem

AI agents don't know what "normal" looks like for your app. When they see 3 network requests taking 200ms each, they can't tell if that's fast or slow for your system. They need a reference point.

### The Solution

`save_baseline` captures what "correct" looks like. `compare_baseline` detects regressions against that reference — without needing explicit test assertions.

Use cases:
- Save baseline after fixing a bug → detect if it regresses
- Save baseline for production behavior → detect drift in development
- Save baseline for performance → detect latency regressions

---

## <i class="fas fa-brain"></i> Persistent Memory

**Status:** Specification Complete
{: .notice--info}

### The Problem

Every AI session starts from scratch. The agent re-discovers which errors are noise, re-learns API schemas, and re-investigates the same false positives. There's no continuity between sessions.

### The Solution

`session_store` and `load_session_context` give agents persistent memory across sessions:

- Noise rules persist (don't re-learn what's irrelevant)
- API schemas persist (don't re-infer structure)
- Baselines persist (regression detection works across days)
- Known errors persist (don't re-investigate the same issue)

---

## <i class="fas fa-project-diagram"></i> API Schema Inference

**Status:** Specification Complete
{: .notice--info}

### The Problem

AI agents need to understand your API contracts to debug integration issues. Today they read documentation (if it exists) or guess from error messages. Neither is reliable.

### The Solution

`get_api_schema` learns API contracts from observed traffic — request/response shapes, status code patterns, and timing characteristics. Your AI knows the API without reading docs.

```json
{
  "endpoint": "POST /api/users",
  "request_shape": {"email": "string", "name": "string", "role": "enum(admin,user)"},
  "response_shapes": {
    "201": {"id": "number", "email": "string", "created_at": "datetime"},
    "422": {"errors": {"field": "string"}}
  },
  "avg_latency_ms": 145
}
```

---

## <i class="fas fa-fingerprint"></i> DOM Fingerprinting

**Status:** Specification Complete
{: .notice--info}

### The Problem

Verifying UI correctness typically requires vision models or screenshot comparison — both expensive and brittle. An agent needs a way to structurally verify "the page looks right" without pixel comparison.

### The Solution

`get_dom_fingerprint` and `compare_dom_fingerprint` create structural hashes of the page:

- Detect unexpected DOM changes (elements added/removed/reordered)
- Verify component rendering without screenshots
- Catch CSS-invisible regressions (wrong structure, correct appearance)
- Works as a component of baselines and diffs

---

## <i class="fas fa-globe"></i> Platform Expansion

**Status:** Future
{: .notice}

### Firefox Extension

The Chrome extension's WebExtensions API is ~90% compatible with Firefox. Main porting work: service worker → event page, `chrome.scripting` API differences.

**Effort:** Low-medium. 1-2 days of porting + testing.

**Note:** Edge, Brave, Arc, Vivaldi, and Opera already work — they're Chromium-based and run the Chrome extension unmodified.

### React Native

Tap into React Native's debug bridge to capture LogBox errors, network requests, and component tree state. Forward to the Gasoline MCP server over the local network.

**Effort:** Medium. New companion package, not an extension.

### Flutter

Dart DevTools extension or `debugPrint` interceptor that forwards runtime events to the Gasoline MCP server.

**Effort:** Medium. Dart package + DevTools integration.

### Native iOS / Android

Stream system logs (`os_log` on iOS, Logcat on Android) to the Gasoline MCP server via a CLI companion tool. Zero app modification required — purely observational.

**Effort:** Low per platform. CLI tool that pipes structured log output to the existing server.

---

## Priority Order

| # | Feature | Why First |
|---|---------|-----------|
| 1 | Compressed Diffs | Unblocks tight feedback loops (token efficiency) |
| 2 | Noise Filtering | Makes all other signals useful (reduces false positives) |
| 3 | Behavioral Baselines | Enables regression detection without tests |
| 4 | Persistent Memory | Agent accumulates understanding over time |
| 5 | API Schema Inference | Agent understands the system without docs |
| 6 | DOM Fingerprinting | Structural UI verification without vision models |

## <i class="fas fa-shield-alt"></i> Engineering Resilience

**Status:** Planned
{: .notice}

Infrastructure hardening that prevents regressions, catches integration drift, and enforces invariants mechanically — without burning agent context.

### Contract & Schema Validation

The extension-to-server interface has an implicit contract (`POST /logs` expects `{ entries: [{ level, message, timestamp, ... }] }`). Make it explicit:

- JSON Schema files defining every HTTP endpoint's request/response format
- Go struct validation that rejects malformed entries at the boundary (not `map[string]interface{}`)
- Extension-side contract tests that verify emitted payloads match the schema
- Shared schema file that both Go and JS tests validate against

**Why:** Prevents silent data corruption when either side drifts. An AI modifying `inject.js` can't accidentally break the server contract without CI catching it.

### End-to-End Integration Tests

All current tests are unit tests (mocked Chrome APIs, `httptest` recorders). Nothing exercises the full pipeline:

```
Browser page → inject.js → content.js → background.js → HTTP POST → Go server → MCP tool response
```

- Playwright-based E2E suite that starts a real server, loads the real extension, and verifies MCP tool output
- Covers: console capture, network errors, WebSocket events, DOM queries, accessibility audits, screenshots
- Runs in CI with `xvfb-run` for headless extension loading

**Why:** Unit tests can't catch message-passing bugs, serialization mismatches, or timing issues between components.

### Zero-Dependency Verification

The Go server's zero-dependency guarantee is documented but not enforced:

- CI step that parses `go.mod` and fails if any `require` directive exists
- CI step that verifies `go.sum` is empty or absent
- Extension check: no `node_modules` imports, no CDN script tags (except optional axe-core)

**Why:** A single accidental `import "github.com/..."` breaks the "single binary, no supply chain risk" promise.

### Typed Response Structs

MCP tool responses currently use `map[string]interface{}`:

```go
// Current (fragile):
result := map[string]interface{}{"entries": entries, "count": len(entries)}

// Target (typed):
type GetLogsResult struct {
    Entries []LogEntry `json:"entries"`
    Count   int        `json:"count"`
}
```

- Replace all MCP tool response construction with typed structs
- Compiler catches missing fields, typos, type mismatches
- JSON tags serve as documentation of the wire format

**Why:** `map[string]interface{}` silently accepts any structure. Typed structs make response format changes a compile error.

### Performance Benchmarks

SLOs are documented but not enforced in CI:

- Go benchmarks for hot paths: `addEntries`, `getEntries`, WebSocket event ingestion, `/snapshot` aggregation
- Baseline file checked into repo (`benchstat` format)
- CI step that runs benchmarks and fails on > 20% regression
- Extension: Playwright performance measurement for intercept overhead

**Why:** Performance regressions are invisible without measurement. A seemingly-innocent refactor can 10x the cost of a hot path.

### Race Detection

Go tests currently run with `go test -v` — no race detector. Add `-race` flag:

```makefile
test:
    CGO_ENABLED=1 go test -race -v ./cmd/dev-console/...
```

- Catches data races in concurrent buffer access under real load patterns
- Server uses `sync.RWMutex` everywhere but `-race` catches any missed paths
- Runs in CI on every push (adds ~30% test runtime)

**Why:** Race conditions are the hardest bugs to reproduce. The race detector catches them deterministically at test time.

### Test Coverage Gate

No coverage measurement exists. Add threshold enforcement:

- `go test -coverprofile=coverage.out` in CI
- Fail if total coverage drops below 70%
- Report per-package coverage breakdown
- Track coverage trend over time (no ratchet — just floor)

**Why:** Prevents large code additions with zero test coverage from merging.

### Fuzz Testing

Go's built-in fuzz testing (since 1.18) for HTTP input parsing:

- Fuzz `POST /logs` with arbitrary JSON → must never panic
- Fuzz `POST /websocket-events` → must never panic
- Fuzz `POST /network-bodies` → must never panic
- Fuzz MCP JSON-RPC request parsing → must never panic
- Corpus seeded with real-world payloads

**Why:** Manual test cases can't cover all malformed input combinations. Fuzzing finds panics on edge-case JSON that would crash the server in production.

### Binary Size Gate

The Go binary should stay small (currently ~8MB). Add a CI size check:

- Fail if binary exceeds 15MB (indicates dependency smuggling or bloat)
- Track size per-commit for trend detection
- Separate check per platform (cross-compilation shouldn't inflate)

**Why:** Binary size is a proxy for dependency smuggling. A jump from 8MB to 25MB means something got linked in.

### Import Path Verification

Stronger than zero-dep check — verify every Go import is from stdlib:

```bash
go list -f '{{join .Imports "\n"}}' ./cmd/dev-console/ | grep "\." && exit 1
```

- Catches internal package paths that might pull in transitive deps
- Runs alongside the `go.sum` absence check
- Extension equivalent: verify no `import` statements reference `node_modules`

**Why:** `go.mod` can be clean while code still imports something that triggers `go mod tidy` to add a dep on next build.

### Goroutine Leak Detection

After tests complete, verify no goroutines leaked:

- `TestMain` wrapper checks goroutine count before/after test suite
- Allow small delta (±5) for runtime internals
- Fail loudly if goroutines accumulate (indicates unclosed HTTP connections, blocked channels)

**Why:** Leaked goroutines accumulate over server lifetime. In long-running dev sessions, they cause memory growth and eventual OOM.

### Response Snapshot Tests

Golden file comparison for every MCP tool response:

- Serialize each MCP tool's response to JSON
- Compare against checked-in `testdata/*.golden.json` files
- `go test -update` flag regenerates goldens intentionally
- Any unintentional format change fails CI

**Why:** Typed structs prevent wrong types, but golden files catch unintentional field additions, removals, or ordering changes that break MCP clients.

---

## <i class="fas fa-dollar-sign"></i> Economic Impact

Combined value per developer per year:

| Savings Source | Estimated Value |
|---------------|----------------|
| Token reduction (compressed diffs) | $3,600-4,800/year |
| Time saved (faster feedback loops) | $12,480/year |
| Fewer false positives (noise filtering) | $4,160/year |
| No re-investigation (persistent memory) | $4,648-5,570/year |
| **Total** | **$24,888-27,010/year** |

Zero cost. Open source. Replaces $65-90K/year commercial alternatives.
