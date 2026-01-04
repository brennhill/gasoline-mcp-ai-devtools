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
npm run demo demo-websocket-toggle
npm run demo demo-network-bodies
npm run demo demo-full-observability

# Run all demos sequentially
npm run demo:all
```

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

## Demo Scripts

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

## Bug Trigger Scenes (Non-UI)

These are simpler scripts that trigger a single bug without showing the popup:

| Scene | Bug Type | Trigger |
|-------|----------|---------|
| `users-500` | Network 500 | Search "admin" on /users |
| `ws-disconnect` | WebSocket 1006 | Wait 10s on /notifications |
| `settings-rejection` | Unhandled rejection | Click save on /settings |
| `activity-422` | Validation error | Auto-fires on /activity load |
| `reports-null` | TypeError | Load /reports (40% chance) |
| `billing-a11y` | A11y violations | Load /billing |
| `dashboard-cls` | CLS shift | Load / (2s chart delay) |

## Tips for Recording Demos

1. **Window size:** Scripts launch at 1280x720 — good for screen recording
2. **slowMo: 80** — Actions are slightly slowed for visibility
3. **Popover timing:** The popover stays open during `pause()` calls. Increase pause duration for longer visibility
4. **Natural typing:** `typeNaturally()` simulates human typing speed (100-150ms per character)
5. **Sequential flow:** Each step has a descriptive log message — useful for voiceover timing
