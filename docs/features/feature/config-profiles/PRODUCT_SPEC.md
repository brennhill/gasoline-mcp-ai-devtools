---
feature: config-profiles
status: proposed
version: null
tool: configure
mode: profile
authors: []
created: 2026-01-28
updated: 2026-01-28
---

# Configuration Profiles

> Named capture configuration presets that AI agents and developers can save, load, and switch between with a single command.

## Problem

Gasoline's AI capture control feature (`configure(action: "capture")`) lets agents adjust five capture settings: `log_level`, `ws_mode`, `network_bodies`, `screenshot_on_error`, and `action_replay`. When an agent begins a debugging session, it often needs to change several settings at once -- for example, turning on all log levels, enabling WebSocket message payloads, and enabling network body capture. At the end of the session, it must remember what to reset.

This creates two problems:

1. **Repetitive multi-setting changes.** Every time an agent starts a "deep debug" investigation, it must issue the same bundle of setting changes. There is no way to express "apply the debug configuration" as a single operation.

2. **No memory across sessions.** Capture overrides are session-scoped (cleared on server restart). If a developer has a preferred debugging configuration, the agent must re-derive or re-apply it every session. There is no way to persist a named bundle of settings.

Configuration profiles solve both problems by letting agents save named presets and activate them with a single call.

## Solution

A **configuration profile** is a named map of capture settings. Profiles are managed through a new `profile` action on the existing `configure` tool. The server provides a small set of built-in profiles for common workflows. AI agents and developers can also create custom profiles that are persisted to the project's `.gasoline/profiles/` directory.

When a profile is activated, its settings are applied as capture overrides (the same mechanism used by `configure(action: "capture")`). This means profiles compose naturally with the existing capture control system -- a profile is simply a saved set of capture overrides.

## User Stories

- As an AI coding agent, I want to activate a "debug" profile so that I can enable verbose logging, full network body capture, and WebSocket message payloads in a single call instead of setting each one individually.
- As an AI coding agent, I want to save the current capture settings as a named profile so that I can quickly return to this configuration in future sessions.
- As an AI coding agent, I want to list available profiles so that I can choose the right one for the current task.
- As a developer using Gasoline, I want built-in profiles for common workflows (debugging, performance, minimal) so that I do not have to configure them manually.
- As a developer using Gasoline, I want custom profiles persisted to the project directory so that they survive server restarts and are available to any agent that connects.

## MCP Interface

**Tool:** `configure`
**Action:** `profile`

The `profile` action uses a `profile_action` sub-action parameter to select the operation.

### Save a Profile

Saves the provided settings (or the current active overrides) as a named profile.

**Request:**
```json
{
  "tool": "configure",
  "arguments": {
    "action": "profile",
    "profile_action": "save",
    "name": "debug",
    "description": "Full verbose capture for deep debugging",
    "settings": {
      "log_level": "all",
      "ws_mode": "messages",
      "network_bodies": "true",
      "screenshot_on_error": "true",
      "action_replay": "true"
    }
  }
}
```

**Response:**
```json
{
  "status": "saved",
  "name": "debug",
  "description": "Full verbose capture for deep debugging",
  "settings": {
    "log_level": "all",
    "ws_mode": "messages",
    "network_bodies": "true",
    "screenshot_on_error": "true",
    "action_replay": "true"
  },
  "builtin": false,
  "persisted": true
}
```

If `settings` is omitted, the server saves a snapshot of the currently active capture overrides. If no overrides are active, the server returns an error.

### Load (Activate) a Profile

Applies the named profile's settings as capture overrides. This replaces all current overrides with the profile's settings.

**Request:**
```json
{
  "tool": "configure",
  "arguments": {
    "action": "profile",
    "profile_action": "load",
    "name": "debug"
  }
}
```

**Response:**
```json
{
  "status": "loaded",
  "name": "debug",
  "description": "Full verbose capture for deep debugging",
  "settings_applied": {
    "log_level": "all",
    "ws_mode": "messages",
    "network_bodies": "true",
    "screenshot_on_error": "true",
    "action_replay": "true"
  },
  "previous_overrides": {
    "log_level": "error"
  }
}
```

The response includes `previous_overrides` so the agent knows what changed. Loading a profile counts as a single rate-limit event (same as `configure(action: "capture")` with multiple settings).

### List Profiles

