---
feature: self-healing-tests
status: proposed
version: null
tool: observe, generate
mode: test_diagnosis, test_fix
authors: []
created: 2026-01-28
updated: 2026-01-28
---

# Self-Healing Tests

> Enable AI coding agents to detect broken E2E tests, diagnose why they broke using live browser telemetry, and generate targeted fix proposals -- all through existing Gasoline MCP tools.

## Problem

E2E test failures are one of the most disruptive events in a development workflow. When a test breaks, the developer must:

1. **Context-switch** from their current task to investigate the failure.
2. **Reproduce locally**, often requiring environment setup and manual browser inspection.
3. **Diagnose the root cause**, which may be a stale selector (the UI changed), an API contract change (response shape evolved), a timing issue (race condition or slow network), or a legitimate regression.
4. **Fix the test or the code**, update any related fixtures/mocks, and verify the fix.

For AI coding agents, this problem is worse: the agent receives a test failure message (e.g., "Element not found: .submit-btn") but has no visibility into the actual browser state at the time of failure. Without browser telemetry, the agent can only guess at the cause.

**Current state:** Gasoline already captures console errors, network traffic, DOM state, and user actions in real time. The `generate({format: "test"})` tool can produce test scaffolds from captured telemetry. But there is no structured workflow for an agent to (a) ingest a test failure, (b) correlate it against live browser state, and (c) produce a targeted fix rather than a whole new test.

## Solution

Self-Healing Tests adds two new modes to existing Gasoline tools:

- **`observe({what: "test_diagnosis"})`** -- Accepts a test failure description (error message, stack trace, failed assertion) and correlates it against captured browser telemetry to produce a structured diagnosis. The server analyzes console errors, network failures, DOM state, and recent changes to classify the failure into a root cause category.

- **`generate({format: "test_fix"})`** -- Given a diagnosis (or raw failure), generates a targeted fix proposal: updated selectors, adjusted wait conditions, corrected API expectations, or a recommendation that the failure is a true regression requiring code changes rather than test changes.

Together, these modes enable a multi-step self-healing workflow:

```
1. Test fails (agent receives error output)
2. Agent calls observe({what: "test_diagnosis", ...}) with the failure details
3. Gasoline correlates failure against live telemetry and returns structured diagnosis
4. Agent calls generate({format: "test_fix", ...}) with the diagnosis
5. Gasoline returns a fix proposal (code patch, selector update, wait adjustment)
6. Agent applies the fix and re-runs the test to verify
```

This is a **multi-step workflow using existing tools**, not a new autonomous loop. The AI agent orchestrates the steps. Gasoline provides the observation and generation primitives.

## User Stories

- As an AI coding agent, I want to submit a test failure message and receive a structured diagnosis of why the test broke, so that I can fix the right thing instead of guessing.
- As an AI coding agent, I want to receive a targeted fix proposal (updated selector, adjusted timeout, corrected API mock) for a diagnosed test failure, so that I can apply the fix without rewriting the entire test.
- As a developer using Gasoline, I want the AI agent to explain why a test failed (DOM changed, API changed, timing issue) so that I can approve or refine the proposed fix.
- As an AI coding agent, I want the diagnosis to tell me whether the failure is a test problem or a code regression, so that I know whether to fix the test or escalate to the developer.
- As a developer using Gasoline, I want self-healing to work regardless of whether my tests use Playwright, Cypress, Puppeteer, or another framework, so that I do not need framework-specific tooling.

## MCP Interface

### Tool 1: `observe` -- Test Diagnosis

**Mode:** `test_diagnosis`

Correlates a test failure against captured browser telemetry to produce a structured root cause analysis.

#### Request

```json
{
  "tool": "observe",
  "arguments": {
    "what": "test_diagnosis",
    "failure": {
      "message": "Element not found: [data-testid='submit-btn']",
      "stack": "at SubmitForm.test.js:42\n  at async click(...)",
      "test_name": "SubmitForm > should submit the form",
      "framework": "playwright"
    },
    "context": {
      "since": "2026-01-28T10:30:00Z",
      "test_file": "tests/e2e/submit-form.test.js"
    }
  }
}
```

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `what` | string | yes | Must be `"test_diagnosis"` |
| `failure.message` | string | yes | The error message from the test runner |
| `failure.stack` | string | no | Stack trace from the test failure |
| `failure.test_name` | string | no | Human-readable test name or describe/it path |
| `failure.framework` | string | no | Test framework hint: `playwright`, `cypress`, `puppeteer`, or omitted for auto-detect |
| `context.since` | string (ISO 8601) | no | Only consider telemetry after this timestamp. Defaults to last 60 seconds. |
| `context.test_file` | string | no | Path to the test file, used for selector extraction |

