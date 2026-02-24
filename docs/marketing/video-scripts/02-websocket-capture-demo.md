---
doc_type: legacy_doc
status: reference
last_reviewed: 2026-02-16
---

# Video Script: WebSocket Capture - See What Others Can't

**Title:** WebSocket Capture - See What Other Browser Tools Miss  
**Duration:** 3:00-3:30  
**Format:** Screen recording with voiceover  
**Target:** Developers working with real-time applications

---

## Script

### [0:00-0:15] Intro - The Problem

**Visual:** 
- Split screen: Left = WebSocket app (chat/dashboard), Right = Chrome DevTools
- Show DevTools missing WebSocket messages
- Show frustration indicator (red X, shaking cursor)

**Voiceover:**
"WebSocket debugging is frustrating. Chrome DevTools shows you messages, but you can't query them programmatically, you can't correlate them with other events, and your AI assistant definitely can't see them."

### [0:15-0:30] The Solution

**Visual:**
- Gasoline logo animation
- Show WebSocket icon
- Show "Full Lifecycle Capture" text

**Voiceover:**
"Gasoline MCP captures the complete WebSocket lifecycle—connections, messages, errors, disconnections—and makes it all available to your AI coding assistant."

### [0:30-1:00] Demo: Real-Time Chat App

**Visual:**
- Open a real-time chat application
- Open Gasoline popup
- Show WebSocket connection established
- Type a message
- Show message captured in Gasoline
- Show response captured

**Voiceover:**
"Let's look at a real-time chat app. Gasoline captures the WebSocket connection, every message sent and received, and any errors. All in real-time."

### [0:30-0:45] Connection Events

**Visual:**
- Zoom in on connection event
- Show JSON payload with connection details
- Highlight key fields: URL, timestamp, tab ID

**Voiceover:**
"First, the connection event. We capture the WebSocket URL, connection time, and which tab it's associated with."

### [0:45-1:00] Message Events

**Visual:**
- Type "Hello, world!" in chat
- Show message captured in Gasoline
- Show both sent and received messages
- Show JSON payload with message content

**Voiceover:**
"Then every message. We capture the direction—sent or received—the timestamp, and the full payload. JSON payloads are automatically parsed for readability."

### [1:00-1:15] Error Events

**Visual:**
- Disconnect network (simulate error)
- Show WebSocket error captured
- Show error code and message
- Highlight error details

**Voiceover:**
"When something goes wrong, we capture the error. Connection failures, abnormal closures, protocol errors—we get it all with error codes and detailed messages."

### [1:15-1:30] AI Integration

**Visual:**
- Open Claude Code
- Ask: "Why is my chat not receiving messages?"
- Show Claude using Gasoline WebSocket tools
- Show Claude seeing WebSocket error
- Show Claude diagnosing the issue

**Voiceover:**
"Now here's the magic. Your AI assistant can query this data. Ask Claude why your chat isn't working, and it can see the WebSocket errors and diagnose the issue."

### [1:30-1:50] Advanced Features

**Visual:**
- Show WebSocket filtering UI
- Filter by direction (sent/received)
- Filter by time range
- Search by content
- Show correlation with console logs

**Voiceover:**
"Gasoline gives you powerful filtering. Filter by direction, time range, or search by content. WebSocket events are automatically correlated with console logs and network requests."

### [1:50-2:10] Real-World Use Case

**Visual:**
- Show collaborative editing app
- Show multiple WebSocket messages
- Show Claude analyzing message sequence
- Show Claude identifying race condition

**Voiceover:**
"Let's look at a real example. In this collaborative editing app, messages were arriving out of order. Claude analyzed the WebSocket sequence, identified a race condition, and suggested a fix."

### [2:10-2:25] Security & Privacy

**Visual:**
- Show "100% Local" badge
- Show "No Cloud" badge
- Show "Auth Stripping" feature

**Voiceover:**
"Gasoline keeps your data private. Everything stays on your machine—no cloud, no telemetry. Authorization headers are automatically stripped from captured data."

### [2:25-2:40] Comparison

**Visual:**
- Comparison table:
  - Gasoline: ✅ Full WebSocket capture
  - Other tools: ❌ Limited/None
  - Gasoline: ✅ AI integration
  - Other tools: ❌ No AI access

**Voiceover:**
"Most browser observability tools either don't capture WebSockets or provide limited visibility. Gasoline gives you complete WebSocket lifecycle capture with full AI integration."

### [2:40-3:00] Getting Started

**Visual:**
- Show terminal: `npx gasoline-mcp@6.0.0`
- Show extension download
- Show WebSocket app
- Show captured data

**Voiceover:**
"Get started with `npx gasoline-mcp@6.0.0`, download the extension, and start capturing WebSocket data today. Your AI assistant will thank you."

### [3:00-3:15] Outro

**Visual:**
- Show cookwithgasoline.com
- Show GitHub repo
- Show Discord link
- Show related videos

**Voiceover:**
"Check the links below for documentation and join our Discord community. Subscribe for more videos on Gasoline's unique features."

