---
title: "How to Connect Gasoline to OpenAI Codex"
description: "Beginner guide to connect OpenAI Codex with Gasoline Agentic Devtools and run your first browser-aware workflow."
date: 2026-03-31
authors: [brenn]
tags: [beginner, codex, openai, mcp, setup]
last_verified_version: 0.8.1
last_verified_date: 2026-03-31
normalized_tags: ['beginner', 'codex', 'openai', 'mcp', 'setup', 'articles', 'connect', 'gasoline']
---

OpenAI Codex is excellent for code changes. Gasoline makes Codex workflows browser-aware.

Here is the fastest setup path.

<!-- more -->

## Quick Terms

- **MCP (Model Context Protocol):** Connects Codex to external tools. https://modelcontextprotocol.io/specification/
- **AGENTS.md:** Project-level instruction file used by Codex in your repository.

## Step 1: Confirm Gasoline command is available

```bash
npx -y gasoline-mcp --help
```

## Step 2: Add Gasoline as an MCP server in Codex

Add this to `~/.codex/config.toml`:

```toml
[mcp_servers.gasoline-browser-devtools]
command = "npx"
args = ["-y", "gasoline-mcp"]
```

## Step 3: Restart Codex

Restart so Codex reloads MCP servers.

## Step 4: Run your first runtime checks

```js
observe({what: "errors"})
observe({what: "network_bodies", status_min: 400})
```

## Step 5: Turn findings into a reproducible artifact

```js
generate({what: "reproduction"})
```

Now you have a repeatable baseline for debugging instead of one-off console checks.

## Image and Diagram Callouts

> [Image Idea] `~/.codex/config.toml` showing `mcp_servers.gasoline-browser-devtools`.

> [Diagram Idea] Codex prompt -> Gasoline observe/analyze -> fix + verification loop.
