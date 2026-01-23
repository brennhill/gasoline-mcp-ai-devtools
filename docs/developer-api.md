---
title: "Developer API"
description: "Gasoline's window.__gasoline API for adding custom context, annotations, user actions, and reproduction scripts to browser error reports."
keywords: "gasoline API, window.__gasoline, browser context annotations, error context API, reproduction scripts, Playwright test generation"
permalink: /developer-api/
toc: true
toc_sticky: true
---

Gasoline exposes `window.__gasoline` for adding context to your logs and controlling capture behavior programmatically.

## Context Annotations

Add semantic context that gets included with all subsequent errors:

```javascript
// Add context
window.__gasoline.annotate('checkout-flow', { step: 'payment', cartId: 'abc123' })
window.__gasoline.annotate('user', { id: 'u123', plan: 'pro' })

// Remove specific annotation
window.__gasoline.removeAnnotation('checkout-flow')

// Clear all annotations
window.__gasoline.clearAnnotations()

// Get current context
const context = window.__gasoline.getContext()
```

### React Example

```jsx
useEffect(() => {
  if (user) {
    window.__gasoline?.annotate('user', {
      id: user.id,
      role: user.role,
    })
  }
  return () => window.__gasoline?.removeAnnotation('user')
}, [user])

function CheckoutPage() {
  useEffect(() => {
    window.__gasoline?.annotate('flow', 'checkout')
    return () => window.__gasoline?.removeAnnotation('flow')
  }, [])
}
```

## User Actions

Control the user action buffer that gets attached to errors:

```javascript
// Get recent actions
const actions = window.__gasoline.getActions()

// Clear the buffer
window.__gasoline.clearActions()

// Enable/disable action capture
window.__gasoline.setActionCapture(true)
```

## Enhanced Action Recording

Record enhanced actions with multi-strategy selectors for reproduction:

```javascript
// Record a custom action
window.__gasoline.recordAction('click', element, { text: 'Submit' })

// Get enhanced action buffer
const enhanced = window.__gasoline.getEnhancedActions()

// Clear enhanced actions
window.__gasoline.clearEnhancedActions()

// Generate a Playwright reproduction script
const script = window.__gasoline.generateScript(enhanced, { baseUrl: 'http://localhost:3000' })
```

## Network & Performance

```javascript
// Network waterfall
window.__gasoline.setNetworkWaterfall(true)
const waterfall = window.__gasoline.getNetworkWaterfall({ since: Date.now() - 30000 })

// Performance marks
window.__gasoline.setPerformanceMarks(true)
const marks = window.__gasoline.getMarks({ since: Date.now() - 60000 })
const measures = window.__gasoline.getMeasures()
```

## AI Context Enrichment

Enrich errors with framework-aware context (component ancestry, app state):

```javascript
// Enable AI context enrichment
window.__gasoline.setAiContext(true)

// Enable state snapshots in AI context
window.__gasoline.setStateSnapshot(true)

// Manually enrich an error
const enriched = window.__gasoline.enrichError(new Error('Something failed'))
```

## Selector Computation

Get multi-strategy selectors for any DOM element:

```javascript
const selectors = window.__gasoline.getSelectors(document.querySelector('#submit-btn'))
// Returns: { testId: 'submit-btn', aria: 'Submit', role: 'button', cssPath: '...' }
```

## Full API Reference

| Method | Description |
|--------|-------------|
| `annotate(key, value)` | Add context annotation (included with errors) |
| `removeAnnotation(key)` | Remove a specific annotation |
| `clearAnnotations()` | Clear all annotations |
| `getContext()` | Get current annotations |
| `getActions()` | Get recent user actions buffer |
| `clearActions()` | Clear the action buffer |
| `setActionCapture(enabled)` | Enable/disable user action capture |
| `setNetworkWaterfall(enabled)` | Enable/disable network waterfall |
| `getNetworkWaterfall(options)` | Get current network waterfall data |
| `setPerformanceMarks(enabled)` | Enable/disable performance marks |
| `getMarks(options)` | Get performance marks |
| `getMeasures(options)` | Get performance measures |
| `enrichError(error)` | Enrich an error with AI context |
| `setAiContext(enabled)` | Enable/disable AI context enrichment |
| `setStateSnapshot(enabled)` | Enable/disable state snapshot |
| `recordAction(type, el, opts)` | Record an enhanced action |
| `getEnhancedActions()` | Get the enhanced action buffer |
| `clearEnhancedActions()` | Clear the enhanced action buffer |
| `generateScript(actions, opts)` | Generate a Playwright reproduction script |
| `getSelectors(element)` | Compute multi-strategy selectors |
| `version` | API version |
