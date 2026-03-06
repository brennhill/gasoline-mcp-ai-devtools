---
title: "How to Debug React and Next.js Apps with AI Using Gasoline MCP"
date: 2026-02-07
authors: [brenn]
tags: [debugging, react, nextjs, ai-development, how-to]
---

React and Next.js applications have a unique set of debugging challenges — hydration mismatches, stale closures, useEffect dependency bugs, SSR/client divergence, and API route failures. Your AI coding assistant can fix all of these faster if it can actually *see* your browser.

Here's how Gasoline MCP gives your AI the runtime context it needs to debug React and Next.js apps effectively.

<!-- more -->

## What Makes React/Next.js Debugging Different

React errors are notoriously unhelpful:

```text
Uncaught Error: Minified React error #418
```

Even in development mode, React errors like "Cannot update a component while rendering a different component" don't tell you *which* component or *what triggered* the update. And Next.js adds its own layer of complexity:

- **Hydration mismatches** — server HTML differs from client render
- **SSR errors** — server-side code fails but the page looks fine on the client
- **API route failures** — `/api/*` routes return 500s that the client silently swallows
- **Middleware issues** — redirects and rewrites that happen before the page loads
- **Client/server boundary confusion** — `"use client"` and `"use server"` scope mistakes

Your AI assistant can read your source code, but without browser data it can't see what's actually happening at runtime.

## The Gasoline Debugging Workflow

### Step 1: See Runtime Errors

```js
observe({what: "errors"})
```

Your AI sees every console error with the full message, stack trace, and source file location. For minified builds, Gasoline resolves source maps — so even in production, the AI sees the original component name and line number.

### Step 2: Correlate with Network Data

Most React bugs involve data:

```js
observe({what: "error_bundles"})
```

Error bundles return each error with its correlated context — the network requests that happened around the same time, the user actions that preceded it, and relevant console logs. One call gives the AI the complete picture:

- The error: `TypeError: Cannot read properties of undefined (reading 'map')`
- The API call: `GET /api/products → 200`, but the response body was `{ products: null }` instead of `{ products: [] }`
- The user action: Clicked "Load More" button

The AI immediately knows: the API returned `null` where the component expected an array.

### Step 3: Check the Timeline

For race conditions and ordering issues:

```js
observe({what: "timeline"})
```

The timeline shows actions, network requests, and errors in chronological order. This reveals:
- Components that fetch data before mounting
- Effects that fire in unexpected order
- Network requests that resolve after the component unmounts

## Common React/Next.js Issues

### Hydration Mismatches

**Symptom**: "Text content does not match server-rendered HTML" or "Hydration failed because the initial UI does not match."

```js
observe({what: "errors"})
```

The AI sees the hydration warning with the mismatched content. Common causes:
- Using `Date.now()` or `Math.random()` during render (different on server vs client)
- Checking `window` or `localStorage` during initial render
- Conditional rendering based on `typeof window !== 'undefined'`

The AI can find the component, identify the non-deterministic code, and move it into a `useEffect` or behind a `suppressHydrationWarning`.

### API Route 500 Errors

**Symptom**: A feature silently fails. No error in the UI, but the data is wrong.

```js
observe({what: "network_bodies", url: "/api"})
```

The AI sees every API route call with the full request and response body. A 500 response from `/api/checkout` with `{"error": "STRIPE_KEY is undefined"}` tells the AI exactly what's wrong — an environment variable isn't set.

### useEffect Dependency Bugs

**Symptom**: The component re-renders endlessly, or an effect doesn't fire when it should.

```js
observe({what: "network_waterfall", url: "/api"})
```

If an effect with a missing dependency is refetching on every render, the waterfall shows dozens of identical API calls in rapid succession. The AI sees the pattern and checks the effect's dependency array.

### State Update on Unmounted Component

**Symptom**: "Can't perform a React state update on an unmounted component."

```js
observe({what: "timeline", include: ["actions", "errors", "network"]})
```

The timeline shows: user navigates away → API call from the previous page resolves → state update on the now-unmounted component. The AI adds cleanup logic to the effect.

### Slow Client-Side Navigation

**Symptom**: Page transitions feel sluggish.

```js
observe({what: "vitals"})
observe({what: "performance"})
```

The AI checks INP (responsiveness) and long tasks. If client-side navigation triggers heavy re-renders, the performance snapshot shows the blocking time. The AI can suggest `React.memo`, `useMemo`, code splitting, or moving work to a Web Worker.

## Next.js-Specific Debugging

### Server Component Errors

Server components run on the server and stream HTML to the client. Errors in server components don't always appear in the browser console.

```js
observe({what: "network_bodies", url: "/"})
```

The response body for a Next.js page includes the serialized server component tree. If a server component throws, the error boundary HTML is visible in the response.

### Middleware Debugging

Next.js middleware runs before the page loads. If a redirect or rewrite misbehaves:

```js
observe({what: "network_waterfall"})
```

The waterfall shows every request including redirects (301, 307, 308). The AI can see if middleware is redirecting to the wrong URL or creating redirect loops.

### Image Optimization Issues

Next.js `<Image>` component can cause CLS if dimensions aren't right:

```js
observe({what: "vitals"})  // Check CLS
configure({action: "query_dom", selector: "img"})  // Check image dimensions
```

### Build Size Regression

After adding a new dependency:

```js
observe({what: "network_waterfall"})
observe({what: "performance"})
```

The network summary shows total JavaScript transfer size. If it jumped from 300KB to 800KB, the waterfall identifies which new bundles appeared.

## Full Debug Session Example

> **You**: "The product page is broken — it shows a blank screen after I click 'Add to Cart'."

The AI:

1. Calls `observe({what: "error_bundles"})` — sees a `TypeError: Cannot read properties of undefined (reading 'quantity')` correlated with `POST /api/cart → 201` that returned `{item: {id: 5}}` (no `quantity` field)

2. Reads the cart component — finds `cartItem.quantity.toString()` without null checking

3. Checks the API route — finds the response omits `quantity` for new items (it defaults to 1 on the backend but isn't serialized)

4. Fixes both: adds `quantity` to the API response and adds a fallback in the component

5. Calls `interact({action: "refresh"})` then `observe({what: "errors"})` — confirms zero errors

Total time: 3 minutes. No manual DevTools inspection. No reproducing the bug by clicking through the UI.

## Tips

**Use `error_bundles` as your first call.** It returns errors with their network and action context in one shot — faster than calling `errors`, then `network_bodies`, then `actions` separately.

**Check the waterfall after deploys.** New React bundles, changed chunk names, and different loading order are all visible in the network waterfall. The AI spots unexpected changes immediately.

**Profile page transitions.** Use `interact({action: "navigate", url: "/products"})` to trigger a client-side navigation. The perf_diff shows the performance impact of that navigation including any heavy re-renders.

**For SSR issues, check response bodies.** The HTML response for a Next.js page contains the server-rendered markup. If something is wrong on the server side, it's visible in the network body before hydration even starts.
