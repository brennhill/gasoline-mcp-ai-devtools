---
status: shipped
scope: feature/budget-thresholds/review
ai-priority: high
tags: [review, issues]
relates-to: [tech-spec.md, product-spec.md]
last-verified: 2026-01-31
---

# Budget Thresholds Review

_Migrated from /specs/budget-thresholds-review.md_

# Budget Thresholds as Config -- Technical Review

**Reviewer:** Principal Engineer
**Date:** 2026-01-26
**Spec:** `docs/ai-first/tech-spec-budget-thresholds.md`
**Target file:** `cmd/dev-console/budgets.go`

---

## Executive Summary

The spec proposes a well-scoped, config-driven absolute budget system that complements the existing relative regression detection in `performance.go`. The core design is sound: declarative JSON config, preset merging, longest-prefix route matching, and integration into the existing `get_changes_since` alert pipeline. However, the spec has three critical gaps that will cause bugs in production: a race condition in the config watch/reload cycle, an unbounded violation store with no eviction policy, and a metric name impedance mismatch between the budget config schema and the existing `PerformanceSnapshot` type hierarchy. These must be resolved before implementation.

---

## 1. Critical Issues (Must Fix)

### 1.1 Race Condition: Config Reload During Evaluation

**Section:** "Config Watch" (lines 158-163)

The spec says the server checks the config file's mtime every 30 seconds and, on change, re-reads, re-resolves, and re-checks all snapshots. But the existing server uses a single `sync.RWMutex` on `Capture` (see `types.go:406`). The config watch goroutine will need write access to update resolved budgets while concurrent snapshot evaluations need read access.

The spec does not address:
- What lock protects the budget config? If it shares `Capture.mu`, the 30s poll + re-evaluation of all snapshots will hold a write lock for the entire re-check, blocking all MCP tool calls.
- If it uses a separate mutex, how are atomicity guarantees maintained between "resolve budgets" and "evaluate snapshot against budgets"?

**Recommendation:** Add a dedicated `sync.RWMutex` for the budget config, separate from `Capture.mu`. Config reload acquires the budget write lock, swaps the resolved map atomically, then releases. Evaluation acquires a budget read lock. Never hold both `Capture.mu` and the budget lock simultaneously -- this prevents deadlock and keeps snapshot ingestion off the critical path.

### 1.2 Unbounded Violation Storage

**Section:** "Budget Violation" data model (lines 149-155)

The spec says violations are "stored alongside regression alerts" but defines no eviction policy. The existing `PerformanceAlert` buffer in `ai_checkpoint.go` has implicit eviction through the checkpoint delivery counter (`deliveredAt`), but budget violations introduce a new category. Consider:

- A developer has 10 routes, each with 9 budget metrics. That is 90 potential violations.
- Config change triggers re-evaluation of all 20 snapshots (max from `maxPerfSnapshots`). That is up to 180 violations generated in a single config reload.
- The spec says violations track "whether the violation is new or persistent" -- this implies historical storage, but with no cap.

**Recommendation:** Cap violations at a fixed buffer (e.g., 100 entries). Use the same ring-buffer eviction pattern as every other buffer in the system (see `types.go:328-351`). Add this constant to the spec alongside `maxPerfSnapshots`.

### 1.3 Metric Name Impedance Mismatch

**Section:** "Config Format" (lines 37-64)

The budget config uses flat metric names: `load_ms`, `fcp_ms`, `lcp_ms`, `cls`, `inp_ms`, `ttfb_ms`, `total_transfer_kb`, `script_transfer_kb`, `image_transfer_kb`.

The existing `PerformanceSnapshot` struct (`types.go:200-208`) uses nested fields:
- `Timing.Load`, `Timing.FirstContentfulPaint` (pointer), `Timing.LargestContentfulPaint` (pointer), `Timing.TimeToFirstByte`
- `Network.TransferSize` (bytes, not KB)
- `CLS` (pointer)
- No `INP` in `PerformanceTiming` (it is `InteractionToNextPaint`, and it is `omitempty`)
- No `script_transfer_kb` or `image_transfer_kb` fields exist anywhere. These require iterating `Network.ByType` map.

This creates three sub-problems:

1. **Unit mismatch:** Config uses KB for transfer sizes; snapshot uses bytes. The evaluation code must convert, and a bug here silently makes budgets 1000x too lenient.
2. **Missing data:** `script_transfer_kb` and `image_transfer_kb` are not first-class snapshot fields. They require aggregating from `Network.ByType["script"]` and `Network.ByType["img"]`. The spec does not mention this aggregation.
3. **Pointer semantics:** FCP, LCP, CLS, and INP are all `*float64` in the snapshot. A nil value means the metric was not captured. The spec says "Budget of 0 treated as no budget" but does not address nil actuals. A nil FCP should not generate a violation -- but the spec is silent on this.

