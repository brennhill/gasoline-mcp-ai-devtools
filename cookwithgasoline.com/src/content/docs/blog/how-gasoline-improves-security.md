---
title: "How Gasoline MCP Improves Your Application Security"
date: 2026-02-07
authors: [brenn]
tags: [security, mcp, ai-development]
---

Most developers discover security issues in production. A penetration test finds exposed credentials in an API response. A security review flags missing headers. A breach notification reveals that a third-party script was exfiltrating form data.

Gasoline MCP flips the timeline. Your AI assistant audits security **while you develop**, catching issues before they ship.

<!-- more -->

## The Problem: Security Is an Afterthought

In the typical development cycle, security checks happen late:

1. **Development** — features built, tested, deployed
2. **Security review** — weeks later, if at all
3. **Penetration test** — quarterly, expensive, findings arrive after context is lost
4. **Incident** — the worst time to learn about a vulnerability

Every step between writing the code and finding the issue adds cost. A missing `HttpOnly` flag caught during development takes 30 seconds to fix. The same flag caught in a pen test takes a meeting, a ticket, a sprint, and a deploy.

## Real-Time Security Auditing During Development

Gasoline gives your AI assistant six categories of security checks that run against live browser traffic:

### Credential Detection

Your AI can scan every network request and response for exposed secrets:

```js
observe({what: "security_audit", checks: ["credentials"]})
```

This catches:
- **AWS Access Keys** (`AKIA...`) in API responses
- **GitHub PATs** (`ghp_...`, `ghs_...`) in console logs
- **Stripe keys** (`sk_test_...`, `sk_live_...`) in client-side code
- **JWTs** in URL parameters (a common mistake)
- **Bearer tokens** in responses that shouldn't contain them
- **Private keys** accidentally bundled in source maps

Every detection runs regex plus validation (Luhn algorithm for credit cards, structure checks for JWTs) to minimize false positives.

### PII Exposure Detection

```js
observe({what: "security_audit", checks: ["pii"]})
```

Finds personal data flowing through your application:
- Social Security Numbers
- Credit card numbers (with Luhn validation — not just pattern matching)
- Email addresses in unexpected API responses
- Phone numbers in contexts where they shouldn't appear

This matters for GDPR, CCPA, and HIPAA compliance. If your user list API is returning full SSNs when the frontend only needs names, your AI catches it during development.

### Security Header Analysis

```js
observe({what: "security_audit", checks: ["headers"]})
```

Validates that your responses include critical security headers:

| Header | What It Prevents |
|--------|-----------------|
| `Strict-Transport-Security` | Downgrade attacks, cookie hijacking |
| `X-Content-Type-Options` | MIME sniffing attacks |
| `X-Frame-Options` | Clickjacking |
| `Content-Security-Policy` | XSS, injection attacks |
| `Referrer-Policy` | Referrer leakage to third parties |
| `Permissions-Policy` | Unauthorized browser feature access |

Missing any of these? Your AI knows immediately — and can fix it.

### Cookie Security

```js
observe({what: "security_audit", checks: ["cookies"]})
```

Session cookies without `HttpOnly` are accessible to XSS attacks. Cookies without `Secure` can be intercepted over HTTP. Missing `SameSite` enables CSRF. Gasoline checks every cookie against every flag and rates severity based on whether it's a session cookie.

### Transport Security

```js
observe({what: "security_audit", checks: ["transport"]})
```

Detects:
- HTTP usage on non-localhost origins (unencrypted traffic)
- Mixed content (HTTPS page loading HTTP resources)
- HTTPS downgrade patterns

### Authentication Gaps

```js
observe({what: "security_audit", checks: ["auth"]})
```

Identifies API endpoints that return PII without requiring authentication. If `/api/users/123` returns a full user profile without an `Authorization` header, that's a finding.

## Third-Party Script Auditing

Third-party scripts are one of the largest attack surfaces in modern web applications. Every `<script src="...">` from an external CDN is a trust decision.

```js
observe({what: "third_party_audit"})
```

Gasoline classifies every third-party origin by risk:

- **Critical risk** — scripts from suspicious domains, data exfiltration patterns
- **High risk** — scripts from unknown origins, data sent to third parties with POST requests
- **Medium risk** — non-essential third-party resources, suspicious TLDs (`.xyz`, `.top`, `.click`)
- **Low risk** — fonts and images from known CDNs

