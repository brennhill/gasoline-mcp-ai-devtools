---
doc_type: legacy_doc
status: reference
last_reviewed: 2026-02-16
---

# Video Script: Gasoline MCP - Quick Start Demo

**Title:** Get Gasoline MCP Running in 2 Minutes  
**Duration:** 2:00-2:30  
**Format:** Screen recording with voiceover  
**Target:** Developers new to Gasoline MCP

---

## Script

### [0:00-0:10] Intro

**Visual:** Gasoline logo animation + title screen

**Voiceover:**
"Gasoline MCP gives your AI coding agents real-time visibility into browser activity. Let's get it running in under two minutes."

### [0:10-0:30] Step 1: Install Extension

**Visual:** 
- Show cookwithgasoline.com/downloads page
- Download CRX file
- Open chrome://extensions
- Enable Developer mode
- Drag and drop CRX

**Voiceover:**
"First, download the Chrome extension from cookwithgasoline.com/downloads. Open chrome://extensions, enable Developer mode, and drag the CRX file to install it."

### [0:30-0:50] Step 2: Start MCP Server

**Visual:**
- Open terminal
- Type: `npx gasoline-mcp@6.0.0`
- Show server starting up
- Show health check: `curl http://localhost:7890/health`

**Voiceover:**
"Now start the MCP server. Just run `npx gasoline-mcp@6.0.0`. The server starts automatically on port 7890. You can verify it's running with a quick health check."

### [0:50-1:10] Step 3: Configure AI Tool

**Visual:**
- Show Claude Desktop settings
- Open MCP config file
- Add Gasoline configuration
- Show JSON config

**Voiceover:**
"Add Gasoline to your AI tool's MCP config. For Claude Desktop, add this JSON configuration. The same pattern works for Cursor, Windsurf, Zed, and any MCP-compatible tool."

### [1:10-1:30] Step 4: Test It Out

**Visual:**
- Open a test website
- Show Gasoline icon in browser
- Click to open popup
- Show captured console logs
- Show network requests

**Voiceover:**
"Open any website and you'll see Gasoline capturing data in real-time. Console logs, network requests, WebSocket events—it's all there."

### [1:30-1:50] Step 5: Use with AI

**Visual:**
- Open Claude Code
- Ask Claude to debug an issue
- Show Claude using Gasoline tools
- Show Claude seeing browser data
- Show Claude suggesting a fix

**Voiceover:**
"Now ask your AI assistant to debug an issue. Claude can see everything happening in the browser and suggest fixes in real-time. No more switching between windows or copying error messages."

### [1:50-2:00] Outro

**Visual:** 
- Show cookwithgasoline.com
- Show GitHub repo
- Show Discord link

**Voiceover:**
"That's it! Gasoline MCP is running and your AI assistant can now see browser telemetry. Check the links below for documentation, or join our Discord community. Happy debugging!"

---

## Production Notes

### Screen Recording Tips
- Use 1080p or 4K resolution
- Record at 60fps for smooth animations
- Use a clean desktop (close unnecessary apps)
- Use system font for terminal/code
- Highlight cursor during clicks

### Audio Tips
- Use a quality microphone
- Record in a quiet environment
- Speak clearly and at moderate pace
- Add background music (optional, low volume)

### Editing Tips
- Add zoom effects for important UI elements
- Use callouts to highlight key steps
- Add text overlays for commands and URLs
- Include chapter markers for easy navigation

### Visual Assets Needed
- Gasoline logo (PNG/SVG)
- cookwithgasoline.com screenshots
- Chrome extension icon
- AI tool logos (Claude, Cursor, Windsurf, Zed)
- Discord logo

### Call to Action
- Subscribe to channel
- Like the video
- Comment with questions
- Share with developer friends
- Join Discord community

### SEO Keywords
- Gasoline MCP
- AI debugging tools
- Browser observability
- Claude Code tutorial
- MCP setup guide
- AI coding assistant

### Related Videos
- How Gasoline Captures WebSocket Messages
- Debugging AI-Generated Code with Gasoline
- Gasoline vs Puppeteer Comparison

---

## Alternative Short Version (30 seconds)

### [0:00-0:05] Hook
"Debug AI-generated code in seconds, not minutes. Here's how."

### [0:05-0:15] Quick Demo
[Fast montage of: download extension → start server → configure → debug]

### [0:15-0:20] CTA
"Get Gasoline MCP at cookwithgasoline.com. Link in description."

### [0:20-0:30] Outro
"Subscribe for more AI devtools tutorials!"

---

## Social Media Cut (15 seconds)

### [0:00-0:05] Hook
"Your AI assistant can now see your browser."

### [0:05-0:10] Demo
[Quick screen capture showing Claude debugging via Gasoline]

### [0:10-0:15] CTA
"Get Gasoline MCP. Link in bio."

---

## Thumbnail Ideas

1. Split screen: Left = AI assistant, Right = Browser dev tools
2. "Debug AI Code in 2 Minutes" with clock icon
3. Gasoline logo + "Browser Observability for AI"
4. Before/After: "Manual Debugging" vs "AI Debugging"

---

## Description Template

```
Get Gasoline MCP running in under 2 minutes! 🚀

Gasoline MCP gives your AI coding agents (Claude Code, Cursor, Windsurf, Zed) real-time visibility into browser activity—console logs, network errors, WebSocket events, and more.

In this video, you'll learn:
✅ How to install the Chrome extension
✅ How to start the MCP server
✅ How to configure your AI tool
✅ How to debug with AI assistance

📦 Download: https://cookwithgasoline.com/downloads/
📖 Docs: https://cookwithgasoline.com
💻 GitHub: https://github.com/brennhill/gasoline-mcp-ai-devtools
💬 Discord: [LINK]

Timestamps:
0:00 - Intro
0:10 - Install Extension
0:30 - Start MCP Server
0:50 - Configure AI Tool
1:10 - Test It Out
1:30 - Use with AI
1:50 - Outro

#GasolineMCP #AI #DevTools #ClaudeCode #BrowserObservability
```

---

## Hashtags

#GasolineMCP #AI #DevTools #ClaudeCode #Cursor #Windsurf #Zed #BrowserObservability #MCP #AIAssistedDevelopment #Debugging #OpenSource

---

## Notes for First Recording

1. **Practice the flow** - Run through the steps a few times before recording
2. **Test everything** - Make sure all commands work and URLs are correct
3. **Have a test site ready** - Use a simple site with console logs and network requests
4. **Prepare AI conversation** - Have a ready-to-use debugging scenario
5. **Check audio levels** - Test microphone and adjust gain
6. **Clear browser cache** - Start with a clean Chrome profile for recording
