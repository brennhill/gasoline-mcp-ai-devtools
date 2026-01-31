---
status: proposed
scope: v6-v7-implementation
ai-priority: critical
tags: [v6, v7, sequencing, sprints, dependencies, critical-path]
relates-to: [360-observability-roadmap.md, 360-observability-architecture.md, roadmap.md]
last-verified: 2026-01-31
---

# Implementation Sequencing: From Architecture to Sprints

**How to build Gasoline 360° Observability in parallel tracks with clear dependencies.**

---

## Philosophy

**Goal:** Ship v6.0 MVP in 4-6 weeks with maximum parallelization after v5.3.

**Constraint:** Some components block others (example: Ingestion must exist before Query Service).

**Strategy:** Break into 3 independent tracks, each with serial dependencies.

---

## Critical Path (Must Serialize)

```
v5.3 (✅ Done)
    ↓
FOUNDATION LAYER (Weeks 1-2)
├─ Ingestion (browser + backend)
├─ Ring buffers
└─ Normalized schema
    ↓
PROCESSING LAYER (Weeks 2-3)
├─ Normalization
└─ Initial query service
    ↓
DEMO-READY (Week 3-4)
├─ Checkpoint system
├─ Regression detection
└─ Basic LLM integration
    ↓
v6.0 RELEASE (Week 4-5)
    ↓
v6.1-6.2 (Parallel tracks)
    ↓
v7.0 FOUNDATION (Weeks 6-8)
├─ Correlation engine
├─ Backend log ingestion (enhanced)
└─ Git tracking
    ↓
v7.0 PROCESSING (Weeks 8-10)
├─ Causality analysis
└─ Enrichment
```

---

## Track-Based Breakdown

### Track A: Core Ingestion & Storage (Weeks 1-2)

**Goal:** Get data in and queryable.

#### Sprint A1: Browser Extension + Buffer Layer (Week 1)

**Team:** 1 engineer (enhancement of existing v5.3 code)

**Dependencies:** None (v5.3 done)

**Deliverables:**

- [ ] Enhanced browser event capture
  - v6: Performance timing per action
  - v6: DOM state snapshots
  - v6: Accessibility event capture
  - ✅ Keep: Console, network, WebSocket (from v5.3)

- [ ] Ring buffer implementation (in-memory storage)
  - Capacity: 10K events
  - TTL: 24h
  - Circular: O(1) push/pop

- [ ] Normalized event schema
  ```typescript
  interface NormalizedEvent {
    id: string;
    timestamp: number;
    source: "browser" | "backend" | "test" | "git";
    level: "debug" | "info" | "warn" | "error" | "critical";
    correlation_id?: string;
    message: string;
    metadata: Record<string, any>;
    tags: string[];
  }
  ```

**Success Criteria:**
- [ ] Browser events normalized to schema
- [ ] 10K events in memory without bloat
- [ ] TTL cleanup working
- [ ] Zero events lost on normal operation

---

#### Sprint A2: Backend Log Streaming (Week 1-2)

**Team:** 1 engineer (new code)

**Dependencies:** Ring buffer (A1)

**Deliverables:**

- [ ] Local backend log streaming
  - Dev server stdout (Node.js, Python, Go)
  - Docker container logs
  - File tail (-f)
  - JSON + plaintext parsing

- [ ] Test execution capture
  - Intercept: `npm test`, `pytest`, `go test`
  - Extract: test name, pass/fail, duration

- [ ] Second ring buffer for backend
  - Capacity: 50K events (higher than browser)
  - TTL: 24h

**Success Criteria:**
- [ ] Backend logs flowing to buffer in real-time
- [ ] Test output captured
- [ ] <1ms ingestion latency per event
- [ ] No blocking on writer thread

---

### Track B: Query Service & LLM Integration (Weeks 2-3)

**Goal:** Make data queryable and usable by LLM.

#### Sprint B1: Query Service Implementation (Week 2)

**Team:** 1 engineer (core query logic)

**Dependencies:** Ring buffers + normalized schema (A1, A2)

**Deliverables:**

- [ ] Timeline query
  - Returns all events sorted by timestamp
  - Supports filtering by source, level, tags
  - Cursor-based pagination

