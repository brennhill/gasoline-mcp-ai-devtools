---

# Context Streaming Review (Migrated)

> **[MIGRATION NOTICE]**
> This file was migrated from `/docs/specs/context-streaming-review.md` on 2026-01-26 as part of the documentation modularization project.
> Related docs: [product-spec.md](product-spec.md), [tech-spec.md](tech-spec.md), [ADRS.md](ADRS.md).

---

# Context Streaming (Feature 5) - Technical Review

**Reviewer:** Principal Engineer Review
**Date:** 2026-01-26
**Specs Reviewed:** `docs/v6-specification.md` (lines 1379-1643) and `docs/ai-first/tech-spec-push-alerts.md`

---

## Executive Summary

The Context Streaming spec conflates two distinct delivery mechanisms (passive alert piggybacking vs. active MCP notifications) which have fundamentally different concurrency, error handling, and testing requirements. The passive mode implementation in `alerts.go` is sound and already partially implemented. However, the active streaming mode introduces significant architectural risks: (1) unsynchronized stdout writes from background goroutines will corrupt the MCP protocol stream, (2) the `SeenMessages` dedup cache has unbounded growth, and (3) the spec lacks a shutdown/cleanup path when the MCP client disconnects.

---

## 1. Critical Issues (Must Fix Before Implementation)

### 1.1 Stdout Corruption from Concurrent Writes

**Location:** v6-specification.md lines 1626-1634, tech-spec-push-alerts.md lines 253-265

**Problem:** The spec shows `emitNotification` writing directly to stdout:

```go
func (s *Server) emitNotification(event StreamEvent) {
	notification := map[string]interface{}{...}
	data, _ := json.Marshal(notification)
	fmt.Println(string(data))  // UNSAFE
}
```

The MCP stdin/stdout loop in `main.go:968-993` uses `fmt.Println()` to write responses. If a background goroutine emits notifications while the main goroutine is writing a tool response, the JSON messages will interleave and corrupt the stream. MCP clients will receive malformed JSON.

**Impact:** Protocol corruption, connection failures, impossible to debug.

**Fix:** Use a synchronized output channel or mutex-protected writer:

```go
type MCPWriter struct {
	mu sync.Mutex
	w  io.Writer
}

func (m *MCPWriter) Write(msg []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, err := m.w.Write(append(msg, '\n'))
	return err
}
```

All stdout writes (responses AND notifications) must go through this single writer.

### 1.2 Unbounded SeenMessages Cache Growth

**Location:** v6-specification.md lines 1509, 1545-1549, tech-spec-push-alerts.md line 291

**Problem:** The `SeenMessages` map grows without bound:

```go
type StreamState struct {
	SeenMessages   map[string]time.Time // Dedup: message -> last sent
	// ...
}
```

The 30-second dedup window check only prevents re-sending, but never evicts old entries. Over a long session with many unique error messages, this map grows indefinitely.

**Impact:** Memory leak proportional to unique error diversity. In pathological cases (DOM XSS scanner, fuzzing), this could exhaust memory.

**Fix:** Add bounded LRU eviction or periodic cleanup:

```go
const maxSeenMessages = 500

func (s *StreamState) cleanupSeenMessages(now time.Time) {
	if len(s.SeenMessages) > maxSeenMessages {
		// Evict entries older than 2x dedup window
		cutoff := now.Add(-60 * time.Second)
		for key, ts := range s.SeenMessages {
			if ts.Before(cutoff) {
				delete(s.SeenMessages, key)
			}
		}
	}
}
```

### 1.3 No Shutdown Path for Active Streaming

**Location:** tech-spec-push-alerts.md lines 207-210

**Problem:** The spec describes `disable` clearing state, but doesn't address what happens when:
1. The MCP client disconnects (stdin closes)
2. The HTTP server shuts down
3. A background goroutine is mid-write to a closed stdout

The main loop in `main.go:995-1012` handles stdin closure with a 100ms grace period, but doesn't coordinate with any active notification goroutines.

**Impact:** Goroutine leak, panic on write to closed pipe, race between cleanup and emission.

