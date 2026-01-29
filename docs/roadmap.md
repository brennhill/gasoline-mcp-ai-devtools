# Gasoline Versioning & Roadmap

## Thesis

**AI will be the driving force in development.**

Gasoline's strategic differentiator is enabling AI to **close the feedback loop autonomously** — observe, diagnose, and repair without human intervention. Every feature is evaluated against this thesis.

---

## Release Strategy

- **v5.2** — ✅ All critical bugs fixed (v5.1 blockers). Ready to release.
- **v6.0** — Release when **core thesis is complete** (Wave 1 + Wave 2 features). Single point release. Marketing moment.
- **v6.1+** — Adjacent features that improve/enable the thesis (observation depth, interaction expansion, production safety).
- **v7** — If all roadmap features are shipped, bump to v7 to signal full-featured product.

---

## Strategic Problem Space

### A. Context / Token Inefficiency

**The problem**

Chrome DevTools MCP and similar tools shove raw browser state at the model:

- Massive DOM trees
- Accessibility dumps
- Screenshots
- Long console/network logs

This blows context windows and makes AI "forget" what it's debugging halfway through.

**Why competitors fail**

They expose everything, but interpret nothing. MCP is plumbing, not intelligence.

**Your opportunity**

Semantic debugging context. You decide what matters before the model sees it.

Examples:

- Only DOM nodes involved in the failing interaction
- Only network requests tied to the error
- Collapsed/abstracted logs with causal hints

✅ **We KNOW it's solved when:**

- A typical debugging session fits in <25% of a model's context window
- The model can explain the bug without re-requesting browser state
- You can show: "Same bug, 10× less context than Chrome DevTools MCP"

**This becomes a killer internal metric:** "Tokens per resolved bug"

---

### B. Shallow Debugging (Symptoms, Not Causes)

**The problem**

Most tools surface:

- "There's a console error"
- "This request failed"
- "This selector didn't match"

But they don't answer why.

**Why competitors fail**

They stop at observation. Root-cause analysis is left to the human.

**Your opportunity**

Causal debugging, not observational debugging. Your system should connect:

User action → DOM mutation → network call → backend response → frontend failure

✅ **We KNOW it's solved when:**

- The AI can answer: "The bug exists because X changed, which broke Y, which surfaces as Z."
- Fix suggestions reference specific causal links, not generic advice
- Engineers stop asking "but how do you know that's the cause?"

**Internal metric:** % of bugs with a single, confident root cause vs multiple guesses

---

### C. Weak Feedback Loops (No "Fix → Verify → Done")

**The problem**

Most AI debugging flows look like:

1. Observe bug
2. Suggest fix
3. 🤞 Hope it worked

Verification is manual or flaky.

**Why competitors fail**

They treat debugging as a one-shot analysis, not a loop.

**Your opportunity**

Closed-loop debugging. The system should automatically:

- Apply or simulate the fix
- Re-run the failing scenario
- Confirm the bug no longer occurs

✅ **We KNOW it's solved when:**

- Bugs are marked "resolved" automatically, not manually
- The AI can say: "This fix removed the error across 3 retries"
- Engineers trust the system enough to merge with confidence

**Metric:** % of fixes with automated verification, reduction in "fix didn't actually fix it" reopens

---

### D. Garbage In → Garbage Out Selectors & Tests

**The problem**

AI generates brittle selectors:

- `div:nth-child(7)`
- Random class names
- Over-fit Playwright steps

**Why competitors fail**

They don't understand UI semantics — only DOM structure.

**Your opportunity**

Selector intelligence + semantic anchoring

- Prefer roles, labels, stable attributes
- Fall back gracefully
- Explain why a selector is stable

✅ **We KNOW it's solved when:**

- Generated tests survive minor UI refactors
- Engineers stop rewriting AI-generated selectors
- Test flakiness drops materially

**Metric:** Test survival rate across UI changes, manual edits required per generated test

---

### E. Raw Data Instead of Developer-Ready Output

**The problem**

