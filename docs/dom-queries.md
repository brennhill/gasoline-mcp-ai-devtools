---
title: "Live DOM Queries"
description: "Query the live DOM in your browser using CSS selectors via MCP. Let AI assistants inspect page structure, element attributes, and content in real time."
keywords: "DOM query MCP, live DOM inspection, CSS selector query, browser DOM state, page inspection tool"
permalink: /dom-queries/
header:
  overlay_image: /assets/images/hero-banner.png
  overlay_filter: 0.85
  excerpt: "Your AI can see the page structure. In real time."
toc: true
toc_sticky: true
---

Gasoline's `query_dom` tool lets AI assistants query the live DOM using CSS selectors — without you copying HTML or taking screenshots.

## <i class="fas fa-cogs"></i> How It Works

1. <i class="fas fa-terminal"></i> Your AI calls `query_dom` with a CSS selector
2. <i class="fas fa-server"></i> MCP server forwards the query to the extension
3. <i class="fas fa-code"></i> Extension runs the selector against the live page
4. <i class="fas fa-reply"></i> Results returned: tag, attributes, text, children

## <i class="fas fa-search"></i> Example Queries

- `query_dom("nav")` — navigation structure
- `query_dom(".error-message")` — visible error messages
- `query_dom("[data-testid='user-menu']")` — test-id elements
- `query_dom("form input[type='email']")` — form fields

## <i class="fas fa-th-list"></i> What's Returned

For each matching element:

- <i class="fas fa-tag"></i> Tag name
- <i class="fas fa-list-ul"></i> Attributes (id, class, data-*, aria-*)
- <i class="fas fa-font"></i> Text content
- <i class="fas fa-sitemap"></i> Child elements (limited depth)
- <i class="fas fa-eye"></i> Visibility state

## <i class="fas fa-info-circle"></i> `get_page_info`

Get basic page info:

- Current URL
- Page title
- Viewport dimensions

## <i class="fas fa-fire-alt"></i> Use Cases

- Verify UI state after an error
- Inspect form validation states
- Check if error messages are visible
- Understand page structure for selectors
- Debug rendering ("Is the element in the DOM?")