**Fix:**
1. Add `context.Context` to `StreamState`
2. Cancel context on stdin close
3. All notification emission must check context before write
4. Use `select` with context.Done() in any blocking operations

```go
type StreamState struct {
	ctx    context.Context
	cancel context.CancelFunc
	// ...
}

func (s *StreamState) emitNotification(event StreamEvent) {
	select {
	case <-s.ctx.Done():
		return // Shutdown in progress
	default:
	}
	// ... emit
}
```

### 1.4 Missing Lock Ordering Documentation

**Location:** alerts.go line 64, tech-spec-push-alerts.md line 116

**Problem:** The spec mentions `alertMu` is "separate from server.mu and capture.mu to avoid lock ordering issues" but doesn't document the actual ordering. The codebase has at least 15 different mutexes (grep shows `sync.Mutex|sync.RWMutex` in 20+ locations). Without documented ordering, future changes risk deadlock.

Current locks that might interact with streaming:
- `server.mu` (Server.entries)
- `capture.mu` (Capture.networkBodies, wsEvents, etc.)
- `alertMu` (AlertBuffer)
- `streamState.mu` (proposed StreamState)
- `cspGenerator.mu`, `securityScanner.mu`, etc.

**Impact:** Deadlock risk increases with each new mutex. Debugging production deadlocks is extremely difficult.

**Fix:** Add lock ordering documentation to `architecture.md`:

```
Lock Acquisition Order (always acquire in this order):
1. server.mu
2. capture.mu
3. alertMu
4. streamState.mu (if added)
5. component-specific locks (cspGenerator.mu, etc.)

Never hold a higher-numbered lock while acquiring a lower-numbered lock.
```

---

## 2. Recommendations (Should Consider)

### 2.1 Race Condition in CheckAndEmit

**Location:** v6-specification.md lines 1526-1570

**Problem:** The `CheckAndEmit` method holds the lock while performing multiple time-based checks, but the spec shows reading `event.Context["url"]` which may not exist:

```go
if s.Config.URLFilter != "" && !strings.Contains(event.Context["url"].(string), s.Config.URLFilter) {
```

If `event.Context["url"]` is nil or not a string, this panics while holding the mutex, leaving the system in a locked state.

**Fix:** Validate event structure before acquiring lock, or use type assertion with ok check:

```go
if s.Config.URLFilter != "" {
	url, ok := event.Context["url"].(string)
	if !ok || !strings.Contains(url, s.Config.URLFilter) {
		return false
	}
}
```

### 2.2 Batch Window Has No Flush Mechanism

**Location:** v6-specification.md lines 1494, 1561-1564

**Problem:** Events that don't meet the throttle threshold are added to `PendingBatch`, but there's no timer or mechanism to flush the batch after the window expires. Events could be lost if no subsequent event triggers the flush.

```go
if time.Since(s.LastNotified) < time.Duration(s.Config.ThrottleSeconds)*time.Second {
	s.PendingBatch = append(s.PendingBatch, event)
	return false  // WHO FLUSHES THIS?
}
```

**Fix:** Add a timer goroutine that flushes the batch after `ThrottleSeconds`, or document that batched events are lost (acceptable tradeoff).

### 2.3 FrustrationDetector Memory Growth

**Location:** v6-specification.md lines 1578-1617

**Problem:** The `clickHistory` map keyed by selector will grow with each unique element clicked. Long sessions with dynamic selectors (UUIDs in test-ids, generated class names) will accumulate entries.

```go
type FrustrationDetector struct {
	clickHistory map[string][]time.Time // selector -> click timestamps
	formSubmits  map[string]time.Time   // form action -> submit time
}
```

**Fix:** Add periodic cleanup (every 60s) that removes entries older than 10s from both maps.

### 2.4 MCP Notification Method Name

**Location:** tech-spec-push-alerts.md lines 216-242

**Problem:** The tech spec uses `notifications/message` which is correct per MCP spec, but v6-specification.md line 1467 uses `notifications/gasoline/event`. These must be consistent.

The MCP spec defines `notifications/message` with `level`, `logger`, `data` params. Using a custom method name (`notifications/gasoline/event`) may not be recognized by MCP clients.