Tools dump:

- Logs
- Traces
- Screenshots

Developers still have to think.

**Why competitors fail**

They optimize for machine access, not human comprehension.

**Your opportunity**

First-class bug reports written by AI. Readable by humans, trusted by teams.

✅ **We KNOW it's solved when:**

- Bug reports can be pasted directly into GitHub/Jira
- Engineers don't ask follow-up clarification questions
- PMs can understand bugs without running the app

**Metric:** % of bug reports accepted without edits, time from bug detection → ticket creation

---

### F. Unsafe / Awkward Production Debugging

**The problem**

Most tools assume:

- Local dev
- No sensitive data
- One user at a time

Reality says otherwise.

**Why competitors fail**

They weren't designed for prod safety from day one.

**Your opportunity**

Production-safe AI debugging

- Data redaction
- Session isolation
- Read-only or replay-based debugging

✅ **We KNOW it's solved when:**

- Security teams approve usage in prod
- Engineers can debug "real user bugs" safely
- No "turn it off in production" footguns

**Metric:** Security approvals, production incidents debugged safely

---

## v6.0: Core Thesis Release

**Goal:** Prove the thesis — AI closes the feedback loop autonomously.

**Release criteria:** Wave 1 + Wave 2 features are shipped and battle-tested.

### v6.0 Features: The Core Loop

**Wave 1 (3 features, parallel)** — Foundations for autonomous closed-loop