- [ ] Regression detection baseline
  - Save checkpoint (snapshot of ring buffers)
  - Load checkpoint (restore for comparison)
  - Compare events (detect changes)

- [ ] Simple indexing
  - Hash map by correlation_id (v6)
  - Hash map by timestamp ranges

**Success Criteria:**
- [ ] Query 10K events in <500ms
- [ ] Checkpoint save/load works
- [ ] Pagination cursor works
- [ ] Can compare baseline to current

---

#### Sprint B2: MCP Server Integration (Week 2-3)

**Team:** 1 engineer (API design)

**Dependencies:** Query service (B1)

**Deliverables:**

- [ ] MCP tool implementation: `observe`
  - Types: "timeline", "logs", "network_waterfall"
  - Filters: level, source, tags
  - Pagination: limit, offset, after_cursor

- [ ] MCP tool implementation: `configure`
  - Action: "save_checkpoint"
  - Action: "clear"

- [ ] MCP tool implementation: `analyze`
  - Type: "regression" (baseline comparison)

- [ ] Context compression
  - Send ~10KB of key events to LLM
  - Not raw logs

**Success Criteria:**
- [ ] Claude Code can call observe() and get results
- [ ] Claude Code can call configure() to save checkpoints
- [ ] Claude Code can call analyze() to detect regressions
- [ ] Context window efficient (<25KB per query)

---

### Track C: Demos & v6.0 Validation (Week 3-4)

**Goal:** Prove the AI-native thesis with two demos.

#### Sprint C1: Demo 1 - Spec-Driven Validation (Week 3)

**Team:** 1 engineer (demo setup) + 1 AI iteration

**Dependencies:** MCP integration (B2)

**Deliverables:**

- [ ] ShopBroken demo setup
  - Simple product catalog
  - Simple checkout form
  - Clear spec: "Form must require 8+ char password"

- [ ] AI workflow
  - Read spec → Explore UI → Validate behavior → Fix bugs

- [ ] Success measurement
  - <3 minutes end-to-end
  - AI finds bug autonomously
  - AI fixes bug
  - AI validates fix

**Success Criteria:**
- [ ] Demo completes in <3 minutes
- [ ] Bug found and fixed without human intervention
- [ ] Repeatable (works multiple times)
- [ ] Can be demoed to stakeholders

---

#### Sprint C2: Demo 2 - Checkpoint Validation (Week 3-4)

**Team:** 1 engineer (demo setup) + 1 AI iteration

**Dependencies:** Checkpoint system (B1), Regression detection (B2)

**Deliverables:**

- [ ] Happy path recording
  - Record: Login → Add to cart → Checkout → Payment
  - Save as checkpoint

- [ ] Feature implementation (non-breaking + breaking)
  - Feature 1: Add product reviews (non-breaking)
  - Feature 2: Convert 3-step → 2-step checkout (breaking)

- [ ] Checkpoint replay and comparison
  - Replay recorded paths after changes
  - Detect expected vs unexpected changes
  - Update checkpoints for expected changes

- [ ] Success measurement
  - AI implements both features
  - AI updates checkpoints appropriately
  - <5 minutes end-to-end
  - All critical paths validated

**Success Criteria:**
- [ ] Demo completes in <5 minutes
- [ ] Features implemented autonomously
- [ ] Checkpoints updated correctly
- [ ] No false positives or negatives
- [ ] Repeatable

---

### Track D: v6.1+ Parallel Expansion (Weeks 4-6)

**After v6.0 ships, these run in parallel:**

#### Advanced Filtering (Signal-to-Noise)

- Content-type filters
- Domain allowlist/blocklist
- Regex pattern filtering
- Response size thresholds

#### Visual-Semantic Bridge

- Smart selector generation
- Accessible element detection
- Computed layout maps
- Auto-test-ID injection

#### Smart Test Recommendations

- Analyze code change
- Suggest tests to run
- Prioritize by risk

#### Self-Healing Tests

- Detect broken selectors
- Auto-generate semantic selectors
- Update tests automatically

---

## Parallel Tracks Summary