**Fix:** Standardize on `notifications/message` per the tech spec. Update v6-specification.md to match.

### 2.5 Performance Regression Alert Threshold

**Location:** tech-spec-push-alerts.md lines 42-43

**Problem:** "20% degradation from baseline for timing metrics" may be too sensitive for normal variance. A page load of 1000ms vs 1200ms (20% regression) is often within noise.

Industry standard is typically 2x baseline or absolute thresholds (e.g., LCP > 2500ms regardless of baseline).

**Fix:** Consider tiered thresholds:
- Info: >20% regression
- Warning: >50% regression OR absolute threshold breach
- Error: >100% regression OR critical threshold breach (LCP > 4000ms)

### 2.6 URL Filter Applied Inconsistently

**Location:** v6-specification.md lines 1435-1437, 1539-1541

**Problem:** The URL filter is applied in `CheckAndEmit` by reading `event.Context["url"]`, but not all events have URLs (e.g., console errors, memory pressure alerts). The filter would crash or behave unexpectedly.

**Fix:** URL filter should only apply to events where a URL is semantically meaningful. Add event-type-aware filtering:

```go
func shouldApplyURLFilter(category string) bool {
	return category == "network_errors" || category == "performance" || category == "security"
}
```

---

## 3. Data Contract Issues

### 3.1 Alert Category Mismatch

**Location:** v6-specification.md lines 1427-1429 vs tech-spec-push-alerts.md lines 34-35

**v6-specification.md** lists event categories:
```json
["errors", "network_errors", "performance", "user_frustration", "security"]
```

**tech-spec-push-alerts.md** lists alert categories:
```json
["regression", "anomaly", "ci", "noise", "threshold"]
```

These are different taxonomies. The `configure_streaming` tool accepts the v6 categories, but the alert buffer uses the tech-spec categories. A translation layer is needed.

**Fix:** Document the mapping explicitly:

| v6 Event Category | Alert Categories |
|-------------------|------------------|
| errors | anomaly |
| network_errors | anomaly, regression |
| performance | regression, threshold |
| user_frustration | (new category needed) |
| security | (feeds from security_audit findings) |
| all | anomaly, regression, ci, noise, threshold |

### 3.2 Missing Type Definitions

**Location:** v6-specification.md lines 1498-1523

The spec defines `StreamConfig` and `StreamState` in Go, but doesn't provide JSON schema for the MCP tool input/output. The `configure_streaming` tool response format is not specified.

**Fix:** Add response schemas:

```json
// configure_streaming {action: "enable"} response
{
  "status": "enabled",
  "config": {
	"events": ["errors", "network_errors"],
	"throttle_seconds": 5,
	"url_filter": null,
	"severity_min": "warning"
  }
}

// configure_streaming {action: "status"} response
{
  "enabled": true,
  "config": {...},
  "stats": {
	"notifications_sent": 47,
	"notifications_throttled": 12,
	"notifications_deduped": 5,
	"pending_batch_size": 2,
	"seen_messages_count": 34
  }
}
```

### 3.3 Correlation ID Format

**Location:** tech-spec-push-alerts.md line 237

The spec shows `"correlation_id": "evt_abc123"` but doesn't specify how this ID is generated or its uniqueness guarantees.

**Fix:** Specify format: `evt_<unix_nano>_<random_4_chars>` or use `xid` library pattern.

---

## 4. Security Considerations

### 4.1 Redaction Timing

**Location:** tech-spec-push-alerts.md lines 246-265

**Problem:** Redaction is applied in `emitNotification`, but the event is constructed earlier. If the event is logged for debugging before redaction, sensitive data may leak.

**Fix:** Redact at event construction time, not emission time. The `StreamEvent` struct should never contain unredacted sensitive data.

### 4.2 CI Webhook Authentication

**Location:** tech-spec-push-alerts.md lines 100-102

"The webhook has no authentication (localhost-only tool)." This is acceptable for localhost, but if the port is exposed (e.g., in a container or tunnel), anyone can inject fake CI results.

**Fix:** Document that `POST /ci-result` should NOT be exposed externally. Consider adding optional bearer token authentication for production deployments.

