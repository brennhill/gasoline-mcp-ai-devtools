---
feature: cpu-network-emulation
status: proposed
version: null
tool: configure
mode: emulate
authors: []
created: 2026-01-28
updated: 2026-01-28
---

# CPU/Network Emulation

> Throttle CPU and network conditions to simulate low-end devices, slow connections, and offline scenarios -- enabling AI agents to test performance degradation and verify resilience without leaving the browser.

## Problem

AI coding agents can observe performance metrics (FCP, LCP, CLS) and network activity through Gasoline's existing `observe` tool, but they cannot *simulate* constrained environments. When a developer says "test how this performs on a low-end mobile device" or "verify the app works offline," the agent has no way to reproduce those conditions. The agent can see what happened but cannot change the environment to provoke what *would* happen.

Chrome DevTools MCP (the primary competitor) offers `emulate_cpu` and `emulate_network` tools that let agents throttle CPU (1-6x slowdown) and apply network profiles (Slow 3G, Fast 3G, Offline). This is a gap in Gasoline's capabilities for comprehensive performance testing workflows.

## Solution

Add an `emulate` action to the `configure` tool that enables CPU throttling and network condition emulation. The extension communicates with the browser's Chrome DevTools Protocol (CDP) via the `chrome.debugger` API to apply throttling settings to the active tab. Emulation is opt-in, requires explicit user consent (similar to AI Web Pilot), and can be reset at any time.

The workflow:
1. AI agent calls `configure({action: "emulate", cpu_rate: 4, network: "Slow 3G"})` to apply conditions
2. Extension attaches the debugger to the target tab and sends CDP commands
3. Agent uses existing `observe` tools to measure impact (vitals, network waterfall, errors)
4. Agent calls `configure({action: "emulate", reset: true})` to restore normal conditions

## User Stories

- As an AI coding agent, I want to throttle CPU speed so that I can measure how the application performs on a low-end device and identify long tasks that only appear under constraint.
- As an AI coding agent, I want to simulate slow network conditions so that I can verify that loading states, timeouts, and fallbacks work correctly.
- As an AI coding agent, I want to simulate offline mode so that I can verify service worker caching, offline UI, and graceful degradation.
- As a developer using Gasoline, I want the AI to automatically test my app under various device conditions so that I can catch performance regressions before deploying.
- As an AI coding agent, I want to reset emulation to normal conditions so that subsequent observations reflect the true environment.

## MCP Interface

**Tool:** `configure`
**Mode/Action:** `emulate`

### Request -- Apply Emulation

```json
{
  "tool": "configure",
  "arguments": {
    "action": "emulate",
    "cpu_rate": 4,
    "network": "Slow 3G"
  }
}
```

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `action` | string | yes | Must be `"emulate"` |
| `cpu_rate` | number | no | CPU throttling factor: 1 (no throttle) to 6 (6x slowdown). Default: no change. |
| `network` | string | no | Network profile name. One of: `"Slow 3G"`, `"Fast 3G"`, `"4G"`, `"Offline"`, `"No Throttle"`. Default: no change. |
| `custom_network` | object | no | Custom network profile (overrides `network` if both provided). |
| `custom_network.download` | number | no | Download throughput in bytes/sec |
| `custom_network.upload` | number | no | Upload throughput in bytes/sec |
| `custom_network.latency` | number | no | Round-trip latency in ms |
| `reset` | boolean | no | If true, remove all emulation and detach debugger. Ignores other params. |
| `tab_id` | number | no | Target tab (applies to multi-tab tracking). Default: active tracked tab. |

### Request -- Reset Emulation

```json
{
  "tool": "configure",
  "arguments": {
    "action": "emulate",
    "reset": true
  }
}
```

### Request -- Custom Network Profile

```json
{
  "tool": "configure",
  "arguments": {
    "action": "emulate",
    "cpu_rate": 2,
    "custom_network": {
      "download": 375000,
      "upload": 75000,
      "latency": 200
    }
  }
}
```

### Response -- Emulation Applied

```json
{
  "status": "emulation_active",
  "cpu": {
    "rate": 4,
    "description": "4x CPU slowdown"
  },
  "network": {
    "profile": "Slow 3G",
    "download_kbps": 400,
    "upload_kbps": 50,
    "latency_ms": 2000
  },
  "tab_id": 42,
  "warning": "chrome.debugger attached -- 'controlled by automated software' banner is visible"
}
```

### Response -- Emulation Reset

```json
{
  "status": "emulation_cleared",
  "cpu": "normal",
  "network": "normal",
  "tab_id": 42
}
```

### Response -- Permission Denied

```json
{
  "error": "emulation_disabled",
  "message": "CPU/Network emulation requires user opt-in. The user must enable 'Allow Emulation' in the Gasoline extension options.",
  "hint": "Ask the user to enable emulation in the extension popup or options page."
}
```

