# Review: Rate Limiting & Circuit Breaker

## Executive Summary

This spec is already implemented in `rate_limit.go` and the implementation is solid. The sliding window counter, circuit breaker state machine, and health endpoint all align with the spec. The critical gap is in the window reset logic -- the current implementation resets the window lazily on the next `RecordEvents` call, which means the streak counter can stall if traffic stops. The extension-side backoff logic is spec-only (not yet visible in the Go codebase) and has a design flaw in the retry budget interaction with the circuit breaker.

## Critical Issues

### 1. Lazy Window Reset Creates Stale Streak Counter

In `rate_limit.go` (lines 40-53), the window resets only when `RecordEvents` is called and detects the window has expired. The `tickRateWindow` call happens at this reset point. If traffic exceeds the threshold for 4 consecutive seconds and then stops entirely, `tickRateWindow` is never called for the 5th second. The streak counter stays at 4 and the circuit never opens -- even though the sustained overload was real.

Conversely, when traffic resumes minutes later, the next `RecordEvents` call triggers a single `tickRateWindow` for the stale window. This records one below-threshold tick and resets the streak to 0. The 4 seconds of overload are forgotten.

**Fix**: Run `tickRateWindow` in a background goroutine on a 1-second ticker. This ensures the streak advances even without incoming traffic. The spec says "every second, the counter resets" (line 15), which implies an active timer, not lazy evaluation.

However, this conflicts with the project's zero-goroutine constraint stated in the Performance Constraints section (line 138: "No additional HTTP requests or goroutines"). If goroutines are off the table, document this limitation explicitly: the circuit breaker requires sustained traffic at the threshold to open. A burst followed by silence will not trigger it.

### 2. `CheckRateLimit` and `RecordEvents` Race Condition

`CheckRateLimit` (lines 57-78) acquires an `RLock`, while `RecordEvents` (lines 40-53) acquires a full `Lock`. Both check `time.Now()` against `rateWindowStart` independently. In the sequence:

1. Request A calls `CheckRateLimit` -- window has expired, returns false (rate is "effectively 0")
2. Request B calls `RecordEvents` -- window has expired, triggers `tickRateWindow`, resets counter
3. Request A proceeds to call `RecordEvents` -- counter is now 0 + batch size

This is safe but can under-count: if A's batch would have pushed the old window over threshold, the tick missed it. In practice this is a minor race since the window is only 1 second, but it means the circuit breaker cannot guarantee exact threshold enforcement.

**Fix**: Combine `CheckRateLimit` and `RecordEvents` into a single atomic operation: `RecordAndCheck(count int) bool`. The handler calls this once per request, getting both the recording and the limit check under the same lock. This eliminates the TOCTOU gap.

### 3. Extension Retry Budget vs. Circuit Breaker Conflict (Spec Section)

The spec defines two independent mechanisms on the extension side:
- **Retry budget**: 3 attempts per batch, then abandon (line 55)
- **Circuit breaker**: 5 consecutive failures, then 30-second pause (lines 46-54)

The interaction is undefined. If a batch hits its 3-retry budget on failures 3, 4, 5 -- does that count toward the circuit breaker's consecutive failure counter? If the batch is abandoned after 3 retries, does the circuit breaker's counter reset (because no failure occurred on attempt 4), or does it increment?

Likely intent: consecutive failures count regardless of retry budget. But the spec should state this explicitly. If retry budget resets the failure counter (because the extension "moves on" to a new batch), the circuit breaker never opens because no batch generates 5 consecutive failures -- each batch only generates 3.

**Fix**: Clarify in the spec: the `consecutiveFailures` counter increments on every failed POST, regardless of which batch it belongs to. The retry budget controls how many times a single batch is retried; the circuit breaker tracks overall server health.

## Recommendations

### A. Memory-Based Rate Limiting is Disconnected from Event Rate

The spec says the circuit breaker monitors memory >50MB (line 27). The implementation in `evaluateCircuit` (lines 99-117) checks `getMemoryForCircuit() > memoryHardLimit`. However, `evaluateCircuit` is only called from `tickRateWindow`, which only runs when a window resets. If memory grows past 50MB due to large bodies but event rate stays at 500/sec (under threshold), `tickRateWindow` resets the streak to 0 each second, `evaluateCircuit` runs, and the memory check fires correctly.

But if event rate is 0 (no traffic), memory is never checked. A stale session with accumulated buffers will never trigger the circuit. This is the same lazy-evaluation problem from Critical Issue 1.

Consider adding a memory check to `CheckRateLimit` itself (which already reads `isMemoryExceeded` on line 67). This provides a synchronous check even without the ticker. The implementation already does this -- good. But `isMemoryExceeded` and the circuit's `memoryHardLimit` may use different thresholds. Verify they are aligned.

### B. Health Endpoint Should Include Retry-After for Circuit Open

The health endpoint (lines 200-210) returns circuit state but no `Retry-After` hint. The spec mentions the extension can poll health "cheaply to detect circuit state changes" (line 122). Adding an estimated time to circuit close (based on `lastBelowThresholdAt` and `circuitCloseSeconds`) would let the extension schedule its probe more intelligently instead of polling every 30 seconds.

### C. Rate Limit Response Should Use Batch-Aware Retry-After

The 429 response always returns `retry_after_ms: 1000` (line 99 of spec, line 186 of implementation). If the batch contained 5000 events (5x the threshold), 1 second is insufficient -- the next window will also exceed threshold if the same batch is retried. Consider `retry_after_ms = max(1000, batch_size / threshold * 1000)` to scale backoff with burst size.

### D. Test Scenario 8 Is Critical for Correctness

"Event count increments by batch size, not request count" -- this is already correct in the implementation (line 50: `v.windowEventCount += count`). But the test should verify edge cases: a single request with 1001 events should trigger rate limiting in one request, not after 1001 requests.

### E. Missing Test: Circuit Close Requires Both Conditions

Test scenario 6 says "Rate drops below threshold for 10 seconds + memory below 30MB -> circuit closes." Ensure there is a test where rate drops but memory remains above 30MB -- the circuit must stay open. And vice versa. The implementation handles this correctly (lines 120-141) but both partial-satisfaction cases need test coverage.

## Implementation Roadmap

1. **Combine `CheckRateLimit` + `RecordEvents` into `RecordAndCheck`** -- Eliminates the TOCTOU race. All three ingest handlers call the combined method. This is the most impactful change for correctness.

2. **Clarify extension retry budget vs. circuit breaker interaction** in the spec. Then implement the extension-side logic accordingly.

3. **Decide on background ticker vs. lazy evaluation** for window advancement. If lazy evaluation is acceptable, add explicit documentation of the limitation. If a ticker is chosen, it must be the only goroutine the rate limiter adds.

4. **Add batch-aware `Retry-After`** calculation to `WriteRateLimitResponse`.

5. **Add test cases** for: (a) circuit stays open when rate drops but memory remains high, (b) circuit stays open when memory drops but rate remains high, (c) single request with >1000 events triggers immediate rate limit, (d) zero traffic after sustained overload -- verify streak behavior.

6. **Implement extension-side backoff** in `extension/background.js` with tests in `extension-tests/rate-limit.test.js`. This is the largest remaining work item.
