# Gasoline Extension Permissions & Security Model

## Overview

This document explains why Gasoline requires each permission and how the extension maintains security and privacy.

## Manifest Permissions

### `activeTab`
**Purpose:** Capture logs only from the currently active tab (focused by the user)
**Justification:**
- Enables the extension to execute content scripts in the active tab
- Limits capture scope to user's currently focused context
- Respects user privacy by not capturing background tabs
- Required for screenshot capture on demand

**Security Model:**
- Only applies when user explicitly activates tab tracking in Gasoline UI
- Combined with `tabs` permission to track which tab is "active"
- Does not grant access to all tabs automatically

### `storage`
**Purpose:** Persist user settings and state across browser restarts
**Justification:**
- Stores user configuration preferences:
  - Server URL (where to send telemetry)
  - Log level filter (what severity to capture)
  - Screenshot-on-error toggle
  - Source map enablement
  - Debug mode state
- Enables saved browser state snapshots (for state replay)
- All data stored locally; nothing sent to remote servers without user consent

**Storage Areas:**
- **chrome.storage.local** (persistent, survives browser restart):
  - User preferences (serverUrl, logLevel, screenshotOnError, etc.)
  - State snapshots (manual saves)

- **chrome.storage.session** (ephemeral, cleared on service worker restart):
  - Currently tracked tab ID
  - In-flight request tracking
  - Temporary cache invalidation markers

**Security:**
- No remote syncing; storage is device-local only
- No sensitive authentication data is persisted
- Auth headers are stripped from network capture
- Request/response bodies are opt-in via `Network Body Capture` toggle
- Users can clear extension storage anytime via Chrome settings

### `alarms`
**Purpose:** Schedule periodic background tasks
**Justification:**
- **Reconnection checks** (every 5 seconds): Maintains WebSocket connection to MCP server
  - Enables AI agent to stay connected even if network is interrupted
  - Backoff strategy prevents connection storms during server downtime
- **Error group flushing** (every 30 seconds): Deduplicate similar errors before sending
  - Reduces network traffic and API quota usage
  - Groups identical errors to show frequency and stack traces
- **Memory checks** (every 30 seconds): Monitor memory usage and disable captures if needed
  - Prevents extension from consuming excessive memory
  - Gracefully degrades capture when browser is memory-constrained
- **Error group cleanup** (every 10 minutes): Remove stale deduplication state
  - Prevents unbounded growth of error group metadata

**Security:**
- All tasks execute within service worker (secure extension context)
- No external communication triggered by alarms
- Only the MCP server URL (user-provided) is contacted

### `tabs`
**Purpose:** Track tab lifecycle and identify which tab is being debugged
**Justification:**
- **Tab removal listener**: Clears tracking state when user closes the debugged tab
  - Prevents stale references to closed tabs
  - Cleans up per-tab resources (screenshot rate limits, pending requests)
- **Tab query**: Lists open tabs (used by `browser_action` query from AI agent)
  - Allows AI to see what tabs are open
  - Enables AI to navigate between tabs programmatically
- **Tab activation tracking**: Updates `trackedTabId` when switching tabs
  - Only the currently tracked tab sends telemetry
  - Other tabs are silently ignored

**Security:**
- Only returns tab metadata (id, url, title)
- Does not capture page content or execute arbitrary code
- Tab activation respects `activeTab` permission model
- Cannot access untracked tabs' DOM or console

---

## Trust Boundaries

### Extension Context (Trusted)
The extension background service worker and popup are fully trusted:
- Validates all incoming messages
- Controls what gets sent to MCP server
- Enforces memory limits and rate limiting
- Sanitizes sensitive headers from network capture

### Content Script (Semi-Trusted)
Content scripts run in a sandbox isolated from the page:
- Can receive commands from background service worker
- Cannot be directly accessed by page JavaScript
- Forwards messages to inject.js via `postMessage` with explicit `targetOrigin`
- Validates sender ID before processing background messages

### Inject Script (Untrusted)
Inject scripts run in the page context alongside user's application code:
- Communicates with content script via `window.postMessage` only
- Cannot access extension background directly
- Response messages are validated by content script
- Timeout protection (30s default) prevents hung requests

### Page Context (Untrusted)
User's application code:
- Cannot directly access extension APIs
- Cannot modify or intercept captured telemetry
- Capture can be disabled by user at any time
- User data is never sent to external servers without consent

---

## Message Security

