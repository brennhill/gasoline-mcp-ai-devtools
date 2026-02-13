// @ts-nocheck
/**
 * @fileoverview dom-primitives.test.js — Bug reproduction tests for rAF hang.
 *
 * Bug: withMutationTracking uses requestAnimationFrame to wait one paint frame
 * before collecting mutation results. In non-visible tabs (backgrounded, headless,
 * automated test environments), rAF callbacks are suppressed or indefinitely deferred.
 * This causes the Promise to hang forever — no dom_summary, no timing, no result.
 *
 * These tests assert CORRECT behavior — they FAIL until the bug is fixed.
 *
 * Run: node --test extension/background/dom-primitives.test.js
 */

import { test, describe, beforeEach, mock } from 'node:test'
import assert from 'node:assert'

// ---------------------------------------------------------------------------
// Minimal DOM mocks — just enough to exercise domPrimitive's click path
// ---------------------------------------------------------------------------
class MockHTMLElement {
  constructor(tag, props = {}) {
    this.tagName = tag
    this.id = props.id || ''
    this.textContent = props.textContent || ''
    this.offsetParent = {} // non-null = visible
    this.style = { position: '' }
  }
  click() {}
  focus() {}
  getAttribute() { return null }
  closest() { return null }
  querySelector() { return null }
  scrollIntoView() {}
  setAttribute() {}
  dispatchEvent() {}
}

// Make instanceof checks work
globalThis.HTMLElement = MockHTMLElement
globalThis.HTMLInputElement = class extends MockHTMLElement {}
globalThis.HTMLTextAreaElement = class extends MockHTMLElement {}
globalThis.HTMLSelectElement = class extends MockHTMLElement {}
globalThis.CSS = { escape: (s) => s }
globalThis.NodeFilter = { SHOW_TEXT: 4 }
globalThis.InputEvent = class extends Event {}
globalThis.KeyboardEvent = class extends Event {}
globalThis.getComputedStyle = () => ({ visibility: 'visible', display: 'block' })

// MutationObserver mock
class MockMutationObserver {
  constructor(cb) { this._cb = cb }
  observe() {}
  disconnect() {}
}
globalThis.MutationObserver = MockMutationObserver

// performance.now mock
let perfNowValue = 0
globalThis.performance = { now: () => perfNowValue++ }

// ---------------------------------------------------------------------------
// Import domPrimitive AFTER globals are set up
// ---------------------------------------------------------------------------
const { domPrimitive } = await import('./dom-primitives.js')

// ---------------------------------------------------------------------------
// Helper: create a mock document with a findable button
// ---------------------------------------------------------------------------
function setupDocument() {
  const btn = new MockHTMLElement('BUTTON', { id: 'test-btn', textContent: 'Test' })
  Object.setPrototypeOf(btn, MockHTMLElement.prototype)

  globalThis.document = {
    querySelector: (sel) => sel === '#test-btn' ? btn : null,
    querySelectorAll: () => [],
    body: {
      querySelectorAll: () => [],
      appendChild: () => {}
    },
    documentElement: {},
    createTreeWalker: () => ({ nextNode: () => null }),
    getSelection: () => null,
    execCommand: () => {}
  }

  return btn
}

// ---------------------------------------------------------------------------
// Bug reproduction: these tests assert CORRECT behavior and FAIL until fixed
// ---------------------------------------------------------------------------

