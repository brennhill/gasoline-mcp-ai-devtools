# Gasoline v5 - Technical Specification

## Overview

v5 adds two features that transform Gasoline from "gives the AI raw data" to "gives the AI actionable understanding":

1. **AI-Preprocessed Errors** - Enrich error entries with source code snippets, component ancestry, and application state so the AI can diagnose issues without extra tool calls
2. **Reproduction Scripts** - Convert captured user actions into runnable Playwright test scripts that reproduce bugs

---

## Architecture Changes

```
┌─────────────────────────────────────────────────────────────────┐
│                         Browser                                  │
│  ┌─────────────────────────────────────────────────────────────┐│
│  │                    Inject Script (v5)                        ││
│  │                                                              ││
│  │  Error enrichment pipeline:                                  ││
│  │    error → source snippets → component tree → state snapshot ││
│  │                                                              ││
│  │  Enhanced action capture:                                    ││
│  │    click/input → robust selector → timing → buffer           ││
│  └─────────────────────────────────────────────────────────────┘│
└─────────────────────────────────────────────────────────────────┘
                     │ HTTP
                     ▼
┌─────────────────────────────────────────────────────────────────┐
│                    Gasoline Server (Go)                           │
│                                                                   │
│  New MCP Tools:                                                  │
│    get_reproduction_script - Generate Playwright test from       │
│                              captured user actions                │
│                                                                   │
│  Enhanced existing:                                              │
│    get_browser_errors - Now includes preprocessed context        │
│                                                                   │
└─────────────────────────────────────────────────────────────────┘
```

---

## Feature 1: AI-Preprocessed Errors

### Purpose

When an error occurs, the AI typically needs to:
1. Read the stack trace to find the file/line
2. Open the source file
3. Read surrounding code
4. Understand the component hierarchy
5. Check application state

This takes 3-5 tool calls. AI-preprocessed errors do all of this upfront, so the AI gets a complete picture in a single `get_browser_errors` call.

### Enrichment Pipeline

When an error is captured, the following enrichments are computed asynchronously (never blocking the main thread):

```
Error caught
    │
    ├─1. Source code snippets (from inline source maps or sourcesContent)
    │
    ├─2. Component ancestry (React fiber tree / Vue instances)
    │
    ├─3. Application state snapshot (Redux/Zustand/Pinia/Svelte stores)
    │
    └─4. Formatted AI context block
```

Each step has a timeout budget. If any step exceeds its budget, it's skipped and the error is sent with whatever enrichments succeeded.

### 1. Source Code Snippets

#### Strategy

Extract source code around the error location from source maps that include `sourcesContent`.

#### Implementation

```javascript
async function extractSourceSnippets(stack) {
  const frames = parseStackFrames(stack)
  const snippets = []

  for (const frame of frames.slice(0, 3)) { // Top 3 frames only
    const snippet = await getSourceSnippet(frame)
    if (snippet) snippets.push(snippet)
  }

  return snippets
}

async function getSourceSnippet(frame) {
  // 1. Check if we have a cached source map for this file
  const sourceMap = await fetchSourceMap(frame.filename)
  if (!sourceMap || !sourceMap.sourcesContent) return null

  // 2. Map the generated position to original position
  const original = resolveOriginalPosition(sourceMap, frame.lineno, frame.colno)
  if (!original) return null

  // 3. Extract surrounding lines (5 before, 5 after)
  const lines = original.sourceContent.split('\n')
  const start = Math.max(0, original.line - 6)
  const end = Math.min(lines.length, original.line + 5)

  return {
    file: original.source,
    line: original.line,
    column: original.column,
    snippet: lines.slice(start, end).map((text, i) => ({
      line: start + i + 1,
      text,
      isError: start + i + 1 === original.line
    }))
  }
}
```

#### Source Map Resolution

Source maps are resolved in priority order:

1. **Inline source maps** (`//# sourceMappingURL=data:...`) — parsed directly from the data URL
2. **External source maps** (`//# sourceMappingURL=foo.js.map`) — fetched via same-origin request
3. **Cache** — previously resolved source maps are cached for the session

If the source map doesn't contain `sourcesContent`, the snippet is unavailable and the frame is skipped.

#### Snippet Format

