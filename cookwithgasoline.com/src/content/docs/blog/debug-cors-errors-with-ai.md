---
title: "How to Debug CORS Errors with AI Using Gasoline MCP"
date: 2026-02-07
authors: [brenn]
tags: [debugging, cors, ai-development, how-to]
---

CORS errors are the most misleading errors in web development. The browser tells you "access has been blocked" — but the actual problem could be a missing header, a wrong origin, a preflight failure, a credentials mismatch, or a server that's simply crashing and returning a 500 without CORS headers.

Here's how to use Gasoline MCP to let your AI assistant see the full picture and fix CORS issues in minutes instead of hours.

<!-- more -->

## Why CORS Errors Are Hard to Debug

The browser console shows you something like:

```
Access to fetch at 'https://api.example.com/users' from origin 'http://localhost:3000'
has been blocked by CORS policy: No 'Access-Control-Allow-Origin' header is present
on the requested resource.
```

This tells you *what* happened but not *why*. Common causes:

1. **The server doesn't send CORS headers at all** — needs configuration
2. **The server sends the wrong origin** — `*` doesn't work with credentials
3. **The preflight OPTIONS request fails** — the server doesn't handle OPTIONS
4. **The server errors out** — a 500 response won't have CORS headers either
5. **A proxy strips headers** — nginx, Cloudflare, or your reverse proxy eats the headers
6. **Credentials mode mismatch** — `withCredentials: true` requires explicit origin, not `*`

Chrome DevTools shows the failed request in the Network tab, but the response body is hidden for CORS-blocked requests. You can't see what the server actually returned. You're debugging blind.

## The Gasoline Approach

With Gasoline connected, your AI can see the error, the network request details, and the response headers — everything needed to diagnose the root cause.

### Step 1: See the Error

```js
observe({what: "errors"})
```

The AI sees the CORS error message with the exact URL, origin, and which header is missing.

### Step 2: Inspect the Network Request

```js
observe({what: "network_bodies", url: "/api/users"})
```

This shows the full request/response pair:
- **Request headers** — the `Origin` header the browser sent
- **Response headers** — whether `Access-Control-Allow-Origin` is present, and what value it has
- **Response status** — is it a 200 with missing headers, or a 500 that *also* lacks headers?
- **Response body** — the actual error payload (which Chrome hides for CORS failures)

### Step 3: Check for Preflight Issues

```js
observe({what: "network_waterfall", url: "/api/users"})
```

The waterfall shows if there are *two* requests — the preflight OPTIONS and the actual request. If the OPTIONS request fails or returns the wrong status, the browser never sends the real request.

### Step 4: Look at the Timeline

```js
observe({what: "timeline", include: ["network", "errors"]})
```

The timeline shows the sequence: did the preflight succeed? Did the main request fire? When did the error appear relative to the request? This catches timing-related CORS issues like the server sending headers on GET but not POST.

## Common CORS Scenarios and Fixes

### Missing CORS Headers Entirely

**What the AI sees**: Request to `api.example.com`, response status 200, no `Access-Control-Allow-Origin` header.

**The fix**: Add CORS headers to the server. The AI can look at your server code and add the appropriate middleware:

```js
// Express
app.use(cors({ origin: 'http://localhost:3000' }));

// Go
w.Header().Set("Access-Control-Allow-Origin", "http://localhost:3000")

// Nginx
add_header 'Access-Control-Allow-Origin' 'http://localhost:3000';
```

### Server Returning 500

**What the AI sees**: Request to `/api/users`, response status 500, body contains `{"error": "database connection failed"}`, no CORS headers.

**The real problem**: The server is crashing, and crash responses don't go through the CORS middleware. The CORS error is a red herring.

This is why seeing the response body matters. Without Gasoline, you'd spend an hour debugging CORS headers when the actual issue is a database connection string.

### Preflight OPTIONS Failure

**What the AI sees**: Two requests in the waterfall — an OPTIONS request returning 404, and no follow-up request.

**The fix**: The server doesn't handle OPTIONS requests for that route. Add an OPTIONS handler or configure your framework's CORS middleware to handle preflight requests.

### Wildcard With Credentials

**What the AI sees**: Response has `Access-Control-Allow-Origin: *`, request has `credentials: include`. Error says "wildcard cannot be used with credentials."

**The fix**: Replace `*` with the specific origin. The AI can read the `Origin` header from the request and configure the server to echo it back (with a whitelist).

### Proxy Stripping Headers

**What the AI sees**: Server code sends CORS headers (the AI can read the source), but the response in the browser doesn't have them.

**Diagnosis**: Something between the server and browser is stripping headers. The AI checks nginx configs, Cloudflare settings, or reverse proxy configuration.

## The Full Workflow

Here's what it looks like end-to-end:

> **You**: "I'm getting a CORS error when calling the API."

The AI:
1. Calls `observe({what: "errors"})` — sees the CORS error with URL and origin
2. Calls `observe({what: "network_bodies", url: "/api"})` — sees the actual response (a 500 with a database error)
3. Reads the server code — finds the missing error handler that skips CORS middleware
4. Fixes the error handler to pass through CORS middleware even on errors
5. Calls `interact({action: "refresh"})` — reloads the page
6. Calls `observe({what: "errors"})` — confirms the CORS error is gone

Total time: 2 minutes. No manual DevTools inspection. No guessing about headers. No Stack Overflow rabbit holes.

## Why This Is Better Than DevTools

Chrome DevTools has a fundamental limitation for CORS debugging: it **hides the response body** for CORS-blocked requests. The Network tab shows the request was blocked, but you can't see what the server actually returned.

This means you can't tell the difference between:
- A correctly configured server that's missing one header
- A server that's completely crashing and returning a 500

Gasoline captures the response at the network level before CORS enforcement, so the AI sees everything — headers, body, status code. The diagnosis goes from "something is wrong with CORS" to "the server returned a 500 because the database is down, and the error handler doesn't set CORS headers."

## Tips

**Check the timeline, not just the error**. CORS errors sometimes cascade — one failed preflight blocks ten subsequent requests. The timeline shows the cascade pattern so you fix the root cause, not the symptoms.

**Look at both staging and production headers**. CORS works in staging with `*` but breaks in production with credentials? The network bodies show exactly which headers each environment returns.

**Watch for mixed HTTP/HTTPS**. `http://localhost:3000` and `https://localhost:3000` are different origins. The AI's transport security check (`observe({what: "security_audit", checks: ["transport"]})`) catches this mismatch.

**Use error_bundles for context**. `observe({what: "error_bundles"})` returns the CORS error along with the correlated network request and recent actions — everything in one call instead of three.
