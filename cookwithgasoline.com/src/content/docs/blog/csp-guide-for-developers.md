---
title: "Content Security Policy (CSP) Guide for Web Developers"
date: 2026-02-07
authors: [brenn]
tags: [security, csp, web-development]
---

Content Security Policy is one of the most effective defenses against XSS attacks, but it's also one of the most confusing security headers to configure. Get it wrong and your site breaks. Get it right and an entire class of attacks becomes impossible.

Here's a practical guide to CSP — what it does, how to build one, and how to use Gasoline to generate a policy from your actual traffic.

<!-- more -->

## What CSP Does

CSP tells the browser which sources are allowed to load resources on your page. If a script tries to load from an origin not in your policy, the browser blocks it.

Without CSP, an XSS vulnerability means an attacker can:
- Load scripts from any domain (`<script src="https://evil.com/steal.js">`)
- Execute inline JavaScript (`<script>document.cookie</script>`)
- Inject styles that hide or modify content

With CSP, even if an attacker injects HTML, the browser refuses to execute scripts or load resources from unauthorized origins.

## The Header

CSP is delivered as an HTTP response header:

```
Content-Security-Policy: default-src 'self'; script-src 'self' https://cdn.example.com; style-src 'self' 'unsafe-inline'
```

Or as a `<meta>` tag (with some limitations):

```html
<meta http-equiv="Content-Security-Policy" content="default-src 'self'; script-src 'self' https://cdn.example.com">
```

## Directives

Each directive controls a resource type:

| Directive | Controls | Examples |
|-----------|----------|---------|
| `default-src` | Fallback for all resource types | `'self'` |
| `script-src` | JavaScript | `'self' https://cdn.jsdelivr.net` |
| `style-src` | CSS | `'self' 'unsafe-inline'` |
| `img-src` | Images | `'self' https: data:` |
| `font-src` | Web fonts | `'self' https://fonts.gstatic.com` |
| `connect-src` | XHR, fetch, WebSocket | `'self' https://api.example.com wss://ws.example.com` |
| `frame-src` | Iframes | `'self' https://www.youtube.com` |
| `media-src` | Audio and video | `'self'` |
| `worker-src` | Web Workers, Service Workers | `'self'` |
| `object-src` | Plugins (Flash, Java) | `'none'` |
| `base-uri` | `<base>` element | `'self'` |
| `form-action` | Form submission targets | `'self'` |

If a specific directive isn't set, `default-src` is used as the fallback.

## Source Values

| Value | Meaning |
|-------|---------|
| `'self'` | Same origin as the page |
| `'none'` | Block everything |
| `'unsafe-inline'` | Allow inline scripts/styles (weakens XSS protection) |
| `'unsafe-eval'` | Allow `eval()`, `new Function()` (weakens XSS protection) |
| `https:` | Any HTTPS origin |
| `data:` | Data URIs (`data:image/png;base64,...`) |
| `https://cdn.example.com` | Specific origin |
| `*.example.com` | Wildcard subdomain |
| `'nonce-abc123'` | Scripts/styles with matching `nonce` attribute |
| `'sha256-...'` | Scripts/styles with matching hash |

## Building a CSP Step by Step

### Start Restrictive

```
Content-Security-Policy: default-src 'none'; script-src 'self'; style-src 'self'; img-src 'self'; font-src 'self'; connect-src 'self'
```

This allows only same-origin resources. Everything else is blocked.

### Add What You Need

Using a CDN for scripts? Add it:

```
script-src 'self' https://cdn.jsdelivr.net
```

Using Google Fonts? Add both origins:

```
style-src 'self' https://fonts.googleapis.com
font-src 'self' https://fonts.gstatic.com
```

API on a different domain? Add it to `connect-src`:

```
connect-src 'self' https://api.myapp.com
```

### Report-Only Mode

Not sure your policy is correct? Use `Content-Security-Policy-Report-Only` instead:

```
Content-Security-Policy-Report-Only: default-src 'self'; script-src 'self'; report-uri /csp-reports
```

The browser logs violations but doesn't block anything. Review the reports, adjust the policy, then switch to enforcement.

## The Hard Part: Knowing What to Allow

The biggest challenge with CSP is knowing which origins your page actually loads resources from. A modern web application might use:

- Your own CDN for static assets
- Google Fonts for typography
- A JavaScript CDN (jsdelivr, unpkg, cdnjs)
- An analytics service (Google Analytics, Segment)
- A payment processor (Stripe)
- An error tracker (Sentry)
- Social media embeds
- Ad networks

You could audit every `<script>`, `<link>`, `<img>`, and `fetch()` call in your codebase. Or you could let Gasoline do it.

## Generate CSP with Gasoline

Gasoline observes all network traffic during your browsing session and generates a CSP from what it sees:

```js
generate({format: "csp"})
```

### Three Modes

**Strict** — only high-confidence origins (observed 3+ times from 2+ pages):

```js
generate({format: "csp", mode: "strict"})
```

This gives you the tightest possible policy. If a script was only loaded once, it might be ad injection or a browser extension — strict mode excludes it.

**Moderate** — includes medium-confidence origins:

```js
generate({format: "csp", mode: "moderate"})
```

Good for most production use cases.

**Report-Only** — generates a `Content-Security-Policy-Report-Only` header:

```js
generate({format: "csp", mode: "report_only"})
```

Deploy this first to find violations before enforcing.

### Smart Filtering

Gasoline automatically excludes:
- **Browser extension origins** (`chrome-extension://`, `moz-extension://`) — these shouldn't be in your CSP
- **Development server origins** — localhost on a different port than your app
- **Low-confidence origins** — observed only once on one page (likely noise)

### Exclude Specific Origins

Don't want analytics in your CSP? Exclude it:

```js
generate({format: "csp", mode: "strict",
          exclude_origins: ["https://analytics.google.com", "https://www.googletagmanager.com"]})
```

### What You Get

The output includes:
- **Ready-to-use header string** — copy-paste into your server config
- **Meta tag equivalent** — for static sites
- **Per-origin details** — which directive each origin maps to, confidence level, observation count
- **Filtered origins** — what was excluded and why
- **Warnings** — e.g., "only 3 pages observed — visit more pages for broader coverage"

## Common CSP Mistakes

### Using `'unsafe-inline'` Everywhere

```
script-src 'self' 'unsafe-inline'
```

This defeats the purpose of CSP for scripts. Inline scripts are the primary XSS vector. Use nonces or hashes instead:

```html
<!-- Server generates a unique nonce per request -->
<script nonce="abc123">
  // This script is allowed
</script>
```

```
script-src 'self' 'nonce-abc123'
```

### Forgetting `connect-src`

Your page loads fine, but all API calls fail. `connect-src` controls fetch/XHR destinations — if your API is on a different origin, you need to allow it.

### Wildcard Overuse

```
script-src 'self' https:
```

This allows scripts from *any* HTTPS origin. An attacker can host a script on any HTTPS server and your CSP won't block it. Be specific about which origins you allow.

### Not Testing in Report-Only Mode

Deploying a new CSP without testing breaks things. Always start with `Content-Security-Policy-Report-Only`, check for violations, then switch to enforcement.

### Missing `object-src 'none'`

Even in 2026, you should explicitly block plugins:

```
object-src 'none'
```

This prevents Flash and Java plugin exploitation (still a vector in some corporate environments).

## CSP for Frameworks

### Next.js

Next.js uses inline scripts for hydration. You'll need nonce-based CSP:

```javascript
// middleware.ts
export function middleware(request) {
  const nonce = crypto.randomUUID();
  const csp = `script-src 'self' 'nonce-${nonce}'; style-src 'self' 'unsafe-inline';`;
  // Pass nonce to components via headers
}
```

### React (Create React App)

CRA inlines a runtime chunk. Either:
- Disable inline runtime: `INLINE_RUNTIME_CHUNK=false`
- Use hash-based CSP for the known inline script

### Vite

Vite's dev server uses inline scripts and HMR WebSocket. Dev CSP will differ from production.

## The Workflow

1. **Browse your app** through its main flows with Gasoline connected
2. **Generate a CSP**: `generate({format: "csp", mode: "report_only"})`
3. **Deploy in report-only mode** and monitor for violations
4. **Adjust** — add any legitimate origins that were missed
5. **Switch to enforcement** once violations are resolved
6. **Regenerate periodically** as your dependencies change

Gasoline takes the guesswork out of step 1 — you don't have to audit your codebase manually. It sees every origin your page communicates with and builds the policy from observation.
