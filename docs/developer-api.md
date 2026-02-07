# Gasoline Extension Developer API

The Gasoline extension exposes a public JavaScript API via `window.__gasoline` that allows developers to interact with the extension's capture capabilities and enrich telemetry with business context.

## Detection

Check if Gasoline is loaded before calling API methods:

```javascript
if (window.__gasoline) {
  // Gasoline is available
  window.__gasoline.annotate('checkout-started');
}
```

## Context Annotations

Context annotations allow you to enrich error logs and performance data with custom business context.

### `annotate(key, value)`

Add a context annotation that will be included with all errors, logs, and telemetry from this point forward.

**Signature:**
```typescript
annotate(key: string, value: any): void
```

**Parameters:**
- `key` — Annotation key (string, max 256 characters)
- `value` — Annotation value (serializable object or primitive)

**Examples:**

```javascript
// Simple label
window.__gasoline.annotate('checkout-flow', 'payment-processing');

// Rich context object
window.__gasoline.annotate('user-session', {
  userId: '12345',
  accountType: 'premium',
  region: 'us-west-2'
});

// API call context
window.__gasoline.annotate('api-call', {
  endpoint: '/api/payment',
  method: 'POST',
  retries: 2,
  duration_ms: 1250
});

// Form interaction
window.__gasoline.annotate('form-submission', {
  formId: 'checkout-form',
  fields: ['email', 'card', 'address'],
  validationErrors: 0
});
```

**Limits:**
- Maximum 50 annotations per page session
- Key length: 1-256 characters
- Value serialized size: max 10 KB
- Oldest annotations evicted when limit reached

**Use Cases:**
- Enriching error context with user/business information
- Tracking feature flags or experiment cohorts
- Recording transaction IDs or session identifiers
- Labeling critical user flows for easier debugging

### `removeAnnotation(key)`

Remove a specific context annotation.

**Signature:**
```typescript
removeAnnotation(key: string): void
```

**Parameters:**
- `key` — Annotation key to remove

**Example:**
```javascript
// Remove outdated context
window.__gasoline.annotate('feature-flag', 'variant-a');
// ... later, when feature changes
window.__gasoline.removeAnnotation('feature-flag');
```

### `clearAnnotations()`

Clear all context annotations.

**Signature:**
```typescript
clearAnnotations(): void
```

**Example:**
```javascript
// Reset all annotations at page transition or logout
window.__gasoline.clearAnnotations();
```

### `getContext()`

Retrieve current context annotations.

**Signature:**
```typescript
getContext(): Record<string, any> | null
```

**Returns:** Object with all current annotations, or `null` if none

**Example:**
```javascript
const context = window.__gasoline.getContext();
if (context) {
  console.log('Current context:', context);
}
```

## Action Capture

Manage the capture of user interactions (clicks, inputs, navigation, etc.).

### `setActionCapture(enabled)`

Enable or disable user action capture.

**Signature:**
```typescript
setActionCapture(enabled: boolean): void
```

**Example:**
```javascript
// Disable during sensitive operations
window.__gasoline.setActionCapture(false);
// ... perform sensitive actions
window.__gasoline.setActionCapture(true);
```

### `getActions()`

Retrieve the action replay buffer.

**Signature:**
```typescript
getActions(): Action[]
```

**Returns:** Array of recent user actions

**Example:**
```javascript
const actions = window.__gasoline.getActions();
console.log(`Last ${actions.length} user actions recorded`);
```

### `clearActions()`

Clear the action replay buffer.

**Signature:**
```typescript
clearActions(): void
```

**Example:**
```javascript
// Reset after completing a flow
window.__gasoline.clearActions();
```

## Network Waterfall

Access and control network request capture.

### `setNetworkWaterfall(enabled)`

Enable or disable network waterfall capture.

**Signature:**
```typescript
setNetworkWaterfall(enabled: boolean): void
```

**Example:**
```javascript
// Disable network capture for performance-sensitive code
window.__gasoline.setNetworkWaterfall(false);
```

### `getNetworkWaterfall(options?)`

