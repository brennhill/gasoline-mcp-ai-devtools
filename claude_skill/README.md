# STRUM — Claude Code Skill

## What is this?

STRUM is an MCP server by default, but it can also run as a CLI. This skill packages the CLI into a Claude Code skill for browser observation, debugging, automation, and testing.

There is no difference in functionality between the MCP and the skill — both expose the same 5 tools and capabilities. The advantage of the skill is that it uses progressive disclosure to load only what's needed, instead of loading ~21,250 tokens of MCP tool schemas into every conversation, saving tokens upfront.

## Installation

1. Install the STRUM Chrome Extension from the [Chrome Web Store](https://chromewebstore.google.com/detail/gasoline/ghgoccajngbgapjmhofojkkagjgffmpj), or [load the unpacked extension](#loading-the-unpacked-extension) for development.
2. Run the install script:
   ```bash
   bash claude_skill/install.sh
   ```
   The script will:
   - Ask where to install the skill files:
     - **Globally** (`~/.claude/skills/gasoline/`) — available in all Claude Code sessions
     - **Project-only** (`.claude/skills/gasoline/`) — available only in the current project
   - Install the MCP server binary (`gasoline-agentic-browser`) via npm. To avoid version mismatches,
     the script asks for your extension version (check `chrome://extensions` → STRUM Devtools).
     Press Enter to install the latest version instead.

   > **Note:** Unlike the standard MCP setup (`npx gasoline-mcp` in `.mcp.json`), this installs
   > the binary globally. The skill calls it directly via HTTP, not as an MCP server.

## Usage

Before using the skill, navigate to the tab you want to inspect, open the STRUM extension popup, and click **Track this tab**. This tells the extension to start capturing telemetry for that tab.

Once tracking, the skill is automatically triggered in Claude Code when you ask about browser debugging, screenshots, automation, audits, etc. Examples:

```
"check my browser for errors"
"take a screenshot of the current page"
"run an accessibility audit"
"click the submit button"
"show me the network requests"
"generate a Playwright test from the last recording"
"what's on the page right now?"
```

## Tools

The skill exposes 5 tools:

| Tool | Purpose | Example modes |
|------|---------|---------------|
| **observe** | Read captured browser telemetry (passive) | errors, logs, network, screenshot, tabs |
| **analyze** | Active analysis queries sent to extension | dom, accessibility, security, performance |
| **generate** | Create artifacts from captured data | test, reproduction, har, sarif |
| **configure** | Session management and diagnostics | health, doctor, store, clear |
| **interact** | Browser automation | click, type, navigate, fill_form, scroll |

## Loading the Unpacked Extension

If you're developing or want to run from source instead of the Chrome Web Store version:

1. Build the extension:
   ```bash
   make compile-ts
   ```
2. Open Chrome and navigate to `chrome://extensions`
3. Enable **Developer mode** (toggle in the top-right corner)
4. Click **Load unpacked**
5. Select the `extension/` folder from this repository
6. The STRUM Devtools extension should now appear in your extensions list

> **Tip:** After making changes to the extension source, run `make compile-ts` again and click the refresh icon on the extension card in `chrome://extensions` to reload it.

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `GASOLINE_PORT` | 7890 | Daemon HTTP port |
