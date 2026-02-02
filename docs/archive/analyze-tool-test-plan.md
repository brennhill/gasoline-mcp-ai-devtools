# Analyze Tool - UAT Test Plan (v7.0)

**Status**: PROPOSED (not yet implemented)
**Target Version**: v7.0
**Based On**: [docs/features/feature/analyze-tool/tech-spec.md](docs/features/feature/analyze-tool/tech-spec.md)

---

## Overview

This test plan covers the **future standalone `analyze` tool** which is currently proposed for v7.0. As of v5.2.0, analysis functionality is implemented within the `observe` tool as modes like `api`, `performance`, `accessibility`, `error_clusters`, etc.

The v7.0 spec proposes re-separating this functionality into a dedicated `analyze` MCP tool for better organization and clarity.

---

## Current State (v5.2.0)

**Analysis features are currently WITHIN observe tool:**

| Analyze Feature | Current Implementation | Status |
|-----------------|----------------------|--------|
| API Schema | `observe({what: "api"})` | ✅ Working |
| Performance | `observe({what: "performance"})` | ✅ Working |
| Accessibility | `observe({what: "accessibility"})` | ❌ BROKEN (runtime error) |
| Error Clusters | `observe({what: "error_clusters"})` | ✅ Working |
| History/Patterns | `observe({what: "history"})` | ✅ Working |
| Security Audit | `observe({what: "security_audit"})` | ✅ Working |
| Third-Party Audit | `observe({what: "third_party_audit"})` | ✅ Working |

**Architecture Decision**: claude.md claims 5 tools (including analyze), but actual implementation has 4 tools with analysis as observe modes.

---

## Proposed v7.0 Analyze Tool

**Tool Signature**:
```javascript
analyze({
  action: "audit" | "memory" | "security" | "regression",
  scope: "accessibility" | "performance" | ...,
  selector: "CSS selector",
  tab_id: number,
  force_refresh: boolean
})
```

**Key Design Principles** (from tech-spec.md):
1. **Async Pattern**: Long operations use correlation_id for polling
2. **Blocking Pattern**: Quick operations return immediately
3. **Scope Parameter**: Narrows the action (e.g., "accessibility", "performance")
4. **Extension Integration**: Runs analysis in browser context (axe-core, etc.)

---

## Pre-Test Requirements

Before testing the analyze tool (when implemented), ensure:

### 1. Extension Prerequisites
- [ ] AI Web Pilot toggle enabled in extension popup
- [ ] Extension connected to server (`configure({action: "health"})` shows `extension_connected: true`)
- [ ] axe-core bundled in `extension/vendor/axe.min.js` (527KB)

### 2. Server Prerequisites
- [ ] Gasoline server running v7.0+
- [ ] `analyze` tool appears in `tools/list` MCP call
- [ ] Pending query system supports async analyze operations

### 3. Code Prerequisites
- [ ] `extension/lib/analyze.js` - Main dispatcher exists
- [ ] `extension/lib/analyze-audit.js` - Audit implementations exist
- [ ] `extension/lib/analyze-memory.js` - Memory analysis exists
- [ ] `extension/lib/analyze-security.js` - Security checks exist
- [ ] `extension/lib/analyze-regression.js` - Regression baseline/compare exists

---

## Test Categories

### Category A: Action = "audit"

#### A1: Accessibility Audit (Async)
**Call**:
```javascript
analyze({action: "audit", scope: "accessibility"})
```

**Expected Behavior**:
1. Returns immediately with `{correlation_id: "...", status: "pending"}`
2. Extension polls `/pending-queries`, picks up analyze request
3. Extension injects axe-core, runs audit
4. Extension POSTs result to `/analyze-result`
5. AI polls `observe({what: "analyze_result", correlation_id: "..."})` until complete

**Expected Result**:
- Status: "complete"
- Duration: 2-10s
- Format: WCAG violations array with impact, description, nodes

