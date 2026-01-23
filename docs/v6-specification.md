# Gasoline v6 - Technical Specification

## Overview

v6 transforms Gasoline from "browser state reporter" to "AI debugging co-pilot" by adding analysis, verification, and proactive intelligence layers on top of existing capture infrastructure.

**Core thesis:** AI coding assistants spend most of their time in a debugging loop — make change, ask user what happened, interpret vague descriptions, repeat. v6 closes this loop by giving the AI structured before/after comparisons, security awareness, API contract monitoring, and proactive notifications.

### New MCP Tools

| Tool | Type | Description |
|------|------|-------------|
| `security_audit` | Analysis | Detect exposed credentials, missing auth, PII leaks, insecure transport |
| `verify_fix` | Verification | Start a verification session, compare before/after browser state |
| `diff_sessions` | Comparison | Compare two named session snapshots for regressions |
| `validate_api` | Analysis | Detect API response shape changes and contract violations |

### Enhanced Existing Tools

| Tool | Enhancement |
|------|-------------|
| `generate_test` | DOM state assertions, API fixtures, visual snapshots (see `generate-test-v2.md`) |
| `check_performance` | Baseline comparison, regression detection (see `performance-budget-spec.md`) |

### New Passive Capability

| Feature | Description |
|---------|-------------|
| Context Streaming | Proactive push of relevant browser events to the AI via MCP notifications |

---

## Architecture Changes

```
┌─────────────────────────────────────────────────────────────────┐
│                         Browser                                  │
│  ┌─────────────────────────────────────────────────────────────┐│
│  │                    Inject Script (v6)                        ││
│  │                                                              ││
│  │  Existing captures (unchanged):                             ││
│  │    console, network, WebSocket, DOM, actions, performance   ││
│  │                                                              ││
│  │  New (passive, zero-cost):                                  ││
│  │    Context event stream (significant events only)            ││
│  └─────────────────────────────────────────────────────────────┘│
└─────────────────────────────────────────────────────────────────┘
                     │ HTTP
                     ▼
┌─────────────────────────────────────────────────────────────────┐
│                    Gasoline Server (Go)                           │
│                                                                   │
│  Analysis layer (NEW — operates on existing buffers):            │
│    security_audit   — scan network bodies + logs for secrets     │
│    validate_api     — detect response shape changes              │
│                                                                   │
│  Verification layer (NEW — session state management):            │
│    verify_fix       — before/after session comparison            │
│    diff_sessions    — named snapshot comparison                  │
│                                                                   │
│  Context streaming (NEW — push-based notifications):             │
│    SSE endpoint for proactive event delivery                     │
│                                                                   │
│  Existing (enhanced):                                            │
│    generate_test    — DOM assertions, fixtures (generate-test-v2)│
│    check_performance — baseline regression (performance-budget)  │
│                                                                   │
└─────────────────────────────────────────────────────────────────┘
```

### Design Principle: Analysis, Not Capture

v6 adds **zero new capture mechanisms** to the extension. All new tools operate as analysis layers over data that Gasoline already captures:

| Tool | Data Sources |
|------|-------------|
| `security_audit` | `networkBodies`, `entries` (console logs), `enhancedActions` |
| `verify_fix` | All existing buffers (snapshot + diff) |
| `diff_sessions` | All existing buffers (snapshot + diff) |
| `validate_api` | `networkBodies` |
| Context streaming | All existing buffers (filtered) |

This means:
- No new extension permissions required
- No new performance overhead on the page
- No new message types between extension and server
- All features work with the existing extension version

---

## Feature 1: Security Scanner (`security_audit`)

### Purpose

Detect security vulnerabilities that are visible in browser traffic. The Lovable CVE (exposed API keys in client-side code) and industry data showing 40% of AI-generated apps leak credentials demonstrate clear need for automated detection during development.

This is NOT a penetration testing tool. It analyzes data Gasoline already captures and flags patterns that indicate security issues the developer should fix.

### MCP Tool Definition

```json
{
  "name": "security_audit",
  "description": "Scan captured network traffic and console logs for security issues: exposed credentials, missing authentication, PII in logs, and insecure transport. Analyzes data already captured by Gasoline — no additional browser access needed.",
  "inputSchema": {
    "type": "object",
    "properties": {
      "checks": {
        "type": "array",
        "items": {
          "type": "string",
          "enum": ["credentials", "auth_patterns", "pii_exposure", "transport"]
        },
        "description": "Which checks to run (default: all)"
      },
      "url_filter": {
        "type": "string",
        "description": "Only analyze traffic to/from URLs matching this substring"
      },
      "severity_min": {
        "type": "string",
        "enum": ["info", "warning", "critical"],
        "description": "Minimum severity to report (default: warning)"
      }
    }
  }
}
```

### Response Format

```json
{
  "findings": [
    {
      "severity": "critical",
      "category": "credentials",
      "title": "API key exposed in URL query parameter",
      "description": "GET request to /api/data includes API key as query parameter 'api_key'. Query parameters are logged in server access logs, browser history, and may be cached by proxies.",
      "evidence": {
        "method": "GET",
        "url": "/api/data?api_key=sk-proj-abc...xyz",
        "field": "query_param",
        "param_name": "api_key",
        "redacted_value": "sk-proj-abc...***"
      },
      "recommendation": "Move API key to Authorization header or request body. Never include secrets in URLs.",
      "cwe": "CWE-598"
    },
    {
      "severity": "warning",
      "category": "auth_patterns",
      "title": "Endpoint returns user data without authentication",
      "description": "GET /api/users/profile returned user PII (email, name) but no Authorization header was present in the request.",
      "evidence": {
        "method": "GET",
        "url": "/api/users/profile",
        "status": 200,
        "auth_header_present": false,
        "response_contains_pii": true,
        "pii_fields": ["email", "phone"]
      },
      "recommendation": "Ensure this endpoint requires authentication. If public by design, verify no sensitive data is exposed."
    },
    {
      "severity": "critical",
      "category": "pii_exposure",
      "title": "Auth token logged to console",
      "description": "console.log() call includes what appears to be a JWT token (eyJ... pattern).",
      "evidence": {
        "type": "console",
        "level": "log",
        "pattern": "jwt_token",
        "redacted_preview": "Token: eyJhbGciOiJIUzI1...***",
        "source": "auth.js:45"
      },
      "recommendation": "Remove console.log statements that output tokens. Use structured logging that redacts sensitive fields."
    },
    {
      "severity": "warning",
      "category": "transport",
      "title": "Mixed content: HTTPS page loading HTTP resource",
      "description": "Page at https://app.example.com loads a script from http://cdn.example.com/lib.js",
      "evidence": {
        "page_url": "https://app.example.com/dashboard",
        "resource_url": "http://cdn.example.com/lib.js",
        "resource_type": "script"
      },
      "recommendation": "Use HTTPS for all resources. Mixed content can be intercepted by network attackers.",
      "cwe": "CWE-319"
    }
  ],
  "summary": {
    "total": 4,
    "critical": 2,
    "warning": 2,
    "info": 0,
    "scanned": {
      "network_requests": 47,
      "console_entries": 156,
      "unique_endpoints": 12
    }
  }
}
```

### Check Categories

#### 1. Credential Exposure (`credentials`)

Scan network bodies and URLs for patterns that indicate exposed secrets.

**Detection patterns:**

