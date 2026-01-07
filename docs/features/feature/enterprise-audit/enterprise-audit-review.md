---
> **[MIGRATED: 2024-xx-xx | Source: /docs/specs/enterprise-audit-review.md]**
> This file was migrated as part of the documentation reorganization. Please update links and references accordingly.
---

# Enterprise Audit & Governance — Technical Review

**Reviewer:** Principal Engineer
**Spec:** `docs/ai-first/tech-spec-enterprise-audit.md`
**Codebase:** `cmd/dev-console/` (Go server, ~80 source files)
**Date:** 2026-01-26

---

## Executive Summary

The spec is well-structured and architecturally sound. Its tiered approach, opt-in design, and zero-dependency constraint are the right calls for a localhost dev tool. However, significant portions of Tier 1 (audit trail, redaction, session management, TTL) and parts of Tier 3 (health metrics, auth, configurable thresholds) are already implemented in the codebase. The spec should be reconciled with existing code to avoid duplicate work and divergent contracts. Beyond that, Tier 4 (project isolation) introduces a shared-memory model that will require non-trivial refactoring of the `Capture` struct, and the per-tool rate limiter spec has concurrency gaps that need resolution before implementation.

---

## Critical Issues (Must Fix Before Implementation)

### C1. Spec Duplicates Existing Implementation — Risk of Divergent Contracts

**Sections affected:** 1.1, 1.2, 1.3, 1.4, 2.1, 2.4, 3.1, 3.4

The following features already exist in the codebase:

| Spec Section | Existing File | Status |
|---|---|---|
| 1.1 Tool Invocation Log | `audit_trail.go` — `AuditTrail` with `Record()`, `Query()` | Implemented |
| 1.2 Client Identification | `audit_trail.go` — `IdentifyClient()`, `ClientIdentifier` | Implemented |
| 1.3 Session ID Assignment | `audit_trail.go` — `CreateSession()`, `generateSessionID()` | Implemented (hex format, not base-36 as spec says) |
| 1.4 Redaction Audit Log | `audit_trail.go` — `RecordRedaction()`, `RedactionEvent` | Implemented |
| 2.1 TTL-Based Retention | `ttl.go` — `ParseTTL()`, `SetTTL()`, `getEntriesWithTTL()` | Implemented for Server; partially for Capture |
| 2.4 Redaction Patterns | `redaction.go` — `RedactionEngine` with Luhn validation, custom config | Implemented (already has more patterns than spec lists) |
| 3.1 API Key Auth | `auth.go` — `AuthMiddleware` with constant-time comparison | Implemented (uses `X-Gasoline-Key` header, not `Authorization: Bearer` as spec says) |
| 3.4 Health Metrics | `health.go` — `HealthMetrics`, `MCPHealthResponse` | Implemented |

**The spec and code disagree on contracts.** Three concrete conflicts:

1. **Session ID format:** Spec says `s_m5kq2a_7f3xab` (base-36 prefix + random). Code uses 32-character hex string from `crypto/rand`. The hex approach is fine; the spec's base-36 scheme adds complexity without clear value for a single-session system. **Recommendation:** Update spec to match code (hex session IDs).

2. **Auth header:** Spec says `Authorization: Bearer <secret>`. Code uses `X-Gasoline-Key`. The custom header is actually better here — it avoids collision with real OAuth flows in intercepted network traffic. **Recommendation:** Update spec to match code (`X-Gasoline-Key`).

3. **Redaction patterns:** Code (`redaction.go`) includes `aws-key`, `github-pat`, `basic-auth`, and `private-key` patterns with Luhn validation on credit cards. The spec lists a smaller set without validation. Code is superior. **Recommendation:** Update spec to document what exists.

**Action:** Reconcile the spec with existing code. Mark implemented features. Define delta work only.

---

### C2. Per-Tool Rate Limiter Has Race Condition

**Section:** 3.2

The spec defines per-tool rate limits with a "sliding window" that "resets every 60 seconds." A true sliding window requires tracking individual request timestamps. A fixed window (resetting on interval boundaries) is simpler but produces edge-case bursts at window boundaries (up to 2x the limit in a 1-second span straddling two windows).

