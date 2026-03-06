---
title: "Gasoline v0.5.43 Released"
description: "Bug fixes and stability improvements"
date: 2026-01-18
authors: [brennhill]
tags: [release]
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
normalized_tags: ['release', 'blog', 'v0.5']
---

## What's New in v0.5.43

Gasoline v0.5.43 includes additional bug fixes and stability improvements following v0.5.41.

### Fixes

- Fixed click handling on elements inside iframes
- Resolved memory leak in toast notification cleanup
- Improved robustness of element scrolling
- Better error recovery when pages reload during interaction

### Performance

- Reduced CPU usage during idle observation
- Optimized selector matching algorithm

## Upgrade

```bash
curl -sSL https://raw.githubusercontent.com/brennhill/gasoline-agentic-browser-devtools-mcp/STABLE/scripts/install.sh | bash
```

## Full Changelog

[v0.5.43 Release](https://github.com/brennhill/gasoline-agentic-browser-devtools-mcp/releases/tag/v5.4.3)
