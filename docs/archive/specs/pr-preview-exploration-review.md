# PR Preview Exploration (Feature 35) — Engineering Review

**Reviewer:** Principal Engineer
**Date:** 2026-01-26
**Spec:** docs/ai-first/tech-spec-agentic-cicd.md (lines 150-179)
**Status:** Pre-implementation review

---

## Executive Summary

Feature 35 proposes an autonomous agent workflow where AI navigates to preview deployments, explores the app, and reports/fixes bugs. The spec is architecturally sound in positioning this as a Claude Code skill (not a Gasoline MCP tool), correctly leveraging existing primitives. However, the spec lacks critical details on **exploration strategy bounds**, **resource lifecycle management**, and **failure mode handling**. The heavy reliance on `execute_javascript` for interaction creates a significant security surface that requires explicit guardrails. Implementation should proceed only after addressing the concurrency and resource exhaustion risks identified below.

---

## 1. Critical Issues (Must Fix Before Implementation)

### 1.1 Missing Exploration Bounds — Resource Exhaustion Risk

**Location:** Workflow step 4 ("Agent explores: clicks buttons, submits forms, navigates pages")

**Problem:** The spec provides no limits on exploration scope:
- No maximum page count
- No maximum action count per page
- No time budget per PR
- No depth limit for navigation

**Impact:** An autonomous agent exploring a complex app could:
- Generate unbounded network traffic (captured in `networkBodies` ring buffer, max 100 entries)
- Accumulate console errors filling the 1000-entry log buffer
- Spawn infinite verification sessions (current limit: 3 concurrent)
- Run for hours consuming CI minutes

**Recommendation:** Add explicit bounds to the skill definition:

```yaml
exploration_limits:
  max_pages: 20
  max_actions_per_page: 10
  max_total_actions: 100
  timeout_minutes: 15
  max_depth: 3  # clicks from landing page
```

### 1.2 execute_javascript Security Surface — No Script Validation

**Location:** Workflow step 4, Gasoline tools used (line 175)

**Problem:** The spec lists `execute_javascript` for page interaction but provides no guidance on:
- What scripts are acceptable
- What data the agent may extract
- How to prevent credential/PII exfiltration

**Current Implementation Risk:** From `pilot.go:240-288`, scripts are executed verbatim with no validation:

```go
queryParams := map[string]interface{}{
    "script":     params.Script,  // No sanitization
    "timeout_ms": timeoutMs,
}
```

An AI-generated script could theoretically:
- Read `localStorage`/`sessionStorage` auth tokens
- Extract form field values (including passwords)
- Make fetch requests with captured credentials

**Recommendation:**
1. Define an allowlist of safe script patterns for exploration (click, type, scroll)
2. Implement script pattern validation in the skill, NOT in Gasoline (preserve capture-vs-interpret boundary)
3. Consider a dedicated `interact` tool with constrained actions instead of raw JS execution

### 1.3 No Webhook Security Model

**Location:** Workflow step 2 ("Agent receives webhook notification")

**Problem:** The spec assumes webhook delivery without addressing:
- How webhooks are authenticated
- Who can trigger agent exploration
- How to prevent DoS via fake webhooks

**Impact:** Malicious actors could:
- Trigger exploration of arbitrary URLs (potential SSRF vector)
- Exhaust CI resources with fake PR events
- Trigger exploration of competitor sites

