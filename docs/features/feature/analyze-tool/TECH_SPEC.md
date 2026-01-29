---
feature: analyze-tool
status: proposed
version: v7.0-rev1
---

# `analyze` Tool — Technical Specification

## Architecture Overview

The `analyze` tool follows Gasoline's existing architecture pattern:

```
AI (MCP Client)
    │
    ▼
┌─────────────────┐
│   Go Server     │  ← Receives analyze requests via MCP
│  (cmd/dev-console)
└────────┬────────┘
         │ HTTP (localhost only)
         ▼
┌─────────────────┐
│ Chrome Extension│  ← Runs analysis tools, returns results
│   (content.js)  │
└─────────────────┘
         │
         ▼
┌─────────────────┐
│  Analysis Libs  │  ← axe-core (bundled), custom analyzers
└─────────────────┘
```

## Key Components

### 1. MCP Tool Registration (Go Server)

New tool registered alongside existing 4 tools:

```
Tool: analyze
Description: Run analysis tools and return structured findings
Parameters:
  - action (string, required): audit|memory|security|regression
  - scope (string, optional): Narrows the action (e.g., "accessibility", "performance")
  - selector (string, optional): CSS selector to scope analysis to element
  - tab_id (integer, optional): Target tab (0 = active)
  - force_refresh (boolean, optional): Bypass cache, default false
```

**Excluded by design:** `render` and `bundle` modes — developers have better native tools (React DevTools, webpack-bundle-analyzer). Gasoline focuses on runtime analysis AI can't access otherwise.

### 2. Request Flow & Concurrency Model

**[CRITICAL RESOLUTION C1-1]** Pattern selection based on expected duration:

| Action | Expected Duration | Pattern | Timeout |
|--------|-------------------|---------|---------|
| `audit.accessibility` | 2-10s | Async (correlation_id) | 15s |
| `audit.performance` | 5-15s | Async (correlation_id) | 20s |
| `audit.full` | 10-30s | Async (correlation_id) | 45s |
| `memory.snapshot` | <2s | Blocking (WaitForResult) | 5s |
| `memory.compare` | <2s | Blocking (WaitForResult) | 5s |
| `security.*` | <3s | Blocking (WaitForResult) | 5s |
| `regression.baseline` | 5-15s | Async (correlation_id) | 20s |
| `regression.compare` | 5-15s | Async (correlation_id) | 20s |

**Async pattern (long operations):**
1. AI calls `analyze({action: 'audit', scope: 'accessibility'})`
2. Server creates pending query with correlation_id, returns immediately
3. Extension polls `/pending-queries`, picks up analyze request
4. Extension runs analysis (axe-core, etc.)
5. Extension POSTs result to `/analyze-result`
6. AI polls `observe({what: 'analyze_result', correlation_id: '...'})` or receives via streaming

**Blocking pattern (quick operations):**
1. AI calls `analyze({action: 'memory', scope: 'snapshot'})`
2. Server creates pending query, waits for result (WaitForResult)
3. Extension polls, executes, POSTs result
4. Server returns result directly to AI (no polling needed)

### 3. Extension Components

**[CRITICAL RESOLUTION M1-1]** File structure matching existing patterns:

```
extension/
├── lib/
│   ├── analyze.js           # Main dispatcher (like actions.js)
│   ├── analyze-audit.js     # Audit implementations (axe-core wrapper)
│   ├── analyze-memory.js    # Memory analysis
│   ├── analyze-security.js  # Security checks
│   └── analyze-regression.js # Regression baseline/compare
├── vendor/
│   └── axe.min.js           # Already bundled (527KB)
```

Each module exports a single async function following the pattern in `actions.js`.

#### Analysis Dispatcher (`extension/lib/analyze.js`)

```javascript
import { runAudit } from './analyze-audit.js';
import { runMemoryAnalysis } from './analyze-memory.js';
import { runSecurityAnalysis } from './analyze-security.js';
import { runRegressionAnalysis } from './analyze-regression.js';

export async function analyze(request) {
  switch(request.action) {
    case 'audit': return runAudit(request);
    case 'memory': return runMemoryAnalysis(request);
    case 'security': return runSecurityAnalysis(request);
    case 'regression': return runRegressionAnalysis(request);
    default: return { error: 'unknown_action', message: `Unknown action: ${request.action}` };
  }
}
```

### 4. axe-core Lazy Loading

**[CRITICAL RESOLUTION P1-1]** Loading state machine:

```
┌─────────────┐     loadAxe()      ┌─────────────┐
│   UNLOADED  │ ─────────────────► │   LOADING   │
└─────────────┘                    └──────┬──────┘
                                          │
                    ┌─────────────────────┴─────────────────────┐
                    │                                           │
                    ▼ success                                   ▼ failure
           ┌─────────────┐                             ┌─────────────┐
           │   LOADED    │                             │   FAILED    │
           └─────────────┘                             └─────────────┘
                    │                                           │
                    │ runAxe()                                  │ retry after 5s
                    ▼                                           │
              [execute audit]                                   │
                    │                                           │
                    └───────────────────────────────────────────┘
```

