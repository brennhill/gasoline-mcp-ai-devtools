---
doc_type: legacy_doc
status: reference
last_reviewed: 2026-02-16
---

# Recording Web Demos with Kaboom

**Duration:** 2-3 minutes
**Focus:** How to record browser interactions with Kaboom, then use those recordings as data for testing, debugging, and creating reproducible scenarios.

## What This Demo Shows

You're building a test, tutorial, or bug report. Instead of manually writing code, you interact with a website naturally. Kaboom records everything: network calls, user actions, and DOM changes. It then transforms your recording into:
- Playwright test code
- Reproduction scripts
- Performance profiles
- Security findings

This is recording for *engineers*, not just video.

---

## The Setup

**Page:** `https://gokaboom.dev/demo/recipe-search`

This page demonstrates:
- Form interactions (search input)
- Network requests (recipe API calls)
- Real-time results rendering
- Error handling (e.g., no recipes found)

---

## The Kaboom Script (Natural Language)

```
Navigate to: https://gokaboom.dev/demo/recipe-search
[3s pause]
Click: searchbox
Type "pasta" into: searchbox
[4s pause]
Click: "Carbonara (Classic Roman Style)" recipe card
[4s pause]
Take screenshot showing the recipe details
Show network timeline
Show performance metrics
Generate Playwright test from this recording
```

**What happens:**
- Navigate to recipe search page
- Wait 3 seconds (page loads)
- Click the search box and type "pasta"
- Wait 4 seconds (results load)
- Click the first recipe card
- Wait 4 seconds (detail page loads)
- Capture screenshots, network data, performance metrics
- Generate test code automatically

---

## Audio Narration

```
"Let's record a real user workflow with Kaboom.

We're on a recipe browsing site. I search for pasta, just like a normal user would.
Kaboom captures this silently: every click, every keystroke, every network request.

[As results load]
Notice the subtitle? That's Kaboom's subtitle feature in action, also recorded.

I click the first recipe. The page loads. Kaboom is watching everything.

Now here's the magic: we ask Claude to generate a Playwright test from this recording.
No manual coding. The test captures exactly what we did.

[Results appear]
Kaboom also extracted the network timeline: three API calls in 150 milliseconds.
Performance metrics: first contentful paint at 0.8 seconds, largest contentful paint at 1.2.

This is what makes Kaboom powerful: recording becomes *data* for testing and debugging."
```

---

## Key Features Highlighted

✅ **Recording** — Kaboom captures user actions + browser telemetry in one pass
✅ **Subtitles** — Real-time narration of what's happening (also a Kaboom feature)
✅ **Test Generation** — `generate test` creates Playwright code automatically
✅ **Telemetry Extraction** — Network timings, performance metrics, all captured
✅ **Reproducibility** — Tests run the exact same interactions, every time

---

## What the Generated Test Looks Like

```typescript
test('recipe_search_workflow', async ({ page }) => {
  await page.goto('https://gokaboom.dev/demo/recipe-search');
  await page.waitForSelector('[role=searchbox]');

  const searchbox = page.locator('[role=searchbox]');
  await searchbox.click();
  await searchbox.type('pasta');

  await page.waitForSelector('.recipe-card');
  await page.locator('.recipe-card').first().click();

  await page.waitForSelector('[data-testid=recipe-detail]');
  expect(page.locator('[data-testid=recipe-detail]')).toBeVisible();
});
```

---

## Why This Matters

- **No manual test writing** — Record once, generate tests instantly
- **Real user flows** — Tests capture actual workflows, not simplified versions
- **Built-in debugging** — Network + performance data comes with every recording
- **Reproducible bugs** — Users can record errors and send you the exact scenario