| Pattern | Regex | Severity | Description |
|---------|-------|----------|-------------|
| API key in URL | `[?&](api_key\|apikey\|key\|token\|secret\|password)=` (case-insensitive) | critical | Secrets in query params are logged everywhere |
| AWS access key | `AKIA[0-9A-Z]{16}` | critical | AWS access key ID format |
| AWS secret key | `[A-Za-z0-9/+=]{40}` (near "aws" or "secret") | critical | AWS secret access key |
| Generic secret in URL | `[?&]\w*(secret\|password\|passwd\|token)\w*=[^&]{8,}` | critical | Any long value in a secret-named param |
| Bearer token in URL | `[?&](auth\|bearer\|token)=[A-Za-z0-9._-]{20,}` | critical | Auth tokens in URLs |
| Private key material | `-----BEGIN (RSA\|DSA\|EC\|OPENSSH) PRIVATE KEY-----` | critical | Private key in body |
| GitHub token | `gh[ps]_[A-Za-z0-9_]{36,}` | critical | GitHub personal access token |
| Stripe key | `sk_(test\|live)_[A-Za-z0-9]{24,}` | critical | Stripe secret key |
| JWT in request body | `eyJ[A-Za-z0-9_-]*\.eyJ[A-Za-z0-9_-]*\.[A-Za-z0-9_-]*` | warning | JWT in unexpected location |
| API key in request body | `"(api_key\|apiKey\|api_secret)":\s*"[^"]{8,}"` | warning | Key in JSON body (may be intentional) |

**Scan scope:**
- Request URLs (including query params)
- Request bodies
- Response bodies (to detect APIs that echo secrets)
- Console log arguments

**NOT scanned** (already stripped by Gasoline):
- Authorization headers (sanitized at capture)
- Cookie values (sanitized at capture)

#### 2. Missing Auth Patterns (`auth_patterns`)

Identify endpoints that return sensitive data without authentication headers present in the request.

**Algorithm:**

```
1. For each captured network request/response:
   a. Check if response contains PII patterns (see PII detection below)
   b. Check if request included an Authorization header
      (Note: Gasoline strips auth header VALUES but records presence via a flag)
   c. If response has PII AND no auth header was present → flag

2. Additional check: look for endpoints that returned 200 with user data
   but were never called with auth headers during the session
```

**Auth header presence detection:**

The existing network body capture strips Authorization header values but we need to know IF one was present. Extension change: include a boolean flag `hasAuthHeader: true/false` in network body entries.

```javascript
// In inject.js fetch wrapper:
emit('network:body', {
  url, method, status,
  hasAuthHeader: !!(init?.headers?.['Authorization'] ||
                    init?.headers?.['authorization'] ||
                    (init?.headers instanceof Headers && init.headers.has('authorization'))),
  // ... existing fields
});
```

**PII field detection** (used by both `auth_patterns` and `pii_exposure`):

| Pattern | Regex/Heuristic | Description |
|---------|----------------|-------------|
| Email | `[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}` | Email addresses |
| Phone | `\+?[1-9]\d{1,14}` or `\(\d{3}\)\s?\d{3}-\d{4}` | Phone numbers |
| SSN | `\d{3}-\d{2}-\d{4}` | US Social Security Number |
| Credit card | `\b\d{4}[\s-]?\d{4}[\s-]?\d{4}[\s-]?\d{4}\b` | Card numbers |
| JWT | `eyJ[A-Za-z0-9_-]*\.eyJ[A-Za-z0-9_-]*\.[A-Za-z0-9_-]*` | JSON Web Tokens |
| PII JSON fields | Response body contains keys matching `email\|phone\|ssn\|social_security\|credit_card\|password\|address\|date_of_birth\|dob` | Sensitive field names |

#### 3. Client-Side Data Exposure (`pii_exposure`)

Detect sensitive data appearing in console logs or in responses to unauthenticated endpoints.

**Console log scanning:**

```
For each console entry (level: log, info, debug, warn, error):
  1. Check for JWT patterns → critical (tokens should never be logged)
  2. Check for API key patterns → critical
  3. Check for email patterns → info (might be intentional debug)
  4. Check for password-related strings → warning
  5. Check for "Bearer " followed by token → critical
```

**Response scanning:**

```
For each network response body:
  1. Parse as JSON (skip non-JSON)
  2. Walk keys recursively (max depth 5)
  3. Flag any key matching PII field patterns
  4. Cross-reference with auth presence:
     - PII in authenticated response → info (expected)
     - PII in unauthenticated response → warning
     - Tokens/passwords in any response → critical
```

#### 4. Insecure Transport (`transport`)

Detect HTTP usage and mixed content issues.

**Checks:**

| Check | Severity | Condition |
|-------|----------|-----------|
| HTTP API call | warning | Request URL uses `http://` and isn't localhost |
| Mixed content (script) | critical | HTTPS page loads HTTP script |
| Mixed content (other) | warning | HTTPS page loads HTTP image/style/font |
| WebSocket insecure | warning | `ws://` connection (not `wss://`) to non-localhost |

**Implementation:**

```go
func checkTransport(bodies []NetworkBody, wsEvents []WebSocketEvent) []Finding {
    findings := []Finding{}

    for _, body := range bodies {
        if isHTTP(body.URL) && !isLocalhost(body.URL) {
            findings = append(findings, Finding{
                Severity: "warning",
                Category: "transport",
                Title: "API call over unencrypted HTTP",
                // ...
            })
        }
    }

    // Check for mixed content: page URL is HTTPS but resource URLs are HTTP
    for _, body := range bodies {
        if isHTTPS(body.PageURL) && isHTTP(body.URL) && !isLocalhost(body.URL) {
            severity := "warning"
            if body.ContentType == "application/javascript" || body.ContentType == "text/javascript" {
                severity = "critical"
            }
            findings = append(findings, Finding{
                Severity: severity,
                Category: "transport",
                Title: "Mixed content: HTTPS page loading HTTP resource",
                // ...
            })
        }
    }

    return findings
}
```

### Security Audit Implementation

#### Go Types

```go
// SecurityFinding represents a single security issue found
type SecurityFinding struct {
    Severity       string            `json:"severity"` // "critical", "warning", "info"
    Category       string            `json:"category"` // "credentials", "auth_patterns", "pii_exposure", "transport"
    Title          string            `json:"title"`
    Description    string            `json:"description"`
    Evidence       map[string]interface{} `json:"evidence"`
    Recommendation string            `json:"recommendation"`
    CWE            string            `json:"cwe,omitempty"`
}

// SecurityAuditResponse is the MCP tool response
type SecurityAuditResponse struct {
    Findings []SecurityFinding `json:"findings"`
    Summary  SecuritySummary   `json:"summary"`
}

// SecuritySummary provides counts
type SecuritySummary struct {
    Total    int          `json:"total"`
    Critical int          `json:"critical"`
    Warning  int          `json:"warning"`
    Info     int          `json:"info"`
    Scanned  ScannedScope `json:"scanned"`
}

// ScannedScope shows what was analyzed
type ScannedScope struct {
    NetworkRequests int `json:"network_requests"`
    ConsoleEntries  int `json:"console_entries"`
    UniqueEndpoints int `json:"unique_endpoints"`
}

// SecurityAuditFilter defines which checks to run
type SecurityAuditFilter struct {
    Checks      []string // "credentials", "auth_patterns", "pii_exposure", "transport"
    URLFilter   string
    SeverityMin string // "info", "warning", "critical"
}
```

