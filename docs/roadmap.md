# v6 Roadmap

## Thesis

**AI will be the driving force in development.**

Gasoline's strategic differentiator is enabling AI to **close the feedback loop autonomously** — observe, diagnose, and repair without human intervention. Every feature is evaluated against this thesis.

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

## v6 Roadmap of Priorities

This is sequenced to maximize trust → leverage → defensibility.

### Phase 1: Trust & Core Value (Foundational)

**Goal:** "This actually debugs better than me."

**Must-have**

1. Semantic context reduction (token efficiency)
2. Causal root-cause analysis
3. Human-readable bug summaries

If you fail here, nothing else matters.

### Phase 2: Closed-Loop Power

**Goal:** "This fixes bugs, not just explains them."

**Build next**

4. Automated fix + verify loop
5. Stable selector generation
6. Replayable failing scenarios

This is where competitors really fall off.

### Phase 3: Workflow & Adoption

**Goal:** "This fits how teams actually work."

**Then add**

7. GitHub/Jira-ready bug reports
8. CI/CD integration
9. IDE handoff (VS Code, etc.)

### Phase 4: Moat & Enterprise Pull

**Goal:** "You can't replace this with another MCP."

**Differentiators**

10. Production-safe debugging modes
11. Cross-browser / mobile
12. Historical learning ("we've seen this bug before")

---

## v5.2: Immediate Priority

These are known bugs and UX issues from UAT. Must be resolved before v6 feature work.

See [KNOWN-ISSUES.md](../KNOWN-ISSUES.md) for user-facing summary and [docs/core/in-progress/UAT-ISSUES-TRACKER.md](core/in-progress/UAT-ISSUES-TRACKER.md) for investigation notes.

### Bug Fixes

| # | Severity | Issue | Status |
|---|----------|-------|--------|
| 2 | High | `query_dom` not implemented — schema advertises it but background.js returns `not_implemented` | Open |
| 3 | High | Accessibility audit runtime error — `runAxeAuditWithTimeout` "not defined" at runtime | Open |
| 4 | Medium | `network_bodies` returns no data — empty arrays on multiple page loads | Open |
| 5 | Medium | Extension timeouts after 5-6 operations — possible message queue backup or memory leak | Open |
| 6 | Medium | `observe()` missing tabId — content.js attaches it but server doesn't surface in MCP responses | Open |

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

## Priority 1: Agentic CI/CD (Thesis Validation)

These features prove the thesis. Build now.

### Parallelization

```
┌─────────────────────────────────────────────────────────────────┐
│                     CAN BUILD IN PARALLEL                        │
├─────────────────────┬─────────────────────┬─────────────────────┤
│  Agent A            │  Agent B            │  Agent C            │
│                     │                     │                     │
│  33. Self-Healing   │  Gasoline CI        │  5. Context         │
│      Tests          │  Infrastructure     │     Streaming       │
│                     │                     │                     │
│  - Claude Code      │  - /snapshot        │  - MCP notifications│
│    integration      │  - /clear           │  - Push alerts      │
│  - Failure diagnosis│  - /test-boundary   │  - Event filtering  │
│  - Auto-fix loop    │  - gasoline-ci.js   │                     │
│                     │  - Playwright fix   │                     │
└─────────────────────┴─────────────────────┴─────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                     THEN IN PARALLEL                             │
├─────────────────────┬─────────────────────┬─────────────────────┤
│  Agent A            │  Agent B            │  Agent C            │
│                     │                     │                     │
│  35. PR Preview     │  34. Agentic E2E    │  36. Deployment     │
│      Exploration    │      Repair         │      Watchdog       │
│                     │                     │                     │
│  - Requires: 33     │  - Requires: 33     │  - Requires: 5, CI  │
│  - Preview deploy   │  - Contract drift   │  - Post-deploy mon  │
│  - Auto-explore     │  - Test/mock update │  - Auto-rollback    │
└─────────────────────┴─────────────────────┴─────────────────────┘
```

### Features

- [ ] **33. Self-Healing Tests** — AI observes test failure, diagnoses via Gasoline context, fixes test or code autonomously
  - Branch: `feature/self-healing-tests`
  - Spec: [ai-first/tech-spec-agentic-cicd.md](ai-first/tech-spec-agentic-cicd.md)
  - Prerequisites: ✅ Tab targeting, ✅ Verification loop
  - Unlocks: CI that unblocks itself