Retrieve network request entries.

**Signature:**
```typescript
getNetworkWaterfall(options?: {
  initiatorType?: string;
  minDuration?: number;
}): PerformanceResourceTiming[]
```

**Parameters:**
- `options.initiatorType` — Filter by type (e.g., 'fetch', 'xhr')
- `options.minDuration` — Filter requests longer than N ms

**Returns:** Array of network waterfall entries (PerformanceResourceTiming)

**Example:**
```javascript
// Get all fetch requests
const fetches = window.__gasoline.getNetworkWaterfall({
  initiatorType: 'fetch',
  minDuration: 100  // >= 100ms
});

fetches.forEach(entry => {
  console.log(`${entry.name}: ${entry.duration.toFixed(2)}ms`);
});
```

## Performance Marks

Control and retrieve performance markers.

### `setPerformanceMarks(enabled)`

Enable or disable performance mark capture.

**Signature:**
```typescript
setPerformanceMarks(enabled: boolean): void
```

### `getMarks(options?)`

Retrieve performance marks.

**Signature:**
```typescript
getMarks(options?: {
  pattern?: string;
}): PerformanceEntryList
```

**Returns:** Array of PerformanceEntry objects with entryType: 'measure'

**Example:**
```javascript
window.performance.mark('checkout-start');
// ... checkout logic
window.performance.mark('checkout-end');
window.performance.measure('checkout', 'checkout-start', 'checkout-end');

const measures = window.__gasoline.getMarks();
measures.forEach(m => {
  console.log(`${m.name}: ${m.duration.toFixed(2)}ms`);
});
```

### `getMeasures(options?)`

Retrieve performance measures.

**Signature:**
```typescript
getMeasures(options?: {
  pattern?: string;
}): PerformanceEntry[]
```

**Returns:** Array of measure entries

## Reproduction & Testing

Generate Playwright scripts and compute selectors for test automation.

### `recordAction(type, element, options?)`

Record an enhanced action for reproduction (testing use case).

**Signature:**
```typescript
recordAction(
  type: 'click' | 'input' | 'keypress' | 'navigate' | 'select' | 'scroll',
  element?: Element,
  options?: {
    value?: string;
    key?: string;
    selectedValue?: string;
    selectedText?: string;
    scrollY?: number;
  }
): void
```

**Example:**
```javascript
const submitButton = document.querySelector('button[type="submit"]');
window.__gasoline.recordAction('click', submitButton);

const emailInput = document.querySelector('input[name="email"]');
window.__gasoline.recordAction('input', emailInput, {
  value: 'test@example.com'
});
```

### `getEnhancedActions()`

Retrieve the enhanced action buffer.

**Signature:**
```typescript
getEnhancedActions(): EnhancedAction[]
```

**Returns:** Array of recorded actions with computed selectors

### `clearEnhancedActions()`

Clear the enhanced action buffer.

**Signature:**
```typescript
clearEnhancedActions(): void
```

### `generateScript(options?)`

Generate a Playwright test script from recorded actions.

**Signature:**
```typescript
generateScript(options?: {
  baseUrl?: string;
  errorMessage?: string;
  lastNActions?: number;
}): string
```

**Parameters:**
- `baseUrl` — Replace origin in URLs with this base
- `errorMessage` — Include error context in script comments
- `lastNActions` — Only generate script for the last N actions

**Returns:** Playwright test file contents (string)

**Example:**
```javascript
// Record some user actions
const script = window.__gasoline.generateScript({
  errorMessage: 'Checkout flow failed at payment',
  baseUrl: 'https://example.com',
  lastNActions: 10
});

console.log(script);
// import { test, expect } from '@playwright/test';
// test('reproduction: Checkout flow failed at payment', async ({ page }) => {
//   await page.goto('https://example.com/checkout');
//   await page.getByLabel('Email').fill('test@example.com');
//   ...
// });
```

### `getSelectors(element)`

Compute multi-strategy selectors for an element.

**Signature:**
```typescript
getSelectors(element: Element): MultiStrategySelectors
```