**Implementation (`extension/lib/analyze-audit.js`):**

```javascript
const AXE_STATE = {
  status: 'UNLOADED', // UNLOADED | LOADING | LOADED | FAILED
  instance: null,
  loadPromise: null,
  lastError: null
};

async function ensureAxeLoaded() {
  if (AXE_STATE.status === 'LOADED') return true;
  if (AXE_STATE.status === 'LOADING') return AXE_STATE.loadPromise;

  AXE_STATE.status = 'LOADING';
  AXE_STATE.loadPromise = new Promise(async (resolve, reject) => {
    try {
      // Inject axe.min.js into page context via content script
      await injectAxeScript();
      AXE_STATE.status = 'LOADED';
      resolve(true);
    } catch (err) {
      AXE_STATE.status = 'FAILED';
      AXE_STATE.lastError = err.message;
      reject(err);
    }
  });

  return AXE_STATE.loadPromise;
}

// Memory cleanup: no automatic unload (axe stays loaded for session)
// Rationale: Re-injection cost (~100ms) outweighs memory cost (~500KB)
```

### 5. CSP Fallback Strategy

**[CRITICAL RESOLUTION E1-1]** Two-tier injection approach:

**Tier 1: Content script injection (default)**
- Inject axe.min.js via `chrome.scripting.executeScript`
- Works for most pages without strict CSP

**Tier 2: Isolated world via chrome.debugger (fallback)**
- If Tier 1 fails with CSP error, attach debugger and run in isolated context
- Requires user to have DevTools closed (debugger limitation)

```javascript
async function runAccessibilityAudit(request) {
  try {
    // Tier 1: Try content script injection
    await ensureAxeLoaded();
    return await executeAxeInPage(request.selector);
  } catch (err) {
    if (isCSPError(err)) {
      // Tier 2: Fallback to isolated context
      return await executeAxeViaDebugger(request.selector, request.tab_id);
    }
    throw err;
  }
}

async function executeAxeViaDebugger(selector, tabId) {
  // Attach debugger if not already attached
  await chrome.debugger.attach({ tabId }, '1.3');

  try {
    // Execute in isolated world
    const result = await chrome.debugger.sendCommand(
      { tabId },
      'Runtime.evaluate',
      {
        expression: `axe.run(${selector ? `document.querySelector('${selector}')` : 'document'})`,
        awaitPromise: true,
        returnByValue: true
      }
    );
    return transformAxeResult(result);
  } finally {
    // Always detach
    await chrome.debugger.detach({ tabId });
  }
}
```

**Manifest permissions required:**
```json
{
  "permissions": ["debugger"]
}
```

### 6. Lighthouse Strategy

**[CRITICAL RESOLUTION P1-2]** Lighthouse deferred to Phase 2.

**Rationale:**
- chrome.debugger blocks DevTools (poor UX during active development)
- Full Lighthouse takes 20-40s (too slow for interactive use)
- Alternative: Use existing `observe({what: 'vitals'})` for Core Web Vitals

**Phase 1 performance audit uses:**
- `observe({what: 'vitals'})` — LCP, FID, CLS from Performance Observer
- `observe({what: 'performance'})` — Navigation timing, resource timing
- Custom analysis layer in `analyze-audit.js` that interprets these metrics

**Phase 2 (future):**
- Lighthouse integration via service worker context
- Runs in background, doesn't block DevTools
- Returns comprehensive report

### 7. Server Endpoints

New endpoints in `cmd/dev-console/`:

```
POST /analyze-result
  - Receives analysis results from extension
  - Correlates with pending query ID
  - Stores for AI retrieval
  - Body: { correlation_id, status, findings[], duration_ms }

GET /pending-queries
  - Extended to include analyze requests
  - Query type: "analyze"
  - Same polling mechanism as interact
```

## Response Schemas

**[CRITICAL RESOLUTION D1-1]** Complete JSON schemas for all response types.

### Base Response Structure

All analyze responses follow this structure (aligned with mcpJSONResponse):

```json
{
  "status": "success" | "error" | "partial",
  "action": "audit" | "memory" | "security" | "regression",
  "scope": "accessibility" | "performance" | "full" | ...,
  "duration_ms": 2450,
  "cached": false,
  "findings": [...],
  "summary": {...},
  "warnings": [],
  "error": null
}
```

### Error Response (aligned with mcpStructuredError)

