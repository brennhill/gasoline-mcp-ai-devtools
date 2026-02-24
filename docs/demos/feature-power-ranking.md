---
doc_type: legacy_doc
status: reference
last_reviewed: 2026-02-16
---

# Gasoline — Feature Power Ranking for Demos

## Tier 1 — "This changes everything"

### 1. Error Bundles
One call returns the error + network requests + user actions + console logs from a configurable time window around it. The AI doesn't just see the error — it sees the full story of what caused it.

**Demo value:** Highest. Shows something no other tool does — contextual debugging in a single call.

### 2. AI Browser Control + Observe Loop
The AI navigates the site, triggers bugs, reads the telemetry, diagnoses, then fixes the code — all autonomously. Closed-loop debugging with no human intervention.

**Demo value:** Highest. The "magic trick" moment. Audience sees AI driving the browser and fixing bugs in real-time.

### 3. Test Generation from Live Sessions
Instead of writing tests from imagination, the AI interacts with the app, captures what actually happened, and generates Playwright tests grounded in real behavior.

**Demo value:** Very high. Developers hate writing tests. This does it for them from facts, not guesses.

### 4. Accessibility Audit → Auto-Fix Loop
Scan for WCAG violations, AI reads structured findings (not screenshots), fixes the code, re-scans to confirm. Measurable before/after.

**Demo value:** Very high. Clear numeric progress (12 violations → 0). Compliance is expensive; this makes it cheap.

### 5. Security Audit + CSP/SRI Generation
One call finds vulnerabilities. Follow-up calls generate the fix (Content Security Policy, Subresource Integrity hashes). Discovery + remediation in one flow.

**Demo value:** High. Security is scary. Showing automated hardening is compelling.

---

## Tier 2 — "That's incredibly useful"

### 6. Performance Vitals + Regression Detection
Track LCP/CLS/INP, detect regressions after code changes, pinpoint the cause via timeline correlation.

**Demo value:** High. Measurable before/after metrics. Core Web Vitals are universally understood.

### 7. Noise Filtering (auto_detect)
AI asks Gasoline to analyze the error buffer and auto-suggest noise rules. Instantly silences framework spam, analytics errors, and extension noise.

**Demo value:** Medium-high. Subtle but powerful. Shows AI debugging real bugs, not noise.

### 8. Network Waterfall + Body Capture
Full request/response pairs including bodies. AI can see malformed API payloads, wrong status codes, missing fields.

**Demo value:** High for API-heavy demos. Shows depth of capture beyond DevTools.

### 9. Recording + Playback + Log Diff
Record a user flow, replay it later, diff the logs. Regression testing without a test suite.

**Demo value:** High. Visual and intuitive. "Record once, catch regressions forever."

### 10. Subtitle Narration
Real-time captions overlaid on the browser during any action. Makes demos self-explanatory.

**Demo value:** Meta — makes all OTHER demos better. Essential for video recordings.

---

## Tier 3 — Supporting features (used in demos, not headlined)

- **DOM Querying** — Extract structured data from live DOM without screenshots
- **Timeline** — Unified chronological view of all events
- **Error Clustering** — Group similar errors by pattern
- **Link Health** — Check all links for 404s/broken resources
- **Third-party Audit** — Identify external dependencies and data flows
- **HAR Export** — HTTP Archive for sharing/debugging
- **SARIF Export** — Static analysis format for CI/CD integration
- **State Snapshots** — Save/load page state for comparison
- **Test Boundaries** — Mark start/end of test for log correlation
- **Streaming Events** — Push notifications for real-time monitoring
