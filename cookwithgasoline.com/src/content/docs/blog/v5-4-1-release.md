---
title: "Gasoline v0.5.41 Released"
description: "Hotfix for interaction reliability"
date: 2026-01-18
authors: [brennhill]
tags: [release]
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
normalized_tags: ['release', 'blog', 'v0.5']
---

## What's New in v0.5.41

Quick patch release fixing element selection and interaction issues in v0.5.40.

### Fixes

- Fixed semantic selector matching on shadow DOM elements
- Resolved toast visibility on pages with z-index layering
- Improved form field detection for hidden labels
- Better handling of contenteditable elements

## Upgrade

```bash
curl -sSL https://raw.githubusercontent.com/brennhill/gasoline-agentic-browser-devtools-mcp/STABLE/scripts/install.sh | bash
```

## Full Changelog

[v0.5.41 Release](https://github.com/brennhill/gasoline-agentic-browser-devtools-mcp/releases/tag/v5.4.1)
