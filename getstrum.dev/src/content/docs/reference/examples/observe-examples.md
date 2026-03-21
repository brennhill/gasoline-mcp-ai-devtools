---
title: "Observe Executable Examples"
description: "Runnable examples for every observe mode with response shapes and failure fixes."
last_verified_version: 0.8.0
last_verified_date: 2026-03-06
normalized_tags: ['reference', 'examples', 'observe']
---

# Observe Executable Examples

Each section provides one runnable baseline call, expected response shape, and one failure example with a concrete fix. Use these as copy/paste starters and then adjust for your page or workflow.

## Quick Reference

```json
{
  "tool": "observe",
  "arguments": {
    "what": "errors"
  }
}
```

## Common Parameters

| Parameter | Type | Description |
| --- | --- | --- |
| `what` | string | Mode name to execute. |
| `tab_id` | number | Optional target browser tab. |
| `telemetry_mode` | string | Optional telemetry verbosity: `off`, `auto`, `full`. |

## Modes

### `errors`

#### Minimal call

```json
{
  "tool": "observe",
  "arguments": {
    "what": "errors"
  }
}
```

#### Expected response shape

```json
{
  "what": "errors",
  "items": [
    {
      "id": "sample",
      "summary": "...mode-specific payload..."
    }
  ],
  "metadata": {
    "limit": 100,
    "next_cursor": "cursor_123"
  }
}
```

#### Failure example and fix

```json
{
  "tool": "observe",
  "arguments": {
    "what": "not_a_real_mode"
  }
}
```

Fix: Use a valid observe mode value, e.g. `errors`.

### `logs`

#### Minimal call

```json
{
  "tool": "observe",
  "arguments": {
    "what": "logs"
  }
}
```

#### Expected response shape

```json
{
  "what": "logs",
  "items": [
    {
      "id": "sample",
      "summary": "...mode-specific payload..."
    }
  ],
  "metadata": {
    "limit": 100,
    "next_cursor": "cursor_123"
  }
}
```

#### Failure example and fix

```json
{
  "tool": "observe",
  "arguments": {
    "what": "not_a_real_mode"
  }
}
```

Fix: Use a valid observe mode value, e.g. `logs`.

### `extension_logs`

#### Minimal call

```json
{
  "tool": "observe",
  "arguments": {
    "what": "extension_logs"
  }
}
```

#### Expected response shape

```json
{
  "what": "extension_logs",
  "items": [
    {
      "id": "sample",
      "summary": "...mode-specific payload..."
    }
  ],
  "metadata": {
    "limit": 100,
    "next_cursor": "cursor_123"
  }
}
```

#### Failure example and fix

```json
{
  "tool": "observe",
  "arguments": {
    "what": "not_a_real_mode"
  }
}
```

Fix: Use a valid observe mode value, e.g. `extension_logs`.

### `network_waterfall`

#### Minimal call

```json
{
  "tool": "observe",
  "arguments": {
    "what": "network_waterfall"
  }
}
```

#### Expected response shape

```json
{
  "what": "network_waterfall",
  "items": [
    {
      "id": "sample",
      "summary": "...mode-specific payload..."
    }
  ],
  "metadata": {
    "limit": 100,
    "next_cursor": "cursor_123"
  }
}
```

#### Failure example and fix

```json
{
  "tool": "observe",
  "arguments": {
    "what": "not_a_real_mode"
  }
}
```

Fix: Use a valid observe mode value, e.g. `network_waterfall`.

### `network_bodies`

#### Minimal call

```json
{
  "tool": "observe",
  "arguments": {
    "what": "network_bodies",
    "url": "/api",
    "status_min": 400
  }
}
```

#### Expected response shape

```json
{
  "what": "network_bodies",
  "items": [
    {
      "id": "sample",
      "summary": "...mode-specific payload..."
    }
  ],
  "metadata": {
    "limit": 100,
    "next_cursor": "cursor_123"
  }
}
```

#### Failure example and fix

