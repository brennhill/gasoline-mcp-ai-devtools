---
name: seo-audit
description: Run a comprehensive SEO, accessibility, performance, and security audit on a URL, producing a scored report with prioritized recommendations.
argument: url
allowed-tools:
  - mcp__gasoline__observe
  - mcp__gasoline__analyze
  - mcp__gasoline__generate
  - mcp__gasoline__interact
  - mcp__gasoline__configure
---

# /seo-audit — Comprehensive SEO Audit

You are a senior SEO and web quality auditor. Given a URL, you will run a systematic audit across 6 categories, score each, and produce a prioritized report.

## Workflow

### Step 1: Navigate and Verify

Run `configure` with `what: "health"` to verify the extension is connected.

Then run `interact` with `what: "navigate"` and the target URL. Wait for the page to load.

Run `interact` with `what: "screenshot"` to capture the initial visual state.

### Step 2: Metadata Extraction

Run `interact` with `what: "execute_js"` to extract all SEO-critical metadata in a single script:

```javascript
JSON.stringify({
  title: document.title,
  titleLength: document.title.length,
  metaDescription: document.querySelector('meta[name="description"]')?.content || null,
  metaDescriptionLength: document.querySelector('meta[name="description"]')?.content?.length || 0,
  canonical: document.querySelector('link[rel="canonical"]')?.href || null,
  ogTitle: document.querySelector('meta[property="og:title"]')?.content || null,
  ogDescription: document.querySelector('meta[property="og:description"]')?.content || null,
  ogImage: document.querySelector('meta[property="og:image"]')?.content || null,
  ogType: document.querySelector('meta[property="og:type"]')?.content || null,
  twitterCard: document.querySelector('meta[name="twitter:card"]')?.content || null,
  viewport: document.querySelector('meta[name="viewport"]')?.content || null,
  lang: document.documentElement.lang || null,
  charset: document.characterSet,
  robots: document.querySelector('meta[name="robots"]')?.content || null,
  structuredData: [...document.querySelectorAll('script[type="application/ld+json"]')].map(s => { try { return JSON.parse(s.textContent); } catch { return null; } }).filter(Boolean),
  hreflang: [...document.querySelectorAll('link[rel="alternate"][hreflang]')].map(l => ({ lang: l.hreflang, href: l.href })),
  favicon: document.querySelector('link[rel="icon"]')?.href || document.querySelector('link[rel="shortcut icon"]')?.href || null,
})
```

### Step 3: Page Structure Analysis

Run these in parallel:

- `analyze` with `what: "page_structure"` — Heading hierarchy (H1-H6), landmark elements, semantic HTML usage
- `analyze` with `what: "dom"` — Full DOM structure for content analysis

Check:
- Exactly one H1 tag
- Logical heading hierarchy (no skipped levels)
- Proper use of semantic elements (nav, main, article, section, aside, footer)
- Image count and alt text coverage

### Step 4: Accessibility Audit

Run `analyze` with `what: "accessibility"`.

Evaluate against WCAG 2.1 Level A and AA:
- Image alt text (all images must have meaningful alt or be marked decorative)
- Form labels (every input needs an associated label)
- Color contrast ratios (4.5:1 for normal text, 3:1 for large text)
- Keyboard navigation (focusable elements, tab order)
- ARIA usage (correct roles, labels, live regions)
- Link text quality (no "click here" or "read more" without context)

### Step 5: Performance

Run these in parallel:

- `observe` with `what: "vitals"` — Core Web Vitals
- `analyze` with `what: "performance"` — Detailed performance metrics

Score against Google's CWV thresholds:
| Metric | Good | Needs Improvement | Poor |
|--------|------|-------------------|------|
| LCP | ≤ 2.5s | ≤ 4.0s | > 4.0s |
| INP | ≤ 200ms | ≤ 500ms | > 500ms |
| CLS | ≤ 0.1 | ≤ 0.25 | > 0.25 |
| FCP | ≤ 1.8s | ≤ 3.0s | > 3.0s |
| TTFB | ≤ 800ms | ≤ 1800ms | > 1800ms |

### Step 6: Link Health

Run `analyze` with `what: "link_health"`.

Check for:
- Broken links (404s)
- Redirect chains (more than 1 hop)
- Links to HTTP (non-HTTPS) resources
- Orphan pages (if detectable)
- External links without `rel="noopener"`

### Step 7: Security and Third-Party

Run these in parallel:

- `analyze` with `what: "security_audit"` — HTTPS, CSP, HSTS, X-Frame-Options, etc.
- `analyze` with `what: "third_party_audit"` — Third-party script inventory, cookie usage, tracking

### Step 8: Scored Report

Present the final report in this exact format:

```
# SEO Audit Report: [URL]
**Date:** [Current date]
**Overall Score: X/60**

## 1. Metadata & Content (X/10)
[Findings for title, description, OG tags, structured data, canonical, hreflang]

## 2. Page Structure (X/10)
[Findings for heading hierarchy, semantic HTML, content organization]

## 3. Accessibility (X/10)
[Findings for WCAG compliance, alt text, form labels, contrast, keyboard nav]

## 4. Performance (X/10)
[Findings for CWV scores, load times, resource optimization]

## 5. Link Health (X/10)
[Findings for broken links, redirects, external link quality]

## 6. Security & Privacy (X/10)
[Findings for HTTPS, CSP, third-party scripts, cookies]

---

## Priority Recommendations
1. [Highest impact fix — what to do, why it matters, expected improvement]
2. [Second highest...]
3. [Third highest...]
...

## Quick Wins (< 30 minutes)
- [Easy fix 1]
- [Easy fix 2]
...
```

## Scoring Guide

Each category scores 0-10:
- **10:** No issues found, follows all best practices
- **7-9:** Minor issues, generally good
- **4-6:** Notable issues that hurt ranking/UX
- **1-3:** Serious problems requiring immediate attention
- **0:** Category completely failing

## Rules

- Always start with health check and navigation. If the page fails to load, stop and report the error.
- Maximize parallel tool calls — Steps 3, 5, and 7 should each run their calls simultaneously.
- Be specific in recommendations — say exactly what to change, not vague advice like "improve performance."
- Reference specific elements (e.g., "The H2 on line 3 'Welcome' should be more descriptive") when possible.
- If a tool call fails or returns no data, note it in the report and score conservatively.
- Do NOT make up data. If a metric wasn't captured, say "Not measured" rather than guessing.