### From Page to Content Script
```
page context (inject.js)
  ↓ window.postMessage with targetOrigin
content script
  ↓ validates source === window
  ↓ validates message type
background service worker
```

**Security Measures:**
- Explicit `targetOrigin` (window.location.origin) prevents cross-origin interception
- Source validation `event.source === window` ensures messages are from page
- Type discrimination ensures message structure matches expected discriminated union
- Timeouts prevent stuck async operations

### From Background to Content Script
```
background service worker
  ↓ chrome.runtime.sendMessage
content script
  ↓ validates sender.id === chrome.runtime.id
  ↓ validates message type
page context (inject.js)
  ↓ window.postMessage with targetOrigin
```

**Security Measures:**
- Sender ID validation ensures message is from extension, not compromised page
- Type guards validate discriminated union before processing
- Exhaustive switch statements (TypeScript) catch missing cases

---

## Sensitive Data Handling

### What is Captured
- Browser console logs (developer debug output)
- Network request metadata (URL, method, status, duration)
- Network request/response bodies (opt-in via toggle)
- JavaScript errors (stack traces, source file locations)
- User actions (clicks, inputs, scrolls)
- WebSocket messages (opt-in via capture mode setting)
- Performance metrics (Core Web Vitals, long tasks, resource timing)

### What is NOT Captured
- Authentication headers (stripped automatically)
- Cookies (never captured)
- Local storage values (only if explicitly enabled for state replay)
- Password fields (input values masked in action capture)
- Response bodies from URLs matching `allowlist` (configurable)

### Data Storage
- **In-memory buffers** (cleared on page unload/refresh)
- **Chrome storage** (local only, never synced)
- **MCP server** (user-provided destination; user controls)

### Data Transmission
- Only to MCP server URL specified in settings
- Only when extension has an active connection
- Respects memory pressure (disables capture if memory > 50MB estimated)
- No telemetry sent to Anthropic or third parties

---

## Performance Impact

Gasoline is designed to have minimal impact on browsing:

- **WebSocket < 0.1ms latency:** Uses non-blocking I/O
- **Fetch < 0.5ms latency:** Network body capture is separate from fetch operation
- **Main thread: Never blocked:** All heavy work (JSON serialization, network I/O) offloaded to service worker
- **Memory: Capped at 50MB:** Circuit breaker disables capture if memory pressure is high
- **Rate limiting:**
  - Max 10 screenshots per minute per tab
  - Max 5-second deduplication window for error groups
  - Max 1000 pending requests buffered

---

## User Controls

Users can configure capture at runtime via Gasoline UI:

- **Server URL:** Change MCP server destination
- **Log Level:** Filter (error, warn, info, log, debug, all)
- **Screenshot on Error:** Auto-capture page when error occurs
- **Source Maps:** Enable/disable resolved stack traces
- **WebSocket Capture:** Off / Lifecycle only / Messages / Full
- **Network Body Capture:** Toggle on/off
- **Performance Marks:** Toggle on/off
- **Action Replay:** Toggle on/off
- **AI Web Pilot:** Toggle on/off (enables ondemand queries from AI)
- **Debug Mode:** Enable internal logging

Users can also:
- Clear all captured logs at any time
- Export debug logs to inspect what the extension is doing
- Clear extension storage/state via Chrome settings
- Disable the extension completely via Chrome settings

---

## Chrome API Availability

Gasoline requires Chrome 102+ for optimal functionality:

- **chrome.storage.session:** Available Chrome 102+
  - Gracefully degrades to memory cache in older versions
  - Ephemeral state survives short service worker interruptions

- **Service Worker:** Available Chrome 91+ (MV3)
  - Replaces background page script model from MV2
  - Reduces memory footprint
  - Automatically restarts if terminated

- **MV3 Restrictions:**
  - No remote code loading (all scripts bundled locally)
  - No eval() or new Function() (prevents code injection)
  - No XMLHttpRequest with `chrome-extension://` URLs
  - Message passing must be async (no synchronous messaging)

---

## Security Principles

1. **Zero Trust:** All messages validated with type guards
2. **Least Privilege:** Only permissions for features actually used
3. **Defense in Depth:** Multiple layers of validation
4. **Fail Secure:** Errors are logged, requests dropped, never sent to remote
5. **Privacy First:** Sensitive data never leaves device unless explicitly enabled
6. **Transparency:** User can inspect what's captured via debug logs
7. **Audit Trail:** All errors/warnings logged to debug console