### 4.3 Rate Limit Bypass via Category Switching

**Location:** v6-specification.md lines 1489-1494

The rate limit is global (12/minute), but deduplication is per-message. An attacker could generate 12 unique messages per minute in each category, exceeding intended load.

**Fix:** Rate limit should be per-category or total should account for category diversity.

---

## 5. Testing Surface

### 5.1 Missing Test Scenarios

The spec lists test scenarios but misses critical edge cases:

1. **Concurrent emit during tool call**: Notification fires while observe response is being written
2. **Rapid enable/disable toggle**: Enable, emit event, disable before event processed
3. **Clock skew in dedup**: System clock moves backward during dedup window
4. **Memory pressure during streaming**: Circuit breaker opens while notifications pending
5. **Extension reconnect during streaming**: Session ID changes mid-stream
6. **Large batch accumulation**: 1000 events arrive during 5s throttle window

### 5.2 Integration Test Gap

No integration test is specified for the full path:
1. Browser event -> Extension -> Server -> Alert buffer -> Observe response with alerts

This path involves multiple async boundaries and should have an E2E test.

---

## 6. Implementation Roadmap

### Phase 1: Foundation (3-4 days)

1. **Add synchronized MCP writer** (Critical 1.1)
   - Create `MCPWriter` with mutex
   - Refactor `main.go` stdin/stdout loop to use it
   - Update `sendMCPError` to use writer
   - Test: verify no interleaving under concurrent writes

2. **Document lock ordering** (Critical 1.4)
   - Audit all existing mutexes
   - Add ordering to `architecture.md`
   - Add deadlock detection in debug builds

3. **Bound SeenMessages cache** (Critical 1.2)
   - Add `maxSeenMessages` constant
   - Implement LRU eviction
   - Test: verify memory stable after 10k unique messages

### Phase 2: Active Streaming Core (4-5 days)

4. **Add StreamState with context** (Critical 1.3)
   - Create `StreamState` struct with `context.Context`
   - Wire cancel to stdin close
   - Add to `ToolHandler`

5. **Implement configure_streaming tool**
   - Enable/disable/status actions
   - Category and URL filtering
   - Return structured response per 3.2

6. **Implement notification emission**
   - Use synchronized writer
   - Apply redaction before emit
   - Respect rate limits and dedup

### Phase 3: Event Sources (3-4 days)

7. **Wire error events** (from `onEntries` callback in tools.go:206)
   - New console error -> check significance -> emit if streaming enabled

8. **Wire network error events** (from network body ingestion)
   - 5xx or status change -> emit

9. **Wire performance regression events** (from baseline comparison)
   - Already generates alerts in alert buffer -> emit to stream

### Phase 4: Advanced Features (2-3 days)

10. **Implement FrustrationDetector**
	- Click tracking with bounded map
	- Form timeout tracking
	- Wire to action capture callback

11. **Add batch flush timer**
	- Goroutine with ticker
	- Coordinate with context cancellation

### Phase 5: Testing & Hardening (2-3 days)

12. **Unit tests** per spec scenarios 15-24
13. **Integration test** for full event flow
14. **Race detection pass** with `-race` flag
15. **Memory profiling** under sustained event load

---

## Appendix: Files to Modify

| File | Changes |
|------|---------|
| `cmd/dev-console/main.go` | Add MCPWriter, wire context cancellation |
| `cmd/dev-console/tools.go` | Add `StreamState` to `ToolHandler`, add `configure_streaming` dispatch |
| `cmd/dev-console/streaming.go` | New file: `StreamState`, `StreamConfig`, `CheckAndEmit`, `emitNotification` |
| `cmd/dev-console/streaming_test.go` | New file: unit tests for streaming |
| `cmd/dev-console/alerts.go` | Add bridge from alert buffer to stream |
| `.claude/docs/architecture.md` | Document lock ordering |

---

**Recommendation:** Address Critical issues 1.1-1.4 before beginning implementation. The passive alert mode (piggybacking on observe) is lower risk and can ship independently. Active streaming should be gated behind a feature flag until thoroughly tested.