#### Key Functions

```go
// RunSecurityAudit performs all requested security checks
func (v *V4Server) RunSecurityAudit(filter SecurityAuditFilter, entries []LogEntry) SecurityAuditResponse

// checkCredentialExposure scans for secrets in URLs, bodies, and logs
func checkCredentialExposure(bodies []NetworkBody, entries []LogEntry, urlFilter string) []SecurityFinding

// checkAuthPatterns identifies endpoints missing authentication
func checkAuthPatterns(bodies []NetworkBody, urlFilter string) []SecurityFinding

// checkPIIExposure finds sensitive data in logs and responses
func checkPIIExposure(bodies []NetworkBody, entries []LogEntry, urlFilter string) []SecurityFinding

// checkTransport detects HTTP usage and mixed content
func checkTransport(bodies []NetworkBody, wsEvents []WebSocketEvent, urlFilter string) []SecurityFinding

// matchesSecretPattern checks if a string matches any known secret format
func matchesSecretPattern(s string) (pattern string, matched bool)

// containsPII checks if a JSON body contains PII-indicative fields
func containsPII(body string) (fields []string, found bool)

// redactSecret returns a redacted version of a secret for evidence display
func redactSecret(s string, patternName string) string
```

### Extension Changes

Minimal — one boolean flag added to network body entries:

```javascript
// In inject.js, within the fetch wrapper:
const hasAuthHeader = !!(
  (init?.headers instanceof Headers && init.headers.has('authorization')) ||
  (typeof init?.headers === 'object' &&
    Object.keys(init?.headers || {}).some(k => k.toLowerCase() === 'authorization'))
);

emit('network:body', {
  // ... existing fields ...
  hasAuthHeader,
});
```

**Performance impact:** Single property check on the headers object — < 0.01ms per request.

### Limits

| Constraint | Limit | Reason |
|-----------|-------|--------|
| Findings per category | 20 | Prevent overwhelming output |
| URL/body scan depth | 10KB per entry | Already truncated by capture |
| Regex matches per body | 100 | Prevent regex DoS on large bodies |
| Total response size | 50KB | Cap MCP response |
| PII field detection depth | 5 levels | Reasonable JSON nesting |
| Secret redaction | Show first 6 + last 3 chars | Enough to identify without exposing |

### False Positive Mitigation

| Pattern | Mitigation |
|---------|-----------|
| Test/development keys | Don't flag keys containing "test", "dev", "example", "sample", "demo", "dummy" |
| Localhost URLs | `transport` checks skip localhost/127.0.0.1/::1 |
| Mock data in responses | Reduce severity to `info` for values matching "test", "mock", "example" |
| Environment indicators | If URL contains "staging" or "dev", add note to recommendation |

---

## Feature 2: Verification Loop (`verify_fix`)

### Purpose

Close the debugging loop. Currently:

```
1. AI identifies bug from browser errors
2. AI makes code change
3. AI asks: "Can you try that again?"
4. User reproduces the scenario
5. User says: "It works now" or "Still broken"  ← VAGUE
6. AI may need more details
7. Repeat...
```

With `verify_fix`:

```
1. AI identifies bug from browser errors
2. AI calls verify_fix("start") to snapshot current (broken) state
3. AI makes code change
4. AI calls verify_fix("watch") to begin monitoring
5. User reproduces the scenario (naturally)
6. Gasoline captures everything
7. AI calls verify_fix("compare") to get structured diff
8. AI sees: "Before: 3 errors, 500 on /api. After: 0 errors, 200 on /api" ← PRECISE
```

### MCP Tool Definition

```json
{
  "name": "verify_fix",
  "description": "Manage a verification session to compare browser state before and after a code change. Start a session to capture the 'broken' baseline, then compare after the user reproduces the scenario to see if the fix worked.",
  "inputSchema": {
    "type": "object",
    "properties": {
      "action": {
        "type": "string",
        "enum": ["start", "watch", "compare", "status", "cancel"],
        "description": "Session action: 'start' captures baseline, 'watch' begins monitoring for new activity, 'compare' diffs baseline vs current, 'status' shows session state, 'cancel' discards the session"
      },
      "session_id": {
        "type": "string",
        "description": "Session ID returned by 'start' (required for watch/compare/status/cancel)"
      },
      "label": {
        "type": "string",
        "description": "Optional label for the session (e.g., 'fix-login-error')"
      },
      "url_filter": {
        "type": "string",
        "description": "Only compare activity on URLs matching this substring"
      }
    },
    "required": ["action"]
  }
}
```

### Session Lifecycle

```
start ──→ watch ──→ compare ──→ (done)
  │          │          │
  │          │          └──→ compare again (re-run scenario)
  │          │
  │          └──→ cancel
  │
  └──→ cancel
```

#### `start` — Capture Baseline

Snapshots the current state of all buffers:
- Console errors (last 50 entries)
- Network responses (status codes, URLs)
- WebSocket connection states
- Performance snapshot (if available)

Returns a `session_id` for subsequent calls.

**Response:**

```json
{
  "session_id": "verify-abc123",
  "status": "baseline_captured",
  "baseline": {
    "captured_at": "2025-01-20T14:30:00Z",
    "console_errors": 3,
    "network_errors": 1,
    "error_details": [
      {"type": "console", "message": "Cannot read property 'user' of undefined", "count": 2},
      {"type": "console", "message": "Failed to load resource", "count": 1},
      {"type": "network", "method": "POST", "url": "/api/login", "status": 500}
    ]
  }
}
```

#### `watch` — Begin Monitoring

Clears the "after" buffer and tells the server to start collecting new events for this session. The user should reproduce the scenario after this call.

**Response:**

```json
{
  "session_id": "verify-abc123",
  "status": "watching",
  "message": "Monitoring started. Ask the user to reproduce the scenario."
}
```

#### `compare` — Diff Before/After

Compares the baseline snapshot against activity captured since `watch` was called.

**Response:**

```json
{
  "session_id": "verify-abc123",
  "status": "compared",
  "label": "fix-login-error",
  "result": {
    "verdict": "improved",
    "before": {
      "console_errors": 3,
      "network_errors": 1,
      "total_issues": 4
    },
    "after": {
      "console_errors": 0,
      "network_errors": 0,
      "total_issues": 0
    },
    "changes": [
      {
        "type": "resolved",
        "category": "console",
        "before": "Cannot read property 'user' of undefined (x2)",
        "after": "(not seen)"
      },
      {
        "type": "resolved",
        "category": "console",
        "before": "Failed to load resource",
        "after": "(not seen)"
      },
      {
        "type": "resolved",
        "category": "network",
        "before": "POST /api/login → 500",
        "after": "POST /api/login → 200"
      }
    ],
    "new_issues": [],
    "performance_diff": {
      "load_time_before": "3200ms",
      "load_time_after": "1100ms",
      "change": "-66%"
    }
  }
}
```

#### Verdict Logic

| Condition | Verdict |
|-----------|---------|
| All baseline errors resolved, no new errors | `"fixed"` |
| Some baseline errors resolved, no new errors | `"improved"` |
| Same errors present | `"unchanged"` |
| Baseline errors resolved but new errors introduced | `"different_issue"` |
| More errors than baseline | `"regressed"` |
| No errors in either baseline or current | `"no_issues_detected"` |

