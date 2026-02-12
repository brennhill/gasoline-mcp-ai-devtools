# ShopBroken — Demo Site Specification

> "Looks good. Works terribly."

A deliberately buggy e-commerce site designed to showcase every Gasoline capability.

## Tech Stack

- **Frontend:** React (Vite) — widely recognized, easy to show code fixes
- **Backend:** Express.js — simple API server
- **WebSocket:** ws library — live inventory updates (intentionally broken)
- **No database** — in-memory data, keeps the demo self-contained

## Site Architecture

```
ShopBroken
├── / (Home)           — Performance issues, third-party scripts
├── /products          — Accessibility violations, broken images
├── /product/:id       — WebSocket live inventory (broken), memory leak
├── /cart              — Console errors on quantity update
├── /checkout          — 500 error on submit, missing validation
├── /admin             — Missing auth, insecure cookies, XSS vulnerability
└── /api/*             — Inconsistent error codes, missing CORS headers
```

## Intentional Bugs by Category

### Errors (Demo 1: Bug Detective)

| Bug | Location | Gasoline Detection |
|-----|----------|--------------------|
| Uncaught TypeError when cart quantity = 0 | `/cart` — quantity input onChange | `observe(error_bundles)` |
| Unhandled promise rejection on failed fetch | `/products` — API call with no catch | `observe(errors)` |
| 500 on checkout submit (string vs number type mismatch) | `/checkout` POST handler | `observe(error_bundles)` + `observe(network_bodies)` |
| ReferenceError: undefined variable in admin panel | `/admin` — missing import | `observe(errors)` |

### Accessibility (Demo 2: Accessibility Blitz)

| Bug | Location | WCAG Rule |
|-----|----------|-----------|
| Missing alt text on all product images | `/products`, `/product/:id` | image-alt |
| Form inputs without associated labels | `/checkout` form | label |
| Low contrast text on sale badges (white on yellow) | `/products` — `.sale-badge` | color-contrast |
| No keyboard navigation on product cards | `/products` — cards are divs, not buttons/links | keyboard |
| Missing skip-to-content link | All pages — layout | bypass |
| No focus indicators on interactive elements | Global CSS — `outline: none` | focus-visible |
| Missing landmark roles | Layout — no `<main>`, `<nav>` semantics | region |
| Empty link text (icon-only buttons) | Header — cart icon, menu icon | link-name |

### Security (Demo 3: Security Lockdown)

| Bug | Location | Check |
|-----|----------|-------|
| No Content-Security-Policy header | Server response headers | headers |
| No HSTS header | Server response headers | headers |
| Cookies without Secure flag | `/admin` — session cookie | cookies |
| Cookies without HttpOnly flag | `/admin` — session cookie | cookies |
| Cookies without SameSite attribute | `/admin` — session cookie | cookies |
| XSS via query parameter reflection | `/admin?name=<script>` | credentials/pii |
| 4 third-party tracking scripts | Home page — analytics, chat, ads, fonts | third_party_audit |
| External scripts without SRI hashes | `<script>` tags in index.html | `generate(sri)` |

### Performance (Demo 4: Performance Rescue)

| Bug | Location | Metric Affected |
|-----|----------|-----------------|
| 3MB uncompressed hero image | `/` — hero section | LCP > 4s |
| No lazy loading on images | `/products` — all images eager | LCP |
| Render-blocking script in `<head>` | `index.html` — analytics.js | FCP |
| Layout shift from late-loading web font | Global — font-display not set | CLS > 0.25 |
| Unminified CSS (50KB) | `styles.css` — comments, whitespace | FCP |
| No image dimensions (width/height) | `/products` — img tags | CLS |

### Network (Demo 5: Test Suite from Zero)

