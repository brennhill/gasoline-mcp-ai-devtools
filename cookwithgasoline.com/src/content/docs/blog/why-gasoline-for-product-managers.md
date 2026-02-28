---
title: "Why Product Managers Love Gasoline"
date: 2026-02-07
authors:
  - brenn
tags:
  - product-managers
  - use-cases
---

Record demos, explore bugs, file detailed issue reports, and even fix simple issues — all without waiting for an engineer. Gasoline gives PMs superpowers.

<!-- more -->

## The PM Bottleneck

You're the product manager. You know the product better than anyone. You found the bug, you can reproduce it, you know exactly when it started. But you can't fix it. You can't even file a _useful_ bug report without an engineer helping you extract the console errors, network responses, and steps to reproduce.

So you write "the checkout button doesn't work sometimes" in Jira, attach a screenshot, and wait three days for an engineer to ask you to reproduce it on a call.

Gasoline changes this equation. With your AI tool connected to the browser, you can:

1. **Record polished product demos** without engineering help
2. **Explore and diagnose bugs** with full technical context
3. **File rich issue reports** with errors, network data, and reproduction scripts
4. **Fix simple issues directly** — yes, actually fix them

## Superpower 1: Record Product Demos

Writing a demo used to mean begging engineering for a staging environment and a walkthrough. Now you write a script in English:

```text
Demo: New Onboarding Flow

1. Navigate to the signup page
2. Subtitle: "New users see a simplified signup with just email and password."
3. Fill in the signup form with test data
4. Click Create Account
5. Subtitle: "After signup, users land directly in the guided setup."
6. Wait for the setup wizard to load
7. Subtitle: "The wizard adapts based on the user's role selection."
8. Select "Product Manager" from the role dropdown
9. Click Continue
10. Subtitle: "PMs get a pre-built dashboard with team metrics and sprint progress."
```

Hand this to your AI. It drives the browser, shows the subtitles, and you narrate over the top. Save it. Replay it for the next meeting. Modify it for a different audience.

No engineering support. No screen recording software. No slides.

## Superpower 2: Explore and Diagnose Bugs

You found a bug. Instead of writing a vague description, tell your AI:

_"The checkout page is showing the wrong total. Can you check what's happening?"_

Your AI, connected to the browser via Gasoline, does what an engineer would do:

**Checks the console:**
```js
observe({what: "errors"})
// -> TypeError: Cannot read property 'discount' of undefined at cart.js:142
```

**Checks the API response:**
```js
observe({what: "network_bodies", url: "/api/cart"})
// -> Response: {items: [...], discount: null, total: 89.99}
```

**Checks the visual state:**
```js
observe({what: "screenshot"})
// -> The page shows "$89.99" but the discount line shows "$0.00 off"
```

**Diagnosis:** "The API returns `discount: null` instead of `discount: {amount: 10, code: 'SAVE10'}`. The frontend crashes trying to read `discount.amount` from null. The total is correct, but the discount display is broken."

You just did 15 minutes of engineering triage in 30 seconds. Without opening DevTools. Without knowing JavaScript.

## Superpower 3: File Rich Issue Reports

Instead of "checkout is broken," you file:

```text
Bug: Discount not displaying on checkout page

Steps to Reproduce:
1. Add item to cart
2. Apply discount code "SAVE10"
3. Navigate to checkout

Expected: Discount shows "$10.00 off"
Actual: Discount shows "$0.00 off", console error on page

Technical Details:
- Console error: TypeError at cart.js:142 — discount is null
- API response from /api/cart: discount field is null
  (expected: {amount: 10, code: "SAVE10"})
- The /api/cart/apply-discount endpoint returned 200
  but didn't persist the discount to the cart object

Reproduction script attached.
```

Your AI can also generate a Playwright reproduction script:

```js
generate({format: "reproduction"})
```

The engineer gets a one-click reproduction, the exact error, the API response, and a root cause hypothesis. They start _fixing_, not _investigating_.

## Superpower 4: Fix Simple Issues Directly

This is the big one. For certain classes of issues, you don't need an engineer at all.

**Copy changes:** "The button says 'Submit' but it should say 'Save Changes.'" Tell your AI, it finds the text in the codebase, changes it, runs the tests, and opens a PR.

**Configuration issues:** "The timeout on the upload page is too short — users with large files are getting errors." Your AI observes the error, finds the timeout configuration, adjusts it, and verifies the fix.

**Styling issues:** "The modal is cut off on mobile." Your AI takes a screenshot, identifies the CSS issue, fixes it, and shows you the before/after.

You're not writing code. You're describing the problem in English, and the AI — with full visibility into the browser via Gasoline — has enough context to fix it.

Obviously, this doesn't replace engineers for complex features, architectural decisions, or security-critical changes. But for the dozens of small issues that sit in the backlog because they're "not worth an engineer's time" — now they're worth _your_ time, because your time is all it takes.

## Superpower 5: Create Living Acceptance Tests

You write acceptance criteria anyway. Now they _run_:

```text
Acceptance Criteria: User Registration

1. Navigate to the registration page
2. Fill in name, email, and password
3. Check the Terms of Service checkbox
4. Click Create Account
5. Verify the welcome page loads
6. Verify no errors in the console
7. Verify the registration API returned 200
```

Tell your AI to run it against staging after each deploy. You get a report: "All 7 steps passed" or "Step 5 failed — the welcome page returned a 500 error from the API."

You're not depending on QA bandwidth. You're not waiting for an engineer to write the test. The acceptance criteria you already wrote _are_ the test.

## The Productivity Shift

| Task | Before Gasoline | With Gasoline |
|---|---|---|
| **File a bug report** | Screenshot + vague description | Full technical report with errors, API data, and reproduction script |
| **Record a demo** | Coordinate with engineering, screen record | Write a text script, AI runs it, replay anytime |
| **Validate a fix** | Wait for deploy, manually test | AI runs the acceptance test and reports results |
| **Explore an issue** | Open DevTools (if you know how) | Tell AI "what's happening on this page?" |
| **Fix a copy typo** | File a ticket, wait for prioritization | AI fixes it and opens a PR in 2 minutes |
| **Run acceptance tests** | Depend on QA schedule | Run your natural language tests on demand |

## Getting Started as a PM

1. **Install Gasoline** — Follow the [Getting Started](/getting-started/) guide. It takes 2 minutes.
2. **Connect your AI tool** — Claude Code, Cursor, or any MCP-compatible tool.
3. **Start with observation** — Browse your product normally and ask the AI: "What errors are happening on this page?" You'll be surprised what you find.
4. **Try a demo script** — Write 5 steps in English. Ask the AI to run them. See it work.
5. **File your first rich bug report** — Next time you find a bug, ask the AI to diagnose it. Paste the diagnosis into your ticket.

You don't need to learn JavaScript. You don't need to understand CSS selectors. You don't need DevTools.

You need to describe what you see and what you expect. The AI and Gasoline handle the rest.
