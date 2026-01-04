# Gasoline v5 - Test Generation Feature Specification

## Overview

This feature extends Gasoline from "reproducing bugs" to "generating regression tests" by correlating data across three existing capture buffers (enhanced actions, network bodies, and browser logs) into a unified timeline, and generating Playwright test scripts that include assertions — not just action replay.

---

## Problem Statement

The existing `get_reproduction_script` tool generates replay-only scripts:

```javascript
await page.getByTestId('email').fill('test@example.com');
await page.getByRole('button', { name: 'Login' }).click();
await page.waitForURL('/dashboard');
```

This confirms the user's steps but asserts nothing about correctness. A real regression test needs to verify:

- API responses returned expected status codes
- No console errors occurred during the flow
- Navigation happened as expected
- Response data had the expected structure

Gasoline already captures all of this data. It just doesn't correlate it or include it in generated scripts.

---

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                    Gasoline Server (Go)                           │
│                                                                   │
│  Existing buffers:                                               │
│    enhancedActions []EnhancedAction  (timestamp: int64 ms)       │
│    networkBodies   []NetworkBody     (timestamp: RFC3339 string) │
│    entries         []LogEntry        (ts: RFC3339 string)        │
│                                                                   │
│  New:                                                            │
│    GetSessionTimeline() — correlates buffers by timestamp         │
│    generateTestScript() — enhanced script with assertions         │
│                                                                   │
│  New MCP Tools:                                                  │
│    get_session_timeline — unified timeline of actions+network+logs│
│    generate_test — Playwright test with assertions                │
│                                                                   │
└─────────────────────────────────────────────────────────────────┘
```

No extension changes are needed. All data capture already works.

---

## Feature 1: Session Timeline (`get_session_timeline`)

### Purpose

Returns a unified, timestamp-ordered timeline combining user actions, network requests, and console events. This gives the AI correlated cause-and-effect data: "user clicked login → POST /api/login returned 200 → navigated to /dashboard → no errors".

### MCP Tool Definition

```json
{
  "name": "get_session_timeline",
  "description": "Get a correlated timeline of user actions, network requests, and console events. Useful for understanding cause-and-effect relationships between user interactions and application behavior.",
  "inputSchema": {
    "type": "object",
    "properties": {
      "last_n_actions": {
        "type": "number",
        "description": "Scope timeline to the last N user actions and their consequences (default: all)"
      },
      "url": {
        "type": "string",
        "description": "Filter by URL substring"
      },
      "include": {
        "type": "array",
        "items": { "type": "string", "enum": ["actions", "network", "console"] },
        "description": "What event types to include (default: all)"
      }
    }
  }
}
```

### Response Format

```json
{
  "timeline": [
    {
      "ts": 1705312200000,
      "kind": "action",
      "type": "click",
      "url": "http://localhost:3000/login",
      "selectors": {"testId": "login-btn", "role": {"role": "button", "name": "Log in"}}
    },
    {
      "ts": 1705312200150,
      "kind": "network",
      "method": "POST",
      "url": "/api/login",
      "status": 200,
      "duration": 145,
      "responseShape": {"token": "string", "user": {"id": "number", "name": "string"}}
    },
    {
      "ts": 1705312200300,
      "kind": "action",
      "type": "navigate",
      "fromUrl": "/login",
      "toUrl": "/dashboard"
    },
    {
      "ts": 1705312200500,
      "kind": "network",
      "method": "GET",
      "url": "/api/dashboard",
      "status": 200,
      "duration": 80
    },
    {
      "ts": 1705312201000,
      "kind": "console",
      "level": "error",
      "message": "Failed to load sidebar widget"
    }
  ],
  "summary": {
    "actions": 2,
    "network_requests": 2,
    "console_errors": 1,
    "duration_ms": 1000
  }
}
```

### Timeline Entry Types

#### Action Entry

```json
{
  "ts": 1705312200000,
  "kind": "action",
  "type": "click|input|keypress|navigate|select|scroll",
  "url": "http://localhost:3000/page",
  "selectors": { ... },
  "value": "...",
  "key": "Enter",
  "toUrl": "/next-page"
}
```

Fields are included only when relevant to the action type (same as EnhancedAction).

#### Network Entry

```json
{
  "ts": 1705312200150,
  "kind": "network",
  "method": "POST",
  "url": "/api/endpoint",
  "status": 200,
  "duration": 145,
  "contentType": "application/json",
  "responseShape": { "field": "type", ... }
}
```

The `responseShape` field contains the structural type signature of the response body (field names + types, not values). This enables stable assertions that don't break when dynamic values change.

#### Console Entry

```json
{
  "ts": 1705312201000,
  "kind": "console",
  "level": "error|warn|info",
  "message": "Error message text"
}
```

Only `error` and `warn` levels are included by default (info is too noisy for test assertions).

### Response Shape Extraction

Response bodies are analyzed to extract their structural type signature:

| JSON Value | Shape |
|-----------|-------|
| `"hello"` | `"string"` |
| `42` | `"number"` |
| `true` | `"boolean"` |
| `null` | `"null"` |
| `{"a": 1, "b": "x"}` | `{"a": "number", "b": "string"}` |
| `[{"id": 1}, {"id": 2}]` | `[{"id": "number"}]` |

Example: `{"token":"abc123","user":{"id":5,"name":"Bob"}}` becomes `{"token":"string","user":{"id":"number","name":"string"}}`.

Only JSON responses are shape-extracted. Non-JSON content types get `responseShape: null`.

### Timestamp Normalization

The three data sources use different timestamp formats:

| Source | Format | Normalization |
|--------|--------|---------------|
| EnhancedAction | `int64` (ms since epoch) | Used as-is |
| NetworkBody | RFC3339 string | Parsed to ms since epoch |
| LogEntry | RFC3339 string (`ts` field) | Parsed to ms since epoch |

All timeline entries use `ts` as milliseconds since epoch (int64) for consistent ordering.

### Correlation Algorithm

```
1. Normalize all timestamps to ms epoch
2. Apply URL filter to all sources
3. Merge-sort all entries by timestamp
4. If last_n_actions specified:
   a. Find the Nth-from-last action entry
   b. Use its timestamp as the start boundary
   c. Include all events (network, console) from that point forward
