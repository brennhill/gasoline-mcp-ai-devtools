---
status: proposed
scope: v6-v7-strategy
ai-priority: critical
tags: [v6, v7, roadmap, feature-taxonomy, 360-observability, ears-eyes-hands]
relates-to: [roadmap.md, ai-native-testing-philosophy.md, backend-frontend-unification.md]
last-verified: 2026-01-31
---

# Gasoline 360° Observability: Complete Feature Taxonomy for v6 & v7

**Master planning document for Gasoline's evolution from single-app AI-native testing (v6) to full-stack AI debugging (v7).**

---

## Vision Statement

Gasoline is evolving into **360° AI observability for feature development and test automation**.

**v6:** Single-app AI-native testing. AI reads specs, explores UIs, finds bugs, fixes autonomously.

**v7:** Full-stack AI debugging. AI understands entire system (browser + backend + tests + git), traces root causes, validates contracts, prevents breaking changes.

---

## The Problem We're Solving

AI needs answers while actively developing and testing:

| Question | v5 | v6 | v7 |
|----------|-----|-----|-----|
| **"What happens when I change this?"** | ❌ Manual | ✅ Impact analysis (single-app) | ✅ Full impact (multi-service) |
| **"Will this break critical paths?"** | ❌ Manual | ✅ Checkpoint validation | ✅ Dependency graph + contracts |
| **"Why did this fail?"** | 🟡 Browser only | ✅ Browser + local backend | ✅ Full causality chain |
| **"Should I retry or try different approach?"** | ❌ Manual | ✅ Doom loop detection | ✅ Semantic understanding |
| **"Did I accidentally break something else?"** | ❌ Manual | ✅ Regression detection | ✅ Cross-service validation |

---

## v6 Features: AI-Native Single-App Testing

**Goal:** Prove AI can autonomously validate and fix web applications through exploration, observation, and intelligent iteration.

**Philosophy:** Don't make LLMs write better tests. Make LLMs better at understanding and fixing web applications.

### TIER 1: Core Observability (Browser + Local Backend)

#### **EARS: Data Ingestion**

| Feature | What It Does | Why It Matters | v5.x | v6.0 | Effort |
|---------|-------------|---|-----|------|--------|
| **Enhanced Browser Telemetry** | Console logs, network bodies, WebSocket events, DOM snapshots, user actions | AI sees everything that happens | ✅ Partial | ✅ Expand | 1 week |
| **Local Backend Log Streaming** | Dev server + Docker + process logs, unified ingestion | AI sees why backend failed | ❌ | ✅ NEW | 2 weeks |
| **Test Execution Capture** | npm test, pytest, go test output, test names, pass/fail | AI knows which tests broke | ❌ | ✅ NEW | 1.5 weeks |
| **Application Events API** | Developers inject `__gasoline.event({name, metadata})` | AI understands business logic | ❌ | ✅ NEW | 1 week |

#### **EYES: Correlation & Understanding**

| Feature | What It Does | Why It Matters | v5.x | v6.0 | Effort |
|---------|-------------|---|-----|------|--------|
| **Unified Execution Timeline** | Single view: browser + network + tests + backend logs | AI sees complete picture | 🟡 Partial | ✅ Expand | 1.5 weeks |
| **Request Tracing** | Link browser request to backend handling by timestamp | AI traces impact of action | ❌ | ✅ NEW | 1 week |
| **State Snapshots & Checkpoints** | Save/restore DOM state, compare before/after | AI detects regressions | ❌ | ✅ NEW | 1.5 weeks |
| **Doom Loop Detection** | Track execution history, recognize retry patterns | AI avoids infinite loops | ❌ | ✅ NEW | 1 week |
| **Edge Case Registry** | Project defines critical edge cases, track testing frequency | AI knows what matters | ❌ | ✅ NEW | 0.5 weeks |

#### **HANDS: Action Capabilities**

