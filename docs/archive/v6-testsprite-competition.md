---
status: proposed
scope: version/v6.0
ai-priority: high
tags: [v6, competition, ai-native, testing]
relates-to: [ai-native-testing-philosophy.md, ai-native-testing-discussion-record.md, roadmap.md]
last-verified: 2026-01-31
---

# v6.0: AI-Native Testing & Validation

**Date:** 2026-01-31
**Strategic Goal:** Transform Gasoline into AI-native toolkit that enables LLMs to autonomously explore, understand, and fix web applications through observation, exploration, and intelligent iteration.

**Philosophy:** Don't make LLMs write better tests. Make LLMs better at understanding and fixing web applications.

---

## Executive Summary

TestSprite proved the market (42% → 93% code quality improvement) but uses traditional QA workflows. Gasoline can deliver **superior value** with AI-native approach:

| Feature | TestSprite | Gasoline v6 |
|---------|-----------|-------------|
| **Approach** | Traditional QA (test-first) | AI-Native (explore-first) |
| **Cost** | $29-99/month | FREE |
| **Privacy** | Cloud-based | 100% localhost |
| **Error context** | Requests blind | Already captured |
| **Understanding** | Test suite coverage | Semantic understanding (contracts, dependencies) |
| **Doom loop prevention** | ❌ None | ✅ Pattern detection |
| **Iteration** | Manual | Intelligent AI iteration |
| **WebSocket + network** | No | YES (unique) |
| **Dependencies** | Node.js + cloud | Single Go binary |

---

## The AI-Native Advantage

### Why Traditional QA Fails for AI