5. Build summary counts
```

### Limits

| Constraint | Limit | Reason |
|-----------|-------|--------|
| Timeline entries | 200 max | Prevent huge responses |
| Response shape depth | 3 levels | Avoid deeply nested type maps |
| Response shape array sampling | First element only | Representative structure |
| Console entries | error + warn only | Info too noisy |
| Output size | 100KB | Cap for large sessions |

---

## Feature 2: Test Generation (`generate_test`)

### Purpose

Generate a complete Playwright test script that includes both action replay AND assertions derived from the correlated timeline. The generated test verifies that the application behaves correctly — not just that it doesn't crash.

### MCP Tool Definition

```json
{
  "name": "generate_test",
  "description": "Generate a Playwright test script from captured user actions with assertions for network responses and error-free execution. Produces a regression test, not just a replay.",
  "inputSchema": {
    "type": "object",
    "properties": {
      "test_name": {
        "type": "string",
        "description": "Name for the generated test (default: derived from actions)"
      },
      "last_n_actions": {
        "type": "number",
        "description": "Use only the last N actions (default: all)"
      },
      "base_url": {
        "type": "string",
        "description": "Replace the origin in URLs (e.g., 'http://localhost:3000')"
      },
      "assert_network": {
        "type": "boolean",
        "description": "Include assertions for network response status codes (default: true)"
      },
      "assert_no_errors": {
        "type": "boolean",
        "description": "Assert no console errors occurred during the flow (default: true)"
      },
      "assert_response_shape": {
        "type": "boolean",
        "description": "Assert response body structure matches captured shape (default: false)"
      }
    }
  }
}
```

### Generated Test Structure

```javascript
import { test, expect } from '@playwright/test';

test('login flow completes successfully', async ({ page }) => {
  // Collect console errors
  const consoleErrors = [];
  page.on('console', msg => {
    if (msg.type() === 'error') consoleErrors.push(msg.text());
  });

  await page.goto('http://localhost:3000/login');

  // Fill email
  await page.getByTestId('email-input').fill('user@example.com');

  // Fill password
  await page.getByTestId('password-input').fill('[user-provided]');

  // Click login button
  const loginResponse = page.waitForResponse(r => r.url().includes('/api/login'));
  await page.getByRole('button', { name: 'Log in' }).click();

  // Assert: POST /api/login responded successfully
  const loginResp = await loginResponse;
  expect(loginResp.status()).toBe(200);

  // Assert: navigated to dashboard
  await expect(page).toHaveURL(/\/dashboard/);

  // Assert: GET /api/dashboard responded successfully
  const dashResp = await page.waitForResponse(r => r.url().includes('/api/dashboard'));
  expect(dashResp.status()).toBe(200);

  // Assert: no console errors during flow
  expect(consoleErrors).toHaveLength(0);
});
```

### Assertion Generation Rules

#### Network Response Assertions

For each network request that occurred between two user actions:

1. Generate a `page.waitForResponse()` call with a URL matcher
2. Assert the status code matches what was captured
3. If `assert_response_shape` is true, add a structural assertion on the response body

```javascript
// Captured: POST /api/login → 200, body: {"token":"abc","user":{"id":1}}
const resp = await page.waitForResponse(r => r.url().includes('/api/login'));
expect(resp.status()).toBe(200);

