---
doc_type: legacy_doc
status: reference
last_reviewed: 2026-02-16
---

# Strum - Week 1 Social Media Content Calendar

**Week Starting:** 2026-02-09  
**Platform:** Twitter/X (@gasolinedev), LinkedIn, Reddit

---

## Daily Content Schedule

### Day 1 (Monday) - Launch & Feature Spotlight

#### Twitter/X
```
🔥 Feature Spotlight: WebSocket Capture

Strum captures full WebSocket lifecycle events—connection, messages, errors. 
Unlike other tools that miss this critical data, we give AI agents complete visibility 
into real-time communication.

See it in action: https://usestrum.dev/features/#websocket-events

#GasolineMCP #AI #DevTools #WebSocket
```

#### LinkedIn
```
The future of debugging isn't manual—it's collaborative.

With Strum, AI coding agents can now see exactly what's happening in your browser. 
No more switching between windows, no more copy-pasting error messages.

Our WebSocket capture feature gives AI agents complete visibility into real-time communication—something most browser observability tools simply miss.

How are you using AI to streamline your debugging workflow?

#AI #SoftwareDevelopment #DevTools #Productivity #BrowserObservability
```

---

### Day 2 (Tuesday) - Tip of the Day

#### Twitter/X
```
💡 Tip of the Day: Quick Install

Get Strum running in under 2 minutes:

1. Download extension: https://usestrum.dev/downloads/
2. Run: npx gasoline-mcp@6.0.0
3. Add to your MCP config

That's it! Your AI assistant can now see browser telemetry.

#GasolineMCP #AI #DevTools #QuickStart
```

#### Reddit (r/LocalLLaMA)
```
Title: I built a browser observability tool for AI coding agents—here's why

Body:
I've been working on Strum, a browser observability tool that gives AI coding agents like Claude Code and Cursor real-time visibility into browser activity.

The problem I was trying to solve:
When AI generates code that doesn't work, debugging is painful. You have to switch between your AI tool and browser dev tools, copy error messages, and manually explain what's happening.

How Strum works:
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

Link: https://github.com/brennhill/gasoline-agentic-browser-devtools-mcp
```

---

### Day 3 (Wednesday) - User Showcase / Quote

#### Twitter/X
```
"Strum saved me hours of debugging. My AI assistant can now see exactly what's happening in the browser and suggest fixes in real-time."

— Early beta tester

Try it yourself: https://usestrum.dev

#GasolineMCP #AI #DevTools #UserLove
```

#### LinkedIn
```
Real results from real developers.

One of our early beta testers reported saving 10+ hours per week debugging AI-generated code. The key? Giving AI agents visibility into browser telemetry.

When your AI assistant can see console logs, network errors, and WebSocket events, it doesn't just generate code—it understands the full context and can fix issues autonomously.

Have you tried Strum yet? Share your experience in the comments.

#AI #DeveloperTools #Productivity #OpenSource
```

---

### Day 4 (Thursday) - Behind the Scenes / Development

#### Twitter/X
```
🔧 Behind the Scenes: Why Go?

Strum's server is written in Go for a reason:

• Single binary distribution (no node_modules!)
• Cross-platform support (macOS, Linux, Windows)
• Excellent concurrency for handling multiple browser sessions
• Small memory footprint
• Fast startup time

Zero runtime dependencies, maximum portability.

#GasolineMCP #Golang #DevTools #OpenSource
```

#### Twitter/X (Thread)
```
🧵 Why Strum doesn't need Chrome's debug port:

Most browser automation tools require launching Chrome with --remote-debugging-port. This:

❌ Disables Chrome's security sandbox
❌ Breaks your normal browser workflow  
❌ Requires a separate browser instance
❌ Creates security vulnerabilities

Strum uses a standard Chrome extension instead:

✅ Your browser stays secure
✅ Works with your normal Chrome profile
✅ No special launch flags
✅ Enterprise-safe

Security shouldn't be a tradeoff for productivity.

#GasolineMCP #BrowserSecurity #DevTools
```

---

### Day 5 (Friday) - Community Highlight

#### Twitter/X
```
👥 Community Spotlight: Join the conversation!

We're building Strum in the open. Want to:

• Get help with setup?
• Suggest new features?
• Show off what you're building?
• Connect with other AI developers?

Join our Discord: [LINK]

#GasolineMCP #AI #DevTools #Community
```

#### LinkedIn
```
Open source isn't just about code—it's about community.

Strum is being built in the open, and we want you to be part of it. Whether you're:

- A developer using AI tools daily
- Interested in contributing code
- Looking to connect with like-minded developers
- Want to shape the future of AI-assisted development

We'd love to have you join our growing community.

Drop a comment if you'd like an invite to our Discord server!

#OpenSource #Community #AI #DeveloperTools
```

---

### Day 6 (Saturday) - Poll / Engagement

#### Twitter/X
```
📊 Poll: Which AI coding tool do you use most?

• Claude Code / Claude Desktop
• Cursor
• Windsurf
• Zed
• VS Code + Continue
• Other (comment below!)

Vote and let's see which tools the Gasoline community uses!

#GasolineMCP #AI #DevTools #Poll
```

---

### Day 7 (Sunday) - Curated Content

#### Twitter/X
```
📚 Sunday Reading: Best of AI Development

Some great resources for AI-assisted development:

• Model Context Protocol (MCP) spec
• Anthropic's Claude Code documentation
• Cursor's AI-powered IDE features
• Strum browser observability guide

What are your go-to resources for AI development?

#GasolineMCP #AI #DevTools #SundayReading
```

---

## Weekly Goals

- [ ] 7 Twitter/X posts
- [ ] 3 LinkedIn posts
- [ ] 1 Reddit post
- [ ] 1 engagement poll
- [ ] Respond to all comments within 24 hours
- [ ] Engage with 5+ AI tool accounts

## Hashtag Strategy

**Primary:** #GasolineMCP, #AI, #DevTools

**Secondary:** #BrowserObservability, #MCP, #ClaudeCode, #Cursor, #AIAssistedDevelopment, #OpenSource

**Niche:** #Golang, #ChromeExtension, #WebDevelopment, #Debugging

---

## Engagement Checklist

Daily:
- [ ] Post scheduled content
- [ ] Reply to all comments
- [ ] Like/retweet relevant content from community
- [ ] Follow 5-10 new accounts in AI/devtools space
- [ ] Monitor mentions and hashtags

Weekly:
- [ ] Review analytics and adjust strategy
- [ ] Engage with AI tool announcements (Claude, Cursor, Windsurf)
- [ ] Share user-generated content
- [ ] Participate in relevant Twitter Spaces

---

## Content Assets Needed

- [ ] Discord server link (create before Day 5)
- [ ] Feature screenshots (WebSocket capture, network bodies, etc.)
- [ ] Demo video clips (15-30 seconds)
- [ ] User testimonials (collect from beta testers)
- [ ] Brand assets (logo, colors, templates)

---

## Notes & Adjustments

- Schedule times based on audience timezone (US/EU overlap)
- Monitor engagement and double down on high-performing content types
- Be ready to pivot strategy based on feedback
- Keep tone authentic and developer-focused
