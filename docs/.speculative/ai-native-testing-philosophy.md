---
status: proposed
scope: core/philosophy
ai-priority: high
tags: [testing, ai-native, philosophy, microservices]
relates-to: [v6-testsprite-competition.md, test-generation/product-spec.md]
last-verified: 2026-01-31
---

# Why Traditional QA is Wrong for AI-Native Development

**Date:** 2026-01-31
**Scope:** Core philosophy for v6 AI testing features
**Status:** proposed

---

## Executive Summary

Traditional QA workflows were designed for a world where humans write code. AI-native development requires a fundamentally different approach: **semantic understanding over comprehensive testing**, **critical paths over test suites**, **impact analysis over regression detection**.

Gasoline MCP doesn't make LLMs write better tests. It makes LLMs better at understanding and fixing web applications through observation, exploration, and intelligent iteration.

---

## The Core Assumption Shift

### Traditional QA Assumes Human Developers

**Assumptions:**
- Humans write code (slow, error-prone)
- Humans write tests (slow, error-prone)
- Humans review results (slow)
- Humans maintain test suites (expensive)
- Developers need guardrails to prevent mistakes

**Reality with AI:**
- LLMs write code (fast, can understand context)
- LLMs can reason about correctness from spec/code
- Tests are communication, not discovery
- LLMs can discover bugs through exploration

---

## What Traditional QA Gets Wrong

### 1. Test-First is Counterproductive

**Traditional approach:**
```
1. Write test (human, slow)
2. Run test → fails (expected)
3. Write code (human, slow)
4. Run test → passes
5. Repeat
```

**Why this fails with AI:**
- LLMs don't need a test to know what code should do
- LLMs can reason about correctness from spec/code
- Tests are a form of communication, not discovery
- LLMs can discover bugs through exploration

**AI-native approach:**
```
1. LLM reads spec/code
2. LLM explores behavior (interact with UI)
3. LLM observes results
4. LLM infers what's wrong
5. LLM fixes it
6. LLM validates the fix
```

### 2. Formal Test Suites Are Rigid

**Traditional approach:**
- 100+ test cases covering every edge case
- Must pass before deployment
- Maintain regression suite forever
- Flaky tests are a blocker

**Why this fails with AI:**
- LLMs can test on-demand, not pre-commit
- LLMs can generate tests for specific scenarios
- LLMs don't need "comprehensive" coverage - they can test what matters
- Flaky tests are information, not blockers

**AI-native approach:**
- No formal test suite
- Tests are generated for specific changes
- Tests are ephemeral - created, used, discarded
- LLMs handle flakiness through retry/timeout heuristics

### 3. Regression Baselines Are Overkill

**Traditional approach:**
```
1. Run full test suite
2. Capture results as "baseline"
3. On every change, re-run and compare
4. Any failure = regression
```

**Why this fails with AI:**
- LLMs can reason about impact of changes
- LLMs don't need to test everything - they can test what changed
- Baselines assume "same = good," but sometimes behavior should change
- LLMs can evaluate "is this change correct?" rather than "is this change the same?"

**AI-native approach:**
- No baseline
- LLMs test changed code paths
- LLMs verify behavior matches spec/intent
- If behavior intentionally changed, LLM accepts it

### 4. Approval Workflows Assume Incompetence

**Traditional approach:**
- Every test change must be reviewed
- Every failure must be triaged
- Complex approval hierarchies
- Gates prevent "bad code" from merging

**Why this fails with AI:**
- LLMs are not incompetent - they're capable but sometimes wrong
- Review workflows are slow and expensive
- Gates assume reviewer knows more than author
- AI can self-correct through iteration

**AI-native approach:**
- Simple feedback loops, not gates
- LLMs get natural language feedback
- Iteration is cheap - try, fail, adjust
- Humans set constraints, not approve every change

### 5. Coverage Metrics Are Vanity Metrics

**Traditional approach:**
- "80% code coverage is the goal"
- Test every function, every line
- More coverage = better quality (false assumption)

**Why this fails with AI:**
- LLMs can reason about what needs testing
- 100% coverage of unimportant code is waste
- 10% coverage of critical paths is valuable
- LLMs can identify which paths matter