| Feature | What It Does | Why It Matters | v5.x | v6.0 | Effort |
|---------|-------------|---|-----|------|--------|
| **Enhanced Browser Control** | Navigate, click, fill, modify state (storage, cookies) | AI can test edge cases | 🟡 Partial | ✅ Expand | 1 week |
| **Local Dev Environment Control** | Mock APIs, inject delays, restart server, modify env vars | AI can reproduce prod bugs locally | ❌ | ✅ NEW | 1.5 weeks |
| **Test Generation & Self-Healing** | Generate Playwright tests, auto-fix broken selectors | AI creates regression tests | ❌ | ✅ NEW | 2 weeks |
| **Code Navigation (Light)** | Show related code, read/diff files, inject logging | AI understands context | ❌ | ✅ NEW | 0.5 weeks |

---

### TIER 2: AI-Native Development Helpers

| Feature | What It Does | Why It Matters | Effort |
|---------|-------------|---|--------|
| **Specification Validation** | Developer provides spec, AI explores UI against it, reports matches/gaps | AI can validate feature requirements | 1 week |
| **Critical Path Definition** | User identifies critical journeys: "login → checkout → payment" | AI knows what mustn't break | 0.5 weeks |
| **Smart Test Recommendations** | Based on code change, suggest which tests to run | AI prioritizes important scenarios | 1 week |
| **Regression Prevention** | Checkpoint-based snapshots, replay after changes, detect regressions | AI catches side effects | 1 week |

---

### v6.0 Implementation Phases

#### **Phase 1: Wave 1 - AI-Native Toolkit (2-3 weeks)**

```
EXPLORE                OBSERVE                INFER
├─ interact.explore    ├─ observe.capture     ├─ analyze.infer
├─ interact.record     └─ observe.compare     └─ analyze.detect_loop
└─ interact.replay

Result: AI can explore → observe → infer → understand behavior
```

**Deliverables:**
- ✅ Enhanced browser telemetry (expand v5 capabilities)
- ✅ Local backend log streaming
- ✅ Execution timeline (unified view)
- ✅ Request tracing (by timestamp)
- ✅ State snapshots
- ✅ Doom loop detection
- ✅ Test execution capture

#### **Phase 2: Wave 2 - Basic Persistence (2-3 weeks)**

```
EXECUTION HISTORY              DOOM LOOP PREVENTION
├─ Track test results          ├─ Pattern detection
├─ Remember past attempts      └─ Suggest alternatives
└─ Enable learning

Result: AI remembers, avoids loops, learns from failures
```

**Deliverables:**
- ✅ Execution history tracking
- ✅ Doom loop detection + prevention
- ✅ Edge case registry
- ✅ Critical path definition
- ✅ Smart test recommendations

#### **Demo Scenarios (Proof of Thesis)**

**Demo 1: Spec-Driven Validation (v6.0)**
- Input: Product spec (markdown)
- AI: Reads spec → explores UI → validates behavior → fixes bugs
- Time: < 3 minutes, fully autonomous
- Proves: AI understands requirements without human guidance

**Demo 2: Feature Implementation with Checkpoint Validation (v6.0)**
- Input: Feature spec (non-breaking + breaking changes)
- AI: Records happy paths → implements features → replays checkpoints → detects expected vs unexpected changes → updates or fixes
- Time: < 5 minutes, fully autonomous
- Proves: AI can implement features while preserving critical paths

---

## v7 Features: Full-Stack AI Debugging

**Goal:** AI debugs entire stack (browser + backend + tests + git + infrastructure) as single coherent system.

**Philosophy:** Make root causes visible, validate contracts, prevent breaking changes proactively.

### TIER 3: Multi-Service Observability

#### **Phase 1: EARS - Backend Data Ingestion (4 features)**

| Feature | What It Does | Why It Matters | Effort |
|---------|-------------|---|--------|
| **Backend Log Streaming** | Ingest logs from multiple services in real-time | AI sees what each service did | 2 weeks |
| **Custom Event API** | Apps inject `gasoline.event()` with correlation IDs | AI links business logic across services | 1 week |
| **Test Execution Capture** | Capture test framework output (Jest, pytest, Mocha, go test) | AI knows which tests covered which code | 1.5 weeks |
| **Git Event Tracking** | File changes, commits, branches, linked to correlation IDs | AI knows "this broke 3 days ago" | 1 week |

#### **Phase 2: EYES - Semantic Correlation (4 features)**