**Test Cases**:
- [ ] Basic call returns correlation_id immediately
- [ ] Poll `analyze_result` shows "pending" initially
- [ ] Poll `analyze_result` shows "complete" after 2-10s
- [ ] Result contains accessibility violations array
- [ ] Violations have required fields: impact, id, description, nodes
- [ ] Timeout after 15s if extension doesn't respond

#### A2: Accessibility Audit with Scope (Async)
**Call**:
```javascript
analyze({action: "audit", scope: "accessibility", selector: ".product-grid"})
```

**Expected**:
- Runs audit only on `.product-grid` subtree
- Faster than full-page audit (1-3s)

**Test Cases**:
- [ ] Scoped audit returns fewer violations than full-page
- [ ] All violations reference elements within selector
- [ ] Invalid selector returns error

#### A3: Accessibility Audit with Force Refresh
**Call**:
```javascript
analyze({action: "audit", scope: "accessibility", force_refresh: true})
```

**Expected**:
- Bypasses any cached audit results
- Always re-runs axe-core

**Test Cases**:
- [ ] First call with `force_refresh: false` caches result
- [ ] Second call without `force_refresh` returns cached result instantly
- [ ] Third call with `force_refresh: true` re-runs audit (takes 2-10s again)

#### A4: Accessibility Audit with Tags
**Call**:
```javascript
analyze({action: "audit", scope: "accessibility", tags: ["wcag2a", "wcag2aa"]})
```

**Expected**:
- Only runs WCAG 2.0 Level A and AA tests
- Skips WCAG 2.1, Section 508, etc.

**Test Cases**:
- [ ] Result only contains WCAG 2.0 A/AA violations
- [ ] Invalid tag returns error

#### A5: Performance Audit (Async)
**Call**:
```javascript
analyze({action: "audit", scope: "performance"})
```

**Expected**:
- Returns correlation_id
- Takes 5-15s to complete
- Returns Lighthouse-style performance metrics

**Test Cases**:
- [ ] Returns FCP, LCP, CLS, INP, TTI metrics
- [ ] Each metric has value and assessment (good/needs-improvement/poor)
- [ ] Includes recommendations array
- [ ] Timeout after 20s

#### A6: Full Audit (Async)
**Call**:
```javascript
analyze({action: "audit", scope: "full"})
```

**Expected**:
- Runs both accessibility AND performance audits
- Takes 10-30s to complete
- Returns combined results

**Test Cases**:
- [ ] Result contains both `accessibility` and `performance` sections
- [ ] Timeout after 45s
- [ ] Shows progress updates during execution

---

### Category B: Action = "memory"

#### B1: Memory Snapshot (Blocking)
**Call**:
```javascript
analyze({action: "memory", scope: "snapshot"})
```

**Expected**:
- Returns immediately (blocking pattern)
- Takes <2s
- Returns heap snapshot summary

**Test Cases**:
- [ ] Returns within 5s (blocking timeout)
- [ ] Result contains `{total_bytes, used_bytes, detached_nodes, listeners}`
- [ ] No correlation_id (blocking pattern)

#### B2: Memory Compare (Blocking)
**Call**:
```javascript
analyze({action: "memory", scope: "compare", baseline_id: "snap1"})
```

**Expected**:
- Compares current memory to previous snapshot
- Returns diff immediately (<2s)

**Test Cases**:
- [ ] First call with `scope: "snapshot"` saves baseline
- [ ] Trigger memory leak (add event listeners, create objects)
- [ ] Second call with `scope: "compare"` shows increased memory
- [ ] Result shows `{delta_bytes, new_detached_nodes, leaked_listeners}`
- [ ] Invalid baseline_id returns error

---

### Category C: Action = "security"

#### C1: Security Scan (Blocking)
**Call**:
```javascript
analyze({action: "security", scope: "credentials"})
```

**Expected**:
- Scans for exposed credentials in network traffic, logs, localStorage
- Returns <3s (blocking)

