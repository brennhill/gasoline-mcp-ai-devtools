---
title: "Why Chrome DevTools MCP Isn't Enough"
date: 2026-02-07
authors:
  - brenn
tags:
  - comparison
  - architecture
---

Chrome DevTools MCP was a great first step. But it doesn't capture WebSockets, can't handle distributed apps, breaks on advanced frameworks, and slows down the development cycle it was supposed to accelerate.

<!-- more -->

## Credit Where It's Due

Chrome DevTools MCP proved something important: AI agents are dramatically better when they can see the browser. That insight changed how developers think about AI-assisted development.

But the implementation has fundamental limitations that surface quickly in real-world projects. If you've tried using DevTools MCP on anything beyond a simple SPA, you've probably hit them.

## Problem 1: The Debug Port Kills Your Security

Chrome DevTools MCP requires launching Chrome with `--remote-debugging-port`. This flag:

- **Disables Chrome's security sandboxing.** The sandbox is Chrome's primary defense against malicious websites. Turning it off means any site you visit during development can access more of your system.
- **Exposes a network port.** Port 9222 accepts remote connections. On a shared network (office, coffee shop, conference WiFi), that's an attack surface.
- **Breaks your normal browser.** You need a special browser launch. Your extensions, bookmarks, and sessions from your regular Chrome instance aren't there. You're working in an unfamiliar environment.

Gasoline uses a standard Chrome extension (Manifest V3). No special launch flags. No exposed ports. Your browser stays secure, and you work in your normal environment with your normal sessions.

## Problem 2: WebSockets Are Invisible

Modern applications are real-time. Chat apps, collaborative editors, dashboards, notification systems, trading platforms — they all use WebSockets.

Chrome DevTools MCP doesn't capture WebSocket messages.

That means your AI can't see:
- What the server is pushing to the client
- Out-of-order messages causing state corruption
- Payload format mismatches (server sends `txt`, client expects `text`)
- Connection drops and failed reconnections
- Authentication token expiration on long-lived connections

With Gasoline:

```js
observe({what: "websocket_status"})    // Active connections
observe({what: "websocket_events"})    // Message stream
```

Every frame, every direction, every connection — captured automatically and queryable by your AI.

## Problem 3: Distributed Applications Break It

Real applications aren't one tab. They're:

- **A customer app** that talks to **an admin panel** that reads from **a shared API**
- **A web app** that authenticates via **an OAuth provider** and fetches data from **a third-party service**
- **A frontend** that sends events to **a message queue** that triggers **a background worker** that updates **a dashboard**

Chrome DevTools MCP gives you one browser tab's console output. It has no concept of cross-tab workflows, multi-service architectures, or the network calls that tie them together.

Gasoline captures the full picture:

- **Network bodies** show exactly what your app sent and what the API returned
- **WebSocket events** show real-time communication between services
- **Multi-tab awareness** means you can observe activity across tabs
- **Timeline** interleaves all events chronologically, so you see the full distributed flow

When your AI can see that Tab A's API call returned a stale token, which caused Tab B's WebSocket to disconnect, which triggered the error the user reported — that's when debugging gets fast.

## Problem 4: It Fails on Constantly Changing UIs

Development moves fast. The UI changes every sprint — new components, renamed classes, restructured layouts. DevTools MCP gives your AI console logs and a DOM snapshot. The AI has to _ask_ you what changed and _guess_ at selectors.

Gasoline's `interact` tool uses semantic selectors that adapt:

```js
interact({action: "click", selector: "text=Submit"})
interact({action: "type", selector: "label=Email", text: "user@example.com"})
interact({action: "list_interactive"})  // Discover all elements
```

When the UI changes, `text=Submit` still finds the submit button. `label=Email` still finds the email field. And if the AI is unsure, it calls `list_interactive` to get a full inventory of every clickable and typeable element on the page.

DevTools MCP can't interact with the page at all. Gasoline lets the AI click, type, navigate, and verify — the full development cycle in one tool.

## Problem 5: It Doesn't Actually Accelerate Development

The promise of browser MCP tools is faster development cycles. But DevTools MCP only gives the AI _some_ of the data. The developer still has to:

1. Copy-paste error details the AI can't see
2. Describe the visual state ("the button is greyed out")
3. Manually check network responses
4. Explain the WebSocket behavior
5. Reproduce the issue step by step

You're still the bottleneck. You're still shuttling context between the browser and the AI.

Gasoline gives the AI _everything_:

| Data | DevTools MCP | Gasoline |
|---|---|---|
| Console errors | Yes | Yes, with deduplication and clustering |
| Network requests | Partial | Full bodies, filtered by URL/status |
| WebSocket messages | No | Full capture with filtering |
| Screenshots | No | Yes |
| User actions | No | Recorded automatically |
| Web Vitals | No | LCP, CLS, INP, FCP with regression detection |
| Accessibility | No | WCAG audits |
| API schemas | No | Auto-inferred from traffic |
| Page interaction | No | Click, type, navigate, verify |

When the AI has the full picture, it doesn't need you to be the intermediary. It observes, diagnoses, and fixes — at the speed of API calls, not the speed of copy-paste.

## Problem 6: Production Dependencies and Supply Chain Risk

Chrome DevTools MCP and BrowserTools MCP require Node.js and npm packages. That's:

- A runtime dependency (Node.js must be installed)
- Package manager overhead (npm/yarn, lock files, version conflicts)
- Supply chain exposure (every dependency is a potential vulnerability)

Gasoline is a single Go binary. Zero production dependencies. No `node_modules`. No supply chain risk.

```bash
npx gasoline-mcp  # Downloads single binary, runs it
```

## The Real Problem DevTools MCP Doesn't Solve

The bottleneck in modern development isn't "the AI can't see console errors." It's "the AI can't see _enough_ to work autonomously."

DevTools MCP gives the AI a partial view — console output and DOM snapshots. That's better than nothing, but it still leaves the developer as the primary context provider.

Gasoline gives the AI a complete view — errors, network, WebSockets, performance, accessibility, visual state, and browser control. The AI becomes a full participant in the development cycle: observe the bug, understand the context, interact with the app, verify the fix.

That's the difference between "AI that helps you debug" and "AI that debugs."

## Side-by-Side Summary

| | Chrome DevTools MCP | Gasoline MCP |
|---|---|---|
| **Setup** | `--remote-debugging-port` flag | Standard extension |
| **Security** | Sandbox disabled | Full sandbox preserved |
| **Console errors** | Yes | Yes + dedup + clustering + bundles |
| **Network bodies** | No | Full request/response capture |
| **WebSocket** | No | Full capture and filtering |
| **Browser control** | No | Click, type, navigate, verify |
| **Screenshots** | No | Yes |
| **Web Vitals** | No | LCP, CLS, INP, FCP |
| **Accessibility** | No | WCAG audits + SARIF export |
| **Test generation** | No | Playwright tests from sessions |
| **Multi-client** | Single connection | Unlimited concurrent clients |
| **Dependencies** | Node.js + npm | Zero (single Go binary) |
| **Privacy** | Local | Local, 127.0.0.1 only |
| **Overhead** | ~5ms per intercept | < 0.1ms per intercept |

Chrome DevTools MCP was the right idea at the right time. Gasoline is what comes next.
