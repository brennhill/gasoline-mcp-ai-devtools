# Gasoline Demo Automation Guide

## Overview

The demo system uses Playwright to automate a Chrome browser with the Gasoline extension loaded. It triggers real bugs in a purpose-built Next.js app and shows the extension capturing errors in real-time — including opening the real extension popover to display state before and after.

## Architecture

```
demo/
├── app/                    # Next.js 14 app with intentional bugs
│   ├── users/              # 500 error on search "admin"
│   ├── notifications/      # WebSocket disconnects after 10s
│   ├── settings/           # Unhandled promise rejection (503)
│   ├── activity/           # Malformed POST body (422)
│   ├── reports/            # Cannot read properties of null
│   ├── billing/            # Accessibility violations
│   ├── analytics/          # Third-party scripts, PII leakage
│   ├── logs/               # Verbose console output (all levels)
│   ├── checkout/           # PII form (card, SSN, email)
│   ├── integrations/       # Multiple API endpoints
│   └── api/                # API routes that return errors
├── ws-server.mjs           # Standalone WebSocket server (port 3001)
└── scripts/
    ├── run-demo.mjs        # Scene runner (npm run demo <name>)
    ├── utils/setup.mjs     # Playwright helpers
    └── scenes/             # Individual demo scripts
```

## Running Demos

```bash
# Prerequisites: demo app + WS server running
cd demo
npm run dev          # Next.js on :3000
npm run ws           # WebSocket server on :3001

# Run a single demo
npm run demo demo-zero-config
npm run demo demo-console-logs
npm run demo demo-security-scan

# Run all demos sequentially
npm run demo:all
```

## MCP Tool Coverage Map

Every MCP tool has at least one dedicated demo scene:

| MCP Tool | Demo Scene | Page Used |
|----------|-----------|-----------|
| `observe({what:'errors'})` | demo-zero-config | /users |
| `observe({what:'logs'})` | demo-console-logs | /logs |
| `observe({what:'network'})` | demo-network-bodies | /users |
| `observe({what:'websocket_events'})` | demo-websocket-toggle | /notifications |
| `observe({what:'websocket_status'})` | demo-websocket-toggle | /notifications |
| `observe({what:'actions'})` | demo-full-observability | multiple |
| `observe({what:'vitals'})` | demo-full-observability | / |
| `observe({what:'page'})` | demo-full-observability | multiple |
| `analyze({target:'performance'})` | demo-full-observability | / |
| `analyze({target:'api'})` | demo-api-schema | /integrations |
| `analyze({target:'accessibility'})` | billing-a11y | /billing |
| `analyze({target:'changes'})` | demo-checkpoint | multiple |
| `analyze({target:'timeline'})` | demo-generate-repro | multiple |
| `generate({format:'reproduction'})` | demo-generate-repro | /users |
| `generate({format:'test'})` | demo-generate-repro | /users |
| `generate({format:'pr_summary'})` | demo-session-diff | multiple |
| `generate({format:'sarif'})` | demo-security-scan | /checkout |
| `generate({format:'har'})` | demo-network-bodies | /users |
| `configure({action:'noise_rule'})` | demo-noise-filter | /logs |
| `configure({action:'store'})` | demo-session-diff | multiple |
| `configure({action:'dismiss'})` | demo-noise-filter | /logs |
| `configure({action:'clear'})` | demo-checkpoint | multiple |
| `query_dom` | demo-dom-query | /checkout |

## Key Techniques

### 1. Loading the Extension in Playwright

Playwright launches a persistent Chrome context with the extension loaded via command-line flags:

```javascript
const context = await chromium.launchPersistentContext("", {
  headless: false,
  slowMo: 80,
  args: [
    `--disable-extensions-except=${extensionPath}`,
    `--load-extension=${extensionPath}`,
  ],
});
```

The extension's service worker registers automatically. We detect its ID from the service worker URL:

