# Gasoline Workflows — Claude Code Plugin

Workflow commands that compose Gasoline MCP's 5 browser telemetry tools into complete, opinionated workflows for debugging, auditing, recording, and interactive development.

## Prerequisites

- [Gasoline MCP](https://github.com/brennhill/gasoline-agentic-browser-devtools-mcp) installed and running
- Chrome extension installed and connected to a tab
- Claude Code with MCP support

## Installation

### Option 1: Symlink (for development)

```bash
ln -s /path/to/gasoline/plugin/gasoline-workflows ~/.claude/plugins/gasoline-workflows
```

### Option 2: Copy

```bash
cp -r /path/to/gasoline/plugin/gasoline-workflows ~/.claude/plugins/gasoline-workflows
```

## Commands

### `/debug-ui [description of issue]`

Debug a UI issue end-to-end with systematic evidence capture and root cause classification.

**What it does:**
1. Verifies extension connection
2. Captures errors, logs, network, vitals, and a screenshot in parallel
3. Analyzes DOM structure and computed styles
4. Classifies root cause into 6 categories
5. Generates a reproduction script
6. Outputs a structured diagnosis with evidence, fix, and verification steps

**Example:**
```
/debug-ui the dropdown menu closes immediately after opening
```

**Access:** Browser tools + file read (for correlating errors with source code)

---

### `/seo-audit [url]`

Comprehensive SEO, accessibility, performance, and security audit with a scored report.

**What it does:**
1. Navigates to the URL
2. Extracts all SEO metadata (title, OG tags, structured data, canonical, etc.)
3. Audits page structure, heading hierarchy, semantic HTML
4. Runs WCAG accessibility checks
5. Measures Core Web Vitals against Google thresholds
6. Checks link health, security headers, and third-party scripts
7. Produces a scored report (X/60) with prioritized recommendations

**Example:**
```
/seo-audit https://mysite.com
```

**Access:** Browser tools only (no file access needed)

---

### `/record-workflow [name]`

Record browser interactions and generate a production-quality Playwright test.

**What it does:**
1. Starts a named recording session
2. Waits while you perform the workflow manually
3. Stops recording and shows captured actions
4. Generates a Playwright test with assertions
5. Enhances selectors, wait strategies, and structure
6. Optionally plays back to verify

**Example:**
```
/record-workflow user-signup-flow
```

**Access:** Browser tools only (no file or shell access)

---

### `/interactive-dev [url]`

Start a persistent interactive development session with natural language browser control.

**What it does:**
1. Opens the URL (or uses the current tab) and snapshots the page
2. Enters an interactive loop where you give natural language commands
3. Translates commands to Gasoline tool calls
4. Auto-screenshots after mutations, auto-checks for new errors
5. Full dev tool access: read files, search code, run shell commands

**Example:**
```
/interactive-dev http://localhost:3000
> click the login button
> type admin@test.com in the email field
> show me any errors
> read src/components/Login.tsx
> run npm test
```

**Access:** Full — browser tools, file read, code search, shell

---

## Auto-Triggered Skills

### Connection Guard

Automatically activates when any Gasoline tool call fails with a connection error. Runs health diagnostics, identifies whether the daemon, extension, or tab tracking is the issue, and guides you through recovery.

No manual invocation needed — it triggers on error patterns like "extension not connected", "daemon not running", or "no tracked tab".

## Architecture

This plugin is **pure prompt orchestration** — no application code, no dependencies. Each command is a Markdown file containing:

- Frontmatter with name, description, allowed tools
- A structured workflow with numbered steps
- Tool call patterns and expected parameters
- Output format templates
- Rules and constraints

The commands compose Gasoline's 5 MCP tools (`observe`, `analyze`, `generate`, `interact`, `configure`) into multi-step workflows that would otherwise require knowing the tool API surface.

```
┌──────────────────────────────────────┐
│         Claude Code                  │
│  ┌────────────────────────────────┐  │
│  │  gasoline-workflows plugin     │  │
│  │  ┌──────┐ ┌──────┐ ┌────────┐ │  │
│  │  │debug │ │seo   │ │record  │ │  │
│  │  │-ui   │ │-audit│ │-wkflow │ │  │
│  │  └──┬───┘ └──┬───┘ └───┬────┘ │  │
│  │     │        │         │      │  │
│  │  ┌──┴────────┴─────────┴──┐   │  │
│  │  │   Gasoline MCP Tools   │   │  │
│  │  │ observe│analyze│generate│  │  │
│  │  │ interact│configure     │   │  │
│  │  └────────────────────────┘   │  │
│  └────────────────────────────────┘  │
└──────────────────────────────────────┘
```

## Gasoline Tool Quick Reference

| Tool | Purpose | Key modes |
|------|---------|-----------|
| `observe` | Read browser state | errors, logs, network_waterfall, vitals, actions, recordings |
| `analyze` | Run active analysis | dom, accessibility, performance, security_audit, page_structure, link_health |
| `generate` | Create artifacts | test, reproduction, har, sarif, csp |
| `interact` | Browser automation | navigate, click, type, screenshot, execute_js, explore_page, fill_form |
| `configure` | Session management | health, event_recording_start/stop, playback, save/load state |
