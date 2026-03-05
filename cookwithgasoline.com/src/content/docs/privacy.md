---
title: Privacy Policy
description: Gasoline Privacy Policy - 100% localhost, zero data collection
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
normalized_tags: ['privacy']
---

# Privacy Policy

**Last updated:** February 2, 2026

## TL;DR

**Gasoline never sends your data anywhere.** Everything stays on your machine. No cloud, no external servers, no telemetry.

---

## What We Collect

When you use Gasoline, the browser extension captures telemetry from **the single tab you explicitly track:**

- Console logs (console.log, console.error, etc.)
- Network requests and responses
- JavaScript errors and exceptions
- WebSocket events and messages
- User interactions (clicks, form submissions)
- Performance metrics
- DOM snapshots (when requested by AI)
- Screenshots (only if explicitly enabled in settings)

---

## Where It Goes

**100% localhost only.** All data is sent exclusively to:

```
http://localhost:7890
```

The MCP server runs **on YOUR machine**. We (Gasoline developers) cannot access this data.

**How to verify:**
- Check browser DevTools Network tab - you'll only see localhost:7890 requests
- Check extension manifest: `"host_permissions": ["http://localhost/*"]`
- Run `lsof -i -n -P | grep gasoline` - you'll only see localhost:7890

---

## What We Automatically Redact

Before sending to your localhost server, Gasoline automatically redacts:

- **Passwords** → `[redacted]`
- **API keys, tokens, secrets** → `[redacted]`
- **Credit card numbers, SSNs** → `[redacted]`
- **Authorization headers, cookies** → `[redacted]`

**Implementation:**
- Input field detection (type="password", autocomplete="cc-number", etc.)
- Header filtering (Authorization, Cookie, X-API-Key, X-Auth-Token, etc.)
- Automatic replacement before transmission