```json
{
  "_sourceSnippets": [
    {
      "file": "src/components/LoginForm.tsx",
      "line": 42,
      "column": 15,
      "snippet": [
        { "line": 37, "text": "  const handleSubmit = async (e) => {" },
        { "line": 38, "text": "    e.preventDefault()" },
        { "line": 39, "text": "    setLoading(true)" },
        { "line": 40, "text": "    try {" },
        { "line": 41, "text": "      const result = await loginApi(credentials)" },
        { "line": 42, "text": "      setUser(result.data.user)", "isError": true },
        { "line": 43, "text": "    } catch (err) {" },
        { "line": 44, "text": "      setError(err.message)" },
        { "line": 45, "text": "    } finally {" },
        { "line": 46, "text": "      setLoading(false)" },
        { "line": 47, "text": "    }" }
      ]
    }
  ]
}
```

#### Limits

| Constraint | Limit | Reason |
|-----------|-------|--------|
| Stack frames processed | 3 | Top frames are most relevant |
| Context lines | 5 before + 5 after | Enough for function context |
| Source map fetch timeout | 2s per file | Don't stall on slow/missing maps |
| Source map cache | 20 files | Bound memory usage |
| Max snippet text per line | 200 chars | Prevent minified single-line files from exploding |
| Total snippets size | 10KB | Cap the enrichment payload |

### 2. Component Ancestry

#### Strategy

When an error occurs in a UI component, walk the DOM tree from the error's origin element upward, reading framework-specific internal properties to reconstruct the component hierarchy.

#### Framework Detection

```javascript
function detectFramework(element) {
  // React 16+: __reactFiber$ or __reactInternalInstance$
  const reactKey = Object.keys(element).find(k =>
    k.startsWith('__reactFiber$') || k.startsWith('__reactInternalInstance$')
  )
  if (reactKey) return { framework: 'react', key: reactKey }

  // Vue 3: __vue_app__ on root, __vueParentComponent on components
  if (element.__vue_app__ || element.__vueParentComponent) {
    return { framework: 'vue' }
  }

  // Svelte: __svelte_meta
  if (element.__svelte_meta) {
    return { framework: 'svelte' }
  }

  return null
}
```

#### React Component Tree Extraction

```javascript
function getReactComponentAncestry(element, reactKey) {
  const fiber = element[reactKey]
  if (!fiber) return null

  const ancestry = []
  let current = fiber

  while (current && ancestry.length < 10) {
    if (typeof current.type === 'function' || typeof current.type === 'object') {
      const name = current.type.displayName || current.type.name || 'Anonymous'
      const entry = { name }

      // Extract props (top-level keys only, no values for privacy)
      if (current.memoizedProps) {
        entry.propKeys = Object.keys(current.memoizedProps).filter(k => k !== 'children')
      }

      // Extract state summary (for class components or useState)
      if (current.memoizedState && typeof current.memoizedState === 'object') {
        entry.hasState = true
        entry.stateKeys = safeGetKeys(current.memoizedState)
      }

      ancestry.push(entry)
    }
    current = current.return
  }

  return ancestry.reverse() // Root → leaf order
}
```

#### Ancestry Format

```json
{
  "_componentAncestry": {
    "framework": "react",
    "components": [
      { "name": "App", "propKeys": ["theme", "locale"] },
      { "name": "AuthProvider", "propKeys": ["children"], "hasState": true },
      { "name": "LoginPage", "propKeys": ["onSuccess", "redirectUrl"] },
      { "name": "LoginForm", "propKeys": ["initialEmail"], "hasState": true, "stateKeys": ["email", "password", "loading", "error"] }
    ]
  }
}
```

#### Finding the Error Origin Element

To find which DOM element is associated with the error:

1. **Exception with component stack** (React) — parse the component stack to identify the component
2. **Error boundary** — if the error was caught by an error boundary, use that element
3. **Active element** — fallback to `document.activeElement` at time of error
4. **Event target** — if error occurred during an event handler, use the event target from the action buffer

#### Limits

| Constraint | Limit | Reason |
|-----------|-------|--------|
| Ancestry depth | 10 components | Sufficient for context |
| Prop keys per component | 20 | Prevent huge prop lists |
| State keys per component | 10 | Summary only |
| Framework detection timeout | 50ms | Don't stall on edge cases |
| Total ancestry size | 5KB | Cap the enrichment payload |

### 3. Application State Snapshot

#### Strategy