// With assert_response_shape:
const body = await resp.json();
expect(body).toHaveProperty('token');
expect(body).toHaveProperty('user.id');
```

#### Navigation Assertions

Navigate actions become `toHaveURL` assertions:

```javascript
// Captured: navigate from /login to /dashboard
await expect(page).toHaveURL(/\/dashboard/);
```

#### Console Error Assertions

When `assert_no_errors` is true:

1. A `page.on('console')` listener is added at the start of the test
2. At the end, `expect(consoleErrors).toHaveLength(0)` is asserted
3. If errors WERE captured during the session, they are listed as comments (known failures)

```javascript
// If the captured session had errors:
// Known errors during captured session:
// - "Failed to load sidebar widget"
// expect(consoleErrors).toHaveLength(0); // DISABLED: errors present in captured session
```

#### Response Shape Assertions

When `assert_response_shape` is true, generate `toHaveProperty` assertions for each top-level key in the response shape:

```javascript
const body = await resp.json();
expect(body).toHaveProperty('token');
expect(body).toHaveProperty('user');
expect(body.user).toHaveProperty('id');
expect(body.user).toHaveProperty('name');
```

Only generated for JSON responses (`contentType` contains `application/json`).

### Network-Action Correlation

Network requests are attributed to the preceding user action:

```
Action A (t=1000) ─────────────────── Action B (t=3000)
         │                                    │
         ├── Network req (t=1050) ← caused by A
         ├── Network req (t=1200) ← caused by A
         │                                    │
         └── 2s boundary ─────────────────────┘
```

Rules:
1. A network request belongs to the most recent preceding action
2. The `waitForResponse` is placed BEFORE the action's click/fill (using Playwright's response promise pattern)
3. If multiple network requests are triggered by one action, they are all asserted

### Differences from `get_reproduction_script`

| | `get_reproduction_script` | `generate_test` |
|---|---|---|
| Purpose | Bug reproduction | Regression prevention |
| Network assertions | No | Yes — status codes |
| Error assertions | No | Yes — assert absence |
| Navigation assertions | `waitForURL` only | `expect(page).toHaveURL()` |
| Response shape | No | Optional |
| Console error listener | No | Yes |
| Test naming | "reproduction: {error}" | Custom or derived |
| waitForResponse pattern | No | Yes |

### Sensitive Data Handling

Same rules as `get_reproduction_script`:

| Data Type | Handling |
|-----------|----------|
| Password inputs | Replace with `'[user-provided]'` |
| Redacted values | Replace with `'[user-provided]'` |
| Response bodies in assertions | Shape only (no values) |
| Request bodies | Not included in test |

---

## Implementation Details

### Go Types

```go
// TimelineEntry represents a single entry in the session timeline
type TimelineEntry struct {
    Timestamp     int64                  `json:"ts"`
    Kind          string                 `json:"kind"` // "action", "network", "console"
    // Action fields
    Type          string                 `json:"type,omitempty"`
    URL           string                 `json:"url,omitempty"`
    Selectors     map[string]interface{} `json:"selectors,omitempty"`
    Value         string                 `json:"value,omitempty"`
    Key           string                 `json:"key,omitempty"`
    FromURL       string                 `json:"fromUrl,omitempty"`
    ToURL         string                 `json:"toUrl,omitempty"`
    SelectedValue string                 `json:"selectedValue,omitempty"`
    SelectedText  string                 `json:"selectedText,omitempty"`
    ScrollY       int                    `json:"scrollY,omitempty"`
    // Network fields
    Method        string                 `json:"method,omitempty"`
    Status        int                    `json:"status,omitempty"`
    Duration      int                    `json:"duration,omitempty"`
    ContentType   string                 `json:"contentType,omitempty"`
    ResponseShape interface{}            `json:"responseShape,omitempty"`
    // Console fields
    Level         string                 `json:"level,omitempty"`
    Message       string                 `json:"message,omitempty"`
}

// TimelineSummary provides counts for the timeline
type TimelineSummary struct {
    Actions        int   `json:"actions"`
    NetworkRequests int  `json:"network_requests"`
    ConsoleErrors  int   `json:"console_errors"`
    DurationMs     int64 `json:"duration_ms"`
}

// SessionTimelineResponse is the response for get_session_timeline
type SessionTimelineResponse struct {
    Timeline []TimelineEntry `json:"timeline"`
    Summary  TimelineSummary `json:"summary"`
}