```json
{
  "tool": "observe",
  "arguments": {
    "what": "network_bodies",
    "url": 404,
    "status_min": 400
  }
}
```

Fix: Use a fully qualified URL string, e.g. `https://example.com`.

### `websocket_events`

#### Minimal call

```json
{
  "tool": "observe",
  "arguments": {
    "what": "websocket_events",
    "last_n": 20
  }
}
```

#### Expected response shape

```json
{
  "what": "websocket_events",
  "items": [
    {
      "id": "sample",
      "summary": "...mode-specific payload..."
    }
  ],
  "metadata": {
    "limit": 100,
    "next_cursor": "cursor_123"
  }
}
```

#### Failure example and fix

```json
{
  "tool": "observe",
  "arguments": {
    "what": "not_a_real_mode",
    "last_n": 20
  }
}
```

Fix: Use a valid observe mode value, e.g. `websocket_events`.

### `websocket_status`

#### Minimal call

```json
{
  "tool": "observe",
  "arguments": {
    "what": "websocket_status"
  }
}
```

#### Expected response shape

```json
{
  "what": "websocket_status",
  "items": [
    {
      "id": "sample",
      "summary": "...mode-specific payload..."
    }
  ],
  "metadata": {
    "limit": 100,
    "next_cursor": "cursor_123"
  }
}
```

#### Failure example and fix

```json
{
  "tool": "observe",
  "arguments": {
    "what": "not_a_real_mode"
  }
}
```

Fix: Use a valid observe mode value, e.g. `websocket_status`.

### `actions`

#### Minimal call

```json
{
  "tool": "observe",
  "arguments": {
    "what": "actions"
  }
}
```

#### Expected response shape

```json
{
  "what": "actions",
  "items": [
    {
      "id": "sample",
      "summary": "...mode-specific payload..."
    }
  ],
  "metadata": {
    "limit": 100,
    "next_cursor": "cursor_123"
  }
}
```

#### Failure example and fix

```json
{
  "tool": "observe",
  "arguments": {
    "what": "not_a_real_mode"
  }
}
```

Fix: Use a valid observe mode value, e.g. `actions`.

### `vitals`

#### Minimal call

```json
{
  "tool": "observe",
  "arguments": {
    "what": "vitals"
  }
}
```

#### Expected response shape

```json
{
  "what": "vitals",
  "items": [
    {
      "id": "sample",
      "summary": "...mode-specific payload..."
    }
  ],
  "metadata": {
    "limit": 100,
    "next_cursor": "cursor_123"
  }
}
```

#### Failure example and fix

```json
{
  "tool": "observe",
  "arguments": {
    "what": "not_a_real_mode"
  }
}
```

Fix: Use a valid observe mode value, e.g. `vitals`.

### `page`

#### Minimal call

```json
{
  "tool": "observe",
  "arguments": {
    "what": "page"
  }
}
```

#### Expected response shape

```json
{
  "what": "page",
  "items": [
    {
      "id": "sample",
      "summary": "...mode-specific payload..."
    }
  ],
  "metadata": {
    "limit": 100,
    "next_cursor": "cursor_123"
  }
}
```

#### Failure example and fix

```json
{
  "tool": "observe",
  "arguments": {
    "what": "not_a_real_mode"
  }
}
```

Fix: Use a valid observe mode value, e.g. `page`.

### `tabs`

#### Minimal call

```json
{
  "tool": "observe",
  "arguments": {
    "what": "tabs"
  }
}
```

#### Expected response shape

```json
{
  "what": "tabs",
  "items": [
    {
      "id": "sample",
      "summary": "...mode-specific payload..."
    }
  ],
  "metadata": {
    "limit": 100,
    "next_cursor": "cursor_123"
  }
}
```

#### Failure example and fix

```json
{
  "tool": "observe",
  "arguments": {
    "what": "not_a_real_mode"
  }
}
```

Fix: Use a valid observe mode value, e.g. `tabs`.

### `history`

#### Minimal call

```json
{
  "tool": "observe",
  "arguments": {
    "what": "history"
  }
}
```

