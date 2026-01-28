---
feature: local-web-scraping
status: proposed
version: null
tool: interact
mode: scrape
authors: []
created: 2026-01-28
updated: 2026-01-28
---

# Local Web Scraping & Automation (LLM-Controlled)

> Enable AI coding agents to extract structured data from and automate interactions with web pages using the user's own browser sessions, entirely on localhost.

## Problem

AI coding agents frequently need data from web applications: internal dashboards, authenticated services, documentation portals, admin panels, and third-party platforms. Today, agents cannot access this data because:

1. **Authentication wall** -- Most valuable data lives behind login walls. Cloud scraping services (Hyperbrowser, Browserbase) use ephemeral browser instances with no access to the user's sessions, cookies, or SSO tokens.
2. **No structured extraction** -- Agents can already use `interact {action: "execute_js"}` to run arbitrary JS, but scraping a page requires orchestrating multiple steps: navigate, wait for load, extract data, paginate, handle errors. The agent must manually chain 5-15 MCP calls per scrape.
3. **No workflow reuse** -- Repetitive scraping tasks (daily metrics export, weekly report pull) require the agent to reconstruct the entire multi-step sequence every time.
4. **Internal tools are unreachable** -- Corporate intranets, staging environments, and internal admin panels are inaccessible to cloud services but readily available in the user's browser.

## Solution

Add `interact {action: "scrape"}` as a high-level orchestration mode that composes existing Gasoline primitives (`execute_js`, `navigate`, DOM observation) into multi-step workflows with structured data extraction, error recovery, and optional recording/replay.

**Key design decision:** `scrape` is a **workflow orchestrator**, not a new execution primitive. It internally dispatches to existing `execute_js`, `navigate`, and `observe` capabilities. This keeps the execution surface unchanged while adding coordination logic.

**Key differentiator from cloud scraping services:**
- Uses YOUR browser with YOUR logged-in sessions
- Localhost-only -- no data leaves your machine
- Integrated with existing Gasoline telemetry (errors, network, WebSocket events captured during scrape)
- AI agent controls the workflow, not a fixed script

## User Stories

- As an AI coding agent, I want to extract structured data from an authenticated web application so that I can use it in code generation, analysis, or testing.
- As a developer using Gasoline, I want to scrape data from my company's internal tools (Jira, Confluence, admin panels) so that the AI can reference it during development.
- As an AI coding agent, I want to record a multi-step scraping workflow so that I can replay it later without reconstructing each step.
- As a developer using Gasoline, I want to export personal data from services I am logged into (banking exports, social media data, email) so that I can process it locally.
- As an AI coding agent, I want structured data extraction with schema hints so that I receive clean, typed data instead of raw HTML.
- As an AI coding agent, I want error recovery built into scraping workflows so that transient failures (slow loads, popups, CAPTCHA) do not silently produce incomplete results.

## MCP Interface

**Tool:** `interact`
**Action:** `scrape`

The scrape action supports three sub-operations: `extract` (single-page structured extraction), `workflow` (multi-step orchestration), and `replay` (replay a recorded workflow).

### Sub-operation: `extract` (Single-page structured extraction)

#### Request
```json
{
  "tool": "interact",
  "arguments": {
    "action": "scrape",
    "operation": "extract",
    "selector": "table.results tbody tr",
    "schema": {
      "name": "td:nth-child(1) | text",
      "email": "td:nth-child(2) | text",
      "status": "td:nth-child(3) | text",
      "avatar": "td:nth-child(4) img | attr:src"
    },
    "options": {
      "wait_for": ".results-loaded",
      "timeout_ms": 5000,
      "paginate": {
        "next_selector": "a.next-page",
        "max_pages": 5,
        "delay_ms": 1000
      }
    }
  }
}
```

#### Response
```json
{
  "status": "complete",
  "correlation_id": "corr-abc123",
  "data": [
    {"name": "Alice", "email": "alice@example.com", "status": "active", "avatar": "/img/alice.png"},
    {"name": "Bob", "email": "bob@example.com", "status": "inactive", "avatar": "/img/bob.png"}
  ],
  "metadata": {
    "pages_scraped": 3,
    "rows_extracted": 47,
    "duration_ms": 4200,
    "url": "https://internal.company.com/admin/users",
    "errors": []
  }
}
```

