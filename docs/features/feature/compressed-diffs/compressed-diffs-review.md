# Compressed State Diffs -- Technical Review

**Spec reviewed:** `docs/ai-first/tech-spec-compressed-diffs.md`
**Implementation reviewed:** `cmd/dev-console/ai_checkpoint.go`, `ring_buffer.go`, `types.go`, `tools.go`, `ai_checkpoint_test.go`, `ai_checkpoint_e2e_test.go`
**Reviewer:** Principal engineer review (automated)
**Date:** 2026-01-26

---

## 1. Executive Summary

The compressed diffs spec is well-designed for its core purpose: giving AI agents a token-efficient feedback loop after code edits. The checkpoint-by-position approach is sound and the implementation is largely faithful to the spec. However, the implementation has a critical lock-nesting hazard where `CheckpointManager.mu` is held while acquiring `server.mu.RLock()` and `capture.mu.RLock()` inside `GetChangesSince`, and the spec's claim that noise filtering integrates with the console diff is unimplemented -- console entries flow through unfiltered.

---

## 2. Critical Issues (Must Fix Before Shipping)

### 2.1. Lock Nesting Creates Deadlock Risk

**Spec reference:** "Edge Cases -- Concurrent access"

The spec states: "Lock ordering is always checkpoint lock first, buffer lock second." The implementation follows this ordering in `GetChangesSince` (acquires `cm.mu.Lock()`, then calls `computeConsoleDiff` which acquires `server.mu.RLock()`). However, `buildKnownEndpoints` at line 805 acquires `capture.mu.RLock()` *while `cm.mu` is still held* from the outer `GetChangesSince` call. Meanwhile, `DetectAndStoreAlerts` at line 879 acquires `cm.mu.Lock()` directly.

The actual hazard: if any code path ever holds `capture.mu` (write lock) and then calls a method that tries to acquire `cm.mu`, you get a deadlock. Today this does not happen because `DetectAndStoreAlerts` is called from the performance path which does not hold `capture.mu`. But this is a fragile invariant with no compile-time or runtime enforcement.

**Recommendation:** Document the lock ordering as a mandatory contract in a `LOCKING.md` or equivalent. Better: release `cm.mu` before computing diffs (snapshot the checkpoint data you need, unlock, compute diffs, re-lock only to advance the auto-checkpoint). This eliminates the nested lock entirely.

### 2.2. Noise Filtering Not Integrated with Console Diff

**Spec reference:** "Integration Points -- Noise Filtering"

The spec says: "The console diff skips entries that match active noise rules." Grep confirms there are zero references to noise matching in `ai_checkpoint.go`. Every console entry is processed regardless of noise rules. This means agents receive alerts about browser extension noise, HMR updates, and analytics errors -- precisely the signals the noise system was built to suppress.

**Recommendation:** Inject the `NoiseFilter` into `CheckpointManager` at construction time. In `computeConsoleDiff`, skip entries where `noiseFilter.IsNoise(entry)` returns true. This is straightforward because the noise filter is already thread-safe with its own mutex.

### 2.3. TOCTOU Race in Buffer Reads

**Spec reference:** "Performance Constraints"

In `computeConsoleDiff` (lines 462-465), the code reads `server.entries` and `server.logTotalAdded` under `server.mu.RLock()`, releases the lock, then slices `entries[available-toRead:]`. But `entries` is a Go slice header -- it shares the underlying array with `server.entries`. After the lock is released, a concurrent `addEntries` call can modify the backing array if rotation occurs (line 517: `s.entries = s.entries[len(s.entries)-s.maxEntries:]`). The slice reassignment creates a new backing array, so the old slice remains valid in memory, but this relies on Go's garbage collector keeping the old array alive. This is safe in practice because Go slices are value types and `newEntries` holds a reference to the old backing array, preventing collection. However, the pattern is error-prone and the **same code in `computeNetworkDiff`** (line 562) and `computeWebSocketDiff` (line 638) copies the slice header from `capture` buffers which use the same rotation pattern.

**Recommendation:** Copy the entries into a new slice while the lock is held, or keep the lock for the duration of the diff computation. Given the spec's 10ms SLO for 1000 entries, holding a read lock for the computation is acceptable and eliminates the subtle correctness concern.

### 2.4. Unbounded `KnownEndpoints` Map Growth

**Spec reference:** "Data Model -- Checkpoint"

Each checkpoint stores `KnownEndpoints map[string]endpointState`. The `buildKnownEndpoints` method (line 796) copies all existing endpoints and adds all current network bodies. Over a long session with varied API traffic, this map grows without bound. For a single-page app hitting hundreds of distinct endpoints (pagination, UUIDs in paths, etc.), each checkpoint could accumulate thousands of entries.

