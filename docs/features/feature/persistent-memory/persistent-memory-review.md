---
status: shipped
scope: feature/persistent-memory/review
ai-priority: high
tags: [review, issues]
relates-to: [tech-spec.md, product-spec.md]
last-verified: 2026-01-31
---

# Review: Persistent Cross-Session Memory Spec

**Reviewer**: Principal Engineer Review
**Spec**: `docs/ai-first/tech-spec-persistent-memory.md`
**Date**: 2026-01-26

---

## Executive Summary

The persistent memory system is already implemented in `ai_persistence.go` and provides a solid file-backed key-value store with size limits, background flushing, and session tracking. The design choice of `.gasoline/` as a project-local directory following the `.vscode/` convention is pragmatic and correct. However, there are critical issues with the concurrent access model (file locking is spec'd but not implemented), a path traversal vulnerability in the namespace/key parameters, and the background flush goroutine has a race condition with `Shutdown()` that can cause writes to a closed channel.

---

## Critical Issues (Must Fix Before Implementation)

### 1. Path Traversal via Namespace and Key Parameters

**Section**: "Tool Interface > session_store"

The `Save`, `Load`, `List`, and `Delete` methods construct file paths by joining `projectDir + namespace + key + ".json"`. The MCP tool handler (`HandleSessionStore`, line 600) passes user-supplied `namespace` and `key` directly to these methods. A malicious or buggy AI agent could pass `namespace: "../../../etc"` and `key: "passwd"` to read arbitrary files, or `namespace: "../../.ssh"` and `key: "authorized_keys"` with a save action to write arbitrary files.

The implementation at line 250-256:
```go
nsDir := filepath.Join(s.projectDir, namespace)
os.MkdirAll(nsDir, dirPermissions)
filePath := filepath.Join(nsDir, key+".json")
os.WriteFile(filePath, data, filePermissions)
```

`filepath.Join` does NOT prevent traversal -- `filepath.Join("/project/.gasoline", "../../../etc")` resolves to `/etc`.

**Fix**: After constructing `nsDir` and `filePath`, validate that they are within `projectDir` using `filepath.Rel` or `strings.HasPrefix(filepath.Clean(filePath), filepath.Clean(s.projectDir))`. Reject any path that escapes. This is a security-critical fix.

### 2. File Locking is Spec'd but Not Implemented

**Section**: "Security and Privacy"

The spec says: "If multiple Gasoline server instances run for the same project, the first one takes a file lock (`flock()`) on meta.json. The second instance operates in read-only mode for persistence."

The implementation in `ai_persistence.go` has no file locking. `NewSessionStoreWithInterval` (line 112) opens `meta.json`, increments the session count, and writes it back without any lock. Two concurrent server instances will both read the same session count, both increment, and the last writer wins (lost update). Worse, concurrent writes to the same namespace/key file can produce corrupted JSON.

**Fix**: Implement file locking using `syscall.Flock` on Unix or `LockFileEx` on Windows. Lock `meta.json` on startup with `LOCK_EX | LOCK_NB`. If the lock fails, set a `readOnly` flag and skip all write operations. This requires platform-specific code (the project already has `proc_unix.go` and `proc_windows.go` as precedent for platform-specific files).

### 3. Background Flush Race with Shutdown

**Section**: "Background Flush" and implementation

The `Shutdown()` method (line 514) closes `s.stopCh`, then calls `s.flushDirty()`. But the background goroutine (line 467) is in a `select` loop. The sequence can be:

1. Background goroutine's ticker fires, enters `flushDirty()`
2. Main thread calls `Shutdown()`, closes `stopCh`
3. Main thread calls `flushDirty()` -- now both the background goroutine and the shutdown path are flushing simultaneously

The `flushDirty()` method (line 481) acquires `dirtyMu`, copies the dirty map, clears it, releases the lock, then writes files. If both goroutines enter `flushDirty()` at the same time, the first one copies and clears the dirty map; the second one finds an empty map and returns. This is actually safe because of the mutex. However, there is a subtler issue: the background goroutine could be in the middle of the file-write loop (line 496-510) when `Shutdown()` returns and the process exits, causing truncated files.

**Fix**: After closing `stopCh`, wait for the background goroutine to exit before proceeding with the final flush. Use a `sync.WaitGroup` or a `done` channel:

```go
func (s *SessionStore) Shutdown() {
    close(s.stopCh)
    <-s.doneCh  // wait for background goroutine to exit
    s.flushDirty()  // final flush
    s.saveMeta()
}
```

### 4. `projectSize()` Holds Read Lock While Walking the Filesystem

**Section**: Implementation, line 540-552

`projectSize()` is called from `Save()` which holds the write lock (line 241). The `filepath.Walk` call (line 542) performs I/O under the write lock. On a slow filesystem (network mount, spinning disk, large directory), this blocks all other store operations for the duration of the walk.

The current walk is bounded by the 10MB total limit (at most a few hundred files), so this is unlikely to be a problem in practice. But it violates the "under 50ms" performance constraint for single file save.

**Fix**: Calculate project size periodically (every flush cycle) and cache the result. Use the cached value for save-time checks. This trades exactness for responsiveness. Alternatively, maintain a running total that is updated on save/delete.

---

## Recommendations (Should Consider)

### 5. No Input Validation on Namespace and Key Names

**Section**: "Tool Interface > session_store"

Beyond path traversal (Critical Issue #1), namespace and key values have no validation for:
- Empty strings (caught for save/load/delete but not for edge cases)
- Characters invalid in filenames on Windows (`<>:"/\|?*`)
- Very long names (filesystem limits are typically 255 bytes)
- Names starting with `.` (hidden files, could conflict with `.gasoline` itself)
- Names containing `/` (would create subdirectories unintentionally)

**Recommendation**: Validate namespace and key against a whitelist pattern: `^[a-zA-Z0-9_-]{1,64}$`. Reject anything else with a clear error message.

### 6. Error History Eviction Algorithm is O(n^2)

**Section**: Implementation, `enforceErrorHistoryCap()` lines 560-578

The function repeatedly scans the entire slice to find the oldest entry, removes it, and repeats. For evicting k entries from n, this is O(k*n). When the cap is 500 and we need to evict 1, this is O(500) -- acceptable. But the algorithm is unnecessarily complex.

**Recommendation**: Sort the slice once by `FirstSeen` (O(n log n)), then truncate. Or, since this only triggers when `len > maxErrorHistory`, which means exactly 1 entry over cap, the current linear scan for the single oldest entry is actually O(n), not O(n^2). The loop `for len(entries) > maxErrorHistory` executes exactly once. This is fine but the code's structure suggests the author expected multiple evictions. Simplify to a single-pass find-and-remove.

### 7. Corrupted JSON Recovery is Silent

**Section**: "Edge Cases" -- "Corrupted JSON: if a stored file can't be parsed, it's silently skipped on load."

The implementation handles this in `loadOrCreateMeta()` (line 192) by starting fresh. For `LoadSessionContext()`, individual namespace files that fail to parse are skipped. This is the correct behavior for resilience, but there is no logging or diagnostic output. When the AI agent calls `load_session_context` and gets empty baselines because the file was corrupted, it has no way to distinguish "no baselines saved" from "baselines corrupted."

**Recommendation**: Add a `warnings` field to the `SessionContext` response that includes messages like "baselines/login.json: corrupted, skipped" so the agent is aware.

### 8. `.gitignore` Modification is Not Atomic

**Section**: Implementation, `ensureGitignore()` lines 143-169

The method reads `.gitignore`, checks for `.gasoline`, and appends if missing. If two server instances start simultaneously (before file locking is implemented), both can read, both see `.gasoline` is missing, and both append -- resulting in duplicate entries. This is cosmetically annoying but not harmful.

More concerning: the method opens the file with `O_APPEND|O_WRONLY` and writes a newline + `.gasoline/\n`. If the write fails partway (disk full), `.gitignore` could end with a partial line. This is unlikely but worth noting.

**Recommendation**: Use atomic write (write to temp file, rename) for `.gitignore` modification, or accept the current behavior as low-risk.

### 9. The 10MB Total Limit May Be Too Small

**Section**: "Size Limits"

10MB total for the `.gasoline/` directory includes all baselines, noise config, error history, API schemas, and performance data. A single large API schema file could be 1MB. Error history at 500 entries with detailed fingerprints could be 500KB. Performance data for 20 endpoints could be 200KB. That leaves only ~8MB for baselines and other data.

For projects with many pages/routes (e-commerce with hundreds of product pages, dashboards with dozens of views), 10MB could be hit within a few sessions.

**Recommendation**: Increase to 25MB or make the limit configurable via an environment variable. The storage is local to the project and gitignored, so disk usage is the developer's own machine.

### 10. `LoadSessionContext` Reads Files Under Read Lock

**Section**: Implementation, line 372-453

`LoadSessionContext()` acquires `s.mu.RLock()` and then reads multiple files from disk (baselines directory listing, noise config, error history, API schema, performance data). This holds the read lock for the duration of all I/O operations, which blocks any concurrent write operations.

**Recommendation**: Read files without the lock (since they are only modified under the write lock, and file reads are atomic at the OS level for small files). Or copy the paths under the lock and release it before doing I/O.

### 11. Session Count Increments on Every Server Start

**Section**: "Session Lifecycle"

The session count increments in `loadOrCreateMeta()` every time the server starts, even if the server is immediately shut down or crashes. For development workflows where the server restarts on every file save (via `make dev` or similar), session counts will inflate rapidly and lose meaning.

**Recommendation**: Only increment the session count when `load_session_context` is first called, not on server startup. This ensures a session is only counted when an AI agent actually connects.

---

## Implementation Roadmap

1. **Fix path traversal vulnerability**: Add path validation in `Save`, `Load`, `List`, and `Delete` to ensure resolved paths are within `projectDir`. This is the highest-priority security fix. Add input validation for namespace/key character set.

2. **Add file locking**: Implement `flock()`-based locking on `meta.json` in a platform-specific file (`persistence_unix.go`, `persistence_windows.go`). Set `readOnly` flag if lock acquisition fails.

3. **Fix shutdown race**: Add a `done` channel to the background flush goroutine. Have `Shutdown()` wait for goroutine exit before performing the final flush.

4. **Write tests (TDD)**: Verify path traversal is rejected. Verify concurrent access with file locking. Verify shutdown flushes all dirty data. Verify corrupted JSON recovery. All 18 test scenarios from the spec.

5. **Cache project size**: Maintain a running total of project size, updated on save/delete, to avoid `filepath.Walk` under the write lock.

6. **Add warnings to SessionContext**: Include a `warnings` array for corrupted/skipped files.

7. **Defer session count increment**: Move session count increment from `loadOrCreateMeta` to `LoadSessionContext`.

8. **Consider increasing the 10MB limit**: Evaluate actual usage patterns after initial deployment. Make configurable via environment variable if needed.
