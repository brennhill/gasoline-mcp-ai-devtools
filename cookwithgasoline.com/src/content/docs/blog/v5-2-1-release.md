---
title: "Gasoline v5.2.1 Released"
description: "Hotfix for error clustering and network filtering"
date: 2026-01-10T21:47:00Z
authors: [brennhill]
tags: [release]
---

## What's New in v5.2.1

Quick patch release fixing issues with error clustering in v5.2.0.

### Fixes

- Fixed error clustering not deduplicating similar stack frames
- Resolved network filter persistence across tab navigation
- Improved WebSocket connection tracking stability

## Upgrade

```bash
npm install -g gasoline-mcp@5.2.1
```

## Full Changelog

[v5.2.1 Release](https://github.com/brennhill/gasoline-agentic-browser-devtools-mcp/releases/tag/v5.2.1)
