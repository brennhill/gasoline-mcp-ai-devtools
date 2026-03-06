---
title: "One Tool Replaces Four: How Gasoline MCP Eliminates Loom, DevTools, Selenium, and Playwright"
date: 2026-02-07
authors: [brenn]
tags: [product, ai-development, productivity]
---

Most development teams juggle at least four tools to ship a feature: Loom for demos and bug reports, Chrome DevTools for debugging, Selenium or Playwright for automated testing, and some combination of all three for QA. Each tool has its own setup, its own learning curve, and its own context switch.

Gasoline MCP replaces all four with a single Chrome extension and one MCP server. And the result isn't just fewer tools — it's dramatically faster cycle times.

<!-- more -->

## The Four Tools You're Using Today

### Loom — "Let Me Show You What's Happening"

Product managers record Loom videos to demo features. Developers record Loom videos to show bugs. QA records Loom videos to document test failures. Everyone records Loom videos because the alternative — writing a detailed description with screenshots — takes even longer.

**The problem**: Loom videos are static. They can't be replayed against a new build. They can't be edited when the flow changes. They can't be version-controlled. And they require $12.50/user/month.

### Chrome DevTools — "Let Me Check the Console"

Every debugging session starts with opening DevTools, switching between Console, Network, and Elements tabs, copying error messages, and pasting them somewhere the AI or another developer can see them.

**The problem**: DevTools is manual and disconnected. The AI can't see what's in DevTools. You're the human bridge between the browser and your tools.

### Selenium / WebDriver — "Let Me Automate This"

Automated browser testing requires WebDriver binaries, a programming language (Java, Python, JavaScript), and coded selectors that break whenever the UI changes.

**The problem**: High setup cost, high maintenance cost, requires developer skills. Product managers and QA without coding experience can't use it.

### Playwright — "Let Me Write a Proper Test"

Modern browser automation that's better than Selenium but still requires JavaScript/TypeScript, an npm project, and coded selectors.

**The problem**: Same fundamental issue — you need code to create tests. And when tests break (they always break), you need code to fix them.

## How Gasoline Replaces Each One

### Loom → Subtitles + Demo Scripts

Instead of recording a video:

```
"Navigate to the dashboard. Add a subtitle: 'Welcome to the Q1 report.'
Click the revenue tab. Subtitle: 'Revenue is up 23% quarter over quarter.'
Click the export button. Subtitle: 'One click to export to PDF.'"
```

The AI navigates the application while displaying narration text at the bottom of the viewport — like closed captions. Action toasts show what's happening ("Click: Revenue Tab"). The audience watches a live, narrated walkthrough.

**Why it's better than Loom**:
- **Replayable** — run the same script against tomorrow's build
- **Editable** — change one line of text, not re-record a whole video
- **Adaptive** — semantic selectors survive UI redesigns
- **Versionable** — store scripts in your repo, diff them in PRs
- **Free** — no per-seat subscription

### Chrome DevTools → observe()

Instead of opening DevTools and copy-pasting:

```
"What browser errors do you see?"
```

The AI calls `observe({what: "errors"})` and sees every console error with full stack traces. Then `observe({what: "network_bodies", url: "/api"})` for the API response body. Then `observe({what: "websocket_status"})` for WebSocket connection state. Then `observe({what: "vitals"})` for performance metrics.

**Why it's better than DevTools**:
- **The AI sees it directly** — no human copy-paste bridge
- **Everything in one place** — errors, network, WebSocket, performance, accessibility, security
- **Correlated** — `error_bundles` returns the error with its network context and user actions
- **Persistent** — data doesn't vanish on page refresh
- **Actionable** — the AI diagnoses and fixes, not just observes

### Selenium → interact() + Natural Language

Instead of writing Java with WebDriver:

```
"Go to the registration page. Fill in 'Jane Doe' as the name,
'jane@example.com' as the email, and 'secure123' as the password.
Click Register. Verify you see the welcome message."
```

The AI navigates, types, clicks, and verifies — using semantic selectors (`label=Name`, `text=Register`) that survive UI changes.

**Why it's better than Selenium**:
- **No code** — describe the test in English
- **No setup** — no WebDriver, no JDK, no project scaffolding
- **Resilient** — semantic selectors adapt to redesigns
- **Anyone can use it** — PMs, QA, designers, not just developers

### Playwright → generate(format: "test")

After running a natural language test, lock it in for CI:

```js
generate({format: "test", test_name: "registration-flow",
          assert_network: true, assert_no_errors: true})
```

