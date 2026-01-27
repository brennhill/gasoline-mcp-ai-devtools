# Redaction Patterns Review (Migrated)

> **[MIGRATION NOTICE]**
> Migrated from `/docs/specs/redaction-patterns-review.md` on 2026-01-26.
> Related docs: [PRODUCT_SPEC.md](PRODUCT_SPEC.md), [TECH_SPEC.md](TECH_SPEC.md), [ADRS.md](ADRS.md).

---

# Configurable Redaction Patterns (Feature 19) - Technical Review

**Reviewer:** Principal Engineer Review
**Spec:** `docs/ai-first/tech-spec-redaction-patterns.md`
**Date:** 2026-01-26
**Verdict:** Spec has a strong design foundation but contains several critical correctness bugs, a significant security gap, and performance traps that must be resolved before implementation.

---

## Executive Summary

The spec proposes runtime-configurable redaction patterns with per-field JSON targeting, multiple replacement strategies, and a caching layer. The architectural intent is sound and the API surface aligns well with the existing `configure` tool pattern. However, the spec contains a **data race in `RedactString`** that will produce corrupt output under concurrent use, a **regex cache miss in `pathMatches`** that will dominate latency on the hot path, and an **unbounded hash strategy** that leaks information about redacted content length. These must be fixed before writing any implementation code.

---

## 1. Critical Issues (Must Fix Before Implementation)

### C1. Data Race in `RedactString` — Lines 652-697

The `RedactString` method holds a read lock (`mu.RLock`) via `RedactJSON`, but calls `m.incrementStat(pattern.Config.ID)` inside `ReplaceAllStringFunc`. `incrementStat` must write to `m.stats.RedactionsByPattern`, which requires a write lock. This is a textbook read-write lock violation.

Worse: `redactedRanges` is a closure-captured slice appended to from within `ReplaceAllStringFunc`, which Go's regex engine calls sequentially for a single `ReplaceAllStringFunc` invocation, but the overall `RedactString` is called concurrently from multiple goroutines (it sits on the MCP hot path at `main.go:307-309`). Each goroutine captures its own `redactedRanges`, so that part is safe per-invocation, but the stats write is not.

**Fix:** Use `atomic.Int64` for per-pattern counters, or accumulate stats into a local struct and flush under a separate lock after the read lock is released. Do not mix `RLock` with writes to shared state.

### C2. Overlap Detection is Broken — Lines 670-673

```go
matchStart := strings.Index(result, match)
```

This finds the **first** occurrence of the matched text in the entire `result` string, not the actual position of the current match. If the same substring appears multiple times (common with redaction placeholders like `[REDACTED:ssn]`), this will return the wrong index. Additionally, after the first pattern replaces content, `result` has changed but `redactedRanges` still holds offsets from the old string, making all subsequent range checks invalid.

**Fix:** The overlap-prevention approach is fundamentally flawed for string-level replacement. Two viable alternatives:

1. **Single-pass multi-pattern:** Build a combined regex (or use `regexp.Regexp.FindAllStringIndex` for each pattern) to find all match positions first, resolve conflicts by priority, then apply replacements in reverse offset order (right-to-left to preserve positions).
2. **Token-fence approach:** After each replacement, insert a unique sentinel that no pattern can match, then strip sentinels at the end. Simpler but fragile.

Option 1 is strongly recommended. It also avoids the O(n*m) problem of running n patterns sequentially over the full string.

### C3. `pathMatches` Compiles a Regex on Every Call — Line 635

```go
matched, _ := regexp.MatchString(regexPattern, current)
```

`regexp.MatchString` compiles the regex on every invocation. `pathMatches` is called once per JSON field per field-targeted pattern per response. For a 50KB JSON response with 200 fields and 5 field-targeted patterns, that is 1000 regex compilations per response. Regex compilation in Go is O(n) in pattern length but has significant constant overhead (memory allocation, NFA construction).

**Fix:** Pre-compile field path patterns at pattern-add time. The spec already has `FieldRegex []*regexp.Regexp` in `CompiledRedactionPattern` (line 76) but `pathMatches` doesn't use it. Wire the pre-compiled regexes through to the match function.