#### Response

```json
{
  "diagnosis": {
    "category": "selector_stale",
    "confidence": "high",
    "summary": "The selector [data-testid='submit-btn'] does not match any element in the current DOM. A button with data-testid='submit-button' exists and matches the expected role and position.",
    "evidence": {
      "dom": {
        "expected_selector": "[data-testid='submit-btn']",
        "matches": 0,
        "candidates": [
          {
            "selector": "[data-testid='submit-button']",
            "tag": "button",
            "text": "Submit",
            "similarity": 0.92
          }
        ]
      },
      "console_errors": [],
      "network_failures": [],
      "timing": {
        "page_load_ms": 1200,
        "dom_ready_ms": 800
      }
    },
    "root_cause": "The data-testid attribute was renamed from 'submit-btn' to 'submit-button'. This is a selector drift issue, not a regression.",
    "recommended_action": "fix_test",
    "related_changes": [
      {
        "type": "dom_change",
        "description": "Button data-testid changed: submit-btn -> submit-button",
        "timestamp": "2026-01-28T10:31:15Z"
      }
    ]
  }
}
```

**Response fields:**

| Field | Type | Description |
|-------|------|-------------|
| `diagnosis.category` | enum | One of: `selector_stale`, `api_contract_changed`, `timing_issue`, `element_removed`, `network_failure`, `js_error`, `true_regression`, `flaky`, `unknown` |
| `diagnosis.confidence` | enum | `high`, `medium`, `low` -- based on evidence strength |
| `diagnosis.summary` | string | Human-readable explanation of what went wrong |
| `diagnosis.evidence` | object | Supporting data from browser telemetry |
| `diagnosis.evidence.dom` | object | DOM state: expected selector, match count, candidate alternatives |
| `diagnosis.evidence.console_errors` | array | Console errors captured during the test window |
| `diagnosis.evidence.network_failures` | array | Failed network requests (4xx/5xx) during the test window |
| `diagnosis.evidence.timing` | object | Page load and DOM ready timings |
| `diagnosis.root_cause` | string | Plain-English root cause explanation |
| `diagnosis.recommended_action` | enum | `fix_test` (selector/timing issue), `fix_code` (true regression), `mark_flaky` (intermittent), `investigate` (insufficient evidence) |
| `diagnosis.related_changes` | array | Relevant state changes observed in the telemetry window |

**Diagnosis categories explained:**

| Category | Meaning | Typical cause |
|----------|---------|---------------|
| `selector_stale` | Selector does not match any element; a close match exists | CSS class renamed, data-testid changed, DOM restructured |
| `api_contract_changed` | Network response shape differs from what the test expects | Backend field rename, new required field, type change |
| `timing_issue` | Element exists but was not ready when the test acted on it | Slow render, animation, lazy loading, async data fetch |
| `element_removed` | No close match exists; the element appears to have been deliberately removed | Feature removed, component replaced, route changed |
| `network_failure` | A network request that the test depends on failed | Server error, endpoint moved, auth expired |
| `js_error` | An uncaught JavaScript error occurred before the assertion | Null reference, import error, build regression |
| `true_regression` | The application behavior changed; this is not a test issue | Bug in application code |
| `flaky` | The test has passed before with the same code; failure appears intermittent | Race condition, external dependency, test isolation |
| `unknown` | Insufficient evidence to classify | No telemetry captured, test ran outside Gasoline window |

### Tool 2: `generate` -- Test Fix

**Mode:** `test_fix`

Generates a targeted fix proposal for a diagnosed test failure.

#### Request

```json
{
  "tool": "generate",
  "arguments": {
    "format": "test_fix",
    "diagnosis": {
      "category": "selector_stale",
      "expected_selector": "[data-testid='submit-btn']",
      "candidates": [
        {
          "selector": "[data-testid='submit-button']",
          "similarity": 0.92
        }
      ]
    },
    "test_file": "tests/e2e/submit-form.test.js",
    "framework": "playwright"
  }
}
```

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `format` | string | yes | Must be `"test_fix"` |
| `diagnosis` | object | yes | The diagnosis object from `observe({what: "test_diagnosis"})`, or a manually constructed equivalent |
| `diagnosis.category` | string | yes | The failure category (from the diagnosis enum) |
| `test_file` | string | no | Path to the test file for context |
| `framework` | string | no | Target framework for the generated fix. Defaults to `"generic"`. |
| `fix_strategy` | string | no | Override: `"selector"` (update selector only), `"wait"` (add/adjust waits), `"mock"` (update API mock), `"full"` (comprehensive). Defaults to auto-select based on category. |