### Session Data Structure

```go
// VerificationSession tracks a before/after comparison
type VerificationSession struct {
    ID        string    `json:"session_id"`
    Label     string    `json:"label"`
    Status    string    `json:"status"` // "baseline_captured", "watching", "compared", "cancelled"
    URLFilter string    `json:"url_filter,omitempty"`
    CreatedAt time.Time `json:"created_at"`

    // Baseline snapshot (captured at "start")
    Baseline SessionSnapshot `json:"baseline"`

    // After snapshot (captured between "watch" and "compare")
    WatchStartedAt *time.Time       `json:"watch_started_at,omitempty"`
    After          *SessionSnapshot `json:"after,omitempty"`
}

// SessionSnapshot is a point-in-time capture of browser state
type SessionSnapshot struct {
    CapturedAt    time.Time        `json:"captured_at"`
    ConsoleErrors []SnapshotError  `json:"console_errors"`
    NetworkErrors []SnapshotNetwork `json:"network_errors"`
    PageURL       string           `json:"page_url,omitempty"`
    Performance   *PerformanceSnapshot `json:"performance,omitempty"`
}

// SnapshotError represents a console error in the snapshot
type SnapshotError struct {
    Message string `json:"message"`
    Count   int    `json:"count"`
    Source  string `json:"source,omitempty"`
}

// SnapshotNetwork represents a network error in the snapshot
type SnapshotNetwork struct {
    Method string `json:"method"`
    URL    string `json:"url"`
    Status int    `json:"status"`
}

// VerificationResult is the diff output
type VerificationResult struct {
    Verdict         string           `json:"verdict"`
    Before          IssueSummary     `json:"before"`
    After           IssueSummary     `json:"after"`
    Changes         []VerifyChange   `json:"changes"`
    NewIssues       []VerifyChange   `json:"new_issues"`
    PerformanceDiff *PerfDiff        `json:"performance_diff,omitempty"`
}

type IssueSummary struct {
    ConsoleErrors int `json:"console_errors"`
    NetworkErrors int `json:"network_errors"`
    TotalIssues   int `json:"total_issues"`
}

type VerifyChange struct {
    Type     string `json:"type"` // "resolved", "new", "changed", "unchanged"
    Category string `json:"category"` // "console", "network"
    Before   string `json:"before"`
    After    string `json:"after"`
}
```

### Comparison Algorithm

```
1. Group baseline console errors by message (dedup with count)
2. Group "after" console errors by message (dedup with count)
3. For each baseline error:
   a. If not present in after → "resolved"
   b. If present with same count → "unchanged"
   c. If present with different count → "changed"
4. For each "after" error NOT in baseline:
   → "new" issue
5. For network errors, match by method+URL_path:
   a. Same endpoint, different status → compare (resolved if 2xx, changed otherwise)
   b. Baseline endpoint not called → skip (can't compare)
   c. New endpoint errors → "new" issue
6. Determine verdict from change list
```

### Session Management

| Constraint | Limit | Reason |
|-----------|-------|--------|
| Max concurrent sessions | 3 | Memory bound |
| Session TTL | 30 minutes | Prevent stale data |
| Baseline snapshot size | 50 entries (errors) + 50 entries (network) | Relevant window |
| "After" buffer | Events since `watch` call only | Focused comparison |
| Session auto-cleanup | After `compare` or TTL | Memory management |

### Error Matching

Errors are matched by normalized message (strip dynamic values):

```go
func normalizeErrorMessage(msg string) string {
    // Remove UUIDs
    msg = uuidRegex.ReplaceAllString(msg, "[uuid]")
    // Remove numbers that look like IDs
    msg = idRegex.ReplaceAllString(msg, "[id]")
    // Remove timestamps
    msg = tsRegex.ReplaceAllString(msg, "[timestamp]")
    // Remove file paths with line numbers
    msg = pathRegex.ReplaceAllString(msg, "[file]")
    return msg
}
```

This prevents "Cannot read property 'x' of undefined at line 42" and "Cannot read property 'x' of undefined at line 45" from being treated as different errors.

---

## Feature 3: Session Comparison (`diff_sessions`)

### Purpose

Store named snapshots of browser state and compare them. Use cases:

1. **Deployment verification:** Navigate same flow before and after deploy, compare
2. **Regression detection:** "Does this change break the checkout flow?"
3. **A/B comparison:** Compare two implementations of the same feature
4. **Session replay analysis:** Compare a known-good session with a current one

### MCP Tool Definition

```json
{
  "name": "diff_sessions",
  "description": "Store and compare named browser session snapshots. Capture the current state as a named snapshot, then compare any two snapshots to find differences in errors, network behavior, and performance.",
  "inputSchema": {
    "type": "object",
    "properties": {
      "action": {
        "type": "string",
        "enum": ["capture", "compare", "list", "delete"],
        "description": "Action to perform"
      },
      "name": {
        "type": "string",
        "description": "Name for the snapshot (required for 'capture', used for 'delete')"
      },
      "compare_a": {
        "type": "string",
        "description": "Name of first snapshot (for 'compare')"
      },
      "compare_b": {
        "type": "string",
        "description": "Name of second snapshot (for 'compare'). Use 'current' for live state."
      },
      "url_filter": {
        "type": "string",
        "description": "Only include data for URLs matching this substring"
      }
    },
    "required": ["action"]
  }
}
```

### Actions

#### `capture` — Store Named Snapshot

Captures the current state of all buffers and stores it under the given name.

**Response:**

```json
{
  "action": "captured",
  "name": "before-deploy",
  "snapshot": {
    "captured_at": "2025-01-20T14:30:00Z",
    "console_errors": 0,
    "console_warnings": 2,
    "network_requests": 47,
    "network_errors": 0,
    "websocket_connections": 1,
    "performance": {
      "load_time": 1100,
      "request_count": 12,
      "transfer_size": 340000
    },
    "unique_endpoints": 8,
    "page_url": "http://localhost:3000/dashboard"
  }
}
```

#### `compare` — Diff Two Snapshots

Compare two named snapshots (or a named snapshot vs current state).

**Response:**

```json
{
  "action": "compared",
  "a": "before-deploy",
  "b": "after-deploy",
  "diff": {
    "errors": {
      "new": [
        {"type": "console", "message": "React hydration mismatch", "count": 3}
      ],
      "resolved": [],
      "unchanged": []
    },
    "network": {
      "new_errors": [
        {"method": "GET", "url": "/api/notifications", "status": 502}
      ],
      "status_changes": [
        {"method": "GET", "url": "/api/dashboard", "before": 200, "after": 200, "duration_change": "+340ms"}
      ],
      "new_endpoints": [
        {"method": "GET", "url": "/api/feature-flags", "status": 200}
      ],
      "missing_endpoints": []
    },
    "performance": {
      "load_time": {"before": 1100, "after": 3200, "change": "+191%", "regression": true},
      "request_count": {"before": 12, "after": 47, "change": "+292%", "regression": true},
      "transfer_size": {"before": 340000, "after": 2400000, "change": "+606%", "regression": true}
    },
    "websockets": {
      "connection_changes": []
    }
  },
  "summary": {
    "verdict": "regressed",
    "new_errors": 3,
    "resolved_errors": 0,
    "performance_regressions": 3,
    "new_network_errors": 1
  }
}
```

