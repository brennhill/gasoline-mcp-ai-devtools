# Autonomous Test Repair Mechanics

**Status:** Design Document for v6.0 Wave 1 (Self-Healing Tests feature)

**Purpose:** This document describes how Gasoline diagnoses failing E2E tests, proposes fixes, applies them autonomously, and verifies success in CI/CD pipelines with full headless capability.

---

## Table of Contents

1. [Overview](#overview)
2. [Architecture](#architecture)
3. [Data Collection Phase](#data-collection-phase)
4. [Analysis Phase](#analysis-phase)
5. [Fix Proposal Phase](#fix-proposal-phase)
6. [Verification Phase](#verification-phase)
7. [Failure Modes & Recovery](#failure-modes--recovery)
8. [Integration Points](#integration-points)
9. [Performance Budgets](#performance-budgets)

---

## Overview

### Problem Statement

E2E test failures today require manual debugging:

```
Test fails in CI
  ↓
Developer gets notification
  ↓
Developer clones repo, reproduces locally
  ↓
Developer inspects DevTools, console, network
  ↓
Developer guesses at root cause
  ↓
Developer proposes fix (code or test)
  ↓
Developer verifies it works
  ↓
Developer pushes to PR

Time: 15 minutes to 1 hour
```

### Solution: Autonomous Loop

```
Test fails in CI
  ↓
Gasoline captures full telemetry (logs, network, DOM, timeline)
  ↓
AI analyzes timeline to identify root cause
  ↓
AI proposes fix (with options)
  ↓
AI applies fix to branch
  ↓
AI reruns test (N times)
  ↓
AI verifies pass/fail
  ↓
Result posted to PR

Time: 1-2 minutes
```

### What "Autonomous" Means

- **No human intervention** during diagnosis and fix application
- **Multiple verification runs** (not just one pass)
- **Fallback handling** if initial fix doesn't work
- **Git-tracked commits** (human can review)
- **Clear reporting** of what was fixed and why

---

## Architecture

### System Components

```
┌─────────────────────────────────────────────────────────────┐
│                        CI/CD Pipeline                        │
│                                                              │
│  $ npm run test:e2e                                         │
│        ↓                                                     │
│  Test Runner (Playwright)                                   │
│        ↓                                                     │
│  [Test passes] OR [Test fails]                              │
│        ↓                                                     │
│  ┌──────────────────────────────────────────┐              │
│  │ Gasoline Server (HTTP + MCP)             │              │
│  │                                          │              │
│  │  ├─ Browser Extension                   │              │
│  │  │   (collect telemetry from test run)  │              │
│  │  │                                       │              │
│  │  ├─ Ring Buffers                        │              │
│  │  │   - Timeline (T+Nms events)          │              │
│  │  │   - Network waterfall                │              │
│  │  │   - Console logs                     │              │
│  │  │   - DOM snapshots                    │              │
│  │  │   - Performance metrics              │              │
│  │  │                                       │              │
│  │  └─ Analysis Engine                     │              │
│  │      (diagnose root cause)              │              │
│  └──────────────────────────────────────────┘              │
│        ↓                                                     │
│  ┌──────────────────────────────────────────┐              │
│  │ AI Agent (Claude via MCP)                │              │
│  │                                          │              │
│  │  1. Read test output + Gasoline data    │              │
│  │  2. Diagnose root cause                 │              │
│  │  3. Propose fix(es)                     │              │
│  │  4. Apply fix to git branch             │              │
│  │  5. Rerun test N times                  │              │
│  │  6. Analyze new results                 │              │
│  │  7. Report: Pass/Fail/Inconclusive      │              │
│  └──────────────────────────────────────────┘              │
│        ↓                                                     │
│  GitHub API: Create/update PR comment                       │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

### Key Systems

| System | Purpose | Data | Update Frequency |
|--------|---------|------|------------------|
| **Timeline Correlator** | Link events to precise millisecond | Navigation, XHR, DOM, console, paint | Real-time |
| **Network Analyzer** | Compare requests/responses | HTTP headers, status, body, timing | Per request |
| **DOM Snapshot** | Capture state at failure | Full DOM tree, accessibility tree | At test failure |
| **Console Aggregator** | Collect logs/errors | Logs, errors, warnings, timestamps | Per log |
| **Causal Graph** | Link cause → effect | What changed, what failed | At analysis time |

---

## Data Collection Phase

### What Gasoline Captures

During test execution, Gasoline's browser extension (running in the test tab) captures:

#### 1. Timeline Events

```javascript
// Every event gets a precise timestamp
class TimelineEvent {
  timestamp: number;           // ms since start
  type: 'navigation' | 'network' | 'dom' | 'console' | 'paint';
  source: 'extension' | 'server';

  // Type-specific fields
  navigation?: { url, navigationType };
  network?: { url, method, status, duration };
  dom?: { action, selector, nodeName, details };
  console?: { level, message, stackTrace };
  paint?: { metric, value };
}

// Example: 100 events captured during 5s test run
Timeline = [
  { timestamp: 0, type: 'navigation', url: 'http://localhost:3000/checkout' },
  { timestamp: 45, type: 'paint', metric: 'FCP', value: 45 },
  { timestamp: 150, type: 'paint', metric: 'LCP', value: 150 },
  { timestamp: 200, type: 'dom', action: 'fill', selector: 'input[name="email"]' },
  { timestamp: 250, type: 'dom', action: 'click', selector: 'button[type="submit"]' },
  { timestamp: 270, type: 'network', method: 'POST', url: '/checkout', status: null },  // Request
  { timestamp: 1200, type: 'network', method: 'POST', url: '/checkout', status: 200, duration: 930 },  // Response
  { timestamp: 1210, type: 'dom', action: 'insert', selector: '.confirmation' },
  { timestamp: 1215, type: 'paint', metric: 'CLS', value: 0.05 },
  { timestamp: 5000, type: 'console', level: 'error', message: 'Timeout assertion failed' },
  // ... more events
]
```

**Collection method:** MutationObserver (DOM), fetch intercept (network), console override, PerformanceObserver (metrics)

#### 2. Network Request/Response Bodies

```javascript
NetworkCapture = {
  url: 'https://api.example.com/checkout',
  method: 'POST',
  requestHeaders: {
    'Content-Type': 'application/json',
    'Authorization': '<REDACTED>',  // Stripped for security
  },
  requestBody: {
    email: 'test@example.com',
    items: [123, 456],
    total: 99.99
  },

  statusCode: 200,
  responseHeaders: {
    'Content-Type': 'application/json',
    'X-RateLimit-Remaining': '4999'
  },
  responseBody: {
    success: true,
    confirmationId: 'CH-12345',
    processingTime: 930  // milliseconds
  },

  // Captured metrics
  startTime: 270,      // ms since test start
  endTime: 1200,       // ms since test start
  transferSize: 4321,  // bytes (compressed)
  decompressedSize: 45123,  // bytes (uncompressed)
};
```

**Collection method:** Override fetch/XMLHttpRequest, capture before/after, store in ring buffer

#### 3. DOM Snapshots

At test failure, Gasoline captures the full DOM:

```javascript
DOMSnapshot = {
  capturedAt: 5000,  // When failure occurred
  selector: '.checkout-form',  // Context (what was the test checking)

  html: `
    <form class="checkout-form">
      <input type="email" name="email" value="test@example.com">
      <button type="submit">Complete Checkout</button>
      <!-- Spinner is NOT here — it was removed at T+1210ms -->
    </form>
  `,

  accessibility: {
    // AOM tree (what screen readers see)
    role: 'form',
    name: 'Checkout form',
    children: [
      { role: 'textbox', name: 'Email address', value: 'test@example.com' },
      { role: 'button', name: 'Complete Checkout' }
    ]
  },

  computedStyles: {
    display: 'block',
    visibility: 'visible',
    opacity: 1,
    // ...
  }
};
```

**Collection method:** document.body.outerHTML + getComputedStyle + AOM serialization

#### 4. Console Logs

```javascript
ConsoleLogs = [
  { timestamp: 100, level: 'log', message: 'Loading checkout...' },
  { timestamp: 270, level: 'log', message: 'Submitting form...' },
  { timestamp: 1200, level: 'log', message: 'Server response received' },
  { timestamp: 1210, level: 'log', message: 'Rendering confirmation...' },
  { timestamp: 5000, level: 'error', message: 'Timeout: element .spinner not found' },
];
```

**Collection method:** Override console methods, timestamp each call

### Data Storage

All captured data is stored in **ring buffers** (bounded memory):

```go
// In Gasoline server
type RingBuffer struct {
  capacity int        // e.g., 1000 entries
  data     []Event    // Fixed-size array
  head     int        // Write position
  tail     int        // Read position
  lock     sync.RWMutex
}

// Buffer sizes (per test run)
Timeline:         10,000 events max (~1MB)
Network bodies:     100 requests max (8MB total)
Console logs:     1,000 entries max (~100KB)
DOM snapshots:       10 snapshots max (~10MB)
```

**Why ring buffers?** Bounded memory even for long test runs. Oldest data drops off as new data arrives. No unbounded growth.

---

## Analysis Phase

### Step 1: Timeline Correlation

When a test fails, Gasoline correlates all events into a causal sequence:

```javascript
// Pseudo-code for analysis engine
function analyzeFailure(testOutput, timeline, networkCapture, domSnapshot, consoleLogs) {

  // 1. Find the failure point
  const failureTime = testOutput.failedAt;  // 5000ms
  const failureReason = testOutput.message;  // "Timeout waiting for .spinner"

  // 2. Build causal chain UP TO failure
  const causalChain = timeline
    .filter(e => e.timestamp <= failureTime)
    .sort((a, b) => a.timestamp - b.timestamp);

  // 3. Analyze what happened BEFORE failure
  const beforeFailure = causalChain.slice(-20);  // Last 20 events

  return {
    timelineUpToFailure: beforeFailure,
    networkAtFailure: networkCapture.filter(n => n.endTime < failureTime),
    domAtFailure: domSnapshot,
    consoleLogs: consoleLogs.filter(l => l.timestamp < failureTime),
  };
}
```

### Step 2: Root Cause Categorization

The analysis engine (or AI, depending on complexity) categorizes the root cause:

```javascript
RootCauseCategory = {
  'TEST_ASSUMPTION': {
    description: 'Test expects behavior that code doesn\'t provide',
    examples: [
      'Spinner visible for 5s, but code hides it immediately',
      'API response in 100ms, but test expects 1s',
      'Form submits synchronously, test expects async'
    ],
    fix: 'Adjust test expectations to match actual behavior'
  },

  'CODE_BUG': {
    description: 'Code changed, breaking expected behavior',
    examples: [
      'Selector changed from .spinner to .loader',
      'API response shape changed (old: name, new: person.fullName)',
      'Race condition: DOM update happens before assertion'
    ],
    fix: 'Fix code to restore expected behavior'
  },

  'ENVIRONMENT_ISSUE': {
    description: 'Test environment differs from expected',
    examples: [
      'API timeout (network slow)',
      'Missing test data (seed data issue)',
      'CSS file didn\'t load (network failure)'
    ],
    fix: 'Retry test or adjust timeouts'
  },

  'FLAKY_TIMING': {
    description: 'Race condition or timing-dependent behavior',
    examples: [
      'API sometimes takes 300ms, sometimes 1200ms',
      'DOM mutation races with assertion',
      'setTimeout behavior undefined'
    ],
    fix: 'Add explicit waits or retries'
  }
};
```

### Step 3: AI Diagnostic Reasoning

Claude (via MCP) receives the correlated data and reasons:

```
Input:
- Test error: "Timeout waiting for .loading-spinner"
- Timeline: [... events from T+0 to T+5000 ...]
- Network: POST /checkout returned 200 at T+1200ms
- DOM at T+5000ms: .loading-spinner is NOT in DOM
- DOM mutation log: .loading-spinner was REMOVED at T+1210ms
- Console: "Rendering confirmation..." at T+1210ms

AI Reasoning:
1. Test expects .loading-spinner to exist at T+5000ms
2. But .loading-spinner was removed at T+1210ms
3. The removal happened AFTER API response (T+1200ms)
4. This is the CORRECT flow (spinner hidden when done loading)
5. Test assumption is WRONG
6. Root cause: TEST_ASSUMPTION (not code bug)
7. Fix: Change assertion from .loading-spinner to .confirmation

Confidence: HIGH (timeline clearly shows element removal)
```

This reasoning is codified in a prompt that Claude receives via MCP `observe()`:

```javascript
observe({
  what: 'timeline',
  analyze: true,  // Request analysis
  context: {
    failedAssertion: ".loading-spinner",
    failureTime: 5000,
    failureMessage: "Timeout waiting for selector"
  }
})
```

Gasoline returns:

```javascript
{
  analysis: {
    rootCause: 'TEST_ASSUMPTION',
    reasoning: [
      'Element .loading-spinner removed at T+1210ms',
      'Test assertion ran at T+5000ms (3.7s later)',
      'This is a flawed test assumption, not a code bug'
    ],
    causalChain: [
      { t: 270, event: 'POST /checkout' },
      { t: 1200, event: 'Response 200' },
      { t: 1210, event: '.loading-spinner removed' },
      { t: 1210, event: '.confirmation inserted' },
      { t: 5000, event: 'Test assertion failed' }
    ],
    confidence: 0.95  // 95% confidence
  },

  suggestedFixes: [
    {
      type: 'UPDATE_TEST',
      description: 'Wait for .confirmation instead of .spinner',
      reasoning: 'The final state is .confirmation visible, not .spinner'
    },
    {
      type: 'UPDATE_CODE',
      description: 'Keep spinner visible longer (e.g., 500ms)',
      reasoning: 'If spinner is meant to be visible, extend its duration'
    }
  ]
}
```

---

## Fix Proposal Phase

### Option Generation

AI generates multiple fix options, ranked by likelihood of success:

```javascript
// fix-proposal.ts

interface FixProposal {
  id: string;
  type: 'TEST_CHANGE' | 'CODE_CHANGE' | 'RETRY' | 'TIMEOUT_INCREASE';
  description: string;
  reasoning: string;
  changes: FileChange[];
  estimatedSuccessRate: number;  // 0.0-1.0
  estimatedRisk: 'LOW' | 'MEDIUM' | 'HIGH';
  verificationSteps: string[];
}

// Example: Timeout issue
const fixes = [
  {
    id: 'fix-001',
    type: 'TEST_CHANGE',
    description: 'Change assertion from spinner to confirmation',
    reasoning: 'Timeline shows spinner removed at T+1210ms, test checks at T+5000ms',
    changes: [{
      path: 'test/checkout.test.ts',
      oldContent: '  await expect(page.locator(\'.loading-spinner\')).toBeVisible();',
      newContent: '  await expect(page.locator(\'.confirmation\')).toBeVisible();'
    }],
    estimatedSuccessRate: 0.95,
    estimatedRisk: 'LOW',
    verificationSteps: [
      'Run test 10 times',
      'Check for any console errors',
      'Verify DOM state matches expectation'
    ]
  },

  {
    id: 'fix-002',
    type: 'TIMEOUT_INCREASE',
    description: 'Increase timeout to 10s',
    reasoning: 'Maybe spinner takes longer than expected in slow environments',
    changes: [{
      path: 'test/checkout.test.ts',
      oldContent: '  await expect(page.locator(\'.loading-spinner\')).toBeVisible();',
      newContent: '  await expect(page.locator(\'.loading-spinner\')).toBeVisible({ timeout: 10000 });'
    }],
    estimatedSuccessRate: 0.30,  // Low success rate — doesn't fix underlying issue
    estimatedRisk: 'MEDIUM',     // Masks the real problem
    verificationSteps: [
      'Run test 10 times',
      'Check if timeout still occurs'
    ]
  }
];

// AI picks the highest success rate + lowest risk
selectedFix = fixes[0];  // fix-001
```

### Commit Generation

AI generates a descriptive commit message:

```
fix: Update dashboard test spinner assertion

Timeline analysis revealed the loading spinner is removed at T+1210ms,
but the test assertion runs at T+5000ms. This is not a code bug; it's
a test assumption error.

Gasoline Analysis:
  - API response received at T+1200ms (acceptable latency)
  - DOM updated immediately after (spinner removed, content shown)
  - Test incorrectly expects spinner to exist 3.8s after completion

Fix: Changed assertion from waiting for .loading-spinner to waiting for
.dashboard-content (the actual final state).

Verification: Test now passes consistently (100/100 runs). No timeouts.

Related: #123 (original test failure in CI)
```

---

## Verification Phase

### Step 1: Apply Fix

```bash
# Autonomously executed by AI agent
$ git checkout -b fix/test-dashboard-spinner-$(date +%s)
$ # Edit file
$ git add test/dashboard.test.ts
$ git commit -m "fix: Update dashboard test spinner assertion..."
```

### Step 2: Run Multiple Test Iterations

```bash
# Run test N times to catch flakiness
$ for i in {1..10}; do
    npm test -- --testNamePattern="dashboard"
  done
```

Gasoline captures full telemetry on each run:

```javascript
VerificationResults = [
  { run: 1, status: 'PASS', duration: 1865ms, failureRate: 0 },
  { run: 2, status: 'PASS', duration: 1870ms, failureRate: 0 },
  { run: 3, status: 'PASS', duration: 1852ms, failureRate: 0 },
  { run: 4, status: 'PASS', duration: 1868ms, failureRate: 0 },
  { run: 5, status: 'PASS', duration: 1875ms, failureRate: 0 },
  { run: 6, status: 'PASS', duration: 1859ms, failureRate: 0 },
  { run: 7, status: 'PASS', duration: 1861ms, failureRate: 0 },
  { run: 8, status: 'PASS', duration: 1870ms, failureRate: 0 },
  { run: 9, status: 'PASS', duration: 1856ms, failureRate: 0 },
  { run: 10, status: 'PASS', duration: 1869ms, failureRate: 0 },
];

successRate = 10/10 = 100%;
averageDuration = 1865ms;
conclusion = 'PASSED — Fix is stable';
```

### Step 3: Analyze Results

```javascript
function analyzeVerificationResults(results) {
  const passed = results.filter(r => r.status === 'PASS').length;
  const failed = results.filter(r => r.status === 'FAIL').length;
  const successRate = passed / results.length;

  if (successRate >= 0.95) {
    return {
      status: 'VERIFIED',
      confidence: 'HIGH',
      reason: 'Test passed 95%+ of runs'
    };
  } else if (successRate >= 0.80) {
    return {
      status: 'PARTIAL',
      confidence: 'MEDIUM',
      reason: 'Test passed 80%+ of runs, still somewhat flaky'
    };
  } else if (successRate >= 0.50) {
    return {
      status: 'INCONCLUSIVE',
      confidence: 'LOW',
      reason: 'Test passes < 80%, fix did not fully resolve issue'
    };
  } else {
    return {
      status: 'FAILED',
      confidence: 'HIGH',
      reason: 'Fix did not work, root cause analysis was incorrect'
    };
  }
}
```

### Step 4: Report Results

If verification succeeds, AI posts to PR:

```markdown
✅ **Gasoline Auto-Fix: VERIFIED**

**Issue:** Test timeout waiting for `.loading-spinner`

**Root Cause:** Test assumption error. Timeline showed spinner was removed
at T+1210ms, but test checked for it at T+5000ms.

**Fix Applied:**
- Changed assertion from `.loading-spinner` to `.dashboard-content`
- Now asserts on final state instead of transient indicator

**Verification:** 10 consecutive test runs
- Status: ✅ All passed
- Average duration: 1865ms
- Success rate: 100%

**Commit:** [e3f4a1e](link-to-commit)

You can review the change and merge with confidence.
```

If verification fails, AI tries next fix option or escalates to human.

---

## Failure Modes & Recovery

### Mode 1: Fix Didn't Work (Success Rate < 50%)

```
Initial fix → Run tests → Success rate 30% → FAIL

Recovery:
1. Revert commit
2. Try next suggested fix
3. Repeat analysis with new fix
4. If all fixes fail: Escalate to human with full diagnostics
```

### Mode 2: Ambiguous Root Cause

```
Timeline shows two possible causes:
  A. Spinner removed too quickly (code issue)
  B. Test assumption wrong (test issue)

AI requests clarification:
  "I'm 60% confident this is a test assumption issue.
   But there's a 40% chance the code timing changed.

   Let me try Fix A (test change) first. If that fails,
   I'll try Fix B (code timeout)."

Try Fix A (60% likely) → Run tests → Success rate 90% → ✅ VERIFIED
```

### Mode 3: Test Passes But Performance Regresses

```
Original test: ~2s
Fixed test: ~5s (now it waits longer)

Recovery:
AI detects regression and notes:
  "Fix is functionally correct (assertions pass),
   but test duration increased 2.5x.

   This suggests the wait target is slow.
   Recommend investigating code performance."
```

### Mode 4: Network Issue (Not Code/Test Issue)

```
Failure: API timeout (5000ms limit)
Timeline: Request sent at T+270ms, response at T+5050ms
Gasoline detects: NETWORK_LATENCY (not code/test issue)

AI skips fix and reports:
  "This appears to be a network/infrastructure issue,
   not a code or test bug. The API is slow (5+ seconds).

   Recommendation: Check server performance metrics."
```

---

## Integration Points

### 1. Test Runner Integration

Gasoline hooks into test runner exit codes:

```bash
# In CI/CD pipeline
npm test:e2e
EXIT_CODE=$?

if [ $EXIT_CODE -ne 0 ]; then
  # Test failed, trigger Gasoline auto-repair
  gasoline-repair --mode=auto --testOutput=$TEST_OUTPUT
fi
```

### 2. Git Integration

AI agent has git access (scoped):

```javascript
// Allowed operations
- Checkout new branch (fix/* prefix)
- Commit to branch
- Push to fork
- Create/update PR comments

// NOT allowed
- Push to main
- Force push
- Delete branches
- Modify git history
```

### 3. MCP Integration

All data flows through MCP:

```javascript
// AI agent calls
observe({ what: 'timeline', analyze: true })
observe({ what: 'network_bodies' })
observe({ what: 'console_logs' })
generate({ format: 'test', fixes: [...] })
configure({ action: 'record_event', event: 'fix_applied' })
interact({ action: 'execute_js', script: 'rerun test' })
```

### 4. GitHub/GitLab Integration

```javascript
// Post results to PR
gh pr comment <PR_ID> -b "✅ **Gasoline Auto-Fix: VERIFIED**

Root Cause: Timeline analysis showed...
Fix Applied: ...
Verification: 10 runs, 100% pass rate
"
```

---

## Performance Budgets

### Time Budgets

| Phase | Target | Notes |
|-------|--------|-------|
| Data Collection | Real-time | Passive, no test overhead |
| Timeline Analysis | < 100ms | Correlate events in-memory |
| AI Diagnosis | 30-60s | Claude reasoning (depends on model) |
| Fix Proposal | 5-10s | Generate diffs |
| Git Operations | 10-15s | Checkout, commit, push |
| Test Verification | 30-120s | 10 runs × 3-12s per run |
| **Total Time** | **~2-5 minutes** | Start to finish |

### Memory Budgets

| Buffer | Max Size | Rationale |
|--------|----------|-----------|
| Timeline | 10,000 events | ~1MB (enough for 5-10 minute test) |
| Network bodies | 100 requests | 8MB (typical test suite) |
| Console logs | 1,000 entries | ~100KB |
| DOM snapshots | 10 snapshots | ~10MB (saved on demand) |
| **Total** | ~19MB | Bounded, no unbounded growth |

### Accuracy Targets (v6.0)

| Metric | Target | Notes |
|--------|--------|-------|
| Root cause identification accuracy | 90%+ | AI correctly categorizes test vs code issues |
| Fix success rate | 80%+ | Applied fix resolves the issue |
| Verification stability | 95%+ | Test passes consistently after fix |
| False positives | < 5% | Don't incorrectly "fix" things that aren't broken |

---

## Data Flow Diagram

```
Test Execution
    ↓
Browser Extension (capture):
  - Timeline events
  - Network requests/responses
  - DOM mutations
  - Console logs
  ↓
Gasoline Server (store):
  - Ring buffers
  - Correlation
  ↓
Test fails (exit code != 0)
    ↓
Analysis Engine:
  - Build causal chain
  - Categorize root cause
  - Generate options
    ↓
MCP: observe({ what: 'timeline', analyze: true })
    ↓
AI Agent (Claude):
  - Diagnose root cause
  - Select fix
  - Generate commit message
    ↓
Git Operations:
  - Checkout branch
  - Apply changes
  - Commit
    ↓
Test Verification:
  - Rerun test 10x
  - Collect telemetry
  - Analyze results
    ↓
Report:
  - Post to PR
  - Update issue
  - Record metrics
```

---

## Future Extensions

### Multi-Failure Analysis

If multiple tests fail:

```
Test A fails (timeout) → Diagnose → Propose fix
Test B fails (API) → Diagnose → Propose fix
Test C fails (selector) → Diagnose → Propose fix

Smart ordering:
  1. Fix shared root causes first (e.g., API schema change)
  2. Then fix isolated issues
  3. Verify all fixes together
```

### Learning from Patterns

```
Track patterns across runs:
  "10 tests failed due to 'selector changed' this week"
  → Suggest codemod to update all affected tests
  → Apply fix to entire test suite
```

### Fallback to Manual Debugging Mode

If AI confidence is low:

```
"I'm only 40% confident in the root cause.
 Rather than risk applying the wrong fix,
 I've captured full telemetry for manual debugging.

 [Download telemetry as HAR + timeline.json]"
```

---

## References

- [Gasoline Architecture](./architecture.md)
- [Timeline Data Structure](../../.claude/refs/timeline.md)
- [Self-Healing Tests Feature Spec](../features/feature/33-self-healing-tests)
- [v6.0 Roadmap](../roadmap.md)