### Sub-operation: `workflow` (Multi-step orchestration)

#### Request
```json
{
  "tool": "interact",
  "arguments": {
    "action": "scrape",
    "operation": "workflow",
    "name": "export_jira_sprint_data",
    "record": true,
    "steps": [
      {
        "type": "navigate",
        "url": "https://jira.company.com/boards/PROJ"
      },
      {
        "type": "wait",
        "selector": ".board-loaded",
        "timeout_ms": 10000
      },
      {
        "type": "click",
        "selector": "button[data-testid='sprint-report']"
      },
      {
        "type": "wait",
        "selector": ".report-table",
        "timeout_ms": 5000
      },
      {
        "type": "extract",
        "selector": ".report-table tbody tr",
        "schema": {
          "ticket": "td.key | text",
          "summary": "td.summary | text",
          "status": "td.status | text",
          "assignee": "td.assignee | text",
          "points": "td.points | text | number"
        }
      }
    ],
    "error_recovery": {
      "on_timeout": "retry_step",
      "max_retries": 2,
      "on_failure": "abort_with_partial"
    }
  }
}
```

#### Response
```json
{
  "status": "complete",
  "correlation_id": "corr-def456",
  "workflow_id": "wf-export_jira_sprint_data-20260128",
  "steps_completed": 5,
  "steps_total": 5,
  "data": [
    {"ticket": "PROJ-101", "summary": "Fix login bug", "status": "Done", "assignee": "Alice", "points": 3},
    {"ticket": "PROJ-102", "summary": "Add dark mode", "status": "In Progress", "assignee": "Bob", "points": 5}
  ],
  "metadata": {
    "duration_ms": 12400,
    "recorded": true,
    "workflow_id": "wf-export_jira_sprint_data-20260128"
  }
}
```

### Sub-operation: `replay` (Replay a recorded workflow)

#### Request
```json
{
  "tool": "interact",
  "arguments": {
    "action": "scrape",
    "operation": "replay",
    "workflow_id": "wf-export_jira_sprint_data-20260128",
    "override_params": {
      "steps[0].url": "https://jira.company.com/boards/PROJ2"
    }
  }
}
```

#### Response
```json
{
  "status": "complete",
  "correlation_id": "corr-ghi789",
  "workflow_id": "wf-export_jira_sprint_data-20260128",
  "steps_completed": 5,
  "steps_total": 5,
  "data": [
    {"ticket": "PROJ2-201", "summary": "API redesign", "status": "Done", "assignee": "Carol", "points": 8}
  ],
  "metadata": {
    "duration_ms": 11800,
    "replayed_from": "wf-export_jira_sprint_data-20260128"
  }
}
```

## Architectural Design

### Why a single `scrape` action, not multiple new actions?

The design composes existing primitives rather than creating parallel action surfaces:

| Approach | Pros | Cons |
|----------|------|------|
| **Single `scrape` action (chosen)** | One entry point, LLM selects sub-operation via `operation` param; workflow logic encapsulated server-side; fewer tool calls | More complex parameter schema; server must orchestrate steps |
| **Multiple actions (`scrape_extract`, `scrape_workflow`, `scrape_replay`)** | Simpler per-action schema | Pollutes the action enum; workflow state management more complex for LLM |
| **LLM chains existing primitives** | No new code needed | 5-15 MCP calls per scrape; no error recovery; no recording; poor DX |

**Verdict:** Single `scrape` action with `operation` sub-selector. This follows Gasoline's established pattern (e.g., `observe` uses `what` to select sub-modes). The LLM makes one call for a complete scraping task instead of manually orchestrating many calls.

### Execution Model

The `scrape` action extends the existing async command architecture (v6.0.0):

