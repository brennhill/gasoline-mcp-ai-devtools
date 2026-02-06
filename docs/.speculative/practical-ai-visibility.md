---
status: proposed
scope: core/vision
ai-priority: high
tags: [vision, practical, immediate, ai-visibility, debugging, observability]
relates-to: [ai-engineering.md, semantic-graph.md, causal-chains-intent-inference.md, architecture.md]
last-verified: 2026-02-02
---

# Practical Immediate Improvements for AI Visibility

**Practical steps to give AI better visibility into current generation of software.**

---

## The Core Insight

Getting a full range of events and accurate timing is the most important thing to do immediately for an LLM to be able to debug. This document outlines practical improvements that can be implemented right now.

---

## Priority 1: High-Precision Timestamps

**Problem:** Current timestamps may be millisecond-precision or inconsistent across event sources.

**Solution:** Use `performance.now()` for sub-millisecond precision across all events.

```javascript
// In inject.js
class EventTracker {
  captureEvent(type, data) {
    return {
      id: generateId(),
      type,
      data,
      timestamp: performance.now(),  // High-precision relative to page load
      absoluteTime: Date.now(),      // For cross-tab correlation
      correlationId: this.currentCorrelationId
    };
  }
}
```

**Impact:** Enables accurate causal inference — AI can distinguish "A caused B" from "coincidence".

---

## Priority 2: Correlation IDs

**Problem:** Events from different sources (DOM, network, console) can't be linked.

**Solution:** Add correlation IDs to track related events across sources.

```javascript
// In inject.js
class CorrelationManager {
  constructor() {
    this.currentCorrelationId = null;
  }
  
  startCorrelation() {
    this.currentCorrelationId = generateId();
    return this.currentCorrelationId;
  }
  
  endCorrelation() {
    const id = this.currentCorrelationId;
    this.currentCorrelationId = null;
    return id;
  }
}

// Usage
function handleUserAction(action) {
  const correlationId = correlationManager.startCorrelation();
  
  // All events in this action get the same correlationId
  captureEvent('user_action', { ...action }, correlationId);
  
  // Network call triggered by this action
  fetch('/api/endpoint').then(response => {
    captureEvent('network_response', { response }, correlationId);
  });
  
  correlationManager.endCorrelation();
}
```

**Impact:** AI can link user action → network call → response → DOM update.

---

## Priority 3: Event Type Expansion

**Problem:** Gasoline currently captures limited event types (logs, network, WebSocket).

**Solution:** Add more event types to capture fuller picture.

| Event Type | Current | Add | Why It Matters |
|------------|---------|-----|----------------|
| User actions | ❌ No | ✅ Yes | Clicks, typing, scrolling — root causes |
| DOM mutations | ❌ No | ✅ Yes | What changed on screen |
| State changes | ❌ No | ✅ Yes | React/Vue/Svelte state updates |
| Error events | ✅ Yes | — | Already captured |
| Console events | ✅ Yes | — | Already captured |
| Network events | ✅ Yes | — | Already captured |
| WebSocket events | ✅ Yes | — | Already captured |

**Implementation:**

```javascript
// User action capture
document.addEventListener('click', (e) => {
  captureEvent('user_action', {
    type: 'click',
    target: getSemanticTarget(e.target),  // "submit_button", not "button.btn-primary"
    coordinates: { x: e.clientX, y: e.clientY }
  });
});

document.addEventListener('input', (e) => {
  captureEvent('user_action', {
    type: 'input',
    target: getSemanticTarget(e.target),
    value: e.target.value
  });
});

// DOM mutation capture
const observer = new MutationObserver((mutations) => {
  for (const mutation of mutations) {
    captureEvent('dom_mutation', {
      type: mutation.type,
      target: getSemanticTarget(mutation.target),
      addedNodes: mutation.addedNodes.length,
      removedNodes: mutation.removedNodes.length
    });
  }
});
observer.observe(document.body, { subtree: true, childList: true, attributes: true });

// React state capture (if React detected)
if (window.__REACT_DEVTOOLS_GLOBAL_HOOK__) {
  const hook = window.__REACT_DEVTOOLS_GLOBAL_HOOK__;
  hook.onCommitFiberRoot = (rendererID, root) => {
    captureEvent('state_change', {
      framework: 'react',
      component: root.current.memoizedState?.element?.type?.name
    });
  };
}
```

**Impact:** AI sees the full picture — what user did, what changed, what state was affected.

---

## Priority 4: Semantic Target Labels

**Problem:** AI sees `button.btn-primary` but doesn't know it's a "submit button for login form".

**Solution:** Add semantic labels to DOM elements.

```javascript
function getSemanticTarget(element) {
  // Try to infer semantic meaning
  const semantic = {
    element: element.tagName.toLowerCase(),
    role: element.getAttribute('role'),
    ariaLabel: element.getAttribute('aria-label'),
    textContent: element.textContent?.slice(0, 50),
    className: element.className,
    id: element.id,
    
    // Inferred semantic type
    semanticType: inferSemanticType(element),
    
    // Form context (if applicable)
    formContext: getFormContext(element)
  };
  
  return semantic;
}

function inferSemanticType(element) {
  // Button types
  if (element.tagName === 'BUTTON' || element.type === 'submit') {
    const text = element.textContent?.toLowerCase() || '';
    if (text.includes('submit') || text.includes('login') || text.includes('sign in')) {
      return 'submit_button';
    }
    if (text.includes('cancel') || text.includes('close')) {
      return 'cancel_button';
    }
    return 'button';
  }
  
  // Input types
  if (element.tagName === 'INPUT') {
    const type = element.type || 'text';
    const name = element.name || element.id || '';
    if (name.includes('email')) return 'email_input';
    if (name.includes('password')) return 'password_input';
    return `${type}_input`;
  }
  
  // Form
  if (element.tagName === 'FORM') {
    const action = element.action || '';
    if (action.includes('login')) return 'login_form';
    if (action.includes('signup')) return 'signup_form';
    return 'form';
  }
  
  return element.tagName.toLowerCase();
}

function getFormContext(element) {
  const form = element.closest('form');
  if (!form) return null;
  
  return {
    formId: form.id,
    formAction: form.action,
    formMethod: form.method,
    formName: form.name
  };
}
```

