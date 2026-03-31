---
title: "Gasoline + OpenAI Codex"
description: "Configure Gasoline as an MCP server for OpenAI Codex. Give Codex access to browser console logs, network errors, and DOM state."
keywords: "OpenAI Codex MCP server, Codex browser errors, Codex debugging, Codex MCP integration"
permalink: /mcp-integration/codex/
header:
  overlay_image: /assets/images/hero-banner.png
  overlay_filter: 0.85
  excerpt: "Fuel OpenAI Codex with live browser data."
toc: true
toc_sticky: true
status: reference
last_reviewed: 2026-03-31
---

Gasoline is an open-source MCP server that gives OpenAI Codex access to browser console logs, network errors, exceptions, WebSocket events, and live DOM state.

## <i class="fas fa-book"></i> Repository Instructions

For Codex project-level instructions, use `AGENTS.md` in your repository root.

## <i class="fas fa-bolt"></i> Auto-Install

```bash
gasoline-mcp --install codex
```

## <i class="fas fa-file-code"></i> Manual Configuration

Add to `~/.codex/config.toml`:

```toml
[mcp_servers.gasoline-browser-devtools]
command = "npx"
args = ["-y", "gasoline-mcp"]
```

Optional per-tool approvals:

```toml
[mcp_servers.gasoline-browser-devtools.tools.observe]
approval_mode = "approve"

[mcp_servers.gasoline-browser-devtools.tools.analyze]
approval_mode = "approve"

[mcp_servers.gasoline-browser-devtools.tools.interact]
approval_mode = "approve"

[mcp_servers.gasoline-browser-devtools.tools.generate]
approval_mode = "approve"

[mcp_servers.gasoline-browser-devtools.tools.configure]
approval_mode = "approve"
```

If you installed a local binary, replace `command = "npx"` with the full binary path.

## <i class="fas fa-fire-alt"></i> Usage

After configuring, restart Codex and ask:

- _"What browser errors do you see?"_
- _"Check failed network requests for this page."_
- _"Run an accessibility audit and summarize issues."_

## <i class="fas fa-wrench"></i> Troubleshooting

1. **Restart Codex** after editing `config.toml`
2. **Verify extension popup** shows "Connected"
3. **Verify MCP tools** by asking Codex what tools are available
4. **Avoid duplicate servers** — stop manually started Gasoline before using MCP-managed mode
