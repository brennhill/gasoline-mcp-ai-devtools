---
status: shipped
scope: feature/push-regression/review
ai-priority: high
tags: [review, issues]
relates-to: [tech-spec.md, product-spec.md]
last-verified: 2026-01-31
---

# Review: Push Notification on Regression

## Executive Summary

This spec is well-scoped and low-risk. It piggybacks regression alerts onto the existing `get_changes_since` checkpoint system, adding minimal memory and zero new goroutines. The implementation already exists in `ai_checkpoint.go` and aligns closely with the spec. The main concerns are threshold mismatch between the spec and the existing `DetectRegressions` implementation, ambiguous alert lifecycle semantics under concurrent multi-client access, and the reuse of `DeltaMs` for non-millisecond values (CLS, transfer bytes).

## Critical Issues

### 1. Threshold Divergence Between Spec and Implementation

The spec (lines 31-37) defines regression thresholds of 20% for load/FCP/LCP, 50% for TTFB, 0.1 absolute for CLS, and 25% for transfer size. The existing `DetectRegressions` in `performance.go` (lines 299-408) uses entirely different thresholds: 50% AND 200ms absolute for load/FCP/LCP, 100% for transfer, and 0.05 for CLS.

The push alert system in `ai_checkpoint.go` (lines 928-1019) implements the spec's thresholds correctly via `detectPushRegressions`. However, this means two regression detection systems coexist with different sensitivity levels. The push system fires at 20% load regression; the `check_performance` tool reports regressions at 50%. An agent receiving a push alert and then calling `check_performance` for details may see "No Regressions" because the full tool uses stricter thresholds.

**Fix**: Unify the threshold constants. Either align `DetectRegressions` to use the same thresholds as push detection, or document explicitly that push alerts use lower (more sensitive) thresholds as an early warning. At minimum, the `recommendation` field should not tell agents to call `check_performance` if that tool might disagree about whether a regression exists.

### 2. Multi-Client Alert Delivery Ambiguity

The `CheckpointManager` maintains a single `alertDelivery` counter and a single `pendingAlerts` slice (lines 162-165 of `ai_checkpoint.go`). When client A polls `get_changes_since` with auto-checkpoint mode, `markAlertsDelivered()` is called (line 358), which stamps all pending alerts with the delivery counter. If client B polls afterward, those alerts are now marked as delivered at a counter that may precede client B's checkpoint.

The `getPendingAlerts` method (lines 1051-1059) checks `deliveredAt > checkpointDelivery`, which means client B might still see the alerts if its checkpoint's `AlertDelivery` is lower. But the semantics are fragile -- it depends on whether client B created a checkpoint before or after client A's poll.

**Fix**: Namespace alert delivery tracking per client, similar to how checkpoints are already namespaced with `clientID:name`. Each client should track its own `alertDelivery` counter in its checkpoint, and `markAlertsDelivered` should be scoped to the polling client.

### 3. Alert `DeltaMs` Field Semantic Overloading

In `AlertMetricDelta` (types.go lines 565-570), the `DeltaMs` field is used for millisecond deltas (load, FCP, etc.), absolute CLS deltas (line 998: `DeltaMs: delta, // for CLS this is the absolute delta, not ms`), and byte deltas (line 1012: `DeltaMs: delta, // for transfer this is the byte delta`). This makes the field unreliable for consumers -- the AI agent cannot interpret `DeltaMs` without knowing which metric it belongs to.

**Fix**: Rename to `Delta` or add a `Unit` field (`"ms"`, `"absolute"`, `"bytes"`). This is a data contract issue that will confuse any consumer parsing the alert generically.

## Recommendations

### A. Add Absolute Minimum Thresholds for Push Alerts

The spec thresholds are percentage-only for most metrics. A page that loads in 10ms regressing to 13ms is a 30% regression that fires an alert, but the 3ms absolute change is meaningless. The existing `DetectRegressions` wisely requires both a percentage AND an absolute minimum (e.g., >200ms). The push detection in `detectPushRegressions` lacks absolute minimums.

Add minimum absolute thresholds: load/FCP/LCP should require at least 100ms absolute change. Transfer size should require at least 10KB absolute change. This prevents noisy alerts on fast pages.

### B. Alert Expiry on Stale Baselines

The spec acknowledges stale baselines (Edge Cases, line 126) and says alerts still fire. This is correct in principle, but a baseline from a previous session may be days old. Consider adding a staleness indicator to the alert (e.g., `baseline_age_hours`) so the agent can discount alerts against very old baselines.

### C. Test Coverage for Concurrent Client Scenarios

Test scenarios 1-15 cover single-client flows well. Add tests for:
- Two clients polling `get_changes_since` -- both should see the same alert
- Client A polls (alert delivered), client B creates checkpoint, client B polls -- alert should still be visible to B
- Alert resolved between client A and client B polls

### D. Consider Debouncing Rapid Reloads

If the agent triggers 5 rapid reloads (common during iterative development), each reload generates a snapshot compared against a baseline that is being updated by the previous snapshots. The weighted averaging (80/20) means the baseline drifts quickly. Consider ignoring snapshots that arrive within 1 second of each other, or freezing the baseline during a burst.

## Implementation Roadmap

1. **Fix `AlertMetricDelta.DeltaMs` overloading** -- Rename to `Delta` and add `Unit` field. Update all consumers. This is a breaking data contract change that should happen before any clients depend on it.

2. **Add absolute minimum thresholds** to `detectPushRegressions`. Mirror the pattern from `DetectRegressions`: percentage AND absolute.

3. **Namespace alert delivery per client** -- Extend `Checkpoint` to track per-client alert delivery counters. Modify `markAlertsDelivered` to accept a `clientID`.

4. **Reconcile threshold constants** between `DetectRegressions` and `detectPushRegressions`. Define a single set of constants or document the intentional difference.

5. **Add concurrent client test scenarios** per recommendation C.

6. **Add baseline staleness indicator** to the alert payload.
