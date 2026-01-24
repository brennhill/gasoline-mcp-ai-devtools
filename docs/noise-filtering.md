---
title: "Noise Filtering"
description: "Automatically detect and dismiss irrelevant console errors, network noise, and WebSocket chatter. Your AI focuses on real issues instead of wading through noise."
keywords: "noise filtering, error filtering, console noise, irrelevant errors, signal to noise, auto-detect noise, dismiss errors, MCP filtering"
permalink: /noise-filtering/
header:
  overlay_image: /assets/images/hero-banner.png
  overlay_filter: 0.85
  excerpt: "Filter the noise. Focus on what matters."
toc: true
toc_sticky: true
---

Gasoline's noise filtering lets your AI automatically identify and suppress irrelevant browser output — third-party script errors, analytics noise, and repetitive warnings — so it can focus on the errors that actually matter.

## <i class="fas fa-exclamation-circle"></i> The Problem

A typical web app's console is full of noise:
- Third-party analytics scripts logging deprecation warnings
- Ad network errors that have nothing to do with your code
- Browser extension interference
- CORS preflight noise from CDNs
- Repetitive "favicon.ico 404" errors

Your AI assistant sees all of this and wastes tokens (and your time) investigating issues that aren't yours to fix. Noise filtering teaches it what to ignore.

## <i class="fas fa-magic"></i> Auto-Detection

The fastest way to filter noise is automatic detection:

```json
{ "tool": "configure", "arguments": {
  "action": "noise_rule",
  "noise_action": "auto_detect"
} }
```

Gasoline analyzes current buffer contents and identifies patterns that look like noise:
- High-frequency repeated messages
- Known third-party script patterns
- Common browser-generated warnings
- Errors from domains you don't control

## <i class="fas fa-terminal"></i> Manual Rules

For precise control, add rules matching specific patterns:

```json
// Dismiss all errors from Google Analytics
{ "tool": "configure", "arguments": {
  "action": "noise_rule",
  "noise_action": "add",
  "rules": [{
    "category": "console",
    "classification": "third-party-analytics",
    "matchSpec": {
      "sourceRegex": "google-analytics\\.com|googletagmanager\\.com"
    }
  }]
} }

// Dismiss 404s to favicon
{ "tool": "configure", "arguments": {
  "action": "noise_rule",
  "noise_action": "add",
  "rules": [{
    "category": "network",
    "classification": "expected-missing-resource",
    "matchSpec": {
      "urlRegex": "favicon\\.ico",
      "statusMin": 404,
      "statusMax": 404
    }
  }]
} }
```

## <i class="fas fa-hand-paper"></i> Quick Dismiss

For one-off noise patterns, use dismiss without creating a persistent rule:

```json
// Dismiss a specific error pattern for this session
{ "tool": "configure", "arguments": {
  "action": "dismiss",
  "category": "console",
  "pattern": "ResizeObserver loop limit exceeded",
  "reason": "Browser-generated, not actionable"
} }
```

## <i class="fas fa-list"></i> Managing Rules

```json
// List all active noise rules
{ "tool": "configure", "arguments": {
  "action": "noise_rule",
  "noise_action": "list"
} }

// Remove a specific rule by ID
{ "tool": "configure", "arguments": {
  "action": "noise_rule",
  "noise_action": "remove",
  "rule_id": "rule-abc123"
} }

// Reset all rules
{ "tool": "configure", "arguments": {
  "action": "noise_rule",
  "noise_action": "reset"
} }
```

## <i class="fas fa-filter"></i> Match Specifications

Rules can match on multiple criteria:

| Field | Applies To | Description |
|-------|-----------|-------------|
| `messageRegex` | console | Regex against log message content |
| `sourceRegex` | console | Regex against error source URL |
| `urlRegex` | network, websocket | Regex against request/connection URL |
| `method` | network | HTTP method (GET, POST, etc.) |
| `statusMin` | network | Minimum status code |
| `statusMax` | network | Maximum status code |
| `level` | console | Log level (error, warn, info) |

Multiple fields in a single rule are AND-matched — all must match for the rule to apply.

## <i class="fas fa-search"></i> What Your AI Can Do With This

- **Clean up on first connect** — "I see 47 errors, but 42 are from third-party scripts. Let me auto-detect and filter those."
- **Focus on your bugs** — "After filtering analytics noise, there are 5 real errors — all from your checkout component."
- **Reduce token waste** — Filtered entries don't appear in `observe` responses, saving context window space.
- **Learn over time** — Rules persist for the session, so noise stays filtered as you continue working.

## <i class="fas fa-link"></i> Related

- [Session Checkpoints](/session-checkpoints/) — Diff only meaningful changes after filtering
- [Web Vitals](/web-vitals/) — Performance data is never filtered
