---
title: "Subresource Integrity (SRI) Explained: Protect Your Site from CDN Compromise"
date: 2026-02-07
authors: [brenn]
tags: [security, sri, web-development]
---

Every `<script src="https://cdn.example.com/library.js">` on your page is a trust decision. You're trusting that the CDN will always serve the exact file you expect. If the CDN is compromised, hacked, or serves a corrupted file, your users execute the attacker's code.

Subresource Integrity (SRI) eliminates this risk. Here's how it works and how to implement it.

<!-- more -->

## The Risk

In 2018, a cryptocurrency mining script was injected into the British Airways website through a compromised third-party script. In 2019, Magecart attacks hit thousands of e-commerce sites through CDN compromises. In 2021, the ua-parser-js npm package was hijacked to serve malware.

The attack pattern is always the same:
1. Attacker compromises a CDN, package registry, or hosting provider
2. The script content changes (malware added, data exfiltration code injected)
3. Every website loading that script from that CDN now serves the attacker's code
4. Users' data is stolen, credentials harvested, or cryptocurrency mined

SRI prevents step 3. Even if the CDN is compromised, the browser refuses to execute the modified script.

## How SRI Works

SRI adds a cryptographic hash to your `<script>` and `<link>` tags:

```html
<script src="https://cdn.jsdelivr.net/npm/lodash@4.17.21/lodash.min.js"
        integrity="sha384-OYp56H6p7T3JKjPdRfL7gAHEdJ7yrfCCe3Ew5EHouvE7qdLCHs6PGoGMOQ/pA6j"
        crossorigin="anonymous"></script>
```

When the browser downloads the file, it computes the SHA-384 hash of the content and compares it to the `integrity` attribute. If they don't match — because the file was modified, corrupted, or replaced — the browser refuses to execute it.

### The `integrity` Attribute

Format: `algorithm-base64hash`

```
integrity="sha384-OYp56H6p7T3JKjPdRfL7gAHEdJ7yrfCCe3Ew5EHouvE7qdLCHs6PGoGMOQ/pA6j"
```

Supported algorithms:
- **SHA-256** — `sha256-...`
- **SHA-384** — `sha384-...` (recommended — good balance of security and performance)
- **SHA-512** — `sha512-...`

You can include multiple hashes for fallback:

```
integrity="sha384-abc... sha256-xyz..."
```

The browser accepts the resource if *any* hash matches.

### The `crossorigin` Attribute

SRI requires CORS. Add `crossorigin="anonymous"` to cross-origin scripts:

```html
<script src="https://cdn.example.com/lib.js"
        integrity="sha384-..."
        crossorigin="anonymous"></script>
```

Without `crossorigin`, the browser can't compute the hash for cross-origin resources and SRI silently fails.

## What SRI Protects Against

| Threat | SRI Protection |
|--------|---------------|
| CDN compromise (attacker modifies hosted files) | Blocks modified scripts |
| Man-in-the-middle attacks (on HTTP resources) | Blocks tampered resources |
| CDN serving wrong version | Blocks unexpected content |
| Package registry hijacking (modified npm package) | Blocks modified bundles |
| DNS hijacking (CDN domain points to attacker) | Blocks attacker's response |

## What SRI Doesn't Protect Against

| Threat | Why SRI Doesn't Help |
|--------|---------------------|
| First-party script compromise | SRI is for *third-party* resources |
| XSS via inline scripts | Use CSP for inline script protection |
| Version pinning | SRI verifies content, not version semantics |
| Availability | If the CDN is down, SRI can't make it work |

## Generating SRI Hashes

### Manual (CLI)

```bash
# Generate a hash for a local file
cat library.js | openssl dgst -sha384 -binary | openssl base64 -A

# Generate a hash for a remote file
curl -s https://cdn.example.com/library.js | openssl dgst -sha384 -binary | openssl base64 -A
```

### With Gasoline

Gasoline observes which third-party scripts and stylesheets your page loads and generates SRI hashes automatically:

```js
generate({format: "sri"})
```

