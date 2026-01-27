# P0 Issues — Fix Before Any New Feature Work

Identified during comprehensive spec review (2026-01-26). All four are in **existing production code**.

---

## 1. Path Traversal Vulnerabilities (Security)

User-supplied parameters are joined via `filepath.Join` without validating the resolved path stays inside the project directory. An AI agent (or attacker on localhost) can read/write arbitrary files.

**Affected code:**

| File | Parameter | Impact |
|------|-----------|--------|
| `ai_persistence.go` | `namespace` + `key` in `Save`/`Load`/`List`/`Delete` | Read/write anywhere on disk |
| `export_sarif.go` | `save_to` in `saveSARIFToFile` | Write anywhere (symlink bypass) |

**Fix pattern** (same for all):
```go
resolved, _ := filepath.EvalSymlinks(filepath.Clean(fullPath))
base, _ := filepath.EvalSymlinks(filepath.Clean(projectDir))
if !strings.HasPrefix(resolved, base+string(os.PathSeparator)) {
    return errors.New("path escapes project directory")
}
```

**Reviews**: persistent-memory-review.md (issue #1), sarif-export-review.md (issue #2)

---

## 2. Memory Eviction is Cosmetic (Correctness)

The entire memory enforcement system doesn't actually free memory. Slice reslicing retains the backing array — the GC can never reclaim evicted entries.

**Affected code**: `memory.go` — `evictBuffers()` does `v.networkBodies = v.networkBodies[n:]`

**What happens**: Eviction reports fewer elements, but process RSS never decreases. Under sustained load, the server will OOM despite eviction appearing to work.

**Fix**:
```go
surviving := make([]NetworkBody, len(v.networkBodies)-n)
copy(surviving, v.networkBodies[n:])
v.networkBodies = surviving
```

Same fix needed for `wsEvents`, `enhancedActions`, and any other buffer using reslicing.

**Review**: memory-enforcement-review.md (issue #1)

---

## 3. `currentClientID` Data Race (Correctness)

`currentClientID` on `ToolHandler` is set/cleared per-HTTP request with no mutex. Multiple concurrent `/mcp` calls will race on this field.

**Affected code**: `tools.go` ~line 156:
```go
h.toolHandler.currentClientID = clientID
defer func() { h.toolHandler.currentClientID = "" }()
```

**What happens**: Under concurrent MCP requests, one client's ID bleeds into another client's tool execution. Audit log entries attribute actions to the wrong client.

**Fix**: Pass `clientID` as a parameter through the call chain instead of storing on the struct. This eliminates the shared mutable state entirely.

**Review**: enterprise-audit-review.md (issue C2)

---

## 4. Redaction Stats Data Race (Correctness)

`RedactString` holds an `RLock` but writes to `stats.RedactionsByPattern` via `incrementStat`. This is a read-write lock violation on the MCP hot path.

**Affected code**: `redaction.go` — `RedactString` method, called from `main.go:307-309`

**What happens**: Concurrent MCP tool calls trigger concurrent redaction. The stats write under RLock will corrupt the stats map or panic.

**Fix**: Use `atomic.Int64` for per-pattern counters, or accumulate stats locally and flush under a separate write lock after releasing the read lock.

**Review**: redaction-patterns-review.md (issue C1)

---

## 5. Network Error Messages Not Captured (Data Quality)

The extension's network error handler sends errors with `payload.error`, but `bridge.js` only extracts messages from `payload.message` or `payload.args[0]`. Result: 140+ network errors arrive at the server with empty `message` fields, making them invisible to AI debugging.

**Affected code:**
- `inject.js` line 272: `postLog({ error: error.message, ... })`
- `lib/bridge.js` line 30: Only checks `payload.message` and `payload.args[0]`, ignores `payload.error`

**What happens**: All network errors (failed API calls, CORS issues, timeouts) lose their error messages during capture. Users see errors reported in logs but with empty text fields. UAT cannot properly test error handling.

**Fix**:
```javascript
message: payload.message || payload.error || (payload.args?.[0] != null ? String(payload.args[0]) : '')
```

**Status**: Fixed in lib/bridge.js (2026-01-26)

---

## Summary

| # | Category | Severity | Effort |
|---|----------|----------|--------|
| 1 | Path traversal | Security — arbitrary file R/W | Small (validation helper + apply to call sites) |
| 2 | Memory eviction no-op | Correctness — eventual OOM | Small (copy instead of reslice, ~4 call sites) |
| 3 | `currentClientID` race | Correctness — wrong audit attribution | Medium (thread `clientID` through call chain) |
| 4 | Redaction stats race | Correctness — map corruption/panic | Small (atomic counters or local accumulate) |
| 5 | Network error messages missing | Data quality — silent errors | Small (add `payload.error` fallback) ✅ **FIXED** |
