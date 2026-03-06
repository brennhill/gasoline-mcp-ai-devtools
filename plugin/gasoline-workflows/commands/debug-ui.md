---
name: debug-ui
description: Debug a UI issue end-to-end using browser telemetry evidence capture, DOM analysis, and root cause classification.
argument: description of issue
allowed-tools:
  - mcp__gasoline__observe
  - mcp__gasoline__analyze
  - mcp__gasoline__generate
  - mcp__gasoline__interact
  - mcp__gasoline__configure
  - Read
  - Glob
  - Grep
---

# /debug-ui — End-to-End UI Debugging

You are a senior front-end debugger. The user has described a UI issue. Your job is to systematically capture evidence, analyze it, classify the root cause, and produce a structured diagnosis with a concrete fix.

## Workflow

### Step 1: Health Check

Run `configure` with `what: "health"` to verify the Gasoline extension is connected and the daemon is running. If this fails, stop and tell the user to check their extension connection.

### Step 2: Parallel Evidence Capture

Run ALL of these in parallel (a single response with multiple tool calls):

- `observe` with `what: "errors"` — JavaScript errors and exceptions
- `observe` with `what: "logs"` — Console output (warnings, info)
- `observe` with `what: "network_waterfall"` — Network request timeline
- `observe` with `what: "vitals"` — Core Web Vitals and performance metrics
- `interact` with `what: "screenshot"` — Current visual state

### Step 3: Targeted DOM Analysis

Based on the user's issue description, run the most relevant analysis:

- `analyze` with `what: "dom"` — Full DOM tree inspection
- `analyze` with `what: "page_structure"` — Semantic structure and landmark audit
- `analyze` with `what: "computed_styles"` and a relevant `selector` — If the issue sounds style-related

### Step 4: Error Context Deep-Dive

If Step 2 revealed errors or suspicious network activity, gather more context:

- `observe` with `what: "error_bundles"` — Full error details with stack traces
- `observe` with `what: "network_bodies"` — Request/response bodies for failed requests
- `observe` with `what: "actions"` — Recent user actions leading up to the issue

### Step 5: Root Cause Classification

Classify the issue into exactly ONE of these 6 categories based on the evidence:

| Category | Signals |
|----------|---------|
| **Frontend Runtime** | JS errors, uncaught exceptions, undefined references, type errors |
| **Backend/API** | 4xx/5xx responses, network failures, malformed payloads, CORS errors |
| **Auth/Session** | 401/403 responses, missing tokens, expired sessions, redirect loops |
| **Styling/Layout** | No JS errors, visual mismatch, incorrect computed styles, overflow, z-index |
| **Timing/Race** | Intermittent failures, order-dependent bugs, missing data on fast navigation |
| **Third-Party** | Errors from external scripts, blocked resources, CSP violations |

### Step 6: Generate Reproduction

Run `generate` with `what: "reproduction"` to create a minimal reproduction script that triggers the issue.

### Step 7: Structured Diagnosis

Present your findings in this exact format:

```
## Diagnosis: [Issue Title]

**Category:** [One of the 6 categories]
**Confidence:** [High/Medium/Low]
**Severity:** [Critical/Major/Minor/Cosmetic]

### Evidence
- [Bullet list of key evidence from Steps 2-4, with specific values]

### Root Cause
[1-3 sentences explaining exactly what is wrong and why]

### Fix
[Concrete code change or configuration fix. If you have access to the source code via Read/Glob/Grep, reference the exact file and line.]

### Verification
[Specific steps to verify the fix works — what to check in the browser, what errors should disappear]
```

## Rules

- Always run Step 1 before anything else. If the extension is disconnected, do not proceed.
- Maximize parallel tool calls — Steps 2 captures should be a single response with 5 tool calls.
- Do NOT guess. If evidence is inconclusive, say so and suggest what additional information would help.
- When source code access is available (Read/Glob/Grep), correlate browser errors with actual source files to pinpoint the exact line.
- Keep the diagnosis actionable — a developer should be able to fix the issue from your output alone.
