# Review: Noise Filtering Spec

**Reviewer**: Principal Engineer Review
**Spec**: `docs/ai-first/tech-spec-noise-filtering.md`
**Date**: 2026-01-26

---

## Executive Summary

The noise filtering system is already implemented in `ai_noise.go` and is one of the more mature features in the codebase. The implementation closely matches the spec, with solid concurrency handling (separate `RWMutex` for rules and `Mutex` for stats) and a well-organized built-in ruleset. However, there are critical issues with the auto-detection algorithm's quadratic complexity, a regex denial-of-service vector in user-supplied patterns, and a spec/implementation mismatch where the spec describes framework detection as a separate system but the implementation bakes all framework rules into the always-on built-in set.

---

## Critical Issues (Must Fix Before Implementation)

### 1. ReDoS via User-Supplied Regex Patterns

**Section**: "Tool Interface > configure_noise (add)" and "dismiss_noise"

Users (and AI agents) can submit arbitrary regex patterns via `AddRules` and `DismissNoise`. These patterns are compiled with `regexp.Compile` and then executed against every buffer entry on every read. Go's `regexp` package uses RE2 semantics (guaranteed linear time), which mitigates catastrophic backtracking. However, a sufficiently complex regex can still consume significant CPU during the linear scan. The spec's "under 0.1ms per entry" constraint (Performance Constraints section) assumes reasonable patterns.

The real risk is not backtracking but regex _complexity_: a pattern like `(a|b|c|d|e|f|g|h|...|z){20}` compiled by RE2 can create a large NFA that is slow to match even with linear guarantees.

**Fix**: Add a compile-time complexity check. Reject patterns longer than 500 characters. Optionally, benchmark the compiled regex against a test string and reject if a single match takes >1ms. The current code silently skips patterns that fail to compile (line 509-511) but does not validate complexity of successfully compiled patterns.

### 2. Auto-Detection Has Quadratic Complexity

**Section**: "Auto-Detection" and implementation in `AutoDetect()`

The `isConsoleCoveredLocked` function (line 916-942) iterates all compiled rules for each candidate message AND then iterates all entries to find matching sources, creating O(rules * entries) complexity per candidate. For 100 rules and 500 console entries with 50 unique messages at count >= 10, this is 50 * (100 + 500 * 100) = 2.5M iterations. The spec claims "under 50ms" for full buffer scan, which may not hold under these conditions.

**Fix**: Pre-compute a set of "already matched messages" by running all rules against all unique messages once, then use set lookup for the coverage check. This reduces complexity from O(candidates * rules * entries) to O(unique_messages * rules + candidates).

### 3. Framework Detection is Spec'd but Not Implemented as Described

**Section**: "Framework Detection" and "Built-in Rules (Global)"

The spec describes a framework detection system that auto-detects the project's framework and conditionally activates framework-specific rules. The spec says: "Framework-specific rules are only active when the framework is detected."

The implementation in `builtinRules()` (line 92-500) includes ALL framework rules unconditionally -- React key warnings, Angular dev mode, Vue devtools, Svelte HMR, etc. There is no framework detection logic and no conditional activation. This means:
- A React-only project has Angular, Vue, and Svelte noise rules active (wasted CPU on every match)
- The framework detection table in the spec is entirely unimplemented
- The "Framework Detection" section's claim that "Detection runs once on the first batch of captured data and caches the result" has no corresponding code

**Decision needed**: Either implement framework detection as spec'd (significant complexity) or remove the framework detection section from the spec and document the current behavior: all framework rules are always active. Given that the rules are regex-based and the linear scan of ~50 rules takes <0.1ms, the performance impact of always-on rules is negligible. Recommend keeping the current always-on behavior and updating the spec to match.

---

## Recommendations (Should Consider)

### 4. Dual Mutex Pattern Creates Subtle Ordering Issues

**Section**: Implementation in `ai_noise.go`

The `NoiseConfig` uses two mutexes: `mu` (RWMutex for rules) and `statsMu` (Mutex for stats). The `IsConsoleNoise` method acquires `mu.RLock` (line 591), then calls `recordMatch` which acquires `statsMu.Lock` (line 699). This is a consistent lock ordering (always mu -> statsMu), which is correct. But the `Reset` method (line 578) acquires `mu.Lock` and then directly writes to `stats` without acquiring `statsMu`. This is a data race: `recordMatch` could be writing to `stats` under `statsMu` while `Reset` writes to `stats` under `mu`.

**Fix**: `Reset()` at line 583-586 must acquire `statsMu` before writing to `nc.stats`. The current code:
```go
nc.stats = NoiseStatistics{
    PerRule: make(map[string]int),
}
```
should be wrapped with `nc.statsMu.Lock()` / `nc.statsMu.Unlock()`.

### 5. "Silently Dropped" Rules Violate the Principle of Least Surprise