Returns all available profiles (built-in and custom).

**Request:**
```json
{
  "tool": "configure",
  "arguments": {
    "action": "profile",
    "profile_action": "list"
  }
}
```

**Response:**
```json
{
  "profiles": [
    {
      "name": "debug",
      "description": "Full verbose capture for deep debugging",
      "builtin": true,
      "settings": {
        "log_level": "all",
        "ws_mode": "messages",
        "network_bodies": "true",
        "screenshot_on_error": "true",
        "action_replay": "true"
      }
    },
    {
      "name": "performance",
      "description": "Minimal capture for performance profiling",
      "builtin": true,
      "settings": {
        "log_level": "error",
        "ws_mode": "off",
        "network_bodies": "false",
        "screenshot_on_error": "false",
        "action_replay": "false"
      }
    },
    {
      "name": "minimal",
      "description": "Errors only - lowest overhead",
      "builtin": true,
      "settings": {
        "log_level": "error",
        "ws_mode": "off",
        "network_bodies": "false",
        "screenshot_on_error": "false",
        "action_replay": "false"
      }
    },
    {
      "name": "my-team-debug",
      "description": "Custom profile saved by agent",
      "builtin": false,
      "settings": {
        "log_level": "warn",
        "network_bodies": "true"
      }
    }
  ],
  "active": "debug"
}
```

The `active` field indicates which profile was most recently loaded (or null if none has been loaded or if overrides were manually set after loading).

### Get a Profile

Returns the full definition of a single profile.

**Request:**
```json
{
  "tool": "configure",
  "arguments": {
    "action": "profile",
    "profile_action": "get",
    "name": "debug"
  }
}
```

**Response:**
```json
{
  "name": "debug",
  "description": "Full verbose capture for deep debugging",
  "builtin": true,
  "settings": {
    "log_level": "all",
    "ws_mode": "messages",
    "network_bodies": "true",
    "screenshot_on_error": "true",
    "action_replay": "true"
  }
}
```

### Delete a Profile

Removes a custom profile. Built-in profiles cannot be deleted.

**Request:**
```json
{
  "tool": "configure",
  "arguments": {
    "action": "profile",
    "profile_action": "delete",
    "name": "my-team-debug"
  }
}
```

**Response:**
```json
{
  "status": "deleted",
  "name": "my-team-debug"
}
```

If the deleted profile is currently active, the active overrides remain in effect but the `active` profile name is cleared.

## Built-in Profiles

The server ships with three built-in profiles. These are always available and cannot be modified or deleted.

### `debug`

Full verbose capture. Use when investigating bugs, WebSocket issues, or unexpected behavior.

| Setting | Value |
|---------|-------|
| `log_level` | `all` |
| `ws_mode` | `messages` |
| `network_bodies` | `true` |
| `screenshot_on_error` | `true` |
| `action_replay` | `true` |

### `performance`

Minimal capture to reduce extension overhead during performance profiling. Disables everything except error logging.

| Setting | Value |
|---------|-------|
| `log_level` | `error` |
| `ws_mode` | `off` |
| `network_bodies` | `false` |
| `screenshot_on_error` | `false` |
| `action_replay` | `false` |

### `minimal`

Errors only. Lowest possible overhead. Use when Gasoline should be running but should not interfere with the page.

| Setting | Value |
|---------|-------|
| `log_level` | `error` |
| `ws_mode` | `off` |
| `network_bodies` | `false` |
| `screenshot_on_error` | `false` |
| `action_replay` | `false` |

Note: `performance` and `minimal` have identical settings in the initial release. They exist as separate profiles because their intended use cases differ and their settings may diverge in the future (e.g., `performance` may gain vitals-specific capture settings).

## Profile Settings Scope

A profile contains a subset (or all) of the five capture settings defined in `CaptureOverrides`:

| Setting | Type | Allowed Values |
|---------|------|---------------|
| `log_level` | string | `error`, `warn`, `all` |
| `ws_mode` | string | `off`, `lifecycle`, `messages` |
| `network_bodies` | string | `true`, `false` |
| `screenshot_on_error` | string | `true`, `false` |
| `action_replay` | string | `true`, `false` |

A profile does not need to specify all five settings. When a partial profile is loaded, only the specified settings are applied as overrides. Settings not mentioned in the profile are left at their current value (either the existing override or the default).

