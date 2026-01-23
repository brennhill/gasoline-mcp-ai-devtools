---
title: "Roadmap — AI-First Features"
description: "Upcoming Gasoline features built for AI-native development: compressed state diffs, noise filtering, behavioral baselines, persistent memory, API schema inference, and DOM fingerprinting."
keywords: "AI-first debugging, AI coding agent features, compressed state diffs, noise filtering, behavioral baselines, persistent memory, API schema inference, DOM fingerprinting, token-efficient debugging"
permalink: /roadmap/
header:
  overlay_image: /assets/images/hero-banner.png
  overlay_filter: 0.85
  excerpt: "What's next: features designed for a world where AI agents are the primary coders."
toc: true
toc_sticky: true
---

These features are designed for the next generation of AI coding — where agents run tight edit-verify loops, need peripheral awareness, and accumulate understanding over time.

## <i class="fas fa-bolt"></i> Compressed State Diffs

**Status:** Specification Complete
{: .notice--info}

### The Problem

AI agents in edit-verify loops waste 10-20K tokens per state check, re-reading entire log buffers to find what changed. Over a 50-edit session, that's 500K-1M tokens wasted on state reads.

### The Solution

`get_changes_since` returns only what changed since the last check — token-efficient deltas instead of full state dumps.

```json
{
  "summary": "1 new console error, 1 network failure (POST /api/users 500), error banner visible",
  "severity": "error",
  "token_count": 287
}
```

**Target:** 95% token reduction for state verification. 10x faster response. < 5% false alarm rate.

---

## <i class="fas fa-filter"></i> Noise Filtering

**Status:** Specification Complete
{: .notice--info}

### The Problem

A typical page load produces dozens of irrelevant entries — extension errors, favicon 404s, HMR logs, analytics failures. Humans ignore these reflexively. AI agents can't distinguish "favicon 404" from "critical API 404" without explicit classification.

### The Solution

`configure_noise` and `dismiss_noise` classify noise automatically. Built-in heuristics catch common patterns (extensions, HMR, analytics). Statistical detection catches the rest.

Auto-detection proposes rules with confidence scores:

```json
{
  "rule": {"category": "console", "match": {"source_pattern": "^chrome-extension://.*"}},
  "evidence": "12 entries from 3 extensions, none application-related",
  "confidence": 0.99
}
```

**Target:** 90% precision (don't filter real errors), 80% recall (catch most noise). < 5% false investigation rate.

---

## <i class="fas fa-chart-bar"></i> Behavioral Baselines

**Status:** Specification Complete
{: .notice--info}

### The Problem

AI agents don't know what "normal" looks like for your app. When they see 3 network requests taking 200ms each, they can't tell if that's fast or slow for your system. They need a reference point.

### The Solution

`save_baseline` captures what "correct" looks like. `compare_baseline` detects regressions against that reference — without needing explicit test assertions.

Use cases:
- Save baseline after fixing a bug → detect if it regresses
- Save baseline for production behavior → detect drift in development
- Save baseline for performance → detect latency regressions

---

## <i class="fas fa-brain"></i> Persistent Memory

**Status:** Specification Complete
{: .notice--info}

### The Problem

Every AI session starts from scratch. The agent re-discovers which errors are noise, re-learns API schemas, and re-investigates the same false positives. There's no continuity between sessions.

### The Solution

`session_store` and `load_session_context` give agents persistent memory across sessions:

- Noise rules persist (don't re-learn what's irrelevant)
- API schemas persist (don't re-infer structure)
- Baselines persist (regression detection works across days)
- Known errors persist (don't re-investigate the same issue)

---

## <i class="fas fa-project-diagram"></i> API Schema Inference

**Status:** Specification Complete
{: .notice--info}

### The Problem

AI agents need to understand your API contracts to debug integration issues. Today they read documentation (if it exists) or guess from error messages. Neither is reliable.

### The Solution

`get_api_schema` learns API contracts from observed traffic — request/response shapes, status code patterns, and timing characteristics. Your AI knows the API without reading docs.

```json
{
  "endpoint": "POST /api/users",
  "request_shape": {"email": "string", "name": "string", "role": "enum(admin,user)"},
  "response_shapes": {
    "201": {"id": "number", "email": "string", "created_at": "datetime"},
    "422": {"errors": {"field": "string"}}
  },
  "avg_latency_ms": 145
}
```

---

## <i class="fas fa-fingerprint"></i> DOM Fingerprinting

**Status:** Specification Complete
{: .notice--info}

### The Problem

Verifying UI correctness typically requires vision models or screenshot comparison — both expensive and brittle. An agent needs a way to structurally verify "the page looks right" without pixel comparison.

### The Solution

`get_dom_fingerprint` and `compare_dom_fingerprint` create structural hashes of the page:

- Detect unexpected DOM changes (elements added/removed/reordered)
- Verify component rendering without screenshots
- Catch CSS-invisible regressions (wrong structure, correct appearance)
- Works as a component of baselines and diffs

---

## <i class="fas fa-globe"></i> Platform Expansion

**Status:** Future
{: .notice}

### Firefox Extension

The Chrome extension's WebExtensions API is ~90% compatible with Firefox. Main porting work: service worker → event page, `chrome.scripting` API differences.

**Effort:** Low-medium. 1-2 days of porting + testing.

**Note:** Edge, Brave, Arc, Vivaldi, and Opera already work — they're Chromium-based and run the Chrome extension unmodified.

### React Native

Tap into React Native's debug bridge to capture LogBox errors, network requests, and component tree state. Forward to the Gasoline MCP server over the local network.

**Effort:** Medium. New companion package, not an extension.

### Flutter

Dart DevTools extension or `debugPrint` interceptor that forwards runtime events to the Gasoline MCP server.

**Effort:** Medium. Dart package + DevTools integration.

### Native iOS / Android

Stream system logs (`os_log` on iOS, Logcat on Android) to the Gasoline MCP server via a CLI companion tool. Zero app modification required — purely observational.

**Effort:** Low per platform. CLI tool that pipes structured log output to the existing server.

---

## Priority Order

| # | Feature | Why First |
|---|---------|-----------|
| 1 | Compressed Diffs | Unblocks tight feedback loops (token efficiency) |
| 2 | Noise Filtering | Makes all other signals useful (reduces false positives) |
| 3 | Behavioral Baselines | Enables regression detection without tests |
| 4 | Persistent Memory | Agent accumulates understanding over time |
| 5 | API Schema Inference | Agent understands the system without docs |
| 6 | DOM Fingerprinting | Structural UI verification without vision models |

## <i class="fas fa-dollar-sign"></i> Economic Impact

Combined value per developer per year:

| Savings Source | Estimated Value |
|---------------|----------------|
| Token reduction (compressed diffs) | $3,600-4,800/year |
| Time saved (faster feedback loops) | $12,480/year |
| Fewer false positives (noise filtering) | $4,160/year |
| No re-investigation (persistent memory) | $4,648-5,570/year |
| **Total** | **$24,888-27,010/year** |

Zero cost. Open source. Replaces $65-90K/year commercial alternatives.
