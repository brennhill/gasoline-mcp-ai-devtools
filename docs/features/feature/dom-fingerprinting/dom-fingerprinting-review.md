# DOM Fingerprinting - Technical Review

**Reviewer:** Principal Engineer
**Date:** 2026-01-26
**Spec Location:** `docs/ai-first/tech-spec-dom-fingerprinting.md`
**Companion Spec:** `docs/ai-first/dom-fingerprinting.md`
**Status:** Pre-implementation

---

## Executive Summary

The DOM fingerprinting spec is well-motivated and addresses a real cost/latency gap in AI-assisted development workflows. The data model is thoughtfully designed, the extraction strategy aligns with existing pending-query infrastructure, and the depth modes offer good token/coverage tradeoffs. However, the spec has three classes of problems that need resolution before implementation: (1) the `compare_dom_fingerprint` tool requires server-side baseline storage that is never specified, creating an undefined data contract; (2) the visibility detection strategy will cause main-thread stalls on element-heavy pages due to unbounded `getComputedStyle` calls; and (3) the comparison algorithm in the companion spec uses positional list matching that will produce false positives on any page with dynamic content ordering.

---

## 1. Critical Issues (Must Fix Before Implementation)

### 1.1 Baseline Storage Is Undefined

**Sections:** Tool Interface > `compare_dom_fingerprint`, Data Model > Comparison Result

The `compare_dom_fingerprint` tool accepts `against` as a "baseline name or fingerprint hash." The `get_dom_fingerprint` tool accepts `baseline_name` to "automatically compare against this baseline's stored fingerprint." Neither spec document defines:

- Where baselines are stored (in-memory map? session store? disk?)
- How baselines are created (is `get_dom_fingerprint` with `baseline_name` a save-and-compare, or compare-only?)
- Baseline eviction policy (how many baselines? TTL? per-URL or global?)
- What happens when `against` references a nonexistent baseline

This is not a minor omission. The entire comparison workflow depends on persistent state that has no specified lifecycle. The existing `SessionStore` (disk-backed, `.gasoline/` directory) and `SessionManager` (in-memory snapshots with cap of 10) are potential homes, but neither is referenced.

**Recommendation:** Add a `Baseline Storage` section specifying:
- In-memory map with 20-baseline cap (consistent with `maxPerfSnapshots`/`maxPerfBaselines` constants in `types.go`)
- Baselines keyed by name (string), stored as `DOMFingerprint` structs
- `get_dom_fingerprint` with `baseline_name` does save-then-compare: extracts current fingerprint, saves it under that name, and if a previous fingerprint existed under that name, returns the comparison
- `compare_dom_fingerprint` with `against` is compare-only against a previously saved baseline
- Nonexistent baseline returns a structured error, not a timeout

### 1.2 Visibility Detection Will Stall Main Thread

**Sections:** Visibility Detection, Performance Constraints

The spec defines visibility as requiring checks for `offsetParent`, `display`, `visibility`, and `opacity` -- the last three via `getComputedStyle`. The spec budgets "under 0.1ms per element" for `isVisible`.

The problem: `getComputedStyle` forces a style recalculation (layout flush) on the first call if the DOM is dirty. On pages with pending layout (common after SPA navigation or React re-render), the first `getComputedStyle` call can take 5-50ms. Subsequent calls within the same frame are fast, but the initial flush is unavoidable.

Worse, `extractInteractive` runs `querySelectorAll` on a broad selector (`button, a[href], input, select, textarea, [role="button"], [tabindex="0"]`) which on a real dashboard page can match 200-500 elements. Calling `getComputedStyle` on each of these after the initial flush is individually cheap, but the flush plus 300 calls at ~0.02ms each = 6ms for style alone. Combined with `extractPageState` (which also checks visibility) and `extractContent`, the 30ms budget is tight.

The existing `dom-queries.js` uses `offsetParent` and `getBoundingClientRect` for visibility but notably avoids `getComputedStyle` in the visibility check path.

**Recommendation:**
1. Use a two-tier visibility check: first `offsetParent !== null` (no layout flush), then `getBoundingClientRect().width > 0` for fixed-position elements. Only call `getComputedStyle` as a fallback for elements that pass tier-1 but need `opacity`/`visibility` confirmation.
2. Add an explicit performance test that runs extraction on a 500-element page and asserts < 30ms. The spec lists this as a constraint but the test scenarios (Section: Test Scenarios) only test correctness, not performance.
3. Consider `requestIdleCallback` or `requestAnimationFrame` wrapping if the extraction is not time-critical (it is not -- the 5-second query timeout provides ample slack). However, this conflicts with the synchronous pending-query response model, so the simpler approach is to optimize the hot path.

### 1.3 Comparison Algorithm Uses Positional List Matching

**Companion spec:** `docs/ai-first/dom-fingerprinting.md`, Section: Comparison Algorithm

```go
for i, baseList := range baseline.Structure.Content.Lists {
    if i < len(current.Structure.Content.Lists) {
        curList := current.Structure.Content.Lists[i]
        // ...
    }
}
```

