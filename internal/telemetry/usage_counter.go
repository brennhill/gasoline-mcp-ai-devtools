// usage_counter.go — Structured tool usage tracking for telemetry beacons.

package telemetry

import (
	"strings"
	"sync"
	"time"
)

// Channel is the release channel (e.g., "stable", "beta", "dev").
var Channel = "dev"

// Usage-key sentinels exposed on the telemetry wire when a tool call cannot
// be classified into a normal mode bucket. Dashboards consume these strings
// directly — renaming any of them is a breaking telemetry change.
//
// Producer: cmd/browser-agent/tools_core.go::usageKey().
// Tests:    cmd/browser-agent/handler_tools_call_usage_test.go.
const (
	// UsageKeyUnknownNoArgs is recorded when the caller passes nil/empty args.
	UsageKeyUnknownNoArgs = "unknown_no_args"

	// UsageKeyUnknownParseError is recorded when args are non-empty but cannot
	// be parsed into a JSON object (malformed JSON, top-level array/string).
	UsageKeyUnknownParseError = "unknown_parse_error"

	// UsageKeyUnknownMissingWhat is recorded when args parse but produce no
	// usable mode (no `what`, no recognised alias, JSON null, type mismatch).
	UsageKeyUnknownMissingWhat = "unknown_missing_what"

	// UsageKeyLegacyAliasPrefix tags calls that landed via a deprecated alias
	// instead of the canonical `what` field. Format: "<prefix><alias>:<value>"
	// (e.g. "legacy_action:click"). Lets dashboards count old-shape clients
	// without polluting the canonical bucket.
	UsageKeyLegacyAliasPrefix = "legacy_"
)

// ToolStat holds per-tool aggregated metrics for one beacon window.
type ToolStat struct {
	Tool         string `json:"tool"`   // "observe:page"
	Family       string `json:"family"` // "observe"
	Name         string `json:"name"`   // "page"
	Count        int    `json:"count"`
	ErrorCount   int    `json:"error_count"`
	LatencyAvgMs int64  `json:"latency_avg_ms"`
	LatencyMaxMs int64  `json:"latency_max_ms"`
}

// UsageSnapshot is the structured output of SwapAndReset.
type UsageSnapshot struct {
	ToolStats     []ToolStat     `json:"tool_stats"`
	AsyncOutcomes map[string]int `json:"async_outcomes"`
}

// toolAccum accumulates per-tool metrics within one beacon window.
type toolAccum struct {
	count      int
	errCount   int
	latencySum int64
	latencyMax int64
}

// UsageTracker is a thread-safe tracker for tool call analytics.
type UsageTracker struct {
	mu            sync.Mutex
	tools         map[string]*toolAccum // key: "family:name"
	asyncOutcomes map[string]int        // "complete", "timeout", etc.
	sessionCalls int      // total calls this session
	sessionStart time.Time // when session started (for duration calc)
}

// NewUsageTracker creates a new empty usage tracker.
// Registers a session-end callback so timeout-based rotation emits session_end.
func NewUsageTracker() *UsageTracker {
	t := &UsageTracker{
		tools:         make(map[string]*toolAccum),
		asyncOutcomes: make(map[string]int),
	}
	SetSessionEndCallback(func(reason string) {
		t.EmitSessionEnd(reason)
	})
	return t
}

// splitKey splits "observe:page" into ("observe", "page").
func splitKey(key string) (family, name string) {
	if i := strings.IndexByte(key, ':'); i >= 0 {
		return key[:i], key[i+1:]
	}
	return key, ""
}

