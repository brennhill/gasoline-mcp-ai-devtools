# Behavioral Baselines Spec Review

**Spec:** `docs/ai-first/tech-spec-behavioral-baselines.md`
**Reviewer:** Principal Engineer Review
**Date:** 2026-01-26

---

## Executive Summary

The spec addresses a real gap: vibe-coded projects with no test suite need a way to detect behavioral regressions after AI-driven changes. The data model is sound, path normalization is practical, and the "shapes not values" approach for JSON comparison is the right call. However, there are five critical issues -- most importantly, a feature overlap with two existing tools (`verify_fix` and `diff_sessions`) that will fragment the UX for AI agents, a path-traversal vulnerability in the baseline name parameter, and a disk persistence design that will corrupt data under concurrent writes. These must be resolved before implementation.

---

## Critical Issues (Must Fix)

### C1. Feature overlap with `verify_fix` and `diff_sessions` creates agent confusion

**Section:** How It Works, Tool Interface

The codebase already has two tools that perform before/after state comparison:

- **`verify_fix`** (`verify.go`): Captures baseline browser state (console errors, network errors, page URL, performance), then compares against current state. Returns a verdict (fixed, improved, regressed, unchanged). Uses `CaptureStateReader` interface.
- **`diff_sessions`** (`sessions.go`): Captures named snapshots of browser state (console errors/warnings, network requests, WS connections, performance, page URL). Compares two snapshots. Returns regressions, improvements, and a summary.

The proposed `save_baseline` / `compare_baseline` duplicates approximately 80% of what these tools already do. The differentiated features are: (a) response shape comparison (top-level field names + types), (b) parameterized path normalization, (c) latency percentile tracking, and (d) disk persistence with `~/.gasoline/baselines/`. Features (a) and (b) already exist in `APIContractValidator` (`validate_api` tool). Feature (d) already exists in `SessionStore` (`ai_persistence.go`).

An AI agent presented with `verify_fix`, `diff_sessions`, `validate_api`, AND `save_baseline`/`compare_baseline` will not reliably choose the right tool. This violates the product philosophy of keeping the tool surface small and composable.

**Fix:** Do not introduce new MCP tools. Instead, extend the existing `diff_sessions` tool with:
1. Add an optional `persist: true` parameter to `capture` action, writing to `~/.gasoline/baselines/<name>.json` via the existing `SessionStore` infrastructure.
2. Add a `tolerance` parameter to the `compare` action with the timing factor and allow-additional-network options.
3. Integrate the response shape comparison from `APIContractValidator` into the comparison diff.
4. Add latency percentile tracking (P50/P95/max) to the existing `SnapshotNetworkRequest` type.

This gives agents one tool (`diff_sessions`) that can do ephemeral OR persistent comparison, with all the behavioral baseline capabilities.

### C2. Path traversal in baseline name parameter

**Section:** Tool Interface > `save_baseline`, Persistence

The baseline name is used directly in a filesystem path: `~/.gasoline/baselines/<name>.json`. The spec does not mandate sanitization. A name like `../../.ssh/authorized_keys` would write outside the baselines directory.

The codebase has `sanitizeForFilename()` in `main.go` (line 421) that strips non-alphanumeric characters, but the spec does not reference it.

**Fix:** The spec must explicitly require:
1. Sanitize the name using the existing `sanitizeForFilename()` before any filesystem operation.
2. After joining the path, call `filepath.Rel(baselinesDir, fullPath)` and reject if it starts with `..`.
3. Reject empty names after sanitization.

### C3. Concurrent disk writes will corrupt baseline files

**Section:** Edge Cases > Concurrent access, Persistence

The spec says: "Saves take a write lock. Compares take a read lock on baselines and a read lock on the buffer." This protects the in-memory map, but the disk write (`os.WriteFile`) is performed inside the write lock on the in-memory map. If the server crashes mid-write, the file is truncated/corrupt, and the next startup will load garbage.

Additionally, `os.WriteFile` is not atomic -- it truncates then writes. If the process is killed between truncation and completion, the baseline is lost with no recovery path.

**Fix:** Use atomic write (write to temp file, then `os.Rename`):
```go
func atomicWriteJSON(path string, data interface{}) error {
    tmp := path + ".tmp"
    f, err := os.Create(tmp)
    if err != nil {
        return err
    }
    enc := json.NewEncoder(f)
    if err := enc.Encode(data); err != nil {
        f.Close()
        os.Remove(tmp)
        return err
    }
    if err := f.Close(); err != nil {
        os.Remove(tmp)
        return err
    }
    return os.Rename(tmp, path)
}
```

This pattern is already implicitly used elsewhere in the codebase (the `SessionStore` writes individual files). Make it explicit and shared.

### C4. Startup loading of 50 baselines under 200ms is not validated against the 100KB-per-file limit

**Section:** Performance Constraints

