# AI Capture Control Review (Migrated)

> **[MIGRATION NOTICE]**
> Migrated from `/docs/specs/ai-capture-control-review.md` on 2026-01-26.
> Related docs: [PRODUCT_SPEC.md](PRODUCT_SPEC.md), [TECH_SPEC.md](TECH_SPEC.md), [ADRS.md](ADRS.md).

---

# AI Capture Control - Technical Review

**Spec:** `docs/ai-first/tech-spec-ai-capture-control.md`
**Reviewer:** Principal engineer review
**Date:** 2026-01-26
**Implementation:** `cmd/dev-console/capture_control.go`, `extension/background.js`

---

## 1. Executive Summary

The spec is well-scoped and the core server-side implementation is solid -- session-scoped overrides, rate limiting, audit logging, and alert emission are all implemented with correct concurrency patterns and clean separation. However, the implementation has three categories of gaps: the extension only applies 3 of 5 controllable settings (missing `ws_mode` and `action_replay`), the agent identity is hardcoded to `"unknown"` because `clientInfo.name` from MCP initialize is never extracted or stored, and the `"capture"` action is missing from the `configure` tool's JSON Schema enum, making it invisible to MCP clients that rely on schema discovery.

---

## 2. Critical Issues (Must Fix Before Shipping)

### C1. Extension does not apply `ws_mode` or `action_replay` overrides

**Spec section:** "Controllable Settings" table, "Extension Integration"

The spec lists 5 controllable settings. The extension's `applyCaptureOverrides()` (background.js line 2051) only handles 3: `log_level`, `network_bodies`, `screenshot_on_error`. The `ws_mode` and `action_replay` settings are accepted by the server, stored in overrides, and returned by `/settings`, but the extension silently ignores them. This means the AI can call `configure(action: "capture", settings: { ws_mode: "messages" })`, receive a success response, and never see actual WebSocket payload data.

**Fix:** Add `ws_mode` and `action_replay` branches to `applyCaptureOverrides()`. For `ws_mode`, this requires the inject.js WebSocket hooks to respect a mutable mode setting rather than the current static config loaded at injection time. This may require a `postMessage`-based override push from background.js to inject.js, since inject.js runs in the page context and cannot access `chrome.storage` directly.

**Risk if unfixed:** The AI will believe it has elevated capture capabilities when it does not. This violates the product philosophy principle of "zero false confidence."

### C2. Agent identity is always `"unknown"`

**Spec section:** "Audit Log" (the `agent` field), line 122

The spec says: "It's extracted from the MCP client metadata (the `clientInfo.name` field from the MCP `initialize` request)." The `handleInitialize` method (main.go line 152) never extracts or stores `clientInfo.name`. The `toolConfigureCapture` handler (capture_control.go line 435) hardcodes `"unknown"` as the agent parameter.

**Fix:** Parse `clientInfo.name` from the MCP initialize params and store it on the `MCPHandler` or `ToolHandler`. Pass it through to `handleCaptureSettings` and `handleCaptureReset` instead of `"unknown"`. In connect mode, include the client identifier from the `X-Gasoline-Client` header.

**Risk if unfixed:** Enterprise audit log is useless for distinguishing which AI agent made a change, which is the stated purpose of the field.

### C3. `"capture"` action missing from configure tool's JSON Schema enum

**Spec section:** "Setting Changes via MCP"

The `configure` tool's `action` enum in `tools.go` line 584 is:
```go
"enum": []string{"store", "load", "noise_rule", "dismiss", "clear"}
```

The `"capture"` value is not listed, even though `toolConfigure` dispatches it (tools.go line 1211). MCP clients that validate against the schema (including Claude Code, which uses tool schemas for argument construction) will not discover or suggest `action: "capture"`. The handler works if called directly, but the schema declares it invalid.

**Fix:** Add `"capture"` to the enum. Also update the tool description to mention capture control.

---

## 3. Recommendations (Should Consider)

### R1. `GetPageInfo()` is missing `changed_at` timestamp

**Spec section:** "Page Info Integration"

The spec shows each override in the page info response should include `"changed_at": "2026-01-24T15:32:00Z"`. The implementation (`GetPageInfo()`, capture_control.go line 187) only returns `value` and `default`. The `CaptureOverrides` struct stores `lastChangeAt` globally but not per-setting.

**Recommendation:** Add a per-setting `changedAt` timestamp to the overrides map, or add the global `lastChangeAt` to each entry. This is useful for the AI to know how stale an override is.

### R2. Rate limiter counts validation failures as changes

**Spec section:** "Rate Limiting"

The `SetMultiple` method (line 109) checks the rate limit before validating individual settings. If an AI calls with 3 settings where 2 are valid and 1 is invalid, the rate limit is consumed but only 2 settings succeed. This is acceptable behavior, but there is a subtler issue: the rate limit window advances (`lastChangeAt` is set) even when all settings in a `SetMultiple` call fail validation, because the timestamp is set unconditionally at line 135 -- but actually, looking at the code more carefully, line 135 is inside `if len(changed) > 0`, so this is correctly guarded. No action needed.

### R3. User override conflict is hostile UX

**Spec section:** "Edge Cases" -- "User changes settings in popup while AI override is active"

The spec says user changes will be "overwritten on the next 5-second poll" and that to truly override the AI, the user should "disconnect the extension or restart the server." This is aggressive. A user manually changing a setting should be an intentional action that takes precedence.