- [ ] **Gasoline CI Infrastructure** — Headless browser capture for CI/CD pipelines
  - Spec: [gasoline-ci-specification.md](gasoline-ci-specification.md)
  - Components:
    - [ ] `/snapshot` endpoint - return all captured state
    - [ ] `/clear` endpoint - reset between tests
    - [ ] `/test-boundary` endpoint - correlate entries to tests
    - [ ] `gasoline-ci.js` - standalone capture script (extract from inject.js)
    - [ ] `@gasoline/playwright` fixture - auto-inject, auto-clear, failure attachment
  - Unlocks: Phase 8 features in CI, not just local browser

- [ ] **5. Context Streaming** — Push significant events to AI via MCP notifications
  - Branch: `feature/context-streaming`
  - Spec: v6-specification.md § Feature 5
  - Prerequisites: None
  - Unlocks: Proactive alerts, enables Deployment Watchdog

- [ ] **35. PR Preview Exploration** — Deploy preview → agent explores app → discovers bugs → proposes fixes pre-merge
  - Branch: `feature/pr-preview-exploration`
  - Spec: [ai-first/tech-spec-agentic-cicd.md](ai-first/tech-spec-agentic-cicd.md)
  - Prerequisites: ✅ Tab targeting, Self-Healing Tests (33)
  - Unlocks: Automated QA on every PR

- [ ] **34. Agentic E2E Repair** — AI detects API contract drift, updates tests/mocks automatically
  - Branch: `feature/agentic-e2e-repair`
  - Spec: [ai-first/tech-spec-agentic-cicd.md](ai-first/tech-spec-agentic-cicd.md)
  - Prerequisites: ✅ API contract validation, Self-Healing Tests (33)
  - Unlocks: Zero-maintenance E2E suites

- [ ] **36. Deployment Watchdog** — Post-deploy monitoring; AI detects regressions, triggers rollback
  - Branch: `feature/deployment-watchdog`
  - Spec: [ai-first/tech-spec-agentic-cicd.md](ai-first/tech-spec-agentic-cicd.md)
  - Prerequisites: ✅ Session comparison, Context Streaming (5), Gasoline CI
  - Unlocks: Self-healing production

---

## Priority 2: Enterprise Unlock

Required for team/enterprise sales. Core enterprise features shipped in v5.0.

### Features

- [x] **16. TTL-Based Retention** — Configurable time-to-live; buffers auto-evict old entries ✅ Shipped v5.0
- [x] **19. Configurable Redaction Patterns** — User-defined regex for sensitive data ✅ Shipped v5.0
- [x] **20. API Key Authentication** — Auto-generated key on startup, `--api-key` and `--no-auth` flags ✅ Shipped v5.0
- [x] **21. Per-Tool Rate Limits** — Prevent runaway AI loops ✅ Shipped v5.0
- [x] **Enterprise Audit** — Audit log, security scanner, redaction log ✅ Shipped v5.0
- [x] **Memory Enforcement** — Hard memory caps with graceful degradation ✅ Shipped v5.0
- [x] **Security Hardening** — Localhost binding, header stripping, input validation ✅ Shipped v5.0

### Remaining

- [ ] **17. Configuration Profiles** — Named bundles (short-lived, restricted, paranoid)
  - Branch: `feature/config-profiles`
  - Complexity: Medium
  - Prerequisites: 16 ✅, 19 ✅, 21 ✅ — all shipped, ready to build
  - Sales unlock: "Bank mode" one-click setup

---

## Priority 3: Enhanced Generation

Improves quality of AI-generated artifacts. Build when self-healing is working.

- [ ] **6. Test Generation v2** — DOM assertions, fixtures, visual snapshots
  - Branch: `feature/generate-test-v2`
  - Spec: generate-test-v2.md
  - Thesis connection: Better generated tests = better self-healing input

- [ ] **7. Performance Budget Monitor** — Baseline regression detection
  - Branch: `feature/performance-budget-monitor`
  - Spec: performance-budget-spec.md
  - Thesis connection: Weak — perf monitoring isn't AI-native

---

## Priority 4: Operational Polish

Build as needed. Low thesis impact.

### Enterprise Audit (Tier 1 extras)

- [ ] **13. Client Identification** — Identify which AI client (Claude Code, Cursor, etc.)
- [ ] **14. Session ID Assignment** — Unique session ID per MCP connection
- [ ] **15. Redaction Audit Log** — Log when data is redacted (pattern, field, tool)

### Enterprise Multi-Tenant (Tier 4)

- [ ] **24. Project Isolation** — Multiple isolated capture contexts on one server
- [ ] **25. Read-Only Mode** — Accept capture data, disable mutation tools
- [ ] **26. Tool Allowlisting** — Restrict which MCP tools are available