50 baselines x 100KB = 5MB of JSON to parse at startup. On a cold filesystem (no page cache), reading 5MB of small files plus JSON parsing 50 times will exceed 200ms on spinning disks and on some CI environments. The spec states "under 200ms for 50 baselines" but provides no fallback behavior if this is exceeded.

More importantly, the startup load blocks the MCP handler from responding. If it takes 500ms, the MCP client (Claude Code, Cursor) may time out the initialization handshake.

**Fix:**
1. Load baselines lazily: on first access to `compare_baseline` or `list_baselines`, not at server startup. The in-memory map starts empty; individual baselines are loaded and cached on demand.
2. Alternatively, load in a background goroutine and have `compare_baseline` block until loading completes (with a timeout).
3. Add a `loaded_at` field to track when each baseline was loaded, enabling future LRU eviction.

### C5. `~/.gasoline/baselines/` conflicts with project-scoped `.gasoline/` storage

**Section:** Persistence

The spec stores baselines in `~/.gasoline/baselines/` (user home directory). But the existing `SessionStore` in `ai_persistence.go` stores data in `<project>/.gasoline/` (project root). This means:
- Baselines are global across all projects -- a baseline named "dashboard-load" saved in Project A will collide with one saved in Project B.
- Baselines are decoupled from the project they describe, making them meaningless after directory changes.
- The `.gitignore` auto-management in `SessionStore` does not cover `~/.gasoline/`.

**Fix:** Store baselines in `<project>/.gasoline/baselines/`, consistent with the existing `SessionStore` pattern. The `SessionStore` already handles directory creation, `.gitignore`, and size management. Use it directly rather than creating a parallel persistence mechanism.

---

## Recommendations (Should Consider)

### R1. Response shape comparison should go deeper than top-level fields

**Section:** Response Shape Extraction

Only recording top-level field names and types misses common regressions:
- An endpoint returns `{ data: { users: [...] } }`. A bug changes it to `{ data: { users: null } }`. Both have shape `{ data: object }` -- no regression detected.
- A pagination field moves from `{ meta: { total: 100 } }` to `{ meta: {} }` -- shape is still `{ meta: object }`.

**Recommendation:** Recurse to depth 2 (configurable). Record nested keys for object-typed fields: `{ data: { users: "array", count: "number" } }`. This catches the most common shape regressions without unbounded recursion. The existing `APIContractValidator` already does this -- reuse its shape extraction.

### R2. Error fingerprinting strategy is undefined

**Section:** Console baseline

The spec says errors are "recognized by fingerprint" but does not define the fingerprinting algorithm. Without a spec, tests cannot validate correctness. The existing `normalizeVerifyErrorMessage()` in `verify.go` (line 701) strips UUIDs, timestamps, numeric IDs, and file:line references. The `ClusterManager` in `ai_noise.go` uses a different normalization.

**Recommendation:** Specify the fingerprinting algorithm explicitly. Reuse `normalizeVerifyErrorMessage()` from `verify.go` -- it handles the common cases. Document which dynamic values are normalized and which are preserved.

### R3. Missing endpoint regression: structural changes (field added/removed)

**Section:** Comparing Against a Baseline

The spec detects: status code changes, latency regressions, new console errors, WS disconnections. It does NOT explicitly call out:
- A field present in the baseline shape is missing in the current response (field removal).
- A field's type changed (e.g., `"count": "number"` becomes `"count": "string"`).
- A new required field appeared (harder to detect, but worth flagging).

These are the most valuable regressions for vibe-coded apps. The spec's "Response Shape Extraction" section implies this, but the "Comparing Against a Baseline" section only lists status/latency/console/WS.

**Recommendation:** Add explicit regression types:
- `shape_field_removed`: A field in the baseline shape is absent in the current response.
- `shape_type_changed`: A field's type differs from the baseline.
- `shape_field_added`: A field not in the baseline appeared (info-level, not regression by default).

### R4. Timing baseline should use per-endpoint latency, not global percentiles

**Section:** Data Model > Timing baseline

The spec computes "P50, P95, and max latency across all network requests" as a single global timing baseline. This is too coarse: a slow image CDN inflates P95, making fast API regressions invisible.

**Recommendation:** Store per-endpoint latency statistics (already captured as `average_latency` in the network baselines). Use the per-endpoint average from the baseline as the comparison target, with the timing factor applied per-endpoint. The global P50/P95/max can remain as a summary metric but should not be the regression trigger.

### R5. `overwrite: false` default creates poor agent ergonomics

**Section:** Tool Interface > `save_baseline`

When an AI agent iterates (save -> make changes -> compare -> fix -> save again), it must remember to pass `overwrite: true` on every subsequent save. Agents will frequently forget, get an error, then retry with `overwrite: true`. This wastes a tool call round-trip.

**Recommendation:** Default `overwrite` to `true`. The version counter already preserves the history of updates. If the user wants to protect a baseline from accidental overwrite, add a `locked: true` field to the baseline that must be explicitly unlocked first.