**Returns:**
```typescript
{
  testId?: string;              // data-testid, data-test-id, or data-cy
  ariaLabel?: string;           // aria-label attribute
  role?: {
    role: string;               // ARIA role (implicit or explicit)
    name: string;               // accessible name
  };
  id?: string;                  // element id
  text?: string;                // visible text (clickable elements only)
  cssPath: string;              // CSS path fallback
}
```

**Example:**
```javascript
const button = document.querySelector('button.primary');
const selectors = window.__gasoline.getSelectors(button);

console.log(selectors);
// {
//   testId: 'submit-btn',
//   role: { role: 'button', name: 'Submit' },
//   text: 'Submit',
//   cssPath: 'main > form > button.primary'
// }
```

**Selection Priority:** testId > ariaLabel > role > id > text > cssPath

## Form Input

Interact with form inputs in a framework-aware manner.

### `setInputValue(selector, value)`

Set an input value and trigger framework change events.

**Signature:**
```typescript
setInputValue(selector: string, value: string | boolean): boolean
```

**Parameters:**
- `selector` — CSS selector for the input element
- `value` — Value to set (string for text, boolean for checkbox/radio)

**Returns:** `true` if successful, `false` if element not found

**Example:**
```javascript
// Text input
window.__gasoline.setInputValue('input[name="email"]', 'test@example.com');

// Checkbox
window.__gasoline.setInputValue('input[type="checkbox"]', true);

// Select dropdown
window.__gasoline.setInputValue('select[name="country"]', 'US');
```

**Supported Elements:**
- `<input>` (text, email, password, tel, url, checkbox, radio, etc.)
- `<textarea>`
- `<select>`

**Framework Support:**
Works with frameworks that rely on event listeners:
- React (React 16+)
- Vue (Vue 2 & 3)
- Svelte
- Angular
- Custom frameworks using `input`, `change`, or `blur` events

## AI Context

Control AI-assisted debugging features (error enrichment, state snapshots).

### `setAiContext(enabled)`

Enable or disable AI context enrichment for errors.

**Signature:**
```typescript
setAiContext(enabled: boolean): void
```

### `setStateSnapshot(enabled)`

Enable or disable browser state snapshots in error context.

**Signature:**
```typescript
setStateSnapshot(enabled: boolean): void
```

### `enrichError(error)`

Enrich an error entry with AI context.

**Signature:**
```typescript
enrichError(error: LogEntry): EnrichedErrorEntry
```

**Example:**
```javascript
try {
  // ... risky operation
} catch (err) {
  const enrichedError = window.__gasoline.enrichError({
    level: 'error',
    message: err.message,
    stack: err.stack,
    timestamp: Date.now(),
    // ... other LogEntry fields
  });
  console.log('Error enriched with AI context:', enrichedError);
}
```

## Version

### `version`

Get the Gasoline API version.

**Type:** `string`

**Example:**
```javascript
console.log(`Gasoline ${window.__gasoline.version}`);
```

## Best Practices

1. **Always check availability:**
   ```javascript
   if (!window.__gasoline) return;
   ```

2. **Use annotations early:** Add context as early as possible in your flow
   ```javascript
   window.__gasoline.annotate('flow', 'checkout');
   // ... rest of checkout flow
   ```

3. **Clean up sensitive context:** Remove annotations after sensitive operations
   ```javascript
   window.__gasoline.clearAnnotations();
   ```

4. **Limit annotation volume:** Keep under 50 annotations to avoid memory pressure

5. **Pause capture during performance-sensitive code:**
   ```javascript
   window.__gasoline.setNetworkWaterfall(false);
   // ... performance-critical code
   window.__gasoline.setNetworkWaterfall(true);
   ```

6. **Use testIds for selectors:** Make your elements easier to find in reproduction
   ```html
   <button data-testid="checkout-submit">Pay Now</button>
   ```

## Message Protocol

For advanced use cases, developers can also communicate with the extension via `chrome.runtime.sendMessage()`. See [Extension Message Protocol](/docs/core/extension-message-protocol.md) for details.
