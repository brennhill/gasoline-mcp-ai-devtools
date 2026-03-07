# BlazeTorch AI DevTools — Agent Setup Guide

**For AI agents (Claude, Copilot, etc.) integrating BlazeTorch via MCP.**

---

## What is BlazeTorch?

BlazeTorch AI DevTools is an MCP (Model Context Protocol) server that streams real-time browser telemetry to AI agents. It enables:

- **observe()** — Read console logs, network errors, WebSocket messages, DOM state, screenshots
- **analyze()** — Run accessibility audits, security checks, performance profiling, form analysis
- **generate()** — Create Playwright tests, reproduction scripts, CSP policies, SARIF reports
- **configure()** — Manage noise filtering, storage, streaming, session settings
- **interact()** — Control the browser (click, type, navigate, fill forms, execute JS)

**Core principle:** All data stays local. No cloud, no telemetry, no accounts.

---

## Installation

### Prerequisites

- Chrome or Chromium browser (with extension support)
- One-shot install script (handles everything)

### Quick Install

**macOS / Linux:**
```bash
curl -sSL https://raw.githubusercontent.com/brennhill/blazetorch-ai-devstack/STABLE/scripts/install.sh | bash
```

**Windows (PowerShell):**
```powershell
irm https://raw.githubusercontent.com/brennhill/blazetorch-ai-devstack/STABLE/scripts/install.ps1 | iex
```

**What the script does:**
1. Downloads the latest stable binary for your platform
2. Installs the browser extension to `~/.blazetorch/extension`
3. Auto-configures your MCP client (Claude Code, Cursor, Windsurf, etc.)

### Manual Extension Setup

If the script auto-config doesn't work:

**Option A: Agent-assisted (recommended)**

Ask your AI agent to help:
```
"Can you open the Chrome extensions page and help me load the BlazeTorch extension?"
```

The agent can call:
```
interact(what='navigate', url='chrome://extensions')
```

Then guide you through enabling Developer Mode and loading the extension.

**Option B: Manual**

1. Open `chrome://extensions` (or `brave://extensions` for Brave)
2. Enable **Developer mode** (top right)
3. Click **Load unpacked**
4. Select `~/.blazetorch/extension`

---

## MCP Configuration

BlazeTorch is configured via your MCP client's settings. After installation, verify it's listed:

**Claude Code / Cursor / Windsurf:**
- Settings → MCP Servers
- Should see `blazetorch-mcp` in the list

**Manual Config (if needed):**

Add to `~/.claude/claude.json` or your IDE's MCP config:

```json
{
  "mcpServers": {
    "blazetorch": {
      "command": "blazetorch-mcp",
      "args": []
    }
  }
}
```

The server will start automatically when you first use a BlazeTorch tool.

---

## Helping Users Setup (for Agents)

When a user asks for help installing BlazeTorch, you can:

1. **Guide them to open extensions page:**
```
interact(what='navigate', url='chrome://extensions')
# or for Brave:
# interact(what='navigate', url='brave://extensions')
```

2. **Take a screenshot to see their current state:**
```
observe(what='screenshot')
```

3. **Give them step-by-step instructions:**
- "I opened the Chrome extensions page for you. Do you see a toggle for 'Developer mode' in the top right?"
- "Once enabled, click 'Load unpacked' and select the folder at `~/.blazetorch/extension`"

4. **Verify installation:**
```
configure(what='health')
# If it returns daemon running + extension connected, you're ready
```

5. **Help with troubleshooting:**
```
configure(what='health')  # Check what's not working
observe(what='logs', min_level='debug')  # See debug logs
```

**Pro tip:** You can call `interact(what='screenshot')` to visually verify what the user is seeing, and guide them more accurately.

---

## The 5 Tools

### 1. observe() — Read Browser State

**Purpose:** Capture what's happening in the browser without modifying anything.

**Common modes:**
- `observe(what='screenshot')` — Screenshot of current page
- `observe(what='logs')` — Console logs (all levels)
- `observe(what='errors')` — Console errors + uncaught exceptions
- `observe(what='network_bodies')` — HTTP request/response payloads
- `observe(what='websocket_events')` — WebSocket messages
- `observe(what='page')` — Current URL, title, readable text
- `observe(what='actions')` — Recorded user actions (clicks, types, navigates)