This compares lists by array index. If the page adds a list before an existing one (e.g., a notification list appears above the project list), every subsequent list comparison will be wrong -- comparing project-list against notification-list by position.

The same issue applies to interactive elements if the comparison used positional matching, but the tech spec correctly specifies indexing by `text+type` for interactive elements. Lists, forms, and tables lack this treatment.

**Recommendation:** Match lists by `selector` field (which the spec already extracts), not by array position. Same for forms (match by selector) and tables (match by selector). The comparison should build maps keyed by selector, then detect added/removed/changed entries.

---

## 2. Recommendations (Should Consider)

### 2.1 Hash Algorithm Is Unspecified

**Section:** Hash for Quick Comparison

The spec says "8-character hash derived from its structure (ignoring timestamps)" but does not specify the algorithm. This matters because:
- Determinism across Go and JS: if the hash is computed extension-side (JS) and compared server-side (Go), they must use the same algorithm
- Collision resistance: 8 hex chars = 32 bits = birthday collision at ~65K fingerprints, which is fine for per-session comparison but should be documented

**Recommendation:** Specify FNV-1a 32-bit (available in Go stdlib `hash/fnv`, trivial to implement in JS) over the JSON-serialized structure with sorted keys. Compute it extension-side so the server receives it as part of the fingerprint payload. This avoids needing the server to re-derive the hash.

### 2.2 Timeout Discrepancy: 5s vs 10s

**Section:** Extension Message Flow

The spec states "The query has a 5-second timeout" but the existing `defaultQueryTimeout` in `types.go` is 10 seconds. All other query types (DOM, a11y, page_info, pilot tools) use the 10-second default via `h.capture.queryTimeout`.

Using a shorter timeout for fingerprinting is defensible (the extraction should be fast), but it creates a special case in the server code that diverges from the established pattern.

**Recommendation:** Use the standard `queryTimeout` (currently 10s). If a tighter timeout is desired, make it configurable at the tool level rather than hardcoding 5s, consistent with how `PilotExecuteJSParams` has its own `TimeoutMs` field.

### 2.3 `above_fold` Scope Needs Clarification

**Sections:** Tool Interface, Edge Cases

The spec says `above_fold` limits to "visible viewport only" but the implementation sketch uses `getVisibleElements()` without defining it. Key questions:
- Does this filter by `getBoundingClientRect().top < window.innerHeight`?
- What about elements that are partially visible (top half visible, bottom half below fold)?
- Does this interact with `isVisible` (an element below the fold that is `display:none` should be excluded by both filters)?

**Recommendation:** Define `above_fold` as: element's `getBoundingClientRect().top < window.innerHeight` AND element passes the standard visibility check. Partially visible elements are included. This is simple, deterministic, and consistent with viewport-based lazy loading thresholds.

### 2.4 Shadow DOM: Spec Contradicts Itself

**Tech spec** (Section: Edge Cases): "Not currently traversed. Only light DOM elements are extracted."
**Companion spec** (Section: Edge Cases): "Pierce shadow DOM for interactive elements; report shadow hosts as landmarks."

These are mutually exclusive. Given the performance constraints, the tech spec's position (skip shadow DOM) is the right call for v1.

**Recommendation:** Explicitly state shadow DOM is out of scope for v1. Add a test case that verifies shadow DOM elements are not included in the fingerprint. Consider shadow DOM support as a future depth mode (`depth: "deep"`).

### 2.5 Dynamic Content Normalization Is Underspecified

**Companion spec** (Section: Edge Cases): "Normalize: dates -> `[date]`, numbers in text -> `[number]`, unless in heading text."

This is a minefield. What regex matches "dates"? Does "January 26, 2026" match? Does "2 hours ago"? Does "3 items"? Aggressive normalization creates false negatives (real changes masked). Conservative normalization creates false positives (timestamps cause "changed" status).

**Recommendation:** Do NOT normalize text content in v1. The fingerprint captures structure, not content. The `text` field on interactive elements is the accessible name (button label, link text), which is typically static. If dynamic content causes noise, address it in v2 with an opt-in `normalize` parameter rather than baking assumptions into the default behavior. This aligns with the product philosophy: "Capture, don't interpret."

### 2.6 Two Tools vs One Tool

**Section:** Tool Interface

The spec defines two separate MCP tools: `get_dom_fingerprint` and `compare_dom_fingerprint`. This adds two entries to the tool list (currently 20 tools). The comparison can already be triggered via the `baseline_name` parameter on `get_dom_fingerprint`.

Given the existing composite tool pattern (`observe`, `analyze`, `generate`, `configure`), fingerprinting fits naturally as a new `analyze` target or a new `observe` mode.

**Recommendation:** Implement as `analyze` with `target: "dom_fingerprint"` for extraction+comparison, and add `configure` with `action: "fingerprint_baseline"` for explicit baseline management. This is consistent with the existing dispatch pattern, avoids adding top-level tools, and keeps the tool list manageable. The `scope`, `depth`, and `baseline_name` parameters become sub-parameters of the analyze target, exactly like `accessibility` has `scope`, `tags`, and `force_refresh`.