#### Response

```json
{
  "fix": {
    "strategy": "selector_update",
    "description": "Replace stale selector with the matching candidate.",
    "changes": [
      {
        "type": "selector_replace",
        "old_value": "[data-testid='submit-btn']",
        "new_value": "[data-testid='submit-button']",
        "confidence": "high",
        "rationale": "Candidate [data-testid='submit-button'] is a button element with text 'Submit' and 0.92 similarity score. It occupies the same DOM position as the expected element."
      }
    ],
    "framework_hint": "In Playwright, update: page.locator(\"[data-testid='submit-button']\")",
    "warnings": [],
    "recommended_action": "fix_test"
  }
}
```

**Response fields:**

| Field | Type | Description |
|-------|------|-------------|
| `fix.strategy` | enum | `selector_update`, `wait_adjustment`, `api_mock_update`, `code_fix_needed`, `mark_flaky`, `no_fix_available` |
| `fix.description` | string | Human-readable summary of the proposed fix |
| `fix.changes` | array | Ordered list of changes to apply |
| `fix.changes[].type` | string | Change type: `selector_replace`, `wait_add`, `wait_adjust`, `mock_update`, `assertion_update` |
| `fix.changes[].old_value` | string | The current value to replace |
| `fix.changes[].new_value` | string | The proposed replacement |
| `fix.changes[].confidence` | enum | `high`, `medium`, `low` |
| `fix.changes[].rationale` | string | Why this change is proposed |
| `fix.framework_hint` | string | Framework-specific code suggestion |
| `fix.warnings` | array | Caveats: e.g., "Multiple candidates found; manual review recommended" |
| `fix.recommended_action` | enum | Same as diagnosis: `fix_test`, `fix_code`, `mark_flaky`, `investigate` |

## Requirements

| # | Requirement | Priority |
|---|-------------|----------|
| R1 | `observe({what: "test_diagnosis"})` accepts a failure message and returns a structured diagnosis with category, confidence, evidence, and recommended action | must |
| R2 | Diagnosis correlates the failure against captured console errors, network traffic, and DOM state within the specified time window | must |
| R3 | Diagnosis classifies failures into one of the defined categories: `selector_stale`, `api_contract_changed`, `timing_issue`, `element_removed`, `network_failure`, `js_error`, `true_regression`, `flaky`, `unknown` | must |
| R4 | `generate({format: "test_fix"})` accepts a diagnosis and returns a structured fix proposal with specific changes, confidence, and rationale | must |
| R5 | For `selector_stale` failures, the diagnosis includes candidate selectors found via DOM query with similarity scoring | must |
| R6 | For `api_contract_changed` failures, the diagnosis includes the actual vs expected response shape diff | must |
| R7 | For `timing_issue` failures, the fix proposal includes specific wait condition suggestions (element visible, network idle, custom predicate) | should |
| R8 | Fix proposals include framework-specific hints for Playwright, Cypress, and Puppeteer when the `framework` parameter is provided | should |
| R9 | Diagnosis leverages existing `error_clusters` data when available, to identify whether a failure is part of a known error pattern | should |
| R10 | Diagnosis leverages existing `changes` (causal diffing) data to identify recent DOM or network changes that correlate with the failure | should |
| R11 | Diagnosis leverages DOM fingerprinting data (when available) to find candidate selectors using structural similarity rather than string matching alone | should |
| R12 | When `recommended_action` is `fix_code` (true regression), the response explicitly states this is NOT a test issue and does NOT propose a test fix | must |
| R13 | When evidence is insufficient (no telemetry in window, extension disconnected), diagnosis returns `category: "unknown"` with `confidence: "low"` and a clear explanation of what data is missing | must |
| R14 | The `context.since` parameter defaults to the last 60 seconds if not provided, limiting the telemetry scan window | should |
| R15 | Fix proposals for `api_contract_changed` include mock/fixture update suggestions showing the old and new response shapes | could |
| R16 | Diagnosis includes a `related_changes` array linking to telemetry events that may have caused the failure | could |
| R17 | The `fix_strategy` parameter allows the agent to override the auto-selected fix strategy | could |

## Non-Goals

- **This feature does NOT run tests.** Gasoline observes browser state and generates artifacts. The AI agent (or CI system) is responsible for executing tests and feeding failure output to Gasoline. Gasoline never invokes Playwright, Cypress, or any test runner.

