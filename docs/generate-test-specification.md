# `generate_test` — Regression Test Generation Tool

## Status: Specification Draft

---

## Problem Statement

The METR study (July 2025) found AI-assisted coding makes experienced developers **19% slower** in complex codebases — largely from context-switching, reviewing AI output, and debugging. Vibe-coded applications compound this: 45% of AI-generated code contains security vulnerabilities (Veracode 2025), and developers do not write tests for code they do not understand.

The result: a generation of applications with zero automated tests, no regression safety net, and production failures from unvalidated AI output.

**Gasoline is uniquely positioned** to solve this because it already passively observes what happens in the browser during development. No other tool combines:
- Real-time console/error capture
- Network request/response bodies
- WebSocket event streams
- User action recording with multi-strategy selectors
- Correlated session timeline

The `generate_test` tool transforms this passive observation into assertion-rich Playwright regression tests — capturing not just *what the user did*, but *what correct behavior looked like*.

---

## Design Philosophy

### Not a Recorder — A Witness

Traditional test recorders (Playwright Codegen, Cypress Studio) capture user actions and replay them. They answer: *"did the user click this button?"*

`generate_test` answers a harder question: *"when the user clicked this button, the network returned a 200 with valid JSON, the console stayed clean, and the DOM updated correctly — does it still?"*

### Assertion Density Over Action Replay

The existing `get_reproduction_script` generates minimal Playwright tests for bug reproduction. `generate_test` inverts the priority:

| | `get_reproduction_script` | `generate_test` |
|---|---|---|
| **Purpose** | Reproduce a bug | Prevent regressions |
| **Actions** | Primary output | Scaffolding for assertions |
| **Network** | Not captured | Assert status, shape, timing |
| **Console** | Error message comment | Assert absence of errors |
| **DOM** | Not captured | Assert expected content |
| **WebSocket** | Not captured | Assert message patterns |

---

## MCP Tool Interface

### Tool Name
`generate_test`

### Parameters

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `test_name` | string | No | Auto-generated from URL/actions | Descriptive test name |
| `last_n_actions` | int | No | All captured | Limit to last N actions |
| `base_url` | string | No | Original URL | Replace origin for portability |
| `framework` | string | No | `"playwright"` | Target framework: `playwright` or `cypress` |
| `assertions` | object | No | All enabled | Fine-grained assertion control (see below) |
| `url_filter` | string | No | None | Only include actions/events from this URL pattern |
| `style` | string | No | `"comprehensive"` | `"comprehensive"`, `"smoke"`, or `"minimal"` |

### `assertions` Object

```json
{
  "network": {
    "enabled": true,
    "status_codes": true,
    "response_shape": true,
    "timing_budget_ms": null,
    "exclude_urls": ["analytics", "tracking"]
  },
  "console": {
    "enabled": true,
    "fail_on_errors": true,
    "fail_on_warnings": false,
    "ignore_patterns": []
  },
  "dom": {
    "enabled": true,
    "visibility": true,
    "text_content": true,
    "element_count": false
  },
  "websocket": {
    "enabled": true,
    "connection_lifecycle": true,
    "message_shape": true
  }
}
```

### Output

Returns a complete, runnable test file as a string. Framework-specific (Playwright or Cypress).

---

## Implementation Architecture

### Data Sources

`generate_test` correlates data from four server-side buffers:

```
┌──────────────────────────────────────────────────┐
│                Session Timeline                   │
│                                                   │
│  Action ──→ Network Req ──→ Network Resp          │
│    │              │               │               │
│    ▼              ▼               ▼               │
│  Click        POST /api/users   200 + JSON body   │
│  button#save  Content-Type:     { id: 1, ...}    │
│               application/json                    │
│                                                   │
│  Console: [clean — no new errors]                 │
│  WebSocket: [no relevant messages]                │
│  DOM: button text changed to "Saved"              │
└──────────────────────────────────────────────────┘
```

**Correlation strategy:**
1. Build timeline from `get_session_timeline` logic
2. For each action, identify network requests that occurred within a **2-second causal window** after the action
3. For each action, identify console entries within the same window
4. For each action, identify WebSocket messages within the same window
5. Use DOM state captured at action time (selectors, text content, visibility)

### Causal Window

The **causal window** determines which network/console/WebSocket events are attributed to a user action:

- **Start:** Action timestamp
- **End:** Next action timestamp OR action timestamp + 2000ms (whichever is shorter)
- **Rationale:** Most UI interactions trigger network calls that resolve within 2s. Anything beyond likely belongs to a different interaction.

### Test Generation Pipeline