#### `list` — Show All Stored Snapshots

**Response:**

```json
{
  "action": "listed",
  "snapshots": [
    {"name": "before-deploy", "captured_at": "2025-01-20T14:30:00Z", "page_url": "/dashboard", "error_count": 0},
    {"name": "after-deploy", "captured_at": "2025-01-20T14:45:00Z", "page_url": "/dashboard", "error_count": 3},
    {"name": "checkout-flow", "captured_at": "2025-01-20T15:00:00Z", "page_url": "/checkout", "error_count": 0}
  ]
}
```

#### `delete` — Remove a Snapshot

**Response:**

```json
{
  "action": "deleted",
  "name": "old-snapshot"
}
```

### Snapshot Data Structure

```go
// NamedSnapshot is a stored point-in-time browser state
type NamedSnapshot struct {
    Name       string    `json:"name"`
    CapturedAt time.Time `json:"captured_at"`
    URLFilter  string    `json:"url_filter,omitempty"`
    PageURL    string    `json:"page_url"`

    // Console state
    ConsoleErrors   []SnapshotError  `json:"console_errors"`
    ConsoleWarnings []SnapshotError  `json:"console_warnings"`

    // Network state
    NetworkRequests []SnapshotNetworkRequest `json:"network_requests"`

    // WebSocket state
    WebSocketConnections []SnapshotWSConnection `json:"websocket_connections"`

    // Performance state
    Performance *PerformanceSnapshot `json:"performance,omitempty"`
}

// SnapshotNetworkRequest includes status and timing
type SnapshotNetworkRequest struct {
    Method       string `json:"method"`
    URL          string `json:"url"`
    Status       int    `json:"status"`
    Duration     int    `json:"duration,omitempty"`
    ResponseSize int    `json:"response_size,omitempty"`
    ContentType  string `json:"content_type,omitempty"`
}

// SnapshotWSConnection represents a WS connection at snapshot time
type SnapshotWSConnection struct {
    URL   string `json:"url"`
    State string `json:"state"` // "open", "closed"
    MessageRate float64 `json:"message_rate,omitempty"`
}

// SessionDiff is the comparison result
type SessionDiff struct {
    Errors      ErrorDiff       `json:"errors"`
    Network     NetworkDiff     `json:"network"`
    Performance PerformanceDiff `json:"performance"`
    WebSockets  WSDiff          `json:"websockets"`
}

type ErrorDiff struct {
    New       []SnapshotError `json:"new"`
    Resolved  []SnapshotError `json:"resolved"`
    Unchanged []SnapshotError `json:"unchanged"`
}

type NetworkDiff struct {
    NewErrors       []SnapshotNetworkRequest `json:"new_errors"`
    StatusChanges   []NetworkChange          `json:"status_changes"`
    NewEndpoints    []SnapshotNetworkRequest `json:"new_endpoints"`
    MissingEndpoints []SnapshotNetworkRequest `json:"missing_endpoints"`
}

type NetworkChange struct {
    Method         string `json:"method"`
    URL            string `json:"url"`
    BeforeStatus   int    `json:"before"`
    AfterStatus    int    `json:"after"`
    DurationChange string `json:"duration_change,omitempty"`
}
```

### Snapshot Storage

| Constraint | Limit | Reason |
|-----------|-------|--------|
| Max stored snapshots | 10 | Memory bound (each ~50KB) |
| Snapshot TTL | 2 hours | Development session scope |
| Console entries per snapshot | 50 | Relevant window |
| Network requests per snapshot | 100 | Buffer match |
| Snapshot name length | 50 chars | Reasonable identifier |
| Reserved names | "current" | Used for live-state comparison |

### Diff Algorithm

```
Error diffing:
  1. Normalize error messages (same as verify_fix)
  2. Set difference: A_errors - B_errors = resolved
  3. Set difference: B_errors - A_errors = new
  4. Intersection = unchanged

Network diffing:
  1. Group requests by (method, URL_path) — ignore query params
  2. For matching endpoints: compare status codes and durations
  3. Endpoints in B but not A → new_endpoints
  4. Endpoints in A but not B → missing_endpoints
  5. Status changes: before != after

Performance diffing:
  1. If both have performance snapshots, compare load/FCP/LCP
  2. Apply regression thresholds (same as check_performance)
  3. Include percentage change

Verdict determination:
  - "improved" if resolved > 0 AND new == 0
  - "regressed" if new > 0 OR performance_regressions > 0
  - "unchanged" if no differences
  - "mixed" if both resolved and new
```

---

## Feature 4: API Contract Validation (`validate_api`)

### Purpose

AI-generated code frequently breaks API contracts — the backend returns a different shape than the frontend expects, or an endpoint suddenly returns errors after a code change. This tool tracks API response shapes over a session and detects when they change.

### MCP Tool Definition

```json
{
  "name": "validate_api",
  "description": "Track API response shapes across a session and detect contract violations. Identifies when response structures change unexpectedly, fields go missing, types change, or error responses replace success responses.",
  "inputSchema": {
    "type": "object",
    "properties": {
      "action": {
        "type": "string",
        "enum": ["analyze", "report", "clear"],
        "description": "'analyze' processes current network bodies and returns violations. 'report' shows all tracked endpoint shapes. 'clear' resets shape tracking."
      },
      "url_filter": {
        "type": "string",
        "description": "Only analyze endpoints matching this URL substring"
      },
      "ignore_endpoints": {
        "type": "array",
        "items": {"type": "string"},
        "description": "URL substrings to exclude from analysis (e.g., '/health', '/metrics')"
      }
    },
    "required": ["action"]
  }
}
```

### Response Format — `analyze`

```json
{
  "action": "analyzed",
  "violations": [
    {
      "endpoint": "GET /api/users/profile",
      "type": "shape_change",
      "description": "Response shape changed: field 'avatar_url' was present in 5 responses but missing in the latest response",
      "expected_shape": {
        "id": "number",
        "name": "string",
        "email": "string",
        "avatar_url": "string"
      },
      "actual_shape": {
        "id": "number",
        "name": "string",
        "email": "string"
      },
      "missing_fields": ["avatar_url"],
      "occurrences": {
        "expected_count": 5,
        "violation_count": 1,
        "first_seen": "2025-01-20T14:30:00Z",
        "last_violation": "2025-01-20T14:45:00Z"
      }
    },
    {
      "endpoint": "POST /api/orders",
      "type": "error_spike",
      "description": "Endpoint returned success (201) 3 times, then switched to error (500) for the last 2 requests",
      "status_history": [201, 201, 201, 500, 500],
      "last_error_body": {"error": "Internal server error", "message": "Database connection refused"}
    },
    {
      "endpoint": "GET /api/products",
      "type": "type_change",
      "description": "Field 'price' changed type from 'number' to 'string' in the latest response",
      "field": "price",
      "expected_type": "number",
      "actual_type": "string",
      "sample_value": "19.99"
    }
  ],
  "tracked_endpoints": 8,
  "total_requests_analyzed": 47,
  "clean_endpoints": 6
}
```

### Response Format — `report`

