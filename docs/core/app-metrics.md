# App Metrics

All telemetry is anonymous, local-first, and opt-out via `KABOOM_TELEMETRY=off`.

## Architecture

The Go server is the **sole analytics sender**. The extension sends no independent analytics.

```
Extension UI action ──> sync payload (features_used) ──┐
                                                        ├──> Go UsageCounter + Session Manager ──> usage_summary / lifecycle beacon
MCP tool call ──> server-side Increment("tool:mode") ──┘
```

## Install ID

- **Source of truth:** Go server, persisted at `~/.kaboom/install_id`
- **Format:** 12-character random hex string (6 bytes via `crypto/rand`)
- **Propagation:** Server sends `install_id` in every `/sync` response. Extension persists it to `chrome.storage.local` for MV3 service worker eviction survival.
- **Opt-out fallback:** If `KABOOM_TELEMETRY=off`, beacons are suppressed but the ID still exists.

## Session ID

- **Source of truth:** Go server only. The extension does not mint or persist session IDs.
- **Format:** 16-character random hex string (8 bytes via `crypto/rand`)
- **Propagation:** Server includes `sid` in every telemetry beacon
- **Persistence:** In-memory only. A daemon restart always creates a new session

### Session Lifecycle

- A new session starts on the first tracked activity when no active session exists
- A session stays active while telemetry-bearing activity continues
- The server rotates the session after **30 minutes of inactivity**
- Any accepted MCP tool increment or extension feature increment refreshes session activity
- Lifecycle beacons also carry the current `sid`. If no active session exists, the server mints one before sending

## Usage Summary Beacon

**Endpoint:** `https://t.gokaboom.dev/v1/event`
**Event:** `usage_summary`
**Interval:** Every 5 minutes (skipped if idle)

### Payload

```json
{
  "event": "usage_summary",
  "v": "0.8.1",
  "os": "darwin-arm64",
  "iid": "a1b2c3d4e5f6",
  "sid": "8f3c1e4b7d92a6ff",
  "window_m": 5,
  "props": {
    "observe:errors": 5,
    "observe:logs": 3,
    "interact:click": 2,
    "ext:screenshot": 1,
    "ext:video": 1
  }
}
```

### Envelope Contract

All telemetry beacons use this top-level envelope:

| Field | Type | Required | Notes |
|-------|------|----------|-------|
| `event` | string | yes | Event name, e.g. `usage_summary`, `daemon_start` |
| `v` | string | yes | App version |
| `os` | string | yes | Platform string, e.g. `darwin-arm64` |
| `iid` | string | yes | Install ID |
| `sid` | string | yes | Session ID |
| `window_m` | integer | usage summaries only | Length of the summarized activity window in minutes |
| `props` | object<string, integer> | usage summaries only | Per-tool or per-feature counters for the window |

`props` values are integers, not strings.

### Key Format

| Prefix | Source | Example |
|--------|--------|---------|
| `tool:mode` | MCP tool call (server-side) | `observe:errors`, `interact:click`, `generate:test` |
| `ext:feature` | Extension UI action (via sync) | `ext:screenshot`, `ext:annotations`, `ext:video`, `ext:dom_action` |

### MCP Tool Tracking

All 5 tools dispatch on the `what` parameter. The server increments `tool:what` on every call:

- `observe:errors`, `observe:logs`, `observe:screenshot`, `observe:network_waterfall`, `observe:actions`, `observe:page`, ...
- `interact:click`, `interact:type`, `interact:navigate`, `interact:execute_js`, ...
- `generate:test`, `generate:reproduction`, `generate:har`, ...
- `configure:noise_rule`, `configure:streaming`, `configure:health`, ...
- `analyze:accessibility`, `analyze:performance`, ...

If `what` is missing, the key is `tool:unknown`.

### Extension UI Feature Tracking

Only actions triggered by the user in the extension UI (not via MCP):

| Feature | Trigger |
|---------|---------|
| `screenshot` | Context menu, popup button |
| `annotations` | Context menu, keyboard shortcut, popup toggle |
| `video` | Context menu, keyboard shortcut, popup record button |
| `dom_action` | Context menu DOM actions |

These are sent as `features_used` in the `/sync` payload. The server validates against an allowlist (only these 4 keys are accepted) and increments the usage counter with the `ext:` prefix.

## Lifecycle Beacons

Separate from usage summaries, the server sends one-shot beacons for lifecycle events:

| Event | When |
|-------|------|
| `daemon_start` | Server starts |
| `extension_connect` | Extension connects or reconnects |
| `extension_version_mismatch` | Extension/server major.minor differs |

Lifecycle beacons use the same top-level envelope (`event`, `v`, `os`, `iid`, `sid`) but omit `window_m` and `props` unless a specific lifecycle event later needs structured counters.

## Opt-Out

Set `KABOOM_TELEMETRY=off` environment variable. All beacons (usage, lifecycle, error) are suppressed. The extension storage key `kaboom_telemetry_off` disables extension-side telemetry beacons.

## Code Locations

| Component | File |
|-----------|------|
| Usage counter | `internal/telemetry/usage_counter.go` |
| Usage beacon loop | `internal/telemetry/usage_beacon.go` |
| Beacon sender | `internal/telemetry/beacon.go` |
| Install ID | `internal/telemetry/install_id.go` |
| Features callback wiring | `cmd/browser-agent/tools_core_constructor.go` |
| Features allowlist + sync | `internal/capture/sync.go` |
| UI usage tracker (ext) | `src/background/ui-usage-tracker.ts` |
| Install ID persistence (ext) | `src/background/sync-client.ts` |
