# Dev Console - Technical Specification

## Architecture Overview

```
┌─────────────────────────────────────────────────────────────────┐
│                         Browser                                  │
│  ┌─────────────────────────────────────────────────────────┐    │
│  │                    Web Page                              │    │
│  │  console.log() ──┐                                       │    │
│  │  console.error()─┤                                       │    │
│  │  fetch() ────────┤                                       │    │
│  │  XHR ────────────┤                                       │    │
│  │  exceptions ─────┤                                       │    │
│  └──────────────────┼───────────────────────────────────────┘    │
│                     │                                            │
│  ┌──────────────────▼───────────────────────────────────────┐    │
│  │              Content Script                               │    │
│  │  - Injects capture script into page                       │    │
│  │  - Receives logs via window.postMessage                   │    │
│  │  - Forwards to background service worker                  │    │
│  └──────────────────┬───────────────────────────────────────┘    │
│                     │                                            │
│  ┌──────────────────▼───────────────────────────────────────┐    │
│  │            Background Service Worker                      │    │
│  │  - Batches logs (debounce 100ms)                         │    │
│  │  - Sends to local server via HTTP POST                    │    │
│  │  - Manages connection status                              │    │
│  │  - Updates badge                                          │    │
│  └──────────────────┬───────────────────────────────────────┘    │
└─────────────────────┼────────────────────────────────────────────┘
                      │ HTTP POST
                      ▼
┌─────────────────────────────────────────────────────────────────┐
│                    Log Server (Node.js)                          │
│  - Listens on http://localhost:7890                              │
│  - POST /logs - Receive log entries                              │
│  - GET /health - Connection check                                │
│  - Appends to ~/dev-console-logs.jsonl                           │
└──────────────────────┬──────────────────────────────────────────┘
                       │
                       ▼
┌─────────────────────────────────────────────────────────────────┐
│                 ~/dev-console-logs.jsonl                         │
│  {"ts":"...","level":"error","msg":"...","url":"..."}            │
│  {"ts":"...","level":"network","status":500,"url":"..."}         │
│  {"ts":"...","level":"exception","error":"...","stack":"..."}    │
└─────────────────────────────────────────────────────────────────┘
```

---

## User Stories

### US-1: Start Log Server

**As a** developer
**I want to** start the log server with a single command
**So that** I can begin capturing browser logs

**Acceptance Criteria:**

- [ ] `npx dev-console` starts server on port 7890
- [ ] Server outputs "Dev Console listening on http://localhost:7890"
- [ ] Server creates log file if it doesn't exist
- [ ] Server shows "Ready to receive logs" message

**Technical Notes:**

- Single-file Node.js server (no dependencies)
- Use built-in `http` module
- Log file: `~/dev-console-logs.jsonl`

---

### US-2: Install Browser Extension

**As a** developer
**I want to** install the extension from local files
**So that** I can start capturing logs immediately

**Acceptance Criteria:**

- [ ] Extension can be loaded via chrome://extensions (unpacked)
- [ ] Extension icon appears in toolbar
- [ ] Extension shows "Disconnected" until server is running
- [ ] Once server is running, shows "Connected"

**Technical Notes:**

- Manifest V3 format
- Minimal permissions: `activeTab`, `storage`
- Host permission for `http://localhost:7890/*`

---

### US-3: Capture Console Logs

**As a** developer
**I want** all console output to be captured
**So that** I can review logs without keeping DevTools open

**Acceptance Criteria:**