```
LLM calls interact({action: "scrape", ...})
    |
    v
Server validates params, creates workflow plan
    |
    v
Server queues first step as pending query
    |--- Returns immediately: {status: "queued", correlation_id: "..."}
    |
    v
Extension polls /pending-queries, picks up step
    |
    v
Extension executes step (navigate/wait/click/extract)
    |
    v
Extension POSTs step result to /execute-result
    |
    v
Server receives step result
    |--- If more steps: queues next step as pending query
    |--- If error + recovery: applies recovery strategy, requeues
    |--- If final step: marks workflow complete
    |
    v
LLM polls observe({what: "command_result", correlation_id: "..."})
    |--- Gets intermediate progress or final result
```

**Key insight:** Each workflow step is dispatched as an individual pending query through the existing async command pipeline. The workflow engine on the server side manages step sequencing, error recovery, and result aggregation. The extension does not need to know it is part of a multi-step workflow -- it executes individual commands as it always has.

### Data Flow for Structured Extraction

```
Server sends pending query:
  {type: "scrape_extract", params: {selector: "...", schema: {...}}}
    |
    v
Extension (background.js) routes to content script
    |
    v
Content script sends to inject.js via postMessage
    |
    v
inject.js executes in page context:
  1. querySelectorAll(selector) -> collect matching elements
  2. For each element, apply schema field selectors
  3. Apply type coercions (text, number, attr:X, html)
  4. Return structured array
    |
    v
Result flows back: inject.js -> content.js -> background.js -> POST /execute-result
```

### Workflow Storage

Recorded workflows are stored in the server's in-memory state (consistent with existing state management in `save_state`/`load_state`):

| Storage | Capacity | Eviction | Persistence |
|---------|----------|----------|-------------|
| Workflow definitions | 20 max | LRU | In-memory only (lost on server restart) |
| Workflow results | Last 10 | Oldest first | In-memory only |

**Future consideration:** JSONL persistence for workflows across server restarts, consistent with log persistence.

## Requirements

| # | Requirement | Priority |
|---|-------------|----------|
| R1 | Single-page structured data extraction with CSS selector + schema mapping | must |
| R2 | Multi-step workflow orchestration (navigate, wait, click, extract) | must |
| R3 | Async execution via existing correlation_id pattern | must |
| R4 | AI Web Pilot toggle gate (scrape requires pilot enabled) | must |
| R5 | Error recovery: retry on timeout, abort with partial data on failure | must |
| R6 | Schema type coercions: text, number, attr:X, html, boolean | must |
| R7 | Pagination support (next_selector + max_pages) | should |
| R8 | Workflow recording for replay | should |
| R9 | Workflow replay with parameter overrides | should |
| R10 | Progress reporting via observe polling (steps completed / total) | should |
| R11 | wait_for selector with configurable timeout per step | should |
| R12 | Rate limiting / delay between steps to avoid triggering anti-bot | should |
| R13 | Telemetry integration: errors and network events captured during scrape | could |
| R14 | Workflow export/import (JSON file) for sharing | could |
| R15 | Conditional steps (if element exists, take branch A) | could |

## Non-Goals

