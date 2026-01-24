---
title: "PR Summaries"
description: "Generate pull request performance impact summaries from browser session data. Gasoline compares before and after metrics so reviewers understand the performance implications of code changes."
keywords: "PR summary, pull request performance, performance impact, code review, performance comparison, before after metrics, deployment impact"
permalink: /pr-summaries/
header:
  overlay_image: /assets/images/hero-banner.png
  overlay_filter: 0.85
  excerpt: "Show reviewers the performance impact of your code change."
toc: true
toc_sticky: true
---

Gasoline generates markdown performance summaries for pull requests — comparing baseline metrics against current observations so code reviewers can see the real-world impact of changes.

## <i class="fas fa-exclamation-circle"></i> The Problem

Code reviews focus on logic and style, but rarely consider performance impact. A reviewer can't tell from a diff whether:
- The new API call adds 200ms to page load
- The refactored component causes layout shifts
- The bundle size increased by 500KB
- An endpoint that was fast is now slow

Performance impact is invisible in code review. PR summaries make it visible.

## <i class="fas fa-cogs"></i> How It Works

1. <i class="fas fa-ruler"></i> Gasoline maintains baselines from before your changes (see [Regression Detection](/regression-detection/))
2. <i class="fas fa-code-branch"></i> You test your feature branch in the browser
3. <i class="fas fa-file-alt"></i> Your AI calls `generate` with `format: "pr_summary"`
4. <i class="fas fa-clipboard"></i> A markdown summary is produced, ready to paste into your PR description

## <i class="fas fa-terminal"></i> Usage

```json
{ "tool": "generate", "arguments": { "format": "pr_summary" } }
```

## <i class="fas fa-file-alt"></i> Generated Output

The summary includes:

```markdown
## Performance Impact

### Page Load Metrics
| Metric | Baseline | Current | Delta |
|--------|----------|---------|-------|
| Load Time | 2100ms | 2450ms | +350ms ⚠️ |
| LCP | 1800ms | 1900ms | +100ms |
| CLS | 0.02 | 0.02 | — |
| FCP | 1200ms | 1250ms | +50ms |

### Network
| Metric | Baseline | Current | Delta |
|--------|----------|---------|-------|
| Requests | 32 | 38 | +6 |
| Transfer Size | 1.4MB | 1.9MB | +500KB ⚠️ |

### Regressions Detected
- ⚠️ Load time increased 17% (within threshold but notable)
- ⚠️ Transfer size increased 36% — new dependencies added

### Endpoints Changed
| Endpoint | Baseline Latency | Current | Status |
|----------|-----------------|---------|--------|
| /api/users | 120ms | 125ms | ✅ |
| /api/orders | 200ms | 580ms | ⚠️ 2.9× |
```

## <i class="fas fa-search"></i> What Your AI Can Do With This

- **Add to PR descriptions** — Paste the summary so reviewers see performance impact at a glance.
- **Block regressions early** — "This PR adds 350ms to page load. Should we investigate before merging?"
- **Validate optimizations** — "The lazy loading PR reduced transfer size by 40% and LCP by 600ms."
- **Track endpoint impact** — "The new middleware is adding 380ms to the orders endpoint."

## <i class="fas fa-lightbulb"></i> Best Practice

Use with [Session Checkpoints](/session-checkpoints/) for the clearest before/after comparison:

1. Create a checkpoint on the `main` branch: `checkpoint: "main-baseline"`
2. Switch to your feature branch
3. Browse the same pages
4. Generate the PR summary — it compares against the checkpoint

## <i class="fas fa-link"></i> Related

- [Regression Detection](/regression-detection/) — The baseline system that powers summaries
- [Web Vitals](/web-vitals/) — Core metrics included in summaries
- [Session Checkpoints](/session-checkpoints/) — Named before/after comparison points
