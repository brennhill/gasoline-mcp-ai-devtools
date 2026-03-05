---
title: "How to Connect Gasoline to Claude Code"
description: "A step-by-step beginner guide to connect Claude Code with Gasoline Agentic Devtools using MCP."
date: 2026-03-05
authors: [brenn]
tags: [beginner, claude-code, mcp, setup]
---

Want Claude Code to understand what is happening in the browser, not just in your source files?

This is the setup.

<!-- more -->

## Quick Terms

- **MCP (Model Context Protocol):** Standard for connecting AI assistants to tools. https://modelcontextprotocol.io/specification/
- **JSON (JavaScript Object Notation):** Text format used for tool config.

## Step 1: Confirm Gasoline is installed

```bash
npx -y gasoline-mcp --help
```

If this command works, you are ready to connect.

## Step 2: Add Gasoline in Claude Code MCP config

Add this server entry:

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

## Step 3: Restart Claude Code

Reload so it picks up the new MCP server configuration.

## Step 4: Run a first browser-aware check

```js
observe({what: "errors"})
observe({what: "network_waterfall", status_min: 400})
```

## Step 5: Turn findings into action

```js
generate({what: "reproduction"})
```

Now Claude Code can help with bug triage using runtime evidence.

## Image and Diagram Callouts

> [Image Idea] Claude Code MCP config panel with the `gasoline` block highlighted.

> [Diagram Idea] Claude Code prompt -> Gasoline observe/analyze -> fix suggestion loop.