```json
{
  "status": "error",
  "error": {
    "code": "axe_injection_failed" | "analysis_timeout" | "csp_blocked" | ...,
    "message": "Human-readable error message",
    "details": {
      "attempted_fallback": true,
      "fallback_error": "DevTools already open"
    }
  }
}
```

### Audit Response Schema

```json
{
  "status": "success",
  "action": "audit",
  "scope": "accessibility",
  "duration_ms": 3200,
  "cached": false,
  "findings": [
    {
      "id": "color-contrast",
      "severity": "critical" | "high" | "medium" | "low" | "info",
      "category": "accessibility" | "performance" | "seo" | "best_practices",
      "message": "Elements must have sufficient color contrast",
      "count": 3,
      "affected": [
        {
          "selector": "button.submit",
          "html": "<button class=\"submit\">Submit</button>",
          "fix": "Change text color to #1a1a1a or background to #ffffff"
        }
      ],
      "reference": {
        "wcag": "1.4.3",
        "level": "AA",
        "url": "https://www.w3.org/WAI/WCAG21/Understanding/contrast-minimum"
      }
    }
  ],
  "summary": {
    "critical": 1,
    "high": 3,
    "medium": 5,
    "low": 2,
    "info": 0,
    "passed": 42,
    "total_rules": 53
  }
}
```

### Memory Response Schema

```json
{
  "status": "success",
  "action": "memory",
  "scope": "snapshot",
  "duration_ms": 1200,
  "snapshot": {
    "timestamp": "2026-01-29T10:30:00Z",
    "heap_used_mb": 45.2,
    "heap_total_mb": 64.0,
    "heap_limit_mb": 2048.0,
    "detached_nodes": 127,
    "dom_node_count": 1523,
    "event_listener_count": 342
  }
}
```

```json
{
  "status": "success",
  "action": "memory",
  "scope": "compare",
  "duration_ms": 1800,
  "baseline": { "timestamp": "...", "heap_used_mb": 45.2 },
  "current": { "timestamp": "...", "heap_used_mb": 62.8 },
  "delta": {
    "heap_growth_mb": 17.6,
    "heap_growth_pct": 38.9,
    "detached_nodes_delta": 847,
    "dom_nodes_delta": 234
  },
  "findings": [
    {
      "severity": "high",
      "message": "Significant heap growth detected (38.9%)",
      "likely_cause": "Detached DOM nodes increased by 847",
      "guidance": "Check for event listeners not being removed on component unmount"
    }
  ]
}
```

### Security Response Schema

```json
{
  "status": "success",
  "action": "security",
  "scope": "full",
  "duration_ms": 890,
  "findings": [
    {
      "severity": "high",
      "category": "headers",
      "issue": "missing_csp",
      "message": "Content-Security-Policy header not set",
      "guidance": "Add CSP header to prevent XSS attacks"
    },
    {
      "severity": "medium",
      "category": "cookies",
      "issue": "insecure_cookie",
      "message": "Cookie 'session_id' missing Secure flag",
      "affected": ["session_id"],
      "guidance": "Set Secure flag on all authentication cookies"
    },
    {
      "severity": "low",
      "category": "storage",
      "issue": "sensitive_in_storage",
      "message": "Potentially sensitive key found in localStorage",
      "affected": ["auth_token", "api_key"],
      "guidance": "Consider using httpOnly cookies instead of localStorage for tokens"
    }
  ],
  "summary": {
    "headers": { "passed": 3, "failed": 2 },
    "cookies": { "secure": 5, "insecure": 2 },
    "storage": { "flagged_keys": 2 }
  }
}
```

**Note on storage audit:** Only key names are returned, never values. Pattern matching identifies sensitive keys (auth, token, key, secret, password, credential).

### Regression Response Schema

```json
{
  "status": "success",
  "action": "regression",
  "scope": "compare",
  "duration_ms": 8500,
  "baseline": {
    "captured_at": "2026-01-29T09:00:00Z",
    "url": "https://example.com/app"
  },
  "comparison": {
    "accessibility": {
      "baseline_issues": 5,
      "current_issues": 8,
      "new_issues": [
        { "id": "color-contrast", "count": 2 },
        { "id": "missing-alt", "count": 1 }
      ],
      "resolved_issues": []
    },
    "performance": {
      "lcp_baseline_ms": 1200,
      "lcp_current_ms": 1850,
      "lcp_delta_ms": 650,
      "lcp_regression": true
    },
    "security": {
      "baseline_findings": 2,
      "current_findings": 2,
      "changes": []
    }
  },
  "verdict": "regression_detected",
  "summary": "3 new accessibility issues, LCP regressed by 650ms"
}
```

## Security Model

**[CRITICAL RESOLUTION S1-1]** Uses existing AI Web Pilot toggle.

**Rationale:**
- Consistency: All extension-controlled features use same toggle
- Simplicity: One toggle for users to understand
- Security: analyze runs code in page context (axe-core), similar trust level to interact