Gasoline generates a complete Playwright test from the session — real selectors, network assertions, error checking. The AI explored in English; Gasoline exports for CI/CD.

**Why it's better than writing Playwright by hand**:
- **Faster** — describe the flow, don't code it
- **Accurate** — generated from real browser behavior, not guessed
- **Maintainable** — when the test breaks, re-run in English and regenerate

## The Compound Effect: Radical Cycle Time Reduction

Replacing four tools isn't just about having fewer subscriptions. It's about what happens when demo, debug, test, and automate are **the same workflow**.

### Before: The 4-Tool Cycle

1. **PM records a Loom** showing the desired feature (10 minutes)
2. **Developer watches the Loom**, opens DevTools, starts building (context switch)
3. **Developer debugs** in DevTools, copies errors, pastes to AI, gets suggestions (context switch)
4. **Developer writes Playwright tests** for the feature (30-60 minutes)
5. **QA records a Loom** of a bug they found (10 minutes)
6. **Developer watches the Loom**, reproduces, opens DevTools again (context switch)
7. **Developer fixes and re-runs tests** (context switch)
8. **PM records another Loom** for the stakeholder demo (10 minutes)

Four tools. Six context switches. Half the time spent on ceremony instead of building.

### After: The Gasoline Cycle

1. **PM describes the feature** to the AI: "The user should be able to export the report as PDF"
2. **AI builds the feature**, debugging in real time — it sees errors as they happen, fixes them, verifies with `observe({what: "errors"})`, checks performance with `observe({what: "vitals"})`
3. **AI generates a test**: `generate({format: "test", test_name: "pdf-export"})`
4. **AI runs the demo** with subtitles for the stakeholder
5. **If QA finds a bug**, the AI already has the error context — `observe({what: "error_bundles"})` — and fixes it in the same session
6. **AI regenerates the test** if the fix changed the flow

One tool. Zero context switches. The cycle from "PM describes feature" to "tested, demo-ready feature" happens in a single conversation.

### The Math

| Activity | 4-Tool Cycle | Gasoline Cycle |
|----------|-------------|----------------|
| Feature demo (PM) | 10 min Loom recording | 0 — AI demos with subtitles |
| Debugging | 20 min (DevTools + copy-paste) | 2 min (AI observes directly) |
| Test creation | 30-60 min (Playwright) | 2 min (generate from session) |
| Bug report | 10 min Loom + reproduce | 1 min (AI already has context) |
| Bug fix verification | 5 min (re-run tests) | 30 sec (refresh + observe) |
| Stakeholder demo | 10 min (new Loom) | 1 min (replay demo script) |
| **Total** | **85-115 min** | **~7 min** |

That's not an incremental improvement. It's an order of magnitude.

### Why Cycle Time Is Everything

Product velocity isn't about how fast you type. It's about how fast you can go from "idea" to "shipped and verified." Every context switch adds latency. Every tool boundary adds friction. Every manual step adds error.

When demo, debug, test, and automate collapse into a single AI conversation:
- **Feedback loops tighten** — the AI sees the result of every change in real time
- **Iteration cost drops** — trying a different approach is a sentence, not a sprint
- **Quality increases** — tests are generated from real behavior, not written from memory
- **Everyone participates** — PMs can demo, test, and file bugs without developer involvement

This is what AI-native development looks like. Not "AI helps you write code faster" — but "AI collapses the entire build-debug-test-demo cycle into minutes."

## What's Still Coming

The one remaining advantage Loom has over Gasoline is shareability — you can send a Loom link to anyone with a browser. Gasoline's demo scripts require the AI to replay them.

The fix: **tab recording**. Chrome's `tabCapture` API can record the active tab as video while the AI runs a demo script. Subtitles and action toasts are already rendered in the page, so they'd be captured automatically. The output: a narrated demo video, generated from a replayable script, with burned-in captions. No Loom subscription. No manual recording. No re-takes.

That feature is on the roadmap. When it ships, the Loom replacement is complete.

## The Bottom Line

You don't need four tools. You need one browser extension, one MCP server, and an AI that can see your browser.

**Loom** → Gasoline subtitles + demo scripts (+ tab recording, coming soon)
**Chrome DevTools** → Gasoline observe()
**Selenium** → Gasoline interact() + natural language
**Playwright** → Gasoline generate(format: "test")

One install. Zero subscriptions. Faster than all four combined.

[Get started →](/getting-started/)