## Interaction with Existing Capture Control

Configuration profiles build on top of the existing `configure(action: "capture")` mechanism. The relationship is:

1. **Profiles are stored bundles of capture settings.** Loading a profile is equivalent to calling `configure(action: "capture", settings: {...})` with the profile's settings.

2. **Manual overrides after profile load.** After loading a profile, the agent can still call `configure(action: "capture")` to change individual settings. This does not modify the stored profile -- it only changes the active overrides.

3. **Active profile tracking.** The server tracks which profile was most recently loaded. If the agent manually changes a setting after loading a profile, the active profile name is cleared (since the overrides no longer match the profile).

4. **Reset clears profile.** Calling `configure(action: "capture", settings: "reset")` clears all overrides and clears the active profile name.

5. **Rate limiting.** Loading a profile counts as one capture settings change for rate-limiting purposes (1 change per second).

6. **Audit logging.** Loading a profile generates a single audit event of type `profile_load` containing the profile name and all settings applied. Individual setting changes within the profile are not logged separately.

## Requirements

| # | Requirement | Priority |
|---|-------------|----------|
| R1 | The server provides built-in profiles: `debug`, `performance`, `minimal` | must |
| R2 | Agents can save custom profiles with a name, optional description, and settings | must |
| R3 | Agents can load a profile to apply its settings as capture overrides | must |
| R4 | Agents can list all available profiles (built-in and custom) | must |
| R5 | Agents can delete custom profiles | must |
| R6 | Built-in profiles cannot be modified or deleted | must |
| R7 | Custom profiles are persisted to `.gasoline/profiles/` as JSON files | must |
| R8 | Custom profiles survive server restarts | must |
| R9 | Profile names are validated: alphanumeric, hyphens, underscores, 1-50 characters | must |
| R10 | Loading a profile respects the existing rate limit (1 change per second) | must |
| R11 | Loading a profile generates an audit log entry | must |
| R12 | Saving a profile without explicit settings captures the current active overrides | should |
| R13 | Agents can get the full definition of a single profile | should |
| R14 | The list response indicates which profile is currently active | should |
| R15 | After loading a profile, manual capture changes clear the active profile name | should |
| R16 | Profile settings are validated against the same rules as direct capture changes | must |

## Non-Goals

- **Profile inheritance.** Profiles do not extend other profiles. Each profile is a flat map of settings. Inheritance adds complexity (circular dependency detection, merge semantics, resolution chains) for minimal practical benefit when there are only five settings. If needed later, it can be added without breaking the existing interface.

- **Buffer size or memory configuration.** Profiles only control the five capture settings in `CaptureOverrides`. Buffer sizes, TTLs, and memory limits are server-level configuration that should remain as CLI flags and environment variables. The previous tech spec's attempt to include these created a parallel configuration system that conflicted with the existing architecture.

- **Tool gating.** Profiles do not enable or disable MCP tools. The four-tool constraint is a hard architectural boundary, and tool availability is not a per-session concern.

- **Redaction configuration.** Redaction patterns are security-sensitive settings that should not be changeable via a single MCP call. They remain managed through the security configuration system.

- **Enterprise profile distribution.** Features like profile locking (`GASOLINE_PROFILE_LOCKED`), profile URLs, or CI/CD import are out of scope for the initial release. The persistence model (JSON files in `.gasoline/profiles/`) naturally supports manual distribution (copy the file).

- **Extension popup integration.** The extension popup does not gain profile selection UI in this release. Profile management is an AI-agent-facing feature.

## Performance SLOs

| Metric | Target |
|--------|--------|
| Profile load (apply settings) | < 1ms |
| Profile save (write to disk) | < 5ms |
| Profile list (read all profiles) | < 10ms |
| Profile delete | < 5ms |
| Memory per profile (in-memory) | < 1KB |
| Maximum custom profiles | 50 |
| Maximum profile file size | 4KB |

## Security Considerations

- **Profile names are file names.** Since profiles are stored as `<name>.json` in `.gasoline/profiles/`, profile names must be validated to prevent path traversal. The same `validateStoreInput` function used by the session store should be reused.

- **Settings values are validated.** Every setting in a profile must pass the same validation as a direct `configure(action: "capture")` call. Invalid values cause the save to fail.

