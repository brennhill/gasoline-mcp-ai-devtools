# DOM Fingerprinting (`get_dom_fingerprint`, `compare_dom_fingerprint`)

## Status: Specification

---

## Justification

### The Problem

After an AI agent edits code, the browser re-renders. The agent needs to verify: "does the page still look right?" Current options:

1. **Playwright MCP screenshots** — Expensive (vision model inference per check), slow (500ms-2s per screenshot analysis), and imprecise ("something looks wrong" vs. "the submit button is missing")
2. **`query_dom` with specific selectors** — Requires knowing WHAT to check; misses unexpected disappearances
3. **`get_page_info`** — Too coarse (counts of elements, not structure)
4. **Full DOM dump** — Too large (10-50K tokens for a real page), mostly noise

### The Insight

Developers don't look at every pixel — they scan for structural landmarks: "is the header there? does the form have all its fields? is there an error banner?" A semantic fingerprint captures this scan as structured data: page regions, interactive elements, content headings, error indicators.

### Why This is AI-Critical

- **Vision models are expensive:** Anthropic charges per image token. At 50+ checks per session, screenshot-based verification costs $1-5/session in API fees alone.
- **Text is faster than images:** A 500-byte JSON fingerprint processes in <1ms. A 1MB screenshot takes 1-2s of vision model inference.
- **Structure > appearance:** The agent doesn't care about CSS styling or pixel alignment — it cares about whether the button exists, the form has fields, and the error message is gone.
- **Diffable:** Text fingerprints can be compared structurally ("button 'Save' disappeared") rather than visually ("there's a gap in the middle of the page").

### The Tradeoff

DOM fingerprinting does NOT replace visual testing. It catches:
- Missing/added elements
- Structural changes (form lost a field, list became empty)
- Error state changes (error banner appeared/disappeared)
- Navigation changes (wrong page)

It does NOT catch:
- CSS layout bugs (element exists but is overlapped/clipped)
- Color/styling regressions
- Responsive breakpoint issues
- Animation/transition bugs

For those, Playwright's `toHaveScreenshot()` in generated tests (CI-time) is the right tool. DOM fingerprinting is the fast, cheap, dev-time structural check.

---

## MCP Tool Interface

### Tool: `get_dom_fingerprint`

Returns a semantic fingerprint of the current page structure.

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `scope` | string | No | `"full"` | `"full"` (entire page), `"above_fold"` (visible viewport), or CSS selector |
| `depth` | string | No | `"standard"` | `"minimal"` (landmarks only), `"standard"` (interactive + content), `"detailed"` (all significant elements) |
| `baseline_name` | string | No | None | If provided, automatically compare against saved baseline |

### Response

```json
{
  "url": "http://localhost:3000/dashboard",
  "title": "Dashboard - MyApp",
  "viewport": {"width": 1280, "height": 720},
  "captured_at": "2026-01-23T10:30:45Z",

  "structure": {
    "landmarks": {
      "header": {
        "present": true,
        "contains": ["nav", "logo", "user-menu"],
        "interactive": ["link:Home", "link:Projects", "button:Logout"]
      },
      "main": {
        "present": true,
        "contains": ["heading:Dashboard", "section:projects", "section:activity"],
        "role": "main"
      },
      "footer": {
        "present": true,
        "contains": ["link:Privacy", "link:Terms"]
      }
    },

    "content": {
      "headings": [
        {"level": 1, "text": "Dashboard"},
        {"level": 2, "text": "Recent Projects"},
        {"level": 2, "text": "Activity Feed"}
      ],
      "lists": [
        {"selector": ".project-list", "items": 5, "type": "ul"},
        {"selector": ".activity-feed", "items": 12, "type": "ol"}
      ],
      "forms": [
        {"selector": "#search-form", "fields": ["input:Search projects"], "buttons": ["button:Search"]}
      ],
      "tables": [],
      "images": {"count": 3, "with_alt": 3, "broken": 0}
    },

    "interactive": [
      {"type": "button", "text": "New Project", "visible": true, "enabled": true},
      {"type": "button", "text": "Logout", "visible": true, "enabled": true},
      {"type": "input", "label": "Search projects", "visible": true, "value": ""},
      {"type": "link", "text": "View All Projects", "href": "/projects"}
    ],

    "state": {
      "error_elements": [],
      "loading_indicators": [],
      "empty_states": [],
      "modals_open": [],
      "notifications": []
    }
  },

  "hash": "a7f3b2c1",
  "token_count": 412
}
```