```json
{
  "action": "report",
  "endpoints": [
    {
      "endpoint": "GET /api/users/profile",
      "method": "GET",
      "call_count": 6,
      "status_codes": {"200": 5, "401": 1},
      "established_shape": {
        "id": "number",
        "name": "string",
        "email": "string",
        "avatar_url": "string",
        "created_at": "string"
      },
      "consistency": "98%",
      "last_called": "2025-01-20T14:45:00Z"
    },
    {
      "endpoint": "POST /api/orders",
      "method": "POST",
      "call_count": 5,
      "status_codes": {"201": 3, "500": 2},
      "established_shape": {
        "id": "number",
        "status": "string",
        "total": "number",
        "items": [{"id": "number", "quantity": "number"}]
      },
      "consistency": "60%",
      "last_called": "2025-01-20T14:44:00Z"
    }
  ]
}
```

### Violation Types

| Type | Trigger | Severity |
|------|---------|----------|
| `shape_change` | A field that appeared in N responses is missing in a subsequent response | warning if N >= 3, info if N < 3 |
| `type_change` | A field's JSON type changed (number → string, object → null) | warning |
| `error_spike` | Endpoint returned success N times then started returning errors | critical if 5xx, warning if 4xx |
| `new_field` | A field appeared that wasn't in the established shape | info (not necessarily bad) |
| `array_shape_change` | Array elements changed structure | warning |
| `null_field` | A previously non-null field became null | info |

### Shape Tracking Algorithm

```go
// EndpointTracker maintains the established shape for an endpoint
type EndpointTracker struct {
    Endpoint        string          `json:"endpoint"` // "METHOD /path"
    EstablishedShape interface{}    `json:"established_shape"`
    CallCount       int             `json:"call_count"`
    StatusHistory   []int           `json:"status_history"` // Last 20 status codes
    ShapeConsistency float64        `json:"consistency"`
    LastCalled      time.Time       `json:"last_called"`
    Violations      []ShapeViolation `json:"violations"`
}

// Algorithm:
// 1. First successful response (2xx) → establish shape
// 2. Subsequent successful responses → compare to established shape
// 3. If shape matches established: consistency++
// 4. If shape differs:
//    a. Identify which fields changed
//    b. If established has >= 3 consistent responses, flag as violation
//    c. If established has < 3, update established shape (still learning)
// 5. Error responses (4xx/5xx) tracked separately:
//    a. If N successes followed by errors → error_spike
//    b. Error responses don't update the established shape
```

### Shape Comparison

Uses the existing `extractResponseShape` function from the timeline feature, extended with comparison:

```go
// compareShapes returns differences between two response shapes
func compareShapes(expected, actual interface{}) []ShapeDiff {
    diffs := []ShapeDiff{}

    expectedMap, eOK := expected.(map[string]interface{})
    actualMap, aOK := actual.(map[string]interface{})

    if !eOK || !aOK {
        // Type-level difference
        if fmt.Sprintf("%T", expected) != fmt.Sprintf("%T", actual) {
            diffs = append(diffs, ShapeDiff{
                Type: "type_change",
                Path: "",
                Expected: describeType(expected),
                Actual: describeType(actual),
            })
        }
        return diffs
    }

    // Check for missing fields
    for key := range expectedMap {
        if _, found := actualMap[key]; !found {
            diffs = append(diffs, ShapeDiff{
                Type: "missing_field",
                Path: key,
                Expected: expectedMap[key],
            })
        }
    }

    // Check for new fields
    for key := range actualMap {
        if _, found := expectedMap[key]; !found {
            diffs = append(diffs, ShapeDiff{
                Type: "new_field",
                Path: key,
                Actual: actualMap[key],
            })
        }
    }

    // Check for type changes in shared fields
    for key := range expectedMap {
        if actualVal, found := actualMap[key]; found {
            childDiffs := compareShapes(expectedMap[key], actualVal)
            for i := range childDiffs {
                childDiffs[i].Path = key + "." + childDiffs[i].Path
            }
            diffs = append(diffs, childDiffs...)
        }
    }

    return diffs
}
```

### Endpoint Grouping

Endpoints are grouped by `METHOD + URL_path` (ignoring query params):

```go
func normalizeEndpoint(method, url string) string {
    parsed, err := url.Parse(url)
    if err != nil {
        return method + " " + url
    }
    // Remove query params and fragments
    path := parsed.Path
    // Normalize dynamic segments: /users/123 → /users/:id
    path = dynamicSegmentRegex.ReplaceAllString(path, "/:id")
    return method + " " + path
}

// Dynamic segment detection:
// - Pure numeric: /users/123 → /users/:id
// - UUID: /items/550e8400-e29b... → /items/:id
// - Long hex: /assets/a1b2c3d4e5 → /assets/:id
var dynamicSegmentRegex = regexp.MustCompile(
    `/([0-9]+|[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}|[0-9a-f]{8,})(?:/|$)`,
)
```

### Limits

| Constraint | Limit | Reason |
|-----------|-------|--------|
| Tracked endpoints | 30 | Memory bound |
| Status history per endpoint | 20 | Recent activity |
| Shape comparison depth | 3 levels | Same as extractResponseShape |
| Minimum calls to establish shape | 3 | Avoid false positives from first response |
| Violations per endpoint | 10 | Cap output size |
| Total response size | 50KB | MCP response cap |
| Endpoint URL normalization | Strip query, normalize IDs | Consistent grouping |

---

## Feature 5: Context Streaming (Passive Mode)

### Purpose

Currently, the AI must explicitly poll for browser state via MCP tool calls. This creates overhead: the AI has to decide WHEN to check, and often checks too late (after the user has already noticed the problem).

Context streaming inverts this — Gasoline pushes significant events to the AI proactively, enabling responses like:
- "I notice you just got a 403 on /api/dashboard — let me check the auth middleware"
- "The page loaded 4 seconds slower than last time — looks like the new analytics script is blocking"
- "You've clicked that button 3 times with no response — the click handler might be broken"

### Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                    Gasoline Server (Go)                           │
│                                                                   │
│  Event Filter:                                                   │
│    All incoming data → significance check → emit if significant  │
│                                                                   │
│  SSE Endpoint:                                                   │
│    GET /events/stream                                            │
│    → Pushes significant events as SSE messages                   │
│                                                                   │
│  MCP Notification:                                               │
│    When AI is connected via MCP, use MCP notifications            │
│    (JSON-RPC notifications, no response expected)                │
│                                                                   │
└─────────────────────────────────────────────────────────────────┘
```

### MCP Tool Definition

```json
{
  "name": "configure_streaming",
  "description": "Configure which browser events Gasoline proactively reports. When enabled, significant events (errors, performance regressions, repeated user frustration) are pushed as MCP notifications without requiring explicit tool calls.",
  "inputSchema": {
    "type": "object",
    "properties": {
      "enabled": {
        "type": "boolean",
        "description": "Enable or disable context streaming (default: false)"
      },
      "events": {
        "type": "array",
        "items": {
          "type": "string",
          "enum": ["errors", "network_errors", "performance", "user_frustration", "security"]
        },
        "description": "Which event categories to stream (default: all)"
      },
      "throttle_seconds": {
        "type": "number",
        "description": "Minimum seconds between notifications (default: 5)"
      },
      "url_filter": {
        "type": "string",
        "description": "Only stream events for URLs matching this substring"
      }
    }
  }
}
```

### Event Significance Filters

Not every captured event warrants a notification. Only "significant" events are streamed:

| Category | Trigger | Notification |
|----------|---------|-------------|
| `errors` | New console error (not seen before in session) | `"New error: {message}"` |
| `errors` | Unhandled promise rejection | `"Unhandled rejection: {message}"` |
| `network_errors` | 5xx response | `"Server error: {method} {url} → {status}"` |
| `network_errors` | 4xx on endpoint that previously returned 2xx | `"Auth/access issue: {method} {url} → {status} (was {prev_status})"` |
| `performance` | Page load > 2x baseline | `"Performance regression: {url} loaded in {time} (baseline: {baseline})"` |
| `performance` | Long task > 500ms | `"Main thread blocked for {duration}ms"` |
| `user_frustration` | Same element clicked 3+ times in 5 seconds | `"Repeated clicks on {selector} — handler may be broken"` |
| `user_frustration` | Form submitted with no network response after 5s | `"Form submit to {action} — no response after 5s"` |
| `security` | Secret pattern detected in new request | `"Possible credential exposure in {method} {url}"` |

### Notification Format (MCP)

MCP notifications use the JSON-RPC 2.0 notification format (no `id` field, no response expected):

```json
{
  "jsonrpc": "2.0",
  "method": "notifications/gasoline/event",
  "params": {
    "category": "network_errors",
    "severity": "error",
    "message": "Server error: POST /api/users → 500",
    "timestamp": "2025-01-20T14:30:00Z",
    "context": {
      "method": "POST",
      "url": "/api/users",
      "status": 500,
      "response_preview": "{\"error\":\"Internal server error\"}",
      "previous_status": 200
    }
  }
}
```

### Throttling

| Parameter | Default | Range | Purpose |
|-----------|---------|-------|---------|
| `throttle_seconds` | 5 | 1-60 | Min gap between notifications |
| Max notifications per minute | 12 | Fixed | Prevent flooding |
| Dedup window | 30s | Fixed | Same message not repeated within window |
| Batch window | 2s | Fixed | Group rapid events into single notification |

### Implementation

```go
type StreamConfig struct {
    Enabled         bool     `json:"enabled"`
    Events          []string `json:"events"`
    ThrottleSeconds int      `json:"throttle_seconds"`
    URLFilter       string   `json:"url_filter"`
}

type StreamState struct {
    Config        StreamConfig
    LastNotified  time.Time
    SeenMessages  map[string]time.Time // Dedup cache (message → last sent)
    NotifyCount   int                  // Count in current minute
    MinuteStart   time.Time            // Current minute window start
    PendingBatch  []StreamEvent        // Events waiting for batch window
    mu            sync.Mutex
}

// StreamEvent is an event ready for notification
type StreamEvent struct {
    Category  string                 `json:"category"`
    Severity  string                 `json:"severity"`
    Message   string                 `json:"message"`
    Timestamp time.Time              `json:"timestamp"`
    Context   map[string]interface{} `json:"context,omitempty"`
}

// CheckAndEmit evaluates whether an incoming event should trigger a notification
func (s *StreamState) CheckAndEmit(event StreamEvent) bool {
    s.mu.Lock()
    defer s.mu.Unlock()

    if !s.Config.Enabled {
        return false
    }

    // Category filter
    if !contains(s.Config.Events, event.Category) {
        return false
    }

    // URL filter
    if s.Config.URLFilter != "" && !strings.Contains(event.Context["url"].(string), s.Config.URLFilter) {
        return false
    }

    // Dedup check
    if lastSent, seen := s.SeenMessages[event.Message]; seen {
        if time.Since(lastSent) < 30*time.Second {
            return false
        }
    }

    // Rate limit
    if time.Since(s.MinuteStart) > time.Minute {
        s.NotifyCount = 0
        s.MinuteStart = time.Now()
    }
    if s.NotifyCount >= 12 {
        return false
    }

    // Throttle
    if time.Since(s.LastNotified) < time.Duration(s.Config.ThrottleSeconds)*time.Second {
        s.PendingBatch = append(s.PendingBatch, event)
        return false
    }

    s.LastNotified = time.Now()
    s.SeenMessages[event.Message] = time.Now()
    s.NotifyCount++
    return true
}
```

### User Frustration Detection

Built from existing action capture data (no new capture needed):

```go
// FrustrationDetector tracks patterns indicating user frustration
type FrustrationDetector struct {
    clickHistory map[string][]time.Time // selector → click timestamps
    formSubmits  map[string]time.Time   // form action → submit time
}

// CheckClick returns a frustration event if the same element was clicked 3+ times in 5s
func (f *FrustrationDetector) CheckClick(selector string, ts time.Time) *StreamEvent {
    f.clickHistory[selector] = append(f.clickHistory[selector], ts)

    // Keep only last 5 seconds
    cutoff := ts.Add(-5 * time.Second)
    recent := filterAfter(f.clickHistory[selector], cutoff)
    f.clickHistory[selector] = recent

    if len(recent) >= 3 {
        return &StreamEvent{
            Category: "user_frustration",
            Severity: "warning",
            Message:  fmt.Sprintf("Repeated clicks on %s (%d times in 5s)", selector, len(recent)),
        }
    }
    return nil
}

