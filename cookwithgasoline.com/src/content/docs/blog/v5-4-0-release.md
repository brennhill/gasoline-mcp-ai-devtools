---
title: "Gasoline v0.5.40 Released"
description: "Redesigned interaction model and improved AI agent integration"
date: 2026-01-17T22:18:00Z
authors: [brennhill]
tags: [release]
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
normalized_tags: ['release', 'blog', 'v0.5']
---

## What's New in v0.5.40

Gasoline v0.5.40 redesigns the interaction model for better AI agent integration and reliability.

### Features

- **Improved interact() Tool** — More reliable element selection and action execution
- **Visual Feedback** — Toast notifications for AI-triggered interactions
- **Better Form Handling** — Enhanced form filling with validation awareness
- **Robust Navigation** — Improved page load detection and state tracking

### Improvements

- More deterministic element targeting with semantic selectors
- Better handling of dynamic content and SPA navigation
- Improved timeout handling for slow operations
- Enhanced compatibility with modern web frameworks

### Fixes

- Fixed element visibility detection on overlaid modals
- Resolved form submission race conditions
- Improved navigation state after dialog close

## Upgrade

```bash
curl -sSL https://raw.githubusercontent.com/brennhill/gasoline-agentic-browser-devtools-mcp/STABLE/scripts/install.sh | bash
```

## Full Changelog

[v0.5.40 Release](https://github.com/brennhill/gasoline-agentic-browser-devtools-mcp/releases/tag/v5.4.0)