### Tool: `compare_dom_fingerprint`

Compares current DOM against a previously captured fingerprint or baseline.

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `against` | string | Yes | — | Baseline name or inline fingerprint hash |
| `severity_threshold` | string | No | `"warning"` | Minimum severity to report: `"info"`, `"warning"`, `"error"` |

### Comparison Response

```json
{
  "status": "changed",
  "severity": "error",

  "changes": [
    {
      "type": "element_missing",
      "severity": "error",
      "element": {"type": "button", "text": "New Project"},
      "location": "main > section:projects",
      "description": "Interactive button 'New Project' no longer present"
    },
    {
      "type": "error_appeared",
      "severity": "error",
      "element": {"selector": ".error-banner", "text": "Failed to load projects"},
      "description": "Error element appeared that was not in baseline"
    },
    {
      "type": "list_empty",
      "severity": "warning",
      "element": {"selector": ".project-list"},
      "was": {"items": 5},
      "now": {"items": 0},
      "description": "Project list is now empty (was 5 items)"
    },
    {
      "type": "element_added",
      "severity": "info",
      "element": {"type": "button", "text": "Import Projects"},
      "description": "New interactive element appeared"
    }
  ],

  "unchanged": ["header", "footer", "forms", "navigation"],
  "summary": "2 errors, 1 warning, 1 info change detected"
}
```

---

## Implementation

### Fingerprint Extraction (Extension-Side)

The fingerprint is extracted via the existing pending query mechanism. A specialized query type captures semantic structure rather than raw DOM:

```javascript
// In inject.js — responds to GASOLINE_DOM_FINGERPRINT query
function extractDOMFingerprint(scope, depth) {
  const root = scope === 'full' ? document.body :
               scope === 'above_fold' ? getVisibleElements() :
               document.querySelector(scope);

  return {
    landmarks: extractLandmarks(root),
    content: extractContent(root, depth),
    interactive: extractInteractive(root),
    state: extractState(root)
  };
}

function extractLandmarks(root) {
  const landmarks = {};
  const roles = ['banner', 'main', 'navigation', 'contentinfo', 'complementary'];
  const tags = {header: 'header', main: 'main', nav: 'navigation', footer: 'footer'};

  // Find by ARIA role or semantic HTML tag
  for (const [tag, role] of Object.entries(tags)) {
    const el = root.querySelector(tag) || root.querySelector(`[role="${role}"]`);
    if (el) {
      landmarks[tag] = {
        present: true,
        contains: summarizeChildren(el),
        interactive: getInteractiveChildren(el)
      };
    }
  }
  return landmarks;
}

function extractInteractive(root) {
  const elements = [];
  const selectors = 'button, a[href], input, select, textarea, [role="button"], [tabindex="0"]';

  for (const el of root.querySelectorAll(selectors)) {
    if (!isVisible(el)) continue;
    elements.push({
      type: el.tagName.toLowerCase() === 'a' ? 'link' : el.tagName.toLowerCase(),
      text: getAccessibleName(el),
      visible: true,
      enabled: !el.disabled,
      ...(el.href && {href: new URL(el.href).pathname}),
      ...(el.type === 'input' && {value: el.value ? '[has value]' : ''})
    });
  }
  return elements;
}

function extractState(root) {
  return {
    error_elements: findElements(root, '[role="alert"], .error, .alert-error, .alert-danger'),
    loading_indicators: findElements(root, '[aria-busy="true"], .loading, .spinner, .skeleton'),
    empty_states: findElements(root, '.empty-state, [data-empty], .no-results'),
    modals_open: findElements(root, '[role="dialog"][aria-hidden!="true"], .modal.show'),
    notifications: findElements(root, '[role="status"], .toast, .notification')
  };
}
```

