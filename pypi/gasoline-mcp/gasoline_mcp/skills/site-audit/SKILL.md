---
name: site-audit
description: Map a site and produce an issue inventory by combining crawl coverage with logs, network failures, and UX evidence.
---

# Gasoline Site Audit

Use this skill for broad health checks across multiple pages.

## Inputs To Request

- Start URL
- Audit boundary (domain/path scope)
- Page budget (max pages)
- Priority flows

## Workflow

1. Define scope and budget:
Set strict crawl bounds to avoid unbounded exploration.

2. Build site map:
Navigate key routes and collect discovered pages and major interactions.

3. Collect diagnostics per page:
Capture `page_summary`, `errors`, `logs`, failing network requests, and one screenshot.

4. Aggregate recurring failures:
Group duplicate error signatures and repeated failing endpoints.

5. Flag high-risk paths:
Prioritize auth, checkout/upload, and any path with repeated client/server failures.

6. Produce remediation backlog:
Return ranked issues with evidence and suggested owner area.

## Output Contract

- `site_map`
- `issue_inventory`
- `recurring_failures`
- `high_risk_paths`
- `recommended_backlog`