Neither approach is currently implemented. The existing rate limiter in `rate_limit.go` operates on the HTTP ingest path (events/second), not on MCP tool calls.

**Problem:** The spec says rate limits apply "per MCP tool" but `handleToolCall` in `tools.go` is currently a single `switch` with no pre-dispatch hook. The `ToolHandler` struct has no per-tool counters, no sliding window state, and no locking strategy for tool-level rate limiting.

More critically, `currentClientID` on `ToolHandler` (line 156 of `tools.go`) is set and cleared per-HTTP request with no mutex — it is a data race waiting to happen if multiple HTTP clients call `/mcp` concurrently:

```go
defer func() { h.toolHandler.currentClientID = "" }()
```

**Recommendation:**

1. Use a fixed-window counter per tool (simpler, adequate for this use case). Store as `map[string]*toolRateState` behind a mutex, where each entry tracks `count` and `windowStart`.
2. Fix `currentClientID` by passing it through the call chain as a parameter instead of storing it on the struct. This is a pre-existing bug that will get worse under multi-client scenarios.
3. Check rate limits in a pre-dispatch wrapper inside `handleToolCall`, before the tool-name switch.

---

### C3. Project Isolation (Tier 4.1) Requires Capture Struct Decomposition

**Section:** 4.1

The `Capture` struct in `network.go`/`websocket.go` is a monolithic state holder: `wsEvents`, `networkBodies`, `enhancedActions`, `connections`, `pendingQueries`, `queryResults`, `perf`, `a11y`, plus the circuit breaker state, noise rules, and memory enforcement — all behind a single `sync.RWMutex`.

Project isolation requires independent buffer sets per project. The spec says each project gets "independent buffer sets (console, network, WebSocket, actions, performance)" plus "independent checkpoint storage" and "independent noise rules."

This means either:

- **Option A:** Create one `Capture` instance per project, with a `ProjectManager` that routes incoming data and MCP queries to the correct instance. This is clean but requires the HTTP endpoints to include a project identifier in every request, and all MCP tools to pass a project parameter.
- **Option B:** Keep a single `Capture` but add per-project namespacing to all buffer slices. This is invasive — every buffer access becomes `capture.wsEvents[projectID]` — and the existing `sync.RWMutex` becomes a contention bottleneck.

Option A is clearly better, but it is a significant refactor. The spec's "largest-first eviction" policy (Section 4.1) also requires a global memory coordinator that can inspect all project instances and evict from the largest one. This does not exist.

**Recommendation:** Defer Tier 4.1 to a separate implementation phase. Extract the buffer collections from `Capture` into a `BufferSet` struct first (preparatory refactor), then build project isolation on top. Do not attempt this in the same PR as Tiers 1-3.

---

### C4. export_data Tool Unbounded Response Size

**Section:** 2.3

The `export_data` tool returns "all buffer contents" as the tool response in JSON Lines format. With default buffer sizes (1000 console entries + 500 network bodies at up to 100KB each + 1000 WebSocket events + 200 actions), the worst-case response is approximately 50MB+ of JSON text returned in a single MCP tool response.