## Requirements

| # | Requirement | Priority |
|---|-------------|----------|
| R1 | CPU throttling via CDP `Emulation.setCPUThrottlingRate` with rate 1-6 | must |
| R2 | Network throttling via CDP `Network.emulateNetworkConditions` with preset profiles | must |
| R3 | Custom network profiles with user-specified download/upload/latency values | should |
| R4 | Reset command to clear all emulation and detach debugger | must |
| R5 | Opt-in toggle in extension (disabled by default), similar to AI Web Pilot | must |
| R6 | Return current emulation state in response (what is active) | must |
| R7 | Auto-reset emulation when tab is closed or navigated away | should |
| R8 | Include emulation state in `configure({action: "health"})` response | should |
| R9 | Offline mode simulation (download=0, upload=0, offline=true) | must |
| R10 | Preset network profiles matching Chrome DevTools defaults | must |
| R11 | Multi-tab support via `tab_id` parameter | could |

## Non-Goals

- This feature does NOT provide device viewport emulation (screen size, device pixel ratio, user agent). Those are separate concerns that may warrant their own feature.
- This feature does NOT provide memory pressure simulation. Chrome's CDP does not offer a stable API for this.
- This feature does NOT provide automated performance benchmarking workflows. It provides the primitives (throttle + observe); the AI agent composes the workflow.
- Out of scope: network request interception or modification. That is a different capability (mocking/stubbing) unrelated to emulation.
- Out of scope: persistent emulation profiles saved across sessions. Emulation is ephemeral and resets on disconnect.

## Network Profile Presets

| Profile | Download (KB/s) | Upload (KB/s) | Latency (ms) | Offline |
|---------|----------------|---------------|---------------|---------|
| Slow 3G | 400 | 50 | 2000 | false |
| Fast 3G | 1500 | 750 | 560 | false |
| 4G | 4000 | 3000 | 170 | false |
| Offline | 0 | 0 | 0 | true |
| No Throttle | -1 | -1 | 0 | false |

These match the standard profiles used in Chrome DevTools and the Chrome DevTools MCP server.

## Performance SLOs

| Metric | Target |
|--------|--------|
| Emulation apply response time | < 500ms (includes debugger attach on first call) |
| Emulation reset response time | < 200ms |
| Memory impact on server | < 1KB (only stores current emulation state) |
| Memory impact on extension | Negligible (chrome.debugger is browser-level) |
| Browsing performance impact when emulation is OFF | Zero (no debugger attached) |

## Security Considerations

- **chrome.debugger is privileged.** Attaching the debugger grants CDP access to the tab, which is a superset of what `execute_js` provides. The same opt-in consent model used for AI Web Pilot must gate this feature.
- **Debugger banner is visible.** Chrome displays "Chrome is being controlled by automated test software" when debugger is attached. This is unavoidable and must be communicated to the user/agent in the response.
- **No new data exfiltration risk.** Emulation only changes CPU/network conditions; it does not read or capture any additional data beyond what Gasoline already captures.
- **Localhost-only.** All communication remains on localhost. The debugger attachment is local to the browser.
- **Auto-cleanup.** Emulation must be cleared when the extension disconnects from the server, when the tab is closed, or when a new MCP session begins. Stale throttling left on a tab is a usability hazard.
- **Opt-in defaults to OFF.** The emulation toggle in the extension options defaults to disabled. Users must explicitly enable it.

## Architecture

### Data Flow

```
AI Agent
  |
  | MCP: configure({action: "emulate", cpu_rate: 4, network: "Slow 3G"})
  v
Go Server
  |
  | Creates pending query: {type: "emulate", params: {cpu_rate: 4, network: "Slow 3G"}}
  v
Extension (background.js) polls /pending-queries
  |
  | Checks: is emulation enabled? (opt-in toggle)
  | If yes:
  |   chrome.debugger.attach({tabId}, '1.3')
  |   chrome.debugger.sendCommand({tabId}, 'Emulation.setCPUThrottlingRate', {rate: 4})
  |   chrome.debugger.sendCommand({tabId}, 'Network.emulateNetworkConditions', {...})
  | Posts result to /execute-result
  v
Go Server returns result to AI agent on next poll
```

### Extension Changes

1. **manifest.json**: Add `"debugger"` to `permissions` array.
2. **background.js**: Add emulation command handler in the pending-queries dispatch.
3. **options.html/js**: Add "Allow Emulation" toggle (default: off), stored in `chrome.storage.local`.
4. **State tracking**: Store current emulation state per tab to support reset and status queries.

### Server Changes

