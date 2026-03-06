---
title: "Gasoline v0.5.21 Released"
description: "Hotfix for error clustering and network filtering"
date: 2026-01-10T21:47:00Z
authors: [brennhill]
tags: [release]
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
normalized_tags: ['release', 'blog', 'v0.5']
---

## What's New in v0.5.21

Quick patch release fixing issues with error clustering in v0.5.20.

### Fixes

- Fixed error clustering not deduplicating similar stack frames
- Resolved network filter persistence across tab navigation
- Improved WebSocket connection tracking stability

## Upgrade

```bash
curl -sSL https://raw.githubusercontent.com/brennhill/gasoline-agentic-browser-devtools-mcp/STABLE/scripts/install.sh | bash
```

## Full Changelog

[v0.5.21 Release](https://github.com/brennhill/gasoline-agentic-browser-devtools-mcp/releases/tag/v5.2.1)
