---
title: "Roadmap"
description: "Gasoline roadmap: endpoint catalog, time-windowed diffs, noise dismissal, performance budgets, and infrastructure hardening."
keywords: "gasoline roadmap, endpoint catalog, compressed diffs, noise filtering, performance budget, MCP browser debugging"
permalink: /roadmap/
header:
  overlay_image: /assets/images/hero-banner.png
  overlay_filter: 0.85
  excerpt: "What's next: capture more, interpret less."
toc: true
toc_sticky: true
---

Every feature follows the [product philosophy](/docs/product-philosophy/): capture and organize for AI agents, but keep output human-verifiable. We don't interpret data the AI reads better than us.

---

## Phase 1: Discovery & Infrastructure

Low effort, high value. Ship first.

### <i class="fas fa-sitemap"></i> Endpoint Catalog

**Status:** Specified
{: .notice--info}

`get_endpoint_catalog` — list every API endpoint the app talks to, with call counts, status codes, and latency.

```json
{
  "endpoints": [
    {"method": "GET", "path": "/api/users", "status_codes_seen": [200, 401], "call_count": 47, "avg_latency_ms": 142},
    {"method": "POST", "path": "/api/users", "status_codes_seen": [201, 422], "call_count": 3, "avg_latency_ms": 289}
  ]
}
```

**Why:** AI agents can analyze JSON, but they can't discover endpoints they haven't seen. This gives the agent a map of the API surface in one call. No type inference, no schema learning — just aggregated facts.

**Effort:** ~150 lines Go. Zero extension changes.

---

### <i class="fas fa-shield-alt"></i> Infrastructure Hardening

**Status:** Partially complete (circuit breaker done in extension)
{: .notice--info}

- **Server rate limiting (429)** — Reject > 1000 events/sec
- **Memory enforcement** — Automatic buffer clearing when limits hit
- **Interception deferral** — Enforce post-`load` + 100ms delay for v4 intercepts

**Why:** Reliability. Without these, the extension can overwhelm the server or degrade the page.

**Effort:** Small, targeted fixes. Must-do before new features.

---

### <i class="fas fa-tachometer-alt"></i> Performance Capture Basics

**Status:** Partially implemented
{: .notice--info}

