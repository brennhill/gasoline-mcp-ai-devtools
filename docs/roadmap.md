# Gasoline Versioning & Roadmap

**Single source of truth. For strategic analysis, see [ROADMAP-REORGANIZATION.md](ROADMAP-REORGANIZATION.md).**

---

## Thesis

**AI will be the driving force in development.**

Gasoline's strategic differentiator is enabling AI to **close the feedback loop autonomously** — observe, diagnose, and repair without human intervention. Every feature is evaluated against this thesis.

---

## Release Strategy

- **v5.2** — ✅ All critical bugs fixed (v5.1 blockers). Ready to release.
- **v5.3** — Critical usability fixes blocking AI workflows (pagination, buffer-specific clearing). **Must ship before v6.0.**
- **v6.0** — Release when core thesis is complete (Wave 1 + Wave 2 features). Single point release. **Marketing moment: "AI closes the feedback loop autonomously."**
- **v6.1-6.8** — Post-thesis roadmap. Tier-based: moat → enterprise → production → optimization.
- **v7** — If all roadmap features shipped, bump to v7 to signal full-featured product.

---

## Strategic Problem Space

### A. Context / Token Inefficiency
**Problem:** Chrome DevTools and similar tools dump raw browser state (massive DOM trees, accessibility dumps, long logs) that blow context windows.

**Why competitors fail:** They expose everything, interpret nothing. MCP is plumbing, not intelligence.

**Gasoline's solution:** Semantic debugging context. Decide what matters before the model sees it.

✅ **Solved when:** Typical debugging session fits in <25% of context window. "10× less context than DevTools MCP."

---

### B. Shallow Debugging (Symptoms, Not Causes)
**Problem:** Most tools surface symptoms ("There's an error") but don't answer why.

**Why competitors fail:** They stop at observation. Root-cause analysis is left to humans.

**Gasoline's solution:** Causal debugging. Connect: User action → DOM mutation → network call → backend response → frontend failure.

✅ **Solved when:** AI can answer "The bug exists because X changed, which broke Y, which surfaces as Z."

---

### C. Weak Feedback Loops (No "Fix → Verify → Done")
**Problem:** Most AI debugging is one-shot: observe → suggest → hope it worked.

**Why competitors fail:** They treat debugging as analysis, not a loop.

**Gasoline's solution:** Closed-loop debugging. Apply/simulate fix → re-run scenario → confirm resolved automatically.

✅ **Solved when:** Bugs marked "resolved" automatically. "This fix removed the error across 3 retries."

---

### D. Garbage In → Garbage Out Selectors & Tests
**Problem:** AI generates brittle selectors (nth-child, random classes) that fail on refactor.

**Why competitors fail:** They don't understand UI semantics — only DOM structure.

**Gasoline's solution:** Selector intelligence + semantic anchoring. Prefer roles, labels, stable attributes.

✅ **Solved when:** Generated tests survive minor UI refactors. Test flakiness drops materially.

---

### E. Raw Data Instead of Developer-Ready Output
**Problem:** Tools dump logs, traces, screenshots. Developers still have to think.

**Why competitors fail:** They optimize for machine access, not human comprehension.

**Gasoline's solution:** First-class bug reports written by AI. Readable by humans, trusted by teams.

✅ **Solved when:** Bug reports can be pasted directly into GitHub/Jira. No follow-up questions needed.

---

### F. Unsafe / Awkward Production Debugging
**Problem:** Most tools assume local dev, no sensitive data, one user. Reality is different.

**Why competitors fail:** They weren't designed for prod safety from day one.

**Gasoline's solution:** Production-safe AI debugging. Data redaction, session isolation, read-only modes.

✅ **Solved when:** Security teams approve prod usage. Engineers debug real user bugs safely.

---

## v5.2: Immediate (Bug Fixes)

✅ **Complete** — All critical v5.1 blockers fixed and ready to release.

---

## v5.3: Critical Blockers (Before v6.0)

**Goal:** Fix usability issues preventing AI workflows today.

### 1. Pagination for Large Datasets ⭐⭐⭐ HIGH PRIORITY

**Problem:** Network waterfall and other buffers return >440K characters, exceeding MCP token limits. AI cannot analyze network traffic.

**Solution:** Add `offset` and `limit` parameters:
```javascript
observe({what: "network_waterfall", limit: 100})
observe({what: "network_waterfall", offset: 100, limit: 100})
```

**Impact:** Solves token limit issue. Enables AI to query large datasets in chunks.

**Effort:** 2-4 hours

---

### 2. Buffer-Specific Clearing ⭐⭐⭐ HIGH PRIORITY

**Problem:** `configure({action: "clear"})` only clears console logs. Network/WebSocket/action buffers accumulate indefinitely.

