# Error Handling and Recovery Standards

> **Error handling, logging, recovery patterns, and resource cleanup**

**Scope:** Error handling strategies, logging standards, retry/backoff patterns, graceful degradation, and proper resource cleanup.

**Related Standards:**
- [api-and-security.md](api-and-security.md) — HTTP error handling and rate limiting
- [memory-and-performance.md](memory-and-performance.md) — Memory management and cleanup
- [code-quality.md](code-quality.md) — Code organization and readability

---

## 🚨 Error Handling (General)

### Error Logging

- ✅ **Log cause AND context:**
  ```go
  if err != nil {
      log.Printf("Failed to parse request: %v (url=%s, method=%s, body_size=%d)",
          err, req.URL, req.Method, bodySize)
      return err
  }
  ```

- ✅ **Include variable values:** Log the actual values that caused the error
- ✅ **Structured logging preferred:** Use key=value format for easy parsing

### Error Handling Requirements

- ✅ **ALL operations that can error MUST have error handling**
  - File operations
  - Network operations
  - JSON marshal/unmarshal
  - Type conversions
  - Database operations

- ✅ **NO silent errors:** Never `_ = operation()`

- ✅ **Catch and log ALL errors:**
  ```typescript
  try {
      await riskyOperation();
  } catch (err) {
      console.error('[Gasoline] Operation failed:', err, {context: value});
      throw err; // Or handle appropriately
  }
  ```

### Error Response Standards

- ✅ **Detailed messages:** Explain what's wrong
- ✅ **Actionable guidance:** Tell user how to fix it
- ✅ **Include context:** Relevant values (sanitized of secrets!)
- ✅ **Error codes:** Use consistent error codes/types

**Example:**
```go
return fmt.Errorf("failed to unmarshal network body: %w (url=%s, content_type=%s, body_length=%d). Check that response is valid JSON",
    err, entry.URL, entry.ContentType, len(entry.Body))
```

---

## 💾 Resource Management

### File Handles

- ✅ **Always close files:** Use defer
- ✅ **Check close errors on writes:** Data might not be flushed
- ✅ **Read close errors can be ignored:** Reading doesn't modify

```go
// Write operation - check close error
file, err := os.Create(path)
if err != nil {
    return err
}
defer func() {
    if closeErr := file.Close(); closeErr != nil {
        log.Printf("Error closing file: %v", closeErr)
    }
}()

// Read operation - can ignore close error
file, err := os.Open(path)
if err != nil {
    return err
}
defer file.Close() // Read-only, safe to ignore error
```

### HTTP Connections

- ✅ **Always defer Body.Close():** Prevents connection leaks
- ✅ **Use connection pooling:** http.DefaultClient handles this
- ✅ **Set timeouts:** On client, not just context

### Cleanup Patterns

- ✅ **LIFO order:** Cleanup in reverse order of allocation
- ✅ **Idempotent cleanup:** Safe to call multiple times
- ✅ **Error on cleanup:** Log but don't block shutdown

---

## 🔄 Error Recovery

### Retry Logic

- ✅ **Retry transient errors:** Network blips, temporary unavailability
- ✅ **Don't retry permanent errors:** 400 errors, auth failures
- ✅ **Exponential backoff:** Don't hammer failing services
- ✅ **Max retry limit:** Don't retry forever

---

## 🎯 Graceful Degradation

- ✅ **Fallback behavior:** What happens when dependency fails?
- ✅ **Circuit breakers:** Stop calling failing services
- ✅ **Partial functionality:** Core features work even if optional features fail
- ✅ **User communication:** Tell user what's degraded

---

## 📊 Observability in Error Scenarios

### Logging

- ✅ **Structured logging:** Key=value format
- ✅ **Log levels:** Error, warn, info, debug
- ✅ **Log context:** Include relevant values
- ✅ **Don't log secrets:** Redact sensitive data
- ✅ **Log strategy documented:** What goes where (stdout/stderr/file)

**Example:**
```go
log.Printf("[gasoline] Request processed: status=%d url=%s duration=%dms client=%s",
    status, sanitizeURL(url), duration, clientID)
```

### Metrics

- ✅ **Track key metrics:**
  - Request counts
  - Error rates
  - Response times
  - Resource usage
- ✅ **Expose via endpoint:** `/diagnostics` or `/metrics`

### Debug Mode

- ✅ **Debug flag/env var:** Enable verbose logging
- ✅ **Debug output controlled:** Off by default, opt-in
- ✅ **Document how to enable:** README or docs

---

**Last updated:** 2026-02-03
**See also:** [README.md](README.md) — Navigation and index