### Performance Budget (Extension-Side)

| Operation | Budget | Rationale |
|-----------|--------|-----------|
| Fingerprint extraction | < 30ms main thread | Page may be interactive; cannot block |
| Element traversal | < 500 elements inspected | Cap at significant elements only |
| Result serialization | < 5ms | JSON.stringify of structured result |
| Total round-trip (server) | < 200ms | Including pending query mechanism |

### Optimization: Incremental Fingerprinting

For frequent checks (agent checking after every edit), avoid full re-extraction:

1. **Hash-based short-circuit:** Compute a fast hash of `document.body.innerHTML.length + querySelectorAll('*').length`. If unchanged from last fingerprint, return cached result.
2. **MutationObserver tracking:** Track which subtrees changed since last extraction. Only re-extract changed subtrees.
3. **Debounced extraction:** If multiple fingerprint requests arrive within 100ms, batch them.

### Server-Side Types

```go
type DOMFingerprint struct {
    URL        string              `json:"url"`
    Title      string              `json:"title"`
    Viewport   Viewport            `json:"viewport"`
    CapturedAt time.Time           `json:"captured_at"`
    Structure  FingerprintStructure `json:"structure"`
    Hash       string              `json:"hash"`
    TokenCount int                 `json:"token_count"`
}

type FingerprintStructure struct {
    Landmarks   map[string]Landmark   `json:"landmarks"`
    Content     ContentSummary        `json:"content"`
    Interactive []InteractiveElement  `json:"interactive"`
    State       PageState             `json:"state"`
}

type Landmark struct {
    Present     bool     `json:"present"`
    Contains    []string `json:"contains"`
    Interactive []string `json:"interactive,omitempty"`
    Role        string   `json:"role,omitempty"`
}

type InteractiveElement struct {
    Type    string `json:"type"`
    Text    string `json:"text"`
    Visible bool   `json:"visible"`
    Enabled bool   `json:"enabled"`
    Href    string `json:"href,omitempty"`
    Value   string `json:"value,omitempty"`
}

type PageState struct {
    ErrorElements     []StateElement `json:"error_elements"`
    LoadingIndicators []StateElement `json:"loading_indicators"`
    EmptyStates       []StateElement `json:"empty_states"`
    ModalsOpen        []StateElement `json:"modals_open"`
    Notifications     []StateElement `json:"notifications"`
}
```

### Comparison Algorithm

```go
func compareFingerprints(baseline, current DOMFingerprint) ComparisonResult {
    changes := []FingerprintChange{}

    // 1. Check landmarks
    for name, baselineLandmark := range baseline.Structure.Landmarks {
        currentLandmark, exists := current.Structure.Landmarks[name]
        if !exists || !currentLandmark.Present {
            changes = append(changes, FingerprintChange{
                Type: "landmark_missing", Severity: "error",
                Description: fmt.Sprintf("Landmark '%s' is no longer present", name),
            })
        }
    }

    // 2. Check interactive elements
    baselineInteractive := indexByTextAndType(baseline.Structure.Interactive)
    currentInteractive := indexByTextAndType(current.Structure.Interactive)

    for key, baseEl := range baselineInteractive {
        if _, exists := currentInteractive[key]; !exists {
            changes = append(changes, FingerprintChange{
                Type: "element_missing", Severity: "error",
                Element: baseEl,
                Description: fmt.Sprintf("Interactive %s '%s' no longer present", baseEl.Type, baseEl.Text),
            })
        }
    }

    // 3. Check error state
    if len(baseline.Structure.State.ErrorElements) == 0 && len(current.Structure.State.ErrorElements) > 0 {
        for _, err := range current.Structure.State.ErrorElements {
            changes = append(changes, FingerprintChange{
                Type: "error_appeared", Severity: "error",
                Element: err,
                Description: fmt.Sprintf("Error element appeared: %s", err.Text),
            })
        }
    }

    // 4. Check content (lists, forms)
    for i, baseList := range baseline.Structure.Content.Lists {
        if i < len(current.Structure.Content.Lists) {
            curList := current.Structure.Content.Lists[i]
            if curList.Items == 0 && baseList.Items > 0 {
                changes = append(changes, FingerprintChange{
                    Type: "list_empty", Severity: "warning",
                    Description: fmt.Sprintf("List '%s' is now empty (was %d items)", baseList.Selector, baseList.Items),
                })
            }
        }
    }

    return ComparisonResult{
        Status:  determineStatus(changes),
        Changes: changes,
    }
}
```