- **This feature does NOT apply fixes automatically.** It generates fix proposals as structured data. The AI agent decides whether to apply them. There is no auto-commit, auto-push, or CI integration loop within Gasoline itself.

- **This feature does NOT create a new MCP tool.** It adds two new modes (`test_diagnosis` and `test_fix`) to the existing `observe` and `generate` tools, respecting the 4-tool constraint.

- **This feature does NOT implement CI/CD pipeline orchestration.** The legacy spec (tech-spec-agentic-cicd.md) describes webhook triggers, GitHub Actions, and Claude Code skills. Those are agent-side orchestration patterns, not Gasoline server features. This spec covers only the Gasoline observation and generation primitives.

- **This feature does NOT support non-browser tests.** Unit tests, integration tests without a browser, and API-only tests are out of scope. Gasoline observes browser telemetry; the diagnosis requires DOM, console, and network data from a browser session.

- **This feature does NOT implement a flaky test database or historical tracking.** The `flaky` category is diagnosed from current-session evidence only (e.g., the test passed on a previous run within the same Gasoline session). Persistent flake tracking across CI runs is a separate feature (see Gasoline CI).

- **Out of scope: Agentic E2E Repair.** That feature (proposed separately) focuses specifically on API contract drift. This spec covers the general self-healing workflow; API contract repair is a specialization that builds on top of this.

## Performance SLOs

| Metric | Target |
|--------|--------|
| `test_diagnosis` response time | < 500ms (correlating against in-memory buffers) |
| `test_fix` response time | < 200ms (generating fix from diagnosis data) |
| Memory impact of diagnosis | < 2MB transient (copies telemetry for analysis, then releases) |
| DOM candidate search | < 100ms (using existing query_dom infrastructure) |

## Security Considerations

- **No new data capture.** This feature reads from existing ring buffers (logs, network, WebSocket, DOM). It does not introduce new capture mechanisms or expand the attack surface.

- **Failure messages may contain sensitive data.** Test output could include API keys, tokens, or PII that leaked into error messages. The diagnosis response must apply the same redaction rules as existing `observe` modes. Redaction patterns configured via `configure({action: "noise_rule"})` apply to diagnosis output.

- **No code execution.** The `test_fix` mode generates text proposals (selectors, wait conditions, mock shapes). It does not execute JavaScript, modify files, or interact with the browser. The AI agent handles code application.

- **Test file paths are informational only.** The `test_file` parameter is used for context in the response (framework hints, selector extraction suggestions). Gasoline does not read or write files on disk.

## Edge Cases

- **No telemetry captured during test window.** Expected behavior: Diagnosis returns `category: "unknown"`, `confidence: "low"`, with `evidence` fields empty and a message explaining that no browser telemetry was found in the specified time window. Suggests the agent verify that Gasoline is capturing (check `observe({what: "page"})` first).

- **Extension disconnected.** Expected behavior: Same as above. The server knows it has no recent data. Diagnosis includes a note that the extension appears disconnected.

- **Multiple matching candidates for a stale selector.** Expected behavior: All candidates are returned in the `candidates` array, sorted by similarity score. The fix proposal uses the highest-scoring candidate but includes a `warning` noting that multiple matches were found and manual review may be needed.

- **Zero candidates for a stale selector.** Expected behavior: Category escalates from `selector_stale` to `element_removed`. The diagnosis notes that no similar element was found in the DOM.

- **Failure message does not match any observed error.** Expected behavior: Diagnosis returns `category: "unknown"` if no console errors, network failures, or DOM issues correlate. This can happen if the test failure occurred before Gasoline started capturing or in a different tab.

- **Very large telemetry window (hours of data).** Expected behavior: The `context.since` parameter bounds the scan. If omitted, the default 60-second window prevents expensive full-buffer scans. The server enforces a maximum scan window (OI-1).

- **Concurrent diagnosis requests.** Expected behavior: Each request operates on a snapshot of current buffer contents. No shared mutable state between diagnosis requests. Thread-safe via existing RWMutex pattern.

- **Diagnosis for a test that uses iframes or shadow DOM.** Expected behavior: DOM candidate search uses the existing `query_dom` infrastructure, which handles iframes and shadow DOM based on the extension's implementation. If the target element is in an iframe the extension cannot access (cross-origin), the diagnosis notes this limitation.

- **Test framework not recognized.** Expected behavior: The `framework` parameter is a hint, not a requirement. If unrecognized or omitted, framework-specific hints in the fix proposal are replaced with generic suggestions (plain CSS selectors, standard wait patterns).