```
Week 1-2: FOUNDATION (Serial)
├─ Track A1: Browser + Buffers
├─ Track A2: Backend logs
└─ (Both must complete before B)

Week 2-3: PROCESSING (Serial after Foundation)
├─ Track B1: Query service
└─ Track B2: MCP integration
    (Both must complete before C)

Week 3-4: DEMOS & v6.0 (Parallel after Processing)
├─ Track C1: Spec-Driven Validation demo
└─ Track C2: Checkpoint Validation demo
    (Can run in parallel)

Week 4-6: EXPANSION (Parallel)
├─ Advanced filtering
├─ Visual-semantic bridge
├─ Smart recommendations
└─ Self-healing tests
    (All independent, 4 engineers can work simultaneously)
```

---

## Team Allocation (Recommended)

### v6.0 Phase (4-6 weeks)

| Week | Track A | Track B | Track C | Notes |
|------|---------|---------|---------|-------|
| 1 | A1: Browser | Blocked | Blocked | Foundation |
| 2 | A2: Backend | B1: Query | Blocked | Ingestion complete |
| 3 | Polish | B2: MCP | C1: Demo 1 | Processing complete |
| 4 | — | — | C2: Demo 2 | v6.0 ready to release |

**Team size:** 3 engineers minimum
- 1 engineer: Ingestion (A1 + A2)
- 1 engineer: Query service (B1)
- 1 engineer: MCP + demos (B2 + C1 + C2)

**Timeline:** 4 weeks critical path (can overlap weeks 2-3)

### v6.1-6.2 Phase (4 weeks, parallel)

| Feature | Engineer | Effort | Start |
|---------|----------|--------|-------|
| Advanced filtering | A | 2 weeks | Week 4 |
| Visual-semantic | B | 2 weeks | Week 4 |
| Smart tests | C | 2 weeks | Week 4 |
| Self-healing | D | 2 weeks | Week 5 |

**Team size:** 2-4 engineers (flexible)

### v7.0 Phase (6-8 weeks, serial foundation)

**Week 6-8:** Correlation + Backend logs + Git tracking
**Week 8-10:** Causality + Enrichment

---

## Sprint Breakdown (Example: 2-week sprints)

### Sprint 1 (v6 Week 1)
- **Goal:** Foundation ingestion working
- **Team:** 2 engineers (A1 + A2 start)
- **Deliverables:**
  - Browser events normalized
  - Ring buffer working
  - Backend log streaming started
  - Test output capture started

### Sprint 2 (v6 Week 2)
- **Goal:** Ingestion complete, Query service started
- **Team:** 3 engineers (A1 done, A2 finishes, B1 starts)
- **Deliverables:**
  - Backend logs + test capture complete
  - Query service timeline working
  - Checkpoint save/load working
  - MCP tool interface started

### Sprint 3 (v6 Week 3)
- **Goal:** Query + MCP complete, Demos running
- **Team:** 3 engineers (A done, B + C start)
- **Deliverables:**
  - MCP tools working
  - Demo 1 (Spec validation) drafted
  - Demo 2 (Checkpoint) drafted
  - Ready for AI iteration

### Sprint 4 (v6 Week 4)
- **Goal:** Demos polished, v6.0 release ready
- **Team:** 2-3 engineers (C focus, B polish)
- **Deliverables:**
  - Both demos working end-to-end
  - Performance targets met
  - v6.0 documentation complete
  - Ready to announce

---

## Dependency Graph

```
A1: Browser Ingestion
 ├─→ A2: Backend Logs
 ├─→ B1: Query Service
 │    ├─→ B2: MCP Integration
 │    │    ├─→ C1: Spec-Driven Demo
 │    │    └─→ C2: Checkpoint Demo
 │    │         ├─→ v6.0 Release
 │    │         └─→ v6.1 Expansion
 │    │              ├─→ Advanced Filtering
 │    │              ├─→ Visual-Semantic
 │    │              ├─→ Smart Tests
 │    │              └─→ Self-Healing
 │    │
 │    └─→ v7.0 Foundation
 │         ├─→ Correlation Engine
 │         ├─→ Git Tracking
 │         └─→ Enhanced Backend Logs
 │              ├─→ v7.0 Processing
 │              │    ├─→ Causality Analysis
 │              │    └─→ Enrichment
 │              └─→ v7.0 Release
```

