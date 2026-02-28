---
title: "Gasoline v5.4.1 Released"
description: "Hotfix for interaction reliability"
date: 2026-01-18
authors: [brennhill]
tags: [release]
---

## What's New in v5.4.1

Quick patch release fixing element selection and interaction issues in v5.4.0.

### Fixes

- Fixed semantic selector matching on shadow DOM elements
- Resolved toast visibility on pages with z-index layering
- Improved form field detection for hidden labels
- Better handling of contenteditable elements

## Upgrade

```bash
npm install -g gasoline-mcp@5.4.1
```

## Full Changelog

[v5.4.1 Release](https://github.com/brennhill/gasoline-mcp-ai-devtools/releases/tag/v5.4.1)