- Include FCP/LCP/CLS in performance snapshots (observers exist, values aren't sent)
- Resource fingerprint in snapshots (top-20-by-size for causal diffing)
- URL path normalization (`/users/123` → `/users/:id`)

**Why:** Pure capture. The extension already observes these metrics — we're just not sending them to the server yet.

**Effort:** Small. Extension + server changes.

---

## Phase 2: Agent Efficiency

Medium effort. Ship after Phase 1.

### <i class="fas fa-bolt"></i> Time-Windowed Diffs

**Status:** Redesigning (simplified from original spec)
{: .notice--warning}

`get_changes_since` — return only log entries, network events, and WebSocket messages that arrived after a given checkpoint.

```json
{
  "checkpoint_from": "2026-01-23T10:30:00.000Z",
  "checkpoint_to": "2026-01-23T10:30:45.123Z",
  "console": {
    "new_entries": [
      {"level": "error", "message": "TypeError: Cannot read property 'id' of undefined", "source": "app.js:42"}
    ]
  },
  "network": {
    "new_failures": [
      {"method": "POST", "url": "/api/users", "status": 500}
    ]
  }
}
```

**Why:** Agents re-reading full buffers waste tokens. Time-windowed filtering is pure aggregation — just "show me what's new." No summaries, no severity classification, no token counting. The AI summarizes.

**Design note:** Original spec included `summary`, `severity`, and `token_count` fields. These are interpretation — removed. The checkpoint mechanism is the value.

**Effort:** ~200 lines Go.

---

### <i class="fas fa-filter"></i> Noise Dismissal

**Status:** Redesigning (simplified from original spec)
{: .notice--warning}

`dismiss_noise` — agent-driven pattern exclusion. The agent decides what's noise, Gasoline applies the filter to future reads.

```json
{"pattern": "chrome-extension://.*", "category": "console", "reason": "Browser extension logs"}
```

**Why:** Reduces tokens on subsequent tool calls by excluding entries the agent has already classified as irrelevant. The agent makes the judgment — we just apply it.

**Design note:** Original spec included `auto_detect` with confidence scores. Removed — the AI already knows extension errors are noise. The value is in *applying* the exclusion, not detecting it.

**Effort:** ~100 lines Go.

---

### <i class="fas fa-chart-line"></i> Performance Diffing

**Status:** Specified
{: .notice--info}

- Resource fingerprint comparison (added/grew/slowed/removed)
- AI auto-check hints in tool descriptions ("call after code changes")
- Cross-tool regression warnings

**Why:** Structured comparison of observable data. No judgment about whether performance is "good" — just "here's what changed."

**Effort:** Medium. Builds on Phase 1 perf basics.

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

Low effort. Ship when convenient.

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

Canonical first-call sequence for AI agents connecting to Gasoline. Reduces wasted tokens on discovery.

Not code — just a recommended tool call sequence baked into tool descriptions or an MCP resource.

**Effort:** Documentation only.

---

### <i class="fas fa-bookmark"></i> Named Checkpoints

`create_checkpoint` — MCP tool to create named markers for `get_changes_since`.

```
create_checkpoint(name: "before_refactor")
// ... make changes ...
get_changes_since(checkpoint: "before_refactor")
```

**Why:** Pure utility. 20 lines of code (map of name → timestamp).

**Effort:** Trivial. Ships with time-windowed diffs.

---

## Deferred

### <i class="fas fa-brain"></i> Persistent Memory

Cross-session storage for noise rules and other state.

**Why deferred:** Without noise dismissal shipping first, there's nothing to persist. If noise rules prove useful, a simple `save_noise_config` / `load_noise_config` tool pair covers the use case without building a generic persistence framework.

**Revisit:** After noise dismissal has real-world usage data.

---

### <i class="fas fa-stream"></i> Context Streaming

Push significant browser events via MCP notifications.

**Why deferred:** MCP notification support in AI clients (Claude Code, Cursor, Windsurf) is not mature enough. Building for a spec with no consumers is premature. The same value is achieved by agents calling `get_changes_since` in their feedback loop.

**Revisit:** When MCP notification handling is reliable across major clients.

---

## Killed

### ~~Behavioral Baselines~~

`save_baseline` / `compare_baseline` — snapshot browser state, detect regressions.

**Why killed:**
- **Interprets rather than reports.** Tolerance thresholds (`timing_tolerance_percent: 20`) are judgments Gasoline shouldn't make. The AI compares states better than hardcoded thresholds.
- **Solved by simpler tools.** Time-windowed diffs + the agent's own context = regression detection without a baseline system.
- **Session lifetime mismatch.** Baselines assume long sessions with save/edit/compare cycles. Most real sessions don't have that lifecycle.
- **Stale cross-session.** Yesterday's baseline breaks on legitimate API changes today.

### ~~API Schema Inference~~

Removed — AI reads JSON natively. Inferred types add noise, not signal.

### ~~DOM Fingerprinting~~

Removed — opaque hashes aren't human-verifiable. DOM queries already provide discovery.

---

## Priority Summary

| # | Feature | Effort | Value | Status |
|---|---------|--------|-------|--------|
| 1 | Endpoint Catalog | Low | High | Specified |
| 2 | Infrastructure (429, memory, deferral) | Low | High | Partial |
| 3 | Perf capture basics (FCP/LCP/CLS) | Low | Medium | Partial |
| 4 | Time-Windowed Diffs | Medium | Medium | Redesigning |
| 5 | Noise Dismissal | Low | Medium-High | Redesigning |
| 6 | Performance Diffing | Medium | Medium | Specified |
| 7 | Workflow Recipe | Negligible | Low-Medium | To specify |
| 8 | Named Checkpoints | Trivial | Medium | To specify |
| — | Persistent Memory | High | Low (now) | Deferred |
| — | Context Streaming | High | Zero (now) | Deferred |
| 9 | Sparky Rollout (Wave 1) | Low | High | Planned |
| 10 | Sparky Rollout (Waves 2-4) | Medium | Medium | Planned |
| ~~—~~ | ~~Behavioral Baselines~~ | High | Low | Killed |

---

## <i class="fas fa-fire"></i> Marketing & Brand: Sparky Rollout

**Status:** Planned
{: .notice}

Sparky is the Gasoline mascot — a friendly anthropomorphic salamander with fire-colored gradient skin and thick outlines, distinct from the abstract logo flame. See [Brand Guidelines](/brand/#mascot-sparky) for design specs. Minimum size: 48px. Only appears where there's room for a pose.

### Wave 1: High-Traffic Pages

| Page | Location | Pose |
|------|----------|------|
| Homepage | Hero section, beside "Gasoline" title | Leaning against the "G", arms crossed, one eyebrow raised |
| Homepage | Pipeline diagram, in the arrow between Server → AI | Riding the arrow like a log flume, arms up |
| Getting Started | "Ignite the Server" header | Striking a match on his own head |
| Getting Started | "Verify the Flame" header | Peeking out from behind a terminal, thumbs-up |
| Getting Started | Bottom of page | Chef's kiss, tiny chef hat — "now you're cooking" |
| 404 Page | Center, large | Holding a crumpled map upside-down, flame flickering low |

### Wave 2: Feature & Trust Pages

| Page | Location | Pose |
|------|----------|------|
| Features | Page hero | Holding a magnifying glass, squinting |
| Features | WebSocket section header | Two tin cans connected by wire, listening |
| Features | Screenshots section header | Posing with camera, flash going off |
| Security | Page hero | Wearing sunglasses, bouncer stance |
| Security | Data flow diagram boundary line | Sitting on the line, legs dangling |
| Security | "Zero Network Calls" header | Unplugging an ethernet cable, smirking |
| Privacy | "100% Local" header | Hugging a tiny server rack protectively |

### Wave 3: Secondary Pages

| Page | Location | Pose |
|------|----------|------|
| Configuration | Page hero | Turning a dial/knob, tongue out in concentration |
| Alternatives | Comparison table area | Standing on #1 podium, tiny trophy, waving |
| MCP Integration | "How MCP Mode Works" flow | Riding a fuel pipe like a waterslide |
| Roadmap | Phase headers (3x) | Growing from tiny ember → medium flame → roaring fire |
| Blog | Index header | Sitting at a tiny desk, quill pen, ink spot on cheek |
| Troubleshooting | Page hero | Hard hat, holding a wrench bigger than himself |

### Wave 4: Demo App

| Page | Location | Pose |
|------|----------|------|
| Error states | Toast/banner | Wincing, one eye closed, holding a fire extinguisher |
| Empty states | "No data" messages | Sitting cross-legged, meditating, eyes closed |
| WebSocket disconnect | Status indicator | Flickering/dimming, reaching toward a severed wire |
| Homepage footer | Below nav | Asleep on a campfire, single "z" ember rising |

**Effort:** Asset creation (illustration), then drop-in placement. No code architecture changes.

---

## Lifecycle Integration — Beyond Local Dev

Gasoline today works in local development. The three largest gaps in the web development lifecycle are places where browser observability doesn't exist at all.

---

### <i class="fas fa-cogs"></i> CI Browser Observability

**Status:** To specify
{: .notice--warning}

**The gap:** CI pipelines run browsers but provide zero browser-level observability. When an E2E test fails, you get the test framework's error and maybe a screenshot. No console logs, no network responses, no WebSocket state, no DOM context. 26% of developer time goes to CI failure investigation. 30% of CI failures are flaky.

**The solution:** Run Gasoline alongside Playwright/Cypress in CI. On test failure, the AI reads console errors, network bodies, and DOM state — skipping the "pull branch, reproduce locally, fail to reproduce, add logging, push, wait" loop.

**Architecture:** The capture logic in `inject.js` is pure JavaScript with no Chrome API dependencies in the core. Two paths:

1. **Script injection** — inject via Playwright's `addInitScript()`, POST directly to the Gasoline server (no extension needed, works in true headless)
2. **Extension loading** — load the extension in CI Chrome (`--load-extension`, requires `--headless=new`)

```typescript
// Playwright integration concept
import { gasolineFixture } from '@aspect-fuel/playwright';

test.afterEach(async ({}, testInfo) => {
  if (testInfo.status === 'failed') {
    const state = await fetch('http://localhost:7890/snapshot').then(r => r.json());
    testInfo.attach('gasoline-state', { body: JSON.stringify(state), contentType: 'application/json' });
  }
});
```

**Estimated value:** $30-60K/year per 10-person team in recovered engineering time.

**Effort:** Medium. Requires: standalone CI capture script (~200 lines), `/snapshot` server endpoint (~50 lines Go), Playwright fixture package.

---

### <i class="fas fa-eye"></i> Preview Deployment Observability

**Status:** To specify
{: .notice--warning}

**The gap:** When someone tests a Vercel/Netlify preview deployment and finds a bug, the feedback is a screenshot + "it broke." The developer spends 15-30 minutes reproducing an environment-specific issue. No existing tool provides client-side observability on preview environments — Vercel's observability is server-side only, and session replay tools aren't deployed on previews.

**The solution:** Run Gasoline during preview QA. When a bug is found, the full browser state (console, network, WebSocket, DOM) is already captured. Attach it to the PR as a structured artifact. The developer's AI reads it immediately — no reproduction needed.

**Architecture:** The reviewer has the Gasoline extension installed. The preview deployment has a lightweight Gasoline server running (or the reviewer's local server captures from the preview URL). Captured state exports as a shareable JSON artifact.

**Estimated value:** Eliminates 1-2 reproduction cycles per PR review (15-30 min each).

**Effort:** Low-Medium. Extension already captures from any URL. Needs: export/share mechanism, artifact format spec.

---

### <i class="fas fa-exchange-alt"></i> Production-to-Local Bridge

**Status:** To specify
{: .notice--warning}

**The gap:** Production error monitoring (Sentry, DataDog) tells you *what* broke but not *how to reproduce it locally*. Developers spend 30-60 minutes per bug setting up local reproduction. Session replay tools show visual state but not developer state (no WebSocket payloads, no computed styles, no a11y tree).

**The solution:** When reproducing a production issue locally, Gasoline captures the full browser context — network bodies, WebSocket messages, console logs, DOM state — so the AI can compare against the production error report and identify the exact trigger conditions.

**Workflow:**
1. Sentry alerts: "TypeError on `/dashboard`"
2. Developer opens the page locally with Gasoline running
3. AI reads Gasoline: API returned `null` instead of `[]`, WebSocket dropped 2s before the error, loading spinner never resolved
4. Root cause identified without manual investigation

**Estimated value:** 30-60 minutes saved per production bug investigation.

**Effort:** Low. Gasoline already captures everything needed. Value comes from documentation, workflow recipes, and optional Sentry/DataDog integration guides.

---

## Integration Opportunities

| Integration | Pain Point | Gasoline Value | Estimated Impact |
|-------------|-----------|----------------|-----------------|
| **E2E flaky test diagnosis** | Root cause is app-side (race conditions, API timing) but tests only show assertions | WS events + network timing + console logs reveal the actual race condition | -70% test failure investigation time |
| **Visual regression context** | Percy/Chromatic show WHAT changed, not WHY | Network bodies reveal different API data; console shows CSS overrides | -50% visual regression triage time |
| **Storybook observability** | Multiple addons needed (console, a11y, network) | Single MCP interface replaces all | Addon consolidation, unified AI access |
| **API debugging** | Postman tests in isolation; browser has CORS, cookies, auth, ordering | Gasoline captures real browser API interactions with full context | Eliminates "copy from network tab" workflow |
| **Code review context** | QA feedback is unstructured screenshots | Structured browser state attached to PR comments | -80% "can't reproduce reviewer's bug" time |
| **Stale test maintenance** | Tests break on legitimate app changes, no way to tell expected vs unexpected | Gasoline state shows whether the app behavior actually changed or just the test is stale | Faster test update decisions |

---

## Economic Impact

| Gap | Annual Cost (10-person team) | Gasoline Saves |
|-----|------------------------------|---------------|
| CI failure investigation | $75-150K (26% of eng time on debugging) | $30-60K (skip reproduce-locally loop) |
| Preview QA reproduction | $15-30K (1-2 cycles/PR × 15-30 min) | $10-20K (zero reproduction needed) |
| Production bug reproduction | $25-50K (30-60 min/bug × frequency) | $15-30K (instant context from Gasoline) |
| Flaky test root cause | $20-40K (30% of CI failures × investigation) | $14-28K (browser state reveals race conditions) |
| **Total addressable** | **$135-270K/year** | **$69-138K/year recovered** |

Zero cost. Open source. No cloud dependency. The savings come from time — not from replacing paid tools.