| Feature | What It Does | Why It Matters | Effort |
|---------|-------------|---|--------|
| **Request/Session Correlation** | W3C Trace Context propagation, link browser → backend logs | AI traces action across services | 1.5 weeks |
| **Causality Analysis** | Root-cause chains, latency breakdown, gap detection | AI answers "why did this happen?" | 2 weeks |
| **Normalized Log Schema** | Unified JSON format for browser, backend, tests, git | AI queries single schema | 1.5 weeks |
| **Historical Snapshots** | Replayable full system state at any point in time | AI can "time travel" | 1 week |

**Exit Criteria:** AI can correlate [browser action] → [backend decision] → [test result]

#### **Phase 3: HANDS - Autonomous Control (4 features)**

| Feature | What It Does | Why It Matters | Effort |
|---------|-------------|---|--------|
| **Backend Control** | Restart services, clear state, run migrations, inject data | AI can test fixes end-to-end | 2 weeks |
| **Code Navigation & Modification** | Code search, read, modify, integrate tests | AI debugs code, not just symptoms | 1.5 weeks |
| **Environment Manipulation** | Toggle feature flags, mock services, switch databases | AI reproduces scenarios safely | 1 week |
| **Timeline & Search** | Unified timeline with microsecond precision, query all correlations | AI finds root cause quickly | 1.5 weeks |

**Exit Criteria:** AI can diagnose → fix → test → verify autonomously

---

### TIER 4: AI-Native Multi-Service Development

| Feature | What It Does | Why It Matters | Effort |
|---------|-------------|---|--------|
| **Contract-First Development** | Simplified JSON contracts + OpenAPI export | AI validates across services without integration tests | 1 week |
| **Cross-Service Test Generation** | Generate end-to-end tests spanning multiple services | AI ensures services work together | 1.5 weeks |
| **Edge Case Registry v2** | Project-specific edge cases (banking, e-commerce, social media) | AI knows domain-specific risks | 0.5 weeks |
| **Semantic Regression Detection** | Detect behavior changes that violate contracts | AI distinguishes "intentional" vs "breaking" | 1 week |
| **Dependency Graph Inference** | LLM infers, developer reviews, auto-updates | AI understands system topology | 1.5 weeks |
| **Impact Analysis** | "Service A changed, affects B, C, D — validate?" | AI proactively prevents breaking changes | 1 week |

---

## Implementation Priority Matrix

### **v6.0 MVP (Must Ship)**

**TIER 1 - EARS (1 week)**
- ✅ Enhanced browser telemetry (expand from v5)
- ✅ Local backend log streaming (v6 NEW)
- ✅ Test execution capture (v6 NEW)

**TIER 1 - EYES (1.5 weeks)**
- ✅ Unified execution timeline
- ✅ Request tracing (timestamp-based)
- ✅ State snapshots & checkpoints

**TIER 1 - HANDS (1.5 weeks)**
- ✅ Enhanced browser control
- ✅ Test generation & self-healing
- ✅ Local dev environment control

**TIER 2 - Persistence (1.5 weeks)**
- ✅ Execution history
- ✅ Doom loop detection
- ✅ Edge case registry

**TIER 2 - AI Helpers (1 week)**
- ✅ Specification validation framework
- ✅ Critical path definition

**Total: 4-6 weeks**

---

### **v6.1-6.2 (Expansion)**

- Advanced filtering (signal-to-noise)
- Visual-semantic bridge (smart selectors)
- State time travel (persistent event buffer)
- Causal diffing (why it broke)
- Smart test recommendations
- Regression prevention
- Network mocking & safe repair

---

### **v6.3-6.4 (Enterprise)**

- Zero-trust sandbox
- Read-only production mode
- GitHub/Jira integration
- CI/CD integration
- Audit trails
- Redaction policies

---

### **v7.0 (Full-Stack)**

**Phase 1: EARS (4 weeks)**
- Backend log streaming
- Custom event API
- Test execution capture
- Git event tracking

**Phase 2: EYES (4 weeks)**
- Request/session correlation
- Causality analysis
- Normalized log schema
- Historical snapshots

**Phase 3: HANDS (3 weeks)**
- Backend control
- Code navigation & modification
- Environment manipulation
- Timeline & search

**Total: 8-10 weeks**

---

