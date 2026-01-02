---
title: "Log Format Reference"
description: "Gasoline log format documentation. JSONL structured logs with enrichments for console errors, network failures, exceptions, screenshots, and more."
keywords: "browser log format, JSONL structured logs, browser error format, log enrichments, error grouping"
permalink: /log-format/
toc: true
toc_sticky: true
---

Logs are stored in [JSONL format](https://jsonlines.org/) (one JSON object per line). Each entry includes an `_enrichments` array that lists what additional data is attached.

## Basic Log Entries

```jsonl
{"ts":"2024-01-22T10:30:00.000Z","level":"error","type":"console","args":["Error message"],"url":"http://localhost:3000/app"}
{"ts":"2024-01-22T10:30:01.000Z","level":"error","type":"network","method":"POST","url":"http://localhost:8789/api","status":401,"response":{"error":"Unauthorized"}}
{"ts":"2024-01-22T10:30:02.000Z","level":"error","type":"exception","message":"Cannot read property 'x' of undefined","stack":"...","filename":"app.js","lineno":42}
```

## Entry Types

| Type | Description | Key Fields |
|------|-------------|------------|
| `console` | Console API calls | `level`, `args` |
| `network` | Failed HTTP requests (4xx, 5xx) | `method`, `url`, `status`, `response`, `duration` |
| `exception` | Uncaught errors & promise rejections | `message`, `stack`, `filename`, `lineno`, `colno` |
| `screenshot` | Page screenshot (saved as JPEG file) | `screenshotFile`, `trigger`, `url` |
| `network_waterfall` | Network timing data | `entries`, `pending` |
| `performance` | Performance marks/measures | `marks`, `measures`, `navigation` |

## Enrichments

The `_enrichments` array tells you what additional data is attached to an entry:

```jsonl
{"type":"exception","level":"error","_enrichments":["context","userActions","sourceMap"],...}
```

| Enrichment | Description | Added When |
|-----------|-------------|-----------|
| `context` | Developer-set annotations via `__gasoline.annotate()` | Error has context annotations |
| `userActions` | Recent clicks, inputs, scrolls before error | Error entry with action buffer |
| `sourceMap` | Stack trace resolved via source maps | Source map resolution enabled & successful |
| `networkWaterfall` | Network timing data | Network waterfall entry |
| `performanceMarks` | Performance marks/measures | Performance entry |
| `aiContext` | Component ancestry and app state | AI context enrichment enabled |

## Enriched Error Example

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

## Linked Enrichment Entries

Some enrichments are sent as separate entries linked by `_errorTs` or `relatedErrorId`:

```jsonl
{"type":"exception","ts":"2024-01-22T10:30:00.000Z","level":"error","message":"...","_errorId":"err_1705921800000_abc123"}
{"type":"network_waterfall","ts":"2024-01-22T10:30:00.100Z","_enrichments":["networkWaterfall"],"_errorTs":"2024-01-22T10:30:00.000Z","entries":[...]}
{"type":"screenshot","ts":"2024-01-22T10:30:00.200Z","level":"info","_enrichments":["screenshot"],"relatedErrorId":"err_1705921800000_abc123","screenshotFile":"localhost-20240122-103000-exception-err_1705921800000_abc123.jpg","trigger":"error"}
```

## Error Grouping

Repeated errors within 5 seconds are deduplicated. Grouped entries include:

```json
{
  "type": "exception",
  "_aggregatedCount": 15,
  "_firstSeen": "2024-01-22T10:30:00.000Z",
  "_lastSeen": "2024-01-22T10:30:04.500Z"
}
```

## Rate Limiting

When errors cascade rapidly (e.g., a render loop), Gasoline prevents log flooding:

- **First occurrence** is sent immediately with full context
- **Subsequent duplicates** increment a counter
- **After 5–10s**, an aggregated entry is sent with `_aggregatedCount`

| Feature | Limit | Reason |
|---------|-------|--------|
| Screenshots | 5s between, 10/session max | Large file size (~100-500KB each) |
| Network Waterfall | 50 entries, 30s window | Reads existing browser data |
| Performance Marks | 50 entries, 60s window | Reads existing browser data |
| User Actions | 20 item buffer, scroll throttled | Lightweight metadata |