- **No sensitive data in profiles.** Profiles contain only capture setting key-value pairs. No credentials, tokens, or user data.

- **Audit trail.** Profile creation, loading, and deletion are logged to the audit log. This provides visibility into who changed capture behavior and when.

- **Built-in profiles are immutable.** An agent cannot save a custom profile with the same name as a built-in profile. This prevents an agent from overriding the `minimal` profile with verbose settings.

## Edge Cases

- **Load non-existent profile.** The server returns an error: "Profile not found: <name>. Use configure({action:'profile', profile_action:'list'}) to see available profiles."

- **Save profile with empty settings and no active overrides.** The server returns an error: "No settings provided and no active capture overrides to snapshot."

- **Save profile with name of built-in profile.** The server returns an error: "Cannot overwrite built-in profile: <name>."

- **Delete built-in profile.** The server returns an error: "Cannot delete built-in profile: <name>."

- **Delete non-existent profile.** The server returns an error: "Profile not found: <name>."

- **Profile name with path traversal characters.** Rejected by name validation: "Profile name contains path traversal sequence" or "Profile name contains path separator."

- **Corrupted profile file on disk.** Skipped during profile loading at startup. The server logs a warning and continues. The corrupted profile does not appear in the list.

- **`.gasoline/profiles/` directory does not exist.** Created automatically on the first profile save.

- **Session store not initialized.** Profile operations that require disk persistence (save, delete) return an error if the session store is not initialized (e.g., server started without a project directory). Built-in profiles and profile loading (from in-memory) still work.

- **Concurrent profile operations.** All profile operations are mutex-protected. Concurrent saves or loads are serialized.

- **Profile loaded, then server restarts.** The active profile name is not persisted. After restart, no profile is active and all capture overrides are cleared (existing session-scope behavior). The custom profiles themselves remain on disk and can be reloaded.

- **Rate-limited profile load.** If the agent loaded a profile less than 1 second ago and tries to load another, the second load is rejected with the standard rate limit error.

- **Profile with partial settings loaded.** Only the specified settings become overrides. Unspecified settings retain their current state (default or previously-set override).

## Dependencies

- **Depends on:** AI Capture Control (shipped) -- profiles bundle capture settings managed by `CaptureOverrides`.
- **Depends on:** Persistent Memory (shipped) -- profiles use the `.gasoline/` project directory and follow the same storage patterns.
- **Depended on by:** None currently.

## Assumptions

- A1: The session store is initialized (server was started with a valid working directory) for custom profile persistence to work. Built-in profiles work regardless.
- A2: The five capture settings (`log_level`, `ws_mode`, `network_bodies`, `screenshot_on_error`, `action_replay`) and their valid values remain stable.
- A3: The existing rate limit (1 change per second) applies to profile loads just as it does to direct capture setting changes.

## Open Items

| # | Item | Status | Notes |
|---|------|--------|-------|
| OI-1 | Should loading a profile persist the active profile name to `.gasoline/meta.json` so it survives restarts? | open | Current design says no (session-scoped like all capture overrides). An alternative is to persist the active profile name so the server auto-loads it on restart, making profiles a lightweight "mode switch" that persists. This would diverge from the session-scoped model of capture overrides. |
| OI-2 | Should `configure(action: "capture")` accept a `profile` shorthand parameter? | open | For example: `configure(action: "capture", profile: "debug")` as sugar for `configure(action: "profile", profile_action: "load", name: "debug")`. This would be more discoverable but adds a second way to do the same thing. |
| OI-3 | Should the `performance` built-in profile differ from `minimal`? | open | In the current five-setting model they are identical. If capture settings expand (e.g., a `vitals_capture` setting), they would diverge. Shipping them as separate profiles now establishes the semantic distinction early. Alternatively, ship only `debug` and `minimal` to avoid confusion. |
| OI-4 | Should profiles support a `save_current` shorthand that snapshots all current overrides plus defaults for unset settings? | open | The current design allows saving with no explicit settings, which snapshots only the active overrides. An agent wanting a "complete" profile would need to explicitly list all five settings. A `save_current` mode could snapshot the fully-resolved state (overrides merged with defaults). |
| OI-5 | Maximum number of custom profiles: 50 is generous. Should it be lower (e.g., 20)? | open | With profiles stored as individual files, 50 is technically fine. But a lower limit prevents accidental profile sprawl from automated agents. |
