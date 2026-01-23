---
title: "Privacy & Security"
description: "Gasoline is 100% local. No cloud, no analytics, no telemetry. Logs never leave your machine. Auth headers are automatically stripped."
keywords: "local only debugging, privacy first developer tools, localhost debugging tool, no telemetry developer tool, secure browser debugging"
permalink: /privacy/
header:
  overlay_image: /assets/images/hero-banner.png
  overlay_filter: 0.85
  excerpt: "Your fuel stays in your tank — 100% local, zero telemetry."
toc: true
toc_sticky: true
---

Gasoline is designed with privacy as a core principle, not an afterthought.

## <i class="fas fa-home"></i> 100% Local

- **Logs never leave your machine** — everything stays on localhost
- **No cloud services** — no accounts, no sign-ups, no data uploads
- **No analytics** — zero telemetry, zero tracking
- **No network calls** — the server binds to `127.0.0.1` only

## <i class="fas fa-shield-alt"></i> Sensitive Data Protection

- **Authorization headers stripped** — tokens, API keys, and bearer tokens are automatically removed from captured network logs
- **No cookie capture** — cookies are not included in log entries
- **No form values by default** — input values in user actions are redacted unless explicitly enabled

## <i class="fas fa-lock"></i> Localhost Only

The Gasoline server binds exclusively to `127.0.0.1`:

- Not accessible from your local network
- Not accessible from the internet
- Other devices on your WiFi cannot reach it
- Firewall rules are not required

## <i class="fab fa-github"></i> Open Source

The entire codebase is open source under AGPL-3.0:

- **Audit the code** — verify exactly what gets captured and where it goes
- **Build from source** — compile the Go binary yourself
- **No obfuscation** — extension code is vanilla JavaScript, readable in Chrome DevTools

## <i class="fas fa-recycle"></i> Data Lifecycle

1. Browser extension captures events in-page
2. Events are sent to `localhost:7890` via HTTP POST
3. Server appends entries to a local JSONL file
4. Your AI tool reads the file via MCP (stdio, not network)
5. Log rotation removes old entries automatically

At no point does data leave your machine.

## <i class="fas fa-key"></i> Extension Permissions

The Chrome extension requests only the minimum permissions needed:

- **activeTab** — to inject capture scripts into the current tab
- **storage** — to persist extension settings locally
- **Host permission (localhost)** — to communicate with the local server

No permissions for browsing history, bookmarks, downloads, or cross-origin requests.