**Test Cases**:
- [ ] Detects API keys in network responses
- [ ] Detects credentials in console.log statements
- [ ] Detects JWT tokens in localStorage
- [ ] Returns `{findings: [...], severity: "critical|high|medium|low"}`

#### C2: Security Scan - PII
**Call**:
```javascript
analyze({action: "security", scope: "pii"})
```

**Expected**:
- Scans for PII (email, phone, SSN, credit card) in logs/network

**Test Cases**:
- [ ] Detects email addresses
- [ ] Detects phone numbers
- [ ] Detects credit card numbers
- [ ] Detects SSNs
- [ ] Redacts values in findings

#### C3: Security Scan - Headers
**Call**:
```javascript
analyze({action: "security", scope: "headers"})
```

**Expected**:
- Checks for missing security headers (CSP, HSTS, X-Frame-Options, etc.)

**Test Cases**:
- [ ] Detects missing Content-Security-Policy
- [ ] Detects missing Strict-Transport-Security
- [ ] Detects missing X-Frame-Options
- [ ] Recommends header values

---

### Category D: Action = "regression"

#### D1: Regression Baseline (Async)
**Call**:
```javascript
analyze({action: "regression", scope: "baseline"})
```

**Expected**:
- Captures current state as baseline
- Takes 5-15s
- Saves performance, accessibility, console errors for comparison

**Test Cases**:
- [ ] Returns correlation_id
- [ ] Baseline saved successfully
- [ ] `observe({what: "analyze_result", correlation_id: "..."})` shows saved baseline ID

#### D2: Regression Compare (Async)
**Call**:
```javascript
analyze({action: "regression", scope: "compare", baseline_id: "baseline_123"})
```

**Expected**:
- Compares current state to baseline
- Shows regressions (new errors, slower performance, new a11y violations)

**Test Cases**:
- [ ] First call saves baseline
- [ ] Introduce regression (add console.error, slow down page)
- [ ] Second call detects regression
- [ ] Result shows `{new_errors: [...], performance_delta: {...}, new_violations: [...]}`
- [ ] Invalid baseline_id returns error

---

## Error Handling

### E1: AI Web Pilot Disabled
**Call**:
```javascript
analyze({action: "audit", scope: "accessibility"})
```

**When**: Pilot toggle disabled in extension popup

**Expected**:
```json
{
  "error": "ai_web_pilot_disabled",
  "message": "Enable AI Web Pilot in the Gasoline extension popup",
  "retry": "Click the extension icon and toggle AI Web Pilot on"
}
```

**Test Cases**:
- [ ] Disable pilot in extension popup
- [ ] Call analyze
- [ ] Error message clear and actionable
- [ ] Calling `observe({what: "pilot"})` first would prevent this

### E2: Axe-Core Injection Failed
**Call**:
```javascript
analyze({action: "audit", scope: "accessibility"})
```

**When**: CSP blocks axe-core injection

**Expected**:
```json
{
  "error": "axe_injection_failed",
  "message": "Could not inject axe-core (CSP or extension permissions)",
  "retry": "Check CSP headers or extension permissions"
}
```

**Test Cases**:
- [ ] Navigate to site with strict CSP
- [ ] Call analyze
- [ ] Error message explains CSP issue

### E3: Timeout
**Call**:
```javascript
analyze({action: "audit", scope: "full"})
```

**When**: Audit takes >45s

**Expected**:
```json
{
  "error": "timeout",
  "message": "Audit timed out after 45s",
  "retry": "Page may be too complex. Try scoping to specific element."
}
```

**Test Cases**:
- [ ] Navigate to very large page (1000+ elements)
- [ ] Call analyze with full scope
- [ ] Timeout after 45s
- [ ] Partial results returned if available

### E4: Invalid Action
**Call**:
```javascript
analyze({action: "invalid"})
```