It detects domain generation algorithm (DGA) patterns — high-entropy hostnames that indicate malware communication. It flags when your application sends PII-containing form data to third-party origins.

And it's configurable. Specify your first-party origins and custom allow/block lists:

```js
observe({what: "third_party_audit",
         first_party_origins: ["https://api.myapp.com"],
         custom_lists: {
           allowed: ["https://cdn.mycompany.com"],
           blocked: ["https://suspicious-tracker.xyz"]
         }})
```

## Security Regression Detection

Security isn't just about finding issues — it's about making sure fixes stay fixed.

```js
// Before your deploy
configure({action: "diff_sessions", session_action: "capture", name: "before-deploy"})

// After
configure({action: "diff_sessions", session_action: "capture", name: "after-deploy"})

// Compare
configure({action: "diff_sessions",
           session_action: "compare",
           compare_a: "before-deploy",
           compare_b: "after-deploy"})
```

The `security_diff` mode specifically tracks:
- **Headers removed** — did someone drop the CSP header?
- **Cookie flags removed** — did `HttpOnly` get lost in a refactor?
- **Authentication removed** — did an endpoint become public?
- **Transport downgrades** — did something switch from HTTPS to HTTP?

Each change is severity-rated. A removed CSP header is high severity. A transport downgrade is critical.

## Generating Security Artifacts

Gasoline doesn't just find problems — it generates the artifacts you need to fix and prevent them.

### Content Security Policy

```js
generate({format: "csp", mode: "strict"})
```

Gasoline observes which origins your page actually loads resources from during development and generates a CSP that allows exactly those origins — nothing more. It uses a confidence scoring system (3+ observations from 2+ pages = high confidence) to filter out extension noise and ad injection.

### Subresource Integrity Hashes

```js
generate({format: "sri"})
```

Every third-party script and stylesheet gets a SHA-384 hash. If a CDN is compromised and serves modified JavaScript, the browser refuses to execute it.

The output includes ready-to-paste HTML tags:

```html
<script src="https://cdn.example.com/lib.js"
        integrity="sha384-oqVuAfXRKap7fdgcCY5uykM6+R9GqQ8K/uxy9rx7HNQlGYl1kPzQho1wx4JwY8w"
        crossorigin="anonymous"></script>
```

## Automatic Credential Redaction

Even before auditing, Gasoline protects against accidental data exposure. The redaction engine automatically scrubs sensitive data from all MCP tool responses before they reach the AI:

- AWS keys become `[REDACTED:aws-key]`
- Bearer tokens become `[REDACTED:bearer-token]`
- Credit card numbers become `[REDACTED:credit-card]`
- SSNs become `[REDACTED:ssn]`

This is a double safety net. The extension strips auth headers before data reaches the server. The server's redaction engine catches anything else before it reaches the AI. Two layers, zero configuration.

## The Security Feedback Loop

Here's the workflow that makes Gasoline transformative for security:

1. **Develop normally** — write code, test features
2. **AI audits continuously** — security checks run against live traffic
3. **Issues found immediately** — in the same terminal where you're coding
4. **Fix in context** — the AI has the code open and the finding in hand
5. **Verify the fix** — re-run the audit, confirm the finding is gone
6. **Prevent regression** — capture a security snapshot, compare after future changes

The entire cycle takes minutes, not months. No separate tool. No context switch. No ticket in a backlog that nobody reads.

## What This Means for Teams

**For developers**: Security becomes part of your flow, not an interruption to it. The AI catches what you'd need a security expert to find — and you fix it while the code is still fresh in your mind.

**For security teams**: Shift-left isn't a buzzword anymore. Developers arrive at security review with most issues already caught and fixed. Reviews focus on architecture and design, not missing headers.

**For compliance**: Every audit finding is captured with timestamp, severity, and evidence. SARIF export integrates directly with GitHub Code Scanning. The audit log records every security check the AI performed.

**For enterprises**: Zero data egress. All security scanning happens on the developer's machine. No credentials sent to cloud services. No browser traffic leaving the network. Localhost only, zero dependencies, open source.

## Try It

Install Gasoline, open your application, and ask your AI:

> "Run a full security audit of this page and tell me what you find."

You might be surprised what's been hiding in plain sight.
