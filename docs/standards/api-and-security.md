# API Design and Security Standards

> **RESTful API design, HTTP patterns, and security standards**

**Scope:** API design conventions, external service integrations, rate limiting, CORS, and security validation.

**Related Standards:**
- [validation-patterns.md](validation-patterns.md) — Input validation
- [error-and-recovery.md](error-and-recovery.md) — Error handling and retry logic
- [data-design.md](data-design.md) — Data model design for APIs

---

## 🌐 HTTP & External APIs

### API Design

**Semantic Naming:**
- ✅ **RESTful conventions:** GET /users, POST /users, PUT /users/:id
- ✅ **Clear paths:** `/network/bodies` not `/nb`, `/websocket/events` not `/ws`
- ✅ **Consistent naming:** Use same terms (e.g., "correlation_id" everywhere, not "corrId" in some places)

**Parameters:**
- ✅ **Mandatory vs optional clear:** Required params are path/body, optional are query params
- ✅ **Good defaults:** Optional params have sensible defaults
- ✅ **Validate all inputs:** Check types, ranges, formats
- ✅ **Prompt if ambiguous:** If parameter meaning is unclear, ask user to clarify

**Configuration:**
- ✅ **All defaults configurable:** Via env vars or config file
- ✅ **Document configuration:** What can be changed, where, and why
- ✅ **Validate configuration:** Check on startup, fail fast if invalid

**Example:**
```go
// /network/bodies endpoint parameters
type NetworkBodiesParams struct {
    // URL substring filter (optional)
    // Default: "" (no filtering)
    // Source: Query param ?url=substring
    URLFilter string

    // HTTP method filter (GET, POST, etc.)
    // Default: "" (all methods)
    // Source: Query param ?method=POST
    Method string

    // Maximum entries to return
    // Default: 100 (from DefaultNetworkBodyLimit constant)
    // Range: 1-1000
    // Source: Query param ?limit=50
    Limit int
}
```

### Error Handling (HTTP)

**Outgoing Calls (You make requests):**
- ✅ **Timeouts mandatory:** Use `context.WithTimeout()`, never indefinite
- ✅ **Retry logic:** For transient errors (503, network issues)
- ✅ **Backoff strategy:** Exponential backoff with jitter
- ✅ **Circuit breaker:** For repeated failures, stop trying
- ✅ **Fallback behavior:** What happens if external service is down?

**Example:**
```go
// fetchGitHubRelease calls GitHub API with retry and backoff
func fetchGitHubRelease() (*Release, error) {
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()

    var lastErr error
    for attempt := 0; attempt < maxRetries; attempt++ {
        req, _ := http.NewRequestWithContext(ctx, "GET", githubAPIURL, nil)
        resp, err := http.DefaultClient.Do(req)

        if err == nil && resp.StatusCode == 200 {
            // Success
            return parseRelease(resp.Body)
        }

        // Retry logic
        if resp != nil && resp.StatusCode >= 500 {
            // Server error - retry with backoff
            lastErr = fmt.Errorf("server error: %d", resp.StatusCode)
            time.Sleep(backoff(attempt))
            continue
        }

        // Client error - don't retry
        return nil, fmt.Errorf("request failed: %w", err)
    }

    return nil, fmt.Errorf("max retries exceeded: %w", lastErr)
}
```

**Incoming Calls (You provide endpoints):**
- ✅ **Rate limiting:** Prevent DoS, abuse
- ✅ **Input validation:** Check all parameters before use
- ✅ **Clear error responses:** Include what's wrong and how to fix it
- ✅ **Appropriate status codes:** 400 for client errors, 500 for server errors
- ✅ **Structured errors:** JSON with error code, message, hint

**Example:**
```go
func handleNetworkBodies(w http.ResponseWriter, r *http.Request) {
    // Rate limiting
    if !rateLimiter.Allow(clientID) {
        jsonError(w, 429, "rate_limit_exceeded",
            "Too many requests. Limit: 10/second. Retry after 1 second.")
        return
    }

    // Validate method
    if r.Method != "GET" {
        jsonError(w, 405, "method_not_allowed",
            "Only GET supported. Use GET /network/bodies")
        return
    }

    // Parse and validate parameters
    params, err := parseNetworkBodiesParams(r)
    if err != nil {
        jsonError(w, 400, "invalid_parameters",
            fmt.Sprintf("Invalid parameters: %v. Check ?limit=N&url=substring", err))
        return
    }

    // Handle request
    // ...
}
```

---

## 🔒 Security

### Input Validation

- ✅ **Validate ALL external inputs:**
  - HTTP parameters (query, path, body)
  - File paths (prevent path traversal)
  - URLs (prevent SSRF)
  - User input (prevent injection)

- ✅ **Whitelist, not blacklist:** Define what's allowed, reject everything else
- ✅ **Sanitize before use:** Escape, validate format, check ranges

See [validation-patterns.md](validation-patterns.md) for detailed validation patterns.

### Authentication & Authorization

- ✅ **Check authentication** on all protected endpoints
- ✅ **Check authorization:** User has permission for this action
- ✅ **Constant-time comparison** for secrets (prevent timing attacks)
- ✅ **Don't log secrets:** Redact tokens, keys, passwords from logs

### Rate Limiting

- ✅ **All upload endpoints** must have rate limits
- ✅ **Per-client limiting:** Prevent single client DoS
- ✅ **Document limits:** Tell users what the limits are
- ✅ **Return appropriate status:** 429 Too Many Requests

### Origin Security

- ✅ **CORS configured correctly:** Validate Host and Origin headers
- ✅ **PostMessage with origin:** NEVER use `'*'`, use specific origin
- ✅ **DNS rebinding protection:** Validate Host header matches localhost variants

---

**Last updated:** 2026-02-03
**See also:** [README.md](README.md) — Navigation and index