| Bug | Location | Observation |
|-----|----------|-------------|
| GET /api/products — 2s artificial delay | Server handler | `observe(network_waterfall)` timing |
| POST /api/checkout — returns 500 for valid data | Server validation bug | `observe(network_bodies)` |
| GET /api/product/999 — returns 200 with empty body instead of 404 | Missing product handler | `observe(network_waterfall)` |
| WebSocket sends malformed JSON every 5s | `/product/:id` — inventory updates | `observe(websocket_events)` |
| Missing CORS headers on /api/* | Server — no cors middleware | `observe(network_waterfall)` |

### Links (Demo 6: Ship Day)

| Bug | Location | Type |
|-----|----------|------|
| Broken link to /about (page doesn't exist) | Footer | 404 |
| Broken link to /privacy (page doesn't exist) | Footer | 404 |
| Redirect loop /old-products → /products → /old-products | Legacy route | Redirect loop |

---

## Design Principles

1. **Site must look polished** — The bugs are invisible to the eye but visible to Gasoline. Professional design with real product images, proper layout, consistent branding.

2. **Each bug fixable in 1-3 lines** — Keeps demos fast. No complex refactoring needed.

3. **Bugs are realistic** — Every bug mirrors real-world issues developers face daily. Nothing contrived.

4. **Self-contained** — No external database, no env vars, no setup. `npm install && npm start`.

5. **Subtitle-ready** — All demos use `interact(subtitle)` for closed-caption narration during recording.

---

## Pages Detail

### Home (`/`)
- Hero section with oversized image (3MB, no optimization)
- Featured products grid (6 items)
- Third-party script tags (fake analytics, chat widget, ad tracker, font loader)
- Render-blocking script in `<head>`
- Newsletter signup form (no labels, no aria)

### Products (`/products`)
- Product grid with 12 items
- Images without alt text
- Sale badges with low contrast
- Product cards as `<div>` (not keyboard accessible)
- Missing product triggers 200 with empty body instead of 404
- 2-second API delay on load

### Product Detail (`/product/:id`)
- Large product image gallery
- WebSocket connection for "live inventory" (sends malformed JSON)
- Event listener leak on gallery (doesn't cleanup on unmount)
- Add to cart button

### Cart (`/cart`)
- Cart items list
- Quantity input that throws TypeError when set to 0
- Total calculation
- "Proceed to Checkout" link

### Checkout (`/checkout`)
- Form: name, email, address, card number
- No form labels (a11y violation)
- Submit sends quantity as string (API expects number → 500)
- No client-side validation
- Unhandled promise rejection on fetch failure

### Admin (`/admin`)
- No authentication check
- Session cookie without Secure/HttpOnly/SameSite
- Query parameter reflected in page without sanitization (XSS)
- Missing import causes ReferenceError

---

## API Endpoints

| Method | Path | Behavior | Bug |
|--------|------|----------|-----|
| GET | /api/products | Returns product list | 2s artificial delay |
| GET | /api/product/:id | Returns single product | Returns 200 + empty body for missing IDs |
| POST | /api/checkout | Process order | 500 when quantity is string |
| GET | /api/cart | Get cart contents | Works correctly |
| POST | /api/cart | Add to cart | Works correctly |
| PUT | /api/cart/:id | Update quantity | Works correctly |
| WS | /ws/inventory | Live inventory stream | Sends malformed JSON |

---

## Third-Party Scripts (Fake)

Loaded in `index.html` to trigger third_party_audit findings:

```html
<!-- These are fake tracking pixels / scripts served locally but with external-looking origins -->
<script src="/fake-third-party/analytics.js"></script>
<script src="/fake-third-party/chat-widget.js"></script>
<script src="/fake-third-party/ad-tracker.js"></script>
<link href="/fake-third-party/fonts.css" rel="stylesheet">
```

Each script does minimal work (sets a cookie, adds a tracking pixel, logs to console) but triggers security/third-party audit findings.

---

## Running the Demo

```bash
cd ~/dev/gasoline-demos/new/shopbroken
npm install
npm start
# → http://localhost:3456
```

Port 3456 chosen to avoid conflicts with Gasoline (7890) and common dev ports (3000, 5173, 8080).