**AI-native approach:**
- No coverage target
- LLMs test critical paths
- LLMs test edge cases they discover
- LLMs adapt testing strategy to codebase

---

## What AI-Native Testing Gets Right

### 1. Exploration Over Specification

**LLMs naturally explore:**
- Try to happy path
- Try edge cases
- Try breaking things
- Observe what happens

**Gasoline MCP supports this:**
- `interact.explore` - Execute actions and capture results
- `observe.capture` - Watch console, network, DOM
- No need to pre-define test cases

### 2. Observation Over Assertion

**Traditional:** "This element must be visible"
**AI-native:** "Is this what the spec said should happen?"

**LLMs reason:**
- Compare actual behavior to expected behavior
- Notice subtle differences (timing, layout, user experience)
- Evaluate "is this acceptable?" not just "does this pass?"

**Gasoline MCP supports this:**
- `observe.compare` - Compare two states
- `analyze.infer` - "What's different here?"

### 3. Context Over Isolation

**Traditional:** Tests run in isolation, no external state
**AI-native:** Context matters - user sessions, network conditions, data

**LLMs understand:**
- Production behavior differs from dev
- State affects behavior (logged in vs logged out)
- External services matter

**Gasoline MCP supports this:**
- Captures full context (network, console, DOM)
- Can replay recordings in different environments
- `observe.compare` shows production vs dev differences

### 4. Iteration Over Perfection

**Traditional:** Get it right the first time (tests pass)
**AI-native:** Try, observe, adjust, try again

**LLMs iterate:**
- Fix fails → adjust approach
- Can't reproduce → try different environment
- Solution causes new issue → iterate

**Gasoline MCP supports this:**
- Fast iteration (no compile for JS/web)
- Immediate feedback
- `interact.replay` to try again

### 5. Intent Over Form

**Traditional:** "Did this test pass?"
**AI-native:** "Does this behavior match the intent?"

**LLMs evaluate:**
- Does the code solve the user's problem?
- Is the UX correct?
- Are edge cases handled appropriately?

**Gasoline MCP supports this:**
- Natural language constraints
- Simple prompts, not complex rules
- LLMs self-evaluate against constraints

---

## The Gasoline MCP Advantage

Gasoline MCP doesn't force LLMs into traditional QA. Instead, it provides:

1. **Observation capabilities** - See what's happening in the browser
2. **Action capabilities** - Try things, explore, experiment
3. **Comparison capabilities** - Notice differences between states
4. **Simple guardrails** - "Don't make tests less specific"

**No test suites. No baselines. No approval workflows.**

Just: Explore → Observe → Infer → Act → Validate

---

## The Microservice Problem: Why Ephemeral Tests Aren't Enough

**Challenge:** In complex systems (microservices, interlocking dependencies), a non-breaking change in Service A might break Service B. LLMs have limited context - how do we handle this?

**Traditional solution:** Integration test suite (expensive, slow)
**AI-native solution:** Semantic persistence - understand the system, not test every path

---

## AI-Native Persistence: System Understanding, Not Test Suite

**Instead of:** "These 200 tests must pass"
**Think:** "These critical paths must work"

### 1. Dependency Graph (Semantic Understanding)

```go
type ServiceDependency struct {
    Service       string   `json:"service"`
    DependsOn     []string `json:"depends_on"`
    Contracts     []string `json:"contracts"`  // API contracts this service relies on
    CriticalPaths []string `json:"critical_paths"`  // User journeys through this service
}

// analyze {type: "map_dependencies"}
// LLM analyzes codebase, infers:
// - Which services call which APIs
// - What contracts exist (OpenAPI specs, TypeScript interfaces, etc.)
// - Which user journeys go through which services
```

**What LLM sees:** A map of the system, not a list of tests.

### 2. Impact Analysis (Semantic Change Detection)

```go
// analyze {type: "impact_analysis"}
// LLM reasons:
// - What changed? (API endpoint, data structure, behavior)
// - Which services depend on this?
// - Which critical paths go through here?
// - What contracts are affected?
```

**LLM doesn't run all tests** - it tests what matters.

### 3. Critical Path Validation (Selective, Not Comprehensive)