Capture a summary of application state from known state management patterns. This is deliberately shallow — keys and types only, not full values (which could be huge and contain sensitive data).

#### Supported Patterns

| Pattern | Detection | Extraction |
|---------|-----------|------------|
| Redux | `window.__REDUX_DEVTOOLS_EXTENSION__` or store on window | `store.getState()` top-level keys + types |
| Zustand | Stores registered via devtools | `store.getState()` top-level keys + types |
| Pinia | `window.__pinia` | `store.$state` top-level keys + types |
| React Context | Via fiber tree (already captured in ancestry) | State keys from providers |
| Svelte stores | `window.__svelte_stores` (if exposed) | Store value summaries |

#### State Format

```json
{
  "_stateSnapshot": {
    "source": "redux",
    "keys": {
      "auth": { "type": "object", "keys": ["user", "token", "loading", "error"] },
      "cart": { "type": "object", "keys": ["items", "total", "discount"] },
      "ui": { "type": "object", "keys": ["sidebarOpen", "theme", "locale"] }
    },
    "relevantSlice": {
      "auth.loading": false,
      "auth.error": "Request failed with status 401",
      "auth.user": null
    }
  }
}
```

#### Relevant State Extraction

Not all state is relevant to every error. The relevant slice is determined by:

1. **Error message keywords** — if the error mentions "auth", include `state.auth`
2. **Error-related state keys** — keys containing `error`, `loading`, `status`, `failed`
3. **Recently changed state** — if the state management library supports it, include keys that changed in the last 2 seconds

#### Limits

| Constraint | Limit | Reason |
|-----------|-------|--------|
| State tree depth | 2 levels | Top-level structure only |
| Relevant slice values | 10 entries | Most relevant state |
| Value truncation | 200 chars per value | Cap payload size |
| State snapshot timeout | 100ms | Don't stall on large stores |
| Total state size | 5KB | Cap the enrichment payload |
| Store detection | Window-accessible only | Can't hook into module-scoped stores |

### 4. Formatted AI Context Block

All enrichments are combined into a single `_aiContext` field designed for direct LLM consumption:

```json
{
  "_aiContext": {
    "summary": "TypeError in LoginForm.tsx:42 — `result.data` is undefined when calling `setUser(result.data.user)`. The auth state shows loading=false, error='Request failed with status 401', user=null. This is inside a React component tree: App > AuthProvider > LoginPage > LoginForm.",
    "sourceSnippets": [...],
    "componentAncestry": {...},
    "stateSnapshot": {...}
  }
}
```

The `summary` field is a plain-text sentence constructed from the available data. It is NOT generated by an LLM — it's a template-based string built from the structured data:

```
Template: "{ErrorType} in {file}:{line} — `{errorExpression}` {errorDescription}.
  {stateContext}. {componentContext}."
```

### Full Enriched Error Example

```json
{
  "ts": "2024-01-22T10:30:00.000Z",
  "type": "exception",
  "level": "error",
  "message": "Cannot read properties of undefined (reading 'user')",
  "stack": "TypeError: Cannot read properties of undefined (reading 'user')\n    at handleSubmit (http://localhost:3000/static/js/main.abc123.js:42:15)",
  "filename": "http://localhost:3000/static/js/main.abc123.js",
  "lineno": 42,
  "url": "http://localhost:3000/login",
  "_enrichments": ["context", "userActions", "sourceMap", "aiContext"],
  "_aiContext": {
    "summary": "TypeError in src/components/LoginForm.tsx:42 — `result.data.user` is undefined. Auth state: loading=false, error='Request failed with status 401', user=null. Component tree: App > AuthProvider > LoginPage > LoginForm.",
    "sourceSnippets": [
      {
        "file": "src/components/LoginForm.tsx",
        "line": 42,
        "column": 15,
        "snippet": [
          { "line": 39, "text": "    setLoading(true)" },
          { "line": 40, "text": "    try {" },
          { "line": 41, "text": "      const result = await loginApi(credentials)" },
          { "line": 42, "text": "      setUser(result.data.user)", "isError": true },
          { "line": 43, "text": "    } catch (err) {" },
          { "line": 44, "text": "      setError(err.message)" },
          { "line": 45, "text": "    } finally {" }
        ]
      }
    ],
    "componentAncestry": {
      "framework": "react",
      "components": [
        { "name": "App", "propKeys": ["theme"] },
        { "name": "AuthProvider", "hasState": true },
        { "name": "LoginPage", "propKeys": ["onSuccess"] },
        { "name": "LoginForm", "propKeys": ["initialEmail"], "hasState": true, "stateKeys": ["email", "password", "loading", "error"] }
      ]
    },
    "stateSnapshot": {
      "source": "redux",
      "keys": {
        "auth": { "type": "object", "keys": ["user", "token", "loading", "error"] },
        "cart": { "type": "object", "keys": ["items", "total"] }
      },
      "relevantSlice": {
        "auth.loading": false,
        "auth.error": "Request failed with status 401",
        "auth.user": null
      }
    }
  },
  "_actions": [
    { "ts": "2024-01-22T10:29:55.000Z", "type": "click", "target": "button#submit", "text": "Login" },
    { "ts": "2024-01-22T10:29:53.000Z", "type": "input", "target": "input#password", "inputType": "password", "value": "[redacted]" }
  ]
}
```