- [ ] `console.log()` captured with arguments
- [ ] `console.warn()` captured with level "warn"
- [ ] `console.error()` captured with level "error"
- [ ] `console.info()` captured with level "info"
- [ ] `console.debug()` captured with level "debug"
- [ ] Circular references handled (don't crash)
- [ ] Large objects truncated at 10KB

**Log Format:**

```json
{
  "ts": "2024-01-22T10:30:00.000Z",
  "level": "error",
  "type": "console",
  "args": ["Error message", { "detail": "..." }],
  "url": "http://localhost:3000/dashboard",
  "source": "app.js:42"
}
```

---

### US-4: Capture Network Errors

**As a** developer
**I want** failed HTTP requests to be logged
**So that** I can see API errors without Network tab

**Acceptance Criteria:**

- [ ] Requests with status >= 400 are captured
- [ ] Request method and URL logged
- [ ] Response status code logged
- [ ] Response body logged (truncated at 5KB)
- [ ] Request duration logged
- [ ] Sensitive headers excluded (Authorization, Cookie)

**Log Format:**

```json
{
  "ts": "2024-01-22T10:30:00.000Z",
  "level": "error",
  "type": "network",
  "method": "POST",
  "url": "http://localhost:8789/auth/login",
  "status": 401,
  "statusText": "Unauthorized",
  "duration": 234,
  "response": { "error": "Invalid credentials" }
}
```

---

### US-5: Capture Unhandled Exceptions

**As a** developer
**I want** JavaScript errors to be automatically logged
**So that** I don't miss errors that happen quickly

**Acceptance Criteria:**

- [ ] `window.onerror` events captured
- [ ] Unhandled promise rejections captured
- [ ] Error message logged
- [ ] Stack trace logged (full)
- [ ] Source file and line number logged

**Log Format:**

```json
{
  "ts": "2024-01-22T10:30:00.000Z",
  "level": "error",
  "type": "exception",
  "message": "Cannot read property 'x' of undefined",
  "stack": "TypeError: Cannot read property 'x' of undefined\n    at foo (app.js:42)\n    at bar (app.js:100)",
  "filename": "app.js",
  "lineno": 42,
  "colno": 15
}
```

---

### US-6: Filter by Log Level

**As a** developer
**I want to** filter which log levels are captured
**So that** I can focus on errors during UAT

**Acceptance Criteria:**

- [ ] Popup shows level dropdown: All, Warnings+, Errors Only
- [ ] Setting persists across browser restarts
- [ ] Default is "Errors Only"
- [ ] Changing filter takes effect immediately

---

### US-7: Filter by Domain

**As a** developer
**I want to** capture logs only from specific domains
**So that** I don't get noise from other tabs

**Acceptance Criteria:**

- [ ] Options page allows adding domain patterns
- [ ] Support glob patterns (e.g., `*.localhost`, `localhost:*`)
- [ ] Default: capture from all domains
- [ ] Empty filter = capture all

---

### US-8: View Connection Status

**As a** developer
**I want to** see if the extension is connected to the server
**So that** I know if logs are being captured

**Acceptance Criteria:**

- [ ] Green badge = connected
- [ ] Red badge = disconnected
- [ ] Badge shows error count when connected
- [ ] Popup shows detailed status message
- [ ] Auto-reconnect every 5 seconds when disconnected

---

### US-9: Clear Logs

**As a** developer
**I want to** clear the log file
**So that** I can start fresh for a new test session

**Acceptance Criteria:**

- [ ] "Clear Logs" button in popup
- [ ] Sends DELETE request to server
- [ ] Server truncates log file
- [ ] Badge count resets to 0
- [ ] Confirmation not required (developer tool)

---

### US-10: Read Logs (AI Assistant)

**As an** AI assistant
**I want to** read the log file
**So that** I can help debug issues

**Acceptance Criteria:**

- [ ] Log file is valid JSONL (one JSON object per line)
- [ ] File is human-readable (pretty timestamps)
- [ ] File can be tailed (`tail -f ~/dev-console-logs.jsonl`)
- [ ] New logs appear within 100ms of browser event
- [ ] File doesn't grow unbounded (max 1000 entries, FIFO)

---

### US-11: Automatic Log Rotation

**As a** developer
**I want** old logs to be automatically removed
**So that** the log file doesn't grow forever

**Acceptance Criteria:**

- [ ] Default max entries: 1000
- [ ] When limit exceeded, oldest entries removed
- [ ] Configurable via server flag: `--max-entries 5000`
- [ ] Current entry count shown in server output

---

## API Specification

### Log Server Endpoints

#### POST /logs

Receive log entries from browser extension.

**Request:**

```json
{
  "entries": [
    {"ts": "...", "level": "error", "type": "console", "args": [...]}
  ]
}
```

**Response:**

```json
{ "received": 3 }
```

#### GET /health

Check server status.

**Response:**

```json
{
  "status": "ok",
  "entries": 42,
  "maxEntries": 1000,
  "logFile": "/Users/dev/dev-console-logs.jsonl"
}
```

#### DELETE /logs

Clear all logs.

**Response:**

```json
{ "cleared": true }
```

---

## File Structure

```
dev-console/                    # Standalone repo (not apps/dev-console)
├── LICENSE                     # MIT License
├── README.md                   # Main documentation
├── CONTRIBUTING.md             # Contribution guidelines
├── CHANGELOG.md                # Version history
├── .github/
│   └── workflows/
│       ├── test.yml            # Run tests on PR
│       ├── publish-npm.yml     # Publish to npm on release
│       └── deploy-landing.yml  # Deploy landing page
├── docs/
│   ├── product-description.md
│   └── specification.md
├── server/
│   ├── package.json            # npm package config
│   ├── index.js                # Single-file server
│   └── __tests__/
│       └── server.test.js
├── extension/
│   ├── manifest.json
│   ├── background.js           # Service worker
│   ├── content.js              # Content script
│   ├── inject.js               # Injected capture script
│   ├── popup.html
│   ├── popup.js
│   ├── options.html
│   ├── options.js
│   └── icons/
│       ├── icon-16.png
│       ├── icon-48.png
│       └── icon-128.png
└── landing/
    ├── index.html              # Single-page landing
    ├── styles.css              # Minimal CSS
    ├── CNAME                   # Custom domain (optional)
    └── assets/
        ├── logo.svg
        ├── demo.gif            # Animated demo
        ├── og-image.png        # Social sharing image
        └── favicon.ico
```

---

## Security Considerations

1. **Local Only** - Server binds to 127.0.0.1, not 0.0.0.0
2. **No Sensitive Data** - Authorization headers excluded from network logs
3. **No Eval** - Extension doesn't execute arbitrary code
4. **Manifest V3** - Uses modern, more secure extension APIs
5. **Minimal Permissions** - Only requests necessary permissions

---

## Performance Requirements

| Metric                   | Target |
| ------------------------ | ------ |
| Log capture latency      | < 10ms |
| Server write latency     | < 50ms |
| Memory usage (extension) | < 10MB |
| Memory usage (server)    | < 50MB |
| CPU usage (idle)         | ~0%    |

---

## Testing Strategy

### Server Tests

- [ ] Server starts on correct port
- [ ] POST /logs writes to file
- [ ] GET /health returns status
- [ ] DELETE /logs clears file
- [ ] Log rotation works at limit
- [ ] Invalid JSON rejected gracefully

### Extension Tests

- [ ] Console methods intercepted correctly
- [ ] Network errors captured
- [ ] Exceptions captured
- [ ] Filtering works
- [ ] Reconnection works
- [ ] Badge updates correctly

### Integration Tests

- [ ] End-to-end: console.error → file
- [ ] End-to-end: fetch 500 → file
- [ ] End-to-end: throw Error → file

---

## Landing Page Specification

### US-12: View Landing Page

**As a** potential user
**I want to** understand what Dev Console does in 10 seconds
**So that** I can decide if it's useful for me

**Acceptance Criteria:**

- [ ] Hero section explains the problem in one sentence
- [ ] Solution is clear without scrolling
- [ ] Primary CTA (Install) is prominent
- [ ] Page loads in < 1 second
- [ ] Works on mobile

---

### US-13: See Demo

**As a** potential user
**I want to** see the tool in action
**So that** I understand how it works before installing

**Acceptance Criteria:**

- [ ] Animated GIF or video shows full flow
- [ ] Demo is < 15 seconds long
- [ ] Shows: browser error → log file → AI reads it
- [ ] Autoplays (GIF) or has obvious play button (video)
- [ ] File size < 2MB for fast loading

---

### US-14: Quick Start Instructions

**As a** developer
**I want to** copy-paste installation commands
**So that** I can get started immediately

**Acceptance Criteria:**

- [ ] `npx dev-console` command is copy-able
- [ ] Chrome Web Store link is prominent
- [ ] Steps are numbered (1, 2, 3)
- [ ] No account required messaging is clear

---

### US-15: Understand Privacy

**As a** security-conscious developer
**I want to** know what data is collected
**So that** I can trust the tool

**Acceptance Criteria:**

- [ ] "100% local" is prominently stated
- [ ] "No tracking" is explicitly mentioned
- [ ] "Open source" links to GitHub
- [ ] Sensitive data exclusions are mentioned

---

### US-16: Find Source Code

**As a** developer
**I want to** view the source code
**So that** I can audit it or contribute

**Acceptance Criteria:**

- [ ] GitHub link in header
- [ ] GitHub link in footer
- [ ] Star count displayed (social proof)
- [ ] License (MIT) is visible

---

### US-17: Mobile Experience

**As a** developer on mobile
**I want to** read about the tool on my phone
**So that** I can save it for later

**Acceptance Criteria:**

- [ ] Page is fully responsive
- [ ] Text is readable without zooming
- [ ] Demo GIF scales appropriately
- [ ] CTAs are tap-friendly (min 44px)

---

### US-18: Social Sharing

**As a** user who wants to share
**I want** the page to have good social previews
**So that** links look good on Twitter/LinkedIn

**Acceptance Criteria:**

- [ ] Open Graph meta tags present
- [ ] og:image is 1200x630 (optimal for Twitter/LinkedIn)
- [ ] og:title: "Dev Console - Pipe browser logs to your AI"
- [ ] og:description: compelling one-liner
- [ ] Twitter card meta tags present

---

## Landing Page Technical Spec

### File Structure

```
landing/
├── index.html          # Single page, all content
├── styles.css          # Minimal CSS, no framework
├── assets/
│   ├── logo.svg        # Dev Console logo
│   ├── demo.gif        # Animated demo (< 2MB)
│   ├── og-image.png    # Social sharing (1200x630)
│   ├── favicon.ico
│   └── icons/          # AI tool logos (if permissions allow)
└── CNAME               # Custom domain (optional)
```

### HTML Structure

```html
<!DOCTYPE html>
<html lang="en">
  <head>
    <meta charset="UTF-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1.0" />
    <title>Dev Console - Pipe browser logs to your AI coding assistant</title>

    <!-- SEO -->
    <meta
      name="description"
      content="Stop copy-pasting browser errors. Dev Console pipes console logs, network errors, and exceptions directly to Claude, Cursor, or any AI coding assistant."
    />

    <!-- Open Graph -->
    <meta property="og:title" content="Dev Console - Pipe browser logs to your AI" />
    <meta
      property="og:description"
      content="Stop copy-pasting browser errors to ChatGPT. Zero friction debugging with your AI coding assistant."
    />
    <meta property="og:image" content="https://devcon.so/assets/og-image.png" />
    <meta property="og:url" content="https://devcon.so" />
    <meta property="og:type" content="website" />

    <!-- Twitter -->
    <meta name="twitter:card" content="summary_large_image" />
    <meta name="twitter:title" content="Dev Console - Pipe browser logs to your AI" />
    <meta
      name="twitter:description"
      content="Stop copy-pasting browser errors. Zero friction debugging."
    />
    <meta name="twitter:image" content="https://devcon.so/assets/og-image.png" />

    <!-- Favicon -->
    <link rel="icon" href="/assets/favicon.ico" />

    <!-- Styles -->
    <link rel="stylesheet" href="styles.css" />
  </head>
  <body>
    <header>...</header>
    <main>
      <section id="hero">...</section>
      <section id="demo">...</section>
      <section id="how-it-works">...</section>
      <section id="quick-start">...</section>
      <section id="features">...</section>
      <section id="privacy">...</section>
      <section id="compatibility">...</section>
    </main>
    <footer>...</footer>
  </body>
</html>
```

### CSS Variables

```css
:root {
  /* Colors - GitHub Dark theme inspired */
  --bg-primary: #0d1117;
  --bg-secondary: #161b22;
  --bg-tertiary: #21262d;
  --text-primary: #c9d1d9;
  --text-secondary: #8b949e;
  --accent: #58a6ff;
  --accent-hover: #79c0ff;
  --success: #3fb950;
  --border: #30363d;

  /* Typography */
  --font-sans: -apple-system, BlinkMacSystemFont, 'Segoe UI', Helvetica, Arial, sans-serif;
  --font-mono: ui-monospace, SFMono-Regular, 'SF Mono', Menlo, monospace;

  /* Spacing */
  --space-xs: 0.25rem;
  --space-sm: 0.5rem;
  --space-md: 1rem;
  --space-lg: 2rem;
  --space-xl: 4rem;

  /* Breakpoints (for reference) */
  /* Mobile: < 640px */
  /* Tablet: 640px - 1024px */
  /* Desktop: > 1024px */
}
```

### Responsive Breakpoints

```css
/* Mobile first */
.container {
  padding: 1rem;
}

/* Tablet */
@media (min-width: 640px) {
  .container {
    padding: 2rem;
    max-width: 640px;
    margin: 0 auto;
  }
}

/* Desktop */
@media (min-width: 1024px) {
  .container {
    max-width: 900px;
  }
}
```

### Performance Budget

| Metric                   | Target  |
| ------------------------ | ------- |
| Total page size          | < 500KB |
| Largest Contentful Paint | < 1.5s  |
| First Contentful Paint   | < 0.5s  |
| Time to Interactive      | < 1s    |
| Lighthouse Performance   | > 95    |

### Deployment

```yaml
# .github/workflows/deploy-landing.yml
name: Deploy Landing Page

on:
  push:
    branches: [main]
    paths: ['landing/**']

jobs:
  deploy:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: peaceiris/actions-gh-pages@v3
        with:
          github_token: ${{ secrets.GITHUB_TOKEN }}
          publish_dir: ./landing
```

---

## Public Release Checklist

### Before Launch

- [ ] Server works via `npx dev-console`
- [ ] Extension loads in Chrome (unpacked)
- [ ] README has clear quick-start
- [ ] LICENSE file present (MIT)
- [ ] CHANGELOG initialized
- [ ] Landing page deployed to GitHub Pages

### Chrome Web Store Submission

- [ ] Developer account created ($5)
- [ ] Screenshots captured (1280x800)
- [ ] Store listing written
- [ ] Privacy policy section complete
- [ ] Permission justifications written
- [ ] Extension tested on multiple sites

### npm Publish

- [ ] `npm publish` tested with `--dry-run`
- [ ] Package name available (`dev-console`)
- [ ] `npx dev-console` works fresh
- [ ] Version set to 1.0.0

### Marketing

- [ ] Tweet/post about launch
- [ ] Submit to Hacker News
- [ ] Post in r/webdev, r/programming
- [ ] Share in AI coding tool communities