### Developer Experience (Phase 7)

- [ ] **27. Test Fixture Page** — Built-in `/test-page` with error triggers
- [ ] **28. CLI Test Mode** — `--test` flag for automated validation
- [ ] **29. Mock Extension Client** — Go package simulating extension calls
- [ ] **30. Event Timestamps in Diagnostics** — `received_at` in `/diagnostics`
- [ ] **31. MCP Test Harness** — CLI for scripted MCP testing
- [ ] **32. CLI Lifecycle Commands** — `gasoline stop`, `restart`, `status`

### Data Export

- [ ] **18. Data Export** — Export buffer state as JSON Lines

---

## Internal Quality

### Fuzz Tests

Build incrementally as attack surface grows.

- [ ] **FuzzJSONRPCParse** — MCP message parser (no panics, no unbounded alloc)
- [ ] **FuzzHTTPBodyParse** — `/logs`, `/network-body` endpoints
- [ ] **FuzzSecurityPatterns** — Credential/PII regex (no catastrophic backtracking)
- [ ] **FuzzWebSocketFrame** — WS message handling
- [ ] **FuzzNetworkBodyStorage** — Large/malformed body storage

---

## In Progress

| Feature | Tool | Mode | Notes |
|---------|------|------|-------|
| Behavioral Baselines | observe | performance | Baseline regression detection |
| Budget Thresholds | configure | health | Configurable alert thresholds |
| Causal Diffing | observe, generate | changes | Root-cause change analysis |
| DOM Fingerprinting | interact, observe | query_dom | Stable selector generation |
| Interception Deferral | observe, configure | (network buffering) | Deferred body capture |
| Self-Testing | interact, generate | execute_js, test | Extension self-validation |
| SPA Route Measurement | observe | performance, timeline | Single-page app route timing |

---

## Dependency Graph

```
                    ┌──────────────────────────────────────────────────────┐
                    │                   COMPLETED                           │
                    │  Tab Targeting, API Validation, Verify Fix,          │
                    │  Session Diff, Security Tools, Audit Log             │
                    └──────────────────────────────────────────────────────┘
                                            │
                    ┌───────────────────────┼───────────────────────┐
                    │                       │                       │
                    ▼                       ▼                       ▼
            ┌───────────────┐       ┌───────────────┐       ┌───────────────┐
            │ 33. Self-     │       │ Gasoline CI   │       │ 5. Context    │
            │ Healing Tests │       │ Infrastructure│       │ Streaming     │
            └───────┬───────┘       └───────┬───────┘       └───────┬───────┘
                    │                       │                       │
        ┌───────────┴───────────┐           │               ┌───────┴───────┐
        │                       │           │               │               │
        ▼                       ▼           │               │               │
┌───────────────┐       ┌───────────────┐   │               │               │
│ 35. PR Preview│       │ 34. Agentic   │   │               │               │
│ Exploration   │       │ E2E Repair    │   │               │               │
└───────────────┘       └───────────────┘   │               │               │
                                            │               │               │
                                            └───────┬───────┘               │
                                                    │                       │
                                                    ▼                       │
                                            ┌───────────────┐               │
                                            │ 36. Deployment│◄──────────────┘
                                            │ Watchdog      │
                                            └───────────────┘
```

---

## Maximum Parallelization

**Wave 1 (Next):** 3 agents — v5.2 bug fixes + v6 foundations
- Agent A: Self-Healing Tests (33)
- Agent B: Gasoline CI Infrastructure
- Agent C: Context Streaming (5)

**Wave 2 (After Wave 1):** 3 agents
- Agent A: PR Preview Exploration (35)
- Agent B: Agentic E2E Repair (34)
- Agent C: Deployment Watchdog (36)

~~**Wave 3 (Enterprise):** TTL, Redaction, API Key, Rate Limits~~ ✅ **All shipped in v5.0**

**Total: Up to 3 parallel agents** can work productively on v6 at any time.

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

### Infrastructure

| Feature | Version | Description |
|---------|---------|-------------|
| MCP Tool Descriptions | 5.0.0 | LLM-optimized tool schema with usage examples |
| Usability Improvements | 5.0.0 | NPM/PyPI install, 5-minute setup, first-run banner |
| Single-Tab Tracking | 5.1.0 | Security: isolate capture to explicitly tracked tab |
| Network Schema Improvements | 5.1.0 | Unit suffixes, compression ratios, timestamps |