describe('BUG: click must resolve even when requestAnimationFrame is suppressed', () => {
  beforeEach(() => {
    perfNowValue = 0
    globalThis.MutationObserver = MockMutationObserver
  })

  test('click resolves within 500ms when rAF is suppressed (backgrounded tab)', async () => {
    const btn = setupDocument()
    btn.click = () => {}

    // Simulate backgrounded tab: rAF accepts callback but NEVER fires it
    globalThis.requestAnimationFrame = () => {}

    const result = domPrimitive('click', '#test-btn', {})
    assert.ok(result instanceof Promise, 'click should return a Promise')

    // CORRECT behavior: click should still resolve via fallback timeout
    const winner = await Promise.race([
      result.then((r) => ({ tag: 'resolved', result: r })),
      new Promise((resolve) => setTimeout(() => resolve({ tag: 'timeout' }), 500))
    ])

    // FAILS: Promise hangs because withMutationTracking depends on rAF
    assert.strictEqual(winner.tag, 'resolved',
      'click MUST resolve even when requestAnimationFrame never fires. ' +
      'In backgrounded/headless tabs, rAF is suppressed indefinitely.'
    )
  })

  test('click returns dom_summary when rAF is suppressed', async () => {
    const btn = setupDocument()
    btn.click = () => {}

    // rAF never fires
    globalThis.requestAnimationFrame = () => {}

    const result = domPrimitive('click', '#test-btn', {})

    const winner = await Promise.race([
      result.then((r) => ({ tag: 'resolved', result: r })),
      new Promise((resolve) => setTimeout(() => resolve({ tag: 'timeout' }), 500))
    ])

    // FAILS: can't check dom_summary because the Promise never resolves
    if (winner.tag === 'timeout') {
      assert.fail(
        'Cannot verify dom_summary — click Promise hung (rAF suppressed). ' +
        'withMutationTracking needs a setTimeout fallback.'
      )
    }

    assert.ok(winner.result.dom_summary,
      'Click result must include dom_summary even when rAF is suppressed')
  })

  test('MutationObserver is disconnected when rAF is suppressed (no resource leak)', async () => {
    setupDocument()

    let disconnected = false
    globalThis.MutationObserver = class {
      constructor(cb) { this._cb = cb }
      observe() {}
      disconnect() { disconnected = true }
    }

    // rAF never fires
    globalThis.requestAnimationFrame = () => {}

    const result = domPrimitive('click', '#test-btn', {})

    // Wait long enough for any safety timeout to fire
    await new Promise((resolve) => setTimeout(resolve, 600))

    // FAILS: observer is never disconnected because rAF never fires
    assert.strictEqual(disconnected, true,
      'MutationObserver MUST be disconnected even when rAF is suppressed. ' +
      'Currently leaks an observer on document.body with childList+subtree+attributes.'
    )

    // Prevent unhandled rejection
    result.catch(() => {})
  })

  test('all mutation-tracked actions resolve when rAF is suppressed', async () => {
    // rAF never fires
    globalThis.requestAnimationFrame = () => {}

    const actionsToTest = [
      { action: 'click', opts: {} },
      { action: 'type', opts: { text: 'hello' } },
      { action: 'key_press', opts: { text: 'Enter' } },
      { action: 'set_attribute', opts: { name: 'data-x', value: '1' } },
    ]

    const hungActions = []

    for (const { action, opts } of actionsToTest) {
      const btn = setupDocument()
      btn.click = () => {}
      btn.type = 'text'
      btn.value = ''
      btn.isContentEditable = false
      btn.setAttribute = () => {}
      btn.getAttribute = () => ''
      btn.dispatchEvent = () => {}
      btn.ownerDocument = { querySelectorAll: () => [] }

      const result = domPrimitive(action, '#test-btn', opts)
      if (result instanceof Promise) {
        const winner = await Promise.race([
          result.then(() => 'resolved'),
          new Promise((resolve) => setTimeout(() => resolve('timeout'), 200))
        ])
        if (winner === 'timeout') hungActions.push(action)
      }
    }

    // FAILS: all mutation-tracked actions hang
    assert.strictEqual(hungActions.length, 0,
      `These actions hung when rAF was suppressed: [${hungActions.join(', ')}]. ` +
      'All mutation-tracked actions need a setTimeout fallback.'
    )
  })
})

describe('compact click feedback contract (when rAF works)', () => {
  beforeEach(() => {
    perfNowValue = 0
    globalThis.MutationObserver = MockMutationObserver
    // rAF fires immediately — happy path
    globalThis.requestAnimationFrame = (cb) => cb()
  })

  test('click returns dom_summary in compact mode (no analyze)', async () => {
    const btn = setupDocument()
    btn.click = () => {}

    const result = await domPrimitive('click', '#test-btn', {})

    assert.strictEqual(result.success, true)
    assert.strictEqual(result.action, 'click')
    assert.ok('dom_summary' in result,
      'Compact click result MUST include dom_summary — always-on feedback')
    assert.strictEqual(typeof result.dom_summary, 'string')
  })

  test('compact mode omits timing; analyze:true includes full breakdown', async () => {
    const btn = setupDocument()
    btn.click = () => {}

    const compactResult = await domPrimitive('click', '#test-btn', {})
    assert.strictEqual(compactResult.timing, undefined,
      'Compact mode should NOT include timing object')

    const analyzeResult = await domPrimitive('click', '#test-btn', { analyze: true })
    assert.ok(analyzeResult.timing, 'analyze:true should include timing')
    assert.strictEqual(typeof analyzeResult.timing.total_ms, 'number')
    assert.ok(analyzeResult.dom_changes, 'analyze:true should include dom_changes')
    assert.ok(analyzeResult.analysis, 'analyze:true should include analysis string')
  })
})