**Example workflow:**
```
1. User navigates to a page
2. You call observe(what='screenshot') to see the page
3. You call observe(what='errors') to check for console errors
4. You call observe(what='network_bodies') to see API payloads
```

### 2. analyze() — Query & Profile

**Purpose:** Active analysis — run audits, query the DOM, profile performance.

**Common modes:**
- `analyze(what='dom', selector='...')` — Query DOM with CSS selectors
- `analyze(what='accessibility')` — WCAG audit (contrast, labels, roles)
- `analyze(what='security_audit')` — Check for credentials, PII, headers
- `analyze(what='performance')` — Web Vitals, core metrics
- `analyze(what='forms')` — Extract form fields and state
- `analyze(what='data_table', selector='...')` — Parse table data

**Example workflow:**
```
1. You see a form on the page
2. You call analyze(what='forms') to extract all fields
3. You call analyze(what='accessibility') to check for ARIA issues
4. You now know what to fill and what errors to fix
```

### 3. generate() — Create Artifacts

**Purpose:** Generate testable, deployable artifacts from captured data.

**Common modes:**
- `generate(what='test', test_name='...')` — Playwright test from context
- `generate(what='reproduction')` — Reproduction script from actions
- `generate(what='har')` — HAR file from network traffic
- `generate(what='csp')` — Content Security Policy from real traffic
- `generate(what='sarif')` — Accessibility/security findings in SARIF format

**Example workflow:**
```
1. You reproduce a bug by clicking through the app
2. You call generate(what='reproduction') to create a Playwright script
3. The script captures the exact sequence: navigate → click → type → assert
4. You get a script that other developers can run
```

### 4. configure() — Session Settings

**Purpose:** Control the extension, manage noise, configure recording.

**Common modes:**
- `configure(what='noise_rule', noise_action='add', ...)` — Add noise filter
- `configure(what='store', store_action='save', key='...', data=...)` — Persist data
- `configure(what='clear', buffer='all')` — Clear buffers
- `configure(what='health')` — Check daemon/extension status
- `configure(what='event_recording_start')` — Start recording actions

**Example workflow:**
```
1. You're debugging a flaky test with console noise
2. You call configure(what='noise_rule', ...) to ignore known errors
3. You call configure(what='event_recording_start') to start recording
4. You reproduce the bug, call generate(what='reproduction')
5. The reproduction script has clean, focused actions
```

### 5. interact() — Control Browser

**Purpose:** Automated browser control (click, type, navigate, execute JS).

**Common modes:**
- `interact(what='navigate', url='...')` — Navigate to URL
- `interact(what='click', selector='...')` — Click an element
- `interact(what='type', selector='...', text='...')` — Type into a field
- `interact(what='fill_form', fields=[...])` — Fill multiple fields
- `interact(what='execute_js', script='...')` — Run JavaScript
- `interact(what='screenshot')` — Screenshot with visible feedback

**Example workflow:**
```
1. You want to test a login form
2. You call interact(what='navigate', url='https://app.example.com/login')
3. You call interact(what='fill_form', fields=[{selector: 'input[name=email]', value: 'test@example.com'}, ...])
4. You call interact(what='click', selector='button[type=submit]')
5. You call observe(what='screenshot') to verify you're logged in
```

---

## Common Agent Workflows

### Workflow 1: Debug a Failing Page Load

```
1. observe(what='screenshot') → see the page state
2. observe(what='errors') → check for console errors
3. observe(what='network_bodies', status_min=400) → find failed API calls
4. analyze(what='performance') → check Web Vitals
5. generate(what='reproduction') → create a script that reproduces the issue
```

### Workflow 2: Test Form Submission

```
1. observe(what='screenshot') → see the form
2. analyze(what='forms') → extract form fields
3. interact(what='fill_form', fields=[...]) → fill the form
4. interact(what='click', selector='button[type=submit]') → submit
5. observe(what='network_bodies', method='POST') → see the request
6. observe(what='screenshot') → confirm success
```

### Workflow 3: Accessibility Audit

```
1. interact(what='navigate', url='https://example.com')
2. analyze(what='accessibility') → run WCAG audit
3. generate(what='sarif') → export findings in SARIF format
4. Share results with your team
```

### Workflow 4: Generate E2E Test from Real Session