**Section**: "Edge Cases" -- "Max 100 rules total... Additional rules are silently dropped."

When the AI agent adds rules and they are silently dropped, the agent has no way to know its configuration was ignored. This will cause confusion: the agent adds a rule, later queries the buffer expecting the rule to filter entries, and finds unfiltered entries with no explanation.

**Recommendation**: Return an error or a warning in the `AddRules` response when rules are dropped due to the limit. The current implementation (line 546-548) breaks silently. At minimum, return the count of rules actually added vs. requested.

### 6. Security Invariant #2 is Ambiguously Scoped

**Section**: "Security Invariants"

Invariant #2 says: "Error-level from application sources are never auto-detected." The implementation's `AutoDetect` method does not filter by level -- it counts all console messages regardless of level. A console.error that repeats 10+ times would be proposed as noise by the frequency heuristic. The invariant is in the spec but not enforced in code.

**Fix**: In the frequency analysis loop of `AutoDetect`, skip entries where the level is "error" and the source does not match known extension/node_modules patterns. This ensures application errors are never auto-proposed.

### 7. Source Analysis Only Catches `node_modules`

**Section**: "Auto-Detection" heuristic #2

The spec says sources from `chrome-extension://`, `moz-extension://`, or `node_modules` are flagged. The implementation (line 819) only checks `strings.Contains(source, "node_modules")`. The `chrome-extension://` and `moz-extension://` sources are already covered by built-in rules, but the auto-detection would not flag them independently if someone removed the built-in rules (which cannot happen, but the logic should be self-contained).

**Recommendation**: Low priority, but add the extension URL check for completeness. Also consider checking for `webpack-internal://` and `vite/` paths.

### 8. Periodicity Detection is Not Implemented

**Section**: "Auto-Detection" heuristic #3

The spec describes periodicity detection with inter-arrival time analysis and standard deviation checks. The implementation has no periodicity detection. The `AutoDetect` method only implements frequency analysis, source analysis, and network frequency. Entropy scoring (heuristic #4) is also missing.

**Recommendation**: Either implement these heuristics or remove them from the spec. Periodicity detection requires timestamp tracking per URL path, which the current data model does not support (network bodies have timestamps but they are strings, not parsed). This is significant new work. Entropy scoring requires a tokenizer. Recommend deferring both to a future iteration and removing from the current spec.

### 9. `builtin_next_internal` Rule Filters Legitimate Network Requests

**Section**: Built-in rules implementation, line 453-458

The rule `builtin_next_internal` filters network requests matching `/_next/(static|data|image)/`. While `/_next/static/` is reasonable (framework assets), `/_next/data/` contains Next.js data fetching responses that are often directly relevant to debugging application behavior. Filtering these means the AI cannot see Next.js server-side props or getStaticProps responses.

**Fix**: Narrow the rule to `/_next/static/` only, or add a status code filter (only filter successful responses, keep 4xx/5xx).

### 10. Network Noise Matching Has a Logic Subtlety

**Section**: Implementation, `IsNetworkNoise()` lines 664-669

The code at line 666 matches when there is no URL regex set but method/status matched:
```go
if cr.rule.MatchSpec.URLRegex == "" && (cr.rule.MatchSpec.Method != "" || cr.rule.MatchSpec.StatusMin > 0) {
```

This means a rule with only `Method: "OPTIONS"` and status range 200-299 will match ALL OPTIONS requests with 2xx status, regardless of URL. This is the intended behavior for the CORS preflight rule, but it creates a trap: if a user adds a rule with only a method filter and no URL, it becomes a wildcard match. Document this behavior or require at least one of URL/method/status to be set.

---

## Implementation Roadmap

1. **Fix the data race in `Reset()`**: Add `statsMu` lock around stats reassignment. This is a one-line fix but a correctness issue.

2. **Add ReDoS mitigation**: Add a pattern length limit (500 chars) in `AddRules` and `DismissNoise`. Return an error for patterns exceeding the limit.

3. **Fix auto-detect security invariant**: Skip error-level entries from non-extension/non-node_modules sources in frequency analysis.

4. **Narrow `builtin_next_internal`**: Change the URL regex from `/_next/(static|data|image)/` to `/_next/static/` or add status filtering.

5. **Return feedback on dropped rules**: Modify `AddRules` to return the count of rules added vs. requested.

6. **Reconcile framework detection**: Update the spec to document the current always-on behavior. Remove the "Framework Detection" section or mark it as "future work."

7. **Remove unimplemented heuristics from spec**: Remove periodicity detection and entropy scoring from the "Auto-Detection" section, or mark them as deferred.

8. **Optimize auto-detection**: Pre-compute rule coverage sets to eliminate the quadratic scan in `isConsoleCoveredLocked`.

9. **Add integration test for auto-detect + apply cycle**: Verify that high-confidence auto-detected rules are actually applied and filter subsequent reads. The current test scenarios cover individual behaviors but not the full cycle.
