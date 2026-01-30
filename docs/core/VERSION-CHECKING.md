# Version Checking & Notifications

Gasoline includes automatic version checking to notify users when a newer server version is available. This document describes how the system works and how to use it.

## Overview

The version checking system has two components:

1. **Extension → Server**: Extension sends its version with every request via `X-Gasoline-Extension-Version` header
2. **Server → Extension**: Extension periodically checks server version and displays a notification badge

Both run automatically with no configuration needed.

## User Experience

### When a New Version is Available

1. Extension periodically checks `/health` endpoint for server version (every 30 minutes)
2. If server version is newer than extension version:
   - A **blue "⬆" badge** appears on the extension icon
   - Extension tooltip shows: `"Gasoline: New version available (v5.2.6)"`
3. User can click extension icon to see version info or download new version

### Version Format

Versions follow semantic versioning: `X.Y.Z` (e.g., `5.2.5`)

- **Major version** (X): Breaking changes
- **Minor version** (Y): New features, backward compatible
- **Patch version** (Z): Bug fixes

### Example Scenarios

| Server | Extension | Result |
|--------|-----------|--------|
| 5.2.5  | 5.2.5     | ✅ No badge |
| 5.2.6  | 5.2.5     | 🔵 Blue "⬆" badge (patch update available) |
| 5.3.0  | 5.2.5     | 🔵 Blue "⬆" badge (minor update available) |
| 5.2.5  | 5.3.0     | ✅ No badge (extension newer than server) |

## How It Works

### Extension Side

#### Version Check Flow

```
Extension starts
    ↓
Connects to server via /health
    ↓
Reads server version: { version: "5.2.6", ... }
    ↓
Compares with manifest version (5.2.5)
    ↓
If server > extension:
  - Sets newVersionAvailable flag
  - Updates badge: "⬆"
  - Updates tooltip
    ↓
Checks again in 30 minutes
```

#### Periodic Polling

