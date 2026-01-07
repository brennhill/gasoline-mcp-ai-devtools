# Dynamic Exposure Review

_Migrated from /specs/dynamic-exposure-review.md_

# Dynamic Tool Exposure (Phase 2) -- Engineering Review

**Spec:** `docs/ai-first/tech-spec-dynamic-exposure.md`
**Reviewer:** Principal Engineer review
**Date:** 2026-01-26

---

## 1. Executive Summary

The spec proposes dynamically filtering the MCP `tools/list` response based on server buffer state, hiding tools and enum values that have no actionable data. The goal -- reducing AI context waste -- is sound and directly supports the product philosophy of discovery over analysis. However, the spec has a critical concurrency flaw in its locking strategy, silently violates the MCP protocol by omitting `tools/list_changed` notifications, and introduces a progressive disclosure gate that will confuse more AI models than it helps.

---

## 2. Critical Issues (Must Fix Before Implementation)

### 2.1 Three-Lock Acquisition Creates Deadlock Risk

**Section:** "Concurrency"

The spec states `toolsList()` acquires read locks on `server.mu` and `capture.mu`. The existing `computeDataCounts()` in `tools.go:328-348` already does this as two sequential lock acquisitions. But the spec ignores that `schemaStore.EndpointCount()` at line 346 acquires a *third* lock (`schemaStore.mu`). Three separate read-lock acquisitions in a single method -- while individually safe for readers -- become a deadlock hazard if any write path acquires these locks in a different order.

Today `addEntries()` (`main.go:505`) holds `server.mu` (write) and then calls `onEntries` which feeds into `clusters.AddError()`. If any future code path acquires `capture.mu` while holding `server.mu` in write mode, and another goroutine holds `capture.mu` and tries to acquire `server.mu`, you deadlock.

**Recommendation:** Snapshot all counts in a single critical section, or compute counts using the existing `computeDataCounts()` pattern (sequential lock/unlock) but document the lock ordering invariant explicitly: `server.mu` -> `capture.mu` -> `schemaStore.mu`. Add a comment at each lock site enforcing this order. Better yet, have `computeDataCounts()` return all values needed for both `_meta` and enum filtering in one pass, which the current code already does.

### 2.2 Missing MCP `tools/list_changed` Notification

**Section:** "How It Works" / "State-Aware Tool List"

The MCP protocol (2024-11-05) defines a `notifications/tools/list_changed` notification that servers **must** emit when the tool list changes. The current `MCPCapabilities` struct (`tools.go:55`) declares an empty `MCPToolsCapability{}` -- it does not advertise `listChanged: true`. If you dynamically change what `tools/list` returns, compliant MCP clients will never know to re-fetch. Claude Code and Cursor may cache the initial `tools/list` response for the entire session.

This means:
- AI calls `tools/list` at initialization, gets 2 tools (progressive disclosure).
- AI makes an `observe` call, progressive disclosure lifts.
- AI never calls `tools/list` again because no notification was sent.
- All analysis/generate/configure tools remain invisible for the entire session.

**Recommendation:** Either:
1. Declare `listChanged: true` in capabilities and emit `notifications/tools/list_changed` when the tool list structurally changes (tools added/removed, not just count updates). This requires writing to stdout outside of a request/response cycle, which the current stdio scanner loop (`main.go:968-993`) does not support. You would need a concurrent write channel.
2. **Simpler alternative:** Always return all tools. Use `_meta.data_counts` (which you already have) as the signal for what is actionable. AI models already handle "0 items available" gracefully. This avoids the notification problem entirely and keeps the architecture simpler.

Option 2 is strongly recommended. It aligns with the zero-deps philosophy and avoids a protocol-level change.

### 2.3 Progressive Disclosure Is Counterproductive

**Section:** "Progressive Disclosure"

The spec gates `analyze`, `generate`, and `configure` behind a "first successful observe call" flag. This creates several problems:

