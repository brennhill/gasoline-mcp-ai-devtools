# Review: Cross-Session Temporal Graph (tech-spec-temporal-graph.md)

## Executive Summary

This spec adds a persistent, cross-session event log with causal linking and pattern detection. The implementation (`temporal_graph.go`) is already largely complete and sound. The core design -- append-only JSONL, deduplication by fingerprint, 90-day retention -- is appropriate for the expected data volume. However, the file I/O model has a concurrency vulnerability on crash recovery, the automatic correlation heuristics risk producing misleading causal links, and the spec fundamentally conflicts with the project's "capture, don't interpret" product philosophy.

## Critical Issues (Must Fix Before Implementation)

### 1. File Write Model Loses Deduplicated Count Updates on Crash

**Section:** "Event Recording" / Implementation: `temporal_graph.go` lines 176-213

When a duplicate event is detected (same fingerprint), the code increments `OccurrenceCount` and updates `Timestamp` on the in-memory event, but does NOT write the update to the file. The JSONL file still contains the original event with `occurrence_count: 1`.

On crash recovery (server restart), `loadFromFile` reads the JSONL and rebuilds in-memory state. All duplicate-count updates are lost. An error that occurred 50 times will appear as if it occurred once.

The implementation writes new events to the file (`writeEvent` on line 212) but never updates existing lines. JSONL is append-only by design -- you cannot update a line in the middle of the file without rewriting.

**Fix options:**
1. **Periodic flush:** Every 60 seconds, rewrite the entire file from in-memory state (like `evict()` does). This bounds data loss to 60 seconds of count updates.
2. **Append update records:** Append a special "update" record (`{"_update": "evt_123", "occurrence_count": 50}`) to the JSONL. On load, apply updates in order. This preserves append-only semantics.
3. **Accept the data loss:** Document that occurrence counts are approximate across restarts. This is the simplest option and may be acceptable given the advisory nature of the data.

Option 3 is recommended for v1 with option 1 as a future improvement.

### 2. Automatic Correlation Produces Misleading Causal Links

**Section:** "Automatic Correlation"

The spec defines two automatic correlations:
- Regression within 30 seconds of resource change -> "possibly_caused_by"
- Error disappears within 60 seconds of reload -> "possibly_resolved_by"

**Problem: Temporal proximity is not causation.** In a developer's workflow, many things happen within 30-second windows: saving files triggers HMR, HMR triggers resource changes, resource changes trigger re-renders, re-renders may expose or hide errors. The 30-second window will produce a causal link for *every* regression that happens during active development -- which is when most regressions are detected.

More concretely: developer saves file -> HMR reload -> new resource list (deploy event inferred) -> unrelated regression detected 15 seconds later (slow API response). The automatic correlation links the regression to the "deploy" even though the API slowdown has nothing to do with the code change.

These "inferred" links will pollute the AI's context with false causality. The spec marks them as `confidence: "inferred"` but does not provide guidance on how the AI should weight them.

**Fix:** Remove automatic correlation from v1. The explicit links (AI-recorded via `configure(action: "record_event", related_to: "evt_123")`) are sufficient and reliable. If automatic correlation is desired later, require stronger evidence:
- Same URL/page context for both events
- Resource change specifically involves files in the stack trace of the regression/error
- Minimum 3 observations of the same correlation pattern before upgrading from "inferred" to "likely"

### 3. Pattern Detection is Unspecified and Contradicts "Capture, Don't Interpret"

**Section:** "Pattern Detection"

The spec lists three pattern types:
- Recurring errors (appear, resolve, reappear)
- Deploy-correlated regressions
- Time-of-day patterns

None of these are specified algorithmically. What constitutes "recurring"? 2 appearances? 3? Over what time window? What is "consistent" deploy correlation -- every deploy, or 2 out of 5?

More importantly, pattern detection is interpretation. The product philosophy doc states: "Capture, don't interpret -- Record what the browser does. Don't decide what it means." Pattern detection decides what error recurrence means ("flaky fix"), which is exactly the kind of interpretation the philosophy warns against.

The AI is better at pattern recognition than any hardcoded heuristic. Give it the raw event list and let it identify patterns.

**Fix:** Remove the `patterns` array from the query response. Instead, ensure the event list is rich enough for the AI to detect patterns itself:
- Include `occurrence_count` per event (already present)
- Include `status` transitions (active -> resolved -> active)
- Include timestamps of all status changes (not just current timestamp)
- Sort events chronologically (already done)

The AI, with access to this structured data, will outperform any heuristic pattern detector.

### 4. Concurrent Server Instances Share a File Without Coordination

**Section:** "Edge Cases" -- "Concurrent server instances: Same locking strategy as persistent memory -- second instance reads only."

**Problem:** The "second instance reads only" claim is not enforced in code. `NewTemporalGraph` opens the file with `O_APPEND|O_CREATE|O_WRONLY` unconditionally (`temporal_graph.go` line 97). Two server instances will both open the file for appending, and both will write events. JSONL is technically safe for concurrent append (each write is atomic if under PIPE_BUF on Unix), but the in-memory state will diverge -- each instance has different events, and neither reads the other's writes.