```javascript
const sw = context.serviceWorkers()[0]
  || await context.waitForEvent("serviceworker", { timeout: 5000 });
const extensionId = sw.url().split("/")[2];
```

### 2. Opening the Extension Popover

The real extension popover (not a tab, not a separate window) is opened via `chrome.action.openPopup()` called from the service worker context:

```javascript
export async function openPopup(context) {
  const sw = context.serviceWorkers()[0];

  // Page must be focused for the popup to attach
  const activePage = context.pages().find((p) => p.url() !== "about:blank");
  await activePage.bringToFront();
  await activePage.waitForLoadState("load");
  await new Promise((r) => setTimeout(r, 200));

  // Opens the real popover anchored to the extension icon
  await sw.evaluate(() => chrome.action.openPopup());
  await new Promise((r) => setTimeout(r, 500)); // render time
}
```

**Important constraints:**
- A focused browser window must exist (call `page.bringToFront()` first)
- The popover dismisses automatically when another page gets focus
- To dismiss: `await page.bringToFront()`

### 3. Changing Extension Settings Programmatically

Settings are toggled via `chrome.storage.local.set()` through the service worker — equivalent to the user clicking toggles in the popup UI:

```javascript
export async function setExtensionSetting(context, settings) {
  const sw = context.serviceWorkers()[0];
  await sw.evaluate((s) => chrome.storage.local.set(s), settings);
}
```

Available storage keys:

| Key | Default | Effect |
|-----|---------|--------|
| `webSocketCaptureEnabled` | `false` | Capture WebSocket lifecycle events |
| `networkWaterfallEnabled` | `false` | Capture network waterfall timing |
| `networkBodyCaptureEnabled` | `true` | Include response bodies in captures |
| `actionReplayEnabled` | `true` | Record user actions (clicks, typing) |
| `logLevel` | `"error"` | Capture level: `"error"`, `"warn"`, `"all"` |
| `performanceMarksEnabled` | `false` | Capture performance marks |
| `screenshotOnError` | `false` | Take screenshot on error |
| `sourceMapEnabled` | `false` | Resolve source maps |
| `debugMode` | `false` | Extension debug output |

### 4. Waiting for Data Flush

The extension batches captured data before sending to the server. After triggering an error, wait 3-5 seconds before closing the browser or checking MCP tools:

```javascript
await pause(3000, "Extension batches and sends data to server...");
```

## Feature Demo Scripts

### demo-zero-config
**Story:** "Install Gasoline, do nothing else, errors are captured automatically."

Flow: Load page → show popover (0 errors) → trigger 500 → wait for flush → show popover (1 error)

### demo-websocket-toggle
**Story:** "One toggle gives you WebSocket visibility."

Flow: Show popover (WS OFF) → enable WS capture → show popover (WS ON) → navigate to notifications → wait for disconnect → show popover (error captured)

### demo-network-bodies
**Story:** "Control what your AI sees — toggle bodies on/off for privacy or token savings."