1. **`configure` should never be gated.** The AI may need to load session context (`configure(action: "load")`) or set up noise rules *before* observing anything. Gating `configure` behind observation breaks the setup-then-observe workflow that the `SessionStore` feature was designed for.

2. **The gate is invisible.** When the AI sees only 2 tools, it has no indication that more tools exist or how to unlock them. The spec provides no mechanism for communicating "call observe first to unlock more tools" -- the AI must already know this, which defeats the purpose of discovery.

3. **It breaks `analyze(target: "accessibility")` and `generate(format: "sarif")`.** Both are marked "Always available" in the availability tables because they trigger live audits. But progressive disclosure hides them until an `observe` call is made. The spec contradicts itself.

4. **State management across reconnections is fragile.** The flag resets on server restart but not on MCP reconnect. If the MCP client reconnects to a running server (multi-client mode via `--connect`), the progressive disclosure state from the previous session persists, which is correct but surprising.

**Recommendation:** Remove progressive disclosure entirely. Use `_meta.data_counts` for the same purpose without hiding tools. If you want to guide AI behavior, add a `_meta.hint` field like `"hint": "no data yet -- call observe first"` on tools with zero counts.

---

## 3. Recommendations (Should Consider)

### 3.1 Enum Filtering Will Break Schema-Caching AI Clients

**Section:** "State-Aware Tool List"

Dynamically changing the `enum` values in `inputSchema` means the JSON schema of each tool changes on every `tools/list` call. Some MCP clients hash the schema for caching or use it for input validation. A tool whose schema changes every time data arrives will:
- Cause cache thrashing in clients that key on schema content.
- Potentially confuse AI models that were trained to treat enum values as fixed.

**Recommendation:** Keep enum values static. Use `_meta.data_counts` to communicate what has data. If a count is 0, the AI knows not to call that mode. This is the same information with none of the schema instability.

### 3.2 Error Count Computation Is O(n)

**Section:** "Data Count Computation"

The error count is computed by iterating all `server.entries` and checking `level == "error"` (currently at `tools.go:331-335`). With `maxEntries=1000`, this is fine. But the spec claims "must respond in under 1ms" and "no allocations beyond the response construction." The full entries scan violates the second constraint (no extra allocations) since the iteration itself is allocation-free, but the error counting is O(n) work under a read lock.

**Recommendation:** Maintain an `errorCount` atomic counter on `Server` that increments in `addEntries()` and decrements during rotation. This makes `computeDataCounts()` O(1) for all fields. This is a small optimization but it aligns with the stated performance constraint and becomes important if `maxEntries` ever increases.

### 3.3 `_meta` Field Placement Needs MCP Spec Alignment

**Section:** "Capability Annotations"