On restart after concurrent sessions, the file contains interleaved events from both instances, but fingerprint indices will be wrong (built from different subsets).

**Fix:** Use file locking (`flock` on Unix, `LockFileEx` on Windows) to enforce single-writer. The second instance should open the file read-only and skip event recording (log a warning). This matches the stated behavior but needs implementation.

Alternatively, since Go does not have a stdlib `flock`, use a `.lock` file alongside `events.jsonl`. Check for the lock file on startup; if present and the PID inside is still alive, open read-only.

### 5. Eviction Rewrites File Without fsync, Risks Data Loss on Crash

**Section:** "Retention" / Implementation: `temporal_graph.go` lines 162-173

The `rewriteFile` function creates a new file (`os.Create`) and writes all events. If the process crashes during the write, the file is truncated and events are lost.

**Fix:** Write to a temp file (`events.jsonl.tmp`), `fsync` it, then `os.Rename` to `events.jsonl`. This is the standard atomic-write pattern. `os.Rename` is atomic on all platforms when source and dest are on the same filesystem.

## Recommendations (Should Consider)

### 1. Event ID Generation Has Low Entropy

**Section:** Implementation: `temporal_graph.go` lines 321-327

```go
r := binary.LittleEndian.Uint16(b[:]) % 10000
return fmt.Sprintf("evt_%d_%04d", ts, r)
```

Two events recorded in the same millisecond have a 1/10000 chance of collision. During rapid error bursts (common when a breaking change is saved), multiple events per millisecond is realistic. The birthday paradox makes collisions likely after ~125 events in the same millisecond.

**Fix:** Use 4 random bytes instead of 2 (collisions become unlikely even at thousands of events per millisecond). Or use a monotonic counter per server instance.

### 2. Query Performance Degrades Linearly With Event Count

**Section:** "Performance Constraints" -- "History query (full 90-day scan): under 50ms for 5000 events"

The query iterates all events and filters in-memory. At 50 events/day for 90 days, that is 4500 events -- within budget. But a project with frequent errors could generate 200+ events/day (10 unique errors x 20 sessions), reaching 18,000 events in 90 days. At this volume, the linear scan may exceed the 50ms target.

**Fix:** Maintain a secondary index by event type (map of type -> list of event indices). This turns type-filtered queries from O(n) to O(k) where k is the number of events of that type. The index is cheap to maintain (updated on each append) and small in memory.

### 3. "Deploy" Event Detection via Resource Change is Fragile

**Section:** "Event Recording" -- item 5

The spec says deploy events are detected "when the resource list changes significantly (new scripts/stylesheets suggest a deploy)." What is "significantly"? Adding one script? Changing one hash in a filename? HMR-triggered reloads change resource lists constantly during development.

This will produce a "deploy" event on every HMR reload, which is noisy to the point of uselessness. The CI webhook path is the reliable signal. The resource-change heuristic should be removed or gated behind a "production mode" flag.

### 4. The `related_to` Query Only Finds Events Linking TO an ID

**Section:** Implementation: `temporal_graph.go` lines 273-284

The `related_to` filter searches for events whose `Links` array contains the target ID. But it does not find the event with that ID itself, nor events that the target links to. For the query "what is related to evt_123?", the user expects to see evt_123, events it links to, AND events linking to it. Currently they only get the last category.

**Fix:** When `related_to` is specified, first find the target event by ID. Then find all events that link to it (outbound from target) and all events that link from it (inbound to target). Return the target event plus both sets, with the relationship direction indicated.

### 5. No Mechanism to Correct Wrong AI-Recorded Events

The AI can record events via `configure(action: "record_event")` but cannot update or delete them. If the AI records "Fixed null user in UserProfile" but the fix was wrong (error reappears), the event persists with no correction mechanism.

**Fix:** Add `configure(action: "update_event", event_id: "evt_123", status: "superseded")` to allow the AI to mark events as superseded. Do not allow deletion (audit trail should be immutable), but allow status updates.

## Implementation Roadmap

1. **File I/O hardening** (0.5 days): Implement atomic file rewrite (temp + rename). Add file locking or `.lock` file for concurrent instance safety. Document occurrence count as approximate across restarts.

2. **Remove automatic correlation** (0.5 days): Remove the 30-second and 60-second proximity-based link generation. Keep explicit links only. Remove pattern detection from query response.

3. **Enrich event data for AI consumption** (0.5 days): Add status transition history to events. Improve `related_to` query to find bidirectional links. Fix event ID entropy.

4. **Add event status update** (0.5 days): Implement `configure(action: "update_event")` for status changes. Append update record to JSONL.

5. **Remove resource-change deploy detection** (0.25 days): Keep only CI webhook deploy events. Remove HMR-triggered false deploys.

6. **Add type index** (0.25 days): Secondary index for O(1) type-filtered queries.

Total: ~3 days of implementation work. The existing implementation (`temporal_graph.go`) covers ~70% of the spec already. The focus should be on hardening rather than new features.