**Solution:** Add buffer parameter:
```javascript
configure({action: "clear", buffer: "network"})    // Clear network waterfall
configure({action: "clear", buffer: "websocket"})  // Clear WebSocket events
configure({action: "clear", buffer: "actions"})    // Clear user actions
configure({action: "clear", buffer: "all"})        // Clear all buffers
```

**Impact:** Granular buffer management. Prevents memory bloat.

**Effort:** 1-2 hours

---

### 3. Server-Side Aggregation (Future) ⭐⭐

**Problem:** Even with pagination, large datasets are verbose. Need summary views.

**Solution:** Add aggregation endpoints:
```javascript
observe({what: "network_stats", group_by: "host"})
→ {"localhost:3000": {count: 150, avg_duration: 45ms, errors: 3}}
```

**Impact:** Compact representation. Useful for overview/analysis.

**Effort:** 4-6 hours. **Deferred until pagination proves insufficient.**

---

## v6.0: Core Thesis Release

**Goal:** Prove the thesis — AI closes the feedback loop autonomously.

**Release criteria:** Wave 1 + Wave 2 features shipped and battle-tested.

### Wave 1: Foundations (3 features, parallel)

1. **Self-Healing Tests** (#33) — AI observes test failure → diagnoses via Gasoline → fixes code/test → verifies
2. **Gasoline CI Infrastructure** — Enable autonomous loops in CI/CD (`/snapshot`, `/clear`, `/test-boundary`, Playwright fixtures)
3. **Context Streaming** (#5) — Real-time push notifications instead of raw data dumps

### Wave 2: Expansion (3 features, parallel after Wave 1)

4. **PR Preview Exploration** (#35) — Deploy preview → explore → discover bugs → propose fixes
5. **Agentic E2E Repair** (#34) — Detect API drift → auto-fix tests/mocks
6. **Deployment Watchdog** (#36) — Post-deploy monitoring → auto-rollback on regression

### Marketing Message

- "AI autonomously fixes tests, not just suggests fixes"
- "Closed-loop verification: fix → test → confirm → done"
- "10× less context than Chrome DevTools MCP"

---

## v6.1+: Post-Thesis Roadmap (Tier Strategy)

**Organization:** Features grouped by tier. Each tier validates part of the thesis and unlocks the next market segment.

**See [ROADMAP-REORGANIZATION.md](ROADMAP-REORGANIZATION.md) for detailed strategic analysis.**

---

## Tier 1: Core Moat — Smart Observation & Safe Repair (v6.0-6.2)

**Goal:** Validate thesis — AI closes the feedback loop autonomously (observe → diagnose → repair → verify).

**Competitive Differentiator:** What Chrome DevTools and other AI agents cannot do.

### v6.1: Smart Observation

**Solves:** Problems A (tokens) + B (causality) + D (selectors)

- **Visual-Semantic Bridge** — Computed layout maps. Auto-generate unique test-IDs. Solves "ghost clicks" and "hallucinated selectors." [Spec](features/feature/visual-semantic-bridge/PRODUCT_SPEC.md)
- **State "Time Travel"** — Persistent event buffer. Before/after snapshots. Enables causal debugging across crashes. [Spec](features/feature/state-time-travel/PRODUCT_SPEC.md)
- **Causal Diffing** — Root-cause analysis: "X changed → broke Y → surfaces as Z"
- **Reverse Engineering Engine** — Auto-infer OpenAPI specs from network traffic
- **Design System Injector** — Inject design tokens; stop AI from writing garbage CSS
- **Deep Framework Intelligence** — React/Vue component tree + props (not HTML soup)
- **DOM Fingerprinting** — Stable selectors survive UI refactors
- **Smart DOM Pruning** — Remove noise; fit DOM in <25% context window
- **Hydration Doctor** — Debug SSR mismatches (Next.js/Nuxt)

### v6.2: Safe Repair & Verification

**Solves:** Problems C (loops) + F (safety)

- **Prompt-Based Network Mocking** — AI instructs Gasoline to mock responses. Verify-fix-verify loops.
- **Shadow Mode** — Intercept POST/PUT/DELETE; return fake 200. Test destructive ops safely.
- **Pixel-Perfect Guardian** — Reject changes that cause CSS regression
- **Healer Mode** — Auto-convert bug fixes into permanent regression tests

---

## Tier 2: Enterprise Unlock — Zero-Trust Safety & Team Collaboration (v6.3)

**Goal:** Enable corporate adoption with safety guarantees and knowledge sharing.

**Competitive Differentiator:** Safety-first design. Enterprises can trust AI agents.

### v6.3: Zero-Trust Enterprise

- **Zero-Trust Sandbox** — Action gating, data masking, prompt injection defense
- **Asynchronous Multiplayer Debugging** — Flight Recorder links for crash sharing
- **Session Replay Exports** — .gas files for hand-off to humans or better AI models

---

## Tier 3: Production Ready — Compliance & Integration (v6.4)

**Goal:** Enable production debugging at scale with governance.

**Competitive Differentiator:** Compliance-ready. Audit trails. Multi-tenant isolation.

### v6.4: Production Compliance

- **Read-Only Mode** — Non-mutating observation in production
- **Tool Allowlisting** — Restrict which MCP tools run
- **Project Isolation** — Multi-tenant capture contexts
- **Configuration Profiles** — Pre-tuned bundles (paranoid, restricted, short-lived)
- **Redaction Audit Log** — Compliance logging with automatic data redaction
- **GitHub/Jira Integration** — Paste-ready bug reports
- **CI/CD Integration** — GitHub Actions, SARIF export, HAR attachment
- **IDE Integration** — VS Code plugin, Claude Code integration
- **Event Timestamps & Session IDs** — Audit trails, precise ordering
- **CLI Lifecycle Commands** — `stop`, `restart`, `status` for ops

---

## Tier 4: Optimization & Specialization — Continuous Shipping (v6.5+)

**Goal:** Continuous incremental value. Non-blocking. Start after v6.2.

### v6.5: Token & Context Efficiency

**Solves:** Problem A — reduce costs

- **Focus Mode** — "Tunnel Vision": AI selects component; Gasoline sends only that. 90% token reduction.
- **Semantic Token Compression** — Lightweight references (@button1, @input2). JIT context pruning.

### v6.6: Specialized Audits & Analytics

**Adjacent features:** Domain-specific superpowers

- **Performance Audit** — Root-cause perf issues
- **Best Practices Audit** — Structural/deprecated/security issues
- **SEO Audit** — Metadata, heading structure, structured data
- **A11y Tree Snapshots** — Accessibility compression
- **Enhanced WCAG Audit** — Deep accessibility beyond axe-core
- **Annotated Screenshots** — Visual context for vision models

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

## v7: Complete Roadmap Delivery

If/when all features shipped:
- v7.0 released as "full-featured" version
- Signal market maturity
- All 40+ features working together

---

## Roadmap Dependencies & Sequencing

### Critical Path (must serialize)
```
v5.2 (bugs) → v5.3 (blockers) → v6.0 (thesis) → v6.1 (moat) → v6.2 (repair) → v6.3 (enterprise) → v6.4 (prod)
```

### Parallel Tracks (can start after v6.2)
```
v6.5 (efficiency), v6.6 (audits), v6.7 (interactions), v6.8 (infra) — all run concurrently
```

### Team Allocation
- **v5.2-6.0:** All hands on critical path (thesis validation)
- **v6.1-6.2:** 2-3 agents (moat features, cannot be parallelized)
- **v6.3-6.4:** 1-2 agents (enterprise/compliance, can overlap with v6.1-6.2)
- **v6.5-6.8:** 1 agent per tier (continuous shipping, true parallelization)

---

## Build Plan: v6.0 Maximum Parallelization

**After v5.2+v5.3 complete:**

```
WAVE 1 (3 agents in parallel, 4-6 weeks)
├── Agent A: Self-Healing Tests (#33)
├── Agent B: Gasoline CI Infrastructure
└── Agent C: Context Streaming (#5)
    ↓ (all complete, merge to next)

WAVE 2 (3 agents in parallel, 4-6 weeks)
├── Agent A: PR Preview Exploration (#35)
├── Agent B: Agentic E2E Repair (#34)
└── Agent C: Deployment Watchdog (#36)
    ↓ (all complete)

v6.0 RELEASE → MARKET VALIDATION
```

---

## Marketing Milestones

| Release | Narrative | Evidence |
|---------|-----------|----------|
| **v6.0** | "AI closes the feedback loop autonomously" | Wave 1 + Wave 2 work in concert for E2E autonomy |
| **v6.2** | "Enterprise-safe autonomous debugging" | Zero-Trust Sandbox + Healer Mode + network mocking |
| **v6.4** | "Production-ready with compliance" | Audit log + GitHub/Jira + CI/CD + read-only mode |
| **v6.5+** | "Specialized debugging superpowers" | Token compression + audits + advanced interactions |

---

## Key Principles

1. **Thesis-First:** Every feature must advance "AI closes the loop autonomously"
2. **Tier Clarity:** Features organized by competitive value and market unlock
3. **Serial Critical Path:** v5.2 → v5.3 → v6.0 → v6.2 → v6.3 → v6.4 (dependencies)
4. **Parallel Polish:** v6.5-6.8 run concurrently, non-blocking
5. **Problem Mapping:** All 6 strategic problems (A-F) addressed by specific tiers

---

## Out of Scope (Deferred to v7+)

- Machine learning-based root cause inference (rule-based linking sufficient)
- Video replay or animated debugging (screenshots sufficient)
- Embedded database for offline analysis (session export sufficient)
- Real-time collaboration (Flight Recorder sufficient)
- Custom browser engines or protocol modifications
