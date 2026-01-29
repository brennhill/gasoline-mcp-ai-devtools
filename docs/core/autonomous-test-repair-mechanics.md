# Autonomous Test Repair Mechanics

**Status:** Design Document for v6.0 Wave 1 (Self-Healing Tests feature)

**Purpose:** This document describes how Gasoline enables Claude to autonomously diagnose failing E2E tests, propose fixes, apply them, and verify success in CI/CD pipelines.

**Design Philosophy:** Gasoline captures and correlates telemetry. Claude handles all reasoning, fix generation, and verification.

---

## Table of Contents

1. [Overview](#overview)
2. [Architecture](#architecture)
3. [Gasoline's Role](#gasolines-role)
4. [Claude's Role](#claudes-role)
5. [Data Collection](#data-collection)
6. [Example Workflow](#example-workflow)
7. [Integration Points](#integration-points)
8. [Performance Budgets](#performance-budgets)

---

## Overview

### The Problem

E2E test failures in CI require manual debugging:

```
Test fails in CI
  ↓
Developer gets notification
  ↓
Developer clones repo, reproduces locally (5 mins)
  ↓
Developer inspects DevTools, console, network (10 mins)
  ↓
Developer guesses at root cause (10 mins)
  ↓
Developer proposes fix (5 mins)
  ↓
Developer verifies it works (5 mins)
  ↓
Developer pushes to PR

Total: 15-60 minutes
```

### The Solution

Gasoline captures full telemetry. Claude reads it and fixes autonomously:

```
Test fails in CI
  ↓
Gasoline captures: timeline, network, logs, DOM
  ↓
Claude reads timeline via MCP
  ↓
Claude diagnoses root cause (no guessing, facts from telemetry)
  ↓
Claude applies fix via git
  ↓
Claude spawns subagent to verify (10 test runs)
  ↓
Claude posts results to PR

Total: 1-2 minutes
```

### Key Insight

**Gasoline provides facts. Claude provides reasoning.**

Gasoline doesn't need to understand why a test failed. It just records everything that happened. Claude reads the facts and reasons over them.

---

## Architecture

### System Components

```
┌──────────────────────────────────────────────────────────────┐
│                    CI/CD Pipeline                            │
│                    npm test:e2e                              │
└──────────────────────┬───────────────────────────────────────┘
                       ↓
┌──────────────────────────────────────────────────────────────┐
│                Gasoline Server (Go)                          │
│                                                              │
│  Browser Extension (Capture)                                │
│  ├─ Timeline events (T+Nms)                                │
│  ├─ Network request/response                               │
│  ├─ DOM mutations                                          │
│  ├─ Console logs                                           │
│  └─ Performance metrics                                    │
│           ↓                                                 │
│  Ring Buffers (Storage)                                    │
│  ├─ Timeline: 10,000 events (~1MB)                        │
│  ├─ Network: 100 requests (8MB)                           │
│  ├─ Logs: 1,000 entries (~100KB)                          │
│  └─ DOM snapshots: 10 snapshots (~10MB)                   │
│           ↓                                                 │
│  Timeline Correlator                                       │
│  └─ Sort events by timestamp                              │
│     Convert raw events → chronological sequence            │
│                                                              │
└──────────────┬───────────────────────────────────────────────┘
               ↓
┌──────────────────────────────────────────────────────────────┐
│           Claude AI Agent (via MCP)                          │
│                                                              │
│  1. observe({ what: 'timeline' })                           │
│     → Get correlated events in time order                   │
│                                                              │
│  2. Read test output + timeline                             │
│     → Diagnose root cause (test vs code vs env)            │
│                                                              │
│  3. Apply fix autonomously                                  │
│     → git checkout -b fix/...                              │
│     → edit files                                           │
│     → git commit                                           │
│     → git push                                             │
│                                                              │
│  4. Spawn verification subagent                             │
│     → Run test 10 times                                    │
│     → Collect results                                      │
│     → Return: { passed: 10, failed: 0 }                    │
│                                                              │
│  5. Post results to PR                                      │
│     → Create GitHub comment                                │
│     → Include diagnosis + verification stats              │
│                                                              │
└──────────────┬───────────────────────────────────────────────┘
               ↓
         GitHub PR Updated
```

### Key Principle: Separation of Concerns

| Component | Responsibility | Rationale |
|-----------|-----------------|-----------|
| **Gasoline** | Capture + correlate telemetry | Record facts objectively |
| **Claude** | Reason + diagnose + apply + verify | Make intelligent decisions |
| **GitHub** | Report results | Provide feedback to team |

Gasoline stays **dumb but accurate**. Claude stays **intelligent but fact-based**.

---

## Gasoline's Role

### 1. Telemetry Capture (Browser Extension)

While the test runs, capture:

```javascript
// Timeline events
[
  { timestamp: 0, type: 'navigation', url: '/checkout' },
  { timestamp: 245, type: 'network', method: 'POST', url: '/api/checkout', status: null },
  { timestamp: 1200, type: 'network', method: 'POST', url: '/api/checkout', status: 200, duration: 955 },
  { timestamp: 1210, type: 'dom', action: 'inserted', selector: '.confirmation' },
  { timestamp: 1210, type: 'dom', action: 'removed', selector: '.loading-spinner' },
  { timestamp: 5000, type: 'console', level: 'error', message: 'Timeout assertion failed' },
]

// Network bodies
[
  {
    url: 'https://api.example.com/checkout',
    method: 'POST',
    requestBody: { email: 'test@example.com' },
    responseBody: { confirmationId: 'CH-12345' },
    statusCode: 200,
    duration: 955
  }
]

// Console logs
[
  { timestamp: 0, level: 'log', message: 'Loading checkout page' },
  { timestamp: 245, level: 'log', message: 'Submitting form' },
  { timestamp: 1200, level: 'log', message: 'Order confirmed' },
  { timestamp: 5000, level: 'error', message: 'Assertion timeout' },
]

// DOM snapshot at failure
{
  capturedAt: 5000,
  html: '<form>...</form>',
  exists: {
    '.loading-spinner': false,  // This is important!
    '.confirmation': true
  }
}
```

**How it's captured:**
- MutationObserver for DOM changes
- fetch/XHR interception for network
- console override for logs
- PerformanceObserver for metrics

All events tagged with microsecond timestamps.

### 2. Ring Buffer Storage (Bounded Memory)

Store captured data in fixed-size buffers:

```go
// In Gasoline server
type RingBuffer struct {
  capacity int        // e.g., 10,000
  data     []Event    // Fixed-size array
  head     int        // Write position
  tail     int        // Read position
}

// Buffers
Timeline:    10,000 events (~1MB)
Network:       100 requests (8MB)
Logs:        1,000 entries (~100KB)
DOM snapshots: 10 snapshots (~10MB)
Total:       ~19MB maximum (bounded)
```

When capacity is reached, oldest entries drop off. No unbounded growth.

### 3. Timeline Correlation

Sort events by timestamp to create a causal sequence:

```go
// Pseudo-code
func correlateTimeline(events []Event) []Event {
  // Sort by timestamp
  sort.Slice(events, func(i, j int) bool {
    return events[i].timestamp < events[j].timestamp
  })
  return events
}

// Input (scrambled order):
[
  { t: 1210, type: 'dom', action: 'inserted', selector: '.confirmation' },
  { t: 245, type: 'network', method: 'POST', status: null },
  { t: 1200, type: 'network', method: 'POST', status: 200 },
  { t: 1210, type: 'dom', action: 'removed', selector: '.spinner' },
]

// Output (time-ordered):
[
  { t: 245, type: 'network', method: 'POST', status: null },      ← Request
  { t: 1200, type: 'network', method: 'POST', status: 200 },     ← Response
  { t: 1210, type: 'dom', action: 'removed', selector: '.spinner' },  ← Spinner hidden
  { t: 1210, type: 'dom', action: 'inserted', selector: '.confirmation' }, ← Confirmation shown
]
```

That's it. No AI reasoning. Just sorting by timestamp.

---

## Claude's Role

### 1. Read Telemetry via MCP

```javascript
Claude calls:
observe({
  what: 'timeline',
  limit: 100  // Last 100 events
})

Gasoline returns:
[
  { timestamp: 0, type: 'navigation', url: '/checkout' },
  { timestamp: 245, type: 'network', ... },
  { timestamp: 1200, type: 'network', status: 200 },
  { timestamp: 1210, type: 'dom', action: 'inserted' },
  { timestamp: 1210, type: 'dom', action: 'removed' },
  { timestamp: 5000, type: 'console', level: 'error', message: 'Timeout' },
]
```

### 2. Diagnose Root Cause

Claude reads test output + timeline and reasons:

```
Test output: "Timeout waiting for .loading-spinner"

Timeline shows:
- T+245ms: POST /checkout starts
- T+1200ms: POST response (200 OK)
- T+1210ms: .loading-spinner removed
- T+1210ms: .confirmation inserted
- T+5000ms: Test assertion (too late!)

Claude reasoning:
"The spinner was removed at T+1210ms, but the test checked for it
at T+5000ms. This is a test assumption issue, not a code bug.

The code flow is correct:
1. Submit form (T+250ms)
2. API responds (T+1200ms)
3. Hide spinner (T+1210ms)
4. Show confirmation (T+1210ms)

The test expected the spinner to persist, but it was immediately
hidden after the API response. This is the correct behavior.

Fix: Change assertion from .loading-spinner to .confirmation"
```

**No Gasoline analysis engine needed.** Claude just reads the facts.

### 3. Apply Fix

```bash
Claude executes:
$ git checkout -b fix/checkout-spinner-timeout-$(date +%s)
$ # Edit test file
$ git add test/checkout.test.ts
$ git commit -m "fix: Update assertion from spinner to confirmation

Timeline analysis showed spinner removed at T+1210ms, test
checked at T+5000ms. This is a test assumption error."
$ git push origin fix/checkout-spinner-timeout-...
```

Standard git operations. Claude can do this directly via Bash tool or MCP `interact()`.

### 4. Verify (Spawn Subagent)

Claude spawns a background subagent to run tests:

```bash
Claude executes:
$ npm test -- --testNamePattern="checkout" --repeat=10

Subagent captures:
Run 1:  ✅ PASS (1865ms)
Run 2:  ✅ PASS (1870ms)
Run 3:  ✅ PASS (1852ms)
...
Run 10: ✅ PASS (1869ms)

Returns to Claude:
{
  status: 'PASSED',
  successRate: 1.0,
  averageDuration: 1865,
  allPassed: true
}
```

Claude gets the result and proceeds.

### 5. Post Results to PR

```bash
Claude executes:
$ gh pr comment <PR_ID> -b "✅ **Gasoline Auto-Fix: VERIFIED**

Root Cause: Test assumption issue. Timeline showed spinner removed
at T+1210ms, but test expected it at T+5000ms.

Fix Applied:
- Changed assertion from .loading-spinner to .confirmation

Verification: 10 test runs
- Status: ✅ All passed
- Average duration: 1865ms
- Success rate: 100%

Commit: [abc1234](link)

Ready to merge!"
```

**No Gasoline reporting engine.** Claude just uses GitHub API.

---

## Data Collection

### What Gasoline Captures

#### Timeline Events

```javascript
TimelineEvent = {
  timestamp: number,           // ms since start
  type: 'navigation' | 'network' | 'dom' | 'console' | 'paint' | 'error',

  // Type-specific data
  navigation?: { url, navigationType },
  network?: { url, method, status, duration, startTime, endTime },
  dom?: { action, selector, html, nodeName },
  console?: { level, message, stackTrace },
  paint?: { metric, value },
  error?: { message, stackTrace, source }
};

// Example: Typical test run captures 50-200 events
```

#### Network Request/Response Bodies

```javascript
NetworkCapture = {
  url: string,
  method: 'GET' | 'POST' | 'PUT' | 'DELETE',

  requestHeaders: { [key]: string },
  requestBody: any,

  responseHeaders: { [key]: string },
  responseBody: any,
  statusCode: number,

  // Metadata
  startTime: number,   // ms since test start
  endTime: number,
  duration: number,
  transferSize: number,
  decompressedSize: number
};

// Example: Test makes 2-10 network requests typically
```

#### Console Logs

```javascript
ConsoleLog = {
  timestamp: number,
  level: 'log' | 'warn' | 'error' | 'debug',
  message: string,
  stackTrace?: string,
  source: 'page' | 'extension'
};

// Example: Test produces 5-50 console entries
```

#### DOM Snapshots

```javascript
DOMSnapshot = {
  capturedAt: number,  // timestamp
  selector: string,    // what test was checking
  html: string,        // full HTML
  exists: {
    [selector]: boolean  // Quick lookup: does element exist?
  }
};

// Example: Captured at test failure time
```

### Storage Model

All captured data goes into **ring buffers** (fixed-size, no unbounded growth):

| Buffer | Capacity | Notes |
|--------|----------|-------|
| Timeline | 10,000 events | ~1MB per test run |
| Network | 100 requests | 8MB max |
| Console | 1,000 entries | ~100KB |
| DOM snapshots | 10 snapshots | ~10MB on demand |
| **Total** | | **~19MB max** |

When capacity is hit, oldest entries drop off. Memory is always bounded.

---

## Example Workflow

### Scenario: Test Timeout

#### Step 1: Test Fails

```
$ npm test:e2e
 FAIL  src/__tests__/checkout.test.ts
  ✕ should complete checkout flow
    Error: Timeout waiting for selector ".loading-spinner"
    at test/checkout.test.ts:18
```

#### Step 2: Gasoline Captures

During the test run, Gasoline's extension captured everything.

Server has ring buffers full of:
- Timeline (245ms POST start, 1200ms response, 1210ms spinner removed, 5000ms timeout)
- Network (POST /checkout → 200 OK, 955ms duration)
- Console (logs about form submission, order confirmation)
- DOM snapshot (at failure: spinner is NOT in DOM)

#### Step 3: Claude Reads Timeline

```javascript
Claude calls:
observe({ what: 'timeline', limit: 50 })

Returns:
[
  { t: 245, type: 'network', method: 'POST', status: null },
  { t: 1200, type: 'network', method: 'POST', status: 200, duration: 955 },
  { t: 1210, type: 'dom', action: 'removed', selector: '.loading-spinner' },
  { t: 1210, type: 'dom', action: 'inserted', selector: '.confirmation' },
  { t: 5000, type: 'console', level: 'error', message: 'Timeout' }
]
```

#### Step 4: Claude Diagnoses

Claude reads timeline and test error. Diagnosis:

> "Timeline clearly shows spinner removed at T+1210ms. Test asserted at T+5000ms.
> The test expected spinner to exist for 5+ seconds, but code hides it immediately.
> This is a test assumption error.
>
> Fix: Change assertion from .loading-spinner to .confirmation (the actual final state)"

#### Step 5: Claude Applies Fix

```bash
$ git checkout -b fix/checkout-timeout
$ # Edit test/checkout.test.ts
$ git add test/checkout.test.ts
$ git commit -m "fix: Update assertion from spinner to confirmation

Timeline showed spinner removed at T+1210ms, test checked at T+5000ms."
```

#### Step 6: Claude Verifies

Spawn subagent to run test 10 times:

```bash
$ for i in {1..10}; do npm test -- --testNamePattern="checkout"; done

Results: 10/10 PASS ✅
```

#### Step 7: Claude Reports

```bash
$ gh pr comment <PR> -b "✅ Gasoline Auto-Fix: VERIFIED

Root Cause: Test assumption. Spinner removed at T+1210ms, test checked T+5000ms.
Fix: Changed assertion to final state (.confirmation).
Verification: 10 runs, 100% pass rate.

Ready to merge."
```

**Total time: 1-2 minutes. Done.**

---

## Integration Points

### 1. Test Runner

CI runs tests. When they fail, Gasoline is already capturing:

```bash
$ npm test:e2e
# Test fails
# Gasoline has captured everything
# Claude reads via MCP
```

No special integration needed. Gasoline runs in background, always capturing.

### 2. MCP Endpoints

Claude calls:

```javascript
// Read timeline
observe({ what: 'timeline' })

// Read network
observe({ what: 'network_bodies', url: '/checkout' })

// Read logs
observe({ what: 'logs' })

// Read DOM snapshots
observe({ what: 'page' })
```

That's it. Gasoline returns raw captured data. Claude interprets it.

### 3. Git Operations

Claude uses standard git commands (already supported):

```bash
# Via Bash tool
git checkout -b fix/...
git add .
git commit

# Or via interact() tool if we add git support
```

### 4. GitHub API

Claude posts results:

```bash
# Via Bash tool
gh pr comment <PR> -b "..."

# Or via Python github library if more complex
```

---

## Performance Budgets

### Time

| Phase | Target | Notes |
|-------|--------|-------|
| Data capture | Real-time | Passive, 0.1ms extension overhead |
| Timeline correlation | < 50ms | Just sorting events |
| Claude diagnosis | 30-60s | Depends on model |
| Fix application | 10-20s | git operations |
| Verification (10 runs) | 30-120s | Depends on test suite duration |
| PR comment | 5-10s | GitHub API |
| **Total** | **1-3 minutes** | Start to finish |

### Memory

| Buffer | Max | Rationale |
|--------|-----|-----------|
| Timeline | 10,000 events (~1MB) | Enough for 10-minute test |
| Network | 100 requests (8MB) | Typical test makes <50 requests |
| Console | 1,000 entries (~100KB) | Typical test logs <100 entries |
| DOM snapshots | 10 snapshots (~10MB) | Captured on demand |
| **Total** | ~19MB | Bounded, no growth |

### Accuracy

| Metric | Target | Notes |
|--------|--------|-------|
| Timeline correctness | 100% | Just sorting, deterministic |
| Root cause diagnosis | 85%+ | Claude's reasoning accuracy |
| Fix success rate | 80%+ | Fix actually resolves issue |

---

## Failure Modes

### Fix Doesn't Work (Success Rate < 50%)

```
Claude's logic:
1. Apply fix
2. Verify: 3/10 tests pass
3. Success rate 30% < target
4. Revert: git reset --hard
5. Report: "Fix didn't work, needs manual review"
```

Claude abandons the fix and escalates.

### Ambiguous Root Cause

```
Claude's logic:
"Timeline shows two possible issues:
1. Test assumption (80% likely)
2. Code timing bug (20% likely)

Try fix for most likely first.
If it fails, escalate to human."
```

Claude picks the best guess and tries it.

### Network Issue (Not Code/Test)

```
Claude reads timeline:
- POST sent at T+245ms
- Response at T+5200ms (4955ms delay!)
- This is timeout, not code/test issue

Report: "Looks like network/infrastructure issue, not code/test.
API endpoint is slow. Recommend investigating backend performance."
```

Claude recognizes it's not a code/test issue and doesn't apply a fix.

---

## What This Requires (Implementation)

### Gasoline (Go Server) — ~1000 lines

- [x] Browser extension (already exists)
- [x] Ring buffer storage (already exists)
- [ ] Timeline correlator (NEW — 100 lines to sort events)
- [ ] MCP `observe({what: 'timeline'})` endpoint (NEW — 50 lines)
- [ ] Ring buffer management for bounded memory (mostly exists, refine)

### Claude (System Prompt)

- [ ] Examples of reading timeline data
- [ ] Examples of diagnosing common failure patterns
- [ ] Guidance on when to apply fixes vs escalate

### Subagent (Verification Script)

- [ ] Simple bash script to run test N times
- [ ] Collect pass/fail results
- [ ] Return summary to Claude

### Integration

- [ ] CI hook that triggers Claude on test failure (already in CI config)
- [ ] GitHub token for PR comments (already available in CI)

---

## Why This Design Is Better

| Aspect | Old Design (Analysis Engine in Gasoline) | New Design (Claude Does Reasoning) |
|--------|----------------------------------------|-----------------------------------|
| **Code in Gasoline** | ~2000 lines | ~1000 lines |
| **Code in Claude** | ~0 lines | ~0 (just system prompt) |
| **Coupling** | Gasoline → Claude (tight) | Gasoline → Claude (loose) |
| **Testability** | Hard (complex state machine) | Easy (stateless) |
| **Flexibility** | Rigid (Gasoline decides strategy) | Flexible (Claude adapts) |
| **Development speed** | Slower (build + test analysis engine) | Faster (just correlation) |

---

## Next Steps

1. Implement timeline correlator in Gasoline (100 lines)
2. Add `observe({what: 'timeline'})` MCP endpoint (50 lines)
3. Write Claude system prompt with examples (200 lines)
4. Test with 1-2 manual scenarios
5. Ship as part of Wave 1

---

## References

- [Gasoline Architecture](./architecture.md)
- [v6.0 Roadmap](../roadmap.md)
- [Self-Healing Tests Feature Spec](../features/feature/33-self-healing-tests)
