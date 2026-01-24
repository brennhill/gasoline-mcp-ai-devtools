---
title: "Session Checkpoints"
description: "Save named checkpoints during your browser session and diff changes since any point in time. Your AI compares before and after states to pinpoint what changed."
keywords: "session checkpoints, state diff, before after comparison, change detection, debugging sessions, checkpoint diff, browser state tracking"
permalink: /session-checkpoints/
header:
  overlay_image: /assets/images/hero-banner.png
  overlay_filter: 0.85
  excerpt: "Save state. Make changes. Diff what happened."
toc: true
toc_sticky: true
---

Gasoline lets your AI save named checkpoints and then diff everything that changed — console errors, network calls, WebSocket events, user actions, and performance — in a single compressed response.

## <i class="fas fa-exclamation-circle"></i> The Problem

You're debugging with your AI assistant. It sees 200 console entries, 50 network calls, and a wall of WebSocket messages. Most of that noise existed before the bug. What your AI really needs to know is: *what changed since I last looked?*

Without checkpoints, the AI has to re-read everything and mentally diff it. That wastes context window tokens and makes it harder to spot the signal in the noise.

## <i class="fas fa-flag-checkered"></i> How Checkpoints Work

1. **Save a checkpoint** — Records the current position in every buffer (logs, network, WebSocket, actions)
2. **Do your work** — Navigate, click, trigger the bug, deploy a fix
3. **Diff against the checkpoint** — Get only what's new, deduplicated and severity-filtered

The checkpoint stores *positions*, not copies of data. This makes checkpoints lightweight and instant to create.

## <i class="fas fa-terminal"></i> Usage

### Create a Checkpoint

```json
// Save a named checkpoint
{ "tool": "analyze", "arguments": { "target": "changes", "checkpoint": "before-fix" } }
```

The first call with a new name creates the checkpoint. Subsequent calls with the same name return the diff since that point.

### Get Changes Since Checkpoint

```json
// See what changed since "before-fix"
{ "tool": "analyze", "arguments": {
  "target": "changes",
  "checkpoint": "before-fix"
} }

// Only show errors (skip info/warning)
{ "tool": "analyze", "arguments": {
  "target": "changes",
  "checkpoint": "before-fix",
  "severity": "errors_only"
} }

// Only show network and console changes
{ "tool": "analyze", "arguments": {
  "target": "changes",
  "checkpoint": "before-fix",
  "include": ["console", "network"]
} }
```

### Auto-Checkpoint

If no checkpoint name is provided, Gasoline uses an automatic checkpoint that advances on each call — showing only changes since the last time you asked.

## <i class="fas fa-compress-arrows-alt"></i> Compressed Diff Response

The diff response is designed to be token-efficient:

```json
{
  "from": "2024-01-15T10:30:00Z",
  "to": "2024-01-15T10:35:22Z",
  "duration_ms": 322000,
  "severity": "all",
  "summary": "3 new errors, 12 network requests, 1 degraded endpoint",
  "token_count": 450,
  "console": {
    "total_new": 8,
    "errors": [
      { "message": "TypeError: Cannot read property 'id' of null", "count": 3 }
    ],
    "warnings": [
      { "message": "Deprecated API usage", "count": 2 }
    ]
  },
  "network": {
    "total_new": 12,
    "errors": [
      { "url": "/api/users/999", "status": 404, "count": 2 }
    ]
  },
  "performance_alerts": [
    "Endpoint /api/search latency 3× baseline (900ms vs 300ms)"
  ]
}
```

Key optimizations:
- **Deduplication** — Repeated errors shown once with a count
- **Severity filtering** — Skip noise, show only what matters
- **Token counting** — Response includes approximate token cost
- **Category filtering** — Request only the categories you care about

## <i class="fas fa-search"></i> What Your AI Can Do With This

- **Before/after debugging** — "Create checkpoint 'before-fix', apply the fix, then show me what changed."
- **Regression testing** — "Since the deploy, 3 new console errors appeared and `/api/orders` started returning 500s."
- **Focused debugging** — "Only 2 things changed since my last check: a new TypeError and a 404 on the user endpoint."
- **Performance validation** — "After the optimization, the performance alert for `/api/search` is gone. Latency back to baseline."

## <i class="fas fa-database"></i> Checkpoint Limits

| Limit | Value |
|-------|-------|
| Max named checkpoints | 20 |
| Checkpoint name length | 50 characters |
| Max diff entries per category | 50 |
| Message truncation | 200 characters |

Oldest checkpoints are evicted when the limit is reached.

## <i class="fas fa-link"></i> Related

- [Regression Detection](/regression-detection/) — Automatic performance alerts in diffs
- [Noise Filtering](/noise-filtering/) — Reduce noise before diffing
- [Web Vitals](/web-vitals/) — Performance metrics included in diffs