The spec claims "Memory for checkpoints: under 100KB (20 checkpoints at ~5KB each)." With unbounded endpoint maps, this guarantee is violated under realistic traffic.

**Recommendation:** Cap `KnownEndpoints` at 200 entries per checkpoint with LRU eviction. Additionally, consider normalizing UUID path segments (e.g., `/api/users/abc123-def456` becomes `/api/users/{id}`) similar to how `FingerprintMessage` normalizes message content. The `extractURLPath` function already strips query params but does not normalize path parameters.

---

## 3. Recommendations (Should Consider)

### 3.1. Token Count Calculation is Double-Serializing

**Spec reference:** "Diff Response -- estimated token count"

Line 352 marshals the entire `DiffResponse` to JSON solely to calculate `len(jsonBytes) / 4`. This response is then marshaled *again* in `toolGetChangesSince` at line 1364. For a 2KB response this is negligible, but for pathological cases (50 entries per category, 4 categories) the first marshal is wasted work.

**Recommendation:** Calculate token count after the final marshal, or estimate from the individual section sizes without a full marshal. Alternatively, set `TokenCount` to 0, marshal once, then update the field in the raw JSON bytes.

### 3.2. First Call Returns Entire Buffer (Spec Ambiguity)

**Spec reference:** "Behavior -- First call (no prior checkpoint)"

The spec says: "the server creates an initial checkpoint at the beginning of all buffers and returns everything currently in the buffers." The implementation creates a checkpoint with `LogTotal: 0`, meaning the diff includes every entry since server start. For a server running for hours with 1000 log entries, the first `get_changes_since` call returns all 1000 entries. This can be 30KB+ -- the exact problem the tool is designed to avoid.

**Recommendation:** Consider creating the first auto-checkpoint at the *current* position (returning an empty diff) rather than at position 0. The agent hasn't seen any data yet, so it has no baseline to diff against. The first call could return a summary like "Checkpoint established. Call again to see changes." Alternatively, keep the current behavior but document it prominently so agents know to call once to establish a baseline before beginning their edit-check loop.

### 3.3. Severity Filtering Semantics are Inconsistent

**Spec reference:** "Tool Interface -- Parameters"

The `severity` parameter accepts `"all"`, `"warnings"`, and `"errors_only"`. But the spec and implementation treat these as *minimum* filters while the naming suggests *exact* matches. When `severity="warnings"` is passed, both warnings and errors are returned (implementation lines 512-514 only skip warnings for `errors_only`). The value `"warnings"` effectively means "warnings and above" -- this is confusing.

**Recommendation:** Rename to `min_severity` or document explicitly that `"warnings"` means "warnings and above." The current behavior is correct, just poorly named.

### 3.4. Timestamp Checkpoint Creates Empty `KnownEndpoints`

**Spec reference:** "Behavior -- Timestamp as checkpoint reference"

When the agent passes an ISO 8601 timestamp, `resolveTimestampCheckpoint` (line 393) creates a checkpoint with an empty `KnownEndpoints` map. This means every endpoint in the buffer appears as "new," and no failure regressions (was-200-now-500) can be detected. Timestamp-based checkpoints are therefore significantly less useful than named or auto checkpoints for network analysis.

**Recommendation:** When resolving a timestamp checkpoint, scan the network bodies before that timestamp to populate `KnownEndpoints`. This is a bounded scan (max 100 bodies) and fits within the 1ms resolution SLO.

### 3.5. Persistent Checkpoint Feature is Specified but Unimplemented

**Spec reference:** "Integration Points -- Persistent Memory"

The spec says: "Named checkpoints can persist across sessions so the agent can compare against known-good states." The `ai_persistence.go` file provides the `SessionStore` infrastructure, but no code saves or loads checkpoints to/from disk. Server restart loses all checkpoints.

**Recommendation:** Implement persistence for named checkpoints using the existing `SessionStore.Save/Load` pattern. Auto-checkpoints should not persist (they are ephemeral by design). Priority: medium -- this enables the "compare against last deploy" workflow described in the checkpoint naming conventions.

### 3.6. `WebSocket.Errors` Should Elevate to Severity "error"

**Spec reference:** "Severity hierarchy"

The `determineSeverity` function (line 734) only checks `Console.Errors` and `Network.Failures` for "error" severity. WebSocket error events (`event: "error"`) are not checked, so a page with only WebSocket errors reports severity "clean" (assuming no disconnections). This seems wrong -- a WebSocket error event is an error.

**Recommendation:** Add a check for `len(resp.WebSocket.Errors) > 0` in the "error" severity branch of `determineSeverity`.

### 3.7. `currentClientID` Set Via Mutable Field is Not Concurrency-Safe

**Spec reference:** Not in spec; found in implementation (`tools.go` line 116)

