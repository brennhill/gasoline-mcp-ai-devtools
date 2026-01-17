# Gasoline MCP Versioning & Roadmap

**Single source of truth. For strategic analysis, see [ROADMAP-STRATEGY-ANALYSIS.md](roadmap-strategy-analysis.md).**

---

## Thesis

**AI will be the driving force in development.**

Gasoline MCP's strategic differentiator is enabling AI to **close the feedback loop autonomously** — observe, diagnose, and repair without human intervention. Every feature is evaluated against this thesis.

**v6.0 Thesis:** AI autonomously validates and fixes web applications through exploration, observation, and intelligent iteration.

---

## Release Strategy

- **v5.2** — ✅ All critical bugs fixed (v5.1 blockers). Ready to release.
- **v5.3** — ✅ Critical usability fixes complete (pagination, buffer-specific clearing). **v6.0 thesis now unblocked.**
- **v6.0** — Release when core AI-native toolkit is complete (Wave 1 + Wave 2 features). Single point release. **Marketing moment: "AI autonomously validates and fixes web applications."**
- **v6.1-6.2** — AI-Native expansion (capabilities, safe repair)
- **v6.3-6.4** — Enterprise features (zero-trust, production)
- **v6.5-6.8** — Post-thesis roadmap. Tier-based: moat → enterprise → production → optimization.
- **v7** — Complete AI-Native testing (semantic understanding for microservices)

---

## Strategic Problem Space

### A. Context / Token Inefficiency
**Problem:** Chrome DevTools and similar tools dump raw browser state (massive DOM trees, accessibility dumps, long logs) that blow context windows.

**Why competitors fail:** They expose everything, interpret nothing. MCP is plumbing, not intelligence.

**Gasoline MCP's solution:** Semantic debugging context. Decide what matters before the model sees it.

✅ **Solved when:** Typical debugging session fits in <25% of context window. "10× less context than DevTools MCP."

---

### B. Shallow Debugging (Symptoms, Not Causes)
**Problem:** Most tools surface symptoms ("There's an error") but don't answer why.

**Why competitors fail:** They stop at observation. Root-cause analysis is left to humans.

**Gasoline MCP's solution:** Causal debugging. Connect: User action → DOM mutation → network call → backend response → frontend failure.

✅ **Solved when:** AI can answer "The bug exists because X changed, which broke Y, which surfaces as Z."

---

### C. Weak Feedback Loops (No "Fix → Verify → Done")
**Problem:** Most AI debugging is one-shot: observe → suggest → hope it worked.

**Why competitors fail:** They treat debugging as analysis, not a loop.

**Gasoline MCP's solution:** Closed-loop debugging. Apply/simulate fix → re-run scenario → confirm resolved automatically.

✅ **Solved when:** Bugs marked "resolved" automatically. "This fix removed the error across 3 retries."

---

### D. Garbage In → Garbage Out Selectors & Tests
**Problem:** AI generates brittle selectors (nth-child, random classes) that fail on refactor.

**Why competitors fail:** They don't understand UI semantics — only DOM structure.

**Gasoline MCP's solution:** Selector intelligence + semantic anchoring. Prefer roles, labels, stable attributes.

✅ **Solved when:** Generated tests survive minor UI refactors. Test flakiness drops materially.

---

### E. Raw Data Instead of Developer-Ready Output
**Problem:** Tools dump logs, traces, screenshots. Developers still have to think.

**Why competitors fail:** They optimize for machine access, not human comprehension.

**Gasoline MCP's solution:** First-class bug reports written by AI. Readable by humans, trusted by teams.

✅ **Solved when:** Bug reports can be pasted directly into GitHub/Jira. No follow-up questions needed.

---

### F. Unsafe / Awkward Production Debugging
**Problem:** Most tools assume local dev, no sensitive data, one user. Reality is different.

**Why competitors fail:** They weren't designed for prod safety from day one.

**Gasoline MCP's solution:** Production-safe AI debugging. Data redaction, session isolation, read-only modes.

✅ **Solved when:** Security teams approve prod usage. Engineers debug real user bugs safely.

---

### G. Traditional QA Fails for AI (NEW)
**Problem:** Traditional QA workflows (test-first, test suites, baselines) don't match how AI naturally works.

**Why competitors fail:** They force AI into human workflows instead of leveraging AI's natural strengths.

**Gasoline MCP's solution:** AI-Native testing. Explore → Observe → Infer → Act → Validate. No test suites, no baselines, no approval workflows.

✅ **Solved when:** AI autonomously validates and fixes web applications without human intervention.