#### Expected response shape

```json
{
  "what": "history",
  "items": [
    {
      "id": "sample",
      "summary": "...mode-specific payload..."
    }
  ],
  "metadata": {
    "limit": 100,
    "next_cursor": "cursor_123"
  }
}
```

#### Failure example and fix

```json
{
  "tool": "observe",
  "arguments": {
    "what": "not_a_real_mode"
  }
}
```

Fix: Use a valid observe mode value, e.g. `history`.

### `pilot`

#### Minimal call

```json
{
  "tool": "observe",
  "arguments": {
    "what": "pilot"
  }
}
```

#### Expected response shape

```json
{
  "what": "pilot",
  "items": [
    {
      "id": "sample",
      "summary": "...mode-specific payload..."
    }
  ],
  "metadata": {
    "limit": 100,
    "next_cursor": "cursor_123"
  }
}
```

#### Failure example and fix

```json
{
  "tool": "observe",
  "arguments": {
    "what": "not_a_real_mode"
  }
}
```

Fix: Use a valid observe mode value, e.g. `pilot`.

### `timeline`

#### Minimal call

```json
{
  "tool": "observe",
  "arguments": {
    "what": "timeline"
  }
}
```

#### Expected response shape

```json
{
  "what": "timeline",
  "items": [
    {
      "id": "sample",
      "summary": "...mode-specific payload..."
    }
  ],
  "metadata": {
    "limit": 100,
    "next_cursor": "cursor_123"
  }
}
```

#### Failure example and fix

```json
{
  "tool": "observe",
  "arguments": {
    "what": "not_a_real_mode"
  }
}
```

Fix: Use a valid observe mode value, e.g. `timeline`.

### `error_bundles`

#### Minimal call

```json
{
  "tool": "observe",
  "arguments": {
    "what": "error_bundles"
  }
}
```

#### Expected response shape

```json
{
  "what": "error_bundles",
  "items": [
    {
      "id": "sample",
      "summary": "...mode-specific payload..."
    }
  ],
  "metadata": {
    "limit": 100,
    "next_cursor": "cursor_123"
  }
}
```

#### Failure example and fix

```json
{
  "tool": "observe",
  "arguments": {
    "what": "not_a_real_mode"
  }
}
```

Fix: Use a valid observe mode value, e.g. `error_bundles`.

### `screenshot`

#### Minimal call

```json
{
  "tool": "observe",
  "arguments": {
    "what": "screenshot",
    "full_page": true
  }
}
```

#### Expected response shape

```json
{
  "what": "screenshot",
  "items": [
    {
      "id": "sample",
      "summary": "...mode-specific payload..."
    }
  ],
  "metadata": {
    "limit": 100,
    "next_cursor": "cursor_123"
  }
}
```

#### Failure example and fix

```json
{
  "tool": "observe",
  "arguments": {
    "what": "not_a_real_mode",
    "full_page": true
  }
}
```

Fix: Use a valid observe mode value, e.g. `screenshot`.

### `storage`

#### Minimal call

```json
{
  "tool": "observe",
  "arguments": {
    "what": "storage"
  }
}
```

#### Expected response shape

```json
{
  "what": "storage",
  "items": [
    {
      "id": "sample",
      "summary": "...mode-specific payload..."
    }
  ],
  "metadata": {
    "limit": 100,
    "next_cursor": "cursor_123"
  }
}
```

#### Failure example and fix

```json
{
  "tool": "observe",
  "arguments": {
    "what": "not_a_real_mode"
  }
}
```

Fix: Use a valid observe mode value, e.g. `storage`.

### `indexeddb`

#### Minimal call

```json
{
  "tool": "observe",
  "arguments": {
    "what": "indexeddb"
  }
}
```

#### Expected response shape

```json
{
  "what": "indexeddb",
  "items": [
    {
      "id": "sample",
      "summary": "...mode-specific payload..."
    }
  ],
  "metadata": {
    "limit": 100,
    "next_cursor": "cursor_123"
  }
}
```

