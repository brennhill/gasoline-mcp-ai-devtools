---
title: "How to Connect Gasoline to Cursor"
description: "Beginner guide to connect Cursor with Strum AI DevTools and run your first browser-aware workflow."
date: 2026-03-05
authors: [brenn]
tags: [beginner, cursor, mcp, setup]
last_verified_version: 0.8.0
last_verified_date: 2026-03-06
normalized_tags: ['beginner', 'cursor', 'mcp', 'setup', 'articles', 'connect', 'gasoline']
---

Cursor is excellent for writing code. Gasoline makes Cursor workflows browser-aware.

Here is the fastest setup path.

<!-- more -->

## Quick Terms

- **MCP (Model Context Protocol):** Connects Cursor to external tools. https://modelcontextprotocol.io/specification/
- **Regression:** A bug that returns after being fixed.

## Step 1: Confirm Gasoline command is available

```bash
npx -y gasoline-mcp --help
```

## Step 2: Add Gasoline as an MCP server in Cursor

Use this config block:

```json
{
  "mcpServers": {
    "gasoline": {
      "command": "npx",
      "args": ["-y", "gasoline-mcp"]
    }
  }
}
```

## Step 3: Restart Cursor

Restart so Cursor reloads MCP servers.

## Step 4: Run your first runtime checks

```js
observe({what: "errors"})
observe({what: "network_bodies", status_min: 400})
```

## Step 5: Save the workflow for next time

```js
generate({what: "test", test_name: "first-cursor-gasoline-check"})
```

Now you have a repeatable baseline, not a one-off debugging session.

## Image and Diagram Callouts

> [Image Idea] Cursor MCP settings showing a connected `gasoline` server.

> [Diagram Idea] Cursor edit loop with runtime checks between each change.
