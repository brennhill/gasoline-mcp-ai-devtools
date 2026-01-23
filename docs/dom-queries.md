---
title: "Live DOM Queries"
description: "Query the live DOM in your browser using CSS selectors via MCP. Let AI assistants inspect page structure, element attributes, and content in real time."
keywords: "DOM query MCP, live DOM inspection, CSS selector query, browser DOM state, page inspection tool, AI DOM access"
permalink: /dom-queries/
header:
  overlay_image: /assets/images/hero-banner.png
  overlay_filter: 0.85
  excerpt: "Your AI can see the page structure. In real time."
toc: true
toc_sticky: true
---

Gasoline's `query_dom` tool lets AI assistants query the live DOM using CSS selectors — without you copying HTML or taking screenshots.

## <i class="fas fa-exclamation-circle"></i> The Problem

When your AI asks "what does the button say?" or "is the modal visible?", you have to inspect the page manually and describe what you see. Screenshots help but don't give your AI structured data it can reason about.

With DOM queries, your AI just asks the browser directly.

## <i class="fas fa-cogs"></i> How It Works

1. <i class="fas fa-terminal"></i> Your AI calls `query_dom` with a CSS selector
2. <i class="fas fa-server"></i> MCP server forwards the query to the extension
3. <i class="fas fa-code"></i> Extension runs the selector against the live page
4. <i class="fas fa-reply"></i> Results returned: tag, attributes, text, children

```json
// AI asks: "What's in the error banner?"
{
  "selector": ".error-banner"
}

// Response:
{
  "elements": [{
    "tag": "div",
    "className": "error-banner visible",
    "textContent": "Invalid email address",
    "attributes": {
      "role": "alert",
      "aria-live": "polite"
    }
  }]
}
```

## <i class="fas fa-search"></i> Example Queries

- `query_dom("nav")` — navigation structure
- `query_dom(".error-message")` — visible error messages
- `query_dom("[data-testid='user-menu']")` — test-id elements
- `query_dom("form input[type='email']")` — form fields
- `query_dom(".spinner")` — check if loading is visible

## <i class="fas fa-th-list"></i> What Gets Returned

For each matching element:

| Field | Description |
|-------|-------------|
| <i class="fas fa-tag"></i> Tag name | `div`, `button`, `input`, etc. |
| <i class="fas fa-list-ul"></i> Class list | Current CSS classes |
| <i class="fas fa-font"></i> Text content | Visible text |
| <i class="fas fa-list"></i> Attributes | id, role, aria-*, data-*, type, value |
| <i class="fas fa-eye"></i> Computed state | Visibility, disabled status |
| <i class="fas fa-sitemap"></i> Children | Child elements (limited depth) |

## <i class="fas fa-info-circle"></i> `get_page_info`

Get high-level page context without a selector:

```json
{
  "url": "http://localhost:3000/dashboard",
  "title": "Dashboard - MyApp",
  "viewport": { "width": 1440, "height": 900 }
}
```

## <i class="fas fa-fire-alt"></i> Use Cases

### Verifying UI State

> "Is the loading spinner showing?"

Your AI queries `.spinner` and knows instantly — no screenshot needed.

### Debugging Form Issues

> "Why won't the form submit?"

Your AI queries the submit button to check if it's disabled, and the form inputs for validation states.

### Checking Rendered Data

> "What items are in the list?"

Your AI queries `.list-item` and sees all rendered items with their content.

### Confirming Fixes

After applying a fix, your AI queries the relevant element to verify the change took effect without you refreshing and checking manually.

### Error Message Inspection

> "What error is the user seeing?"

Your AI queries `[role="alert"]` or `.error-message` and reads the exact text — then correlates with console errors and network failures to diagnose the root cause.
