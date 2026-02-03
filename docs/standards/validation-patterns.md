# Data Validation Patterns

> **Validation standards, trust models, and Gasoline-specific validation patterns**

**Scope:** Input validation at system boundaries, validation trust models, and Gasoline-specific validation patterns for all external inputs.

**Related Standards:**
- [data-design.md](data-design.md) — Data model design
- [api-and-security.md](api-and-security.md) — Security and input validation
- [error-and-recovery.md](error-and-recovery.md) — Error handling for validation failures

---

## Validation Boundaries (Trust Model)

- ✅ **Validate at system boundaries:** User input, HTTP requests, extension messages
- ✅ **Trust internal code:** No need to validate data from trusted internal packages
- ✅ **Never trust external data:** Always validate browser/user/network input

---

## Gasoline-Specific Validation Patterns

### Timestamps

```go
// Validate timestamp format (RFC3339 or RFC3339Nano)
func validateTimestamp(ts string) error {
    if ts == "" {
        return nil // Optional field
    }
    // Try both formats
    if _, err := time.Parse(time.RFC3339, ts); err != nil {
        if _, err := time.Parse(time.RFC3339Nano, ts); err != nil {
            return fmt.Errorf("invalid timestamp format: %s (expected RFC3339 or RFC3339Nano)", ts)
        }
    }
    return nil
}
```

### URLs

```go
// Validate URL (prevent SSRF, path traversal)
func validateURL(urlStr string) error {
    if urlStr == "" {
        return fmt.Errorf("URL cannot be empty")
    }

    u, err := url.Parse(urlStr)
    if err != nil {
        return fmt.Errorf("invalid URL format: %w", err)
    }

    // Check scheme
    if u.Scheme != "http" && u.Scheme != "https" && u.Scheme != "ws" && u.Scheme != "wss" {
        return fmt.Errorf("unsupported URL scheme: %s (allowed: http, https, ws, wss)", u.Scheme)
    }

    // Prevent localhost URLs if from external source (SSRF prevention)
    if isLocalhostURL(u) && !isFromTrustedSource() {
        return fmt.Errorf("localhost URLs not allowed from external sources")
    }

    return nil
}
```

### Correlation IDs

```go
// Validate correlation ID format (prefix_timestamp_randomhex)
func validateCorrelationID(id string) error {
    if id == "" {
        return fmt.Errorf("correlation ID cannot be empty")
    }

    // Expected format: exec_1234567890_abc123
    parts := strings.Split(id, "_")
    if len(parts) < 3 {
        return fmt.Errorf("invalid correlation ID format: %s (expected: prefix_timestamp_random)", id)
    }

    // Validate prefix is known
    validPrefixes := []string{"exec", "nav", "dom", "highlight"}
    if !contains(validPrefixes, parts[0]) {
        return fmt.Errorf("unknown correlation ID prefix: %s", parts[0])
    }

    return nil
}
```

### Numeric Ranges

```go
// Validate limit parameter (1 to maxLimit)
func validateLimit(limit, maxLimit int) error {
    if limit < 0 {
        return fmt.Errorf("limit cannot be negative: %d", limit)
    }
    if limit > maxLimit {
        return fmt.Errorf("limit exceeds maximum: %d > %d", limit, maxLimit)
    }
    return nil
}
```

### JSON Schema Validation

```go
// Validate MCP message structure
func validateMCPRequest(req *JSONRPCRequest) error {
    if req.JSONRPC != "2.0" {
        return fmt.Errorf("invalid JSON-RPC version: %s (expected: 2.0)", req.JSONRPC)
    }

    if req.Method == "" {
        return fmt.Errorf("method cannot be empty")
    }

    if req.ID == nil {
        return fmt.Errorf("id cannot be null for requests")
    }

    return nil
}
```

---

## Validation Rules

- ✅ **Validate early:** At function entry, before processing
- ✅ **Return specific errors:** "invalid timestamp" not "invalid input"
- ✅ **Include actual value:** "invalid limit: 5000" not just "invalid limit"
- ✅ **Suggest fix:** "expected: 1-1000" in error message
- ✅ **Fail fast:** Don't continue with invalid data

### When to Skip Validation

- ✅ **Internal package calls:** Data from internal/types can be trusted
- ✅ **Already validated:** Don't re-validate at every layer
- ✅ **Performance-critical paths:** Pre-validated data in hot loops

---

**Last updated:** 2026-02-03
**See also:** [README.md](README.md) — Navigation and index