## Dependencies

- **Depends on:**
  - **observe({what: "errors"})** (shipped) -- Console error data for diagnosis.
  - **observe({what: "network_waterfall"})** (shipped) -- Network request/response data for API contract analysis.
  - **observe({what: "changes"})** (shipped) -- Causal diffing data for correlating recent state changes.
  - **observe({what: "error_clusters"})** (shipped) -- Error pattern grouping to identify known vs novel failures.
  - **configure({action: "query_dom"})** (shipped) -- DOM queries for finding candidate selectors.
  - **generate({format: "test"})** (shipped) -- Existing test generation infrastructure (shared code for framework-specific output).
  - **DOM fingerprinting** (in-progress) -- Structural similarity matching for selector candidates. Self-healing tests can ship without this (using string similarity), but quality improves significantly with it.
  - **Causal diffing** (in-progress) -- Identifying what changed between passing and failing runs. Self-healing tests can ship without this, falling back to raw `changes` data.

- **Depended on by:**
  - **Agentic E2E Repair** (proposed) -- Specializes self-healing for API contract drift scenarios.
  - **Agentic CI/CD** (proposed) -- Orchestrates self-healing as part of CI pipeline automation.
  - **Gasoline CI** (proposed) -- Provides the headless capture infrastructure that makes self-healing practical in CI environments.

## Assumptions

- A1: The Gasoline extension is connected and actively capturing telemetry from the tab where the test is running. Without live telemetry, diagnosis quality degrades to `unknown`.
- A2: The test failure occurs within the browser (E2E test). Non-browser failures (unit tests, API tests) cannot be diagnosed because Gasoline has no telemetry for them.
- A3: The AI agent has access to the test failure output (error message, optionally stack trace) and can pass it to `observe({what: "test_diagnosis"})`.
- A4: The browser tab remains open (or its telemetry is still in the ring buffer) at the time the agent calls for diagnosis. Data evicted from the ring buffer is unavailable.
- A5: For selector candidate matching, the current DOM state is representative. If the page has navigated away from the failure state, DOM candidates may not be available.

## Open Items

| # | Item | Status | Notes |
|---|------|--------|-------|
| OI-1 | Maximum telemetry scan window | open | What is the maximum allowed value for `context.since`? A scan across the entire buffer (1000+ entries) is feasible in-memory but could be slow under load. Propose: max 5 minutes, with a recommendation to use the default 60 seconds. |
| OI-2 | Selector similarity algorithm | open | For `selector_stale` candidate matching, should we use (a) Levenshtein distance on selector strings, (b) DOM structural position matching via DOM fingerprinting, or (c) both? Option (c) is best but depends on DOM fingerprinting shipping first. Propose: ship with (a) for v1, upgrade to (c) when DOM fingerprinting is available. |
| OI-3 | Diagnosis caching | open | Should the server cache recent diagnoses? If an agent calls `test_diagnosis` twice with the same failure message within seconds (e.g., retry logic), should it return the cached result? Propose: no cache for v1; diagnosis is fast enough (<500ms) and caching adds staleness risk. |
| OI-4 | Multi-tab test diagnosis | open | If tests run across multiple tabs (e.g., Playwright parallel workers), which tab's telemetry is used? Currently Gasoline tracks a single tab. Multi-tab diagnosis may require tab ID correlation. Propose: use the currently-tracked tab for v1; multi-tab support depends on Gasoline CI's test ID correlation feature. |
| OI-5 | Structured vs natural-language fix proposals | open | Should `test_fix` return structured change objects (as specified above) or natural-language instructions that the AI agent interprets? Structured is more precise but less flexible. Propose: structured for v1 with a `description` field for human-readable context. The AI agent can use either. |
| OI-6 | Integration with `generate({format: "test"})` | open | Should `test_fix` delegate to the existing `test` generation mode for framework-specific output, or should it have its own generation pipeline? Propose: share the framework-specific template infrastructure but keep the entry points separate, since test generation (new test from telemetry) and test fixing (patch to existing test) are different operations. |
| OI-7 | Flaky detection heuristic | open | How does the server determine a test is `flaky` vs `true_regression` from a single failure? Without historical data, the best signal is: (a) no observable browser-side error, (b) no DOM or API changes, (c) timing-related error message. Propose: classify as `flaky` only when the failure message matches known flaky patterns (timeout, intermittent, network error) AND no browser-side evidence supports a different category. Otherwise, use `unknown`. |