---

## v5.2: Immediate (Bug Fixes)

✅ **Complete** — All critical v5.1 blockers fixed and ready to release.

---

## v5.3: Critical Blockers (Before v6.0)

✅ **Complete** — All critical blockers removed. v6.0 thesis unblocked.

### ✅ 1. Pagination for Large Datasets

**Implemented:** Cursor-based pagination with `limit`, `offset`, `after_cursor`, `before_cursor`, `since_cursor` parameters.
```javascript
observe({what: "network_waterfall", limit: 100})
observe({what: "network_waterfall", after_cursor: "2026-01-30T10:15:23.456Z:1234", limit: 100})
observe({what: "logs", since_cursor: "2026-01-30T10:00:00Z:0"})
```

**Impact:** Solves MCP token limit issue. AI can query large datasets in manageable chunks.

### ✅ 2. Buffer-Specific Clearing

**Implemented:** Granular buffer control via `buffer` parameter in `configure({action: "clear"})`.
```javascript
configure({action: "clear", buffer: "network"})    // Clear network waterfall + bodies
configure({action: "clear", buffer: "websocket"})  // Clear WebSocket events + status
configure({action: "clear", buffer: "actions"})    // Clear user action buffer
configure({action: "clear", buffer: "all"})        // Clear all buffers
```

**Impact:** Prevents memory bloat. Enables granular cleanup for long sessions.

### 3. Server-Side Aggregation (Future) ⭐⭐

**Status:** Deferred. Pagination proves sufficient for current needs.

**Problem:** Even with pagination, large datasets can be verbose. Need summary views.

**Solution:** Add aggregation endpoints:
```javascript
observe({what: "network_stats", group_by: "host"})
→ {"localhost:3000": {count: 150, avg_duration: 45ms, errors: 3}}
```

**Effort:** 4-6 hours. Start only if pagination proves insufficient.

---

## v6.0: AI-Native Testing & Validation (Core Thesis Release)

**Goal:** Prove the AI-native thesis — AI autonomously validates and fixes web applications through exploration, observation, and intelligent iteration.

**Release criteria:** Wave 1 (AI-native toolkit) + Wave 2 (basic persistence) features shipped and battle-tested.

**Philosophy:** Don't make LLMs write better tests. Make LLMs better at understanding and fixing web applications through observation, exploration, and intelligent iteration.

### Wave 1: AI-Native Toolkit (2-3 weeks)

**Goal:** Give AI "eyes, ears, hands" to explore and fix web applications

```
┌─────────────────────────────────────────────────────────────┐
│           4 CAPABILITIES FOR AI TO USE                   │
├──────────────┬───────────────┬────────────────────────┤
│ Capability A │ Capability B  │ Capability C            │
│              │               │                         │
│  EXPLORE     │  OBSERVE      │  COMPARE & INFER       │
│              │               │                         │
│  interact     │  observe       │  analyze                 │
│  .explore     │  .capture      │  .compare + .infer       │
│  .record      │               │  .detect_loop            │
│  .replay      │               │                         │
└──────────────┴───────────────┴────────────────────────┘
                    ↓
         AI USES THESE FOR TWO DEMOS:
         1. Spec-Driven Validation
         2. Production Error Reproduction
```

**Components:**

1. **Explore Capability** (`interact`)
   - `interact.explore` - Execute actions, capture full state (console, network, DOM, screenshots)
   - `interact.record` - Capture user interactions for later replay
   - `interact.replay` - Reproduce recordings in different environments (dev vs prod)

2. **Observe Capability** (`observe`)
   - `observe.capture` - Capture comprehensive state (console, network, DOM)
   - `observe.compare` - Compare two states (before/after, prod/dev)

3. **Infer Capability** (`analyze`)
   - `analyze.infer` - "What's different here?" - Natural language analysis
   - `analyze.detect_loop` - Detect doom loops from execution history

**Exit criteria:** AI can explore → observe → infer → understand behavior

### Wave 2: Basic Persistence (2-3 weeks)

**Goal:** Simple memory to prevent doom loops

```
┌─────────────────────────────────────────────────────────────┐
│           2 PERSISTENCE LAYERS (MVP)                    │
├──────────────┬───────────────┬────────────────────────┤
│ Layer 1      │ Layer 2       │                         │
│              │               │                         │
│  EXECUTION    │  DOOM LOOP    │                         │
│  HISTORY      │  PREVENTION    │                         │
│              │               │                         │
│  Test results │  Pattern       │                         │
│  (recent N)   │  detection     │                         │
└──────────────┴───────────────┴────────────────────────┘
```