Flow: Show popover (bodies ON) → trigger error → disable bodies → show popover (bodies OFF) → trigger same error → show popover (2 errors, first has body, second doesn't)

### demo-full-observability
**Story:** "Turn everything on, trigger 3 different bug types, AI sees it all."

Flow: Enable all settings → show popover (all ON) → trigger 500 + 503 + 422 → wait for flush → show popover (multiple errors)

### demo-console-logs
**Story:** "Your AI sees every console.log, console.warn, and console.error — automatically."

Flow: Navigate to /logs → wait for streaming logs → trigger error burst → trigger heartbeats → show that `observe({what:'logs'})` captures all output with levels and timestamps.

### demo-dom-query
**Story:** "Ask about the DOM state from your AI — no DevTools needed."

Flow: Navigate to /checkout → fill PII form fields → show that `query_dom({selector:'.card-input'})` returns live element attributes and content.

### demo-api-schema
**Story:** "Gasoline builds an API schema from observed traffic — zero config."

Flow: Navigate to /integrations → trigger sync/test/stats calls → search users → show that `analyze({target:'api'})` produces endpoint list with methods, status codes, and response shapes.

### demo-security-scan
**Story:** "Find PII leaking in network requests before it ships."

Flow: Fill checkout with card/SSN/email → submit → visit analytics page → show that `generate({format:'sarif'})` flags PII in request bodies and third-party script loads.

### demo-csp-generator
**Story:** "Build a Content Security Policy from real browser behavior — passively."

Flow: Visit /analytics (third-party scripts) → navigate around → revisit analytics → show that observed origins produce a ready-to-use CSP header.

### demo-noise-filter
**Story:** "30 heartbeat logs drowning out 3 real errors? Filter the noise."

Flow: Navigate to /logs → flood heartbeats 3x → trigger error burst → show 33+ entries → apply noise rule → only 3 critical errors remain.

### demo-session-diff
**Story:** "Compare before and after — what changed when the bugs appeared?"

Flow: Browse normally (baseline) → checkpoint → trigger 3 bugs (500, rejection, 422) → diff shows exactly what went wrong.

### demo-checkpoint
**Story:** "Time-travel debugging — see everything that happened since a point in time."

Flow: Checkpoint at clean state → user searches, syncs, navigates → error occurs → `analyze({target:'changes'})` shows all activity since checkpoint.

### demo-generate-repro
**Story:** "One command generates a reproduction script from captured actions."

Flow: Navigate → trigger 500 bug → retry → show that `generate({format:'reproduction'})` produces a runnable Playwright script replicating the exact user journey.

## Bug Trigger Scenes (Non-UI)

These are simpler scripts that trigger a single bug without showing the popup:

| Scene | Bug Type | Trigger |
|-------|----------|---------|
| `dashboard-cls` | CLS shift | Load / (2s chart delay) |
| `users-500` | Network 500 | Search "admin" on /users |
| `notifications-ws` | WebSocket 1006 | Wait 10s on /notifications |
| `settings-rejection` | Unhandled rejection | Click save on /settings |
| `reports-undefined` | TypeError | Load /reports (40% chance) |
| `billing-a11y` | A11y violations | Load /billing |
| `activity-payload` | Validation 422 | Auto-fires on /activity load |

## Demo Pages

| Page | Purpose | Bugs/Behaviors |
|------|---------|----------------|
| `/` | Dashboard | CLS layout shift (delayed chart) |
| `/users` | User management | 500 on search "admin" |
| `/notifications` | Live feed | WS disconnects after 10s |
| `/settings` | Config panel | Unhandled promise rejection (503) |
| `/activity` | Activity log | Malformed POST body (422) |
| `/reports` | Reports view | `null` property access (40%) |
| `/billing` | Billing page | A11y violations (contrast, labels) |
| `/analytics` | Analytics dashboard | Third-party scripts, PII leakage to external origins |
| `/logs` | Log viewer | Console output at all levels, heartbeat floods |
| `/checkout` | Checkout form | PII fields (card, SSN), raw PII in POST body |
| `/integrations` | Integrations hub | Multiple API endpoints, sync/test/stats calls |

## Tips for Recording Demos

1. **Window size:** Scripts launch at 1280x720 — good for screen recording
2. **slowMo: 80** — Actions are slightly slowed for visibility
3. **Popover timing:** The popover stays open during `pause()` calls. Increase pause duration for longer visibility
4. **Natural typing:** `typeNaturally()` simulates human typing speed (100-150ms per character)
5. **Sequential flow:** Each step has a descriptive log message — useful for voiceover timing
6. **Run order:** For a full product demo, run scenes in this order: zero-config → console-logs → network-bodies → websocket-toggle → dom-query → api-schema → noise-filter → checkpoint → session-diff → security-scan → csp-generator → generate-repro → full-observability