#### Failure example and fix

```json
{
  "tool": "observe",
  "arguments": {
    "what": "not_a_real_mode"
  }
}
```

Fix: Use a valid observe mode value, e.g. `indexeddb`.

### `command_result`

#### Minimal call

```json
{
  "tool": "observe",
  "arguments": {
    "what": "command_result",
    "correlation_id": "cmd_123"
  }
}
```

#### Expected response shape

```json
{
  "what": "command_result",
  "items": [
    {
      "id": "sample",
      "summary": "...mode-specific payload..."
    }
  ],
  "metadata": {
    "limit": 100,
    "next_cursor": "cursor_123"
  }
}
```

#### Failure example and fix

```json
{
  "tool": "observe",
  "arguments": {
    "what": "not_a_real_mode",
    "correlation_id": "cmd_123"
  }
}
```

Fix: Use a valid observe mode value, e.g. `command_result`.

### `pending_commands`

#### Minimal call

```json
{
  "tool": "observe",
  "arguments": {
    "what": "pending_commands"
  }
}
```

#### Expected response shape

```json
{
  "what": "pending_commands",
  "items": [
    {
      "id": "sample",
      "summary": "...mode-specific payload..."
    }
  ],
  "metadata": {
    "limit": 100,
    "next_cursor": "cursor_123"
  }
}
```

#### Failure example and fix

```json
{
  "tool": "observe",
  "arguments": {
    "what": "not_a_real_mode"
  }
}
```

Fix: Use a valid observe mode value, e.g. `pending_commands`.

### `failed_commands`

#### Minimal call

```json
{
  "tool": "observe",
  "arguments": {
    "what": "failed_commands"
  }
}
```

#### Expected response shape

```json
{
  "what": "failed_commands",
  "items": [
    {
      "id": "sample",
      "summary": "...mode-specific payload..."
    }
  ],
  "metadata": {
    "limit": 100,
    "next_cursor": "cursor_123"
  }
}
```

#### Failure example and fix

```json
{
  "tool": "observe",
  "arguments": {
    "what": "not_a_real_mode"
  }
}
```

Fix: Use a valid observe mode value, e.g. `failed_commands`.

### `saved_videos`

#### Minimal call

```json
{
  "tool": "observe",
  "arguments": {
    "what": "saved_videos"
  }
}
```

#### Expected response shape

```json
{
  "what": "saved_videos",
  "items": [
    {
      "id": "sample",
      "summary": "...mode-specific payload..."
    }
  ],
  "metadata": {
    "limit": 100,
    "next_cursor": "cursor_123"
  }
}
```

#### Failure example and fix

```json
{
  "tool": "observe",
  "arguments": {
    "what": "not_a_real_mode"
  }
}
```

Fix: Use a valid observe mode value, e.g. `saved_videos`.

### `recordings`

#### Minimal call

```json
{
  "tool": "observe",
  "arguments": {
    "what": "recordings"
  }
}
```

#### Expected response shape

```json
{
  "what": "recordings",
  "items": [
    {
      "id": "sample",
      "summary": "...mode-specific payload..."
    }
  ],
  "metadata": {
    "limit": 100,
    "next_cursor": "cursor_123"
  }
}
```

#### Failure example and fix

```json
{
  "tool": "observe",
  "arguments": {
    "what": "not_a_real_mode"
  }
}
```

Fix: Use a valid observe mode value, e.g. `recordings`.

### `recording_actions`

#### Minimal call

```json
{
  "tool": "observe",
  "arguments": {
    "what": "recording_actions",
    "recording_id": "rec_123"
  }
}
```

#### Expected response shape

```json
{
  "what": "recording_actions",
  "items": [
    {
      "id": "sample",
      "summary": "...mode-specific payload..."
    }
  ],
  "metadata": {
    "limit": 100,
    "next_cursor": "cursor_123"
  }
}
```

#### Failure example and fix

```json
{
  "tool": "observe",
  "arguments": {
    "what": "recording_actions",
    "recording_id": 123
  }
}
```

