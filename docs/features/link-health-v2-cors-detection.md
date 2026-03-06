---
doc_type: legacy_doc
status: reference
last_reviewed: 2026-02-16
---

# Link Health Analyzer v2: CORS Detection & Server-Side Validation

## Problem: CORS Limitations in Browser-Based Link Checking

The initial link health analyzer (v6.0.0) used the browser extension to check all links on a page. While this works well for internal links (same domain), it has a critical limitation with external links:

**CORS (Cross-Origin Resource Sharing)** prevents the browser from reading HTTP status codes of cross-origin requests. When a CORS policy blocks a request, the response is "opaque"—the extension can't read `response.status`, `response.headers`, or even distinguish between:
- A genuinely broken link (404)
- A CORS-blocked link that's actually valid

### Example: The GitHub Link Bug

On https://cookwithgasoline.com/architecture/, there's a link to the C2 diagram:

```
https://github.com/anthropics/gasoline-mcp/blob/main/docs/architecture/diagrams/c2-containers.md
```

#### Result before fix:
- Browser extension: ✅ **FALSE POSITIVE** (appears as "ok")
- Server verification: ❌ **404 broken** (repository owner is wrong—should be `brennhill`, not `anthropics`)

The browser couldn't detect this broken link due to CORS, giving users unreliable results.

---

## Solution: Two-Tier Verification System

### Tier 1: Browser Extension (`analyze({what: 'link_health'})`

#### What it checks:
- ✅ Internal links (same origin)
- ✅ External links without CORS restrictions
- 🚩 **CORS-blocked links** — Now explicitly flagged instead of misclassified

#### New behavior:

1. When `response.status === 0`, the link is categorized as `cors_blocked` (not `broken`)
2. A `needsServerVerification` flag is set for external links
3. Returns summary count of `corsBlocked` and `needsServerVerification`

#### Example response:

```json
{
  "summary": {
    "totalLinks": 33,
    "ok": 28,
    "redirect": 2,
    "broken": 1,
    "timeout": 0,
    "corsBlocked": 2,
    "needsServerVerification": 2
  },
  "results": [
    {
      "url": "https://github.com/anthropics/gasoline-mcp/blob/main/docs/architecture/diagrams/c2-containers.md",
      "status": null,
      "code": "cors_blocked",
      "timeMs": 150,
      "isExternal": true,
      "error": "CORS policy blocked the request",
      "needsServerVerification": true
    }
  ]
}
```

### Tier 2: Server-Side Verification (`analyze({what: 'link_validation'})`

For CORS-blocked links, use the server-side tool:

```javascript
// Example: Verify links that were blocked by CORS
const response = await analyze({
  what: 'link_validation',
  urls: [
    'https://github.com/anthropics/gasoline-mcp/blob/main/docs/architecture/diagrams/c2-containers.md',
    'https://github.com/brennhill/gasoline-mcp-ai-devtools/blob/main/docs/architecture/diagrams/c2-containers.md'
  ],
  timeout_ms: 15000,
  max_workers: 20
});
```

#### What it does:
1. Makes actual HTTP HEAD/GET requests from the server
2. No CORS restrictions (server-side code doesn't face browser security policies)
3. Returns accurate status codes for all links
4. Concurrent checking with worker pool (up to 100 workers)

#### Example response:

```json
{
  "status": "completed",
  "total": 2,
  "results": [
    {
      "url": "https://github.com/anthropics/gasoline-mcp/blob/main/docs/architecture/diagrams/c2-containers.md",
      "status": 404,
      "code": "broken",
      "time_ms": 339,
      "error": ""
    },
    {
      "url": "https://github.com/brennhill/gasoline-mcp-ai-devtools/blob/main/docs/architecture/diagrams/c2-containers.md",
      "status": 200,
      "code": "ok",
      "time_ms": 1005,
      "redirect_to": ""
    }
  ]
}
```

---

## Why Browser-Based Checking Still Has Value

Even with CORS limitations, browser-based checking provides critical advantages:

1. **Security Context** — Inherits browser's authentication (cookies, session tokens)
   - Can verify links behind login pages
   - Checks reflect actual user experience

2. **Privacy** — All checking happens locally
   - No external service sees which links you're validating
   - Zero data transmission

3. **Accuracy for Internal Links** — 100% reliable for same-origin links
   - No CORS issues
   - Perfect for validating internal site structure

4. **User Context** — Extension runs in user's browser
   - Respects user's network (VPN, proxies)
   - Uses user's geographic location

### Example use case:
- An authenticated app checks internal links → Browser can verify
- Same app checks external CDN links with CORS → Browser flags as `cors_blocked`
- Engineering team then runs server-side verification on the flagged links

---

## Implementation Details

### Browser Extension Changes (`src/lib/link-health.ts`)

```typescript
// Detect CORS-blocked opaque responses (status 0)
if (response.status === 0) {
  return {
    url,
    status: null,
    code: 'cors_blocked',
    timeMs,
    isExternal,
    error: 'CORS policy blocked the request',
    needsServerVerification: isExternal,
  }
}
```

### Server-Side Tool (`cmd/dev-console/tools_analyze.go`)

```go
// New analyze mode: link_validation
// - Takes array of URLs
// - Makes HTTP requests from server
// - Returns accurate status codes
// - No CORS restrictions
```

---

## Workflow: Complete Link Validation

1. **Step 1: Browser Analysis** (Fast, 1-2s for 30 links)
   ```
   analyze({what: 'link_health'})
   ```
   Result: Categorizes links, flags CORS issues

2. **Step 2: Check Summary**
   - `corsBlocked` = 2 (needs server verification)
   - `broken` = 1 (confirmed broken by browser)
   - `ok` = 28 (confirmed working)

3. **Step 3: Server Verification** (For CORS-blocked links only)
   ```
   analyze({what: 'link_validation', urls: [cors_blocked_urls]})
   ```
   Result: Accurate status for external links

4. **Step 4: Complete Report**
   - Browser results + Server results = Full picture
   - User knows exact status of all links

---

## Test Results

### Architecture Page Analysis

#### Browser Analysis:
- Total links: 33
- Ok: 28
- Redirect: 2
- Broken: 1 (detected by browser)
- CORS Blocked: 2 (now properly flagged)

#### Server Verification of CORS-Blocked Links:

| URL | Status | Result |
|-----|--------|--------|
| github.com/anthropics/gasoline-mcp/... | 404 | ❌ Broken (wrong repo owner) |
| github.com/brennhill/gasoline-mcp-ai-devtools/... | 200 | ✅ Valid |

**Conclusion:** Server verification confirmed the issue that browser couldn't detect. The anthropics link was a false positive in browser analysis—now properly flagged for verification.

---

## Comparison: v1 vs v2

| Feature | v1 | v2 |
|---------|----|----|
| Internal links | ✅ Accurate | ✅ Accurate |
| External links (no CORS) | ✅ Accurate | ✅ Accurate |
| CORS-blocked external links | ❌ False positives | 🚩 Flagged for verification |
| Server-side fallback | ❌ None | ✅ `link_validation` tool |
| CORS detection | ❌ No | ✅ Explicit status |
| Reliability | ⚠️ Medium (false negatives) | ✅ High (clear separation) |

---

## Summary

**The Problem:** CORS prevents browsers from accurately checking external links.

### The Solution:
- Browser detects and flags CORS-blocked links (instead of guessing)
- Server provides optional verification for flagged links
- Users get accurate results with full transparency

### The Value:
- Honest error reporting (CORS-blocked ≠ broken)
- Optional server verification for complete picture
- Privacy-preserving (no data transmission for browser checks)