```
1. Retrieve enhanced actions (filtered by last_n_actions, url_filter)
2. Retrieve network bodies for the session period
3. Retrieve console logs for the session period
4. Retrieve WebSocket events for the session period
5. Build correlated timeline (action → caused effects)
6. For each action:
   a. Generate action code (click, fill, navigate, etc.)
   b. Generate network assertions (if requests occurred in causal window)
   c. Generate console assertions (assert no new errors)
   d. Generate DOM assertions (element state after action)
   e. Generate WebSocket assertions (if messages in window)
7. Wrap in test framework boilerplate
8. Apply style filter (comprehensive/smoke/minimal)
```

---

## Output Format: Playwright

### Comprehensive Style

```typescript
import { test, expect } from '@playwright/test';

test('user creates a new project and verifies dashboard update', async ({ page }) => {
  // Console error tracking
  const consoleErrors: string[] = [];
  page.on('console', msg => {
    if (msg.type() === 'error') consoleErrors.push(msg.text());
  });

  // Navigate to starting page
  await page.goto('http://localhost:3000/dashboard');

  // Step 1: Click "New Project" button
  const createResponse = page.waitForResponse(
    resp => resp.url().includes('/api/projects') && resp.request().method() === 'POST'
  );
  await page.getByRole('button', { name: 'New Project' }).click();

  // Assert: Network response
  const resp = await createResponse;
  expect(resp.status()).toBe(201);
  const body = await resp.json();
  expect(body).toHaveProperty('id');
  expect(body).toHaveProperty('name');
  expect(body).toHaveProperty('createdAt');

  // Assert: DOM updated
  await expect(page.getByText('Project created')).toBeVisible();

  // Step 2: Fill project name
  await page.getByLabel('Project name').fill('My New Project');

  // Step 3: Click save
  const saveResponse = page.waitForResponse(
    resp => resp.url().includes('/api/projects/') && resp.request().method() === 'PUT'
  );
  await page.getByRole('button', { name: 'Save' }).click();

  const saveResp = await saveResponse;
  expect(saveResp.status()).toBe(200);

  // Assert: Navigation occurred
  await page.waitForURL('**/projects/*');

  // Assert: No console errors during test
  expect(consoleErrors).toEqual([]);
});
```

### Smoke Style

Reduces to status code checks and critical DOM assertions only:

```typescript
test('user creates project - smoke', async ({ page }) => {
  await page.goto('http://localhost:3000/dashboard');

  await page.getByRole('button', { name: 'New Project' }).click();
  await expect(page.getByText('Project created')).toBeVisible();

  await page.getByLabel('Project name').fill('My New Project');
  await page.getByRole('button', { name: 'Save' }).click();
  await page.waitForURL('**/projects/*');
});
```

### Minimal Style

Actions only, no assertions (equivalent to `get_reproduction_script` but with the newer pipeline):

```typescript
test('user creates project - actions only', async ({ page }) => {
  await page.goto('http://localhost:3000/dashboard');
  await page.getByRole('button', { name: 'New Project' }).click();
  await page.getByLabel('Project name').fill('My New Project');
  await page.getByRole('button', { name: 'Save' }).click();
});
```

---

## Output Format: Cypress

### Comprehensive Style

```javascript
describe('user creates a new project and verifies dashboard update', () => {
  it('creates project and updates dashboard', () => {
    // Console error tracking
    const consoleErrors = [];
    cy.on('window:before:load', (win) => {
      cy.stub(win.console, 'error').callsFake((...args) => {
        consoleErrors.push(args.join(' '));
      });
    });

    cy.visit('http://localhost:3000/dashboard');

    // Step 1: Click "New Project" button
    cy.intercept('POST', '/api/projects').as('createProject');
    cy.contains('button', 'New Project').click();

    // Assert: Network response
    cy.wait('@createProject').then((interception) => {
      expect(interception.response.statusCode).to.equal(201);
      expect(interception.response.body).to.have.property('id');
      expect(interception.response.body).to.have.property('name');
      expect(interception.response.body).to.have.property('createdAt');
    });

    // Assert: DOM updated
    cy.contains('Project created').should('be.visible');

    // Step 2: Fill project name
    cy.get('[aria-label="Project name"]').type('My New Project');

    // Step 3: Click save
    cy.intercept('PUT', '/api/projects/*').as('saveProject');
    cy.contains('button', 'Save').click();
    cy.wait('@saveProject').its('response.statusCode').should('equal', 200);

    // Assert: Navigation
    cy.url().should('include', '/projects/');

    // Assert: No console errors
    cy.then(() => {
      expect(consoleErrors).to.have.length(0);
    });
  });
});
```

---

## Assertion Strategies

### Network Assertions

**What gets asserted:**
- HTTP status code matches captured value
- Response body shape (top-level property names from JSON responses)
- Request method matches
- Optional: response time within budget (if `timing_budget_ms` set)

**What gets excluded:**
- URLs matching `assertions.network.exclude_urls` patterns
- Requests to common analytics/tracking endpoints (auto-detected)
- Responses with no body or non-JSON content types