---

## Risk Mitigation

### Risk 1: Ingestion Bottleneck

**Risk:** Backend logs arrive faster than processing can handle

**Mitigation:**
- Ring buffers with async writer
- Bounded memory caps (50K events max)
- Graceful overflow handling (oldest events evicted)

### Risk 2: Context Window Bloat

**Risk:** LLM can't use 10K events efficiently

**Mitigation:**
- Semantic compression (references instead of raw data)
- Filtering by relevance
- Summarization for old events
- Track: "Use only last 100 events" in testing

### Risk 3: Demo Flakiness

**Risk:** Demos work once, fail under conditions

**Mitigation:**
- Run demos 10x in a row
- Test with different network conditions
- Test with different browser states
- Record demo failures for analysis

### Risk 4: v7 Complexity

**Risk:** Correlation + causality too complex

**Mitigation:**
- Start with simple correlation (by timestamp)
- Add trace ID matching incrementally
- Causality analysis is LLM responsibility (not system)
- Defer to v7.1 if needed

---

## Definition of Done

### Sprint Definition

A sprint is done when:
- [ ] Code compiles/runs
- [ ] Tests pass
- [ ] No regressions in v5.3
- [ ] Feature works end-to-end
- [ ] Documentation updated
- [ ] Ready to ship to main

### v6.0 Definition

v6.0 is done when:
- [ ] All critical path components shipped
- [ ] Both demos work repeatedly
- [ ] Performance targets met
- [ ] No known blockers for v6.1
- [ ] Marketing materials ready
- [ ] Release notes written
- [ ] Tagged and ready for release

### v7.0 Definition

v7.0 is done when:
- [ ] Correlation working end-to-end
- [ ] Causality chains visible
- [ ] Full-stack debugging demo works
- [ ] Performance targets met
- [ ] All 12 features shipped
- [ ] Ready for enterprise sales

---

## Success Metrics

### v6.0 Success

- Demo 1: Spec validation completes in <3 minutes
- Demo 2: Feature + checkpoint completes in <5 minutes
- Memory usage: <200MB sustained
- Ingestion latency: <1ms per event
- Query latency: <500ms for timeline queries
- Context window: <25KB per observation

### v6.1-6.2 Success

- Advanced filtering reduces noise by 70%+
- Smart test recommendations 80%+ accurate
- Self-healing tests reduce flakiness by 90%+
- Visual-semantic bridge improves selector stability by 95%+

### v7.0 Success

- Correlation latency: <100ms
- Causality chains accurate 95%+ of the time
- Full-stack demo completes in <5 minutes
- Enterprise feature adoption 50%+ within 6 months

---

## Rollback & Checkpoint Strategy

### Checkpoints (Save Before Major Work)

- **After v5.3:** Baseline (reference for regressions)
- **After A1+A2:** Ingestion checkpoint (before query work)
- **After B1+B2:** Query checkpoint (before demo work)
- **After C1+C2:** v6.0 checkpoint (release candidate)

### Rollback Plan

If a sprint fails:

1. Revert to last checkpoint
2. Analyze failure root cause
3. Adjust plan or timeline
4. Retry next sprint

Example:
- Demo 1 doesn't complete in <3min → Debug ingestion latency
- Query latency > 500ms → Optimize ring buffer queries
- Context > 25KB → Implement semantic compression

---

## Next Steps

1. **Get stakeholder alignment**
   - Confirm v6.0 4-week timeline is acceptable
   - Confirm 3-engineer team allocation
   - Confirm demo scope is appropriate

2. **Create detailed feature specs**
   - Per-component product + tech + QA specs
   - Start with A1 (Browser Ingestion)
   - Then A2 (Backend Logs)
   - Then B1 (Query Service)

3. **Spin up Sprint 1**
   - Assign engineers to A1 and A2
   - Set 2-week sprint goal
   - Daily standups to track progress

4. **Prepare demo environments**
   - Clone ShopBroken as demo base
   - Set up simple product catalog + checkout
   - Write specs for both demos

---

**Document Status:** Implementation Sequencing v1
**Last Updated:** 2026-01-31
**Ready for:** Team kickoff and sprint planning