MCP clients (Claude Code, Cursor) parse the full response into memory. A 50MB JSON string will cause significant memory pressure in the AI client, and many clients impose response size limits (Claude Code's context window has practical limits around 200K tokens).

**Recommendation:**

1. Add a mandatory `max_size` parameter (default 1MB) that truncates the export with a warning.
2. For full exports, write to a temp file and return the file path instead (similar to how `generate` with `save_to` works for SARIF/HAR).
3. At minimum, document the size risk and recommend `save_to` for large exports.

---

## Recommendations (Should Consider)

### R1. TTL Not Applied to All Buffer Read Paths

**Section:** 2.1

`ttl.go` implements `getEntriesWithTTL()` for the Server's log entries, but TTL filtering is not applied in the actual tool read paths. Specifically:

- `toolGetBrowserErrors` (tools.go:1224) reads `h.MCPHandler.server.entries` directly — no TTL check.
- `toolGetBrowserLogs` (tools.go:1246) reads `h.MCPHandler.server.entries` directly — no TTL check.
- Network bodies, WebSocket events, and actions in `Capture` have `addedAt` parallel slices but no TTL filter functions.

The TTL infrastructure exists but is not wired into read paths. This is a straightforward fix: replace direct `entries` access with `getEntriesWithTTL()` and add equivalent filter functions for Capture buffers.

---

### R2. Audit Trail is Not Wired Into Tool Dispatch

The `AuditTrail` struct exists and the `get_audit_log` tool handler works, but tool invocations are not actually being recorded. Looking at `handleToolCall` in `tools.go`, there is no call to `h.auditTrail.Record(...)` anywhere in the dispatch path. The audit trail has zero entries unless something else populates it.

Similarly, `h.healthMetrics.IncrementRequest()` is never called from the dispatch path — the health metrics counters are always zero.

**Recommendation:** Add a pre/post dispatch wrapper in `handleToolCall` that:
1. Records start time
2. Calls `h.auditTrail.Record()` with tool name and parameter summary
3. Calls `h.healthMetrics.IncrementRequest()` / `IncrementError()`
4. Updates the entry with duration and response size after execution

This is a 20-line change that activates all of Tier 1 and the audit portion of Tier 3.4.

---

### R3. Configuration Profile Implementation Should Use Functional Options

**Section:** 2.2

The spec defines three profiles (`short-lived`, `restricted`, `paranoid`) as bundles of flags. Implementing this as a series of `if profile == "restricted" { ... }` blocks in `main()` will be fragile and untestable.

**Recommendation:** Define profiles as `ServerConfig` structs:

```go
type ServerConfig struct {
    TTL              time.Duration
    ReadOnly         bool
    RedactionPatterns []string  // pattern names to enable
    RateLimits       map[string]int
    ToolAllowlist    []string
    AuditSize        int
    // ... all configurable values
}

var profiles = map[string]ServerConfig{
    "short-lived": { TTL: 15*time.Minute, ... },
    "restricted":  { TTL: 30*time.Minute, ReadOnly: true, ... },
    "paranoid":    { TTL: 5*time.Minute, ReadOnly: true, ... },
}
```

Then merge: `defaults <- profile <- config file <- env vars <- CLI flags`. Each layer overwrites non-zero fields. This is testable, composable, and extensible.

---

### R4. Read-Only Mode Should Use a Middleware, Not Per-Tool Checks

**Section:** 4.2

The spec lists specific operations that read-only mode disables. Implementing this as `if readOnly { return error }` checks inside each mutation tool is error-prone — new mutation tools could forget the check.

**Recommendation:** Tag each tool with a `mutates: bool` field in the tool definition. Add a single check in `handleToolCall` before dispatch:

```go
if h.readOnly && toolDef.Mutates {
    return readOnlyError(req)
}
```

This guarantees all mutation tools are blocked without per-tool code changes.

---

### R5. Redaction Engine Performance — Compile Once, Apply Many

**Section:** 2.4

The existing `RedactionEngine` in `redaction.go` already compiles patterns once at startup and reuses them. This is correct. However, the spec's performance claim of "< 2ms for 7 patterns on < 5KB text" should be validated with a benchmark test.

The existing implementation applies patterns sequentially. For 10+ custom patterns on large responses, consider:
1. Combining all patterns into a single alternation regex: `(pattern1|pattern2|...)`. Go's RE2 engine handles alternation efficiently.
2. Adding a fast-path: if the response contains no alphanumeric characters longer than 8 consecutive chars, skip redaction entirely (no secrets in short tokens).

The current implementation walks all string fields in the MCP response JSON by parsing, redacting text blocks, and re-serializing (`RedactJSON`). This is the right approach — it avoids accidentally redacting JSON structural characters.

---

### R6. Tool Allowlist/Blocklist Should Filter at tools/list Time

**Section:** 4.3

The spec correctly says hidden tools should not appear in `tools/list`. The current `toolsList()` method in `tools.go` returns a hardcoded slice of all tools. Implementing filtering requires:

1. Store the allow/block config on `ToolHandler`.
2. Filter the slice in `toolsList()` before returning.
3. In `handleToolCall`, check before the switch statement (not inside each case).

The spec correctly specifies that if both allow and block are set, allow takes priority. This is the right call — it avoids ambiguity.

---

### R7. API Key Should Be Compared With Authorization: Bearer OR X-Gasoline-Key

**Section:** 3.1

The existing `AuthMiddleware` uses `X-Gasoline-Key`. The spec proposes `Authorization: Bearer`. Supporting both is trivial and avoids a breaking change:

```go
key := r.Header.Get("X-Gasoline-Key")
if key == "" {
    auth := r.Header.Get("Authorization")
    key = strings.TrimPrefix(auth, "Bearer ")
}
```

This maintains backward compatibility while supporting the standard header.

---

### R8. Memory Constant Mismatch

**Section:** 3.3

The spec says the memory hard limit default is 50MB. The code in `memory.go` defines `memoryHardLimit` (used but not shown — referenced from `rate_limit.go` and `health.go`). However, `memory.go` defines `memorySoftLimit = 20MB` and `memoryCriticalLimit = 100MB`. The hard limit value should be verified for consistency — the spec's configurable thresholds table (Section 3.3) lists a 50MB default for hard limit and 20MB for soft limit, which aligns, but the code path for `memoryHardLimit` should be confirmed before making it configurable.

---

## Implementation Roadmap (Ordered Steps)

### Phase 1: Wire Existing Code (1-2 days)

The highest-value, lowest-effort work. These features are built but not connected.

1. **Wire audit recording into tool dispatch.** Add pre/post hooks in `handleToolCall` that call `auditTrail.Record()` and `healthMetrics.IncrementRequest()`. This activates Tier 1.1, 1.2, and 3.4 audit metrics.
2. **Wire TTL filtering into read paths.** Replace direct `server.entries` access in tool handlers with `getEntriesWithTTL()`. Add equivalent TTL filters for Capture buffers. This activates Tier 2.1.
3. **Fix `currentClientID` data race.** Pass client ID as a parameter through the call chain instead of storing on `ToolHandler`. Pre-existing bug — fix before adding more concurrent access.

### Phase 2: Per-Tool Rate Limits + Read-Only Mode (2-3 days)

New but straightforward features.

4. **Implement per-tool rate limiter.** Fixed-window counter per tool name, checked in `handleToolCall` pre-dispatch. Default limits as specified. Configurable via `--rate-limits` flag.
5. **Implement read-only mode.** Add `readOnly` field to `ToolHandler`. Check before dispatch. Tag mutation tools. Activate via `--read-only` flag.
6. **Implement tool allow/blocklist.** Filter `toolsList()` output. Check in `handleToolCall`. `--tools-allow` and `--tools-block` flags.

### Phase 3: Configuration System (2-3 days)

7. **Implement `ServerConfig` struct.** Define all configurable parameters in one place.
8. **Implement config file parsing.** JSON file loaded at startup, merged with flags and env vars using the priority order: CLI > env > config > defaults.
9. **Implement profiles.** Three named presets (`short-lived`, `restricted`, `paranoid`) that pre-populate `ServerConfig`.
10. **Make buffer sizes configurable.** Replace hardcoded constants (`maxWSEvents`, `maxNetworkBodies`, etc.) with values from `ServerConfig`.

### Phase 4: Export Tool (1 day)

11. **Implement `export_data` MCP tool.** Support `audit` and `captures` scopes. JSON Lines output. Enforce response size limit (default 1MB). Support `save_to` parameter for large exports.

### Phase 5: Project Isolation (Tier 4) — Separate Phase (5+ days)

12. **Extract `BufferSet` from `Capture`.** Preparatory refactor — move buffer slices, `addedAt` slices, and noise config into a standalone struct.
13. **Implement `ProjectManager`.** Create/delete projects, route HTTP data and MCP queries by project ID.
14. **Implement global memory coordinator.** Shared memory budget with largest-first eviction across projects.
15. **Update extension and MCP protocol.** Project selection in extension options and MCP `initialize` params.

### Quality Gates (Apply at Each Phase)

- Tests first (TDD per `testing.md`).
- `make test` + `go vet` + `node --test` pass.
- Race detector clean: `go test -race ./cmd/dev-console/`.
- Benchmark redaction and rate-limit paths to validate SLO claims.