## Feature Dependency Graph

```
v5.3 Browser Telemetry (✅ exists)
    ↓
┌───────────────────────────────────────┐
│ v6.0: AI-NATIVE SINGLE-APP TESTING   │
│                                       │
│ EARS:                                 │
│ ├─ Enhanced Browser Telemetry ✅      │
│ ├─ Local Backend Logs (NEW)           │
│ └─ Test Capture (NEW)                 │
│                                       │
│ EYES:                                 │
│ ├─ Unified Timeline ✅                │
│ ├─ Request Tracing (NEW)              │
│ └─ State Snapshots (NEW)              │
│                                       │
│ HANDS:                                │
│ ├─ Browser Control ✅                 │
│ ├─ Test Generation (NEW)              │
│ └─ Dev Environment Control (NEW)      │
│                                       │
│ PERSISTENCE:                          │
│ ├─ Execution History (NEW)            │
│ └─ Doom Loop Detection (NEW)          │
└───────────────────────────────────────┘
    ↓ (all v6.0 features complete)
    ↓
Demo 1: Spec-Driven Validation ✅
Demo 2: Checkpoint-Based Feature Dev ✅
    ↓
MARKET VALIDATION
    ↓
┌───────────────────────────────────────┐
│ v6.1-6.2: AI-NATIVE EXPANSION        │
│ (Advanced filtering, safe repair,     │
│  smart recommendations)               │
└───────────────────────────────────────┘
    ↓
┌───────────────────────────────────────┐
│ v6.3-6.4: ENTERPRISE FEATURES        │
│ (Zero-trust, production, compliance)  │
└───────────────────────────────────────┘
    ↓
┌───────────────────────────────────────────────────┐
│ v7.0: FULL-STACK AI DEBUGGING                     │
│                                                   │
│ EARS (Backend Visibility):                       │
│ ├─ Backend Log Streaming                         │
│ ├─ Custom Events                                 │
│ ├─ Test Capture                                  │
│ └─ Git Tracking                                  │
│                                                   │
│ EYES (Semantic Understanding):                   │
│ ├─ Request/Session Correlation                   │
│ ├─ Causality Analysis                            │
│ ├─ Normalized Schema                             │
│ └─ Historical Snapshots                          │
│                                                   │
│ HANDS (Autonomous Control):                      │
│ ├─ Backend Control                               │
│ ├─ Code Navigation & Modification                │
│ ├─ Environment Manipulation                      │
│ └─ Timeline & Search                             │
└───────────────────────────────────────────────────┘
    ↓
Demo 3: Production Error → Root Cause ✅
Demo 4: Service A Changed → Validate B, C, D ✅
    ↓
FULL-STACK AI DEBUGGING PROVEN
```

---

## What Gets Captured & Analyzed

### v6.0

**Captured:**
- Browser events (console, network, WebSocket, DOM, screenshots)
- Local backend logs (dev server, containers)
- Test execution (pass/fail, duration)
- User actions (clicks, fills, navigates)
- Application events (custom injected events)

**Analyzed:**
- Unified timeline (when did what happen?)
- Request tracing (action → API call)
- State changes (before/after comparison)
- Doom loops (repeated failures)
- Regression detection (checkpoint comparison)

### v7.0 (Added)

**Captured:**
- Multi-service backend logs
- Git commits & file changes
- Correlation IDs (link browser → backend)
- Historical snapshots
- External service calls

**Analyzed:**
- Full causality chains (action → service 1 → service 2 → database → response)
- Latency attribution ([Browser 20ms] → [Network 80ms] → [Server 150ms])
- Impact analysis (which services affected by change?)
- Contract validation (breaking changes?)
- Edge case coverage

---

## Success Criteria

### v6.0 Release Criteria

- [ ] Wave 1 features working (explore, observe, infer)
- [ ] Wave 2 features working (persistence, doom loop detection)
- [ ] Demo 1: Spec-Driven Validation completes in < 3 minutes, fully autonomous
- [ ] Demo 2: Feature Implementation completes in < 5 minutes, fully autonomous
- [ ] All critical paths from Spec Demo pass after fixes
- [ ] No infinite loops detected in execution history
- [ ] Doom loop prevention suggests correct alternatives
- [ ] Test generation produces valid, non-flaky tests

