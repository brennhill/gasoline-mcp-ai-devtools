# Dev Console - Product Description

## Overview

**Dev Console** is an open-source developer tool that captures browser console logs, network errors, and JavaScript exceptions, piping them to a local file for AI assistants (like Claude Code) to read during UAT and debugging sessions.

**Public Release:** This tool will be released as a standalone open-source project, independent of appFoundation, with:

- Chrome Web Store extension
- npm package for the server (`npx dev-console`)
- Landing page at devcon.so (or similar)

## Problem Statement

During User Acceptance Testing (UAT) with AI assistants, developers must manually copy/paste browser console errors. This is:

- Tedious and error-prone
- Loses context (stack traces get truncated)
- Misses transient errors (errors that flash by)
- Doesn't capture network request/response details

## Solution

A lightweight browser extension + local server that automatically captures and persists all browser diagnostics to a file that AI assistants can read.

## Target Users

- Developers doing UAT with AI coding assistants
- Developers debugging frontend issues
- QA engineers documenting browser errors

## Core Components

| Component         | Technology                     | Purpose                                   |
| ----------------- | ------------------------------ | ----------------------------------------- |
| Browser Extension | Chrome Extension (Manifest V3) | Captures console, network, exceptions     |
| Log Server        | Node.js (single file)          | Receives logs via HTTP, writes to file    |
| Log File          | JSONL format                   | Structured logs readable by AI assistants |

## Key Features

### 1. Console Capture

- `console.log`, `console.info`, `console.warn`, `console.error`, `console.debug`
- Preserves arguments (serialized to JSON)
- Captures timestamp and source URL

### 2. Network Error Capture

- All requests with status >= 400
- Request URL, method, status code
- Response body (truncated if large)
- Request headers (excluding sensitive ones)

### 3. Exception Capture

- Unhandled JavaScript errors
- Unhandled promise rejections
- Full stack traces
- Source file and line number

### 4. Filtering

- Filter by log level (error only, warn+error, all)
- Filter by URL pattern (only capture from specific domains)
- Exclude noisy logs (e.g., React DevTools, browser extensions)

---

## UI Design

### Design Principles

1. **Invisible by Default** - The extension should not interfere with normal browsing
2. **Glanceable Status** - Icon badge shows connection status and error count
3. **Zero Configuration** - Works out of the box with sensible defaults
4. **Developer-Focused** - No unnecessary polish, function over form

### Core UI Elements

| Element        | Purpose            | Key Components                                      |
| -------------- | ------------------ | --------------------------------------------------- |
| Extension Icon | Status indicator   | Badge with error count, color for connection status |
| Popup Panel    | Quick settings     | On/off toggle, filter level dropdown, clear button  |
| Options Page   | Full configuration | URL filters, server URL, log retention              |

### Visual Language

| Element      | Value                   |
| ------------ | ----------------------- |
| Connected    | Green badge             |
| Disconnected | Red badge               |
| Capturing    | Pulsing dot             |
| Error count  | Numeric badge (max 99+) |

### Extension Popup Layout

```
┌─────────────────────────────┐
│ Dev Console          [ON/OFF]│
├─────────────────────────────┤
│ Status: Connected           │
│ Errors captured: 3          │
│ Last error: 2s ago          │
├─────────────────────────────┤
│ Level: [Errors ▼]           │
│ Domain: [All ▼]             │
├─────────────────────────────┤
│ [Clear Logs]    [Settings]  │
└─────────────────────────────┘
```

### Responsive Breakpoints

N/A - Browser extension popup has fixed dimensions.

### Accessibility

- WCAG 2.1 AA compliance for popup UI
- Keyboard navigable
- Screen reader compatible status announcements

---

## Analytics & Tracking

### Privacy Approach

**No tracking whatsoever.** This is a local-only developer tool.

- No telemetry
- No usage analytics
- No crash reporting
- No network calls except to localhost

### Cookie Consent

| Region | Banner Required | Reason                                        |
| ------ | --------------- | --------------------------------------------- |
| All    | No              | No cookies used, no tracking, local-only tool |

### What IS Tracked

Nothing. Zero analytics.

### What is NOT Tracked

- Usage patterns
- Error frequencies
- Feature usage
- User identity
- Browsing history