**Recommendation:** Add a "Metric Resolution" section to the spec that maps each budget metric name to its exact source field, unit conversion factor, and nil-handling behavior. Example:

| Budget metric | Source | Conversion | Nil handling |
|---|---|---|---|
| `load_ms` | `Timing.Load` | none | skip if 0 |
| `total_transfer_kb` | `Network.TransferSize` | bytes / 1024 | skip if 0 |
| `script_transfer_kb` | `Network.ByType["script"].Size` | bytes / 1024 | skip if key absent |
| `fcp_ms` | `Timing.FirstContentfulPaint` | none | skip if nil |

---

## 2. Recommendations (Should Consider)

### 2.1 Config File Discovery is Underspecified

**Section:** "Config File" (line 33)

The spec says the config lives at `.gasoline.json` or `.gasoline/budgets.json` at the "project root." But the server does not know what the project root is. `tools.go:173` uses `os.Getwd()` for the session store, but CWD depends on how the server is launched (npm global, npx, direct binary). If launched via MCP stdio from Claude Code, CWD is the project root. If launched as a standalone daemon, CWD may be `$HOME`.

**Recommendation:** Use the same CWD-based discovery as `NewToolHandler` (`tools.go:173`), but document this explicitly. Add a `--budgets` flag or `GASOLINE_BUDGETS_PATH` env var as an override. Add test scenario: "server started from non-project directory finds no config."

### 2.2 30-Second Poll is Coarse for Developer Feedback Loop

**Section:** "Config Watch" (lines 92, 158)

A developer edits `.gasoline.json`, saves, and calls `check_budgets`. If the edit happened 1 second ago and the last poll was 2 seconds ago, the AI sees stale config for up to 29 more seconds. This breaks the "AI enforcer" value proposition.