### v7.0 Release Criteria

- [ ] Phase 1 EARS: Backend logs from 3+ services flowing in
- [ ] Phase 2 EYES: Browser request correlated to backend log in < 100ms
- [ ] Phase 2 EYES: Causality chains showing full stack trace
- [ ] Phase 3 HANDS: Code modification triggers tests automatically
- [ ] Demo 3: Production error traced to root cause (code, timing, dependency)
- [ ] Demo 4: Service A change validated against B, C, D contracts
- [ ] Impact analysis prevents breaking changes before deploy
- [ ] Edge case registry catches project-specific risks

---

## Marketing Milestones

| Version | Message | Evidence |
|---------|---------|----------|
| **v6.0** | "AI autonomously validates and fixes web applications" | Specs Validated + Features Implemented + Tests Generated |
| **v6.1-6.2** | "AI-native development helpers" | Advanced filtering + Smart recommendations + Safe repair |
| **v6.3-6.4** | "Enterprise-safe with production debugging" | Zero-trust + Audit logs + GitHub/Jira integration |
| **v7.0** | "AI debugs the full stack, not just browser" | Backend correlation + Causality analysis + Multi-service validation |
| **v7.1** | "Fully autonomous debugging across services" | Code modification + Backend control + Cross-service fixes |

---

## Implementation Notes

### Architecture Principles

1. **Ring Buffer Storage** — Never lose events, configurable TTL (24h default)
2. **Streaming, Not Batch** — Real-time analysis as events arrive
3. **Local-Only Processing** — All correlation happens on localhost
4. **Zero Dependencies** — Keep Gasoline's single Go binary
5. **Privacy-First** — Automatic PII redaction, opt-in sensitive data capture

### Developer Experience

1. **Zero-Config Baseline** — Works without setup
2. **Graduated Complexity** — v6 simple, v7 requires configuration
3. **Clear Feedback** — Show captured data, show understanding, show recommendations
4. **Optional Features** — Backend logging opt-in, contract management opt-in

### LLM Integration

1. **Context Window Efficiency** — ~10KB of key events, not raw logs
2. **Semantic Compression** — "Element @button1 clicked" not "HTML: <div id='...'>"
3. **Structured Reasoning** — JSON timeline, not narrative logs
4. **Failure Explanation** — Why test failed, not just that it failed

---

## Cross-Cutting Concerns

### Performance & Scalability

- **Ingest rate:** 1000+ events/sec without latency impact
- **Query performance:** Complex filters in <500ms
- **Storage:** 24h of all data in <2GB
- **Memory overhead:** <20MB for extension, <50MB for server

### Privacy & Security

- **Data locality:** Never leaves localhost
- **Automatic redaction:** Auth tokens, API keys, PII
- **Audit trail:** What was captured, when, by whom
- **Compliance:** GDPR, SOC2 ready

### Integration Points

- **CI/CD:** GitHub Actions, GitLab CI, etc.
- **Issue Trackers:** GitHub, Jira, Linear
- **IDEs:** VS Code, Claude Code, Cursor, etc.
- **Chat Interfaces:** Claude, ChatGPT, Copilot, etc.

---

## Out of Scope (Deferred)

- Machine learning-based root cause inference (rule-based sufficient)
- Video replay or animated debugging (screenshots sufficient)
- Embedded database for offline analysis (export sufficient)
- Real-time collaboration (Flight Recorder sufficient)
- Automatic service dependency inference (manual in v7.0, auto in v7.1+)
- Production multi-tenant isolation (read-only mode only)
- Advanced APM features (flame graphs, detailed spans)

---

## Related Documents

- [roadmap.md](roadmap.md) — Release sequencing and timing
- [ai-native-testing-philosophy.md](ai-native-testing-philosophy.md) — Why AI-native is different
- [backend-frontend-unification.md](backend-frontend-unification.md) — v7 vision
- [ai-native-testing-discussion-record.md](ai-native-testing-discussion-record.md) — Discussion & decisions

---

**Status:** Comprehensive Feature Taxonomy v1
**Last Updated:** 2026-01-31
**Next:** Break into individual feature specs, create per-feature product/tech/QA documents