- **Interval**: Every 30 minutes (configurable in [src/background/polling.ts:28](../src/background/polling.ts#L28))
- **First check**: Immediately when server connection established
- **Rate limiting**: Prevents excessive checks if `/health` is called more frequently
- **Graceful degradation**: Silently fails if server is unreachable

#### Version Header

Every request to the server includes:

```
X-Gasoline-Extension-Version: 5.2.5
```

This header is automatically added to all API requests:
- `/logs` - Console logs
- `/websocket-events` - WebSocket events
- `/network-bodies` - Network requests
- `/enhanced-actions` - User actions
- `/settings` - Settings sync
- `/pending-queries` - Query polling
- Plus 5+ other endpoints

### Server Side

#### Version Validation

When the server receives a request with `X-Gasoline-Extension-Version` header:

1. **Extracts** the header from the request
2. **Compares** with server version (set at build time via `-ldflags`)
3. **Logs** version mismatches to stderr:
   ```
   [gasoline] Version mismatch: server=5.2.5 extension=5.2.6
   ```

#### Exposing Server Version

The `/health` endpoint returns server version:

```json
{
  "status": "ok",
  "version": "5.2.5",
  "logs": { ... },
  "buffers": { ... }
}
```

## For Developers

### Getting Current Versions

```typescript
import { getExtensionVersion, getLastServerVersion } from 'gasoline';

// Extension version (from manifest.json)
const extVersion = getExtensionVersion(); // "5.2.5"

// Last checked server version
const serverVersion = getLastServerVersion(); // "5.2.6" or null
```

### Checking Update Status

```typescript
import { isNewVersionAvailable } from 'gasoline';

if (isNewVersionAvailable()) {
  console.log('User should update extension');
}
```

### Manual Version Check

```typescript
import { checkServerVersion } from 'gasoline';

// Force a version check (respects rate limiting)
await checkServerVersion(serverUrl, debugLog);
```

### Version Comparison Utilities

Low-level semver comparison functions are available:

```typescript
import {
  parseVersion,
  compareVersions,
  isVersionNewer,
  isVersionSameOrNewer,
  formatVersionDisplay
} from 'gasoline/lib/version';

// Parse version string
const version = parseVersion("5.2.5");
// { major: 5, minor: 2, patch: 5 }

// Compare versions: -1 (A < B), 0 (A == B), 1 (A > B)
compareVersions("5.2.5", "5.2.6"); // -1

// Check if newer
isVersionNewer("5.2.6", "5.2.5"); // true

// Check if same or newer
isVersionSameOrNewer("5.2.5", "5.2.5"); // true
isVersionSameOrNewer("5.3.0", "5.2.5"); // true

// Format for display
formatVersionDisplay("5.2.5"); // "v5.2.5"
```

### Resetting State (Testing)

```typescript
import { resetVersionCheck } from 'gasoline';

// Clear version check state (for testing)
resetVersionCheck();
```

## Configuration

### Extension Polling Interval

Edit [src/background/polling.ts:28](../src/background/polling.ts#L28):

```typescript
const VERSION_CHECK_INTERVAL_MS = 30 * 60 * 1000; // 30 minutes
```

Common values:
- `10 * 1000` = 10 seconds (for testing)
- `5 * 60 * 1000` = 5 minutes (frequent checks)
- `30 * 60 * 1000` = 30 minutes (default, balanced)
- `6 * 60 * 60 * 1000` = 6 hours (infrequent)

### Server Rate Limit

Edit [src/background/version-check.ts:15](../src/background/version-check.ts#L15):

```typescript
const VERSION_CHECK_INTERVAL_MS = 6 * 60 * 60 * 1000; // 6 hours
```

This prevents multiple version checks for the same server in quick succession.

### Setting Server Version at Build Time

```bash
# Build with custom version
go build -ldflags "-X main.version=5.2.6" ./cmd/dev-console
```

The version is automatically read from:
1. Compile-time flag (`-ldflags`)
2. Fallback in code: `main.go:30`

## Troubleshooting

### Badge Not Showing

**Symptoms**: Server is newer but no "⬆" badge appears

**Solutions**:
1. Check that server version is actually newer (semantic comparison)
   ```bash
   curl http://localhost:7890/health | jq .version
   ```
2. Wait for next polling cycle (default 30 minutes, check logs with DevTools)
3. Check browser console for errors in version check
4. Verify extension has permission to access `/health` endpoint

### Version Mismatch in Logs

**Symptom**: Server logs show version mismatch

```
[gasoline] Version mismatch: server=5.2.5 extension=5.2.6
```

**Why**: Extension and server have different versions (may be intentional)

**What to do**:
- This is informational only, not an error
- If blocking issues occur, rebuild extension to match server version
- Check [RELEASE.md](RELEASE.md) for upgrade notes

### Extension Header Not Sent

**Symptom**: Server logs don't show version mismatch (header missing)

**Causes**:
1. Extension is outdated (before version checking was added)
2. Requests are being made through proxy that strips headers
3. CORS blocking (unlikely for localhost)

**Solution**: Rebuild and reinstall extension

## API Reference

### Extension Functions

#### `getExtensionVersion(): string`

Returns the extension version from `manifest.json`.

**Example**:
```typescript
getExtensionVersion(); // "5.2.5"
```

#### `getLastServerVersion(): string | null`

Returns the last checked server version, or `null` if never checked.

**Example**:
```typescript
getLastServerVersion(); // "5.2.6" or null
```

#### `isNewVersionAvailable(): boolean`

Returns `true` if a newer server version is available based on last check.

**Example**:
```typescript
if (isNewVersionAvailable()) {
  showNotification("Update available!");
}
```

#### `checkServerVersion(serverUrl: string, debugLogFn?: Function): Promise<void>`

Manually trigger a version check (respects rate limiting).

**Parameters**:
- `serverUrl`: Server URL (e.g., `http://localhost:7890`)
- `debugLogFn`: Optional debug logging function

**Example**:
```typescript
import { debugLog, DebugCategory } from 'gasoline';

await checkServerVersion(
  'http://localhost:7890',
  (category, message, data) => {
    debugLog(DebugCategory.CONNECTION, message, data);
  }
);
```

#### `updateVersionBadge(): void`

Manually update the extension badge based on current state.

**Example**:
```typescript
// Updates badge to "⬆" if newVersionAvailable is true
updateVersionBadge();
```

#### `resetVersionCheck(): void`

Reset version check state to initial values. **For testing only.**

**Example**:
```typescript
// Clear all version state
resetVersionCheck();
```

### Server Functions

#### `GET /health`

Returns server health info including version.

**Response**:
```json
{
  "status": "ok",
  "version": "5.2.5",
  "logs": {
    "entries": 42,
    "maxEntries": 1000,
    "logFile": "/home/user/gasoline-logs.jsonl",
    "logFileSize": 102400
  },
  "buffers": {
    "websocket_events": 10,
    "network_bodies": 25,
    "actions": 5,
    "connections": 2
  },
  "extension": {
    "connected": true,
    "status": "connected",
    "last_poll_ms": 500,
    "pilot_enabled": false
  }
}
```

#### Request Header: `X-Gasoline-Extension-Version`

All requests from the extension include this header.

**Example**:
```
POST /logs HTTP/1.1
X-Gasoline-Extension-Version: 5.2.5
Content-Type: application/json
```

## Internals

### Files Involved

**Extension (TypeScript)**:
- [src/lib/version.ts](../src/lib/version.ts) - Semver parsing & comparison
- [src/background/version-check.ts](../src/background/version-check.ts) - Version checking logic
- [src/background/polling.ts](../src/background/polling.ts) - Polling loop management
- [src/background/server.ts](../src/background/server.ts) - HTTP header injection
- [src/background/index.ts](../src/background/index.ts) - Integration with polling

**Server (Go)**:
- [cmd/dev-console/main.go](../../cmd/dev-console/main.go) - Header extraction & logging

### State Machine

```
┌──────────────┐
│   IDLE       │ No version checked yet
└──────┬───────┘
       │ First connection
       ↓
┌──────────────────┐
│ CHECKING         │ Polling /health endpoint
└──────┬───────────┘
       │
       ├─→ Match: Version strings are equal
       │   └─→ ✅ NO_UPDATE (badge cleared)
       │
       └─→ Newer: Server > Extension
           └─→ 🔵 UPDATE_AVAILABLE (badge shown)
```

## Migration Guide

### For Existing Installations

The version checking system is **automatic** and requires no configuration.

1. **Update Server**: Build with new version number
   ```bash
   go build -ldflags "-X main.version=5.2.6" ./cmd/dev-console
   ```

2. **Update Extension**: Rebuild and install new version
   ```bash
   make compile-ts
   # Then load updated extension in chrome://extensions
   ```

3. **First Check**: Happens immediately on next connection
   - Extension checks `/health` endpoint
   - Badge updates automatically

### For Custom Deployments

If you're running Gasoline in a custom environment:

1. Ensure `/health` endpoint is accessible from extension
2. Ensure `X-Gasoline-Extension-Version` header is preserved (not stripped by proxy)
3. Set version at build time with `-ldflags`

## See Also

- [RELEASE.md](RELEASE.md) - Version history and upgrade notes
- [README.md](../../README.md) - Installation and setup
- [docs/plugin-server-communications.md](plugin-server-communications.md) - Full protocol spec
