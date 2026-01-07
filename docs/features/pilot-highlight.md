# Agent Assignment: highlight_element

**Branch:** `feature/pilot-highlight`
**Worktree:** `../gasoline-pilot-highlight`
**Priority:** P4 Phase 2 (parallel — requires Phase 1 complete)
**Dependency:** Merge `feature/pilot-toggle` first

---

## Objective

Implement `highlight_element` MCP tool that injects a visual overlay on DOM elements so the AI can point at things for human verification.

---

## Deliverables

### 1. Inject.js Handler

**File:** `extension/inject.js`

Add in AI Web Pilot section:
```javascript
// ============================================================================
// AI WEB PILOT: HIGHLIGHT
// ============================================================================

let gasolineHighlighter = null

function highlightElement(selector, durationMs = 5000) {
  // Remove existing highlight
  if (gasolineHighlighter) {
    gasolineHighlighter.remove()
    gasolineHighlighter = null
  }

  const element = document.querySelector(selector)
  if (!element) {
    return { success: false, error: 'element_not_found', selector }
  }

  const rect = element.getBoundingClientRect()

  gasolineHighlighter = document.createElement('div')
  gasolineHighlighter.id = 'gasoline-highlighter'
  Object.assign(gasolineHighlighter.style, {
    position: 'fixed',
    top: `${rect.top}px`,
    left: `${rect.left}px`,
    width: `${rect.width}px`,
    height: `${rect.height}px`,
    border: '4px solid red',
    borderRadius: '4px',
    backgroundColor: 'rgba(255, 0, 0, 0.1)',
    zIndex: '2147483647',
    pointerEvents: 'none',
    boxSizing: 'border-box'
  })

  document.body.appendChild(gasolineHighlighter)

  setTimeout(() => {
    if (gasolineHighlighter) {
      gasolineHighlighter.remove()
      gasolineHighlighter = null
    }
  }, durationMs)

  return {
    success: true,
    selector,
    bounds: { x: rect.x, y: rect.y, width: rect.width, height: rect.height }
  }
}

// Handle scroll — update position
window.addEventListener('scroll', () => {
  if (gasolineHighlighter) {
    const selector = gasolineHighlighter.dataset.selector
    if (selector) {
      const el = document.querySelector(selector)
      if (el) {
        const rect = el.getBoundingClientRect()
        gasolineHighlighter.style.top = `${rect.top}px`
        gasolineHighlighter.style.left = `${rect.left}px`
      }
    }
  }
}, { passive: true })
```

### 2. Message Routing

**File:** `extension/background.js`

Handle `GASOLINE_HIGHLIGHT` message:
- Check `isAiWebPilotEnabled()`
- Forward to content script → inject.js
- Return result to server

**File:** `extension/content.js`

Forward `GASOLINE_HIGHLIGHT` to page context, return response.

### 3. MCP Tool Handler

**File:** `cmd/dev-console/pilot.go`

```go
func (v *Capture) handleHighlightElement(params map[string]any) (any, error) {
    selector, ok := params["selector"].(string)
    if !ok || selector == "" {
        return nil, errors.New("selector is required")
    }

    durationMs := 5000
    if d, ok := params["duration_ms"].(float64); ok {
        durationMs = int(d)
    }

    // Send to extension, wait for response
    result := v.sendPilotCommand("highlight", map[string]any{
        "selector":    selector,
        "duration_ms": durationMs,
    })

    return result, nil
}
```

---

## Tests

**File:** `extension-tests/pilot-highlight.test.js` (new)

1. Highlight creates div with correct styles
2. Highlight positions on element bounds
3. Highlight auto-removes after duration
4. Second highlight removes first
5. Returns error for non-existent selector
6. Highlight updates position on scroll

---

## Verification

```bash
node --test extension-tests/pilot-highlight.test.js
go test -v ./cmd/dev-console/ -run Highlight
```

---

## Files Modified

| File | Change |
|------|--------|
| `extension/inject.js` | `highlightElement()` function |
| `extension/background.js` | Route GASOLINE_HIGHLIGHT |
| `extension/content.js` | Forward highlight message |
| `cmd/dev-console/pilot.go` | `handleHighlightElement()` |
| `extension-tests/pilot-highlight.test.js` | New file |
