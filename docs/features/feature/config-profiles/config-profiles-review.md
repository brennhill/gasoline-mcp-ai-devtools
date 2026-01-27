# Config Profiles Review

_Migrated from /specs/config-profiles-review.md_

# Configuration Profiles Spec Review

**Spec**: `docs/ai-first/tech-spec-config-profiles.md`
**Reviewer**: Principal Engineer Review
**Date**: 2026-01-26

---

## Executive Summary

The Configuration Profiles spec proposes a well-motivated feature: named configuration bundles with inheritance, runtime overrides, and enterprise distribution ("bank mode"). The data model is clean and the merge semantics are clearly defined. However, the spec has three critical gaps: (1) `ToolSettings.Enabled` uses `interface{}` which creates a type-safety hole across the entire tool filtering pipeline, (2) the profile activation path lacks atomicity guarantees for the concurrent buffer resize that must accompany settings changes, and (3) the spec introduces a parallel configuration system (`ProfileSettings`) that duplicates and conflicts with the existing `CaptureOverrides` and hardcoded constants in `types.go`, with no migration path defined.

---

## 1. Critical Issues (Must Fix Before Implementation)

### C1. `ToolSettings.Enabled` is `interface{}` -- Type Safety Hole

**Section**: Data Model, line 254-257

```go
type ToolSettings struct {
    Enabled  interface{} `json:"enabled,omitempty"`  // "*" for all, or []string of tool names
    Disabled []string    `json:"disabled,omitempty"`
}
```

`interface{}` in a data model that gets serialized, deserialized, deep-merged, compared, and persisted to disk is a defect generator. After JSON round-tripping, `"*"` becomes `string` but `["observe"]` becomes `[]interface{}` (not `[]string`). Every consumer must type-switch on two branches, and any missed branch is a silent misconfiguration that disables security tools.

**Fix**: Use a typed union:

```go
type ToolSettings struct {
    EnableAll bool     `json:"enable_all,omitempty"`
    Enabled   []string `json:"enabled,omitempty"`
    Disabled  []string `json:"disabled,omitempty"`
}
```

If `EnableAll` is true, all tools are enabled. If false, only `Enabled` tools are available. `Disabled` always takes precedence. No ambiguity, no type switches.

### C2. Profile Activation Does Not Define Buffer Resize Atomicity

**Section**: How It Works, Performance Constraints

The spec says "Profile changes take effect immediately -- no server restart required." Buffer limits are part of `ProfileSettings`. The existing codebase uses hardcoded constants (`maxWSEvents=500`, `maxNetworkBodies=100`, `maxEnhancedActions=50` in `types.go:329-331`) with pre-allocated slices (`make([]WebSocketEvent, 0, maxWSEvents)` in `types.go:470`).

When a profile activates with `console_entries: 100` (down from 1000), what happens to the 900 existing entries? The spec is silent on:

1. **Eviction policy during resize**: Truncate oldest? Truncate newest? Block until TTL expires?
2. **Atomicity**: The server has separate mutexes for `Server.mu` (console entries) and `Capture.mu` (WS/network/actions). A profile activation that changes both buffer limits requires coordinated writes under both locks. The spec's single `ProfileManager.mu` does not address this.
3. **Memory pressure**: Shrinking `network_bodies` from 100 to 0 (paranoid profile) while 100 bodies are buffered means those entries must be freed. If a concurrent `toolObserve` call holds `Capture.mu.RLock()`, the resize blocks and the 10ms activation SLO is violated.

**Fix**: Define explicit resize semantics:
- On shrink: evict oldest entries immediately, under the owning buffer's lock.
- On expansion: capacity increases lazily (no pre-allocation; append naturally grows the slice).
- Activation sequence: resolve profile, then apply each buffer's new limit under its own lock, sequentially. Document that the 10ms SLO excludes the eviction I/O (which may trigger `saveEntries` for console logs).

### C3. Dual Configuration Systems With No Migration Path

The spec introduces a parallel configuration system (`ProfileSettings`) that duplicates and conflicts with the existing `CaptureOverrides` and hardcoded constants in `types.go`, with no migration path defined. This creates a complex, error-prone system that is difficult to maintain and debug.

**Fix**: Remove the `ProfileSettings` system and use only `CaptureOverrides` and hardcoded constants in `types.go`. This will simplify the system and reduce the risk of errors.

---