**Expected**:
```json
{
  "error": "invalid_param",
  "message": "Invalid action 'invalid'. Use: audit, memory, security, regression"
}
```

**Test Cases**:
- [ ] Invalid action returns error
- [ ] Error lists valid actions

### E5: Invalid Scope
**Call**:
```javascript
analyze({action: "audit", scope: "invalid"})
```

**Expected**:
```json
{
  "error": "invalid_param",
  "message": "Invalid scope 'invalid' for action 'audit'. Use: accessibility, performance, full"
}
```

**Test Cases**:
- [ ] Invalid scope for audit returns error
- [ ] Error lists valid scopes for the action

---

## Integration Tests

### I1: Analyze → Generate → Export
**Flow**:
1. `analyze({action: "audit", scope: "accessibility"})`
2. Wait for completion
3. `generate({format: "sarif"})`
4. Save SARIF to file

**Expected**:
- SARIF file contains all accessibility violations
- File is valid SARIF 2.1.0 format

**Test Cases**:
- [ ] SARIF validates against schema
- [ ] All violations have required fields
- [ ] File can be imported into GitHub Security tab

### I2: Baseline → Regression Detection → Alert
**Flow**:
1. `analyze({action: "regression", scope: "baseline"})`
2. Deploy new code with bug
3. `analyze({action: "regression", scope: "compare"})`
4. Detect new console errors

**Expected**:
- Regression detected automatically
- Alert sent via streaming notifications (if enabled)

**Test Cases**:
- [ ] Baseline captures clean state
- [ ] Regression compare detects new errors
- [ ] Streaming notification sent if configured

### I3: Memory Leak Detection
**Flow**:
1. `analyze({action: "memory", scope: "snapshot"})` → baseline
2. Navigate through app (add items, click around)
3. `analyze({action: "memory", scope: "compare", baseline_id: "..."})`
4. Detect memory growth

**Expected**:
- Memory increase detected
- Detached nodes identified
- Event listeners leak identified

**Test Cases**:
- [ ] Clean navigation shows minimal delta
- [ ] Memory leak shows significant delta
- [ ] Recommendations provided

---

## Performance Requirements

| Action | Scope | Expected Duration | Timeout |
|--------|-------|-------------------|---------|
| audit | accessibility | 2-10s | 15s |
| audit | performance | 5-15s | 20s |
| audit | full | 10-30s | 45s |
| memory | snapshot | <2s | 5s |
| memory | compare | <2s | 5s |
| security | credentials | <3s | 5s |
| security | pii | <3s | 5s |
| security | headers | <3s | 5s |
| regression | baseline | 5-15s | 20s |
| regression | compare | 5-15s | 20s |

---

## Comparison: Current (v5.2.0) vs. Proposed (v7.0)

| Feature | Current (v5.2.0) | Proposed (v7.0) |
|---------|------------------|-----------------|
| API Schema | `observe({what: "api"})` | `analyze({action: "audit", scope: "api"})` |
| Performance | `observe({what: "performance"})` | `analyze({action: "audit", scope: "performance"})` |
| Accessibility | `observe({what: "accessibility"})` | `analyze({action: "audit", scope: "accessibility"})` |
| Error Clusters | `observe({what: "error_clusters"})` | `analyze({action: "regression", scope: "compare"})` |
| Security | `observe({what: "security_audit"})` | `analyze({action: "security", scope: "credentials"})` |
| Memory | ❌ Not available | `analyze({action: "memory", scope: "snapshot"})` ✨ NEW |
| Regression | ❌ Not available | `analyze({action: "regression", scope: "baseline"})` ✨ NEW |

**Benefits of v7.0 Separation**:
1. ✅ Clearer API (analyze vs. observe distinction)
2. ✅ Better organization (analysis actions grouped)
3. ✅ New memory analysis features
4. ✅ Regression testing capabilities
5. ✅ Async pattern for long operations

