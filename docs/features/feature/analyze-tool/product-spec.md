---
feature: analyze-tool
status: proposed
tool: analyze
mode: [audit, memory, render, bundle, security, regression]
version: v7.0
---

# The `analyze` Tool

## Problem Statement

Developers need AI to autonomously identify and fix issues in web applications. Currently:
1. `observe` captures raw telemetry (logs, network, DOM, performance metrics)
2. AI must reason about raw data to identify problems
3. Some analysis requires specialized tools (Lighthouse, axe-core) that AI cannot replicate by reasoning alone

This creates two gaps:
- **Efficiency gap**: AI spends tokens reasoning about raw data when specialized tools could provide structured findings
- **Capability gap**: Some analysis (computed accessibility trees, actual paint timing, security header validation) requires running code in the browser

## Solution

Introduce `analyze` as Gasoline's 5th MCP tool. It runs specialized analysis tools in the browser and returns structured, actionable findings.

**Key principle**: `observe` and `analyze` are complementary:
- `observe` = raw data for AI to reason about (deep investigation)
- `analyze` = structured findings from specialized tools (quick actionable results)

AI chooses the right approach for each task, or uses both.

## Requirements

### Functional Requirements

#### 1. Audit Mode (`audit`)

Run comprehensive audits using industry-standard tools.

##### Actions:
- `audit.accessibility` — WCAG compliance via axe-core
- `audit.performance` — Lighthouse performance analysis
- `audit.seo` — SEO best practices
- `audit.best_practices` — General web best practices
- `audit.full` — All of the above

##### Request:
```json
{
  "action": "audit",
  "scope": "accessibility",
  "selector": "main",
  "tab_id": 0
}
```

##### Response:
```json
{
  "status": "success",
  "findings": [
    {
      "severity": "critical",
      "category": "accessibility",
      "rule": "color-contrast",
      "message": "Text has insufficient contrast ratio (2.5:1, need 4.5:1)",
      "affected": ["button.submit", "a.nav-link"],
      "guidance": "Increase text color darkness or background lightness",
      "wcag": "1.4.3 (AA)"
    }
  ],
  "summary": {
    "critical": 2,
    "high": 5,
    "medium": 12,
    "low": 3
  },
  "duration_ms": 2400
}
```

#### 2. Memory Mode (`memory`)

Detect memory issues in the running application.

##### Actions:
- `memory.snapshot` — Capture heap snapshot
- `memory.leaks` — Detect growing allocations and detached DOM nodes
- `memory.compare` — Compare two snapshots to find growth

##### Response includes:
- Heap size and growth rate
- Detached DOM node count
- Top memory consumers (by constructor)
- Suspected leak patterns

#### 3. Render Mode (`render`)

Analyze rendering performance for React/Vue/Svelte applications.

##### Actions:
- `render.profile` — Capture render timing for components
- `render.wasted` — Identify unnecessary re-renders
- `render.tree` — Component tree with render counts

##### Response includes:
- Components ranked by render time
- Unnecessary re-renders (props unchanged)
- Suggested memoization opportunities

#### 4. Bundle Mode (`bundle`)

Analyze JavaScript bundle composition.

##### Actions:
- `bundle.size` — Total and per-chunk sizes
- `bundle.duplicates` — Duplicate dependencies
- `bundle.unused` — Unused exports (tree-shaking opportunities)

##### Response includes:
- Bundle size breakdown
- Duplicate packages
- Large dependencies
- Suggested optimizations

#### 5. Security Mode (`security`)

Check for common security issues.

##### Actions:
- `security.headers` — Validate security headers (CSP, HSTS, X-Frame-Options)
- `security.cookies` — Check cookie flags (HttpOnly, Secure, SameSite)
- `security.storage` — Audit localStorage/sessionStorage for sensitive data
- `security.deps` — Check for known vulnerabilities in dependencies

##### Response includes:
- Missing/misconfigured security headers
- Insecure cookie settings
- Potentially sensitive data in storage
- CVEs in dependencies (if detectable)

#### 6. Regression Mode (`regression`)

Compare before/after states to detect regressions.

##### Actions:
- `regression.baseline` — Capture current state as baseline
- `regression.compare` — Compare current state against baseline
- `regression.clear` — Clear stored baseline

##### Response includes:
- Performance delta (LCP, FID, CLS changes)
- New accessibility violations
- Bundle size changes
- New console errors

### Non-Functional Requirements

1. **Performance**
   - Audits complete in < 5 seconds for typical pages
   - Memory snapshots complete in < 3 seconds
   - No main thread blocking during analysis

2. **Privacy**
   - All analysis runs locally
   - No external services called
   - Results never leave localhost

3. **Opt-in**
   - Requires "AI Web Pilot" toggle enabled
   - Returns explicit error if disabled

### Out of Scope

- Visual regression (screenshot comparison) — requires baseline management
- Full-site crawling — assumes single active page
- Real user monitoring (RUM) — this is synthetic analysis only

## Success Criteria

1. **AI can identify and fix issues autonomously**
   - Runs `analyze({action: 'audit', scope: 'accessibility'})`
   - Receives structured findings
   - Uses `interact` to fix issues
   - Re-runs analysis to confirm fix

2. **Complements `observe`**
   - AI uses `observe` for raw data when reasoning is needed
   - AI uses `analyze` for quick structured findings
   - Both can be used in the same debugging session

3. **Performance acceptable**
   - Full audit < 5 seconds
   - No degradation to browsing

## User Workflow

### Quick Accessibility Fix

```
Developer: "Make sure this form is accessible"
AI: [Runs analyze({action: 'audit', scope: 'accessibility', selector: 'form'})]
Result: "Missing label on input#email, missing aria-describedby on error messages"
AI: [Uses interact to add labels and ARIA attributes]
AI: [Re-runs analyze to confirm]
Result: "All accessibility checks passed"
```

### Deep Performance Investigation

```
Developer: "Why is this page slow?"
AI: [Runs analyze({action: 'audit', scope: 'performance'})]
Result: "LCP 3.2s, caused by large hero image"
AI: [Wants more detail, runs observe({what: 'performance_trace'})]
AI: [Reasons about raw trace data]
AI: [Identifies specific render-blocking CSS]
AI: [Uses generate to create optimized code]
```

### Memory Leak Detection

```
Developer: "I think there's a memory leak"
AI: [Runs analyze({action: 'memory', scope: 'snapshot'})]
AI: [Asks developer to reproduce the issue]
AI: [Runs analyze({action: 'memory', scope: 'compare'})]
Result: "Heap grew 15MB, 847 detached DOM nodes, likely leak in EventListener not being removed"
AI: [Uses observe to capture code context]
AI: [Generates fix with proper cleanup]
```

## Relationship to Other Tools

| Tool | When to use |
|------|-------------|
| `observe` | Need raw data to reason about; deep investigation |
| `analyze` | Need quick structured findings; actionable results |
| `generate` | Create code/tests based on findings |
| `interact` | Apply fixes in the browser |
| `configure` | Change Gasoline settings |

## Notes

- This is Gasoline's 5th tool, expanding the previous 4-tool constraint
- `analyze` leverages existing tools (axe-core) rather than reinventing
- Designed for active development workflow, not batch CI/CD auditing
- AI chooses between `observe` and `analyze` based on task requirements
- **Excluded by design:** `render` and `bundle` modes — developers have better native tools (React DevTools, webpack-bundle-analyzer). Gasoline focuses on things AI can't access otherwise.