### C4. ReDoS via User-Supplied Patterns — Security

While Go's `regexp` package uses RE2 (guaranteed linear time), the spec allows up to **100 patterns of up to 1024 characters each** applied to responses up to **1MB**. RE2's linear guarantee is O(n*m) where n is input length and m is pattern complexity. With adversarial patterns (deeply nested alternations), 100 patterns * 1MB input * high NFA state count could still exceed the 10ms SLO.

More concerning: the spec does not validate pattern complexity beyond "it compiles." A pattern like `(a|b|c|d|e|f|...){100}` compiles fine in RE2 but creates an exponential number of NFA states during compilation, not matching.

**Fix:** Add a compilation timeout or NFA state limit. Go's `regexp` package does not expose this directly, so: (a) enforce a `regexp.Compile` timeout via goroutine + context, or (b) limit alternation depth and repetition count via pre-parse validation, or (c) benchmark each compiled pattern against a ~1KB sample at add time and reject if it exceeds 1ms.

### C5. Hash Strategy Leaks Sensitive Data Distinguishability — Lines 522-527

```go
func hashString(s string, patternName string) string {
    hash := sha256.Sum256([]byte(s))
    shortHash := hex.EncodeToString(hash[:])[:8]
    return fmt.Sprintf("[HASH:%s:%s]", patternName, shortHash)
}
```

Using only 8 hex characters (32 bits) of SHA-256 means collision probability reaches 50% at ~65K unique values (birthday paradox). For a feature designed to provide "deterministic replacement" (so the AI can track that two occurrences refer to the same entity), 32 bits is dangerously small. Two different credit card numbers could hash to the same short string, misleading the AI into correlating unrelated data.

Additionally, the hash has no salt. If an attacker knows the pattern name and a candidate value, they can brute-force the 8-character hash to confirm whether that value appeared in the session.

**Fix:** Use at least 16 hex characters (64 bits, collision-safe to ~4B values) and include a per-session random salt in the hash input. The salt does not need to be secret (it just needs to be unique per server session to prevent precomputation).

---

## 2. Recommendations (Should Consider)

### R1. `RedactJSON` Double-Serializes — Lines 554-578

The current design: unmarshal JSON, walk and redact field-targeted patterns, re-marshal, then run global string patterns on the serialized form. This means:

1. **Two full JSON parse/serialize cycles** for every response
2. Global patterns run on JSON-encoded strings, meaning they match inside JSON keys, structural characters, and escaped content — not just values
3. A regex matching `"remove"` (the strategy name) could accidentally redact JSON structure

**Alternative:** Walk the JSON tree once. For each string value, check if it matches any field-targeted pattern (by current path) or any global pattern. This is a single pass, avoids re-serialization artifacts, and is more predictable.

### R2. Redaction Cache Stores Full Response Text — Lines 775-823

The cache maps `hash(input + patternVersion) -> redacted output`. For a 1MB response, each cache entry stores up to 2MB (input hash + output string). With `maxSize: 1000`, worst case is 2GB of cached strings. The stated 100KB memory budget for the cache is aspirational at best.

**Fix:** Either (a) limit cache to responses under a size threshold (e.g., 10KB), (b) remove the cache entirely (the 10ms SLO should be achievable without it if the matching algorithm is efficient), or (c) use an LRU with a byte-size budget, not an entry-count budget.

### R3. Cache Eviction is Non-Deterministic — Lines 798-808

```go
for k := range c.cache {
    delete(c.cache, k)
    count++
    if count >= c.maxSize/2 {
        break
    }
}
```

Go map iteration order is random. This evicts arbitrary entries, not least-recently-used. This is acknowledged in the comment ("LRU would be better") but for a security feature, unpredictable cache behavior can cause hard-to-reproduce bugs where the same request sometimes returns cached (redacted) results and sometimes doesn't.

**Recommendation:** Use `container/list` + map for O(1) LRU, or remove caching entirely. The caching adds significant complexity for marginal benefit in a feature that processes MCP tool responses (not high-QPS HTTP traffic).

### R4. Missing `maxResponseSize` Enforcement — Line 768