**Components:**

1. **Execution History**
   - Track test executions
   - Store results (passed/failed)
   - Simple JSON file

2. **Doom Loop Prevention**
   - `analyze.detect_loop` - Detect repeated failures
   - Suggest alternative approaches
   - Prevent AI from trying same fix forever

**Exit criteria:** AI remembers past attempts, avoids loops

### Demo Scenarios

**Demo 1: Spec-Driven Validation**
- User provides product spec
- LLM reads spec, explores UI autonomously
- LLM validates behavior against spec
- LLM fixes bugs it finds
- **Time:** < 3 minutes, fully autonomous

**Demo 2: Production Error Reproduction**
- User encounters error in production
- User enables recorder, reproduces error
- LLM analyzes recording, attempts to reproduce in dev
- LLM identifies differences (prod vs dev)
- LLM fixes root cause
- **Time:** < 5 minutes

**Total v6.0: 4-6 weeks**

### Marketing Message

- "AI autonomously validates and fixes web applications"
- "Explore → Observe → Infer → Act → Validate loop"
- "No test suites. No baselines. No approval workflows."
- "10× less context than Chrome DevTools MCP"

---

## v6.1-6.2: AI-Native Expansion

**Goal:** Expand AI-native capabilities for safe repair

### v6.1: Advanced Exploration & Observation

- **Advanced Filtering (Signal-to-Noise)** — Content-type filters, domain allowlist/blocklist, regex patterns, response size thresholds. Reduce noise before AI sees it. Complements v5.3 pagination.
- **Visual-Semantic Bridge** — Computed layout maps. Auto-generate unique test-IDs. Solves "ghost clicks" and "hallucinated selectors." [Spec](features/feature/visual-semantic-bridge/product-spec.md)
- **State "Time Travel"** — Persistent event buffer. Before/after snapshots. Enables causal debugging across crashes. [Spec](features/feature/state-time-travel/product-spec.md)
- **Causal Diffing** — Root-cause analysis: "X changed → broke Y → surfaces as Z"
- **Reverse Engineering Engine** — Auto-infer OpenAPI specs from network traffic
- **Design System Injector** — Inject design tokens; stop AI from writing garbage CSS
- **Deep Framework Intelligence** — React/Vue component tree + props (not HTML soup)
- **DOM Fingerprinting** — Stable selectors survive UI refactors
- **Smart DOM Pruning** — Remove noise; fit DOM in <25% context window
- **Hydration Doctor** — Debug SSR mismatches (Next.js/Nuxt)

### v6.2: Safe Repair & Verification

- **Prompt-Based Network Mocking** — AI instructs Gasoline MCP to mock responses. Verify-fix-verify loops.
- **Shadow Mode** — Intercept POST/PUT/DELETE; return fake 200. Test destructive ops safely.
- **Pixel-Perfect Guardian** — Reject changes that cause CSS regression
- **Healer Mode** — Auto-convert bug fixes into permanent regression tests

---

## v6.3-6.4: Enterprise Features

**Goal:** Enable corporate adoption with safety guarantees and production debugging

### v6.3: Zero-Trust Enterprise

- **Zero-Trust Sandbox** — Action gating, data masking, prompt injection defense
- **Asynchronous Multiplayer Debugging** — Flight Recorder links for crash sharing
- **Session Replay Exports** — .gas files for hand-off to humans or better AI models

### v6.4: Production Compliance

- **Read-Only Mode** — Non-mutating observation in production
- **Tool Allowlisting** — Restrict which MCP tools run
- **Project Isolation** — Multi-tenant capture contexts
- **Configuration Profiles** — Pre-tuned bundles (paranoid, restricted, short-lived)
- **Redaction Audit Log** — Compliance logging with automatic data redaction
- **GitHub/Jira Integration** — Paste-ready bug reports
- **CI/CD Integration** — GitHub Actions, SARIF export, HAR attachment
- **IDE Integration** — VS Code plugin, Claude Code integration
- **Documentation Links (docs_url)** — Add `docs_url` to `/health` response and MCP tool schemas. Links to versioned markdown docs on GitHub repo. Supports offline-first workflows while providing navigation to detailed API documentation.
- **Event Timestamps & Session IDs** — Audit trails, precise ordering
- **CLI Lifecycle Commands** — `stop`, `restart`, `status` for ops

---

## v6.5-6.8: Post-Thesis Roadmap (Tier Strategy)