**Shape extraction logic:**
```
For a captured response body like:
{
  "id": 42,
  "name": "Test Project",
  "members": [{"userId": 1, "role": "admin"}],
  "settings": {"theme": "dark", "notifications": true}
}

Generated assertions:
expect(body).toHaveProperty('id');
expect(body).toHaveProperty('name');
expect(body).toHaveProperty('members');
expect(body).toHaveProperty('settings');
```

Only top-level properties are asserted by default. Nested shape assertions are opt-in via `assertions.network.response_shape_depth` (future parameter).

### Console Assertions

**Strategy:** Assert that no *new* console errors appeared during the test.

```typescript
// Setup: Capture errors
const consoleErrors: string[] = [];
page.on('console', msg => {
  if (msg.type() === 'error') consoleErrors.push(msg.text());
});

// Teardown: Assert clean console
expect(consoleErrors).toEqual([]);
```

**Ignore patterns:** Common noise filtered out:
- `assertions.console.ignore_patterns` (user-specified regex array)
- Auto-excluded: React DevTools warnings, browser extension errors, favicon 404s

### DOM Assertions

**What gets asserted (derived from action context):**
- Element visibility after action (the target element or a success indicator)
- Text content changes (if text changed in the causal window)
- Navigation (URL changes)

**What is NOT asserted:**
- Pixel-perfect layout (that's visual regression, not functional testing)
- Element count (disabled by default, opt-in)
- CSS properties

### WebSocket Assertions

**What gets asserted:**
- Connection established (if a new WS connection opened in causal window)
- Message received with expected shape (top-level keys of JSON messages)
- Connection lifecycle (open → messages → close)

**Generated code:**

```typescript
// Assert: WebSocket connection and message
const wsPromise = page.waitForEvent('websocket');
await page.getByRole('button', { name: 'Connect' }).click();
const ws = await wsPromise;
expect(ws.url()).toContain('/ws/events');

const msgPromise = ws.waitForEvent('framereceived');
const frame = await msgPromise;
const data = JSON.parse(frame.payload.toString());
expect(data).toHaveProperty('type');
expect(data).toHaveProperty('payload');
```

---

## Style Modes

| Style | Actions | Network Status | Response Shape | Console Clean | DOM | WebSocket | Use Case |
|-------|---------|---------------|----------------|---------------|-----|-----------|----------|
| `comprehensive` | All | Yes | Yes | Yes | Yes | Yes | Full regression coverage |
| `smoke` | All | No | No | No | Critical only | No | Quick sanity check |
| `minimal` | All | No | No | No | No | No | Action replay only |

---

## Edge Cases & Limitations

### Buffer Overflow
- Enhanced actions buffer: 50 actions max. If the session exceeds this, only the most recent 50 are available.
- Network bodies: 100 entries max. Long sessions may lose early requests.
- **Mitigation:** Test name includes a warning comment if buffer overflow is detected.

### Timing Sensitivity
- Actions that trigger async responses beyond 2s may not correlate correctly.
- **Mitigation:** The causal window is configurable in future iterations. For now, a comment notes uncorrelated network activity.

### Dynamic Content
- UUIDs, timestamps, and random values in responses will cause assertion failures on replay.
- **Mitigation:** The response shape strategy asserts *property existence*, not *property values*. Status codes and shapes are stable; values are not.

### Authentication
- Auth tokens are stripped from captured headers (existing security behavior).
- Generated tests assume the test environment handles auth separately (via `storageState` in Playwright or `cy.session()` in Cypress).
- **Mitigation:** Generated test includes a comment about auth setup if auth-related network requests are detected.

### Single-Page Applications
- SPA navigation may not trigger `page.waitForURL` correctly.
- **Mitigation:** Use `page.waitForURL` only when the URL actually changed in captured actions. Otherwise, assert DOM changes.

### Password Fields
- Already redacted as `[redacted]` in capture. Generated test uses `'test-password'` placeholder with a comment.

---

## Performance Budget

| Operation | Budget | Rationale |
|-----------|--------|-----------|
| Timeline correlation | < 50ms | Linear scan of sorted buffers |
| Script generation | < 100ms | String building, no complex computation |
| Total tool response | < 200ms | Including JSON serialization |
| Output size | < 50KB | Truncate with warning if exceeded |

---

## Server-Side Implementation

### New/Modified Files

| File | Changes |
|------|---------|
| `cmd/dev-console/v4.go` | Replace existing `generate_test` stub with full implementation |
| `cmd/dev-console/v4_test.go` | TDD test cases for all assertion strategies |

### Core Functions

```go
// Entry point — MCP tool handler
func (h *MCPHandlerV4) toolGenerateTest(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse

// Timeline correlation
func correlateSessionData(actions []EnhancedAction, network []NetworkBody,
    console []LogEntry, ws []WSEvent, windowMs int) []CorrelatedStep

// Framework-specific generators
func generatePlaywrightTest(steps []CorrelatedStep, opts GenerateTestOpts) string
func generateCypressTest(steps []CorrelatedStep, opts GenerateTestOpts) string

// Assertion generators (per-step)
func generateNetworkAssertions(req NetworkBody, framework string) string
func generateConsoleAssertions(framework string) string
func generateDOMAssertions(action EnhancedAction, framework string) string
func generateWSAssertions(events []WSEvent, framework string) string

// Helpers
func extractResponseShape(body string) map[string]string  // property → type
func shouldExcludeURL(url string, patterns []string) bool
func autoDetectAnalytics(url string) bool
```

### Types

```go
type GenerateTestOpts struct {
    TestName    string
    BaseURL     string
    Framework   string   // "playwright" or "cypress"
    Style       string   // "comprehensive", "smoke", "minimal"
    Assertions  AssertionConfig
    URLFilter   string
    LastN       int
}

type AssertionConfig struct {
    Network   NetworkAssertionConfig
    Console   ConsoleAssertionConfig
    DOM       DOMAssertionConfig
    WebSocket WSAssertionConfig
}

type NetworkAssertionConfig struct {
    Enabled        bool
    StatusCodes    bool
    ResponseShape  bool
    TimingBudgetMs *int
    ExcludeURLs    []string
}

type ConsoleAssertionConfig struct {
    Enabled        bool
    FailOnErrors   bool
    FailOnWarnings bool
    IgnorePatterns []string
}

type DOMAssertionConfig struct {
    Enabled      bool
    Visibility   bool
    TextContent  bool
    ElementCount bool
}

type WSAssertionConfig struct {
    Enabled             bool
    ConnectionLifecycle bool
    MessageShape        bool
}

type CorrelatedStep struct {
    Action        EnhancedAction
    NetworkReqs   []NetworkBody    // Requests in causal window
    ConsoleEntries []LogEntry      // Console entries in causal window
    WSEvents      []WSEvent        // WebSocket events in causal window
    PauseBeforeMs int64            // Gap since previous action
}
```

---

## Extension Changes

**None required.** All data is already captured by the existing v4/v5 extension infrastructure. The `generate_test` tool operates entirely on server-side buffers.

---

## Future Iterations

### v1 (This Spec)
- Playwright + Cypress output
- Network status + shape assertions
- Console error assertions
- DOM visibility assertions
- WebSocket lifecycle assertions
- Three style modes

### v2 (Planned)
- `response_shape_depth` parameter for nested JSON assertions
- Visual snapshot integration (Playwright `toHaveScreenshot()`)
- Custom assertion injection via template strings
- Multi-tab/window support
- Test file organization hints (suggest file paths based on project structure)

### v3 (Planned)
- AI-enhanced assertion selection (use Claude to determine which assertions matter most)
- Flakiness detection (identify timing-sensitive steps and add appropriate waits)
- Parameterized test generation (data-driven from multiple captured sessions)
- Integration test generation (combine multiple user flows into a single suite)

---

## Competitive Positioning

See [competitive-analysis.md](./competitive-analysis.md) for full analysis.

### Gasoline's Unique Advantages

1. **Passive capture** — No explicit recording session needed. Tests generate from what was already observed during development.
2. **Full-stack assertions** — Network bodies + console + DOM + WebSocket in one test. No competitor combines all four.
3. **MCP-native** — Integrates directly into AI coding workflows. The AI assistant that wrote the code can also write the tests from the same session.
4. **Zero cost** — Free, open-source, no API token costs per test run (unlike Shortest).
5. **Portable output** — Generates standard Playwright/Cypress files. No vendor lock-in.
6. **Privacy-first** — All data stays on localhost. No cloud recording, no third-party JS snippets.
7. **Developer-owned** — Tests are source code in your repo, not hosted on a platform.

### What Competitors Cannot Do

| Capability | Gasoline | Playwright Codegen | Meticulous | QA Wolf | Shortest |
|-----------|----------|-------------------|------------|---------|----------|
| Passive session capture | Yes | No (explicit record) | Yes (snippet) | No (managed) | No |
| Network body assertions | Yes | No | Visual only | Yes | No |
| Console error assertions | Yes | No | No | No | No |
| WebSocket assertions | Yes | No | No | No | No |
| MCP integration | Native | Via MCP server | No | No | No |
| No per-run cost | Yes | Yes | No | No | No (API tokens) |
| Portable code output | Yes | Yes | No | Yes | No (NL prompts) |
| Works during development | Yes | No (separate step) | Staging/prod | No | No |
| Privacy (localhost only) | Yes | Yes | No (cloud) | No (cloud) | No (API) |