### R6. Test scenario 23 ("concurrent save and compare") needs more specificity

**Section:** Test Scenarios

"No race conditions" is not a testable assertion. Specify what to test:
- Two goroutines: one calling `save_baseline("x", overwrite=true)` in a loop, the other calling `compare_baseline("x")` in a loop. After 1000 iterations, no panics, no corrupt state, all responses are well-formed JSON.
- One goroutine saving while another deletes the same name. Deletion should either succeed (baseline removed) or fail (save completed first). Never a partial state.

### R7. The 50-baseline limit should be configurable

**Section:** Persistence

50 is reasonable for most projects but arbitrary for larger ones. The existing architecture uses constants at the top of each file (see `types.go` lines 1-12 pattern, `verify.go` lines 21-25). Follow the same pattern: define `maxBaselines = 50` as a package-level constant, allowing future configuration.

### R8. The spec should define behavior when buffer data is empty

**Section:** How It Works > Saving a Baseline

If no network requests have been captured (e.g., extension not connected), `save_baseline` will create a baseline with zero endpoints. A subsequent `compare_baseline` will always return "match" because there are no endpoints to compare against. This is misleading.

**Recommendation:** When the baseline would have zero network endpoints AND zero WS connections AND zero console entries, return a warning: "Baseline is empty -- is the browser extension connected?" Check `capture.lastPollAt` to determine extension connectivity (see `main.go` lines 1266-1270).

---

## Implementation Roadmap

Given the critical issues above (especially C1), here is the recommended implementation order. The approach extends existing tools rather than creating new ones.

### Phase 1: Extend `diff_sessions` with persistence (2-3 hours)

**Files:** `cmd/dev-console/sessions.go`, `cmd/dev-console/sessions_test.go`

1. Add `Persist bool` and `LoadFrom string` fields to the capture action params.
2. On `capture` with `persist: true`, serialize the snapshot to `<project>/.gasoline/baselines/<name>.json` using `SessionStore.Save()`.
3. On `compare`, if `compare_a` or `compare_b` names are not in memory, attempt to load from disk.
4. Add `list` action that includes both in-memory and persisted baselines.
5. Write tests: save to disk, restart (new manager instance), load and compare.

### Phase 2: Add response shape comparison (1-2 hours)

**Files:** `cmd/dev-console/sessions.go` or new `cmd/dev-console/shape.go`

1. Extract the shape comparison logic from `APIContractValidator` into a reusable function: `extractJSONShape(body string, depth int) map[string]interface{}`.
2. Add `ResponseShape map[string]interface{}` to `SnapshotNetworkRequest`.
3. During snapshot capture, parse JSON response bodies and store shapes.
4. During comparison, diff shapes: detect `field_removed`, `type_changed`, `field_added`.
5. Write tests for shape extraction (test scenario 18), non-JSON handling (scenario 19).

### Phase 3: Add latency percentile tracking and tolerance (1 hour)

**Files:** `cmd/dev-console/sessions.go`

1. Add `P50, P95, Max float64` to a new `LatencyStats` field on each endpoint in the snapshot.
2. Compute percentiles during snapshot capture from the `Duration` field on `NetworkBody`.
3. Add `Tolerance` struct to compare params: `TimingFactor float64`, `AllowAdditionalNetwork bool`, `AllowAdditionalConsoleInfo bool`.
4. Apply timing factor per-endpoint during comparison (R4).
5. Write tests for latency regression detection (scenarios 7, 8, 13).

### Phase 4: Add path normalization (30 minutes)

**Files:** `cmd/dev-console/sessions.go` or `cmd/dev-console/normalize.go`

1. Add a `NormalizePath(url string) string` function: replace UUIDs with `{uuid}`, numeric path segments with `{id}`.
2. Use existing regex patterns from `verify.go` (`clusterUUIDRegex`, `clusterNumericIDRegex`).
3. Apply normalization when grouping endpoints during snapshot capture.
4. Write tests for path normalization (scenario 20).

### Phase 5: Update tool schema and wire up (30 minutes)

**Files:** `cmd/dev-console/tools.go`

1. Update `diff_sessions` tool schema in `toolsList()` to include new parameters: `persist`, `tolerance`, etc.
2. Update the tool description to mention persistent baselines.
3. Do NOT add new `save_baseline` / `compare_baseline` / `list_baselines` / `delete_baseline` tools.

### Phase 6: Quality gates (30 minutes)

1. Run `go vet ./cmd/dev-console/`.
2. Run `make test`.
3. Run `node --test extension-tests/*.test.js`.
4. Verify all 23 test scenarios from the spec are covered (mapping them to the extended `diff_sessions` tests).

**Total estimated effort:** 5-7 hours, significantly less than implementing a parallel tool system from scratch. The result is a cohesive tool surface rather than four overlapping comparison tools.