**Organization:** Features grouped by tier. Each tier validates part of the thesis and unlocks the next market segment.

**See [ROADMAP-STRATEGY-ANALYSIS.md](roadmap-strategy-analysis.md) for detailed strategic analysis.**

---

## Tier 1: Core Moat — Smart Observation & Safe Repair (v6.0-6.2)

**Goal:** Validate thesis — AI autonomously validates and fixes web applications (explore → observe → infer → act → validate).

**Competitive Differentiator:** What Chrome DevTools and other AI agents cannot do.

### v6.5: Token & Context Efficiency

**Solves:** Problem A — reduce costs

- **Focus Mode** — "Tunnel Vision": AI selects component; Gasoline MCP sends only that. 90% token reduction.
- **Semantic Token Compression** — Lightweight references (@button1, @input2). JIT context pruning.

### v6.6: Specialized Audits & Analytics

**Adjacent features:** Domain-specific superpowers

- **Performance Audit** — Root-cause perf issues
- **Best Practices Audit** — Structural/deprecated/security issues
- **SEO Audit** — Metadata, heading structure, structured data
- **A11y Tree Snapshots** — Accessibility compression
- **Enhanced WCAG Audit** — Deep accessibility beyond axe-core
- **Annotated Screenshots** — Visual context for vision models
- **Design Audit & Archival** — Screenshot archival + queryable design system compliance. Enables design regression testing across responsive variants (desktop/tablet/mobile) with semantic queries (component, variant, viewport, date, URL). Auto-capture on `observe({what: 'design-audit'})`. Disk-aware cleanup (5GB default). [Spec](../screenshot-archival-and-query.md) [Review](../screenshot-archival-and-query-review.md)

### v6.7: Advanced Interactions

**Adjacent features:** Rich interactions for complex scenarios

- **Form Filling** — Auto-fill complex forms
- **Dialog Handling** — Handle alerts, confirms, prompts
- **Drag & Drop** — Complex UI interactions
- **CPU/Network Emulation** — Throttle, load test
- **Local Web Scraping** — Authenticated multi-step extraction

### v6.8: Infrastructure & Quality

**Adjacent features:** Continuous delivery and reliability

- **Fuzz Tests** (5 types) — JSONRPC, HTTP, security, WebSocket, network
- **Async Command Execution** — Prevent MCP server hangs
- **Multi-Client MCP Architecture** — Multiple AI clients on one server
- **Test Generation v2** — DOM assertions, fixtures, visual snapshots
- **Performance Budget Monitor** — Baseline regression detection

---

## v7: Unified Backend-Frontend Debugging

**Goal:** AI debugs full stack—browser, backend, infrastructure, tests—as single coherent system.

**Vision:** [Backend-Frontend Unification](docs/core/backend-frontend-unification.md) — Complete eyes, ears, and hands for AI debugging.

**Release criteria:** All three phases (EARS, EYES, HANDS) complete. Full-stack correlation working. Causality analysis proven.

### v7.0: EARS + EYES (Backend Integration & Correlation)

**Goal:** Ingest backend data + correlate with browser telemetry. Make causality visible to AI.

#### Phase 1: EARS (Backend Data Ingestion) — 4 features

Make backend visible to AI. Ingest logs, tests, code changes.

- [Backend Log Streaming](features/feature/backend-log-streaming/product-spec.md) — Real-time backend log tail (gRPC, WebSocket, file watch)
- [Custom Event API](features/feature/custom-event-api/product-spec.md) — App-injected structured events with correlation IDs
- [Test Execution Capture](features/feature/test-execution-capture/product-spec.md) — Test framework output capture (Jest, pytest, Mocha)
- [Git Event Tracking](features/feature/git-event-tracking/product-spec.md) — File changes, commits, branches linked to correlation IDs

#### Phase 2: EYES (Semantic Correlation) — 4 features

Link browser ↔ backend ↔ tests. Answer "why did this happen?"

- [Request/Session Correlation](features/feature/request-session-correlation/product-spec.md) — W3C Trace Context propagation, link browser requests to backend logs
- [Causality Analysis](features/feature/causality-analysis/product-spec.md) — Root-cause chains, latency attribution, gap detection
- [Normalized Log Schema](features/feature/normalized-log-schema/product-spec.md) — Unified JSON for browser, backend, tests, git
- [Historical Snapshots](features/feature/historical-snapshots/product-spec.md) — Replayable state, before/after comparisons

**Exit criteria:** AI can correlate browser action → backend decision → test result. Causality chains working end-to-end.