1. **tools.go**: Add `"emulate"` to the `configure` action enum. Add parameter schema for `cpu_rate`, `network`, `custom_network`, `reset`.
2. **emulation.go** (new file): Handler for `configure(action: "emulate")`. Validates parameters, creates pending query, returns result via async command pipeline.
3. **types.go**: Add emulation state struct to track active emulation per tab.

### CDP Commands Used

| CDP Domain | Method | Purpose |
|------------|--------|---------|
| `Emulation` | `setCPUThrottlingRate` | Apply CPU throttle (rate: 1-6) |
| `Network` | `emulateNetworkConditions` | Apply network profile (offline, latency, download, upload) |
| `Network` | `enable` | Required before `emulateNetworkConditions` |

## Edge Cases

- **Emulate called when emulation is disabled in extension**: Return `{error: "emulation_disabled"}` with a hint to enable it. Same pattern as AI Web Pilot disabled.
- **Emulate called when debugger is already attached (by another tool or manually)**: Reuse existing debugger session. Do not double-attach. Track attachment ownership.
- **Tab closed while emulation is active**: Debugger auto-detaches. Server should clear emulation state for that tab on the next status check.
- **Extension disconnects from server while emulation is active**: Extension should reset all emulation on disconnect (detach debugger, clear throttling).
- **Invalid cpu_rate (0, negative, > 6)**: Return structured error with valid range hint.
- **Both `network` and `custom_network` provided**: `custom_network` takes precedence. Document this in the response.
- **Reset called when no emulation is active**: Return success with `"status": "emulation_cleared"`. This is idempotent.
- **Multiple tabs with different emulation settings**: Each tab has independent emulation state. Debugger attaches per-tab.
- **User manually detaches debugger via chrome://inspect**: Extension detects `chrome.debugger.onDetach` event and clears emulation state for that tab.

## Dependencies

- **Depends on:** Async command pipeline (v6.0.0) for MCP-to-extension communication
- **Depends on:** Tab targeting system for multi-tab support
- **Depends on:** `chrome.debugger` permission (new permission in manifest.json)
- **Depended on by:** None currently. This is a standalone capability.
- **Complements:** `observe({what: "vitals"})`, `observe({what: "performance"})`, `observe({what: "network_waterfall"})` -- agents will use these to measure the impact of emulation.

## Assumptions

- A1: The extension is connected to the Gasoline server and tracking at least one tab.
- A2: The user has explicitly enabled the "Allow Emulation" toggle in extension options.
- A3: Chrome supports the `chrome.debugger` API for the target tab (not a chrome:// URL, not a DevTools page).
- A4: The `Emulation.setCPUThrottlingRate` and `Network.emulateNetworkConditions` CDP methods are stable and available in Chrome 120+.
- A5: The debugger "controlled by automated software" banner is an acceptable UX trade-off for development/testing scenarios.

## Open Items

| # | Item | Status | Notes |
|---|------|--------|-------|
| OI-1 | Does `chrome.debugger` require explicit user gesture to attach in MV3? | open | MV3 tightened permissions. Need to verify that `chrome.debugger.attach` works from a service worker without a user gesture. If it requires a gesture, the extension popup may need to trigger attachment. |
| OI-2 | Can we avoid the "controlled by automated software" banner? | open | This banner is a known pain point (see reproduction-enhancements review). For emulation it may be acceptable since it is explicitly opt-in and development-focused. But it needs user testing. |
| OI-3 | Should emulation state persist across page reloads within the same tab? | open | CDP debugger stays attached across navigations, so emulation settings should persist. But should we re-send CDP commands after navigation to guarantee state? Need to test behavior. |
| OI-4 | Alternative: chrome.networking API instead of CDP for network throttling? | open | The `chrome.networking` API (if available in MV3) might provide network throttling without requiring debugger attachment. This would avoid the banner entirely. Needs investigation. |
| OI-5 | Should the `observe({what: "vitals"})` response include current emulation state? | open | When an agent observes performance metrics, knowing that CPU is throttled 4x is critical context. Consider including emulation state as metadata in vitals/performance responses. |
| OI-6 | Interaction with existing AI Web Pilot debugger usage | open | If AI Web Pilot or screenshot capture also needs `chrome.debugger` in the future, we need a shared debugger session manager to avoid conflicts. Should this feature introduce that abstraction now? |

## Estimated Effort

| Component | Lines | Notes |
|-----------|-------|-------|
| Go server (emulation.go + tools.go changes) | ~150 | Handler, validation, preset profiles, pending query creation |
| Extension JS (background.js) | ~50 | CDP command dispatch, state tracking, opt-in check |
| Extension UI (options.html/js) | ~20 | Toggle for "Allow Emulation" |
| Tests (Go) | ~100 | Parameter validation, preset profiles, state management, error cases |
| Tests (Extension) | ~30 | CDP command formatting, toggle behavior |
| **Total** | **~350** | |
