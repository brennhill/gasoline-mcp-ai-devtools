---
title: "Gasoline v5.2.5 Released"
description: "Stability and reliability improvements"
date: 2026-01-12T23:52:00Z
authors: [brennhill]
tags: [release]
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
normalized_tags: ['release', 'blog', 'v5']
---

## What's New in v5.2.5

Gasoline v5.2.5 focuses on stability and fixes edge cases discovered in production use.

### Improvements

- Better handling of rapid tab switching
- Improved memory cleanup on long sessions
- Fixed race conditions in message queue
- Enhanced resilience to malformed responses

### Fixes

- Resolved observer disconnection on network errors
- Fixed log queue overflow handling
- Improved error message formatting for edge cases

## Upgrade

```bash
npm install -g gasoline-mcp@5.2.5
```

## Full Changelog

[v5.2.5 Release](https://github.com/brennhill/gasoline-agentic-browser-devtools-mcp/releases/tag/v5.2.5)
