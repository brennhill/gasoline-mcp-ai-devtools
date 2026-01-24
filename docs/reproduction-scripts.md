---
title: "Reproduction Scripts"
description: "Automatically generate Playwright reproduction scripts from recorded user actions. Your AI captures what you did in the browser and produces a runnable script to reproduce bugs."
keywords: "reproduction script, bug reproduction, Playwright script, user action recording, browser session replay, automated reproduction, bug report"
permalink: /reproduction-scripts/
header:
  overlay_image: /assets/images/hero-banner.png
  overlay_filter: 0.85
  excerpt: "You triggered the bug. Gasoline writes the repro script."
toc: true
toc_sticky: true
---

Gasoline records your browser interactions and generates Playwright scripts that reproduce exactly what you did — clicks, navigation, form input, and all.

## <i class="fas fa-exclamation-circle"></i> The Problem

"Steps to reproduce" is the most dreaded section of any bug report. You found a bug, but:
- You can't remember the exact sequence of clicks
- The bug only happens after navigating through 5 pages
- It requires specific form input in a specific order
- By the time you write it up, you've lost the context

Your AI assistant saw the bug happen in real time through Gasoline. It can generate a reproduction script instantly.

## <i class="fas fa-cogs"></i> How It Works

1. <i class="fas fa-mouse-pointer"></i> Gasoline records every user action: clicks, typing, navigation, scrolls, selects
2. <i class="fas fa-layer-group"></i> Smart selectors are captured: test IDs, ARIA roles, labels, text, CSS paths
3. <i class="fas fa-code"></i> Your AI calls `generate` with `format: "reproduction"` to produce a Playwright script
4. <i class="fas fa-play"></i> The script is immediately runnable with `npx playwright test`

## <i class="fas fa-terminal"></i> Usage

```json
// Generate reproduction from all recorded actions
{ "tool": "generate", "arguments": { "format": "reproduction" } }

// Include error context in the script
{ "tool": "generate", "arguments": {
  "format": "reproduction",
  "error_message": "TypeError: Cannot read property 'id' of null"
} }

// Only use the last 10 actions
{ "tool": "generate", "arguments": {
  "format": "reproduction",
  "last_n_actions": 10
} }

// Replace the origin for local testing
{ "tool": "generate", "arguments": {
  "format": "reproduction",
  "base_url": "http://localhost:3000"
} }
```

## <i class="fas fa-file-code"></i> Generated Output

```javascript
import { test, expect } from '@playwright/test';

test('reproduction: TypeError: Cannot read property id of null', async ({ page }) => {
  await page.goto('http://localhost:3000/dashboard');

  await page.getByRole('button', { name: 'Add User' }).click();
  await page.getByLabel('Email').fill('test@example.com');
  await page.getByRole('button', { name: 'Submit' }).click();

  // [3s pause]
  await page.waitForURL('/dashboard/users');
  await page.getByTestId('user-row-1').click();

  // Error occurred here: TypeError: Cannot read property 'id' of null
});
```

## <i class="fas fa-crosshairs"></i> Smart Selectors

Gasoline captures multiple selector strategies and picks the most resilient one:

| Priority | Selector Type | Example |
|----------|--------------|---------|
| 1 | Test ID | `getByTestId('submit-btn')` |
| 2 | ARIA role + name | `getByRole('button', { name: 'Submit' })` |
| 3 | ARIA label | `getByLabel('Email')` |
| 4 | Text content | `getByText('Sign In')` |
| 5 | Element ID | `locator('#user-email')` |
| 6 | CSS path | `locator('div.form > input')` |

This prioritization follows Playwright's recommended selector strategy — test IDs first, CSS paths as last resort.

## <i class="fas fa-clock"></i> Timing Awareness

The generated script includes pause comments for gaps longer than 2 seconds:

```javascript
await page.getByRole('button', { name: 'Submit' }).click();

// [3s pause]
await page.waitForURL('/results');
```

This helps identify where the app was loading or processing between user actions.

## <i class="fas fa-shield-alt"></i> Privacy

- Sensitive form inputs (passwords) are redacted as `[user-provided]`
- Scripts use `base_url` replacement so production URLs aren't hardcoded
- Generated scripts never contain auth tokens or session data

## <i class="fas fa-link"></i> Related

- [Test Generation](/generate-test/) — Full regression tests with API assertions
- [Session Checkpoints](/session-checkpoints/) — Scope reproduction to a specific time window