The HTTP handler sets `h.toolHandler.currentClientID = clientID` on a shared `ToolHandler` instance. If two HTTP requests arrive concurrently (different clients), they race on this field. The MCP stdio path is single-threaded (one reader), but the HTTP path (`HandleHTTP`) is not.

**Recommendation:** Pass `clientID` as a parameter through the tool dispatch chain instead of storing it as mutable state on `ToolHandler`. This eliminates the race entirely.

---

## 4. Implementation Roadmap

Ordered by dependency and risk, not by severity alone.

### Phase 1: Fix Concurrency Hazards (1-2 days)

1. **Refactor `GetChangesSince` to snapshot-then-compute.** Under `cm.mu`, snapshot the checkpoint data (copy `cp`). Release `cm.mu`. Compute diffs (acquiring buffer read locks as needed). Re-acquire `cm.mu` to advance auto-checkpoint. This eliminates the nested lock.
2. **Fix `currentClientID` race.** Thread `clientID` as a parameter through `handleToolCall` and into each tool function.
3. **Verify with `go test -race`.** The existing `race_test.go` build tag ensures the race detector runs in CI. Add a test that calls `GetChangesSince` from 10 goroutines concurrently while another goroutine adds entries.

### Phase 2: Correctness Gaps (1-2 days)

4. **Integrate noise filtering into `computeConsoleDiff`.** Inject `*NoiseFilter` into `CheckpointManager`. Before fingerprinting a console entry, check `noiseFilter.IsConsoleNoise(entry)` and skip if true. Add test: insert a noise-matching entry and a real error; diff should only show the real error.
5. **Cap `KnownEndpoints` to 200 entries.** Add LRU eviction to `buildKnownEndpoints`. Add a test with 300 unique endpoints verifying the map stays at 200.
6. **Add WebSocket errors to severity "error".** One-line fix in `determineSeverity`. Add test.

### Phase 3: Quality of Life (1 day)

7. **Populate `KnownEndpoints` for timestamp checkpoints.** Scan network bodies before the target timestamp when resolving a timestamp-based checkpoint.
8. **Eliminate double-marshal for token count.** Marshal once, calculate byte length, set `TokenCount` in the response struct after marshaling.
9. **Consider first-call behavior.** Either document the current "return everything" semantics or change to "establish checkpoint, return clean."

### Phase 4: Persistence (2 days)

10. **Implement checkpoint persistence.** Serialize named checkpoints (not auto) to `.gasoline/checkpoints.json` via `SessionStore`. Load on startup. Add integration test: create checkpoint, simulate restart (new `CheckpointManager` with loaded state), verify diff against the restored checkpoint.

### Phase 5: Polish (1 day)

11. **Rename `severity` parameter to `min_severity`** in the MCP tool schema (breaking change -- gate behind next major version or support both).
12. **Add path parameter normalization** to `extractURLPath` (collapse UUID segments to `{id}`).
13. **Copy buffer slices under lock** in all `compute*Diff` methods to eliminate the TOCTOU concern.

---

## 5. Test Coverage Assessment

The test suite is thorough for the happy path. All 19 spec test scenarios have corresponding tests. The E2E tests exercise the full MCP JSON-RPC stack.

**Gaps identified:**

| Gap | Test Needed |
|-----|-------------|
| Concurrent `GetChangesSince` + `addEntries` | Race detector test with 10+ goroutines |
| Noise filtering in diff | Insert noise entry + real error, assert diff only contains real error |
| `KnownEndpoints` growth | Add 300 endpoints, verify cap enforced |
| Timestamp checkpoint with populated endpoints | Timestamp query after known-good traffic, verify failures detected |
| WebSocket error severity | WS error-only scenario, assert severity "error" |
| Buffer overflow (ring wrapped past checkpoint) | Fill buffer past capacity, checkpoint references evicted position |
| Named checkpoint LRU eviction (21st checkpoint evicts 1st) | Create 21 checkpoints, verify first is gone |

The existing buffer overflow test (spec scenario 11) appears in `ai_checkpoint_test.go` but should be verified with the E2E path as well.

---

## 6. Summary of Verdicts

| Issue | Severity | Effort | Verdict |
|-------|----------|--------|---------|
| Lock nesting hazard | Critical | Medium | Must fix |
| Noise filtering missing | Critical | Low | Must fix |
| TOCTOU buffer read | High | Low | Must fix |
| Unbounded KnownEndpoints | High | Low | Must fix |
| currentClientID race | High | Low | Must fix |
| WebSocket error severity | Medium | Trivial | Should fix |
| Double-marshal token count | Low | Low | Should fix |
| First-call returns everything | Low | Low | Should document or fix |
| Timestamp checkpoint empty endpoints | Medium | Medium | Should fix |
| Checkpoint persistence | Medium | Medium | Should implement |
| Severity parameter naming | Low | Low | Should rename |
| Path parameter normalization | Low | Medium | Nice to have |