```go
// interact {type: "validate_path"}
// LLM traces through entire user journey:
// - Frontend → Service A → Service B → Database
// - Captures all API calls
// - Validates contracts
// - Checks end-to-end behavior
```

**One test, but it's the RIGHT test.**

### 4. Contract Persistence (Not Test Cases)

Instead of test cases, we persist **contracts**:

```json
// .gasoline/contracts/auth-service.json
{
  "contracts": [
    {
      "endpoint": "POST /api/login",
      "request": {"email": "string", "password": "string"},
      "response": {"id": "number", "email": "string", "token": "string"}
    },
    {
      "endpoint": "GET /api/users/:id",
      "response": {"id": "number", "email": "string", "name": "string"}
    }
  ],
  "last_verified": "2026-01-31T10:00:00Z",
  "dependencies": ["order-service", "inventory-service"]
}
```

**When Service A changes:**
```go
// validate {type: "contract"}
// LLM compares new contract to saved contract
// - Breaking change? → Must notify dependent services
// - Non-breaking? → Just update
// - Ambiguous? → Ask human
```

---

## AI-Native Approach in Practice

### Scenario: Service A Changes, Does Service B Still Work?

**Traditional approach:**
```
1. Developer changes Service A
2. Runs Service A tests → pass
3. Commits
4. CI runs full test suite → Service B tests fail
5. Developer fixes Service B
6. Re-run CI → pass
7. Deploy
```

**AI-native approach:**
```
1. Developer changes Service A
2. LLM: analyze {type: "impact_analysis", change: "..."}
   Result: "Service B and C depend on this contract change"

3. LLM: interact {type: "validate_path", path: "user_login → place_order"}
   Result: "Service B expects {id}, got {user_id}"

4. LLM: "I found that Service B will break. Options:
   A) I can update Service B to use {user_id}
   B) I can add backward compatibility to Service A
   C) We can do this later (mark as tech debt)

   Which approach?"

5. Developer: "Add backward compatibility"

6. LLM: Updates Service A to support both {id} and {user_id}
7. LLM: Re-validates path → passes
8. LLM: Updates contract file with new field
9. Commits with confidence
```

**Key difference:**
- Traditional: Wait for tests to fail, then fix
- AI-native: Understand impact, test specific path, fix proactively

---

## How This Solves the Microservice Problem

### Problem 1: Breaking Changes Propagate

**Traditional:** Integration tests catch it (if they exist)
**AI-native:** Dependency graph + impact analysis catches it before tests run

### Problem 2: Context Limitation (LLM Can't See Everything)

**Traditional:** Need comprehensive test suite (expensive)
**AI-native:** Dependency graph compresses understanding
- LLM doesn't need to read all code
- LLM reads the graph: "A → B → C"
- LLM knows: "If A changes, test B and C"
- Compressed understanding, not exhaustive code review

### Problem 3: Orchestration Complexity

**Traditional:** CI orchestrates all tests (slow)
**AI-native:** LLM orchestrates critical paths (fast)
- Only test what changed or depends on change
- Parallel execution of independent paths
- Stop early if critical path fails

### Problem 4: Maintenance Overhead

**Traditional:** Test suite grows forever (maintenance nightmare)
**AI-native:** Contracts + graph evolve with system
- New service? LLM adds to graph
- New dependency? LLM updates graph
- Deprecated service? LLM removes from graph
- Self-maintaining

---

## What Gets Persisted?

**NOT:**
- ❌ Thousands of test cases
- ❌ Test execution history
- ❌ Expected outputs for every scenario

**YES:**
- ✅ Dependency graph (system topology)
- ✅ API contracts (what each service provides)
- ✅ Critical paths (user journeys)
- ✅ Last-known-good state (baseline for specific paths)

**This is semantic persistence, not syntactic.**

**BUT: Important Correction**

After discussion with stakeholders, we realized this was incomplete. The following ARE important:

**YES:**
- ✅ Test execution history - Prevents AI doom loops (trying same fix repeatedly)
- ✅ Edge case registry - Project-specific critical edge cases (varies by project)

**Why:**
- AI often gets into "doom loops" - tries same fix over and over
- Having execution history is critical to recognize this loop and escape
- Edge cases matter - In some software, edge cases are life/death (banking, medical)
- What varies by project - Not one-size-fits-all