The spec defines `maxResponseSize = 1<<20` (1MB) but no code path enforces it. `RedactJSON` and `RedactString` accept arbitrary input sizes. A 10MB response (e.g., from a large network body capture) would blow through the 10ms SLO.

**Fix:** Add an early-exit check: if `len(input) > maxResponseSize`, skip redaction and log a warning. Alternatively, only redact the first 1MB and append the remainder unredacted (with a warning marker).

### R5. Tool Should Be `configure_redaction`, Not Nested Under `configure`

The spec defines the tool as `configure_redaction` (line 159) but the existing codebase uses a `configure` composite tool with an `action` parameter (lines 1190-1218 in `tools.go`). The spec should clarify whether this is:

(a) A new top-level tool named `configure_redaction` (adds to the tool count, more discoverable)
(b) New actions under the existing `configure` tool (consistent with current pattern)

Given that the `configure` tool already has 6 actions and adding 8 more (`add`, `update`, `remove`, `enable`, `disable`, `list`, `test`, `clear`) would make it unwieldy, option (a) is recommended. But this is a **breaking change** for any existing MCP clients that enumerate tools.

### R6. `Replacement` Field Conflicts with `Strategy` — Lines 48-51

```go
Replacement string            `json:"replacement,omitempty"` // Custom replacement (overrides strategy)
```

If both `Replacement` and `Strategy` are set, `Replacement` silently wins (line 484). This is a confusing API. The AI agent could set `strategy: "hash"` and `replacement: "$1"` and get neither behavior.

**Fix:** Make them mutually exclusive. If `Replacement` is non-empty, `Strategy` must be empty or unset, and vice versa. Return a validation error if both are provided.

### R7. MaskConfig Should Validate Against Content Length — Lines 500-519

If `ShowFirst + ShowLast >= len(match)`, the entire match is masked (line 510). But the spec doesn't mention that adding a pattern with `show_first: 20, show_last: 20` on content that's always 12 characters will always fully mask. The AI agent has no feedback that its mask config is effectively equivalent to "remove."

**Recommendation:** In the `test` action response, include a warning when mask config would fully mask the test input.

### R8. ID Generation Allows Collisions — Lines 449-451

```go
config.ID = fmt.Sprintf("user_%s_%s", config.Name, randomID(6))
```

A 6-character random ID (assuming hex or alphanumeric) gives 36^6 = ~2.2B possibilities. This is fine for collision avoidance, but the spec doesn't define `randomID`. Ensure it uses `crypto/rand`, not `math/rand`, since pattern IDs appear in API responses and a predictable ID could allow an attacker to guess and remove another user's pattern.

### R9. Migration Path From `RedactionEngine` to `RedactionManager` Is Underspecified

The spec says to rename `RedactionEngine` to `RedactionManager` (line 936). The existing `RedactionEngine` is referenced in `tools.go` (line 152, 192, 307-309). The migration must:

1. Keep the `RedactionEngine` interface working during transition
2. Ensure the `redactionConfigPath`-based loading (line 192) still works
3. Not break the existing `RedactJSON` contract (lines 176-197 in `redaction.go`) which parses `MCPToolResult` specifically, not arbitrary JSON

The spec's `RedactJSON` (line 554) parses into `interface{}`, which is a different contract than the current `MCPToolResult`-aware version. This needs explicit migration steps.

### R10. `walkAndRedact` Allocates New Maps for Every Object — Lines 594-613

```go
result := make(map[string]interface{})
for key, value := range v {
```

For a JSON response with 500 nested objects, this allocates 500 new maps even if no field matches. This is O(n) allocations where n is the total number of JSON objects in the response.

**Fix:** Use copy-on-write. Only allocate a new map if a field actually changes. Return the original map pointer if no modifications were made at any depth.

---

## 3. Additional Observations

### Concurrency Model

The `sync.RWMutex` on `RedactionManager` is the right choice. Pattern updates (writes) are rare; redaction (reads) is frequent. The read lock in `RedactJSON` (line 555-556) correctly protects the pattern slice. However, be careful about the lock scope: `json.Unmarshal` and `json.Marshal` inside the read lock (lines 559, 575) are potentially slow operations. Consider copying the pattern slice under a short read lock, then operating on the copy without holding any lock.

### Testing Strategy

