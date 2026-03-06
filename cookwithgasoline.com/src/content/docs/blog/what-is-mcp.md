---
title: "What Is MCP? The Model Context Protocol Explained for Web Developers"
date: 2026-02-07
authors: [brenn]
tags: [mcp, ai-development, explainer]
---

MCP — the Model Context Protocol — is the USB-C of AI tools. It's a standard that lets AI assistants plug into external data sources and capabilities without custom integrations. If you've ever wished your AI coding assistant could *see* your browser, *read* your database, or *check* your CI pipeline, MCP is how that works.

Here's what MCP means for web developers and why it changes how you build software.

<!-- more -->

## The Problem MCP Solves

AI coding assistants are powerful but blind. They can read your source code, but they can't see:

- The runtime error in your browser console
- The 500 response from your API
- The layout shift that happens after your component mounts
- The WebSocket connection that silently drops
- The third-party script that's loading slowly

Without this context, every debugging session starts with you *describing* the problem to the AI instead of the AI *observing* it directly. You become a human copy-paste bridge between your browser and your terminal.

MCP eliminates that bridge.

## How MCP Works

MCP is a JSON-RPC 2.0 protocol with a simple contract:

1. **Servers** expose **tools** (functions the AI can call) and **resources** (data the AI can read)
2. **Clients** (AI assistants like Claude Code, Cursor, Windsurf) discover and invoke those tools
3. **Transport** is flexible — stdio pipes, HTTP, or any bidirectional channel

A typical MCP server might expose tools like:

```
observe({what: "errors"})        → returns browser console errors
generate({format: "test"})       → generates a Playwright test
configure({action: "health"})    → returns server status
interact({action: "click", selector: "text=Submit"})  → clicks a button
```

The AI assistant discovers what tools are available, reads their descriptions, and calls them as needed during a conversation. No custom plugin architecture. No vendor-specific API. Just a protocol.

## Why MCP Matters for Web Development

### Your AI Can See What You See

Before MCP, debugging with AI looked like this:

> *You:* "I'm getting an error when I submit the form."
> *AI:* "What error? Can you paste the console output?"
> *You:* [switches to browser, opens DevTools, copies error, pastes]
> *AI:* "Can you also show me the network request?"
> *You:* [switches to Network tab, finds request, copies, pastes]

With an MCP server like Gasoline connected:

> *You:* "I'm getting an error when I submit the form."
> *AI:* [calls observe({what: "errors"})] "I can see the TypeError. The API returned a 422 because the email field is missing from the request body. Let me check the form handler..."

The AI skips the back-and-forth and goes straight to diagnosing.

### Tool Composition

MCP tools compose naturally. An AI assistant with a browser MCP server and a filesystem MCP server can:

1. **Observe** a runtime error in the browser
2. **Read** the relevant source file
3. **Edit** the code to fix the bug
4. **Refresh** the browser
5. **Verify** the error is gone

That's a complete debugging loop without human intervention beyond the initial request.

### Works With Any AI Tool

Because MCP is a standard protocol, the same server works with every compatible client:

| AI Tool | MCP Support |
|---------|------------|
| Claude Code | Built-in |
| Cursor | Built-in |
| Windsurf | Built-in |
| Claude Desktop | Built-in |
| Zed | Built-in |
| VS Code + Continue | Plugin |

You configure the server once. Every AI tool that speaks MCP can use it.

## The MCP Ecosystem

MCP servers exist for many data sources:

| Category | Examples |
|----------|---------|
| **Browser** | Gasoline (real-time telemetry, browser control) |
| **Filesystem** | Read, write, search files |
| **Databases** | PostgreSQL, SQLite, MongoDB |
| **APIs** | GitHub, Slack, Jira, Linear |
| **DevOps** | Docker, Kubernetes, CI/CD |
| **Search** | Brave Search, web fetch |

The power comes from combining them. A browser MCP server plus a GitHub MCP server means your AI can observe a bug, fix it, and open a PR — all in one conversation.

## What Makes a Good Browser MCP Server

Not all browser MCP servers are equal. The critical capabilities for web development:

### Real-Time Observability

The server should capture browser state as it happens — console logs, network errors, exceptions, WebSocket events — not just static snapshots. When you're debugging a race condition, you need the sequence of events, not a point-in-time dump.

### Browser Control

Observation alone isn't enough. The AI needs to navigate, click, type, and interact with the page. Otherwise it's reading but not testing. Semantic selectors (`text=Submit`, `label=Email`) are more resilient than CSS selectors that break with every redesign.

### Artifact Generation

Captured session data should translate into useful outputs: Playwright tests, reproduction scripts, accessibility reports, performance summaries. The AI has the data — let it produce the artifacts.

### Security by Design

A browser MCP server sees everything — network traffic, form inputs, cookies. It must:
- Strip credentials before storing or transmitting data
- Bind to localhost only (no network exposure)
- Minimize permissions (no broad host access)
- Keep all data on the developer's machine

### Performance Awareness

Web Vitals, resource timing, long tasks, layout shifts — performance data should flow alongside error data. The AI shouldn't need a separate tool to check if the page is fast.

## Getting Started with MCP

If you want to add browser observability to your AI workflow:

### 1. Install the Extension

```bash
git clone https://github.com/brennhill/gasoline-mcp-ai-devtools.git
```

Load the `extension/` folder as an unpacked Chrome extension.

### 2. Configure Your AI Tool

Add to your MCP config (example for Claude Code's `.mcp.json`):

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

### 3. Start Debugging

Open your app, restart your AI tool, and ask:

> "What browser errors do you see?"

The AI calls `observe({what: "errors"})`, gets the real-time error list, and starts diagnosing. No copy-paste. No screenshots. No description of the problem. The AI sees it directly.

## The Bigger Picture

MCP is still early. The protocol is evolving, new servers appear weekly, and AI tools are deepening their integration. But the direction is clear: AI assistants are becoming aware of their environment, not just their context window.

For web developers, this means the feedback loop between writing code and seeing results gets tighter. The AI sees the browser. The AI sees the error. The AI sees the fix work. All in real time.

That's what MCP enables. And it's just getting started.