**Total v7.0: 4-6 weeks (Phases 1 + 2)**

### v7.1: HANDS (Autonomous Control)

**Goal:** Expand AI's ability to act. Not just observe, but fix and test end-to-end.

#### Phase 3: HANDS (Autonomous Control) — 4 features

AI controls backend, code, environment. Tests fixes autonomously.

- [Backend Control](features/feature/backend-control/product-spec.md) — State management, data injection, failure simulation
- [Code Navigation & Modification](features/feature/code-navigation-modification/product-spec.md) — Code search, read, modify, test integration
- [Environment Manipulation](features/feature/environment-manipulation/product-spec.md) — Environment inspection, modification, config management
- [Timeline & Search](features/feature/timeline-search/product-spec.md) — Unified timeline with microsecond precision, full-stack correlation queries

**Exit criteria:** AI can diagnose bug, fix code, run tests, verify resolution. Fully autonomous workflows.

**Total v7.1: 3 weeks (Phase 3)**

---

## Marketing Milestones

| Release | Narrative | Evidence |
|---------|-----------|----------|
| **v6.0** | "AI autonomously validates and fixes web applications" | Wave 1 + Wave 2 work together for autonomous validation |
| **v6.2** | "Enterprise-safe autonomous debugging" | Zero-Trust Sandbox + Healer Mode + network mocking |
| **v6.4** | "Production-ready with compliance" | Audit log + GitHub/Jira + CI/CD + read-only mode |
| **v7.0** | "AI debugs the full stack, not just browser" | Backend logs + correlation + causality analysis |
| **v7.1** | "Fully autonomous debugging across services" | Code modification + backend control + test execution |
| **v6.5+** | "Specialized debugging superpowers" | Token compression + audits + advanced interactions |

---

## Key Principles

1. **Thesis-First:** Every feature must advance "AI autonomously validates and fixes web applications"
2. **Tier Clarity:** Features organized by competitive value and market unlock
3. **Serial Critical Path:** v5.2 → v5.3 → v6.0 → v6.2 → v6.3 → v6.4 → v7.0 (dependencies)
4. **Parallel Polish:** v6.5-6.8 run concurrently, non-blocking
5. **Problem Mapping:** All 7 strategic problems (A-G) addressed by specific tiers
6. **AI-Native Philosophy:** Leverage AI's natural strengths (explore, observe, infer, iterate) instead of forcing it into traditional QA workflows

---

## Roadmap Dependencies & Sequencing

### Critical Path (must serialize)
```
✅ v5.2 (bugs) → ✅ v5.3 (blockers) → ⏳ v6.0 (AI-native thesis) → v6.1-6.2 (expansion) → v6.3-6.4 (enterprise) → v7.0 (semantic understanding)
```

**Status:** v5.2 + v5.3 complete. v6.0 AI-native thesis (Wave 1 + Wave 2) is next priority.

### Parallel Tracks (can start after v6.2)
```
v6.5 (efficiency), v6.6 (audits), v6.7 (interactions), v6.8 (infra) — all run concurrently
```

### Team Allocation
- **v5.2-6.0:** All hands on critical path (thesis validation)
- **v6.1-6.2:** 2-3 agents (expansion features, cannot be parallelized)
- **v6.3-6.4:** 1-2 agents (enterprise/compliance, can overlap with v6.1-6.2)
- **v7.0:** 2-3 agents (semantic understanding, must be serial)
- **v6.5-6.8:** 1 agent per tier (continuous shipping, true parallelization)

---

## Build Plan: v6.0 Maximum Parallelization

**After v5.2+v5.3 complete:**

```
WAVE 1 (2-3 weeks)
├── Explore Capability (interact.explore, record, replay)
├── Observe Capability (observe.capture, compare)
└── Infer Capability (analyze.infer, detect_loop)
    ↓ (all complete, merge to next)

WAVE 2 (2-3 weeks)
├── Execution History (track test executions)
└── Doom Loop Prevention (pattern detection)
    ↓ (all complete)

v6.0 RELEASE → MARKET VALIDATION
```

---

## Out of Scope (Deferred)

- Machine learning-based root cause inference (rule-based linking sufficient)
- Video replay or animated debugging (screenshots sufficient)
- Embedded database for offline analysis (session export sufficient)
- Real-time collaboration (Flight Recorder sufficient)
- Custom browser engines or protocol modifications
- Automatic service dependency graph inference (manual registration in v7.1+)
- Production multi-tenant isolation (read-only mode only)
- Advanced APM features (flame graphs, detailed spans)