---

## Proving Improvements

### Metrics

| Metric | Screenshot approach | DOM fingerprint approach | Measurement |
|--------|--------------------|-----------------------|-------------|
| Verification latency | 1-3s (vision model) | < 200ms (text comparison) | Time from request to actionable result |
| Cost per check | $0.01-0.05 (image tokens) | $0.0001 (text tokens) | API cost per verification |
| Token consumption | 1-5K tokens (image) | 300-500 tokens (JSON) | Tokens consumed per check |
| Actionability | "Something looks wrong" | "button 'Save' is missing" | Can agent fix without further investigation? |
| Checks per session | 5-10 (cost-limited) | 50+ (negligible cost) | How often agent can verify |

### Cost Comparison

**50-check session (typical active development):**
- Screenshots: 50 × $0.03 avg = **$1.50/session** in vision model costs
- DOM fingerprints: 50 × $0.0001 = **$0.005/session** in text tokens
- **Savings: 300x cost reduction** for structural verification

### Benchmark: Regression Detection Accuracy

1. Create 20 controlled regressions:
   - 5 × element removed (button, form field, navigation link)
   - 5 × error state introduced (error banner, alert)
   - 5 × content disappeared (list empty, heading removed)
   - 5 × CSS-only bugs (element hidden via CSS, z-index overlap)

2. Run DOM fingerprint comparison on each

3. Measure:
   - Detection rate for structural regressions (first 15): **Target > 95%**
   - Detection rate for CSS-only bugs (last 5): **Expected ~0%** (known limitation)
   - False positive rate: **Target < 5%**

### Benchmark: Speed vs. Depth

Compare three verification approaches on same 50 pages:

| Approach | Latency | Detections | Cost |
|----------|---------|-----------|------|
| `get_page_info` | 50ms | Low (counts only) | Free |
| `get_dom_fingerprint` | 200ms | Medium (structural) | Negligible |
| Playwright screenshot + vision | 2000ms | High (visual) | $0.03/check |

**Target:** DOM fingerprint provides 80% of screenshot detection accuracy at 1% of the cost and 10x the speed.

---

## Edge Cases

| Case | Handling |
|------|---------|
| Page still loading | Detect loading indicators in `state`; return `"status": "loading"` with partial fingerprint |
| SPA route change (same DOM, different content) | URL change is part of fingerprint; comparison detects route drift |
| Dynamic content (timestamps, counters) | Normalize: dates → `[date]`, numbers in text → `[number]`, unless in heading text |
| Very large page (1000+ elements) | Cap interactive elements at 100 (prioritize by visibility/viewport position) |
| iframe content | Skip by default; opt-in with `scope: "iframe#name"` |
| Shadow DOM | Pierce shadow DOM for interactive elements; report shadow hosts as landmarks |
| Page with no landmarks | Fall back to `<div>` structure; report `"no_landmarks": true` |
| Animation mid-frame | MutationObserver debounce (100ms) ensures stable state captured |

---

## Extension Changes Required

Unlike most AI-first features (which operate on server-side buffers), DOM fingerprinting requires a new query type in the extension:

| File | Change |
|------|--------|
| `extension/inject.js` | Add `extractDOMFingerprint()` function and message handler |
| `extension/background.js` | Route `GASOLINE_DOM_FINGERPRINT` query type to content script |
| `extension/content.js` | Relay fingerprint query/response between inject and background |
| `cmd/dev-console/v4.go` | Add `get_dom_fingerprint` and `compare_dom_fingerprint` MCP handlers |

The extraction logic runs in the page context (inject.js) to access the live DOM, computed styles, and visibility information.