**Recommendation:** Add webhook security requirements:
- Signature verification (GitHub's `X-Hub-Signature-256`)
- URL allowlist (only `*.example.com` preview domains)
- Rate limiting (max 5 explorations per hour)

---

## 2. Concurrency & Data Race Analysis

### 2.1 Tab Targeting Race Condition

**Location:** Workflow step 3 ("Agent opens preview URL in browser via tab targeting")

**Problem:** The `browser_action {action: "open"}` creates a new tab, returning a `tab_id`. Subsequent tools must target this tab. However:

1. Tab creation is async (extension creates tab, polls result)
2. No guarantee the tab is fully loaded before next tool call
3. Multiple concurrent PR explorations could target wrong tabs

**Current Implementation:** From `pilot.go:334-338`:
```go
queryID := h.capture.CreatePendingQueryWithTimeout(PendingQuery{
    Type:   "browser_action",
    Params: queryParamsJSON,
    TabID:  params.TabID,  // Not set for "open" action
}, 10*time.Second)
```

**Recommendation:**
1. `browser_action {action: "open"}` must return the new `tab_id` in the result
2. The skill must capture this ID and pass it to ALL subsequent tool calls
3. Add a "wait for load" mechanism (poll `observe {what: "page"}` until `readyState === "complete"`)

### 2.2 Verification Session Starvation

**Location:** Gasoline tools used (`verify_fix`)

**Problem:** From `verify.go:224-225`:
```go
if len(vm.sessions) >= maxVerificationSessions {
    return nil, fmt.Errorf("maximum concurrent verification sessions (%d) reached", maxVerificationSessions)
}
```

With `maxVerificationSessions = 3` and multiple PR explorations running, sessions could:
- Block each other waiting for verification slots
- Leave orphaned sessions if agent crashes

**Recommendation:**
1. Increase `maxVerificationSessions` or make it configurable per-client
2. Add session scoping by PR/client ID (already have `currentClientID` in `ToolHandler`)
3. Implement session lease/heartbeat to auto-expire abandoned sessions faster than 30 minutes

### 2.3 Buffer Overflow Under Load

**Problem:** Exploration generates high event volume. Current buffer limits from `architecture.md`:

| Buffer | Limit | Risk |
|--------|-------|------|
| Log entries | 1000 | Errors from early pages evicted before analysis |
| Network bodies | 100 | API responses lost if many endpoints |
| WebSocket events | 500 | WS-heavy apps could lose critical messages |

**Recommendation:**
1. Add `analyze {target: "changes", checkpoint: "exploration_start"}` at exploration start
2. Process errors incrementally per-page rather than at end
3. Consider increasing `maxNetworkPerSnapshot` (currently 100) for this use case

---

## 3. Error Handling & Recovery

### 3.1 Missing Error Propagation Strategy

**Location:** Workflow steps 5-8

**Problem:** The workflow assumes happy path. No guidance on:
- What if preview deployment is down?
- What if exploration timeout occurs mid-workflow?
- What if GitHub API rate-limits the PR comment?

**Current Tool Behavior:** From `pilot.go:82-88`:
```go
if err != nil {
    // Timeout - don't assume disabled, report accurately
    return JSONRPCResponse{
        Result: mcpErrorResponse("Timeout waiting for extension response..."),
    }
}
```

Errors are reported but not structured for retry decisions.

**Recommendation:** Define error taxonomy and recovery:

| Error Type | Example | Recovery |
|------------|---------|----------|
| Transient | Network timeout | Retry 3x with backoff |
| Permanent | 404 preview not found | Fail fast, comment on PR |
| Resource | Session limit reached | Queue and retry later |
| Security | Blocked domain | Abort, alert security team |

### 3.2 No Graceful Degradation for Extension Disconnection

**Problem:** All exploration tools depend on browser extension. If extension disconnects:
- All pending queries timeout after 10 seconds
- No mechanism to detect "extension gone" vs "slow response"

**From architecture.md:**
> On-demand queries: block MCP caller until result or timeout (10s)

**Recommendation:**
1. Add `observe {what: "extension_health"}` or use existing `/health` endpoint
2. Check extension connectivity before starting exploration
3. If extension disconnects mid-exploration, capture partial results

---

## 4. Data Contract Analysis

### 4.1 Reproduction Script Format Stability

**Location:** Gasoline tools used (`generate {format: "reproduction"}`)

**Problem:** The reproduction script format is not versioned. If format changes, historical PR comments become confusing.

**Recommendation:** Include format version in output:
```json
{
  "format_version": "1.0",
  "steps": [...]
}
```

### 4.2 Tool Response Schema Drift Risk

**Problem:** The skill depends on specific response shapes from:
- `observe {what: "errors"}` — array of `LogEntry`
- `analyze {target: "accessibility"}` — structured violation report
- `browser_action` — `{success: bool, tab_id?: number, error?: string}`

None of these have formal schemas. Changes to response format break the skill.

**Recommendation:**
1. Define JSON Schema for each tool response
2. Add response validation in skill preconditions
3. Consider MCP schema extension for typed responses

---

## 5. Performance Analysis

### 5.1 Hot Path: Repeated DOM Queries

**Problem:** Exploration likely involves many `query_dom` calls. Each:
1. Creates pending query (mutex lock on `capture.mu`)
2. Waits for extension poll (100ms default interval from architecture)
3. Extension executes querySelector
4. Returns result via HTTP POST

**Latency:** ~100-200ms per query minimum

**For 100 exploration actions:** 10-20 seconds just in DOM query overhead

**Recommendation:**
1. Batch DOM queries where possible (query multiple selectors at once)
2. Consider a "scan page" meta-tool that returns all interactive elements
3. Cache selector results if page hasn't navigated

### 5.2 Memory Allocation: Snapshot Deep Copies

**From verify.go and sessions.go:**
```go
// Deep copy performance snapshot if present
var perfCopy *PerformanceSnapshot
if perf != nil {
    p := *perf
    perfCopy = &p
}
```

This is shallow copy of the struct but:
- `PerformanceSnapshot` likely contains slices/maps
- Multiple snapshots per exploration = memory growth

**Recommendation:** Review `PerformanceSnapshot` for nested pointers and implement proper deep copy or use copy-on-write pattern.

---

## 6. Security Analysis

### 6.1 URL Validation Gap

**Location:** Workflow step 3 (`browser_action {action: "navigate", url: "..."}`)

**Problem:** From `pilot.go:316-322`:
```go
if (params.Action == "navigate" || params.Action == "open") && params.URL == "" {
    return JSONRPCResponse{
        Result: mcpErrorResponse("URL required for " + params.Action + " action"),
    }
}
```

Only checks URL is non-empty. No validation that URL:
- Is HTTPS
- Is on an allowed domain
- Isn't `file://`, `javascript:`, or `data:` scheme

**Impact:** Agent could be tricked into navigating to:
- `file:///etc/passwd` (information disclosure)
- `javascript:alert(document.cookie)` (XSS in extension context)
- Phishing sites mimicking preview deployments

**Recommendation:**
1. Add URL scheme allowlist (`https://` only)
2. Add domain allowlist in skill configuration
3. Validate URL format before sending to extension

### 6.2 No Audit Trail for Autonomous Actions

**Problem:** The skill will perform actions (click, type, navigate) without human observation. If something goes wrong:
- No record of what the agent did
- No way to reproduce the exploration path
- No accountability trail

**Existing Infrastructure:** `auditTrail` in `tools.go` records tool invocations but:
- Only records tool name and params
- Doesn't capture the AI's reasoning
- Doesn't link actions to specific PR

**Recommendation:**
1. Add exploration session ID to all tool calls
2. Generate exploration report with full action sequence
3. Store reports persistently (not just in-memory)

---

## 7. Maintainability & Extensibility

### 7.1 Skill vs Tool Boundary Unclear

**Problem:** The spec says "Implementation lives in Claude Code skills" but doesn't clarify:
- What logic goes in the skill YAML?
- What logic requires code changes?
- How are skills versioned and deployed?

**Recommendation:** Add explicit boundary definition:
- **Skill responsibility:** Workflow orchestration, prompts, limits
- **Gasoline responsibility:** Data capture, tool execution, state management
- **CI responsibility:** Webhook handling, secret management, permissions

### 7.2 Testing Surface

**Problem:** How do you test an autonomous exploration skill?

**Recommendation:**
1. Define deterministic test fixtures (mock preview site)
2. Add "dry run" mode that logs actions without executing
3. Create regression test suite with known-bug preview sites

---

## 8. Implementation Roadmap

### Phase 1: Foundation (Week 1-2)
1. **Add URL validation to browser_action** — Security critical
2. **Implement exploration bounds in skill template** — Resource protection
3. **Add webhook signature verification** — Security critical
4. **Create exploration session tracking** — Audit trail

### Phase 2: Core Implementation (Week 3-4)
5. **Implement skill YAML structure** — Define workflow steps
6. **Add tab lifecycle management** — Wait for load, track tab_id
7. **Implement incremental error processing** — Per-page analysis
8. **Add partial result capture on failure** — Graceful degradation

### Phase 3: Hardening (Week 5-6)
9. **Implement script pattern validation** — execute_javascript guardrails
10. **Add extension health checking** — Pre-flight connectivity test
11. **Create exploration replay/reproduction** — Debugging support
12. **Performance optimization** — Batch queries, caching

### Phase 4: Production Readiness (Week 7-8)
13. **Integration testing with real preview deployments**
14. **Load testing with concurrent explorations**
15. **Security audit of full workflow**
16. **Documentation and runbook creation**

---

## Summary of Recommendations

| Priority | Issue | Recommendation |
|----------|-------|----------------|
| **P0** | No exploration bounds | Add max_pages, max_actions, timeout limits |
| **P0** | execute_javascript unvalidated | Define safe script patterns, validate in skill |
| **P0** | URL not validated | Add scheme and domain allowlist |
| **P1** | Tab race condition | Return tab_id from open, wait for load |
| **P1** | Webhook unauthenticated | Require signature verification |
| **P1** | No audit trail | Add exploration session tracking |
| **P2** | Verification session starvation | Scope sessions by client ID |
| **P2** | Buffer overflow under load | Process errors incrementally |
| **P2** | No error taxonomy | Define error types and recovery strategies |
| **P3** | DOM query latency | Batch queries, add page scan tool |
| **P3** | Tool response schema drift | Define JSON schemas |

---

## Appendix: Relevant Code References

| File | Lines | Relevance |
|------|-------|-----------|
| `cmd/dev-console/pilot.go` | 1-386 | browser_action, execute_javascript implementation |
| `cmd/dev-console/verify.go` | 1-841 | Verification loop, session limits |
| `cmd/dev-console/sessions.go` | 1-686 | Session snapshots, diff logic |
| `cmd/dev-console/tools.go` | 1-1755 | Tool dispatch, buffer access patterns |
| `.claude/docs/architecture.md` | 1-132 | Memory limits, security model |