### Data Retention

- Logs stored locally only
- User controls retention via settings
- Default: 1000 log entries (FIFO)
- Logs can be cleared manually

---

## Observability

### Alerting Strategy

N/A - This is itself an observability tool. No external alerting.

### Local Monitoring

The extension popup shows:

- Connection status to local server
- Number of captured errors
- Time since last error
- Current filter settings

### Error Handling

| Error                  | User Feedback            | Recovery            |
| ---------------------- | ------------------------ | ------------------- |
| Server not running     | Red badge, popup message | Auto-retry every 5s |
| Server connection lost | Yellow badge             | Auto-reconnect      |
| Log write failure      | Console warning          | Retry with backoff  |

### Performance Considerations

- Debounce rapid console outputs (100ms)
- Truncate large payloads (>10KB)
- Don't block page rendering
- Minimal memory footprint

---

## Non-Goals

- Cloud sync
- Team sharing
- Historical analysis
- Pretty log viewer UI
- Support for Firefox/Safari (Chrome-first)

## Success Criteria

1. AI assistant can read browser errors without user copy/paste
2. Zero impact on page performance
3. Works with any web application
4. Setup takes < 2 minutes

---

## Landing Page

### Purpose

A single-page marketing site that:

- Explains the problem and solution clearly
- Shows the tool in action (demo/video)
- Provides quick-start instructions
- Links to Chrome Web Store and npm

### URL Strategy

- Primary: `devcon.so` or `devconsole.dev` (TBD)
- Fallback: `{username}.github.io/dev-console`

### Page Structure

