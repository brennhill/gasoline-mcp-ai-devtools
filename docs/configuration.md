---
title: "Configuration"
description: "Gasoline server options and extension settings. Configure port, log file path, log rotation, capture levels, and domain filters."
keywords: "gasoline configuration, server options, log rotation, capture level, domain filter, extension settings"
permalink: /configuration/
header:
  overlay_image: /assets/images/hero-banner.png
  overlay_filter: 0.85
  excerpt: "Tune the burner — ports, paths, and capture levels."
toc: true
toc_sticky: true
---

## <i class="fas fa-terminal"></i> Server Options

```bash
npx gasoline-mcp [options]

Options:
  --port <number>        Port to listen on (default: 7890)
  --log-file <path>      Path to log file (default: ~/gasoline-logs.jsonl)
  --max-entries <number> Max log entries before rotation (default: 1000)
  --mcp                  No-op (MCP mode is the default, kept for backwards compatibility)
  --help, -h             Show help message
```

## <i class="fas fa-search-location"></i> Log File Auto-Discovery

The extension automatically discovers the log file path from the server. When you use `--log-file` to set a custom location, the server reports the actual path via its `/health` endpoint. The extension popup displays the correct path under "Server Info."

## <i class="fas fa-sync-alt"></i> Log Rotation

The default limit is **1000 entries**. When reached, the oldest entries are removed.

### Why 1000?

With enrichments enabled, a single error can generate multiple entries:

| Entry Type | Per Error | Typical Size |
|-----------|-----------|-------------|
| Error/exception | 1 | ~1-2 KB |
| Network waterfall | 1 | ~10-30 KB |
| Performance marks | 1 | ~5-10 KB |
| Screenshot | 1 | ~0.1 KB (filename reference) |
| User actions | included | ~1-2 KB |

A fully-enriched error = ~4 entries, so 1000 entries = **~250 fully-enriched errors**. For a typical debugging session (10-50 errors), this provides ample history.

### Increase for Verbose Logging

```bash
npx gasoline-mcp --max-entries 5000
```

### Decrease for Constrained Disk

```bash
npx gasoline-mcp --max-entries 200
```

## <i class="fas fa-sliders-h"></i> Extension Settings

Click the Gasoline extension icon to configure:

### Capture Level

- **Errors Only** — console.error, network failures, exceptions
- **Warnings+** — errors plus console.warn
- **All Logs** — everything including console.log, info, debug

### Advanced Features

Toggle these independently:

- **WebSocket monitoring** — connection lifecycle and messages
- **Network waterfall** — request timing data
- **Performance marks** — performance.mark() and measure()
- **User actions** — click/input/scroll buffer
- **Screenshot on error** — auto-capture on exceptions
- **Source maps** — resolve minified stack traces

### WebSocket Capture Mode

- **Lifecycle only** — open/close events
- **Include messages** — message payloads with adaptive sampling

### Domain Filters

In Options, configure which domains to capture from. This prevents noise from third-party scripts and analytics.

## <i class="fas fa-plug"></i> Custom Port

If port 7890 is in use:

```bash
npx gasoline-mcp --port 7891
```

Then update the extension:

1. Click the Gasoline icon
2. Click "Options"
3. Change **Server URL** to `http://localhost:7891`
4. Click "Save Options"