### Performance Budget

The entire enrichment pipeline runs asynchronously after the error is captured and queued. It MUST NOT delay the error entry from being sent — the base error is sent immediately, and the AI context is appended before the next batch flush.

| Step | Budget | Fallback |
|------|--------|----------|
| Source map fetch + parse | 2s total | Skip snippets |
| Source snippet extraction | 50ms | Skip remaining frames |
| Component ancestry walk | 50ms | Truncate at current depth |
| State snapshot | 100ms | Skip state |
| Summary generation | 5ms | Skip summary |
| Total pipeline | 3s | Send whatever completed |

### Extension Settings

New toggle in popup: **"AI Error Context"** (default: ON).

When enabled, errors are enriched with source snippets, component ancestry, and state snapshots. This adds processing time after each error but does not affect page performance (all work is async).

Sub-toggles (in Options page):
- **Source code snippets** (default: ON) — requires source maps with sourcesContent
- **Component ancestry** (default: ON) — React, Vue, or Svelte detection
- **State snapshot** (default: OFF) — opt-in due to potential sensitivity

#### Storage Keys

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `aiContextEnabled` | boolean | true | Master toggle |
| `aiContextSourceSnippets` | boolean | true | Include source code |
| `aiContextComponentTree` | boolean | true | Include component ancestry |
| `aiContextStateSnapshot` | boolean | false | Include state snapshot |

---

## Feature 2: Reproduction Scripts

### Purpose

When a user encounters a bug, they've already performed the steps to reproduce it. Gasoline captures those steps as user actions. This feature converts that action buffer into a runnable Playwright test that reproduces the exact sequence of interactions.

The AI can then:
1. Run the test to confirm the bug
2. Make code changes
3. Run the test again to verify the fix

### Enhanced Action Capture

The existing action capture records clicks and inputs with basic target selectors. v5 enhances this with:

1. **Robust selectors** — multiple selector strategies per element
2. **Navigation tracking** — URL changes between actions
3. **Scroll positions** — viewport state at each action
4. **Form submissions** — form submit events
5. **Keyboard events** — key presses (Enter, Escape, Tab)
6. **Wait indicators** — loading states, network activity

#### Selector Strategy

For each captured action, compute multiple selectors in priority order:

```javascript
function computeSelectors(element) {
  const selectors = {}

  // Priority 1: Test ID (most stable)
  const testId = element.getAttribute('data-testid') ||
                 element.getAttribute('data-test-id') ||
                 element.getAttribute('data-cy')
  if (testId) selectors.testId = testId

  // Priority 2: ARIA label (accessibility-friendly)
  const ariaLabel = element.getAttribute('aria-label')
  if (ariaLabel) selectors.ariaLabel = ariaLabel

  // Priority 3: Role + accessible name
  const role = element.getAttribute('role') || getImplicitRole(element)
  const name = getAccessibleName(element)
  if (role && name) selectors.role = { role, name }

  // Priority 4: Unique ID
  if (element.id && isUniqueId(element.id)) {
    selectors.id = element.id
  }

  // Priority 5: Text content (for buttons, links)
  const text = getVisibleText(element)
  if (text && text.length <= 50 && isClickableElement(element)) {
    selectors.text = text
  }

  // Priority 6: CSS path (last resort, most brittle)
  selectors.cssPath = computeCssPath(element)

  return selectors
}
```