**Verification:**
- Source code: [extension/lib/serialize.js](https://github.com/brennhill/gasoline-agentic-browser-devtools-mcp/blob/main/extension/lib/serialize.js) (lines 107-134)
- Header filters: [extension/lib/constants.js](https://github.com/brennhill/gasoline-agentic-browser-devtools-mcp/blob/main/extension/lib/constants.js) (lines 10-17)

---

## What We Never Do

❌ **Send data to external servers** - All communication is localhost-only
❌ **Upload to cloud** - No cloud service, no SaaS
❌ **Share with third parties** - No third-party integrations
❌ **Track you across websites** - Only tracks the ONE tab you select
❌ **Collect analytics or telemetry** - No usage tracking
❌ **Require account creation** - No sign-up, no authentication
❌ **Store data remotely** - Everything stays on your machine

---

## Your Control

**You decide:**
- ✅ **Which tab to track** - Explicit "Track This Tab" button (not automatic)
- ✅ **Whether to enable AI Web Pilot** - Default: OFF, requires toggle
- ✅ **Whether to save logs to disk** - Default: OFF (in-memory only)
- ✅ **What features to enable** - Screenshot on error, network waterfall, etc.
- ✅ **When to stop** - Click "Stop Tracking" or uninstall extension

**Storage options:**
- **In-memory only** (default) - Data cleared when server restarts
- **Local disk** (optional) - Use `--log-file ~/my-logs.jsonl` flag
- **Your own infrastructure** (future) - S3, Postgres, etc. under your control

---

## Data Retention

**Browser extension:**
- Ring buffers (cleared on browser close)
- Settings persist in chrome.storage.local until you uninstall

**MCP server:**
- In-memory ring buffers (cleared on server restart)
- Optional log file (if you use `--log-file` flag, you control retention)

**We don't retain anything** - You control all storage and retention.

---

## Open Source

**Full transparency:**
- Source code: [github.com/brennhill/gasoline-agentic-browser-devtools-mcp](https://github.com/brennhill/gasoline-agentic-browser-devtools-mcp)
- License: AGPL-3.0
- All code is readable (no obfuscation)
- Community can audit our privacy claims

**What you can inspect:**
- [extension/manifest.json](https://github.com/brennhill/gasoline-agentic-browser-devtools-mcp/blob/main/extension/manifest.json) - Permissions requested
- [extension/background/server.js](https://github.com/brennhill/gasoline-agentic-browser-devtools-mcp/blob/main/extension/background/server.js) - Where data is sent
- [extension/lib/serialize.js](https://github.com/brennhill/gasoline-agentic-browser-devtools-mcp/blob/main/extension/lib/serialize.js) - Sensitive data redaction

---

## Permissions Explained

### `tabs` Permission
**Why:** Track a specific browser tab and send telemetry to it
**What we do:** Query tracked tab info, send messages to it
**What we DON'T do:** Track all tabs automatically, read untracked tabs

### `storage` Permission
**Why:** Remember your settings across browser restarts
**What we store:** Tracked tab ID, AI Pilot toggle state, log level, server URL
**What we DON'T store:** Captured telemetry (that goes to localhost), passwords, personal data

### `alarms` Permission
**Why:** Background timers for polling localhost server
**What we do:** Poll localhost:7890 every 1-2 seconds
**What we DON'T do:** Contact external servers, track time-based behavior

### `host_permissions` (localhost only)
**Why:** Send captured telemetry to your local MCP server
**What we do:** POST to localhost:7890 endpoints
**What we DON'T do:** Access any external websites, send data remotely

### Content Scripts (`<all_urls>`)
**Why:** Capture telemetry from any web application being debugged
**What we do:** Inject into the ONE tab you explicitly track
**What we DON'T do:** Inject into untracked tabs, modify page behavior, track browsing

**Why `<all_urls>`?** Gasoline is a developer tool. You need to debug any web application (localhost, staging, production, any domain). Restricting to specific domains would make it useless.

---

## Compliance

### GDPR (EU)
✅ **Compliant** - No personal data leaves your device
- **Data processor:** You (data stays on your machine)
- **Data controller:** You (you decide what to capture)
- **Right to deletion:** Uninstall extension or restart server
- **Right to access:** All data available via localhost API

### CCPA (California)
✅ **Compliant** - Not applicable
- No sale of personal information
- No external data collection

### SOC 2 / Enterprise
✅ **Compliant:**
- Customer-controlled storage
- No third-party processors
- Audit trail (correlation IDs)
- Open source (auditable)

---

## Competitive Difference

**Unlike SaaS observability tools** (Sentry, LogRocket, DataDog):
- ❌ They send data to their cloud
- ❌ You lose control of sensitive information
- ❌ Subject to their data retention policies
- ❌ Vendor lock-in

**Gasoline:**
- ✅ All data stays on your infrastructure
- ✅ You control storage and retention
- ✅ No vendor lock-in
- ✅ Open source (no secrets)

---

## Changes to This Policy

We'll notify users of any changes via:
- GitHub release notes
- Extension update notes
- This page (check "Last updated" date)

---

## Contact

**Questions about privacy?**
- Email: [support@cookwithgasoline.com](mailto:support@cookwithgasoline.com)
- GitHub Issues: [github.com/brennhill/gasoline-agentic-browser-devtools-mcp/issues](https://github.com/brennhill/gasoline-agentic-browser-devtools-mcp/issues)
- Documentation: [cookwithgasoline.com](https://cookwithgasoline.com)

---

## Trust But Verify

**Don't take our word for it:**
1. Open browser DevTools Network tab while using Gasoline
2. Filter by domain - you'll only see localhost:7890
3. Run `lsof -i | grep gasoline` in terminal - only localhost
4. Read the source code on GitHub
5. Ask your security team to audit (it's all open source)

**We're transparent because we have nothing to hide.**

---

*This privacy policy covers the Gasoline browser extension and MCP server. For questions or concerns, please contact us.*