---

## Confirmed Decisions: Hybrid Approaches

### 1. Dependency Graph Inference

**Decision:** ✅ Hybrid approach - LLM infers, humans review

**Why:**
- LLM is good at analyzing code and finding dependencies
- But can miss implicit dependencies (e.g., shared database schema)
- Developer review catches these
- Over time, graph becomes accurate

**Implementation:**
```go
// analyze {type: "map_dependencies", mode: "infer"}
// LLM infers graph from code

// analyze {type: "map_dependencies", mode: "review"}
// LLM presents inferred graph to developer for review
// Developer can add/remove edges

// configure {type: "set_dependencies"}
// Developer manually sets graph (override inference)
```

**File format:**
```json
// .gasoline/dependencies.json
{
  "source": "inferred", // or "manual"
  "last_inferred": "2026-01-31T10:00:00Z",
  "last_reviewed": "2026-01-31T11:30:00Z",
  "confidence": 0.85,
  "services": [
    {
      "name": "auth-service",
      "depends_on": ["database"],
      "contracts": ["POST /api/login", "GET /api/users/:id"],
      "critical_paths": ["signup", "login", "profile"]
    }
  ]
}
```

### 2. Contract Format

**Decision:** ✅ Both - Simplified JSON + OpenAPI export

**Why:**
- LLMs understand simple JSON structures better than verbose OpenAPI
- We can generate OpenAPI from simplified format when needed
- Faster iteration (no need to maintain full schema)
- Captures what matters: endpoints, inputs, outputs

**Implementation:**
```json
// .gasoline/contracts/auth-service.json
{
  "contracts": [
    {
      "endpoint": "POST /api/login",
      "inputs": {"email": "string", "password": "string"},
      "outputs": {"id": "number", "email": "string", "token": "string"},
      "errors": ["invalid_credentials", "rate_limited"]
    }
  ],
  "last_verified": "2026-01-31T10:00:00Z"
}
```

**Optional: Export to OpenAPI**
```go
// generate {type: "export_openapi"}
// Converts .gasoline/contracts/*.json to OpenAPI spec
```

### 3. Critical Paths

**Decision:** ✅ Hybrid approach - LLM suggests, users prioritize

**Why:**
- LLM can identify frequently-used paths from logs
- But LLM doesn't know business importance
- User knows "checkout flow is critical, but 'forgot password' is not"
- Hybrid combines data with business knowledge

**Implementation:**
```go
// analyze {type: "suggest_critical_paths"}
// LLM analyzes logs, suggests paths by frequency

// configure {type: "set_critical_paths"}
// User explicitly sets what's critical

// analyze {type: "prioritize_paths", paths: [...]}
// LLM validates critical paths first
```

**File format:**
```json
// .gasoline/critical-paths.json
{
  "paths": [
    {
      "name": "checkout_flow",
      "steps": ["login", "cart", "payment", "confirmation"],
      "priority": "critical",
      "sla_ms": 5000,
      "business_importance": "revenue-generating"
    }
  ]
}
```

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

2. LLM uses Gasoline MCP to explore:
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
- User enables Gasoline MCP recorder
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

## MVP vs v7: Phased Approach

### v6.0 MVP: Basic Tooling (4-6 weeks)

**Scope:** Single-app scenarios only

**What's included:**
- **Wave 1: AI-Native Toolkit**
  - Explore Capability (interact.explore, record, replay)
  - Observe Capability (observe.capture, compare)
  - Infer Capability (analyze.infer, detect_loop)

- **Wave 2: Basic Persistence**
  - Execution History (track test executions)
  - Doom Loop Prevention (pattern detection)

**What's deferred to v7:**
- ❌ Dependency graphs
- ❌ API contracts
- ❌ Critical paths
- ❌ Edge case registry

**Why defer:**
- Faster to build (2-3 weeks vs 6-8 weeks)
- Validates AI-native philosophy without complexity
- Demo is impressive enough without microservices
- Focus on core capabilities first

### v7.0: Complete AI-Native Testing (4-6 weeks)

**Scope:** Multi-service scenarios with semantic understanding