### 2.7 Token Count Estimation

**Section:** Data Model > Fingerprint

The spec includes `token_count` in the fingerprint but does not specify how it is calculated. Token counting is model-dependent (cl100k_base vs. o200k_base) and the server has no tokenizer.

**Recommendation:** Estimate as `len(jsonBytes) / 4` (rough approximation for English text JSON). Document that this is an estimate, not an exact count. Alternatively, omit it entirely -- the AI model knows its own context window and can assess the fingerprint size from the response.

### 2.8 Missing `tab_id` Support

The spec does not mention tab targeting, but every other on-demand query tool now supports `tab_id` for multi-tab workflows (see `PendingQuery.TabID` in `types.go` and the `tab_id` parameters on `query_dom`, `highlight_element`, etc.).

**Recommendation:** Add `tab_id` as an optional parameter on `get_dom_fingerprint`, consistent with `query_dom`.

---

## 3. Implementation Roadmap

The following order minimizes integration risk and follows the project's TDD workflow.

### Phase 1: Server-side types and comparison logic (Go)

1. **Define types** in a new `ai_fingerprint.go`: `DOMFingerprint`, `FingerprintStructure`, `Landmark`, `InteractiveElement`, `PageState`, `StateElement`, `ComparisonResult`, `FingerprintChange`. Add baseline storage to `Capture` struct (map + order slice, consistent with `PerformanceStore` pattern).

2. **Write comparison tests** in `ai_fingerprint_test.go`. Cover all 11 server test scenarios from the spec. Use table-driven tests. Assert the comparison result shape first (contract test), then behavioral tests for each change type.

3. **Implement comparison logic**. Key corrections from spec:
   - Match lists/forms/tables by selector, not position
   - Index interactive elements by `type+text` key
   - Handle the case where baseline has landmarks the current page lacks AND vice versa (new landmarks should be `info` severity, not ignored)

4. **Wire up MCP tool dispatch**. Add `dom_fingerprint` as a new `analyze` target (or add standalone tools if the team prefers). Create pending query of type `"dom_fingerprint"`, wait for result, parse into `DOMFingerprint`, optionally compare.

### Phase 2: Extension-side extraction (JavaScript)

5. **Create `extension/lib/fingerprint.js`** with `extractDOMFingerprint(scope, depth)` function. Export from `inject.js` barrel.

6. **Write extension tests** in `extension-tests/fingerprint.test.js`. Cover all 18 extension test scenarios from the spec. Mock `document` with JSDOM or manual DOM stubs (consistent with existing extension test patterns).

7. **Implement extraction**:
   - `extractLandmarks`: Fixed set of `querySelector` calls. Budget: 2ms.
   - `extractInteractive`: `querySelectorAll` + visibility filter + accessible name. Budget: 15ms. Use two-tier visibility (offsetParent first, getComputedStyle only for edge cases).
   - `extractContent`: Headings, lists, forms, images. Budget: 10ms.
   - `extractPageState`: Fixed selector queries. Budget: 3ms.
   - Performance guard: `performance.now()` before/after, `console.warn` if > 30ms.

8. **Wire message handling** in `content.js` and `background.js`:
   - `content.js`: Listen for `dom_fingerprint` query type from background, relay to inject via `window.postMessage`, relay response back.
   - `background.js`: Route `dom_fingerprint` pending query to content script.
   - Post result to `/dom-result` (reuses existing endpoint).

### Phase 3: Integration and baseline management

9. **Add baseline CRUD operations** to server: save, load, list, delete. Expose via `configure` action (`fingerprint_baseline`) or as part of the fingerprint tool.

10. **Add hash computation** (FNV-1a 32-bit) in both JS (extraction time) and Go (verification). Test determinism: same structure produces same hash across both implementations.

11. **End-to-end test**: Create a Go integration test that simulates the full flow (create pending query, post mock fingerprint result, verify comparison output).

### Phase 4: Polish

12. **Performance benchmarks**: Add a benchmark test with a mock 500-element DOM. Assert extraction < 30ms.

13. **Documentation**: Update tool descriptions in `tools.go` tool list. Add to feature docs.

14. **Quality gates**: `go vet`, `make test`, `node --test` must pass. Verify tool appears in `mcp-initialize.golden.json`.

---

## 4. Questions for the Author

1. Should `compare_dom_fingerprint` be a standalone tool, or should comparison be exclusively available via the `baseline_name` parameter on `get_dom_fingerprint`? The two-tool approach doubles the API surface for a feature that can be expressed as one tool with optional comparison.

2. The companion spec mentions "MutationObserver tracking" and "debounced extraction" as optimizations. Are these in scope for v1, or should the initial implementation be stateless (extract fresh every call)?

3. The companion spec references `v4.go` as the server file for MCP handlers, but the current codebase uses `tools.go` for dispatch and separate files per feature domain. Should this follow the `ai_fingerprint.go` pattern used by other AI-first features?