Output per resource:
- **URL** — the resource location
- **Hash** — `sha384-...` in browser-standard format
- **Ready-to-use HTML tag** — `<script>` or `<link>` with `integrity` and `crossorigin` attributes
- **File size** — for reference
- **Already protected** — flags resources that already have SRI

Filter to specific resource types or origins:

```js
generate({format: "sri", resource_types: ["script"]})
generate({format: "sri", origins: ["https://cdn.jsdelivr.net"]})
```

The output is copy-paste ready:

```html
<script src="https://cdn.jsdelivr.net/npm/lodash@4.17.21/lodash.min.js"
        integrity="sha384-OYp56H6p7T3JKjPdRfL7gAHEdJ7yrfCCe3Ew5EHouvE7qdLCHs6PGoGMOQ/pA6j"
        crossorigin="anonymous"></script>
```

## Implementation

### Static HTML

Replace your existing tags with integrity-protected versions:

```html
<!-- Before -->
<script src="https://cdn.example.com/chart.js"></script>

<!-- After -->
<script src="https://cdn.example.com/chart.js"
        integrity="sha384-abc123..."
        crossorigin="anonymous"></script>
```

### Webpack

Use the `webpack-subresource-integrity` plugin:

```javascript
const { SubresourceIntegrityPlugin } = require('webpack-subresource-integrity');

module.exports = {
  output: { crossOriginLoading: 'anonymous' },
  plugins: [new SubresourceIntegrityPlugin()]
};
```

### Vite

Vite doesn't generate SRI natively. Use a plugin or add integrity attributes to your HTML template for third-party CDN scripts.

### Next.js

For scripts loaded via `<Script>` component, add the `integrity` prop:

```jsx
<Script
  src="https://cdn.example.com/analytics.js"
  integrity="sha384-abc123..."
  crossOrigin="anonymous"
/>
```

## Common Issues

### Hash Mismatch After CDN Update

If a CDN updates the file content (even whitespace changes), the hash won't match and the browser blocks the script.

**Fix**: Pin to specific versions (`lodash@4.17.21` not `lodash@latest`) and update hashes when you upgrade versions.

### Vary: User-Agent

Some CDNs serve different content based on the User-Agent header (e.g., minified vs unminified). This means the hash differs across browsers.

**Fix**: Use CDNs that serve consistent content regardless of User-Agent. Gasoline warns you when it detects `Vary: User-Agent` on a resource.

### Dynamic Script Loading

If your framework dynamically creates `<script>` elements, you'll need to add integrity attributes programmatically:

```javascript
const script = document.createElement('script');
script.src = 'https://cdn.example.com/lib.js';
script.integrity = 'sha384-abc123...';
script.crossOrigin = 'anonymous';
document.head.appendChild(script);
```

### Service Workers

Service workers can modify responses, which breaks SRI. If you're using a service worker that caches CDN resources, ensure it passes through the original response without modification.

## SRI + CSP: Defense in Depth

SRI and CSP work together:

- **CSP** restricts *which origins* can load resources
- **SRI** verifies *the content* of resources from allowed origins

CSP alone: an attacker compromises an allowed CDN → your CSP still allows the compromised script.

SRI alone: an attacker injects a `<script>` from a new origin → SRI doesn't help because the new tag doesn't have an integrity attribute.

Both together: CSP blocks scripts from unauthorized origins, and SRI blocks modified scripts from authorized origins. The attacker needs to compromise your specific CDN *and* produce content that matches the hash — which is cryptographically impossible.

## Workflow

1. **Browse your app** with Gasoline connected
2. **Generate SRI hashes**: `generate({format: "sri"})`
3. **Add integrity attributes** to your third-party script and link tags
4. **Test** — verify all resources load correctly
5. **Regenerate** when you update third-party library versions

## Should You Use SRI?

**Yes, if** you load scripts or stylesheets from third-party CDNs. The implementation cost is minimal (add two attributes to each tag) and the protection against supply chain attacks is significant.

**Skip it for** first-party resources served from your own domain. SRI adds value when you don't control the server — if you control both the page and the resource server, CSP provides sufficient protection.

The combination of SRI for third-party resources and CSP for all resources gives you defense in depth against the most common web supply chain attacks.