The spec lists 49 test cases, which is thorough. Missing tests:
- **50.** Concurrent add + redact: add a pattern while redaction is in progress
- **51.** Update a pattern that is currently being used in a `test` action
- **52.** JSON path with unicode keys (`$.["emoji-field"]`)
- **53.** Regex with named groups where group name matches a JSON key
- **54.** Hash determinism across server restarts (relevant if salt is per-session)
- **55.** `clear` action while concurrent redaction is using custom patterns

### Alignment with Product Philosophy

Per `product-philosophy.md`, Gasoline should "capture, don't interpret." Redaction is inherently interpretation (deciding what is sensitive). However, redaction serves the privacy principle ("sensitive data never leaves localhost"), which is a higher-priority architectural constraint. The feature is justified, but the `test` action (showing matches without redacting) is the right design — it keeps the AI in the loop about what was redacted.

---

## 4. Implementation Roadmap

### Phase 1: Core Engine (Days 1-2)
1. **Fix overlap detection** (C2): Implement single-pass multi-pattern matching with priority resolution
2. **Fix stats atomics** (C1): Use `atomic.Int64` or separate stats lock
3. **Pre-compile path matchers** (C3): Wire `FieldRegex` through to `pathMatches`
4. **Extend hash to 16 chars + session salt** (C5)
5. Write unit tests for all replacement strategies (spec tests 6-12)
6. Write unit tests for priority and overlap handling (spec tests 18-21)

### Phase 2: Pattern Management (Days 2-3)
7. Implement `RedactionManager` struct with `Add`, `Update`, `Remove`, `Enable`, `Disable`, `List`, `Clear`
8. Add input validation: pattern length, name format, strategy/replacement mutual exclusion (R6)
9. Add pattern complexity check at compile time (C4)
10. Write management unit tests (spec tests 1-5, 22-29)

### Phase 3: JSON Field Targeting (Day 3)
11. Implement single-pass JSON walk with copy-on-write (R1, R10)
12. Pre-compile JSON path patterns at add time
13. Write field targeting tests (spec tests 13-17)

### Phase 4: MCP Tool Integration (Day 4)
14. Register `configure_redaction` tool in `tools.go` (decide on R5)
15. Implement all 8 actions with proper error responses
16. Wire `RedactionManager` into `handleToolsCall` redaction path
17. Migrate existing `RedactionEngine` callers
18. Write integration tests (spec tests 30-37)

### Phase 5: Performance and Edge Cases (Day 5)
19. Drop the redaction cache (R2, R3) — measure first; add back only if needed
20. Add `maxResponseSize` enforcement (R4)
21. Write performance benchmarks (spec tests 38-41)
22. Write edge case and concurrency tests (spec tests 42-49 + additional tests 50-55)
23. Run `go vet`, `make test`, `node --test` quality gates

---

## Appendix: Spec Section Cross-References

| Issue | Spec Section | Severity |
|-------|-------------|----------|
| C1. Stats data race | Implementation Details, lines 670-692 | Critical |
| C2. Overlap detection broken | Priority and Ordering, lines 652-697 | Critical |
| C3. Regex compiled per call | Per-Field Pattern Matching, line 635 | Critical |
| C4. ReDoS via complexity | Performance Considerations, lines 758-768 | Critical |
| C5. Hash too short, no salt | Replacement Strategies, lines 522-527 | Critical |
| R1. Double serialization | Per-Field Pattern Matching, lines 554-578 | High |
| R2. Cache memory unbounded | Caching Strategy, lines 775-823 | High |
| R3. Non-deterministic eviction | Caching Strategy, lines 798-808 | Medium |
| R4. No size enforcement | Performance Considerations, line 768 | High |
| R5. Tool naming conflict | API Surface, line 159 | Medium |
| R6. Replacement/Strategy conflict | Data Model, lines 48-51 | Medium |
| R7. Mask config silent full-mask | Replacement Strategies, lines 500-519 | Low |
| R8. ID randomness source | Implementation Details, lines 449-451 | Medium |
| R9. Migration underspecified | Migration Notes, lines 935-945 | High |
| R10. Unnecessary allocations | Per-Field Pattern Matching, lines 594-613 | Medium |