```
┌─────────────────────────────────────────────────────────────────┐
│  [Logo] Dev Console                    [GitHub] [Chrome Store]  │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│         Stop copy-pasting browser errors to ChatGPT.            │
│                                                                 │
│    Dev Console pipes your browser logs directly to your         │
│    AI coding assistant. Zero friction debugging.                │
│                                                                 │
│         [Install Extension]  [View on GitHub]                   │
│                                                                 │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  ┌─────────────────────────────────────────────────────────┐    │
│  │                                                         │    │
│  │              [Animated Demo GIF/Video]                  │    │
│  │   Shows: error in browser → appears in Claude Code      │    │
│  │                                                         │    │
│  └─────────────────────────────────────────────────────────┘    │
│                                                                 │
├─────────────────────────────────────────────────────────────────┤
│                       How it works                              │
│                                                                 │
│   ┌─────────┐      ┌─────────┐      ┌─────────┐                │
│   │ Browser │ ───► │ Server  │ ───► │ AI Tool │                │
│   │Extension│      │(localhost)     │ (reads) │                │
│   └─────────┘      └─────────┘      └─────────┘                │
│                                                                 │
│   1. Extension captures console.log, fetch errors, exceptions  │
│   2. Sends to local server (nothing leaves your machine)       │
│   3. AI assistant reads the log file for instant context       │
│                                                                 │
├─────────────────────────────────────────────────────────────────┤
│                      Quick Start                                │
│                                                                 │
│   # 1. Start the server                                        │
│   $ npx dev-console                                            │
│                                                                 │
│   # 2. Install the extension                                   │
│   [Chrome Web Store button]                                    │
│                                                                 │
│   # 3. That's it! Errors now pipe to ~/dev-console-logs.jsonl  │
│                                                                 │
├─────────────────────────────────────────────────────────────────┤
│                     What gets captured                          │
│                                                                 │
│   ✓ Console logs (log, warn, error, info, debug)               │
│   ✓ Network errors (any request with status >= 400)           │
│   ✓ JavaScript exceptions (with full stack traces)            │
│   ✓ Unhandled promise rejections                              │
│                                                                 │
├─────────────────────────────────────────────────────────────────┤
│                    Privacy First                                │
│                                                                 │
│   • 100% local - nothing leaves your machine                   │
│   • No accounts, no tracking, no analytics                     │
│   • Open source - audit the code yourself                      │
│   • Sensitive headers (Auth, Cookies) are excluded             │
│                                                                 │
├─────────────────────────────────────────────────────────────────┤
│                Works with any AI assistant                      │
│                                                                 │
│   [Claude Code]  [Cursor]  [Copilot]  [Aider]  [Continue]      │
│                                                                 │
│   Any tool that can read files works with Dev Console.         │
│                                                                 │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│   MIT License  •  GitHub  •  npm  •  Chrome Web Store          │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

### Design Principles (Landing Page)

1. **Single Page** - No navigation, everything visible with scroll
2. **Dark Mode First** - Developers prefer dark themes
3. **Code-Focused** - Show real commands and output
4. **Fast** - Static HTML, minimal JS, no framework
5. **Mobile Friendly** - Responsive, works on all devices

### Visual Style

| Element     | Style                                     |
| ----------- | ----------------------------------------- |
| Background  | Dark (#0d1117 - GitHub dark)              |
| Text        | Light gray (#c9d1d9)                      |
| Accent      | Green (#58a6ff) for CTAs                  |
| Code blocks | Darker (#161b22) with syntax highlighting |
| Font        | System fonts (fast loading)               |

### Key Sections

| Section       | Purpose                            |
| ------------- | ---------------------------------- |
| Hero          | Problem statement + primary CTA    |
| Demo          | Animated GIF showing the flow      |
| How it works  | Simple 3-step diagram              |
| Quick start   | Copy-paste commands                |
| Features      | What gets captured                 |
| Privacy       | Build trust                        |
| Compatibility | Show AI tool logos                 |
| Footer        | Links to GitHub, npm, Chrome Store |

### Technical Implementation

- **Hosting**: GitHub Pages (free, reliable)
- **Build**: Static HTML + CSS (no framework)
- **Analytics**: None (practice what we preach)
- **Assets**:
  - Logo (SVG)
  - Demo GIF (< 2MB)
  - AI tool logos (with permission or generic icons)

---

## Public Release Requirements

### Distribution Channels

| Channel          | Purpose                       | Requirements                     |
| ---------------- | ----------------------------- | -------------------------------- |
| GitHub           | Source code, issues, releases | MIT License, good README         |
| npm              | Server package                | `npx dev-console` works          |
| Chrome Web Store | Extension distribution        | $5 developer fee, privacy policy |
| Landing page     | Marketing, docs               | Domain or GitHub Pages           |

### Required Files for Release

```
dev-console/
├── LICENSE                 # MIT
├── README.md              # Quick start, features, links
├── CONTRIBUTING.md        # How to contribute
├── CHANGELOG.md           # Version history
├── docs/
│   ├── product-description.md
│   └── specification.md
├── server/
│   ├── package.json       # name: "dev-console", bin: "dev-console"
│   └── ...
├── extension/
│   ├── manifest.json
│   └── ...
└── landing/
    ├── index.html
    ├── styles.css
    └── assets/
        ├── logo.svg
        ├── demo.gif
        └── og-image.png   # Social sharing
```

### Chrome Web Store Requirements

1. **Developer account** ($5 one-time fee)
2. **Privacy policy** (can be in README - "no data collected")
3. **Store listing**:
   - Title: "Dev Console - AI Debug Assistant"
   - Short description (132 chars)
   - Full description
   - Screenshots (1280x800 or 640x400)
   - Promo images (440x280 small, 920x680 large)
   - Icon (128x128)
4. **Justification for permissions**

### npm Package Requirements

1. **package.json**:
   ```json
   {
     "name": "dev-console",
     "version": "1.0.0",
     "description": "Pipe browser logs to your AI coding assistant",
     "bin": {
       "dev-console": "./index.js"
     },
     "keywords": ["debugging", "ai", "claude", "cursor", "devtools"],
     "license": "MIT"
   }
   ```
2. **No dependencies** (keep it simple)
3. **Node.js >= 18** (use built-in fetch if needed)

### README Structure

```markdown
# Dev Console

> Pipe browser logs to your AI coding assistant

[Demo GIF]

## Quick Start

\`\`\`bash

# Start the server

npx dev-console

# Install the extension

[Chrome Web Store link]
\`\`\`

## What gets captured

- Console logs (log, warn, error)
- Network errors (status >= 400)
- JavaScript exceptions

## Privacy

100% local. Nothing leaves your machine.

## How it works

[Diagram]

## Configuration

[Options]

## License

MIT
```
