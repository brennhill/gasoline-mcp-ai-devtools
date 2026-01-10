# v6.0: Compete Directly with TestSprite

**Date:** 2026-01-29
**Strategic Goal:** Transform Gasoline from "browser telemetry" to "complete validation loop" that directly competes with TestSprite's $29-99/month SaaS.

---

## Executive Summary

TestSprite proved the market (42% → 93% code quality improvement). Gasoline can deliver the same value with competitive advantages:

| Feature | TestSprite | Gasoline v6 |
|---------|-----------|-------------|
| **Cost** | $29-99/month | FREE |
| **Privacy** | Cloud-based | 100% localhost |
| **Error context** | Requests blind | Already captured |
| **Cross-session memory** | No | YES (regressions) |
| **WebSocket + network** | No | YES (unique) |
| **Dependencies** | Node.js + cloud | Single Go binary |

---

## Missing Features (Critical for v6.0)

These features are NOT in the current v6 spec but are required to compete with TestSprite:

### 1. Test Generation from Error Context ❌ NEW

**Current:** `generate {type: "test"}` creates reproduction scripts
**Needed:** Generate Playwright test that validates the fix

```
Error occurs → Gasoline has full context (console, network, DOM, actions)
AI calls: generate {type: "test", from_error: "err-abc123"}
Output: Playwright test that reproduces error + asserts it's fixed
```

**Estimated effort:** ~600 lines Go + ~200 lines extension JS

---

### 2. Failure Classification ❌ NEW

**Current:** Tests fail, no analysis of WHY
**Needed:** Classify failure type (real bug vs flaky test vs environment)

```
Test fails → Analyze context (retries, network, timing)
Classification: "real_bug" (95% confidence) vs "flaky_test" vs "selector_drift"
Evidence: "Error occurred 3/3 retries, same stack trace"
```

**Estimated effort:** ~500 lines Go

---

### 3. Auto-Repair Suggestions ❌ NEW

**Current:** AI manually analyzes errors
**Needed:** Pattern-match errors → suggest specific code fixes

```
Error: "Cannot read property 'user' of undefined at auth.js:42"
Context: Network 401, response.user undefined
Suggestion: "Add optional chaining: response?.user || null"
```

**Estimated effort:** ~400 lines Go

---

###

 4. Local Test Execution ❌ NEW

**Current:** Tests generated, AI must manually run them
**Needed:** Gasoline runs Playwright tests locally, returns results

```
AI calls: interact {action: "run_test", file: "login-error.spec.js"}
Gasoline spawns Playwright, captures output
Returns: {status: "failed", errors: ["Selector not found"], retry_count: 3}
```

**Estimated effort:** ~300 lines Go

---

### 5. Test Persistence Enhancement ⚠️ PARTIAL

**Current:** Reproduction scripts exist but no test suite management
**Needed:** Save tests to `.gasoline/tests/`, maintain manifest

```
.gasoline/
  tests/
    login-error.spec.js
    api-null-error.spec.js
  test-suite.json  ← manifest with metadata
```

**Estimated effort:** ~200 lines Go

---

### 6. Regression Detection Enhancement ⚠️ PARTIAL

**Current:** Session comparison exists but no test-specific regression
**Needed:** Track test results, flag "test X passed, now fails"

```
Run test suite → compare to previous run
Regression: "login-error test passed last run, now fails"
New error: "dashboard-error not covered by existing tests"
```

**Estimated effort:** ~300 lines Go

---

### 7. Self-Healing Tests ✅ ALREADY PLANNED

**Current:** Planned in Wave 1
**Keep as-is:** Detect broken selectors → find alternatives → update test

---

## Revised v6.0 Build Plan

### Wave 1 (6-8 weeks) — Core Validation Loop

```
┌──────────────────────────────────────────────────────────────┐
│              4 AGENTS IN PARALLEL                            │
├─────────────┬─────────────┬──────────────┬──────────────────┤
│  Agent A    │  Agent B    │  Agent C     │  Agent D         │
│             │             │              │                  │
│  Test Gen   │  Self-      │  Failure     │  Local Test      │
│  from Error │  Healing    │  Classify +  │  Execution       │
│             │  Tests      │  Auto-Repair │                  │
└─────────────┴─────────────┴──────────────┴──────────────────┘
```

**Exit criteria:** Test generated from error → runs locally → classified → fixed

### Wave 2 (4-6 weeks) — Persistence + Regression

```
┌──────────────────────────────────────────────────────────────┐
│              3 AGENTS IN PARALLEL                            │
├─────────────┬──────────────┬──────────────────────────────┤
│  Agent A    │  Agent B     │  Agent C                    │
│             │              │                             │
│  Test       │  Regression  │  CI Infra + Context Stream  │
│  Persistence│  Detection   │  (already specced)          │
└─────────────┴──────────────┴─────────────────────────────┘
```

**Exit criteria:** Tests persisted → regressions detected → v6.0 ready

---

## v6.0 Release Criteria

✅ v5.2 bugs fixed (done)
✅ Wave 1 complete (7 core features)
✅ Wave 2 complete (persistence, regressions, CI)
✅ All features battle-tested
✅ No regressions in v5.x
✅ Marketing: "TestSprite but free, local, better error context"

**Timeline:** 10-14 weeks total → Release v6.0

---

## Deferred to v6.1+

These were in the original v6 spec but NOT needed for TestSprite competition:

- PR Preview Exploration (Wave 2 → v6.1+)
- Agentic E2E Repair (Wave 2 → v6.1+)
- Deployment Watchdog (Wave 2 → v6.1+)
- SEO Audit (v6.1 → v6.1+)
- Performance Audit (v6.1 → v6.1+)
- Best Practices Audit (v6.1 → v6.1+)
- Dialog Handling (v6.2 → v6.2+)
- CPU/Network Emulation (v6.2 → v6.2+)

**Rationale:** Valuable, but don't differentiate from TestSprite. Ship after v6.0.

---

## Key Metrics

| Metric | Target |
|--------|--------|
| Test generation accuracy | 80%+ tests reproduce error |
| Self-healing success | 90%+ selectors auto-fixed |
| Failure classification | 85%+ correctly classified |
| Regression detection | 95%+ regressions caught |
| Localhost guarantee | 100% no cloud dependency |

---

## Risks

| Risk | Mitigation |
|------|------------|
| TestSprite adds localhost variant | Ship v6.0 ASAP to capture market first |
| TestSprite goes open-source | Differentiate on WebSocket, cross-session memory, zero deps |
| Microsoft integrates TestSprite | Position as "privacy-first alternative" |

---

## Marketing (Post-v6.0)

**Tagline:** "The local, open-source alternative to TestSprite"

**Value props:**
- "42% → 93% code quality improvement (same as TestSprite), but free"
- "No cloud. No pricing tiers. No vendor lock-in."
- "Already has your error context (TestSprite requests it blind)"

**Comparison:**

| Feature | TestSprite | Gasoline v6 |
|---------|-----------|-------------|
| Test generation | ✅ | ✅ From captured errors |
| Self-healing | ✅ | ✅ |
| Failure classification | ✅ | ✅ |
| Auto-repair | ✅ | ✅ |
| Regression detection | ❌ | ✅ Cross-session |
| Error capture | ⚠️ Requests it | ✅ Already has it |
| WebSocket monitoring | ❌ | ✅ |
| Cost | 💰 $29-99/mo | 💰 FREE |
| Privacy | ⚠️ Cloud | ✅ Localhost |

---

## Files Modified

See [docs/competitors.md](competitors.md) for full competitive analysis.
See [docs/roadmap.md](roadmap.md) for original v6 spec (to be updated).