#### Implicit Role Mapping

```javascript
function getImplicitRole(element) {
  const tag = element.tagName.toLowerCase()
  const type = element.getAttribute('type')

  const roleMap = {
    'button': 'button',
    'a[href]': 'link',
    'input[type=checkbox]': 'checkbox',
    'input[type=radio]': 'radio',
    'input[type=text]': 'textbox',
    'input[type=email]': 'textbox',
    'input[type=password]': 'textbox',
    'input[type=search]': 'searchbox',
    'input[type=number]': 'spinbutton',
    'textarea': 'textbox',
    'select': 'combobox',
    'img': 'img',
    'nav': 'navigation',
    'main': 'main',
    'header': 'banner',
    'footer': 'contentinfo',
  }

  if (tag === 'input') return roleMap[`input[type=${type || 'text'}]`] || 'textbox'
  if (tag === 'a' && element.hasAttribute('href')) return 'link'
  return roleMap[tag] || null
}
```

#### CSS Path Computation

```javascript
function computeCssPath(element) {
  const parts = []
  let current = element

  while (current && current !== document.body && parts.length < 5) {
    let selector = current.tagName.toLowerCase()

    if (current.id && isUniqueId(current.id)) {
      selector = `#${CSS.escape(current.id)}`
      parts.unshift(selector)
      break
    }

    // Add distinguishing class if helpful
    const classes = Array.from(current.classList)
      .filter(c => !isDynamicClass(c)) // Skip generated classes (css-xxx, sc-xxx)
      .slice(0, 2)
    if (classes.length) {
      selector += '.' + classes.map(c => CSS.escape(c)).join('.')
    }

    // Add nth-child if needed for uniqueness
    if (needsNthChild(current)) {
      const idx = getChildIndex(current)
      selector += `:nth-child(${idx})`
    }

    parts.unshift(selector)
    current = current.parentElement
  }

  return parts.join(' > ')
}

function isDynamicClass(className) {
  // Skip CSS-in-JS generated classes
  return /^(css|sc|emotion|styled|chakra)-/.test(className) ||
         /^[a-z]{5,8}$/.test(className) // Random hash classes
}
```

#### Enhanced Action Entry Format

```json
{
  "ts": "2024-01-22T10:29:55.000Z",
  "type": "click",
  "selectors": {
    "testId": "login-button",
    "ariaLabel": "Log in to your account",
    "role": { "role": "button", "name": "Log in" },
    "text": "Log in",
    "cssPath": "form#login > button.submit-btn"
  },
  "position": { "x": 450, "y": 320 },
  "url": "http://localhost:3000/login",
  "timestamp": 1705921795000
}
```

```json
{
  "ts": "2024-01-22T10:29:53.000Z",
  "type": "input",
  "selectors": {
    "testId": "email-input",
    "ariaLabel": "Email address",
    "role": { "role": "textbox", "name": "Email address" },
    "id": "email",
    "cssPath": "form#login > input#email"
  },
  "inputType": "email",
  "value": "user@example.com",
  "url": "http://localhost:3000/login",
  "timestamp": 1705921793000
}
```

#### New Action Types

| Type | Trigger | Captured Data |
|------|---------|---------------|
| `click` | click event | selectors, position, text |
| `input` | input event | selectors, value (redacted if sensitive), inputType |
| `submit` | submit event | selectors, form action, method |
| `keypress` | keydown (Enter/Escape/Tab) | selectors, key |
| `navigate` | popstate / pushState | fromUrl, toUrl |
| `scroll` | scroll (throttled 500ms) | scrollX, scrollY, target |
| `select` | change on `<select>` | selectors, selectedValue, selectedText |

#### Action Buffer Changes

| Parameter | v3 | v5 |
|-----------|-----|-----|
| Buffer size | 20 | 50 |
| Selector strategy | Basic `tagName#id.class` | Multi-strategy (testId, aria, role, css path) |
| Navigation tracking | No | Yes (URL changes) |
| Keyboard events | No | Yes (Enter, Escape, Tab) |
| Form submissions | No | Yes |
| Scroll tracking | Yes (throttled) | Yes (throttled, includes target) |

### MCP Tool: `get_reproduction_script`

**Description**: Generate a Playwright test script from the captured user action buffer. Returns a complete, runnable test file.

**Parameters**:

| Param | Type | Default | Description |
|-------|------|---------|-------------|
| `format` | string | `"playwright"` | Output format: `"playwright"` or `"cypress"` |
| `include_assertions` | boolean | true | Include assertions for navigation and visible text |
| `base_url` | string | null | Base URL override (for dynamic ports) |
| `last_n_actions` | number | null | Only use last N actions (null = all) |

**Response**:

```json
{
  "script": "import { test, expect } from '@playwright/test';\n\ntest('reproduction: Cannot read properties of undefined', async ({ page }) => {\n  await page.goto('http://localhost:3000/login');\n\n  // Fill email field\n  await page.getByTestId('email-input').fill('user@example.com');\n\n  // Fill password field\n  await page.getByTestId('password-input').fill('[user-provided]');\n\n  // Click login button\n  await page.getByRole('button', { name: 'Log in' }).click();\n\n  // Error occurred here: Cannot read properties of undefined (reading 'user')\n  // The page should handle this error gracefully\n});\n",
  "actions_used": 3,
  "error_context": {
    "message": "Cannot read properties of undefined (reading 'user')",
    "file": "src/components/LoginForm.tsx",
    "line": 42
  },
  "selectors_used": ["testId", "role"],
  "warnings": ["Password field value redacted — replace '[user-provided]' with test credentials"]
}
```

### Script Generation

#### Playwright Selector Priority

When generating Playwright locators, use the best available selector:

| Priority | Available Data | Generated Locator |
|----------|---------------|-------------------|
| 1 | `testId` | `page.getByTestId('login-button')` |
| 2 | `role` + `name` | `page.getByRole('button', { name: 'Log in' })` |
| 3 | `ariaLabel` | `page.getByLabel('Email address')` |
| 4 | `text` (clickable) | `page.getByText('Submit')` |
| 5 | `id` | `page.locator('#login-button')` |
| 6 | `cssPath` | `page.locator('form#login > button.submit-btn')` |

#### Action-to-Code Mapping

| Action Type | Playwright Code |
|-------------|----------------|
| `click` | `await page.getByTestId('x').click();` |
| `input` (text) | `await page.getByTestId('x').fill('value');` |
| `input` (sensitive) | `await page.getByTestId('x').fill('[user-provided]');` |
| `submit` | `await page.getByTestId('x').click();` (submit button) |
| `keypress` (Enter) | `await page.keyboard.press('Enter');` |
| `keypress` (Escape) | `await page.keyboard.press('Escape');` |
| `navigate` | `await expect(page).toHaveURL('...');` (assertion) |
| `select` | `await page.getByTestId('x').selectOption('value');` |
| `scroll` | Comment only: `// User scrolled to y=320` |

#### Timing and Waits

Between actions, the script inserts appropriate waits:

| Condition | Wait Strategy |
|-----------|---------------|
| Time gap > 2s between actions | `// [2.3s pause]` comment |
| Navigation occurred | `await page.waitForURL('...');` |
| Input followed by click | No extra wait (natural Playwright sequencing) |
| Multiple rapid clicks | `await page.getByTestId('x').click();` (sequential, no delay) |

#### Sensitive Data Handling

