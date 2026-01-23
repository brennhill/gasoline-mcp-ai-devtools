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

## Phase 3: Agent Ergonomics

Low effort. Ship when convenient.

### <i class="fas fa-book"></i> Workflow Recipe

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
| ~~—~~ | ~~Behavioral Baselines~~ | High | Low | Killed |

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