- This feature does NOT implement cloud-based scraping. All execution happens in the user's local browser.
- This feature does NOT bypass same-origin policy, CORS, or browser security. It operates within the same constraints as manual browsing.
- This feature does NOT include CAPTCHA solving, proxy rotation, or anti-bot evasion. If a site blocks automated access, the user must handle it manually.
- This feature does NOT interpret or analyze scraped data. It extracts and structures data; the AI agent decides what to do with it (consistent with Gasoline's "capture, don't interpret" philosophy).
- This feature does NOT persist workflows across server restarts in the initial implementation. In-memory only.
- Out of scope: Natural language action execution ("click the login button"). The LLM must provide explicit selectors. This was explicitly deferred in competitive analysis.
- Out of scope: Cross-tab or cross-origin orchestration. Workflow operates on a single tab at a time.
- Out of scope: Headless browser mode. This feature uses the user's visible browser.

## Performance SLOs

| Metric | Target |
|--------|--------|
| Single extract (no pagination) | < 2s for up to 500 elements |
| Per-step overhead (workflow engine) | < 50ms server-side |
| Step dispatch latency | < 2s (extension poll interval) |
| Full workflow (5 steps) | < 30s typical |
| Schema mapping per element | < 1ms |
| Memory: workflow definitions (20) | < 200KB |
| Memory: workflow results (10) | < 2MB |
| Main thread blocking (extension) | 0ms (all async) |

## Security Considerations

### Data Sensitivity

This is the highest-risk feature in Gasoline. Unlike `execute_js` (which runs individual scripts), `scrape` automates multi-step data extraction from authenticated sessions. This fundamentally changes the threat model:

| Risk | Severity | Mitigation |
|------|----------|------------|
| LLM exfiltrates sensitive data (banking, email, PII) | **Critical** | Localhost-only; data never leaves machine; LLM sees results only in its context window |
| LLM scrapes unintended pages via workflow | High | AI Web Pilot toggle required; user must enable scraping explicitly |
| Workflow replays against wrong target | Medium | Workflow stores original URL; warn on domain mismatch during replay |
| Scraped data leaks via LLM context to cloud provider | **Critical** | Cannot fully mitigate; user accepts this risk by enabling AI Web Pilot; documented clearly |
| Excessive automated requests trigger account lockout | Medium | Rate limiting (R12); delay_ms between steps; max_pages cap |
| Stored workflows contain sensitive URLs/selectors | Low | In-memory only; no disk persistence in v1; cleared on server restart |

### Privacy Architecture

```
                          TRUST BOUNDARY
                               |
  [Browser + Sessions] -----> [Gasoline Server (localhost)] -----> [LLM Context Window]
        |                           |                                     |
  User's cookies/auth         Data stays local               Data MAY be sent to
  Never sent to server        In-memory only                 LLM provider (cloud)
  (execute in page ctx)       No disk by default             User accepts this risk
```

**Critical privacy distinction:** Gasoline itself keeps all data on localhost. However, the LLM that consumes the scraped data may run in the cloud (e.g., Claude API, OpenAI API). The scraped content enters the LLM's context window and is subject to the LLM provider's data handling policies. This must be documented prominently.

### Security Controls

1. **AI Web Pilot toggle (existing)** -- Must be ON for any scrape action. Toggle defaults OFF.
2. **No new permissions** -- Scrape uses existing `execute_js` and DOM query capabilities. No new browser APIs required.
3. **No credential extraction** -- Schema extraction operates on visible DOM content only. No access to `document.cookie`, `localStorage`, or password field values through the schema mapping engine.
4. **Explicit selectors only** -- LLM must provide CSS selectors, not natural language. This keeps the user (via the LLM's code) in explicit control.
5. **Rate limiting** -- Configurable delay between steps (default 500ms). Max pages cap prevents runaway pagination.
6. **Domain lock on replay** -- Replaying a workflow against a different domain than the original requires explicit `allow_domain_change: true` flag.

### Sensitive Data Redaction in Schema

The schema extraction engine must NOT extract:
- `input[type="password"]` values (always redacted to `"[REDACTED]"`)
- Elements matching `[data-sensitive]`, `[data-private]`, or `.sensitive` (configurable)

## Edge Cases

- **What happens when the page requires login mid-workflow?** Expected behavior: Step fails with `{error: "navigation_redirect", redirect_url: "..."}`. Workflow pauses; LLM can instruct user to log in and retry.
- **What happens when a selector matches zero elements?** Expected behavior: Returns empty array `[]` with `metadata.rows_extracted: 0`. Not an error -- the selector simply did not match.
- **What happens when pagination reaches a page with no results?** Expected behavior: Pagination stops. Returns data collected so far.
- **What happens when the extension disconnects mid-workflow?** Expected behavior: Current step times out (10s). Workflow enters `error` state with partial results from completed steps.
- **What happens when a workflow step encounters a popup/modal?** Expected behavior: The `wait_for` selector times out if the expected content is not visible. LLM receives timeout error and can decide to dismiss the modal via `execute_js` or abort.
- **What happens when the tab is closed during a workflow?** Expected behavior: Extension reports tab closed. Workflow aborts with partial data and `{error: "tab_closed"}`.
- **What happens when two workflows run concurrently?** Expected behavior: Only one scrape workflow can run at a time per tab. Second request returns `{error: "workflow_in_progress", active_workflow_id: "..."}`.
- **What happens when schema field selector is invalid CSS?** Expected behavior: Field returns `null` with warning in metadata. Other fields still extracted.
- **What happens when extract returns very large data (>1MB)?** Expected behavior: Data is truncated to 1MB with `metadata.truncated: true`. Warning returned to LLM.

## Dependencies

- **Depends on:**
  - Async command architecture (v6.0.0) -- for correlation_id pattern and non-blocking execution
  - AI Web Pilot toggle -- for security gating
  - `execute_js` infrastructure -- for page-context code execution
  - DOM query infrastructure -- for selector-based element access
  - `navigate` action -- for URL navigation in workflows
- **Depended on by:** None (new capability, no existing features depend on this)

## Assumptions

- A1: Extension is connected and AI Web Pilot toggle is ON.
- A2: Target page is loaded in the tracked (or active) tab.
- A3: User has already authenticated with the target service (Gasoline does not handle login flows).
- A4: CSS selectors provided by the LLM are valid and match the current page structure.
- A5: Pages use standard DOM structure (no Shadow DOM v1 closed-mode elements for extraction -- open Shadow DOM may be supported in future).
- A6: The async command pipeline (pending-queries / execute-result) is functioning normally.
- A7: The user understands that scraped data will enter the LLM's context window and may be processed by the LLM provider.

## Implementation Estimate

| Component | Effort | Description |
|-----------|--------|-------------|
| Go server: scrape action dispatcher | ~100 lines | Parse params, route to sub-operation handler |
| Go server: workflow engine | ~250 lines | Step sequencing, error recovery, result aggregation |
| Go server: workflow storage | ~80 lines | In-memory store for recorded workflows, LRU eviction |
| Extension: schema extraction engine | ~150 lines | inject.js: selector-based structured extraction with type coercion |
| Extension: scrape query handler | ~80 lines | background.js: route scrape_extract queries to content/inject |
| Extension: wait/click primitives | ~60 lines | inject.js: waitForSelector, clickElement utilities |
| Tests (Go + Extension) | ~150 lines | Unit tests for workflow engine, schema extraction, edge cases |
| **Total** | **~870 lines** | Consistent with competitive analysis estimate (~800) |

## Open Items

| # | Item | Status | Notes |
|---|------|--------|-------|
| OI-1 | Should workflow recording persist to JSONL file? | open | In-memory-only means workflows lost on restart. JSONL would be consistent with log persistence. Risk: stored workflows may contain sensitive URLs. |
| OI-2 | Should `scrape` require a separate toggle from AI Web Pilot? | open | Scrape is higher-risk than individual `execute_js` calls. A dedicated "scraping enabled" toggle would add granular control but increases UX friction. |
| OI-3 | Should extraction support Shadow DOM (open mode)? | open | Many modern web apps use Shadow DOM. Supporting it adds complexity (~50 lines) but broadens usefulness. |
| OI-4 | Should there be a maximum workflow step count? | open | Unbounded workflows could run indefinitely. Suggest max 50 steps as safety limit. |
| OI-5 | How should conditional branching work in workflows? | open | R15 (could priority). Options: (a) if/else step type, (b) LLM handles branching by inspecting intermediate results and issuing new workflow. Option (b) is simpler and keeps the workflow engine stateless. |
| OI-6 | Should extracted data be redacted before entering LLM context? | open | Could offer configurable redaction patterns (e.g., mask credit card numbers, SSNs). Adds complexity but significantly reduces data exposure risk. |
| OI-7 | Should this feature integrate with `generate` tool? | open | E.g., `generate {type: "scrape_report"}` to produce a summary of scraped data. May violate "capture, don't interpret" principle. |
