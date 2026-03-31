---
title: Gasoline + OpenAI Codex
description: "Configure Gasoline as an MCP server for OpenAI Codex. Give Codex access to browser console logs, network errors, and DOM state."
last_verified_version: 0.8.1
last_verified_date: 2026-03-31
normalized_tags: ['mcp', 'integration', 'openai', 'codex']
---

Gasoline is an open-source MCP server that gives OpenAI Codex access to browser console logs, network errors, exceptions, WebSocket events, and live DOM state. Zero dependencies.

## Auto-Install

```bash
gasoline-mcp --install codex
```

## Manual Configuration

Add to `~/.codex/config.toml`:

```toml
[mcp_servers.gasoline-browser-devtools]
command = "npx"
args = ["-y", "gasoline-mcp"]
```

Optional per-tool approval mode:

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

If you installed a local binary, replace the `command` value with the full binary path.

## Usage

After configuring, restart Codex and ask:

- _"What browser errors do you see?"_
- _"Check failed network requests for this page."_
- _"Run an accessibility audit and summarize issues."_

## Troubleshooting

1. **Restart Codex** after editing `config.toml`
2. **Verify extension popup** shows "Connected"
3. **Check MCP visibility** by asking Codex which MCP tools are available
4. **Avoid duplicate servers** — stop manually started Gasoline processes before using MCP-managed mode
