# Audit and Security Workflow

Use this workflow for site audits, UX reviews, accessibility checks, security audits, and API validation.
Merges: site-audit, ux-audit, security-redaction, api-validation.

## Inputs

- Start URL and audit boundary (allowed domains/paths)
- Auth/session readiness
- Page budget and time budget
- Priority flows and exclusions (logout, destructive actions, billing)
- Secret classes to test (for security: API keys, tokens, cookies)
- API endpoints/operations to validate (for API validation)

## Step 1: Confirm Starting State

```bash
bash scripts/ensure-daemon.sh
bash scripts/kaboom-call.sh observe '{"what":"tabs"}'
bash scripts/kaboom-call.sh observe '{"what":"page"}'
```

Verify the user is logged in and on the intended start page.

## Step 2: Discover Navigation Structure

```bash
# Explore the current page
bash scripts/kaboom-call.sh interact '{"what":"explore_page"}'

# List all interactive elements
bash scripts/kaboom-call.sh interact '{"what":"list_interactive","visible_only":true}'

# Get page structure
bash scripts/kaboom-call.sh analyze '{"what":"page_structure"}'

# Capture navigation architecture
bash scripts/kaboom-call.sh analyze '{"what":"navigation"}'
```

Build a discovery queue from nav, sidebars, footers, and major CTAs.

## Step 3: Per-Page Audit

For each page in the queue:

```bash
# Navigate
bash scripts/kaboom-call.sh interact '{"what":"navigate","url":"<page_url>","wait_for_stable":true}'

# Screenshot evidence (top, scrolled, bottom)
bash scripts/kaboom-call.sh observe '{"what":"screenshot","save_to":"audit/<page_id>_top.png"}'
bash scripts/kaboom-call.sh interact '{"what":"scroll_to","direction":"bottom"}'
bash scripts/kaboom-call.sh observe '{"what":"screenshot","save_to":"audit/<page_id>_bottom.png"}'

# Page summary and diagnostics
bash scripts/kaboom-call.sh analyze '{"what":"page_summary"}'
bash scripts/kaboom-call.sh observe '{"what":"errors","limit":20}'
bash scripts/kaboom-call.sh observe '{"what":"network_waterfall","summary":true}'
```

Per-page capture checklist:
- Page purpose and primary user jobs
- Major UI regions/components
- Outbound links and routes
- Key interactions and outcomes
- UX strengths and issues
- Errors/warnings/network failures

## Step 4: Accessibility Audit

```bash
# Full accessibility check
bash scripts/kaboom-call.sh analyze '{"what":"accessibility","summary":true}'

# Detailed check on specific area
bash scripts/kaboom-call.sh analyze '{"what":"accessibility","selector":"main","tags":["wcag2a","wcag2aa"]}'

# Form analysis
bash scripts/kaboom-call.sh analyze '{"what":"forms"}'
bash scripts/kaboom-call.sh analyze '{"what":"form_validation","summary":true}'

# Export as SARIF for CI integration
bash scripts/kaboom-call.sh interact '{"what":"run_a11y_and_export_sarif","save_to":"audit/a11y.sarif"}'
```

Check: labels, focus flow, landmarks, contrast, heading order, keyboard navigation.

## Step 5: Security Audit

```bash
# Comprehensive security scan
bash scripts/kaboom-call.sh analyze '{"what":"security_audit","summary":true}'

# Targeted checks
bash scripts/kaboom-call.sh analyze '{"what":"security_audit","checks":["credentials","pii","headers","cookies"]}'

# Third-party script analysis
bash scripts/kaboom-call.sh analyze '{"what":"third_party_audit","summary":true}'

# Generate CSP recommendation
bash scripts/kaboom-call.sh generate '{"what":"csp","mode":"strict"}'

# Generate SRI hashes
bash scripts/kaboom-call.sh generate '{"what":"sri"}'
```

### Security Redaction Testing

Seed synthetic secrets and verify they are redacted in all outputs:

```bash
# Trigger flows that handle sensitive data
bash scripts/kaboom-call.sh observe '{"what":"network_bodies","limit":20}'
bash scripts/kaboom-call.sh observe '{"what":"logs","limit":30}'
bash scripts/kaboom-call.sh observe '{"what":"storage"}'
```

Check: raw and transformed outputs for token/secret leaks.

## Step 6: API Validation

```bash
# Collect traffic
bash scripts/kaboom-call.sh observe '{"what":"network_waterfall","limit":50}'
bash scripts/kaboom-call.sh observe '{"what":"network_bodies","limit":30}'

# Run API contract validation
bash scripts/kaboom-call.sh analyze '{"what":"api_validation"}'

# Get validation report
bash scripts/kaboom-call.sh analyze '{"what":"api_validation","operation":"report"}'

# Link health check
bash scripts/kaboom-call.sh analyze '{"what":"link_health","timeout_ms":15000}'
```

Classify mismatches: schema drift, status drift, auth failure, routing mismatch, serialization issue.

## Step 7: UX Assessment

Review across all captured evidence:

- Information hierarchy: heading order, CTA prominence, scan order, above-the-fold clarity
- Interaction clarity: affordances, feedback, error/help text quality
- Consistency: visual patterns, terminology, behavior across pages
- Loading/empty/error states: are they handled gracefully?

## Step 8: Generate Report

```bash
# Generate SARIF report for all findings
bash scripts/kaboom-call.sh generate '{"what":"sarif","save_to":"audit/full-report.sarif"}'

# Generate annotation report if draw mode was used
bash scripts/kaboom-call.sh generate '{"what":"annotation_report","save_to":"audit/annotations.md"}'
```

### Report Structure

1. Executive summary
2. Scope, constraints, and method
3. Coverage summary (routes discovered, audited, gaps)
4. Navigation and menu inventory
5. Page-by-page findings (with screenshot references)
6. Feature-by-feature findings
7. Accessibility assessment
8. Security findings
9. API contract findings
10. UX/usability report
11. Strengths (what works well)
12. Issues ranked by severity
13. Prioritized remediation plan
14. Evidence appendix

## Troubleshooting

- **Blocked by auth:** Verify session is active with `observe(storage, storage_type="cookies")`.
- **Too many pages:** Set strict crawl bounds and use page budget. Prioritize breadth first.
- **Accessibility timeout:** Use `background:true` and poll with `observe(command_result)`.
- **Security false positives:** Use `severity_min:"high"` to filter noise.