**Drawbacks**:
1. ❌ Breaking change (migration required)
2. ❌ More complex (2 tools for related data: observe + analyze)
3. ❌ Requires significant refactoring

---

## Migration Testing

When migrating from v5.2.0 to v7.0:

### M1: Backward Compatibility
**Test**: Old `observe({what: "accessibility"})` calls

**Expected Options**:
- Option A: Return deprecation warning + redirect to analyze
- Option B: Fail with migration instructions
- Option C: Work indefinitely (no breaking change)

**Recommended**: Option A (deprecate but support)

**Test Cases**:
- [ ] `observe({what: "api"})` returns deprecation warning
- [ ] Warning includes migration command: `analyze({action: "audit", scope: "api"})`
- [ ] Old command still works (backward compat)

### M2: Documentation Migration
**Test**: All docs updated

**Test Cases**:
- [ ] UAT-TEST-PLAN-V2.md updated with analyze tool tests
- [ ] claude.md lists 5 tools (observe, generate, configure, interact, analyze)
- [ ] MCP tool descriptions show analyze tool
- [ ] examples/ folder has analyze examples

---

## Test Execution Order

When v7.0 is implemented, run tests in this order:

1. **Pre-Tests** (verify prerequisites)
   - Extension connected
   - Pilot enabled
   - axe-core bundled

2. **Basic Tests** (simple happy paths)
   - A1: Accessibility audit
   - B1: Memory snapshot
   - C1: Security scan (credentials)

3. **Advanced Tests** (scoped, filtered, parameterized)
   - A2-A6: All audit variations
   - B2: Memory compare
   - C2-C3: All security scopes

4. **Error Tests** (E1-E5)
   - Pilot disabled
   - Timeout
   - Invalid params

5. **Integration Tests** (I1-I3)
   - Analyze → Generate
   - Regression detection
   - Memory leak detection

6. **Performance Tests** (verify duration requirements)
   - All actions complete within timeout
   - Async pattern works correctly

7. **Migration Tests** (M1-M2)
   - Old observe calls still work
   - Deprecation warnings shown

---

## Sign-Off Criteria

Before approving v7.0 analyze tool for production:

- [ ] **All basic tests pass** (A1, B1, C1)
- [ ] **All advanced tests pass** (A2-A6, B2, C2-C3)
- [ ] **All error tests pass** (E1-E5)
- [ ] **All integration tests pass** (I1-I3)
- [ ] **All performance requirements met** (within timeout)
- [ ] **Migration plan tested** (M1-M2)
- [ ] **Accessibility audit fixed** (BUG #1 from v5.2.0 UAT)
- [ ] **Parameter validation fixed** (BUG #2 from v5.2.0 UAT)
- [ ] **Documentation complete** (all 5 tools documented)
- [ ] **No regressions** (existing observe/generate/configure/interact still work)

---

## Notes for Future Implementation

1. **Do NOT implement v7.0 analyze tool until**:
   - BUG #1 (accessibility runtime error) is fixed in v5.2.0
   - BUG #2 (parameter validation) is fixed in v5.2.0
   - User confirms they want the 5-tool architecture

2. **Consider alternatives**:
   - Keep analysis within observe tool (current state)
   - Only separate if there's user demand for clearer API

3. **If implementing**:
   - Follow [docs/features/feature/analyze-tool/tech-spec.md](docs/features/feature/analyze-tool/tech-spec.md) exactly
   - Use async pattern for operations >2s
   - Use blocking pattern for operations <2s
   - Add comprehensive error handling
   - Test with multiple browser tabs (user has 43 open!)

---

**Test Plan Created**: 2026-01-30
**For Version**: v7.0 (proposed)
**Current Version**: v5.2.0
**Total Test Scenarios**: 40+
**Estimated Test Duration**: 4-6 hours (when implemented)

---

_This is a **future test plan** for a proposed feature. The analyze tool does NOT exist in v5.2.0._
