<div align="center">

```
     ██████╗  █████╗ ███████╗ ██████╗ ██╗     ██╗███╗   ██╗███████╗
    ██╔════╝ ██╔══██╗██╔════╝██╔═══██╗██║     ██║████╗  ██║██╔════╝
    ██║  ███╗███████║███████╗██║   ██║██║     ██║██╔██╗ ██║█████╗
    ██║   ██║██╔══██║╚════██║██║   ██║██║     ██║██║╚██╗██║██╔══╝
    ╚██████╔╝██║  ██║███████║╚██████╔╝███████╗██║██║ ╚████║███████╗
     ╚═════╝ ╚═╝  ╚═╝╚══════╝ ╚═════╝ ╚══════╝╚═╝╚═╝  ╚═══╝╚══════╝
```

**Adding fuel to the AI fire**

[![License](https://img.shields.io/badge/license-PolyForm%20Internal%20Use-blue.svg)](LICENSE)
[![Version](https://img.shields.io/badge/version-3.0.0-green.svg)](https://github.com/brennhill/gasoline/releases)
[![Go](https://img.shields.io/badge/Go-1.21+-00ADD8.svg?logo=go&logoColor=white)](https://go.dev/)
[![Chrome](https://img.shields.io/badge/Chrome-Manifest%20V3-4285F4.svg?logo=googlechrome&logoColor=white)](https://developer.chrome.com/docs/extensions/mv3/)
[![macOS](https://img.shields.io/badge/macOS-supported-000000.svg?logo=apple&logoColor=white)](https://github.com/brennhill/gasoline)
[![Linux](https://img.shields.io/badge/Linux-supported-FCC624.svg?logo=linux&logoColor=black)](https://github.com/brennhill/gasoline)
[![Windows](https://img.shields.io/badge/Windows-supported-0078D6.svg?logo=windows&logoColor=white)](https://github.com/brennhill/gasoline)
[![PRs Welcome](https://img.shields.io/badge/PRs-welcome-brightgreen.svg)](https://github.com/brennhill/gasoline/pulls)

<br />

[Quick Start](#quick-start) •
[Features](#what-gets-captured) •
[Privacy](#privacy--security) •
[Roadmap](#features-roadmap) •
[Development](#development)

</div>

---

**Stop copy-pasting browser errors.** Gasoline captures console logs, network errors, and exceptions from your browser and writes them to a local file that your AI coding assistant (Claude Code, Cursor, etc.) can read.

## Quick Start

Gasoline has two parts: a **local server** (receives and stores logs) and a **browser extension** (captures logs from your pages). Your AI assistant connects to the server via [MCP](https://modelcontextprotocol.io/) (Model Context Protocol) — a standard that lets AI tools talk to external services.

No global install required — `npx` handles everything.

### 1. Add the MCP server to your AI tool

Pick your tool below. This config tells your AI tool to start Gasoline automatically:

**Claude Code** — create `.mcp.json` in your project root:

```json
{
  "mcpServers": {
    "gasoline": {
      "command": "npx",
      "args": ["gasoline-mcp", "--mcp"]
    }
  }
}
```

> Alternatively, add to `~/.claude/settings.json` to enable globally across all projects.

<details>
<summary>Other AI tools (Cursor, Windsurf, Claude Desktop, Zed, Continue)</summary>

**Cursor** — add to `~/.cursor/mcp.json`:

```json
{
  "mcpServers": {
    "gasoline": {
      "command": "npx",
      "args": ["gasoline-mcp", "--mcp"]
    }
  }
}
```

**Windsurf** — add to `~/.codeium/windsurf/mcp_config.json`:

```json
{
  "mcpServers": {
    "gasoline": {
      "command": "npx",
      "args": ["gasoline-mcp", "--mcp"]
    }
  }
}
```

**Claude Desktop** — edit config file:

- macOS: `~/Library/Application Support/Claude/claude_desktop_config.json`
- Windows: `%APPDATA%\Claude\claude_desktop_config.json`

```json
{
  "mcpServers": {
    "gasoline": {
      "command": "npx",
      "args": ["gasoline-mcp", "--mcp"]
    }
  }
}
```

**Zed** — add to `~/.config/zed/settings.json`:

```json
{
  "context_servers": {
    "gasoline": {
      "command": {
        "path": "npx",
        "args": ["gasoline-mcp", "--mcp"]
      }
    }
  }
}
```

**VS Code with Continue** — add to `~/.continue/config.json`:

```json
{
  "experimental": {
    "modelContextProtocolServers": [
      {
        "transport": {
          "type": "stdio",
          "command": "npx",
          "args": ["gasoline-mcp", "--mcp"]
        }
      }
    ]
  }
}
```

</details>

**After adding the config, restart your AI tool.** The server starts automatically when the tool launches.

### 2. Install the browser extension

**Chrome Web Store** (coming soon)

**Load unpacked** (works now):

1. Download or clone this repository
2. Open `chrome://extensions` in Chrome
3. Enable **Developer mode** (top right toggle)
4. Click **Load unpacked**
5. Select the `extension/` folder

### 3. Verify it's working

1. **Check your AI tool** — in Claude Code, run `/mcp` and confirm "gasoline" is listed
2. **Check the extension** — click the Gasoline icon in your browser toolbar; it should show "Connected"
3. **Test it** — open your web app, trigger an error (e.g., `console.error("test")`), then ask your AI: _"What browser errors do you see?"_

Your AI assistant now has access to these tools:

| Tool                 | What it does                                            |
| -------------------- | ------------------------------------------------------- |
| `get_browser_errors` | Recent console errors, network failures, and exceptions |
| `get_browser_logs`   | All logs (errors + warnings + info)                     |
| `clear_browser_logs` | Clears the log file                                     |

### Alternative: Manual server mode (no MCP)

If your AI tool doesn't support MCP, run the server standalone and point your AI at the log file:

```bash
npx gasoline-mcp
```

The server will listen on `http://localhost:7890` and write logs to `~/gasoline-logs.jsonl`.

## What Gets Captured

- **Console logs** - `console.log()`, `.warn()`, `.error()`, `.info()`, `.debug()` with full arguments
- **Network errors** - Failed API calls (4xx, 5xx) with URL, method, status, and response body
- **Exceptions** - Uncaught errors and unhandled promise rejections with full stack traces

## Log Format

Logs are stored in [JSONL format](https://jsonlines.org/) (one JSON object per line). Each entry includes an `_enrichments` array that lists what additional data is attached, making it easy for AI assistants to understand the available context.

### Basic Log Entries

```jsonl
{"ts":"2024-01-22T10:30:00.000Z","level":"error","type":"console","args":["Error message"],"url":"http://localhost:3000/app"}
{"ts":"2024-01-22T10:30:01.000Z","level":"error","type":"network","method":"POST","url":"http://localhost:8789/api","status":401,"response":{"error":"Unauthorized"}}
{"ts":"2024-01-22T10:30:02.000Z","level":"error","type":"exception","message":"Cannot read property 'x' of undefined","stack":"...","filename":"app.js","lineno":42}
```

### Entry Types

| Type                | Description                          | Key Fields                                        |
| ------------------- | ------------------------------------ | ------------------------------------------------- |
| `console`           | Console API calls                    | `level`, `args`                                   |
| `network`           | Failed HTTP requests (4xx, 5xx)      | `method`, `url`, `status`, `response`, `duration` |
| `exception`         | Uncaught errors & promise rejections | `message`, `stack`, `filename`, `lineno`, `colno` |
| `screenshot`        | Page screenshot (PNG base64)         | `dataUrl`, `sizeBytes`, `trigger`                 |
| `dom_snapshot`      | DOM tree snapshot                    | `snapshot`, `viewport`                            |
| `network_waterfall` | Network timing data                  | `entries`, `pending`                              |
| `performance`       | Performance marks/measures           | `marks`, `measures`, `navigation`                 |

### Enrichments

The `_enrichments` array tells you what additional data is attached to an entry:

```jsonl
{"type":"exception","level":"error","_enrichments":["context","userActions","sourceMap"],...}
```

| Enrichment         | Description                                           | Added When                                 |
| ------------------ | ----------------------------------------------------- | ------------------------------------------ |
| `context`          | Developer-set annotations via `__gasoline.annotate()` | Error has context annotations              |
| `userActions`      | Recent clicks, inputs, scrolls before error           | Error entry with action buffer             |
| `sourceMap`        | Stack trace resolved via source maps                  | Source map resolution enabled & successful |
| `domSnapshot`      | DOM tree captured on error                            | DOM snapshot entry                         |
| `networkWaterfall` | Network timing data                                   | Network waterfall entry                    |
| `performanceMarks` | Performance marks/measures                            | Performance entry                          |
| `screenshot`       | Page screenshot                                       | Screenshot entry                           |

### Enriched Error Example

```json
{
  "ts": "2024-01-22T10:30:00.000Z",
  "type": "exception",
  "level": "error",
  "message": "Cannot read property 'user' of undefined",
  "stack": "TypeError: Cannot read property 'user' of undefined\n    at handleLogin (src/auth.ts:42:15)",
  "filename": "src/auth.ts",
  "lineno": 42,
  "url": "http://localhost:3000/login",
  "_enrichments": ["context", "userActions", "sourceMap"],
  "_context": {
    "checkout-flow": { "step": "payment", "items": 3 },
    "user": { "id": "u123", "plan": "pro" }
  },
  "_actions": [
    {
      "ts": "2024-01-22T10:29:55.000Z",
      "type": "click",
      "target": "button#submit",
      "text": "Login"
    },
    {
      "ts": "2024-01-22T10:29:56.000Z",
      "type": "input",
      "target": "input#email",
      "value": "user@example.com"
    }
  ],
  "_sourceMapResolved": true
}
```

### Linked Enrichment Entries

Some enrichments are sent as separate entries linked by `_errorTs`:

```jsonl
{"type":"exception","ts":"2024-01-22T10:30:00.000Z","level":"error","message":"..."}
{"type":"dom_snapshot","ts":"2024-01-22T10:30:00.100Z","_enrichments":["domSnapshot"],"_errorTs":"2024-01-22T10:30:00.000Z","snapshot":{...}}
{"type":"network_waterfall","ts":"2024-01-22T10:30:00.100Z","_enrichments":["networkWaterfall"],"_errorTs":"2024-01-22T10:30:00.000Z","entries":[...]}
{"type":"screenshot","ts":"2024-01-22T10:30:00.200Z","_enrichments":["screenshot"],"relatedErrorId":"err_1705921800000_abc123","dataUrl":"data:image/png;base64,..."}
```

### Error Grouping

Repeated errors within 5 seconds are deduplicated. Grouped entries include:

```json
{
  "type": "exception",
  "_aggregatedCount": 15,
  "_firstSeen": "2024-01-22T10:30:00.000Z",
  "_lastSeen": "2024-01-22T10:30:04.500Z"
}
```

### Rate Limiting & Error Grouping

When errors cascade rapidly (e.g., a render loop throwing repeatedly), Gasoline prevents log flooding:

**Error Grouping** - Identical errors within 5 seconds are deduplicated:

- First occurrence is sent immediately with full context
- Subsequent duplicates increment a counter
- After 5s or 10s, an aggregated entry is sent with `_aggregatedCount`

**Feature Rate Limits:**

| Feature           | Limit                            | Reason                           |
| ----------------- | -------------------------------- | -------------------------------- |
| Screenshots       | 5s between, 10/session max       | Large payload size (~500KB each) |
| DOM Snapshots     | 5s between                       | DOM traversal cost               |
| Network Waterfall | 50 entries, 30s window           | Reads existing browser data      |
| Performance Marks | 50 entries, 60s window           | Reads existing browser data      |
| User Actions      | 20 item buffer, scroll throttled | Just metadata, very lightweight  |

**Why some features aren't rate-limited:** Network waterfall and performance marks simply read data the browser already collected - there's no capture cost. They're size-limited instead (50 entries max), and since error grouping deduplicates rapid errors, the same data isn't sent repeatedly anyway.

## Developer API

Gasoline exposes `window.__gasoline` for adding context to your logs:

```javascript
// Add context that will be included with all subsequent errors
window.__gasoline.annotate('checkout-flow', { step: 'payment', cartId: 'abc123' })
window.__gasoline.annotate('user', { id: 'u123', plan: 'pro' })

// Remove specific annotation
window.__gasoline.removeAnnotation('checkout-flow')

// Clear all annotations
window.__gasoline.clearAnnotations()

// Get current context
const context = window.__gasoline.getContext()
```

### Available Methods

| Method                         | Description                                   |
| ------------------------------ | --------------------------------------------- |
| `annotate(key, value)`         | Add context annotation (included with errors) |
| `removeAnnotation(key)`        | Remove a specific annotation                  |
| `clearAnnotations()`           | Clear all annotations                         |
| `getContext()`                 | Get current annotations                       |
| `getActions()`                 | Get recent user actions buffer                |
| `clearActions()`               | Clear the action buffer                       |
| `setActionCapture(enabled)`    | Enable/disable user action capture            |
| `setDOMSnapshot(enabled)`      | Enable/disable DOM snapshots                  |
| `setNetworkWaterfall(enabled)` | Enable/disable network waterfall              |
| `setPerformanceMarks(enabled)` | Enable/disable performance marks              |
| `getNetworkWaterfall(options)` | Get current network waterfall data            |
| `getMarks(options)`            | Get performance marks                         |
| `getMeasures(options)`         | Get performance measures                      |
| `version`                      | API version (currently "3.0.0")               |

### Example: Add Context in React

```jsx
useEffect(() => {
  // Set user context when authenticated
  if (user) {
    window.__gasoline?.annotate('user', {
      id: user.id,
      role: user.role,
    })
  }
  return () => window.__gasoline?.removeAnnotation('user')
}, [user])

// Set flow context
function CheckoutPage() {
  useEffect(() => {
    window.__gasoline?.annotate('flow', 'checkout')
    return () => window.__gasoline?.removeAnnotation('flow')
  }, [])
}
```

## Server Options

```bash
npx gasoline-mcp [options]

Options:
  --port <number>        Port to listen on (default: 7890)
  --log-file <path>      Path to log file (default: ~/gasoline-logs.jsonl)
  --max-entries <number> Max log entries before rotation (default: 1000)
  --mcp                  Run in MCP mode for Claude Code integration
  --help, -h             Show help message
```

### Log File Auto-Discovery

The extension automatically discovers the log file path from the server. When you use `--log-file` to set a custom location, the server reports the actual path via its `/health` endpoint. The extension popup will display the correct path under "Server Info" once connected — no manual configuration needed in the extension.

### Log Rotation (`--max-entries`)

The default limit is **1000 entries**. When this limit is reached, the oldest entries are removed to make room for new ones.

**Why 1000?** With enrichments enabled, a single error can generate multiple entries:

| Entry Type        | Per Error | Typical Size |
| ----------------- | --------- | ------------ |
| Error/exception   | 1         | ~1-2 KB      |
| DOM snapshot      | 1         | ~5-20 KB     |
| Network waterfall | 1         | ~10-30 KB    |
| Performance marks | 1         | ~5-10 KB     |
| Screenshot        | 1         | ~300-500 KB  |
| User actions      | included  | ~1-2 KB      |

A fully-enriched error = ~5 entries, so 1000 entries ≈ **200 fully-enriched errors**. For a typical debugging session (10-50 errors), this provides ample history.

**File size considerations:** Screenshots are the largest payload. With 10 screenshots (session max), the file can reach ~5 MB. Without screenshots, 1000 entries typically stays under 1 MB.

**When to increase:** If you set capture level to "All Logs" on a chatty application, routine `console.log` calls may push out errors. In that case, either increase the limit or use "Errors Only" capture level:

```bash
# Increase to 5000 entries for verbose logging
npx gasoline-mcp --max-entries 5000
```

**When to decrease:** If disk space is constrained or you only need the most recent errors:

```bash
# Keep only the last 200 entries
npx gasoline-mcp --max-entries 200
```

### MCP Mode

When using `--mcp`, Gasoline runs as an MCP (Model Context Protocol) server:

- HTTP server runs in the background for the browser extension
- MCP protocol runs over stdio for Claude Code integration

This allows Claude Code to automatically query browser errors while still receiving logs from the browser extension.

## MCP Integration

Gasoline supports the [Model Context Protocol](https://modelcontextprotocol.io/) for seamless AI assistant integration. Here's how to configure it with popular tools:

### Claude Code (CLI)

Add to `~/.claude/settings.json`:

```json
{
  "mcpServers": {
    "gasoline": {
      "command": "npx",
      "args": ["gasoline-mcp", "--mcp"]
    }
  }
}
```

### Claude Desktop

Add to your Claude Desktop config file:

- **macOS**: `~/Library/Application Support/Claude/claude_desktop_config.json`
- **Windows**: `%APPDATA%\Claude\claude_desktop_config.json`

```json
{
  "mcpServers": {
    "gasoline": {
      "command": "npx",
      "args": ["gasoline-mcp", "--mcp"]
    }
  }
}
```

### Cursor

Add to Cursor's MCP settings (Settings → MCP Servers → Add Server):

```json
{
  "gasoline": {
    "command": "npx",
    "args": ["gasoline-mcp", "--mcp"]
  }
}
```

Or add directly to `~/.cursor/mcp.json`:

```json
{
  "mcpServers": {
    "gasoline": {
      "command": "npx",
      "args": ["gasoline-mcp", "--mcp"]
    }
  }
}
```

### Windsurf

Add to Windsurf's MCP configuration (`~/.codeium/windsurf/mcp_config.json`):

```json
{
  "mcpServers": {
    "gasoline": {
      "command": "npx",
      "args": ["gasoline-mcp", "--mcp"]
    }
  }
}
```

### VS Code with Continue

Add to Continue's config (`~/.continue/config.json`):

```json
{
  "experimental": {
    "modelContextProtocolServers": [
      {
        "transport": {
          "type": "stdio",
          "command": "npx",
          "args": ["gasoline-mcp", "--mcp"]
        }
      }
    ]
  }
}
```

### Zed

Add to your Zed settings (`~/.config/zed/settings.json`):

```json
{
  "context_servers": {
    "gasoline": {
      "command": {
        "path": "npx",
        "args": ["gasoline-mcp", "--mcp"]
      }
    }
  }
}
```

### Custom Port Configuration

If you need to run Gasoline on a different port, add the `--port` flag:

```json
{
  "mcpServers": {
    "gasoline": {
      "command": "npx",
      "args": ["gasoline-mcp", "--mcp", "--port", "7891"]
    }
  }
}
```

### Available MCP Tools

Once connected, your AI assistant has access to these tools:

| Tool                 | Description                                                              |
| -------------------- | ------------------------------------------------------------------------ |
| `get_browser_errors` | Get recent browser errors (console errors, network failures, exceptions) |
| `get_browser_logs`   | Get all browser logs (errors, warnings, info)                            |
| `clear_browser_logs` | Clear the log file                                                       |

### Verifying MCP Connection

After configuring, verify the connection:

1. Restart your AI tool to load the new MCP server
2. The Gasoline server should start automatically
3. Check the extension popup - it should show "Connected"
4. Ask your AI assistant "What browser errors do you see?" to test

## Extension Settings

Click the extension icon to:

- View connection status
- Set capture level (Errors Only, Warnings+, All Logs)
- Toggle advanced capture features (DOM snapshots, network waterfall, etc.)
- Clear all logs

In Options, you can configure domain filters to only capture logs from specific sites.

## Privacy & Security

- **100% Local** - Logs never leave your machine. No cloud, no analytics, no telemetry.
- **Localhost Only** - Server binds to 127.0.0.1, not accessible from network.
- **Sensitive Data Excluded** - Authorization headers are automatically stripped from network logs.
- **Open Source** - Audit the code yourself.

## How It Works

```
Browser Page                Extension                 Server              AI Assistant
    │                          │                        │                      │
    │  console.error()         │                        │                      │
    │─────────────────────────>│                        │                      │
    │                          │  POST /logs            │                      │
    │                          │───────────────────────>│                      │
    │                          │                        │  append to file      │
    │                          │                        │──────────────────────│
    │                          │                        │                      │
    │                          │                        │                      │ read file
    │                          │                        │<─────────────────────│
```

1. The **inject script** runs in every page and intercepts console methods, fetch errors, and exceptions
2. Captured logs are sent via `postMessage` to the **content script**
3. The content script forwards logs to the **background service worker**
4. The service worker batches logs and POSTs them to the **local server**
5. The server appends each log entry to the JSONL file
6. Your **AI assistant** can read the file to help debug issues

## Performance SLOs

Gasoline is designed to have minimal impact on page performance. These are the Service Level Objectives (SLOs) enforced by our benchmark tests:

### Latency Targets

| Operation                   | Target  | Description                                |
| --------------------------- | ------- | ------------------------------------------ |
| Console interception        | < 0.1ms | Overhead per console.log/warn/error call   |
| Error serialization         | < 1ms   | Serializing typical error payloads         |
| Error signature computation | < 0.1ms | Computing dedup signature per error        |
| Log entry formatting        | < 0.1ms | Formatting entry before sending            |
| Error group processing      | < 0.2ms | Deduplication and grouping logic           |
| User action recording       | < 0.1ms | Recording a click/input event              |
| Network waterfall (50 req)  | < 5ms   | Collecting timing data for 50 requests     |
| DOM snapshot (simple)       | < 5ms   | Capturing small DOM subtree                |
| DOM snapshot (complex)      | < 50ms  | Capturing large DOM with node limits       |
| Full error path             | < 5ms   | Total time from error to queued for server |

### Safeguards

Built-in limits prevent runaway resource usage:

| Safeguard            | Limit               | Purpose                        |
| -------------------- | ------------------- | ------------------------------ |
| DOM snapshot nodes   | 100 max             | Prevent huge snapshots         |
| DOM snapshot depth   | 5 levels            | Limit tree traversal           |
| String truncation    | 10KB                | Cap large log arguments        |
| Screenshots          | 5s rate, 10/session | Prevent capture flood          |
| DOM snapshots        | 5s rate             | Prevent capture flood          |
| Network waterfall    | 50 entries          | Limit data collection          |
| Performance marks    | 50 entries          | Limit data collection          |
| User action buffer   | 20 items            | Rolling buffer, oldest dropped |
| Error dedup window   | 5 seconds           | Suppress duplicate errors      |
| Error groups tracked | 100 max             | Bound memory usage             |
| Debug log buffer     | 200 entries         | Circular buffer                |

### Running Benchmarks

```bash
# Run performance benchmark tests
cd apps/dev-console
node --test extension-tests/performance.test.js
```

## Features Roadmap

### v1 (Complete)

- [x] Console log capture (log, warn, error, info, debug)
- [x] Network error capture (4xx, 5xx responses)
- [x] Exception capture (uncaught errors, promise rejections)
- [x] Configurable log levels
- [x] Domain filtering
- [x] Log rotation

### v2 (Complete)

- [x] **Screenshot capture** - Auto-capture on error (configurable)
- [x] **DOM snapshot** - Capture relevant DOM subtree on error
- [x] **Error grouping** - Deduplicate repeated errors
- [x] **Rate limiting** - Prevent screenshot/snapshot flood
- [x] **Context annotations** - `window.__gasoline.annotate()` API for semantic context
- [x] **User action replay** - Buffer of recent clicks/inputs before error
- [x] **Source map support** - Resolve minified stack traces
- [x] **Network waterfall** - Full request/response timing data
- [x] **Performance marks** - Capture performance.mark() and measure()
- [x] **Toggle controls** - Enable/disable advanced features from popup
- [x] **Debug logging** - Internal extension logging for troubleshooting

### v3 (Current)

- [x] **Configurable server URL** - Change port in extension Options
- [x] **Performance benchmarks** - SLO tests for all critical paths
- [x] **Debug log export** - Download JSON with recent extension activity

## Troubleshooting

### Extension not connecting to server

1. **Check server is running**: Look for `Gasoline server listening on http://localhost:7890`
2. **Check extension badge**: Red `!` means disconnected, green means connected
3. **Check port conflict**: Another process may be using port 7890. Try `--port 7891`
4. **Update extension URL**: If using a different port, go to Options and change the Server URL to match (e.g., `http://localhost:7891`)
5. **Check browser console**: Open extension popup, right-click → Inspect, check for errors

### Changing the server port

If port 7890 is in use, you can run the server on a different port:

```bash
# Start server on port 7891
./gasoline --port 7891

# Or with make
make run PORT=7891
```

Then update the extension to use the new port:

1. Click the Gasoline extension icon
2. Click "Options" at the bottom
3. Change **Server URL** to `http://localhost:7891`
4. Click "Save Options"

### Logs not appearing

1. **Check capture level**: Popup may be set to "Errors Only" - try "All Logs"
2. **Check domain filter**: Options page may have filters excluding your domain
3. **Check log file**: `cat ~/gasoline-logs.jsonl | tail -5` to see recent entries
4. **Reload the page**: Extension injects on page load

### MCP mode not working with Claude Code

1. **Check settings.json path**: Should be in Claude Code's config directory
2. **Restart Claude Code**: MCP servers are loaded on startup
3. **Check Claude Code logs**: Look for MCP connection errors

### Using Debug Mode

The extension has a built-in debug logging system for troubleshooting issues:

1. **Enable Debug Mode**: Open the extension popup, scroll to "Debugging" section, toggle "Debug Mode" on
2. **View Debug Logs**: With debug mode on, extension activity is logged to the browser console
3. **Export Debug Log**: Click "Export Debug Log" to download a JSON file with recent activity
4. **What's Captured**: Connection status changes, log capture events, rate limiting, errors, settings changes

Debug log categories:

- `connection` - Server connection/disconnection events
- `capture` - Log capture, filtering, and processing events
- `error` - Extension internal errors
- `lifecycle` - Extension startup/shutdown events
- `settings` - Configuration changes

The debug buffer holds up to 200 entries and is circular (oldest entries are dropped). Debug logs are stored even when debug mode is off, so you can export them after an issue occurs.

## Reporting Issues

If you encounter a bug, please [open an issue](https://github.com/brennhill/gasoline/issues/new) with:

1. **Environment**: OS, Chrome version, Gasoline version (`window.__gasoline.version`)
2. **Steps to reproduce**: What you did before the issue occurred
3. **Expected behavior**: What should have happened
4. **Actual behavior**: What actually happened
5. **Extension popup screenshot**: Shows connection status and settings
6. **Debug log export**: Click "Export Debug Log" in the popup and attach the JSON file
7. **Browser console errors**: Right-click extension icon → Inspect → Console tab
8. **Server output**: Any errors from `npx gasoline-mcp`

For log format or enrichment questions, include a sample log entry (redact sensitive data).

## Development

```bash
# Clone the repository
git clone https://github.com/brennhill/gasoline
cd gasoline

# Build the Go server
make build

# Run server locally
make run

# Run extension tests
node --experimental-vm-modules --test extension-tests/*.test.js
```

## Publishing (npm)

Gasoline is distributed as an npm package (`gasoline-mcp`) with platform-specific binaries, similar to how esbuild works.

### Package Structure

```
npm/
├── gasoline-cli/               # Main package (gasoline-mcp on npm)
├── darwin-arm64/               # @brennhill/gasoline-darwin-arm64
├── darwin-x64/                 # @brennhill/gasoline-darwin-x64
├── linux-arm64/                # @brennhill/gasoline-linux-arm64
├── linux-x64/                  # @brennhill/gasoline-linux-x64
└── win32-x64/                  # @brennhill/gasoline-win32-x64
```

When a user runs `npm install gasoline-mcp`, npm installs only the platform-specific optional dependency matching their OS/architecture. The main package's bin script detects the platform and runs the correct binary.

### Publishing a New Version

```bash
# 1. Update version in all package.json files (npm/ directory)
# 2. Build and publish
cd apps/dev-console
./npm/publish.sh

# Or do a dry run first
./npm/publish.sh --dry-run
```

The script will:

1. Build Go binaries for all platforms (`make build`)
2. Copy binaries into the correct npm package directories
3. Publish each `@brennhill/gasoline-*` platform package
4. Publish the main `gasoline-mcp` package

### Supported Platforms

| Platform | Architecture        | Package                            |
| -------- | ------------------- | ---------------------------------- |
| macOS    | Apple Silicon (M1+) | `@brennhill/gasoline-darwin-arm64` |
| macOS    | Intel x64           | `@brennhill/gasoline-darwin-x64`   |
| Linux    | ARM64               | `@brennhill/gasoline-linux-arm64`  |
| Linux    | x64                 | `@brennhill/gasoline-linux-x64`    |
| Windows  | x64                 | `@brennhill/gasoline-win32-x64`    |

## Requirements

- **Server**: Go 1.21+ (or use `npx gasoline-mcp` for pre-built binary)
- **Extension**: Chrome/Chromium-based browser (Manifest V3)

## License

PolyForm Internal Use License 1.0.0 - see [LICENSE](LICENSE)

---

<div align="center">

**Made for developers who debug with AI.**

<br />

If you find Gasoline useful, please consider giving it a star!

[![Star on GitHub](https://img.shields.io/github/stars/brennhill/gasoline.svg?style=social)](https://github.com/brennhill/gasoline)

</div>