// CheckFormTimeout checks if a form submission had no network response after 5s
func (f *FrustrationDetector) CheckFormTimeout(action string) *StreamEvent {
    if submitTime, exists := f.formSubmits[action]; exists {
        if time.Since(submitTime) > 5*time.Second {
            delete(f.formSubmits, action)
            return &StreamEvent{
                Category: "user_frustration",
                Severity: "warning",
                Message:  fmt.Sprintf("Form submit to %s — no response after 5s", action),
            }
        }
    }
    return nil
}
```

### MCP Integration

Context streaming uses MCP's built-in notification mechanism. The server sends notifications to the connected MCP client (AI assistant) without expecting a response:

```go
// In the MCP handler, after processing tool calls:
func (s *Server) emitNotification(event StreamEvent) {
    notification := map[string]interface{}{
        "jsonrpc": "2.0",
        "method":  "notifications/gasoline/event",
        "params":  event,
    }

    data, _ := json.Marshal(notification)
    // Write to stdout (MCP stdio transport)
    fmt.Println(string(data))
}
```

### Streaming Off By Default

Context streaming is **disabled by default**. The AI must explicitly enable it via `configure_streaming`. This prevents:
- Unexpected output to MCP clients that don't support notifications
- Noise when the AI is focused on a specific task
- Performance concerns from the notification pipeline

---

## Feature 6: Enhanced Test Generation (v2)

This feature is fully specified in [generate-test-v2.md](generate-test-v2.md). Summary of additions:

| Capability | Description |
|-----------|-------------|
| DOM state assertions | Assert visible headings and test-ID elements render correctly |
| API fixture generation | Turn captured responses into Playwright route mocks |
| Visual snapshot hooks | Insert `toHaveScreenshot()` at navigation steps |
| Deep response contracts | `expect.objectContaining()` patterns for response structure |
| Wait strategies | Intelligent waits based on observed patterns |
| Assertion confidence | Comments indicating reliability of each assertion |

No additional specification needed — see `generate-test-v2.md` for full details.

---

## Feature 7: Performance Budget Monitor

This feature is fully specified in [performance-budget-spec.md](performance-budget-spec.md). Summary:

| Capability | Description |
|-----------|-------------|
| Navigation timing | FCP, LCP, TTFB, DOMContentLoaded, load event |
| Network summary | Request count, transfer size, by-type breakdown |
| Long tasks | Count, total blocking time, longest task |
| Web vitals | CLS via PerformanceObserver |
| Baseline comparison | Rolling weighted average, regression detection |
| Formatted report | Human-readable performance report with regression indicators |

No additional specification needed — see `performance-budget-spec.md` for full details.

---

## Shared Concerns

### Extension Changes Summary

v6 requires **minimal** extension changes because it operates as an analysis layer over existing data:

| Change | File | Impact |
|--------|------|--------|
| Add `hasAuthHeader` boolean to network body entries | `inject.js` | < 0.01ms per request |
| DOM snapshot capture (for generate_test v2) | `inject.js` | < 2ms per navigation |
| Performance observers (for check_performance) | `inject.js` | < 0.1ms per observer callback |
| Forward new message types | `content.js`, `background.js` | Trivial routing |

Total page performance impact: **< 3ms per page load** (all async, after load event).

### Server Memory Budget

| Feature | Buffer Size | Max Memory |
|---------|-------------|------------|
| Security audit | No new storage (reads existing buffers) | 0 |
| Verification sessions | 3 sessions × ~100KB each | 300KB |
| Named snapshots | 10 snapshots × ~50KB each | 500KB |
| API contract tracking | 30 endpoints × ~10KB each | 300KB |
| Stream state | Config + dedup cache | 50KB |
| Performance snapshots | 20 URLs × 4KB each | 80KB |
| Performance baselines | 20 URLs × 2KB each | 40KB |
| **Total v6 addition** | | **~1.3MB** |

Combined with existing v4 buffers (~15MB max), total server memory stays well under the 100MB hard limit.

### Privacy & Security

- **Security audit:** Findings include redacted evidence (secrets are masked). Full values are NEVER included in MCP responses.
- **Verification sessions:** Only stores error messages and status codes, not request/response bodies.
- **Named snapshots:** Same — summary data only, not full payloads.
- **API validation:** Stores response SHAPES (types), not values. Actual data never leaves the shape extractor.
- **Context streaming:** Notifications contain summaries and previews (first 100 chars), not full payloads.
- **All data stays local:** Nothing leaves localhost. No external services contacted.

### Backward Compatibility

- All new tools are additive — no existing tool behavior changes
- The one extension change (`hasAuthHeader` flag) is additive to the network body format
- `generate_test` gains new input fields but old-style calls continue to work
- Context streaming is opt-in and disabled by default
- Existing MCP clients that don't understand notifications will simply ignore them

---

## Testing Requirements

### Security Audit

| Test Category | Cases |
|--------------|-------|
| Credential detection | API key in URL, AWS key, GitHub token, JWT, Stripe key, private key |
| False positive mitigation | Test keys, dev URLs, mock data, localhost |
| Auth pattern detection | PII without auth, PII with auth, public endpoints |
| PII scanning | Email, phone, SSN, credit card, JWT in logs |
| Transport checks | HTTP API, mixed content (script vs image), WS insecure |
| URL filtering | Only scans matching URLs |
| Severity filtering | Respects minimum severity |
| Evidence redaction | Secrets properly masked in output |
| Edge cases | Empty buffers, binary bodies, very large responses |

### Verification Loop

| Test Category | Cases |
|--------------|-------|
| Session lifecycle | start → watch → compare, start → cancel, double compare |
| Baseline capture | Errors captured, network captured, performance captured |
| After capture | Only events since watch, respects URL filter |
| Comparison | All verdicts (fixed, improved, unchanged, different_issue, regressed) |
| Error matching | Normalized messages, dynamic values stripped |
| Network matching | Status changes, new endpoints, missing endpoints |
| Session limits | Max 3 concurrent, TTL expiry, auto-cleanup |
| Edge cases | Empty baseline, empty after, no matching events |

### Session Comparison

| Test Category | Cases |
|--------------|-------|
| Capture | All buffer types included, URL filter works |
| Compare | Error diff, network diff, performance diff, WS diff |
| Verdicts | improved, regressed, unchanged, mixed |
| Special names | "current" compares to live state |
| Storage limits | Max 10 snapshots, LRU eviction, TTL expiry |
| Edge cases | Comparing same snapshot, missing snapshot, empty snapshot |

### API Validation

| Test Category | Cases |
|--------------|-------|
| Shape tracking | First response establishes, subsequent confirm/violate |
| Violation types | shape_change, type_change, error_spike, new_field, null_field |
| Endpoint grouping | Method+path, dynamic segment normalization, query param ignore |
| Consistency calculation | Percentage of responses matching established shape |
| Minimum calls | No violations until 3 consistent responses established |
| Edge cases | Non-JSON responses, empty bodies, very deep nesting, arrays |

### Context Streaming

| Test Category | Cases |
|--------------|-------|
| Configuration | Enable/disable, event filtering, URL filtering |
| Throttling | Respects throttle_seconds, max per minute, dedup window |
| Significance | Only significant events emitted, non-significant filtered |
| Frustration detection | Repeated clicks, form timeout |
| Batching | Rapid events grouped within batch window |
| MCP integration | Notification format correct, sent to stdout |
| Edge cases | Streaming disabled, no events match filter, rate limit hit |

---

## Implementation Order

Each feature can be implemented independently (no cross-feature dependencies):

### Phase 1: Analysis Layer (security_audit + validate_api)
1. Define Go types for findings and violations
2. Write tests for pattern matching and shape comparison
3. Implement pattern matchers (credential regex, PII detection)
4. Implement shape comparison algorithm
5. Wire up MCP tool handlers
6. Add `hasAuthHeader` flag to extension (single line)

### Phase 2: Verification Layer (verify_fix + diff_sessions)
1. Define session/snapshot types
2. Write tests for comparison algorithms
3. Implement session state machine
4. Implement snapshot capture and diffing
5. Wire up MCP tool handlers

### Phase 3: Proactive Intelligence (context streaming)
1. Define stream config and event types
2. Write tests for significance filters and throttling
3. Implement frustration detector
4. Implement MCP notification emission
5. Wire up configuration tool

### Phase 4: Enhanced Generation (generate_test v2 + check_performance)
These are specified separately and can be implemented in parallel with phases 1-3.

---

## Files to Change

| File | Changes |
|------|---------|
| `cmd/dev-console/v4.go` | New types, security audit, validation, verification, streaming |
| `cmd/dev-console/v4_test.go` | Tests for all new functionality |
| `cmd/dev-console/main.go` | New MCP tool registrations, SSE endpoint |
| `extension/inject.js` | `hasAuthHeader` flag, DOM snapshots, perf observers |
| `extension/background.js` | Forward new message types |
| `extension/content.js` | Bridge new message types |
| `extension-tests/security-audit.test.js` | Extension-side auth flag tests |
| `extension-tests/dom-snapshot.test.js` | DOM snapshot capture tests |
| `extension-tests/performance-snapshot.test.js` | Performance observer tests |

---

## Non-Goals

Things v6 explicitly does NOT do:

- **No AI-powered analysis**: All detection is pattern-based. No LLM calls within Gasoline.
- **No code modification**: Gasoline reports issues; the AI assistant decides what to fix.
- **No external network calls**: Everything stays on localhost.
- **No persistent storage**: All data lives in memory, scoped to the server process lifetime.
- **No authentication**: The server is localhost-only, no auth needed.
- **No browser automation**: Gasoline observes; it never controls the browser.
- **No test execution**: `generate_test` outputs scripts; the AI runs them via its own tools.
