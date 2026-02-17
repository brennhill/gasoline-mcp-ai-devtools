---
feature: local-web-scraping
status: proposed
doc_type: tech-spec
feature_id: feature-local-web-scraping
last_reviewed: 2026-02-16
---

# Tech Spec: Local Web Scraping

> Plain language only. No code. Describes HOW the implementation works at a high level.

## Architecture Overview

Local web scraping is a workflow pattern, not a new technical feature. It uses existing tools (interact for navigation/execution, observe for data extraction, configure for DOM queries, generate for export) orchestrated by the AI agent. The key insight: user's active browser session provides authentication, extension provides telemetry and control.

## Key Components

- **Session persistence**: Browser maintains cookies, localStorage, sessionStorage across agent operations
- **Navigate action**: Move between pages within authenticated session
- **Execute_js action**: Extract data via custom JavaScript in page context
- **Query_dom action**: Wait for dynamic content to load before extraction
- **Generate tool**: Export scraped data as JSON

## Data Flows

```
User logs in manually (session established)
  → Agent: observe({what: "page"}) — confirms logged in
  → Agent: interact({action: "navigate", url: "/data-page"})
  → Agent: configure({action: "query_dom", selector: ".data-table", wait: true})
  → Agent: interact({action: "execute_js", code: "/* extract table */"})
  → Agent receives extracted data
  → Agent: generate({type: "json_export", data: scraped})
  → Server writes JSON to file
```

## Implementation Strategy

**No new server/extension code required.** This is a documentation and workflow pattern feature.

### Agent workflow for scraping:
1. Verify authentication: Use observe({what: "page"}) to check for login indicators
2. Navigate to target: Use interact({action: "navigate"})
3. Wait for content: Use configure({action: "query_dom", wait: true}) for AJAX-loaded elements
4. Extract data: Use interact({action: "execute_js"}) with custom extraction code
5. Handle pagination: Loop with click next + wait for content
6. Rate limit: Add delays between requests (agent responsibility)
7. Export: Use generate({type: "json_export"})

### Session handling:
- Browser automatically maintains cookies across navigation
- No special session management needed
- If session expires, agent detects (login page appears), alerts user

### Dynamic content handling:
- Use query_dom with wait=true to detect when elements appear
- Use execute_js to poll for AJAX completion indicators
- Set appropriate timeouts (2-10s) for slow-loading content

## Edge Cases & Assumptions

- **Edge Case 1**: Session expires mid-scrape → **Handling**: Agent detects login page, returns error, user must re-authenticate
- **Edge Case 2**: AJAX content never loads → **Handling**: query_dom times out, agent handles error
- **Edge Case 3**: Pagination changes URL structure → **Handling**: Agent detects URL pattern, adjusts navigation
- **Edge Case 4**: Rate limiting by target site → **Handling**: Agent must implement delays, handle 429 errors
- **Assumption 1**: User manually authenticates before scraping starts
- **Assumption 2**: Target site allows scraping (ethical/legal responsibility)

## Risks & Mitigations

- **Risk 1**: Agent scrapes too fast, triggers rate limits → **Mitigation**: Document rate limiting best practices
- **Risk 2**: Session expires, scrape fails → **Mitigation**: Agent checks authentication before each operation
- **Risk 3**: Dynamic content timing issues → **Mitigation**: Use query_dom wait, increase timeouts
- **Risk 4**: Violating site's ToS → **Mitigation**: User responsibility, document ethical scraping guidelines

## Dependencies

- Existing interact tool (navigate, execute_js)
- Existing configure tool (query_dom with wait)
- Existing generate tool (json export capability)
- Browser session management (cookies, storage)

## Performance Considerations

- Navigation: 1-5s per page load
- Data extraction: <500ms per page (execute_js is fast)
- Large datasets: Agent should batch, avoid memory issues
- Rate limiting: Agent should add 1-2s delays between pages

## Security Considerations

- Scraping uses user's authenticated session (user's permissions apply)
- No elevation of privilege
- Agent cannot bypass authentication (requires manual login)
- Scraped data subject to same redaction rules as logs
- User responsible for securing exported JSON files
- Cannot scrape cross-origin content (same-origin policy enforced)