Fix: Use `recording_id` as a string like `rec_123`.

### `playback_results`

#### Minimal call

```json
{
  "tool": "observe",
  "arguments": {
    "what": "playback_results",
    "recording_id": "rec_123"
  }
}
```

#### Expected response shape

```json
{
  "what": "playback_results",
  "items": [
    {
      "id": "sample",
      "summary": "...mode-specific payload..."
    }
  ],
  "metadata": {
    "limit": 100,
    "next_cursor": "cursor_123"
  }
}
```

#### Failure example and fix

```json
{
  "tool": "observe",
  "arguments": {
    "what": "playback_results",
    "recording_id": 123
  }
}
```

Fix: Use `recording_id` as a string like `rec_123`.

### `log_diff_report`

#### Minimal call

```json
{
  "tool": "observe",
  "arguments": {
    "what": "log_diff_report",
    "original_id": "rec_123",
    "replay_id": "rec_456"
  }
}
```

#### Expected response shape

```json
{
  "what": "log_diff_report",
  "items": [
    {
      "id": "sample",
      "summary": "...mode-specific payload..."
    }
  ],
  "metadata": {
    "limit": 100,
    "next_cursor": "cursor_123"
  }
}
```

#### Failure example and fix

```json
{
  "tool": "observe",
  "arguments": {
    "what": "not_a_real_mode",
    "original_id": "rec_123",
    "replay_id": "rec_456"
  }
}
```

Fix: Use a valid observe mode value, e.g. `log_diff_report`.

### `summarized_logs`

#### Minimal call

```json
{
  "tool": "observe",
  "arguments": {
    "what": "summarized_logs"
  }
}
```

#### Expected response shape

```json
{
  "what": "summarized_logs",
  "items": [
    {
      "id": "sample",
      "summary": "...mode-specific payload..."
    }
  ],
  "metadata": {
    "limit": 100,
    "next_cursor": "cursor_123"
  }
}
```

#### Failure example and fix

```json
{
  "tool": "observe",
  "arguments": {
    "what": "not_a_real_mode"
  }
}
```

Fix: Use a valid observe mode value, e.g. `summarized_logs`.

### `page_inventory`

#### Minimal call

```json
{
  "tool": "observe",
  "arguments": {
    "what": "page_inventory"
  }
}
```

#### Expected response shape

```json
{
  "what": "page_inventory",
  "items": [
    {
      "id": "sample",
      "summary": "...mode-specific payload..."
    }
  ],
  "metadata": {
    "limit": 100,
    "next_cursor": "cursor_123"
  }
}
```

#### Failure example and fix

```json
{
  "tool": "observe",
  "arguments": {
    "what": "not_a_real_mode"
  }
}
```

Fix: Use a valid observe mode value, e.g. `page_inventory`.

### `transients`

#### Minimal call

```json
{
  "tool": "observe",
  "arguments": {
    "what": "transients"
  }
}
```

#### Expected response shape

```json
{
  "what": "transients",
  "items": [
    {
      "id": "sample",
      "summary": "...mode-specific payload..."
    }
  ],
  "metadata": {
    "limit": 100,
    "next_cursor": "cursor_123"
  }
}
```

#### Failure example and fix

```json
{
  "tool": "observe",
  "arguments": {
    "what": "not_a_real_mode"
  }
}
```

Fix: Use a valid observe mode value, e.g. `transients`.

### `inbox`

#### Minimal call

```json
{
  "tool": "observe",
  "arguments": {
    "what": "inbox"
  }
}
```

#### Expected response shape

```json
{
  "what": "inbox",
  "items": [
    {
      "id": "sample",
      "summary": "...mode-specific payload..."
    }
  ],
  "metadata": {
    "limit": 100,
    "next_cursor": "cursor_123"
  }
}
```

#### Failure example and fix

```json
{
  "tool": "observe",
  "arguments": {
    "what": "not_a_real_mode"
  }
}
```

Fix: Use a valid observe mode value, e.g. `inbox`.