1. **Self-Healing Tests** (#33) — AI observes test failure → diagnoses via Gasoline → fixes code/test → verifies
2. **Gasoline CI Infrastructure** — Enable autonomous loops in CI/CD pipelines (`/snapshot`, `/clear`, `/test-boundary`, Playwright fixtures)
3. **Context Streaming** (#5) — Real-time push notifications instead of raw data dumps

**Wave 2 (3 features, parallel after Wave 1)** — Expand closed-loop across scenarios

4. **PR Preview Exploration** (#35) — Deploy preview → explore → discover bugs → propose fixes
5. **Agentic E2E Repair** (#34) — Detect API drift → auto-fix tests/mocks
6. **Deployment Watchdog** (#36) — Post-deploy monitoring → auto-rollback on regression

### v6.0 Marketing Moment

When all 6 features ship:
- "Same bug, 10× less context than Chrome DevTools MCP" (Context Streaming solves Problem A)
- "AI autonomously fixes tests, not just suggests fixes" (Self-Healing solves Problem C)
- "Closed-loop verification: fix → test → confirm → done" (all 3 Wave 2 features prove Problem C)

Release v6.0. This is the thesis validation point.

---

## v6.1+: Thesis Expansion & Adjacent Features

These features improve or enable the core loop but don't validate the thesis. Ship after v6.0.

### v6.1: Observation Depth (Improves "diagnose" leg)

- **Causal Diffing** — Root-cause analysis ("X changed → broke Y → surfaces as Z")
- **DOM Fingerprinting** — Stable selectors for self-healing tests
- **A11y Tree Snapshots** — Compress accessibility for <25% context window
- **Performance Audit** — Root-cause perf issues (render-blocking, bundle size, DOM bloat)
- **Best Practices Audit** — Structural issues (HTTPS, deprecated APIs, security headers)
- **SEO Audit** — SEO debugging (metadata, heading structure, structured data)
- **Annotated Screenshots** — Visual context for AI vision models
- **Enhanced WCAG Audit** — Deep a11y beyond axe-core

### v6.2: Interaction Expansion (Broadens "repair" scope)

- **Form Filling** — Auto-fill complex forms
- **Dialog Handling** — Handle alerts, confirms, prompts
- **Drag & Drop** — Complex UI interactions
- **CPU/Network Emulation** — Throttle to reproduce issues under load
- **Local Web Scraping** — Authenticated multi-step data extraction

### v6.3: Production Safety (Enables enterprise autonomous debugging)

- **Read-Only Mode** — Non-mutating capture in production
- **Tool Allowlisting** — Restrict which MCP tools run
- **Project Isolation** — Multi-tenant capture contexts
- **Configuration Profiles** — Pre-tuned bundles (paranoid, restricted, short-lived)
- **Dynamic Exposure** — Feature flags for safe rollout

### v6.4: DX & Workflow Integration (Adoption enablers)

- **GitHub/Jira Bug Reports** — Paste-ready output
- **CI/CD Integration** — GitHub Actions, SARIF, HAR attachment
- **IDE Handoff** — VS Code plugin, Claude Code integration
- **Client Identification** — Track which AI client is debugging
- **Session IDs** — Unique per MCP connection
- **Redaction Audit Log** — Compliance logging
- **Event Timestamps** — `received_at` for precise ordering
- **Test Fixture Page** — `/test-page` with error triggers
- **CLI Test Mode** — `--test` flag for self-validation
- **Mock Extension Client** — Go package for server testing
- **MCP Test Harness** — Scripted MCP testing
- **CLI Lifecycle Commands** — `stop`, `restart`, `status`
- **Data Export** — JSON Lines buffer export
- **E2E Testing Integration** — Playwright fixtures, failure attachment
- **Workflow Integration** — CI/CD automation

### v6.5+: Infrastructure & Quality (Continuous shipping)

- **Fuzz Tests** (5 types) — JSONRPC parser, HTTP body, security patterns, WebSocket, network body
- **Async Command Execution** — Prevent MCP server hangs
- **Multi-Client MCP Architecture** — Multiple AI clients on one server
- **Test Generation v2** — DOM assertions, fixtures, visual snapshots
- **Performance Budget Monitor** — Baseline regression detection

---

## v7: Complete Roadmap Delivery

If/when all features are shipped:
- v7.0 released as "full-featured" version
- Signal market maturity
- All 40+ features working together

---

## v5.2: Immediate Priority

These are known bugs and UX issues from UAT. Must be resolved before v6 feature work.

See [KNOWN-ISSUES.md](../KNOWN-ISSUES.md) for user-facing summary and [docs/core/in-progress/UAT-ISSUES-TRACKER.md](core/in-progress/UAT-ISSUES-TRACKER.md) for investigation notes.

### Bug Fixes

| # | Severity | Issue | Status |
|---|----------|-------|--------|
| 2 | High | `query_dom` not implemented — schema advertises it but background.js returns `not_implemented` | ✅ FIXED |
| 3 | High | Accessibility audit runtime error — `runAxeAuditWithTimeout` "not defined" at runtime | ✅ FIXED |
| 4 | Medium | `network_bodies` returns no data — empty arrays on multiple page loads | ✅ FIXED |
| 5 | Medium | Extension timeouts after 5-6 operations — possible message queue backup or memory leak | ✅ FIXED |
| 6 | Medium | `observe()` missing tabId — content.js attaches it but server doesn't surface in MCP responses | ✅ FIXED |

### UX Improvements

- [ ] Visual indicator on tracked tab (extension badge icon)
- [ ] Confirmation dialog when switching tracked tab
- [ ] Tab switch suggestion when tracked tab closes

### Completed (v5.0–v5.1)

- [x] **Usability Improvements** — NPM/PyPI install, MCP config, --check, --persist, first-run banner, inline troubleshooting
- [x] **Single-tab tracking isolation** — Security fix: only captures from explicitly tracked tab
- [x] **Network schema improvements** — Unit suffixes, compression ratios, timestamps
- [x] **validate_api parameter fix** — Renamed conflicting parameter to `operation`

---

## v6.0 Build Plan: Maximum Parallelization

**v5.2 completion** → **Wave 1 (3 agents, parallel)** → **Wave 2 (3 agents, parallel)**

### Wave 1: Thesis Foundations (Concurrent)

```
┌─────────────────────────────────────────────────────────────────┐
│                 3 AGENTS IN PARALLEL                             │
├─────────────────────┬─────────────────────┬─────────────────────┤
│  Agent A            │  Agent B            │  Agent C            │
│                     │                     │                     │
│  33. Self-Healing   │  Gasoline CI        │  5. Context         │
│      Tests          │  Infrastructure     │     Streaming       │
│                     │                     │                     │
│  - Detect failure   │  - /snapshot        │  - Push events      │
│  - Diagnose via     │  - /clear           │  - Real-time feed   │
│    Gasoline         │  - /test-boundary   │  - Curated context  │
│  - Auto-fix code    │  - gasoline-ci.js   │    (not raw dumps)  │
│    or test          │  - Playwright       │                     │
│  - Verify fix       │    fixtures         │                     │
└─────────────────────┴─────────────────────┴─────────────────────┘
```

**Wave 1 Prerequisites:** ✅ All shipped (Tab targeting, Verification loop, API validation)
**Wave 1 Duration:** ~4-6 weeks (estimated)
**Wave 1 Exit Criteria:** All 3 features tested, merged to next

### Wave 2: Thesis Expansion (After Wave 1, Concurrent)

```
┌─────────────────────────────────────────────────────────────────┐
│                 3 AGENTS IN PARALLEL                             │
├─────────────────────┬─────────────────────┬─────────────────────┤
│  Agent A            │  Agent B            │  Agent C            │
│                     │                     │                     │
│  35. PR Preview     │  34. Agentic E2E    │  36. Deployment     │
│      Exploration    │      Repair         │      Watchdog       │
│                     │                     │                     │
│  - Deploy preview   │  - Detect API drift │  - Post-deploy      │
│  - Auto-explore     │  - Update tests     │    monitoring       │
│  - Report bugs      │  - Update mocks     │  - Detect regs      │
│  - Propose fixes    │  - Verify fixes     │  - Auto-rollback    │
└─────────────────────┴─────────────────────┴─────────────────────┘
```

**Wave 2 Prerequisites:** ✅ Wave 1 complete
**Wave 2 Duration:** ~4-6 weeks (estimated)
**Wave 2 Exit Criteria:** All 3 features tested, merged to next, v6.0 release candidate ready

### v6.0 Release Criteria

- ✅ v5.2 bugs fixed
- ✅ Wave 1 features shipped (Self-Healing, CI, Context Streaming)
- ✅ Wave 2 features shipped (PR Preview, E2E Repair, Deployment Watchdog)
- ✅ All 6 features tested in realistic scenarios
- ✅ No new regressions in v5.1 features
- ✅ Marketing narrative ready ("10× less context than DevTools MCP, autonomous closed loops")

**Then: Release v6.0 as single point release. Market moment.**

---

## v6.1+: Post-Thesis Roadmap

See separate sections above. These are shipped concurrent or after v6.0. Not blockers for v6.0 release.

---

## In-Progress Features (Partial Implementation)

These features are >50% complete but not yet shipped. Resume work in v6.1+:

| Feature | Status | Notes |
|---------|--------|-------|
| Behavioral Baselines | ~60% | Baseline regression detection for performance |
| Budget Thresholds | ~60% | Configurable alert thresholds (v6.1) |
| Causal Diffing | ~70% | Root-cause change analysis (v6.1) |
| DOM Fingerprinting | ~80% | Stable selector generation for self-healing (v6.1) |
| Interception Deferral | ~50% | Deferred network body capture |
| Self-Testing | ~40% | Extension self-validation via own tools |
| SPA Route Measurement | ~60% | Single-page app route timing |

**Recommendation:** Complete Causal Diffing + DOM Fingerprinting during Wave 2 as they enable Self-Healing Tests (Wave 1). Complete others in v6.1 after v6.0 release.

---

## v6.0 Dependency Graph

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                           v5.1 COMPLETE                                     │
│  Tab Targeting, API Validation, Verification Loop, Session Diff,            │
│  Security Auditing, Audit Logging, Enterprise Features                      │
└─────────────────────────────────────────────────────────────────────────────┘
                              │ v5.2 bugs fixed
                              ▼
        ┌─────────────────────────────────────────────────────────┐
        │           WAVE 1: v6.0 Foundations (parallel)           │
        │                                                         │
        │  ┌──────────┐  ┌──────────┐  ┌──────────┐             │
        │  │ Self-    │  │ Gasoline │  │ Context  │             │
        │  │ Healing  │  │ CI       │  │ Streaming│             │
        │  │ Tests    │  │ Infra    │  │          │             │
        │  │ (33)     │  │          │  │ (5)      │             │
        │  └────┬─────┘  └────┬─────┘  └────┬─────┘             │
        │       │             │             │                   │
        │       └─────────────┴─────────────┘                   │
        │               │ Wave 1 complete
        └───────────────┼─────────────────────────────────────────┘
                        ▼
        ┌─────────────────────────────────────────────────────────┐
        │           WAVE 2: v6.0 Expansion (parallel)             │
        │                                                         │
        │  ┌──────────┐  ┌──────────┐  ┌──────────┐             │
        │  │ PR       │  │ Agentic  │  │ Deploy   │             │
        │  │ Preview  │  │ E2E      │  │ Watchdog │             │
        │  │ Explora- │  │ Repair   │  │          │             │
        │  │ tion(35) │  │ (34)     │  │ (36)     │             │
        │  └─────────┘  └─────────┘  └──────────┘              │
        │               │ Wave 2 complete
        └───────────────┼─────────────────────────────────────────┘
                        ▼
        ╔═════════════════════════════════════════════════════════╗
        ║  RELEASE v6.0 — Thesis Validated                        ║
        ║  "AI closes the feedback loop autonomously"             ║
        ║  ✓ Wave 1 + Wave 2 = 6 core features                   ║
        ╚═════════════════════════════════════════════════════════╝
                        ▼
        ┌─────────────────────────────────────────────────────────┐
        │  v6.1+ — Adjacent Features (concurrent, non-blocking)   │
        │                                                         │
        │  • Observation depth (Causal Diffing, Audits)          │
        │  • Interaction breadth (Forms, Drag-Drop, etc.)        │
        │  • Production safety (Read-Only, Isolation, etc.)      │
        │  • DX/Workflow (CI/CD, IDE, GitHub/Jira, etc.)         │
        │  • Quality (Fuzz tests, test harness)                  │
        │                                                         │
        │  Note: Can start during Wave 2, don't block v6.0       │
        └─────────────────────────────────────────────────────────┘
```

---

## Parallelization Strategy

**Phase 1 (Complete):** ✅ All v5.2 bugs fixed
- ✅ Query DOM implementation
- ✅ Accessibility audit runtime error fix
- ✅ Network bodies capture fix
- ✅ Extension timeout fix
- ✅ Tab ID attached to all responses

**Phase 2 (After v5.2):** Build v6.0 thesis (Wave 1 + Wave 2)
- **Wave 1:** 3 parallel agents, 4-6 weeks
- **Wave 2:** 3 parallel agents, 4-6 weeks
- **Release v6.0** when both waves complete

**Phase 3 (Concurrent with Wave 2):** Start v6.1+ features (1-2 parallel agents, non-blocking)
- Observation improvements (Causal Diffing, DOM Fingerprinting, Audits)
- Can start mid-Wave 2 if agents available
- Don't block v6.0 release

**Maximum parallelization:** 3 agents on v6.0 critical path, 1-2 agents on v6.1+ concurrent work

---

## Completed Features (Canonical List)

All shipped features as of v5.1.0. This is the single source of truth. See also [features/FEATURE-INDEX.md](features/FEATURE-INDEX.md) for the machine-readable table.

### Core Observation (observe)

| Feature | Mode | Version | Description |
|---------|------|---------|-------------|
| API Schema Inference | api | 5.0.0 | Infer API schemas from observed network traffic |
| Binary Format Detection | network_bodies | 5.0.0 | Detect and label binary response formats |
| Compressed Diffs | changes | 5.0.0 | Compact before/after diffs for state changes |
| Error Clustering | error_clusters | 5.0.0 | Group related errors by pattern |
| Performance Budget | performance | 5.0.0 | Observe performance metrics against budgets |
| Push Alerts | (alert system) | 5.0.0 | Push significant events to AI |
| Push Regression | performance | 5.0.0 | Detect performance regressions across sessions |
| Temporal Graph | history | 5.0.0 | Time-series event graph with causal links |
| Web Vitals | vitals | 5.0.0 | Core Web Vitals (LCP, CLS, INP, FCP, TTFB) |
| Accessibility Audit | accessibility | 5.0.0 | Axe-core accessibility scanning |
| Tab Targeting | tabs | Pre-v5 | `tab_id` parameter on all tools |
| API Contract Validation | validate_api | Pre-v5 | Track response shapes, detect contract violations |
| Verification Loop | (verify_fix) | Pre-v5 | Before/after session comparison for fix verification |
| Health Metrics | health | Pre-v5 | Server uptime, buffer utilization, memory usage |
| Session Comparison | diff_sessions | Pre-v5 | Named snapshot storage and comparison |
| Security Scanner | security_audit | Pre-v5 | Credentials, PII, insecure transport, headers, cookies |
| Security Diff | security_diff | Pre-v5 | Security posture comparison before/after changes |
| Third-Party Audit | third_party_audit | Pre-v5 | External domain mapping, risk classification |

### Generation (generate)

| Feature | Mode | Version | Description |
|---------|------|---------|-------------|
| HAR Export | har | 5.0.0 | Export network waterfall as HAR archive |
| Reproduction Enhancements | reproduction, test | 5.0.0 | Generate reproduction steps and test code |
| SARIF Export | sarif | 5.0.0 | Static analysis results interchange format |
| CSP Generator | csp | Pre-v5 | Content-Security-Policy from observed origins |
| SRI Hash Generator | sri | Pre-v5 | Subresource Integrity hashes for third-party resources |

### Configuration (configure)

| Feature | Mode | Version | Description |
|---------|------|---------|-------------|
| AI Capture Control | capture | 5.0.0 | Enable/disable specific capture categories |
| Memory Enforcement | health | 5.0.0 | Hard memory caps with graceful degradation |
| Noise Filtering | noise_rule, dismiss | 5.0.0 | Suppress known-noisy entries |
| Persistent Memory | store, load, record_event | 5.0.0 | Cross-session key-value and event storage |
| Rate Limiting | (throttling) | 5.0.0 | Per-tool rate limits |
| Redaction Patterns | (data masking) | 5.0.0 | User-defined regex for sensitive data |
| Security Hardening | (security config) | 5.0.0 | Localhost binding, header stripping, input validation |
| TTL Retention | (data TTL) | 5.0.0 | Time-to-live auto-eviction of buffer entries |
| Enterprise Audit | audit_log | 5.0.0 | Ring-buffer log of all MCP tool calls |
| API Key Auth | (request validation) | 5.0.0 | Auto-generated API key authentication |

### Interaction (interact)

| Feature | Mode | Version | Description |
|---------|------|---------|-------------|
| AI Web Pilot | highlight, save_state, load_state, execute_js, navigate | 5.0.0 | Full browser automation for AI agents |

---

## Deferred Features

These features were originally planned for v5.0-v5.1 but have been deferred pending v6.0 completion and team bandwidth prioritization.

| Feature | Planned Version | Description |
|---------|-----------------|-------------|
| MCP Tool Descriptions | 5.0.0 | LLM-optimized tool schema with usage examples |
| Usability Improvements | 5.0.0 | NPM/PyPI install, 5-minute setup, first-run banner |
| Single-Tab Tracking | 5.1.0 | Security: isolate capture to explicitly tracked tab |
| Network Schema Improvements | 5.1.0 | Unit suffixes, compression ratios, timestamps |

**Status:** Deprioritized. Review for inclusion in v6.1+ or later releases pending v6.0 completion.