**Implementation:**
- Check `ai_web_pilot_enabled` before any analyze operation
- Return `{ error: "ai_web_pilot_disabled" }` if OFF
- Same error handling as interact tool

**Redaction rules (reusing existing patterns from redaction.go):**
- Cookie values: `session_id=[REDACTED]`
- Storage values: Never returned, only keys
- Authorization headers: Stripped entirely
- Passwords in DOM: `<input type="password" value="[REDACTED]">`

## Data Flows

### Audit Flow (Accessibility Example)

```
1. AI: analyze({action: 'audit', scope: 'accessibility'})
2. Server: Check ai_web_pilot_enabled
3. Server: Create pending query {id: "abc123", type: "analyze", ...}
4. Server: Return {status: "pending", correlation_id: "abc123"}
5. Extension: Poll /pending-queries → receive analyze request
6. Extension: Check AXE_STATE, call ensureAxeLoaded() if needed
7. Extension: Run axe.run(document, options)
8. Extension: Transform to Gasoline schema
9. Extension: POST /analyze-result {correlation_id: "abc123", findings: [...]}
10. Server: Store result, mark query complete
11. AI: Poll observe({what: 'analyze_result'}) or receive via streaming
12. AI: Receive structured findings
```

### Memory Analysis Flow

```
1. AI: analyze({action: 'memory', scope: 'snapshot'})
2. Server: Create pending query, use blocking pattern (WaitForResult)
3. Extension: Poll, receive request
4. Extension: Read performance.memory (Chrome-only)
5. Extension: Count detached nodes via TreeWalker
6. Extension: POST /analyze-result
7. Server: WaitForResult completes, return to AI directly
```

## Edge Cases & Assumptions

### Edge Cases

1. **axe-core injection fails** (CSP blocks inline scripts)
   - Tier 1 fails → Tier 2 (chrome.debugger isolated context)
   - If DevTools open, Tier 2 fails → Return actionable error

2. **Page navigates during analysis**
   - content.js detects `beforeunload`
   - Sends abort signal to running analysis
   - Returns partial results with `status: "partial"`, `warning: "page_navigated"`

3. **Multiple concurrent analyses on same tab**
   - Server enforces per-tab mutex
   - Second request queued, returns when first completes
   - Timeout applies to queue wait + execution

4. **Analysis timeout**
   - Per-action timeouts (see table above)
   - Cancel and return partial results
   - Include `warning: "timeout"` in response

5. **performance.memory unavailable** (Firefox, Safari)
   - Feature-detect before use
   - Return `error: "memory_api_unavailable"` with guidance


### Assumptions

- Extension has permission to inject scripts into active tab
- AI Web Pilot toggle is enabled (enforced before analysis starts)
- Page is fully loaded before analysis (check `document.readyState`)
- Single tab context (multi-tab analysis out of scope)
- chrome.debugger permission granted (for CSP fallback)

## Risks & Mitigations

| Risk | Impact | Mitigation |
|------|--------|------------|
| axe-core bundle size (~500KB) | Slower extension load | Lazy-load only when analyze called; already bundled |
| chrome.debugger conflicts with DevTools | Users can't debug while analyzing | Document limitation; prefer Tier 1 injection |
| Memory API Chrome-only | No memory analysis on Firefox/Safari | Feature-detect, return clear error |

## Dependencies

### Required
- axe-core (already bundled at extension/vendor/axe.min.js, 527KB)
- Existing Gasoline infrastructure (Go server, extension, MCP)
- `debugger` permission in manifest (for CSP fallback)

### Optional (Future)
- Lighthouse (via service worker for deeper performance analysis)

## Performance Considerations

1. **Lazy loading**: axe-core loaded on first analyze call, stays loaded
2. **Caching**: Results cached 10 seconds; bypass with `force_refresh: true`
3. **Per-action timeouts**: Fast operations (memory, security) have tight timeouts
4. **Scoped analysis**: CSS selector limits scope, improves performance
5. **No Lighthouse in Phase 1**: Use existing vitals/performance observe modes

## Error Codes

New error codes for analyze tool:

| Code | Meaning |
|------|---------|
| `ai_web_pilot_disabled` | User has not enabled AI Web Pilot toggle |
| `axe_injection_failed` | Could not inject axe-core (CSP or other) |
| `axe_csp_blocked_devtools_open` | CSP blocked, fallback failed because DevTools open |
| `analysis_timeout` | Operation exceeded timeout |
| `memory_api_unavailable` | performance.memory not available (non-Chrome) |
| `invalid_selector` | CSS selector did not match any elements |
| `page_navigated` | Page navigation interrupted analysis |
| `concurrent_analysis` | Another analysis already running on this tab |