| Data Type | Handling |
|-----------|----------|
| Password inputs | Replace with `'[user-provided]'` + warning |
| Redacted inputs | Replace with `'[user-provided]'` + warning |
| Auth tokens in URLs | Strip token params, add warning |
| PII in text fields | Pass through (user's choice what to enter in test) |

### Server Implementation

The script generation happens entirely on the server (Go). The server:

1. Receives the action buffer from the extension (already enhanced with selectors)
2. When `get_reproduction_script` MCP tool is called:
   a. Takes the buffered actions
   b. Selects the best selector for each action
   c. Maps actions to Playwright/Cypress code
   d. Inserts navigation assertions and waits
   e. Wraps in a test function
   f. Returns the complete script

#### Template Structure

```go
const playwrightTemplate = `import { test, expect } from '@playwright/test';

test('reproduction: {{.ErrorMessage}}', async ({ page }) => {
  await page.goto('{{.StartURL}}');
{{range .Steps}}
  {{.Comment}}
  {{.Code}}
{{end}}
  // Error occurred here: {{.ErrorMessage}}
});
`
```

### Extension Settings

Enhanced action capture is controlled by the existing action capture toggle. No new toggle needed — the enhanced selectors are always computed when action capture is enabled (the additional computation is < 1ms per action).

#### Storage Keys

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `enhancedSelectorsEnabled` | boolean | true | Compute multi-strategy selectors |
| `reproductionBufferSize` | number | 50 | Number of actions to buffer |

### Limits

| Constraint | Limit | Reason |
|-----------|-------|--------|
| Action buffer size | 50 entries | More history for reproduction |
| Selectors per action | 6 strategies | All computed, best chosen at generation time |
| CSS path depth | 5 levels | Prevent overly specific selectors |
| Dynamic class filtering | Regex-based | Skip CSS-in-JS generated classes |
| Text content for selectors | 50 chars | Prevent long text selectors |
| Script generation time | < 50ms | Simple template expansion |
| Output script size | < 50KB | Cap for very long action sequences |

### Performance Budget

| Operation | Budget | Notes |
|-----------|--------|-------|
| Selector computation per action | < 1ms | Runs on click/input handler |
| CSS path computation | < 0.5ms | Limited to 5 levels |
| `getImplicitRole` | < 0.1ms | Simple lookup table |
| `isDynamicClass` | < 0.1ms | Regex test |
| Full script generation (server) | < 50ms | Template expansion + iteration |

---

## Shared Concerns

### Privacy & Security

- **Source code snippets**: Only available if source maps include `sourcesContent`. Production builds typically strip this. Development builds (where Gasoline is most useful) include it.
- **Component ancestry**: Only prop/state *keys* are captured, never values (unless they appear in the relevant state slice).
- **State snapshot**: OFF by default. When enabled, only error-related state values are included, and large values are truncated.
- **Reproduction scripts**: Sensitive inputs (passwords, tokens) are automatically redacted. A warning is included in the output.
- **All data stays local**: Nothing leaves localhost. The enrichments are only available via the MCP tools.

### Backward Compatibility

- The `_aiContext` field is additive — existing `get_browser_errors` responses gain a new field. No breaking changes.
- Enhanced action entries are backward-compatible — they add `selectors` alongside the existing `target` field.
- The `get_reproduction_script` tool is new and doesn't affect existing tools.
- If AI context is disabled, error entries look exactly like v4.

### New MCP Tools Summary

| Tool | Type | Description |
|------|------|-------------|
| `get_reproduction_script` | On-demand | Generate Playwright/Cypress test from captured actions |

### Enhanced Existing Tools

| Tool | Enhancement |
|------|-------------|
| `get_browser_errors` | Error entries now include `_aiContext` field with source snippets, component ancestry, and state |

### Extension Manifest Changes

No new permissions required. Source maps are fetched via standard `fetch()` from the page's origin. Component ancestry reads DOM properties already accessible to the inject script.

---

## Testing Requirements

### AI-Preprocessed Errors

| Test Category | Cases |
|--------------|-------|
| Source map parsing | Inline data URLs, external .map files, missing sourcesContent |
| Snippet extraction | Normal case, minified files, line out of range, multi-frame |
| Component ancestry (React) | Fiber tree walk, displayName, anonymous components, depth limit |
| Component ancestry (Vue) | Vue 3 instances, composition API, options API |
| State snapshot | Redux store, Zustand store, no store present |
| Relevant state | Keyword matching, error-related keys, empty state |
| Summary generation | Template with all data, partial data, minimal data |
| Performance | Pipeline within budget, individual step timeouts |
| Settings | Toggle on/off, sub-toggles, persistence |

### Reproduction Scripts

| Test Category | Cases |
|--------------|-------|
| Selector computation | testId, aria, role, id, text, cssPath, priority order |
| CSS path | Simple path, dynamic classes filtered, nth-child, ID shortcut |
| Implicit roles | All mapped elements, unknown elements, type-specific inputs |
| Dynamic class detection | CSS-in-JS patterns, legitimate classes |
| Action-to-code mapping | click, input, submit, keypress, navigate, select |
| Playwright selectors | Priority order, fallback chain |
| Timing/waits | Navigation waits, long pauses, rapid sequences |
| Sensitive data | Password redaction, token stripping, warnings |
| Script generation | Full template, partial actions, error context |
| Server endpoint | MCP tool params, format selection, base_url override |
| Edge cases | No actions, single action, 50 actions, no selectors available |
