---
title: Privacy & Data Collection
description: Strum Privacy Policy — what we collect, what we don't, and how to opt out
last_verified_version: 0.9.0
last_verified_date: 2026-03-21
normalized_tags: ['privacy']
---

# Privacy & Data Collection

**Last updated:** March 21, 2026

## TL;DR

**Your browser data stays on your machine.** Console logs, network requests, screenshots, and everything Strum captures from your browser never leaves localhost.

**We collect anonymous usage metrics** — which tools you use and how often — to improve Strum. No personal information, no URLs, no code. You can opt out with one environment variable.

---

## Two Separate Things

Strum handles two types of data very differently:

### 1. Browser Telemetry (YOUR data — stays local)

Everything Strum captures from your browser stays on your machine:

- Console logs, errors, exceptions
- Network requests and responses
- WebSocket events
- User interactions (clicks, form submissions)
- Screenshots (when requested)
- DOM snapshots
- Performance metrics

**Where it goes:** `http://localhost:7890` only. The Strum server runs on YOUR machine. We cannot access this data.

**Verification:** Check browser DevTools Network tab — you'll only see `localhost:7890` requests.

### 2. Anonymous Product Metrics (OUR data — sent to us)

We collect anonymous usage counters to understand how Strum is used:

| What we collect | Example | Why |
|----------------|---------|-----|
| Random install ID | `f7a2c1e9b4d8` | Correlate usage over time without identifying you |
| Tool usage counts | `observe:errors: 12` | Know which features matter |
| OS and version | `darwin-arm64`, `0.8.1` | Know what to support |
| Install/scaffold outcomes | `install_complete`, `scaffold_complete` | Measure onboarding success |
| Error categories | `bridge_connection_error` | Fix common failure patterns |

**What we DON'T collect:**

| Never collected | Why not |
|----------------|---------|
| IP addresses | Not stored, not logged — our endpoint discards them |
| Your name, email, or identity | No accounts, no sign-up |
| URLs you're debugging | Not included in any beacon |
| Your code or file paths | Not included in any beacon |
| Error messages from your app | Could contain PII — only error *categories* sent |
| Screenshots or page content | Never leaves localhost |
| Project names or descriptions | Could identify you — excluded |
| Machine fingerprints | Install ID is pure random, not derived from hardware |

---

## The Install ID

When Strum first starts, it generates a random 12-character hex string (e.g., `f7a2c1e9b4d8`) and saves it at `~/.strum/install_id`. This is:

- **Randomly generated** — `crypto/rand`, not derived from your machine, username, or IP
- **Not reversible** — cannot be traced back to you
- **Used for** — "this install uses observe:errors a lot" not "this person does X"

---

## Usage Summary Beacon

Every 10 minutes (if there was activity), Strum sends one aggregated event:

```json
{
  "event": "usage_summary",
  "v": "0.8.1",
  "os": "darwin-arm64",
  "iid": "f7a2c1e9b4d8",
  "props": {
    "window_m": "10",
    "observe:errors": "12",
    "interact:click": "24",
    "analyze:accessibility": "1"
  }
}
```

That's it. Tool names and counts. No URLs, no selectors, no content. If Strum is idle, no beacon is sent.

---

## How to Opt Out

Set one environment variable:

```bash
export STRUM_TELEMETRY=off
```

All beacons stop immediately. Strum works exactly the same — no features are degraded or locked.

Add it to your shell profile (`~/.zshrc`, `~/.bashrc`) to make it permanent.

---

## What We Automatically Redact (Browser Data)

Before browser telemetry reaches the localhost server, Strum redacts:

- **Passwords** → `[redacted]`
- **API keys, tokens, secrets** → `[redacted]`
- **Credit card numbers, SSNs** → `[redacted]`
- **Authorization headers, cookies** → `[redacted]`

This is defense-in-depth — the data never leaves your machine anyway, but we redact it before it even reaches the local server.

---

## Your Control

**Browser data (local):**
- You choose which tab to track (not automatic)
- You choose whether to enable AI Web Pilot (default: off)
- You choose whether to save logs to disk (default: in-memory only)
- Stop anytime — click "Stop Tracking" or uninstall

**Product metrics (remote):**
- Opt out anytime with `STRUM_TELEMETRY=off`
- Delete your install ID: `rm ~/.strum/install_id`
- We cannot correlate your install ID to your identity

---

## Permissions Explained

### `tabs` Permission
**Why:** Track a specific browser tab
**What we DON'T do:** Track all tabs, read untracked tabs

### `storage` Permission
**Why:** Remember your settings across restarts
**What we store:** Tracked tab ID, toggle states, server URL

### `host_permissions` (localhost only)
**Why:** Send captured telemetry to your local Strum server
**What we DON'T do:** Access external websites, send data remotely

### Content Scripts (`<all_urls>`)
**Why:** Capture telemetry from any web app you're debugging
**What we do:** Inject into the ONE tab you explicitly track
**Why `<all_urls>`?** Strum is a developer tool — you need to debug apps on any domain.

---

## Compliance

### GDPR (EU)
- **Browser data:** Compliant — never leaves your device
- **Product metrics:** Compliant — no personal data collected. Random install ID is not PII (not linkable to a natural person). No IP addresses stored.
- **Right to deletion:** `rm ~/.strum/install_id` + `STRUM_TELEMETRY=off`
- **Right to access:** All local data available via localhost API

### CCPA (California)
- No sale of personal information
- No personal data collected in product metrics

---

## Open Source

Everything is auditable:

- **Telemetry beacon code:** [internal/telemetry/beacon.go](https://github.com/brennhill/Strum-AI-Devtools/blob/UNSTABLE/internal/telemetry/beacon.go)
- **Install ID generator:** [internal/telemetry/install_id.go](https://github.com/brennhill/Strum-AI-Devtools/blob/UNSTABLE/internal/telemetry/install_id.go)
- **Telemetry endpoint:** [github.com/brennhill/strum-analytics](https://github.com/brennhill/strum-analytics) (the Cloudflare Worker that receives beacons)
- **Extension manifest:** [extension/manifest.json](https://github.com/brennhill/Strum-AI-Devtools/blob/UNSTABLE/extension/manifest.json)
- **Redaction logic:** [extension/lib/serialize.js](https://github.com/brennhill/Strum-AI-Devtools/blob/UNSTABLE/extension/lib/serialize.js)

**Don't take our word for it.** Read the source. The telemetry code is ~80 lines of Go. Every beacon call site is searchable with `grep -rn 'BeaconEvent\|BeaconError'`.

---

## Changes to This Policy

We'll notify users via:
- GitHub release notes
- Extension update notes
- This page (check "Last updated" date)

---

## Contact

**Questions about privacy?**
- GitHub Issues: [github.com/brennhill/Strum-AI-Devtools/issues](https://github.com/brennhill/Strum-AI-Devtools/issues)
- Email: [privacy@usestrum.dev](mailto:privacy@usestrum.dev)
