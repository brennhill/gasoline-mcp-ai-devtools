---
doc_type: legacy_doc
status: reference
last_reviewed: 2026-02-16
---

# Replay from Natural Language

**Duration:** 2-3 minutes
**Focus:** Describing what you want to test in plain English, then letting an LLM (Claude Code with Gasoline) execute it and capture the results.

## What This Demo Shows

Instead of writing test code, you describe a scenario in natural language:

> "Navigate to the recipe search page, search for 'carbonara', click the top result, and tell me if the cooking time is correct."

Claude Code reads this, uses Gasoline to execute the actions, and returns the results—with full telemetry captured along the way. No code required.

This is the inverse of "recording for replay." Instead of record-first, it's replay-first.

---

## The Setup

**Page:** `https://cookwithgasoline.com/demo/recipe-search`

**LLM:** Claude Code (with Gasoline MCP enabled)

---

## The Natural Language Request

```
"Using Gasoline, navigate to https://cookwithgasoline.com/demo/recipe-search
and search for 'carbonara'. Click on the first recipe result and take a screenshot
showing the recipe details. Also tell me the cooking time and any errors that occurred
during the interactions. Show me the network timeline and performance metrics."
```

---

## What Claude Code Does (The Gasoline Script)

Claude Code interprets the natural language request and builds this Gasoline script:

```
Navigate to: https://cookwithgasoline.com/demo/recipe-search
[2s pause]
Click: searchbox
Type "carbonara" into: searchbox
[4s pause]
Click: first recipe card
[3s pause]
Take screenshot
Extract text from: [data-testid=cook-time]
Show network timeline
Show performance metrics
Check for errors
```

**What Gasoline captures:**
- Network timeline: 2 API calls (search + recipe detail)
- Performance metrics: FCP 0.8s, LCP 1.1s
- Recipe cooking time: 45 minutes
- Errors: None detected

---

## Audio Narration

```
"Imagine you want to test a recipe site, but you don't want to write code.
You just describe what you want: 'Search for carbonara and show me the recipe details.'

Claude Code reads that request and uses Gasoline to make it happen.

[As Claude navigates]
Watch: Claude navigates to the page, finds the search box, types your query.
The subtitle shows what Claude is doing, step by step.

[As results appear]
Results loaded. Claude clicks the first recipe.

[Detail page appears]
Here's the recipe. Claude captured everything:
- The network requests that fetched the recipe data
- Performance timing: page loaded in 1.2 seconds
- The cooking time: 45 minutes
- Any errors that occurred during interaction

All of this came from a natural language description. No test code, no manual Gasoline commands.

Claude saw your request, understood the intent, built the Gasoline script, and executed it.
The results flow back with full telemetry attached."
```

---

## Key Features Highlighted

✅ **Natural Language Input** — Describe what you want, Claude Code understands it
✅ **Automatic Script Generation** — Claude builds the Gasoline script from your description
✅ **Live Execution** — Gasoline executes the script and captures all telemetry
✅ **Structured Output** — Results include screenshots, text extraction, network data, performance metrics
✅ **Zero Code** — No test frameworks, no syntax, just English

---

## What Claude Code Returns

```
Here's what I found:

**Navigation & Interaction:**
✓ Navigated to https://cookwithgasoline.com/demo/recipe-search
✓ Searched for "carbonara"
✓ Opened first recipe result

**Recipe Details:**
- Title: Carbonara (Classic Roman Style)
- Cooking Time: 45 minutes
- Difficulty: Medium
- Servings: 4

**Performance Metrics:**
- Page Load Time: 1.2s
- First Contentful Paint: 0.8s
- Largest Contentful Paint: 1.1s

**Network Activity:**
- API calls: 2
  - GET /api/recipes?q=carbonara (240ms)
  - GET /api/recipes/123 (180ms)
- Total network time: 420ms

**Errors:**
- None detected

**Screenshot Captured:**
[Image of recipe detail page]
```

---

## Why This Matters

- **Anyone can write tests** — Non-engineers can describe scenarios, Claude executes them
- **Natural communication** — Describe bugs, features, edge cases in English
- **Full telemetry capture** — Every script run gets network, performance, error data
- **Fast feedback loops** — Get results in seconds, not hours of test writing
- **Audit trail** — The Gasoline data proves what happened during execution

---

## Real-World Use Cases

| Scenario | What You Say | What Claude Does |
|----------|--------------|------------------|
| Bug report | "Try to buy something but it fails on checkout" | Records the error, network calls, screenshot |
| Feature test | "Sign up as a new user and verify email confirmation" | Executes signup, captures telemetry |
| Performance audit | "Load the homepage and measure how fast it is" | Gets First Contentful Paint, Largest Contentful Paint, network timeline |
| Accessibility check | "Navigate the site using keyboard only" | Runs interactions, captures accessibility audit results |
| Security baseline | "Check for XSS vulnerabilities on the search page" | Runs security_audit, returns findings |