**Impact:** AI understands "user clicked submit button" not "user clicked button.btn-primary".

---

## Priority 5: Event Ordering & Batching

**Problem:** Events arrive out of order, making causal inference difficult.

**Solution:** Ensure events are ordered and batched properly.

```javascript
class OrderedEventBuffer {
  constructor() {
    this.buffer = [];
    this.flushInterval = 100;  // Flush every 100ms
    this.maxBufferSize = 100;
    
    setInterval(() => this.flush(), this.flushInterval);
  }
  
  add(event) {
    this.buffer.push(event);
    
    if (this.buffer.length >= this.maxBufferSize) {
      this.flush();
    }
  }
  
  flush() {
    if (this.buffer.length === 0) return;
    
    // Sort by timestamp
    this.buffer.sort((a, b) => a.timestamp - b.timestamp);
    
    // Send to background
    chrome.runtime.sendMessage({
      type: 'events_batch',
      events: this.buffer
    });
    
    this.buffer = [];
  }
}
```

**Impact:** AI receives events in chronological order, enabling accurate causal analysis.

---

## Priority 6: Error Context Capture

**Problem:** Errors are captured without surrounding context.

**Solution:** Capture events leading up to errors.

```javascript
class ErrorContextBuffer {
  constructor() {
    this.recentEvents = [];
    this.maxEvents = 50;  // Keep last 50 events
  }
  
  addEvent(event) {
    this.recentEvents.push(event);
    if (this.recentEvents.length > this.maxEvents) {
      this.recentEvents.shift();
    }
  }
  
  captureError(error) {
    return {
      error,
      context: this.recentEvents.slice(-10),  // Last 10 events before error
      timestamp: performance.now()
    };
  }
}

// Usage
const errorBuffer = new ErrorContextBuffer();

// All events go through buffer
function captureEvent(type, data) {
  const event = { type, data, timestamp: performance.now() };
  errorBuffer.addEvent(event);
  // ... rest of capture logic
}

// On error
window.addEventListener('error', (e) => {
  const errorWithContext = errorBuffer.captureError(e);
  sendToBackground('error_with_context', errorWithContext);
});
```

**Impact:** AI sees "what happened before the error" → can diagnose root cause.

---

## Priority 7: Network Request/Response Linking

**Problem:** Network requests and responses are separate events, hard to correlate.

**Solution:** Link requests to responses using request IDs.

```javascript
class NetworkTracker {
  constructor() {
    this.pendingRequests = new Map();
  }
  
  captureRequest(request) {
    const requestId = generateId();
    this.pendingRequests.set(requestId, {
      url: request.url,
      method: request.method,
      timestamp: performance.now()
    });
    
    return requestId;
  }
  
  captureResponse(requestId, response) {
    const request = this.pendingRequests.get(requestId);
    if (!request) return;
    
    const linkedEvent = {
      request,
      response,
      duration: performance.now() - request.timestamp,
      requestId
    };
    
    this.pendingRequests.delete(requestId);
    return linkedEvent;
  }
}
```

**Impact:** AI can correlate "POST /api/login" with "401 Unauthorized" response.

---

## Implementation Summary

| Priority | Feature | Effort | Impact | Timeline |
|----------|----------|--------|--------|----------|
| 1 | High-precision timestamps | Low | High | Immediate |
| 2 | Correlation IDs | Medium | High | 1-2 weeks |
| 3 | Event type expansion | Medium | Very High | 2-3 weeks |
| 4 | Semantic target labels | Medium | High | 1-2 weeks |
| 5 | Event ordering & batching | Low | Medium | 1 week |
| 6 | Error context capture | Low | High | 1 week |
| 7 | Network request/response linking | Low | Medium | 1 week |

**Total effort:** ~8-10 weeks for all improvements
**Immediate value:** High-precision timestamps + semantic labels can be done in 1-2 weeks

---

## Integration with Gasoline

These improvements can be integrated into Gasoline's existing architecture:

1. **inject.js** — Add event capture for user actions, DOM mutations, state changes
2. **content.js** — Forward new event types to background
3. **background.ts** — Batch and send to server
4. **server** — Store and serve via MCP tools

No major architectural changes required — incremental additions to existing systems.

---

## Related Documents

- [ai-engineering.md](ai-engineering.md) — AI-first vision overview
- [semantic-graph.md](semantic-graph.md) — Semantic State Graph deep dive
- [causal-chains-intent-inference.md](causal-chains-intent-inference.md) — Causal chains and intent inference
- [architecture.md](../../.claude/refs/architecture.md) — System architecture

---

**Last Updated:** 2026-02-02
**Status:** Proposed — Practical guide for immediate implementation