**Recommendation:** Add a `user_override` flag in `chrome.storage.sync` that the extension sets when the user manually changes a setting while AI overrides are active. When this flag is set, the extension should stop applying AI overrides for that specific setting until the AI explicitly sets it again. This preserves user agency without requiring a server restart. At minimum, document this behavior prominently in the popup's "AI-controlled" indicator.

### R4. Audit log rotation has a TOCTOU race

**Spec section:** "Rotation"

The `rotate()` method (capture_control.go line 291) closes the file, renames files, then opens a new file. Between `file.Close()` and the new `os.OpenFile`, concurrent `Write()` calls that acquire the mutex will write to a closed file. This is prevented by the mutex, so it is safe, but if `os.OpenFile` fails after rotation (disk full, permissions), `al.file` becomes the zero-value and subsequent writes will panic or silently fail.

**Recommendation:** If the new file open fails, set a flag that disables further writes rather than leaving `al.file` in an indeterminate state. Or assign the old (now-renamed) file handle back and log the rotation failure.

### R5. No `session_end` audit event on server shutdown

**Spec section:** "Audit Log" format example shows `{\"event\":\"capture_reset\",\"reason\":\"session_end\",\"source\":\"server\"}`

The spec defines a `session_end` event that should be written when the server shuts down. The implementation has no shutdown hook for the audit logger. The `AuditLogger.Close()` method exists but is never called from `main.go`'s shutdown path.

**Recommendation:** Wire `auditLogger.Close()` into the graceful shutdown path. Before closing, write the `session_end` event.

### R6. Settings poll is on a fixed 5-second interval, not on-demand

**Spec section:** "Extension Integration"

The extension polls `/settings` every 5 seconds as part of its connection check. This means a capture override takes up to 5 seconds to take effect. For debugging workflows where the AI needs data "now," this delay is material. Consider also triggering a settings re-fetch when the extension receives a pending query response that includes capture overrides, or reduce the poll interval to 1-2 seconds (it is already a cheap GET).

### R7. Multi-agent conflict resolution is underspecified

**Spec section:** "Edge Cases" -- "Multiple AI clients"

"Last writer wins" is stated but not enforced with any awareness. If Agent A sets `ws_mode: "messages"` and Agent B sets `ws_mode: "off"` 2 seconds later, Agent A has no way to know its override was superseded. The `observe(what: "page")` response shows current overrides but not who set them.

**Recommendation:** Include the `agent` name in the `GetPageInfo()` response for each override. This lets agents detect when their overrides have been superseded.

### R8. Consider adding a `settings` parameter to the configure tool schema

**Spec section:** "Setting Changes via MCP"

The `configure` tool's JSON Schema does not declare a `settings` property for the `capture` action. This means AI clients get no schema-level guidance on what to pass. Adding a `settings` property with a description (or a oneOf for the map vs. "reset" string) would improve discoverability.

---

## 4. Implementation Roadmap (Ordered Steps)

### Phase 1: Fix Schema and Identity (server-only, no extension changes)

1. **Add `"capture"` to configure tool enum** (tools.go line 584). Add `"capture"` to the action enum and update the tool description. Add a `settings` property to the schema.
2. **Extract and store `clientInfo.name`** from MCP initialize params. Store on `MCPHandler`. Pass to `toolConfigureCapture` instead of hardcoded `"unknown"`.
3. **Wire audit logger shutdown** in `main.go` shutdown paths. Write `session_end` event before close.
4. **Add `changed_at` to `GetPageInfo()`** response. Store per-setting timestamps in `CaptureOverrides`.
5. **Tests:** Update `capture_control_test.go` to verify agent identity propagation, schema completeness, and `changed_at` presence.

### Phase 2: Extension Integration (extension changes)

6. **Implement `ws_mode` override in extension.** This requires a message channel from background.js to inject.js (via content.js) to update the WebSocket capture mode at runtime. The inject.js hook must check a mutable variable rather than the static config.
7. **Implement `action_replay` override in extension.** Toggle the action recording hooks based on the override value.
8. **Reduce settings poll to 2 seconds** or add an event-driven path.
9. **Tests:** Add `extension-tests/capture-control.test.js` cases for all 5 settings, including override application and revert-on-reset.

### Phase 3: UX and Robustness

10. **Add agent identity to `GetPageInfo()` per-override entries** for multi-agent visibility.
11. **Handle audit logger rotation failure gracefully** (set a disabled flag rather than leaving file handle invalid).
12. **Improve user-override UX**: add popup messaging that explains AI control and provides a one-click "take back control" button that resets overrides via a DELETE to `/settings` or similar.
13. **Add integration test** that exercises the full loop: MCP `configure` call, `/settings` poll, extension override application, `observe(what: "page")` confirmation.

---

## 5. Minor Notes

- The `maxPostBodySize` constant used in HTTP handlers is not visible in the reviewed files but is referenced. Confirm it is >= 1MB to handle large batch posts.
- The `CaptureOverrides` struct uses a `sync.RWMutex` correctly throughout. The `Set` and `SetMultiple` methods use exclusive locks; `GetAll`, `GetPageInfo`, and `GetSettingsResponse` use read locks. No concurrency issues found.
- The audit log JSONL format is clean and parseable. Field names are consistent. The rotation logic correctly shifts `.1` -> `.2` -> `.3` and caps at 3 files.
- Test coverage in `capture_control_test.go` is strong: 27 tests covering validation, rate limiting, concurrency, alerts, audit logging, rotation, nil safety, and session scoping. The gap is in integration-level tests that exercise the MCP tool handler end-to-end.
