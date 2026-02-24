---
doc_type: legacy_doc
status: reference
last_reviewed: 2026-02-16
---

# Reddit Post: Gasoline MCP Introduction

**Subreddit:** r/LocalLLaMA, r/ProgrammingTools, r/webdev, r/devtools

**Title Options:**

1. **I built a browser observability tool for AI coding agents—here's why**
2. **Browser observability for AI: Give Claude/Cursor real-time browser visibility**
3. **Debug AI-generated code in seconds, not minutes: Gasoline MCP**

---

## Body Content

```
I've been working on Gasoline MCP, a browser observability tool that gives AI coding agents like Claude Code and Cursor real-time visibility into browser activity.

The problem I was trying to solve:
When AI generates code that doesn't work, debugging is painful. You have to switch between your AI tool and browser dev tools, copy error messages, and manually explain what's happening.

How Gasoline MCP works:
- Chrome extension captures console logs, network errors, WebSocket events, and user actions
- MCP server forwards this data to your AI assistant in real-time
- Your AI can now "see" the browser and suggest fixes autonomously

Key features that make it different:
- No debug port required (standard extension = no security tradeoff)
- Single Go binary (zero runtime dependencies)
- Captures WebSocket messages, full request/response bodies, user action recording
- Works with Claude Code, Cursor, Windsurf, Zed, and any MCP-compatible tool
- 100% local, no cloud, no telemetry

How to get started:
```bash
npx gasoline-mcp@6.0.0
```
Then download the extension and add to your MCP config.

Looking for feedback from the community! What would make this more useful for your AI development workflow?

Link: https://github.com/brennhill/gasoline-mcp-ai-devtools
Docs: https://cookwithgasoline.com
```

---

## Posting Guidelines

### r/LocalLLaMA
- Best fit for AI/LLM audience
- Emphasize MCP integration
- Focus on AI debugging workflow

### r/ProgrammingTools
- Focus on the tool aspect
- Highlight technical features
- Compare with other tools

### r/webdev
- Focus on browser debugging
- Web development use cases
- WebSocket capture feature

### r/devtools
- Technical audience
- Deep-dive on features
- Architecture discussion

---

## Tips for Success

1. **Engage with comments** - Reply to every comment within 24 hours
2. **Answer questions** - Be helpful and thorough
3. **Share examples** - If someone asks for use cases, share specific examples
4. **Follow up** - Post updates in comments as you develop new features
5. **Cross-reference** - If posting in multiple subreddits, mention it

---

## Timing

**Best times to post:**
- Weekdays: 9-11 AM EST or 2-4 PM EST
- Weekends: 10 AM - 12 PM EST

**Avoid:**
- Late night (11 PM - 6 AM EST)
- Early morning (6-8 AM EST)

---

## Follow-up Posts

If the initial post gets traction, consider follow-ups:

1. **"How Gasoline MCP captures WebSocket messages"** - Technical deep-dive
2. **"Debugging AI-generated code: Before and After Gasoline"** - Real example
3. **"Gasoline vs Puppeteer: Why extensions win"** - Comparison

---

## Common Questions to Prepare For

**Q: Is this free?**
A: Yes! Open source and free to use (AGPL-3.0 license)

**Q: Does it work with [my AI tool]?**
A: If it supports MCP, yes. Works with Claude Code, Cursor, Windsurf, Zed, Claude Desktop, VS Code + Continue.

**Q: Is my data sent to the cloud?**
A: No. 100% local, no telemetry, no cloud.

**Q: Why not use Chrome DevTools?**
A: DevTools shows data but doesn't integrate with AI. Gasoline bridges that gap.

**Q: What's the difference between this and [other tool]?**
A: Gasoline is specifically designed for AI integration, captures unique data (WebSocket, full bodies), and uses a secure extension approach.

---

## Monitoring

After posting, track:
- Upvotes
- Comments
- Clicks to GitHub/website
- New GitHub stars
- New Slack members

Use this data to refine future posts.