Traditional QA assumes:
1. **Test-first workflow** - Write tests before code (slow for AI)
2. **Formal test suites** - Must maintain forever (AI doesn't need this)
3. **Regression baselines** - Compare to previous runs (AI can reason about impact)
4. **Approval gates** - Every change reviewed (AI can self-correct)
5. **Coverage metrics** - Measure % code tested (AI knows what matters)

### Why AI-Native Succeeds

AI naturally:
1. **Explores** - Try things, see what happens
2. **Observes** - Watch console, network, DOM
3. **Infers** - "What's different here?"
4. **Iterates** - Try, fail, adjust, try again
5. **Validates** - "Does this solve the user's problem?"

**Gasoline provides:** Eyes, ears, and hands for AI to do this autonomously.

---

## v6.0: Core Capabilities (2-3 weeks)

### Wave 1: AI-Native Toolkit

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

### Capability 1: Explore (interact)

**Purpose:** Let AI try things and see what happens

**Modes:**
- `interact.explore` - Execute actions, capture full state (console, network, DOM, screenshots)
- `interact.record` - Capture user interactions for later replay
- `interact.replay` - Reproduce recordings in different environments (dev vs prod)

**Example:**
```javascript
interact({
  type: "explore",
  actions: [
    {action: "goto", url: "https://example.com/signup"},
    {action: "fill", selector: "#email", value: "invalid"},
    {action: "click", selector: "button[type=submit]"}
  ],
  capture: {console: true, network: true, dom: true}
})
```

**Response:**
```json
{
  "result": "completed",
  "actions_executed": 3,
  "observations": {
    "console": [{"level": "error", "message": "Invalid email format"}],
    "network": [{"url": "/api/signup", "status": 400}],
    "dom": {"form_valid": false, "errors_shown": false},
    "screenshot": "base64..."
  }
}
```

### Capability 2: Observe (observe)

**Purpose:** Watch what's happening in the browser

**Modes:**
- `observe.capture` - Capture comprehensive state (console, network, DOM)
- `observe.compare` - Compare two states (before/after, prod/dev)

**Example:**
```javascript
// Compare production vs dev
let prodState = observe({type: "capture", source: "production"});
let devState = observe({type: "capture", source: "development"});

let diff = observe({
  type: "compare",
  before: prodState,
  after: devState
});
```

**Response:**
```json
{
  "summary": "Production: API timeout 500ms. Dev: No timeout.",
  "differences": {
    "network": [
      {
        "url": "/api/payment",
        "prod": {"timeout": 500},
        "dev": {"timeout": null}
      }
    ],
    "console": ["New error in production: Payment gateway timeout"]
  },
  "suggestions": ["API timeout changed between prod and dev"]
}
```

### Capability 3: Compare & Infer (analyze)

**Purpose:** Help AI understand what's different and why

**Modes:**
- `analyze.infer` - "What's different here?" - Natural language analysis
- `analyze.detect_loop` - Detect doom loops from execution history

**Example (loop detection):**
```javascript
analyze({
  type: "detect_loop",
  recent_attempts: [
    {timestamp: "10:00", test: "login.spec.ts", result: "failed", fix: "Updated selector", confidence: 0.9},
    {timestamp: "10:05", test: "login.spec.ts", result: "failed", fix: "Updated selector", confidence: 0.85},
    {timestamp: "10:10", test: "login.spec.ts", result: "failed", fix: "Updated selector", confidence: 0.88}
  ]
})
```

**Response:**
```json
{
  "in_loop": true,
  "confidence": 0.95,
  "analysis": "You've tried selector updates 3 times, all failed. This is likely not a selector issue. The element might not exist, or test logic is wrong.",
  "suggestion": "Verify element actually exists in DOM first, or try different approach."
}
```

### Capability 4: Doom Loop Prevention

**Purpose:** Track what AI has tried, prevent infinite loops

**File format:**
```json
// .gasoline/execution-history.json
{
  "records": [
    {
      "timestamp": "2026-01-31T10:00:00Z",
      "change_id": "abc123",
      "test_file": "login.spec.ts",
      "result": "failed",
      "error": "Selector not found",
      "fix_attempt": "Updated selector to .login-btn",
      "confidence": 0.9
    }
    // ... more records
  ]
}
```

**How it works:**
1. AI makes a change, runs test
2. Gasoline records result in execution history
3. Next time AI tries similar change, Gasoline detects pattern
4. Gasoline warns: "You've tried this 3 times, it's not working"
5. AI switches approach

---

## Wave 2: Basic Persistence (2-3 weeks)

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

### Layer 1: Execution History

**Purpose:** Track test executions, store results

**Implementation:**
- Simple JSON file
- Track: timestamp, change ID, test file, result, error, fix attempt, confidence
- Keep recent N records (configurable)

### Layer 2: Doom Loop Prevention

**Purpose:** Detect repeated failures, suggest alternatives

**Implementation:**
- `analyze.detect_loop` - Pattern matching on recent attempts
- Natural language suggestions
- Prevent AI from trying same fix forever

---

## Demo Scenarios

### Demo 1: Spec-Driven Validation

**Setup:**
- User provides product spec (markdown)
- User opens their website
- User: "Validate that the signup flow works as specified"

**LLM Flow:**
```
1. LLM reads spec:
   "New user must enter valid email, password (8+ chars), and confirm password.
    Form should show inline errors for invalid input."

2. LLM uses Gasoline to explore:
   - interact.explore: Goto /signup
   - interact.explore: Try email = "invalid" → Observe: No inline error
   - interact.explore: Try password = "short" → Observe: Accepts 6 chars
   - interact.explore: Try valid input → Observe: Success

3. LLM captures results:
   ✅ Email validation works
   ❌ Password accepts 6 chars (spec says 8+)
   ❌ No inline error shown, just console error
   ✅ Successful signup works

4. LLM reports:
   "Found 2 issues:
   1. Password accepts 6 chars, spec requires 8+
   2. Errors go to console, should be inline in UI
   
   Shall I fix these?"
   
5. User: "Yes"
   
6. LLM fixes code, re-tests, confirms both issues resolved.

Total time: 3 minutes, fully autonomous
```

**Why impressive:**
- LLM reads natural language spec
- LLM autonomously explores UI
- LLM validates behavior against spec
- LLM fixes bugs it finds
- User just watches and approves

### Demo 2: Production Error Reproduction

**Setup:**
- User encounters error in production
- User enables Gasoline recorder
- User reproduces error
- User: "Fix this error - here's what happened"

**LLM Flow:**
```
1. LLM analyzes recording:
   - Page: /checkout
   - Actions: Fill form, click "Pay"
   - Error: "Payment gateway timeout (500ms)"
   - Network: POST /api/payment timed out

2. LLM attempts to reproduce in dev:
   - interact.explore: Navigate to /checkout
   - interact.explore: Fill same form
   - interact.explore: Click "Pay"
   - Result: Payment succeeds instantly

3. LLM analyzes differences:
   - observe.compare: Production API has 500ms timeout, dev has no timeout
   - analyze.infer: Likely cause: Production API is overloaded or has different config

4. LLM suggests:
   "I can't reproduce in dev. Differences:
   - Dev API has no timeout, production has 500ms
   - Options:
     A) Set dev timeout to 500ms to test fix
     B) Mock timeout response
     C) Add more logging to production
   
   Which approach?"

5. User: "Try A - set dev timeout"

6. LLM modifies dev config, retries:
   - Sets API timeout to 500ms
   - interact.replay: Re-runs checkout
   - Result: Reproduces error!

7. LLM investigates:
   "The API is slow because it's validating credit card via external
    service. Timeout is too aggressive. Options:
   1. Increase timeout to 2000ms
   2. Make validation async (fire & forget)
   3. Cache validation results

   Which approach?"

8. User: "Make it async"

9. LLM refactors code, tests:
   - Payment no longer blocks on validation
   - Error still works but is non-blocking
   - UX improved

Total time: 5 minutes
```

**Why impressive:**
- LLM understands production vs dev difference
- LLM suggests concrete options
- LLM can modify environment to reproduce
- LLM fixes root cause, not just symptom

---

## What's Different from TestSprite

### TestSprite: Traditional QA

- "Write tests first, then fix bugs"
- Test suite management
- Regression baselines
- Coverage metrics
- Classification of failures
- Auto-repair of tests

### Gasoline v6: AI-Native

- "Explore, observe, understand, then fix what's broken"
- No test suites (ephemeral, on-demand)
- No baselines (AI reasons about impact)
- No coverage (AI knows what matters)
- Doom loop detection (prevents AI from getting stuck)
- Production vs dev comparison
- Semantic understanding (future)

**Result:** Gasoline is faster, more flexible, and works the way AI naturally works.

---

## v7: Semantic Understanding (Future)

**Goal:** Full semantic understanding for complex systems (microservices)

**Deferred to v7:**
- Dependency graphs (hybrid inference: LLM + human review)
- API contracts (simplified JSON + OpenAPI export)
- Critical paths (hybrid: LLM suggests, user prioritizes)
- Edge case registry (project-specific)

**Why deferred:**
- MVP focuses on single-app scenarios
- Validates AI-native philosophy
- Faster time to market (4-6 weeks vs 10-14 weeks)
- Dependency graphs add complexity, can be added later

---

## v6.0 Release Criteria

✅ v5.2 bugs fixed (done)
✅ v5.3 critical blockers removed (done)
✅ Wave 1 complete (4 capabilities)
✅ Wave 2 complete (2 persistence layers)
✅ Both demo scenarios working
✅ No regressions in v5.x
✅ Marketing: "AI autonomously validates and fixes web applications"

**Timeline:** 4-6 weeks total → Release v6.0

---

## Key Metrics

| Metric | Target |
|--------|--------|
| Demo 1 completion time | < 3 minutes |
| Demo 2 completion time | < 5 minutes |
| Doom loop detection accuracy | 90%+ loops caught |
| Execution history storage | Recent N records (configurable) |
| Localhost guarantee | 100% no cloud dependency |

---

## Risks

| Risk | Mitigation |
|------|------------|
| TestSprite adds localhost variant | Ship v6.0 ASAP to capture market first |
| TestSprite goes open-source | Differentiate on AI-native approach, WebSocket, doom loop prevention |
| Market prefers traditional QA | Educate on AI-native benefits (faster, more flexible) |

---

## Marketing (Post-v6.0)

**Tagline:** "The AI-native alternative to TestSprite"

**Value props:**
- "AI autonomously validates and fixes, not just suggests"
- "Explore → Observe → Infer → Act → Validate loop"
- "No test suites. No baselines. No approval workflows."
- "10× less context than Chrome DevTools MCP"

**Comparison:**

| Feature | TestSprite | Gasoline v6 |
|---------|-----------|-------------|
| Approach | Traditional QA (test-first) | AI-Native (explore-first) |
| Test generation | ✅ | ✅ From captured context |
| Self-healing | ✅ | ✅ |
| Doom loop prevention | ❌ | ✅ Pattern detection |
| Production vs dev comparison | ❌ | ✅ observe.compare |
| Error capture | ⚠️ Requests it | ✅ Already has it |
| WebSocket monitoring | ❌ | ✅ |
| Semantic understanding | ❌ | ✅ (v7) |
| Cost | 💰 $29-99/mo | 💰 FREE |
| Privacy | ⚠️ Cloud | ✅ Localhost |

---

## Files Modified

- **AI-Native Testing Philosophy** - `docs/core/ai-native-testing-philosophy.md`
- **Discussion Record** - `docs/core/ai-native-testing-discussion-record.md`
- **Roadmap** - `docs/roadmap.md` (to be updated)