// RecordToolCall records a tool call with latency and outcome.
// Fires a per-call tool_call beacon and aggregates for usage_summary.
func (u *UsageTracker) RecordToolCall(key string, elapsed time.Duration, isError bool) {
	ms := elapsed.Milliseconds()
	family, name := splitKey(key)

	u.mu.Lock()
	acc := u.tools[key]
	if acc == nil {
		acc = &toolAccum{}
		u.tools[key] = acc
	}
	acc.count++
	acc.latencySum += ms
	if ms > acc.latencyMax {
		acc.latencyMax = ms
	}
	if isError {
		acc.errCount++
	}
	u.sessionCalls++
	u.mu.Unlock()

	// TouchSession may rotate the session (firing session_end callback which
	// resets u.sessionStart). Check for new session AFTER touch so post-timeout
	// rotation is detected.
	TouchSession()
	firstEver := markFirstToolCallEmittedForInstall()

	u.mu.Lock()
	newSession := u.sessionStart.IsZero()
	if newSession {
		u.sessionStart = time.Now()
	}
	u.mu.Unlock()

	// Emit session_start on first call of a new session.
	if newSession {
		fireStructuredBeacon(map[string]any{
			"event":  "session_start",
			"reason": ConsumeSessionStartReason(),
		})
	}

	// Fire per-call beacon.
	outcome := "success"
	if isError {
		outcome = "error"
	}
	fireStructuredBeacon(map[string]any{
		"event":      "tool_call",
		"family":     family,
		"name":       name,
		"tool":       key,
		"outcome":    outcome,
		"latency_ms": ms,
	})

	if firstEver {
		fireStructuredBeacon(map[string]any{
			"event":  "first_tool_call",
			"family": family,
			"name":   name,
			"tool":   key,
		})
	}
}

// RecordAsyncOutcome tracks the terminal status of an async command.
func (u *UsageTracker) RecordAsyncOutcome(status string) {
	u.mu.Lock()
	u.asyncOutcomes[status]++
	u.mu.Unlock()
}

// SessionDepth returns the total tool calls in the current session.
func (u *UsageTracker) SessionDepth() int {
	u.mu.Lock()
	d := u.sessionCalls
	u.mu.Unlock()
	return d
}

// Peek returns a flat count map for the debug endpoint (backward compat).
func (u *UsageTracker) Peek() map[string]int {
	u.mu.Lock()
	defer u.mu.Unlock()
	cp := make(map[string]int)
	for key, acc := range u.tools {
		cp[key] = acc.count
		if acc.errCount > 0 {
			cp["err:"+key] = acc.errCount
		}
	}
	for k, v := range u.asyncOutcomes {
		cp["async:"+k] = v
	}
	return cp
}

// SwapAndReset atomically returns the structured snapshot and resets counters.
func (u *UsageTracker) SwapAndReset() *UsageSnapshot {
	u.mu.Lock()

	if len(u.tools) == 0 && len(u.asyncOutcomes) == 0 {
		u.mu.Unlock()
		return nil // nothing to report
	}

	stats := make([]ToolStat, 0, len(u.tools))
	for key, acc := range u.tools {
		family, name := splitKey(key)
		avgMs := int64(0)
		if acc.count > 0 {
			avgMs = acc.latencySum / int64(acc.count)
		}
		stats = append(stats, ToolStat{
			Tool:         key,
			Family:       family,
			Name:         name,
			Count:        acc.count,
			ErrorCount:   acc.errCount,
			LatencyAvgMs: avgMs,
			LatencyMaxMs: acc.latencyMax,
		})
	}

	outcomes := make(map[string]int, len(u.asyncOutcomes))
	for k, v := range u.asyncOutcomes {
		outcomes[k] = v
	}

	u.tools = make(map[string]*toolAccum)
	u.asyncOutcomes = make(map[string]int)
	u.mu.Unlock()

	return &UsageSnapshot{
		ToolStats:     stats,
		AsyncOutcomes: outcomes,
	}
}

// EmitSessionEnd fires a session_end beacon. Called when the session rotates.
func (u *UsageTracker) EmitSessionEnd(reason string) {
	u.mu.Lock()
	calls := u.sessionCalls
	start := u.sessionStart
	u.sessionCalls = 0
	u.sessionStart = time.Time{}
	u.mu.Unlock()

	if calls == 0 {
		return
	}

	durationS := int64(0)
	if !start.IsZero() {
		durationS = int64(time.Since(start).Seconds())
	}

	fireStructuredBeacon(map[string]any{
		"event":      "session_end",
		"reason":     reason,
		"duration_s": durationS,
		"tool_calls": calls,
	})
}

// fireStructuredBeacon sends a beacon with the standard envelope + extra fields.
func fireStructuredBeacon(fields map[string]any) {
	event, ok := fields["event"].(string)
	if !ok || event == "" {
		return // silently drop malformed beacon
	}
	payload, ok := buildEnvelope(event)
	if !ok {
		callOnFireBeacon(false)
		return
	}
	payload["ts"] = time.Now().UTC().Format(time.RFC3339)
	payload["channel"] = Channel
	for k, v := range fields {
		if k != "event" { // event already in envelope
			payload[k] = v
		}
	}
	fireBeacon(payload)
}