// TimelineFilter defines filtering for get_session_timeline
type TimelineFilter struct {
    LastNActions int
    URLFilter    string
    Include      []string // "actions", "network", "console"
}

// TestGenerationOptions defines options for generate_test
type TestGenerationOptions struct {
    TestName           string
    LastNActions       int
    BaseURL            string
    AssertNetwork      bool
    AssertNoErrors     bool
    AssertResponseShape bool
}
```

### Key Functions

```go
// GetSessionTimeline correlates actions, network bodies, and log entries
func (v *V4Server) GetSessionTimeline(filter TimelineFilter, entries []LogEntry) SessionTimelineResponse

// extractResponseShape extracts structural type signature from a JSON response body
func extractResponseShape(body string) interface{}

// describeShape recursively describes the type structure of a parsed JSON value
func describeShape(v interface{}, depth int) interface{}

// generateTestScript generates a Playwright test with assertions
func generateTestScript(timeline []TimelineEntry, opts TestGenerationOptions) string

// normalizeTimestamp converts RFC3339 string to ms since epoch
func normalizeTimestamp(ts string) int64
```

### Response Shape Implementation

```go
func extractResponseShape(body string) interface{} {
    var parsed interface{}
    if err := json.Unmarshal([]byte(body), &parsed); err != nil {
        return nil
    }
    return describeShape(parsed, 0)
}

func describeShape(v interface{}, depth int) interface{} {
    if depth > 3 {
        return "..."
    }
    switch val := v.(type) {
    case map[string]interface{}:
        shape := map[string]interface{}{}
        for k, v := range val {
            shape[k] = describeShape(v, depth+1)
        }
        return shape
    case []interface{}:
        if len(val) > 0 {
            return []interface{}{describeShape(val[0], depth+1)}
        }
        return []interface{}{}
    case string:
        return "string"
    case float64:
        return "number"
    case bool:
        return "boolean"
    default:
        return "null"
    }
}
```

---

## Performance Budget

| Operation | Budget | Notes |
|-----------|--------|-------|
| Timeline construction | < 10ms | Merge-sort of in-memory buffers |
| Response shape extraction | < 1ms per body | Simple JSON type walk |
| Test script generation | < 50ms | Template expansion |
| Total MCP tool call | < 100ms | Well within MCP response expectations |

---

## Testing Requirements

### Session Timeline

| Test Category | Cases |
|--------------|-------|
| Timestamp normalization | RFC3339 → ms, int64 passthrough, invalid strings |
| Merge ordering | Actions + network interleaved correctly |
| URL filtering | Filters all three sources, partial match |
| last_n_actions scoping | Correct boundary, includes subsequent network/console |
| Include filtering | Only actions, only network, combinations |
| Response shape extraction | Objects, arrays, primitives, nested, invalid JSON, depth limit |
| Summary counts | Correct tallies for each kind |
| Empty buffers | No actions, no network, no logs |
| Max entries cap | Truncation at 200 entries |

### Test Generation

| Test Category | Cases |
|--------------|-------|
| Basic script structure | Imports, test wrapper, goto, console listener |
| Network assertions | waitForResponse pattern, status assertion, URL matcher |
| Navigation assertions | toHaveURL with regex |
| Console error assertions | Listener setup, final assertion, disabled when errors present |
| Response shape assertions | toHaveProperty for each key, nested properties |
| Network-action correlation | Requests attributed to correct preceding action |
| Multiple requests per action | All asserted in sequence |
| Options: assert_network=false | No network assertions generated |
| Options: assert_no_errors=false | No console listener/assertion |
| Options: assert_response_shape=true | Shape assertions included |
| Options: base_url | Origins replaced throughout |
| Options: test_name | Custom name in test() call |
| Sensitive data | Passwords redacted, shape-only for responses |
| Edge cases | No actions, no network, single action, 50 actions |

---

## Files to Change

| File | Changes |
|------|---------|
| `cmd/dev-console/v4.go` | Add types, `GetSessionTimeline()`, `extractResponseShape()`, `generateTestScript()`, MCP tool definitions + handlers |
| `cmd/dev-console/v4_test.go` | Tests for all new functions |

No extension changes needed.

---

## Backward Compatibility

- `get_reproduction_script` remains unchanged — existing behavior preserved
- `generate_test` is a new tool, no conflicts
- `get_session_timeline` is a new tool, no conflicts
- The V4Server gains a new method but existing methods are unchanged
- `GetSessionTimeline` requires access to log entries (passed from Server), which establishes a new cross-reference between v3 and v4 data — but only at read time, not storage