---

## Production Notes

### Screen Recording Tips
- Use a real WebSocket application (chat app, dashboard, etc.)
- Prepare scenarios for connection, messages, and errors
- Show both sent and received messages clearly
- Use zoom effects for important details
- Highlight JSON payloads with color coding

### Visual Enhancements
- Use color coding: Blue = sent, Green = received, Red = errors
- Add animated arrows showing message flow
- Use timeline visualization for message sequence
- Show correlation indicators between WebSocket and console events

### Audio Tips
- Use clear, confident voice
- Pause at key moments for emphasis
- Use sound effects sparingly (message sent/received sounds)
- Keep background music low and subtle

### Demo Preparation
1. **Test WebSocket app** - Ensure it works reliably
2. **Prepare error scenarios** - Have reliable ways to trigger errors
3. **Test Gasoline capture** - Verify all events are captured
4. **Prepare AI conversation** - Have a ready debugging scenario
5. **Check network conditions** - Ensure stable connection

### Visual Assets Needed
- Gasoline logo
- WebSocket icon
- Real-time chat app (or similar)
- Error icons
- Security/privacy badges
- Comparison table graphics

### Call to Action
- Try WebSocket capture today
- Join Discord for support
- Share your WebSocket debugging stories
- Subscribe for more feature demos

### SEO Keywords
- WebSocket debugging
- WebSocket capture
- Browser observability
- Real-time app debugging
- AI debugging tools
- Gasoline MCP WebSocket

### Related Videos
- How Gasoline Captures WebSocket Messages (blog companion)
- Debugging Real-Time Applications with AI
- Gasoline vs Other Browser Tools

---

## Thumbnail Ideas

1. "WebSocket Debugging Made Easy" with WebSocket icon
2. Split screen: "What DevTools Shows" vs "What Gasoline Shows"
3. "See What Others Can't" with eye icon and WebSocket graphic
4. "AI + WebSocket = Debugging Superpowers"

---

## Description Template

```
🔌 WebSocket Capture - See What Other Browser Tools Miss

Chrome DevTools shows WebSocket messages, but that's where it stops. Gasoline MCP captures the complete WebSocket lifecycle—connections, messages, errors, disconnections—and makes it all available to your AI coding assistant.

In this video, you'll see:
✅ Full WebSocket lifecycle capture
✅ Real-time message tracking
✅ Error detection and diagnosis
✅ AI-powered debugging
✅ Advanced filtering and search

📦 Download: https://cookwithgasoline.com/downloads/
📖 Docs: https://cookwithgasoline.com/features/#websocket-events
💻 GitHub: https://github.com/brennhill/gasoline-mcp-ai-devtools
💬 Discord: [LINK]

Timestamps:
0:00 - The Problem with WebSocket Debugging
0:15 - Gasoline's Solution
0:30 - Demo: Real-Time Chat App
0:45 - Connection Events
1:00 - Message Events
1:15 - Error Events
1:30 - AI Integration
1:50 - Advanced Features
2:10 - Real-World Use Case
2:25 - Security & Privacy
2:40 - Comparison with Other Tools
2:55 - Getting Started
3:10 - Outro

#GasolineMCP #WebSocket #Debugging #AI #RealTime #DevTools
```

---

## Hashtags

#GasolineMCP #WebSocket #Debugging #AI #RealTime #DevTools #BrowserObservability #ClaudeCode #OpenSource #WebDevelopment

---

## Alternative Short Version (60 seconds)

### [0:00-0:10] Hook
"Your AI assistant can now see every WebSocket message. Here's how."

### [0:10-0:35] Quick Demo
[Fast montage: WebSocket connection → messages → errors → AI diagnosis]

### [0:35-0:45] Key Features
"Full lifecycle capture. Real-time tracking. AI-powered debugging. All in one tool."

### [0:45-0:60] CTA
"Get Gasoline MCP at cookwithgasoline.com. Link in description."

---

## Social Media Cut (30 seconds)

### [0:00-0:05] Hook
"See what other browser tools miss."

### [0:05-0:20] Demo
[Quick screen capture showing WebSocket capture + AI debugging]

### [0:20-0:30] CTA
"WebSocket capture with Gasoline MCP. Link in bio."

---

## Notes for First Recording

1. **Test WebSocket app thoroughly** - Ensure reliable connection and message flow
2. **Prepare error scenarios** - Have multiple ways to trigger errors
3. **Practice the AI conversation** - Have a natural debugging dialogue
4. **Check audio levels** - Ensure clear voiceover
5. **Test screen recording** - Verify smooth capture of animations
6. **Have backup scenarios** - In case the primary app has issues

---

## Advanced Demo Ideas (for future videos)

1. **WebSocket Security** - Show auth stripping, sensitive data masking
2. **Performance Analysis** - Show message latency, connection timing
3. **Protocol Debugging** - Show custom protocol analysis
4. **Multi-Connection** - Show managing multiple WebSocket connections
5. **Binary Payloads** - Show handling of binary WebSocket messages