The spec places `_meta` as a top-level field on the tool object. The existing code (`tools.go:74`) serializes it as `"_meta"` via the JSON tag. The MCP spec (2024-11-05) defines `_meta` as a convention for server-to-client metadata, but its placement at the tool level is not formally specified -- it is a de facto convention. This is fine as long as:
- No MCP client treats unknown top-level fields as errors (most don't).
- The underscore prefix convention is respected (it is).

The current implementation is acceptable. No change needed, but document that this is a non-standard extension that clients may ignore.

### 3.4 Security Tools Are Missing From Availability Rules

**Section:** "Availability Rules"

The spec defines availability rules for `observe`, `analyze`, `generate`, `configure`, and `query_dom`. It omits rules for 8 other tools that exist today:
- `generate_csp`
- `security_audit`
- `audit_third_parties`
- `diff_security`
- `get_audit_log`
- `diff_sessions`
- `validate_api`
- `get_health`
- `generate_sri`
- `verify_fix`
- `highlight_element`
- `manage_state`
- `execute_javascript`
- `browser_action`

The current `toolsList()` returns 19 tools. The spec only covers 5. This means either:
1. The 14 other tools are always exposed (undocumented).
2. The spec is incomplete.

**Recommendation:** Add a catch-all rule: "All tools not listed in the availability rules table are always exposed." Or enumerate them. The spec should be explicit about the full tool set.

### 3.5 Race Between Data Arrival and `tools/list`

**Section:** "Edge Cases"

Consider this sequence:
1. AI calls `tools/list` -- gets `observe` with `network: 0`.
2. Extension POSTs network bodies to `/network-bodies`.
3. AI (based on cached tool list) believes no network data exists.
4. AI never calls `observe(what: "network")`.

The `_meta.data_counts` approach mitigates this because the AI sees counts at the moment of the `tools/list` call, not cached schema. But if enum filtering removes `"network"` from the enum, the AI literally *cannot* call `observe(what: "network")` even if data has since arrived.

**Recommendation:** Another reason to keep enums static and use `_meta` for availability signals only.

### 3.6 Test Scenarios Are Incomplete

**Section:** "Test Scenarios"

The 8 test scenarios are good but missing:
- Concurrent `tools/list` calls while data is being written (race condition test).
- `tools/list` response size regression test (ensure dynamic filtering does not inflate response beyond a threshold).
- `_meta.data_counts` accuracy after buffer rotation (counts should reflect post-rotation state, not pre-rotation).
- Multi-client isolation: does progressive disclosure apply per-client or globally?

---

## 4. Implementation Roadmap

Given the analysis above, the recommended implementation order:

### Step 1: Do Nothing to the Schema (Keep Enums Static)

The current implementation already has `_meta.data_counts` on every data-dependent tool. This provides 80% of the spec's value (AI knows what has data) with 0% of the risk (no schema changes, no protocol violations, no progressive disclosure bugs).

Validate this is sufficient by measuring: how often do AI models call tools with zero available data? If the answer is "rarely," stop here.

### Step 2: Add Atomic Error Counter

Add `errorCount int64` to `Server` with `atomic.AddInt64` in `addEntries()`. Replace the O(n) scan in `computeDataCounts()` with an atomic load. This tightens the 1ms performance guarantee.

### Step 3: Add `_meta.hint` for Zero-Data Tools

When a tool's relevant data counts are all zero, add `"hint": "No data captured yet. The browser extension must be connected and activity must occur."` to `_meta`. This guides AI behavior without hiding tools.

### Step 4: If Enum Filtering Is Still Desired, Implement with `tools/list_changed`

Only if Step 1 proves insufficient:
1. Add `listChanged: true` to `MCPToolsCapability`.
2. Add a write channel to the MCP stdio loop for async notifications.
3. Track previous tool list hash; when it changes after data ingestion, emit `notifications/tools/list_changed`.
4. Implement enum filtering in `toolsList()`.
5. Do NOT implement progressive disclosure -- it creates more problems than it solves.

### Step 5: Tests

Write tests in this order (TDD per project rules):
1. `TestToolsListEnumFilteringWithData` -- verify enum values include modes with data.
2. `TestToolsListEnumFilteringWithoutData` -- verify always-available modes persist.
3. `TestToolsListAlwaysAvailableModes` -- errors, logs, page, accessibility, sarif, configure always present.
4. `TestToolsListConcurrentDataWrite` -- write data while calling `tools/list` in parallel.
5. `TestToolsListAfterRotation` -- counts reflect post-rotation state.
6. `TestToolsListMetaHintOnEmptyData` -- hint field present when counts are zero.
7. `TestToolsListChangedNotification` -- if Step 4 is implemented.

---

## 5. Verdict

**Do not implement enum filtering or progressive disclosure as specified.** The existing `_meta.data_counts` mechanism already achieves the spec's stated goal ("reduces cognitive load on AI consumers") without the protocol violations, schema instability, and self-contradictions in the current spec. If data shows AI models are wasting calls on empty modes despite `_meta` signals, revisit with `tools/list_changed` support.