```
1. configure(what='event_recording_start') → start recording
2. User interacts with the app (clicks, types, navigates)
3. configure(what='event_recording_stop')
4. generate(what='reproduction') → create Playwright test
5. Test runs deterministically in CI/CD
```

### Workflow 5: Security Audit

```
1. interact(what='navigate', url='https://app.example.com')
2. analyze(what='security_audit', checks=['credentials', 'pii', 'headers'])
3. observe(what='error_bundles') → see any exposed data
4. generate(what='sarif') → export security findings
```

---

## Important Patterns

### Always Check Status First

```
status = configure(what='health')
if status.daemon != 'running':
    # Daemon is down, restart your IDE/tool
```

### Use Semantic Selectors When Possible

Instead of:
```
interact(what='click', selector='div.container > button:nth-child(3)')
```

Use:
```
interact(what='click', selector='text=Submit')
# or
interact(what='click', selector='role=button[name=Submit]')
```

Semantic selectors are more resilient to layout changes.

### Screenshots for Verification

After any significant action, capture a screenshot:
```
interact(what='click', selector='...')
observe(what='screenshot') → verify the action worked
```

### Observe Before Analyze

Always observe the current state before running analysis:
```
observe(what='screenshot') → see what you're working with
analyze(what='accessibility') → now audit it
```

### Use Noise Filtering for CI

In CI environments, filter known transient errors:
```
configure(what='noise_rule',
  noise_action='add',
  category='console',
  classification='alert',  # filter browser alerts
  reason='Known browser notification, not a bug'
)
```

---

## Error Handling

**Common errors and recovery:**

**"Daemon not running"**
- Restart your AI tool (Claude Code, Cursor, etc.)
- The daemon starts automatically on first tool call

**"Extension not connected"**
- Load the extension (chrome://extensions → Load unpacked → ~/.blazetorch/extension)
- Restart your browser
- Restart your AI tool

**"Selector not found"**
- Call `analyze(what='dom', selector='...')` to verify the selector works
- Try a semantic selector instead (text=, role=, etc.)
- Screenshot to see current page state

**"Network call not captured"**
- Ensure you called observe(what='network_bodies') AFTER the request fired
- Check the URL filter: `observe(what='network_bodies', url='...')`
- Verify the request actually failed (status 4xx or 5xx)

---

## Performance Notes

- **observe()** calls are fast (~10-50ms) — safe to call frequently
- **analyze()** calls are slower (~100-500ms) — use sparingly
- **interact()** calls vary (100ms-5s depending on wait times)
- **generate()** calls are async — use `background=true` for long tasks

**Best practice:** Observe → Analyze → Interact → Generate (in that order, reuse data)

---

## Privacy & Security

- **All data is local.** Nothing leaves your machine.
- **No telemetry.** No calls home, no analytics.
- **Auth headers stripped.** Credentials in requests are redacted.
- **Use noise filtering.** Filter out PII from logs before sharing.

```
configure(what='noise_rule',
  noise_action='add',
  category='network',
  url_regex='.*api/private.*',
  reason='Filtering private API calls before sharing'
)
```

---

## Examples

See `/docs/reference/examples/` for detailed examples:
- [observe examples](/docs/reference/examples/observe-examples)
- [analyze examples](/docs/reference/examples/analyze-examples)
- [generate examples](/docs/reference/examples/generate-examples)
- [interact examples](/docs/reference/examples/interact-examples)
- [configure examples](/docs/reference/examples/configure-examples)

---

## Getting Help

**In-session debugging:**
```
configure(what='health')  # Check status
observe(what='logs', min_level='debug')  # See debug logs
configure(what='audit_log', operation='analyze')  # Review recent operations
```

**Online resources:**
- [Full documentation](https://blazetorch.dev)
- [Architecture overview](https://blazetorch.dev/architecture)
- [Troubleshooting](https://blazetorch.dev/troubleshooting)

---

**TL;DR for Agents:**

1. **Install:** Run the install script or manually load the extension
2. **Verify:** Call `configure(what='health')`
3. **Observe:** Capture state with `observe(what='screenshot')`
4. **Analyze:** Query with `analyze(what='dom')` or audit with `analyze(what='accessibility')`
5. **Interact:** Control with `interact(what='click')`, `interact(what='type')`, etc.
6. **Generate:** Create artifacts with `generate(what='test')` or `generate(what='reproduction')`

You're ready to debug, test, and build.
