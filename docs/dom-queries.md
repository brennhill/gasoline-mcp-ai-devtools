---
title: "Live DOM Queries"
description: "Query the live DOM in your browser using CSS selectors via MCP. Let AI assistants inspect page structure, element attributes, and content in real time."
keywords: "DOM query MCP, live DOM inspection, CSS selector query, browser DOM state, page inspection tool"
permalink: /dom-queries/
toc: true
toc_sticky: true
---

Gasoline's `query_dom` MCP tool lets AI assistants query the live DOM in your browser using CSS selectors — without you needing to copy HTML or take screenshots.

## How It Works

1. Your AI calls `query_dom` with a CSS selector
2. The MCP server forwards the query to the browser extension
3. The extension runs the selector against the live page
4. Results are returned with element tag, attributes, text content, and children

## MCP Tool: `query_dom`

Query the page with any CSS selector:

- `query_dom("nav")` — get the navigation structure
- `query_dom(".error-message")` — find visible error messages
- `query_dom("[data-testid='user-menu']")` — find test-id elements
- `query_dom("form input[type='email']")` — inspect form fields

## What's Returned

For each matching element:

- Tag name
- Attributes (id, class, data-*, aria-*, etc.)
- Text content
- Child elements (limited depth)
- Visibility state

## MCP Tool: `get_page_info`

Get basic page information:

- Current URL
- Page title
- Viewport dimensions

## Use Cases

- Verify UI state matches expectations after an error
- Inspect form field values and validation states
- Check if error messages are displayed to the user
- Understand page structure for writing selectors or tests
- Debug rendering issues ("Is the element actually in the DOM?")