**What's added:**
- **Dependency Graph Mapping** (hybrid: LLM + human review)
- **API Contract Validation** (simplified JSON + OpenAPI export)
- **Critical Path Prioritization** (hybrid: LLM suggests, user sets)
- **Edge Case Registry** (project-specific)

**Why v7:**
- Validates AI-native philosophy works for complex systems
- Enables microservice debugging
- Complete semantic understanding
- Production-ready for enterprise

---

## Revised Persistence Model (3 Layers)

### Layer 1: Execution History (Prevents Doom Loops)

**Purpose:** Track what AI has tried, detect loops

```go
type ExecutionRecord struct {
    Timestamp   string    `json:"timestamp"`
    ChangeID    string    `json:"change_id"`
    TestFile    string    `json:"test_file"`
    Result      string    `json:"result"`       // "passed", "failed"
    Error       string    `json:"error,omitempty"`
    FixAttempt  string    `json:"fix_attempt"` // Natural language
    Confidence  float64   `json:"confidence"`
}

// analyze {type: "detect_loop", recent_attempts: []ExecutionRecord}
// Returns: {"in_loop": true, "suggestion": "Try different approach"}
```

**Doom loop detection:**
```
LLM: "I see you've tried selector updates 3 times, all failed.
     This is likely not a selector issue.
     The element might not exist, or test logic is wrong.
     
     Suggestion: Verify element actually exists in DOM first."
```

### Layer 2: Semantic Understanding (v7)

**Purpose:** Compress system understanding for LLM

**Contents:**
- Dependency graphs (hybrid inference)
- API contracts (simplified JSON + OpenAPI export)
- Critical paths (hybrid: LLM suggests, user prioritizes)

### Layer 3: Edge Case Registry (v7)

**Purpose:** Track edge cases that matter for THIS project

**Why project-specific:** "Edge cases matter" varies by project
- Banking: Race conditions in money transfer = life/death
- E-commerce: Inventory oversell = revenue impact
- Social media: Privacy leak = legal issue

**Implementation:**
```go
// configure {type: "register_edge_case"}
// User explicitly registers critical edge cases
```

```json
// .gasoline/edge-cases.json
{
  "edge_cases": [
    {
      "name": "race_condition_transfer",
      "description": "Two users transfer from same account simultaneously",
      "severity": "critical",
      "test_frequency": "every_change",
      "last_tested": "2026-01-31T10:00:00Z"
    }
  ]
}
```

---

## How LLMs Use Semantic Persistence

### Example: Change Detected in Service A (v7)

```
LLM: "I see Service A changed. Let me understand impact."

1. LLM: analyze {type: "map_dependencies"}
   Result: "A is depended on by B and C"

2. LLM: analyze {type: "impact_analysis", change: "..."}
   Result: "Breaking change: {id} → {user_id}"

3. LLM: "This breaks B and C. Let me verify."

4. LLM: interact {type: "validate_path", path: "login → order"}
   Result: "Service B fails: expected {id}"

5. LLM: "Confirmed. Should I fix B or add backward compatibility to A?"

6. Human: "Add backward compatibility"

7. LLM: Fixes A, re-tests, updates contract file

8. LLM: "All good. Critical paths validated."
```

**Context window usage:**
- Dependency graph: ~5KB (small)
- Contracts for changed service: ~2KB (small)
- Critical paths to test: ~3KB (small)
- **Total: ~10KB** - fits easily in context window

### Example: Change Detected in Service A

```
LLM: "I see Service A changed. Let me understand impact."

1. LLM: analyze {type: "map_dependencies"}
   Result: "A is depended on by B and C"

2. LLM: analyze {type: "impact_analysis", change: "..."}
   Result: "Breaking change: {id} → {user_id}"

3. LLM: "This breaks B and C. Let me verify."

4. LLM: interact {type: "validate_path", path: "login → order"}
   Result: "Service B fails: expected {id}"

5. LLM: "Confirmed. Should I fix B or add backward compatibility to A?"

6. Human: "Add backward compatibility"

7. LLM: Fixes A, re-tests, updates contract file

8. LLM: "All good. Critical paths validated."
```

**Context window usage:**
- Dependency graph: ~5KB (small)
- Contracts for changed service: ~2KB (small)
- Critical paths to test: ~3KB (small)
- **Total: ~10KB** - fits easily in context window