**Recommendation:** The `check_budgets` MCP tool should force a config re-read on every invocation (stat + conditional re-parse). The 30-second background poll remains for passive alert generation via `get_changes_since`. This is cheap (under 5ms per the spec's own constraint) and eliminates the stale-config window for explicit checks.

### 2.3 `check_budgets` Tool Should Be Integrated into `analyze`, Not Standalone

**Section:** "MCP Tool: check_budgets" (lines 117-134)

The existing architecture uses composite tools: `observe`, `analyze`, `generate`, `configure`. Adding `check_budgets` as a standalone top-level tool breaks this pattern (see `tools.go:1024-1067` dispatch). Every other performance-related operation is under `analyze` (`performance`, `changes`).

**Recommendation:** Add `check_budgets` as a new `target` value in the `analyze` composite tool: `analyze { target: "budgets", url: "...", show_passing: true }`. This keeps the tool surface area manageable (currently 19 tools, already high) and follows the established pattern for the `analyze` dispatcher at `tools.go:1134-1162`.

### 2.4 Alert Deduplication for Persistent Violations

**Section:** "Budget Violation Alert" (lines 99-115)

The spec mentions tracking "whether the violation is new or persistent" but does not define deduplication behavior. If a page reload produces the same budget violation, does the system emit a new alert every time? The existing alert pipeline in `alerts.go` deduplicates by `title + category`, but budget violations for the same metric on the same URL would need the same dedup key across snapshots.

**Recommendation:** Define explicitly: a budget violation alert is emitted once per (URL, metric) pair per budget config version. Re-evaluation after config change resets the dedup state. Persistent violations are reported in `check_budgets` output but do not re-trigger push alerts.

### 2.5 `over_by` Format is Stringly-Typed

**Section:** "Budget Violation Alert" (line 109)

The `over_by` field uses human-readable strings like `"120KB (24%)"`. This is fine for display but makes programmatic consumption by AI agents harder. The AI cannot parse `"120KB (24%)"` back into numbers without string manipulation.

**Recommendation:** Split into structured fields:
```json
{
  "metric": "total_transfer_kb",
  "budget": 500,
  "actual": 620,
  "over_by_value": 120,
  "over_by_pct": 24.0,
  "unit": "KB"
}
```
Keep `over_by` as a convenience summary if desired, but add the numeric fields.

### 2.6 Missing: Relationship to Existing Regression Detection

The spec does not explain how budget violations interact with regression alerts (already implemented in `performance.go:299-408` and surfaced through `ai_checkpoint.go`). Both systems fire on the same snapshot ingestion path. Questions:

- If a snapshot exceeds its budget AND triggers a regression, does the AI see two alerts or one correlated alert?
- Should budget violations feed into the existing `Alert` correlation pipeline (`alerts.go:187-224`) that merges `regression + anomaly` pairs?

**Recommendation:** Budget violations should be a new alert category (`"budget"` alongside `"regression"`, `"anomaly"`, `"ci"`). The correlation engine in `alerts.go:226-244` should be extended to correlate `budget + regression` pairs -- if a metric regresses AND exceeds its budget, that is a single compound alert, not two.

### 2.7 Test Scenario Gaps

**Section:** "Test Scenarios" (lines 189-205)

Missing scenarios that the existing codebase patterns suggest are important:

- **Concurrent config reload during snapshot ingestion.** The race condition from 1.1 needs an explicit `-race` test.
- **Config file with valid JSON but wrong shape** (e.g., `budgets` key missing, or metric value is a string instead of number). The spec only covers "Invalid JSON."
- **Route matching with query strings and hash fragments.** The existing `normalizeResourceURL` in `performance.go:615-643` strips query params. Should `/dashboard?tab=charts` match the `/dashboard` budget?
- **KB/byte conversion boundary.** Budget says `500` (KB), snapshot has `512000` bytes. Does `512000 / 1024 = 500.0` pass or fail? (Boundary precision.)
- **Nil metric in snapshot vs. defined budget.** FCP budget is 1800ms but FCP is nil in snapshot. Must not produce a violation.

---

## 3. Implementation Roadmap

Ordered to maximize test coverage at each step, following the project's TDD mandate.

### Step 1: Types and Config Parsing (budgets.go)

Define the config types: `BudgetConfig`, `BudgetThresholds`, `BudgetViolation`, `BudgetPreset`. Write a pure function `ParseBudgetConfig(data []byte) (*BudgetConfig, error)` that handles all parsing, preset merging, and route resolution. No I/O, no concurrency.

**Tests first:** Parse valid config, preset merging, route override, invalid JSON, wrong shape, unknown metric names ignored, budget of 0 treated as absent, multiple presets merged in order.

### Step 2: Metric Resolution and Evaluation (budgets.go)

Write `EvaluateBudget(thresholds BudgetThresholds, snapshot PerformanceSnapshot) []BudgetViolation`. Pure function. Handles unit conversion, nil metrics, and the metric name mapping table.

**Tests first:** Each metric within/exceeding budget, nil FCP with FCP budget, byte-to-KB conversion boundary, script/image transfer from ByType map, all presets produce correct thresholds.

### Step 3: Route Matching (budgets.go)

Write `ResolveThresholds(config *BudgetConfig, url string) BudgetThresholds`. Longest-prefix match, fallback to default, fallback to presets.

**Tests first:** Exact match, prefix match, longest prefix wins, no match falls to default, default inherits from presets, no config returns zero thresholds.

### Step 4: Config File Loading and Watching (budgets.go)

Add `BudgetManager` struct with its own `sync.RWMutex`. Load on startup from CWD-based discovery. Background goroutine polls mtime every 30s. Expose `GetThresholds(url string) BudgetThresholds` under read lock.

**Tests first:** Load from `.gasoline.json`, load from `.gasoline/budgets.json`, prefer `.gasoline.json`, no file returns nil, file appears mid-session, invalid file clears previous config, mtime-based reload.

### Step 5: Wire into Snapshot Ingestion (performance.go)

After `AddPerformanceSnapshot`, call `BudgetManager.Evaluate` and feed violations into the alert pipeline as `Alert{Category: "budget"}`. Add to `PerformanceAlert` for `get_changes_since`.

**Tests first:** Snapshot exceeding budget generates alert, snapshot within budget generates nothing, budget + regression produces two distinct alerts, config reload clears stale violations.

### Step 6: MCP Tool Integration (tools.go)

Add `budgets` target to the `analyze` composite tool. Force config re-read on tool invocation. Return structured violation/passing response.

**Tests first:** Tool returns violations, tool returns passing with `show_passing`, tool returns "no config" when absent, tool forces config refresh.

### Step 7: Race Testing

Add `-race` test covering concurrent snapshot ingestion + config reload + tool invocation.

**Tests first:** `go test -race` with goroutines doing parallel snapshot adds, config reloads, and budget checks.

---

## 4. Minor Notes

- The spec says `image_transfer_kb` but the `ByType` map in `NetworkSummary` likely uses `"img"` as the key (matching the Performance API's `initiatorType`). Verify and document the exact key.
- The `recommendation` field in the violation alert ("Reduce bundle size or increase budget in .gasoline.json") is static. Consider making it metric-aware: a CLS violation should recommend layout stability fixes, not bundle size reduction.
- The spec correctly handles forward-compatibility ("Unknown metric name silently ignored") -- this is good design.
- The 18 test scenarios listed in the spec are a solid foundation. The 5 additional scenarios in section 2.7 above should be added.
