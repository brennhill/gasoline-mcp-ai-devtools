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
  getAttribute() {
    return null
  }
  closest() {
    return null
  }
  querySelector() {
    return null
  }
  querySelectorAll() {
    return []
  }
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
globalThis.ShadowRoot = class ShadowRoot {}
globalThis.InputEvent = class extends Event {}
globalThis.KeyboardEvent = class extends Event {}
globalThis.getComputedStyle = () => ({ visibility: 'visible', display: 'block' })

// MutationObserver mock
class MockMutationObserver {
  constructor(cb) {
    this._cb = cb
  }
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
function setupDocument(extraElements = []) {
  const btn = new MockHTMLElement('BUTTON', { id: 'test-btn', textContent: 'Test' })
  Object.setPrototypeOf(btn, MockHTMLElement.prototype)

  const allElements = [btn, ...extraElements]

  globalThis.document = {
    querySelector: (sel) => (sel === '#test-btn' ? btn : null),
    querySelectorAll: (sel) => {
      if (sel === '#test-btn') return [btn]
      // Return extra elements that match the selector pattern
      return allElements.filter((el) => {
        if (sel.startsWith('[role=')) return el.getAttribute && el.getAttribute('role')
        return false
      })
    },
    getElementById: (id) => allElements.find((el) => el.id === id) || null,
    body: {
      querySelectorAll: (sel) => {
        return allElements.filter((el) => {
          const tag = el.tagName.toLowerCase()
          if (sel === 'button') return tag === 'button'
          if (sel === 'a[href]') return tag === 'a'
          if (sel === 'input') return tag === 'input'
          if (sel === 'select') return tag === 'select'
          if (sel === 'textarea') return tag === 'textarea'
          if (sel === 'label') return tag === 'label'
          if (sel.startsWith('[role=')) {
            const match = sel.match(/\[role="(.+?)"\]/)
            return match && el.getAttribute && el.getAttribute('role') === match[1]
          }
          if (sel.startsWith('[contenteditable=')) return el.getAttribute && el.getAttribute('contenteditable') === 'true'
          if (sel.startsWith('[onclick]')) return false
          if (sel.startsWith('[tabindex]')) return false
          return false
        })
      },
      appendChild: () => {},
      children: { length: 0 }
    },
    documentElement: {
      children: { length: 0 }
    },
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
    assert.strictEqual(
      winner.tag,
      'resolved',
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

    assert.ok(winner.result.dom_summary, 'Click result must include dom_summary even when rAF is suppressed')
  })

  test('MutationObserver is disconnected when rAF is suppressed (no resource leak)', async () => {
    setupDocument()

    let disconnected = false
    globalThis.MutationObserver = class {
      constructor(cb) {
        this._cb = cb
      }
      observe() {}
      disconnect() {
        disconnected = true
      }
    }

    // rAF never fires
    globalThis.requestAnimationFrame = () => {}

    const result = domPrimitive('click', '#test-btn', {})

    // Wait long enough for any safety timeout to fire
    await new Promise((resolve) => setTimeout(resolve, 600))

    // FAILS: observer is never disconnected because rAF never fires
    assert.strictEqual(
      disconnected,
      true,
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
      { action: 'set_attribute', opts: { name: 'data-x', value: '1' } }
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
    assert.strictEqual(
      hungActions.length,
      0,
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
    assert.ok('dom_summary' in result, 'Compact click result MUST include dom_summary — always-on feedback')
    assert.strictEqual(typeof result.dom_summary, 'string')
    assert.ok(result.matched, 'mutating action should include matched target evidence')
    assert.strictEqual(result.matched.tag, 'button')
    assert.strictEqual(result.matched.selector, '#test-btn')
  })

  test('compact mode omits timing; analyze:true includes full breakdown', async () => {
    const btn = setupDocument()
    btn.click = () => {}

    const compactResult = await domPrimitive('click', '#test-btn', {})
    assert.strictEqual(compactResult.timing, undefined, 'Compact mode should NOT include timing object')

    const analyzeResult = await domPrimitive('click', '#test-btn', { analyze: true })
    assert.ok(analyzeResult.timing, 'analyze:true should include timing')
    assert.strictEqual(typeof analyzeResult.timing.total_ms, 'number')
    assert.ok(analyzeResult.dom_changes, 'analyze:true should include dom_changes')
    assert.ok(analyzeResult.analysis, 'analyze:true should include analysis string')
  })
})

describe('list_interactive returns index, element_type, and deduplicates selectors', () => {
  beforeEach(() => {
    perfNowValue = 0
    globalThis.MutationObserver = MockMutationObserver
    globalThis.requestAnimationFrame = (cb) => cb()
  })

  test('list_interactive returns elements with index and element_type fields', () => {
    const btn1 = new MockHTMLElement('BUTTON', { id: 'btn1', textContent: 'Save' })
    Object.setPrototypeOf(btn1, MockHTMLElement.prototype)
    btn1.getBoundingClientRect = () => ({ width: 100, height: 30 })
    btn1.getRootNode = () => globalThis.document
    btn1.getAttribute = (name) => {
      if (name === 'role') return null
      if (name === 'aria-label') return null
      if (name === 'title') return null
      if (name === 'placeholder') return null
      if (name === 'contenteditable') return null
      return null
    }

    const btn2 = new MockHTMLElement('BUTTON', { id: 'btn2', textContent: 'Cancel' })
    Object.setPrototypeOf(btn2, MockHTMLElement.prototype)
    btn2.getBoundingClientRect = () => ({ width: 100, height: 30 })
    btn2.getRootNode = () => globalThis.document
    btn2.getAttribute = (name) => {
      if (name === 'role') return null
      if (name === 'aria-label') return null
      if (name === 'title') return null
      if (name === 'placeholder') return null
      if (name === 'contenteditable') return null
      return null
    }

    setupDocument([btn2])

    // querySelectorAllDeep calls document.querySelectorAll first
    const origDocQSA = globalThis.document.querySelectorAll
    globalThis.document.querySelectorAll = (sel) => {
      if (sel === 'button') return [btn1, btn2]
      return origDocQSA(sel)
    }

    const result = domPrimitive('list_interactive', '', {})

    assert.strictEqual(result.success, true)
    assert.ok(Array.isArray(result.elements), 'elements should be an array')
    assert.ok(result.elements.length >= 2, 'should find at least 2 buttons')

    // Check index field
    assert.strictEqual(result.elements[0].index, 0, 'first element should have index 0')
    assert.strictEqual(result.elements[1].index, 1, 'second element should have index 1')

    // Check element_type field
    assert.strictEqual(result.elements[0].element_type, 'button', 'button should have element_type "button"')
    assert.strictEqual(result.elements[1].element_type, 'button', 'button should have element_type "button"')
  })

  test('list_interactive deduplicates selectors with :nth-match(N)', () => {
    // Two buttons with same text (no id) produce duplicate selectors
    const btn1 = new MockHTMLElement('BUTTON', { textContent: 'Submit' })
    btn1.id = ''
    Object.setPrototypeOf(btn1, MockHTMLElement.prototype)
    btn1.getBoundingClientRect = () => ({ width: 100, height: 30 })
    btn1.getRootNode = () => globalThis.document
    btn1.getAttribute = (name) => {
      if (name === 'role') return null
      if (name === 'aria-label') return null
      if (name === 'title') return null
      if (name === 'placeholder') return null
      if (name === 'contenteditable') return null
      return null
    }

    const btn2 = new MockHTMLElement('BUTTON', { textContent: 'Submit' })
    btn2.id = ''
    Object.setPrototypeOf(btn2, MockHTMLElement.prototype)
    btn2.getBoundingClientRect = () => ({ width: 100, height: 30 })
    btn2.getRootNode = () => globalThis.document
    btn2.getAttribute = (name) => {
      if (name === 'role') return null
      if (name === 'aria-label') return null
      if (name === 'title') return null
      if (name === 'placeholder') return null
      if (name === 'contenteditable') return null
      return null
    }

    setupDocument([btn2])

    // querySelectorAllDeep calls document.querySelectorAll first
    const origDocQSA = globalThis.document.querySelectorAll
    globalThis.document.querySelectorAll = (sel) => {
      if (sel === 'button') return [btn1, btn2]
      return origDocQSA(sel)
    }

    const result = domPrimitive('list_interactive', '', {})

    assert.strictEqual(result.success, true)
    const selectors = result.elements.map((e) => e.selector)

    // Both have same base text, so selectors should be deduplicated
    const uniqueSelectors = new Set(selectors)
    assert.strictEqual(
      uniqueSelectors.size,
      selectors.length,
      `All selectors must be unique. Got: ${JSON.stringify(selectors)}`
    )

    // At least one should have :nth-match suffix
    const nthMatchSelectors = selectors.filter((s) => s.includes(':nth-match('))
    assert.ok(
      nthMatchSelectors.length > 0,
      `Duplicate selectors should use :nth-match(N). Got: ${JSON.stringify(selectors)}`
    )
  })

  test('list_interactive scopes results to selector when provided', () => {
    const outsideBtn = new MockHTMLElement('BUTTON', { id: 'outside-btn', textContent: 'Outside Action' })
    Object.setPrototypeOf(outsideBtn, MockHTMLElement.prototype)
    outsideBtn.getBoundingClientRect = () => ({ width: 100, height: 30 })
    outsideBtn.getRootNode = () => globalThis.document
    outsideBtn.getAttribute = () => null

    const postBtn = new MockHTMLElement('BUTTON', { id: 'post-btn', textContent: 'Post' })
    Object.setPrototypeOf(postBtn, MockHTMLElement.prototype)
    postBtn.getBoundingClientRect = () => ({ width: 100, height: 30 })
    postBtn.getRootNode = () => globalThis.document
    postBtn.getAttribute = () => null

    const dialog = new MockHTMLElement('DIV', { id: 'composer-dialog' })
    Object.setPrototypeOf(dialog, MockHTMLElement.prototype)
    dialog.getBoundingClientRect = () => ({ width: 300, height: 240 })
    dialog.getRootNode = () => globalThis.document
    dialog.getAttribute = (name) => (name === 'role' ? 'dialog' : null)
    dialog.querySelectorAll = (sel) => {
      if (sel === 'button') return [postBtn]
      return []
    }
    dialog.children = { length: 0 }

    globalThis.document = {
      querySelector: () => null,
      querySelectorAll: (sel) => {
        if (sel === '[role="dialog"]') return [dialog]
        if (sel === 'button') return [outsideBtn, postBtn]
        return []
      },
      getElementById: () => null,
      body: {
        querySelectorAll: (sel) => {
          if (sel === 'button') return [outsideBtn, postBtn]
          return []
        },
        appendChild: () => {},
        children: { length: 0 }
      },
      documentElement: { children: { length: 0 } },
      createTreeWalker: () => ({ nextNode: () => null }),
      getSelection: () => null,
      execCommand: () => {}
    }

    const scopedResult = domPrimitive('list_interactive', '[role="dialog"]', {})
    assert.strictEqual(scopedResult.success, true)
    const labels = scopedResult.elements.map((e) => e.label)
    assert.ok(labels.includes('Post'), 'scoped list should include dialog button')
    assert.ok(!labels.includes('Outside Action'), 'scoped list should exclude non-dialog controls')
  })

  test('list_interactive returns scope_not_found for missing scoped container', () => {
    setupDocument()

    const result = domPrimitive('list_interactive', '[role="dialog"]', {})
    assert.strictEqual(result.success, false)
    assert.strictEqual(result.error, 'scope_not_found')
  })

  test('list_interactive scopes results to scope_rect when provided', () => {
    const outsideBtn = new MockHTMLElement('BUTTON', { id: 'outside-btn', textContent: 'Outside Action' })
    Object.setPrototypeOf(outsideBtn, MockHTMLElement.prototype)
    outsideBtn.getBoundingClientRect = () => ({
      x: 10,
      y: 20,
      left: 10,
      top: 20,
      width: 100,
      height: 30,
      right: 110,
      bottom: 50
    })
    outsideBtn.getRootNode = () => globalThis.document
    outsideBtn.getAttribute = () => null

    const insideBtn = new MockHTMLElement('BUTTON', { id: 'inside-btn', textContent: 'Inside Action' })
    Object.setPrototypeOf(insideBtn, MockHTMLElement.prototype)
    insideBtn.getBoundingClientRect = () => ({
      x: 430,
      y: 340,
      left: 430,
      top: 340,
      width: 120,
      height: 36,
      right: 550,
      bottom: 376
    })
    insideBtn.getRootNode = () => globalThis.document
    insideBtn.getAttribute = () => null

    globalThis.document = {
      querySelector: () => null,
      querySelectorAll: (sel) => {
        if (sel === 'button') return [outsideBtn, insideBtn]
        return []
      },
      getElementById: () => null,
      body: {
        querySelectorAll: (sel) => {
          if (sel === 'button') return [outsideBtn, insideBtn]
          return []
        },
        appendChild: () => {},
        children: { length: 0 }
      },
      documentElement: { children: { length: 0 } },
      createTreeWalker: () => ({ nextNode: () => null }),
      getSelection: () => null,
      execCommand: () => {}
    }

    const result = domPrimitive('list_interactive', '', {
      scope_rect: { x: 400, y: 300, width: 220, height: 180 }
    })
    assert.strictEqual(result.success, true)
    const labels = result.elements.map((e) => e.label)
    assert.ok(labels.includes('Inside Action'), 'rect-scoped list should include in-rect control')
    assert.ok(!labels.includes('Outside Action'), 'rect-scoped list should exclude out-of-rect control')
    assert.strictEqual(result.candidate_count, 1)
    assert.deepStrictEqual(result.scope_rect_used, { x: 400, y: 300, width: 220, height: 180 })
  })

  test('list_interactive auto-selects composer-like dialog when scope matches multiple dialogs', () => {
    const notifBtn = new MockHTMLElement('BUTTON', { id: 'notif-mark-read', textContent: 'Mark as read' })
    Object.setPrototypeOf(notifBtn, MockHTMLElement.prototype)
    notifBtn.getBoundingClientRect = () => ({ width: 80, height: 24 })
    notifBtn.offsetParent = null // hidden-ish notification control
    notifBtn.getRootNode = () => globalThis.document
    notifBtn.getAttribute = (name) => (name === 'role' ? 'button' : null)

    const postBtn = new MockHTMLElement('BUTTON', { id: 'composer-post', textContent: 'Post' })
    Object.setPrototypeOf(postBtn, MockHTMLElement.prototype)
    postBtn.getBoundingClientRect = () => ({ width: 110, height: 32 })
    postBtn.getRootNode = () => globalThis.document
    postBtn.getAttribute = (name) => (name === 'role' ? 'button' : null)

    const textbox = new MockHTMLElement('DIV', { id: 'composer-textbox', textContent: 'Draft content' })
    Object.setPrototypeOf(textbox, MockHTMLElement.prototype)
    textbox.getBoundingClientRect = () => ({ width: 420, height: 180 })
    textbox.getRootNode = () => globalThis.document
    textbox.getAttribute = (name) => {
      if (name === 'role') return 'textbox'
      if (name === 'contenteditable') return 'true'
      return null
    }

    const notifDialog = new MockHTMLElement('DIV', { id: 'notif-dialog' })
    Object.setPrototypeOf(notifDialog, MockHTMLElement.prototype)
    notifDialog.getBoundingClientRect = () => ({ width: 320, height: 500 })
    notifDialog.getRootNode = () => globalThis.document
    notifDialog.getAttribute = (name) => (name === 'role' ? 'dialog' : null)
    notifDialog.children = { length: 0 }
    notifDialog.querySelectorAll = (sel) => {
      if (sel.includes('button')) return [notifBtn]
      if (sel.includes('input[type="submit"]')) return []
      if (sel.includes('textbox') || sel.includes('contenteditable') || sel.includes('textarea')) return []
      if (sel.includes('a[href]') || sel.includes('[role="button"]') || sel.includes('[role="link"]')) return [notifBtn]
      return []
    }

    const composerDialog = new MockHTMLElement('DIV', { id: 'composer-dialog' })
    Object.setPrototypeOf(composerDialog, MockHTMLElement.prototype)
    composerDialog.getBoundingClientRect = () => ({ width: 620, height: 640 })
    composerDialog.getRootNode = () => globalThis.document
    composerDialog.getAttribute = (name) => (name === 'role' ? 'dialog' : null)
    composerDialog.children = { length: 0 }
    composerDialog.querySelectorAll = (sel) => {
      if (sel.includes('textbox') || sel.includes('contenteditable') || sel.includes('textarea')) return [textbox]
      if (sel.includes('button') || sel.includes('input[type="submit"]')) return [postBtn]
      if (sel.includes('a[href]') || sel.includes('[role="button"]') || sel.includes('[role="link"]')) return [postBtn, textbox]
      if (sel === 'input' || sel === 'select') return []
      return []
    }

    globalThis.document = {
      querySelector: () => null,
      querySelectorAll: (sel) => {
        if (sel === '[role="dialog"]') return [notifDialog, composerDialog]
        if (sel === 'button') return [notifBtn, postBtn]
        if (sel === '[role="button"]') return [notifBtn, postBtn]
        if (sel === '[role="textbox"]') return [textbox]
        if (sel === '[contenteditable="true"]') return [textbox]
        return []
      },
      getElementById: () => null,
      body: {
        querySelectorAll: (sel) => {
          if (sel === '[role="dialog"]') return [notifDialog, composerDialog]
          if (sel === 'button') return [notifBtn, postBtn]
          if (sel === '[role="button"]') return [notifBtn, postBtn]
          if (sel === '[role="textbox"]') return [textbox]
          if (sel === '[contenteditable="true"]') return [textbox]
          return []
        },
        appendChild: () => {},
        children: { length: 0 }
      },
      documentElement: { children: { length: 0 } },
      createTreeWalker: () => ({ nextNode: () => null }),
      getSelection: () => null,
      execCommand: () => {}
    }

    const result = domPrimitive('list_interactive', '[role="dialog"]', {})
    assert.strictEqual(result.success, true)
    const labels = result.elements.map((e) => e.label)
    assert.ok(labels.includes('Post'), 'dialog disambiguation should pick composer dialog')
    assert.ok(!labels.includes('Mark as read'), 'dialog disambiguation should avoid notification dialog')
  })

  test(':nth-match(N) resolves to the Nth matching element', () => {
    const btn1 = new MockHTMLElement('BUTTON', { id: '', textContent: 'OK' })
    Object.setPrototypeOf(btn1, MockHTMLElement.prototype)
    btn1.getBoundingClientRect = () => ({ width: 100, height: 30 })
    btn1.getRootNode = () => globalThis.document

    const btn2 = new MockHTMLElement('BUTTON', { id: '', textContent: 'OK' })
    Object.setPrototypeOf(btn2, MockHTMLElement.prototype)
    btn2.getBoundingClientRect = () => ({ width: 100, height: 30 })
    btn2.getRootNode = () => globalThis.document

    setupDocument([btn2])

    // Mock querySelectorAll to return both for a CSS selector
    globalThis.document.querySelector = (sel) => {
      if (sel === '.dup-btn') return btn1
      return null
    }
    globalThis.document.querySelectorAll = (sel) => {
      if (sel === '.dup-btn') return [btn1, btn2]
      return []
    }
    globalThis.document.body.querySelectorAll = () => []

    // :nth-match(1) should resolve to first
    const result1 = domPrimitive('get_text', '.dup-btn:nth-match(1)', {})
    assert.strictEqual(result1.success, true, 'nth-match(1) should find an element')

    // :nth-match(2) should resolve to second
    const result2 = domPrimitive('get_text', '.dup-btn:nth-match(2)', {})
    assert.strictEqual(result2.success, true, 'nth-match(2) should find an element')

    // :nth-match(3) should not find anything
    const result3 = domPrimitive('get_text', '.dup-btn:nth-match(3)', {})
    assert.strictEqual(result3.success, false, 'nth-match(3) should not find an element (only 2 exist)')
  })
})

describe('ambiguity-safe mutating actions', () => {
  beforeEach(() => {
    perfNowValue = 0
    globalThis.MutationObserver = MockMutationObserver
    globalThis.requestAnimationFrame = (cb) => cb()
  })

  test('click returns ambiguous_target when selector matches multiple visible elements', async () => {
    let clickCount = 0
    const btn1 = new MockHTMLElement('BUTTON', { id: '', textContent: 'Post' })
    Object.setPrototypeOf(btn1, MockHTMLElement.prototype)
    btn1.getBoundingClientRect = () => ({ width: 100, height: 30 })
    btn1.getRootNode = () => globalThis.document
    btn1.getAttribute = () => null
    btn1.click = () => { clickCount++ }

    const btn2 = new MockHTMLElement('BUTTON', { id: '', textContent: 'Post' })
    Object.setPrototypeOf(btn2, MockHTMLElement.prototype)
    btn2.getBoundingClientRect = () => ({ width: 120, height: 36 })
    btn2.getRootNode = () => globalThis.document
    btn2.getAttribute = () => null
    btn2.click = () => { clickCount++ }

    globalThis.document = {
      querySelector: (sel) => (sel === '.dup-post' ? btn1 : null),
      querySelectorAll: (sel) => (sel === '.dup-post' ? [btn1, btn2] : []),
      getElementById: () => null,
      body: { querySelectorAll: () => [], appendChild: () => {}, children: { length: 0 } },
      documentElement: { children: { length: 0 } },
      createTreeWalker: () => ({ nextNode: () => null }),
      getSelection: () => null,
      execCommand: () => {}
    }

    const raw = domPrimitive('click', '.dup-post', {})
    const result = raw instanceof Promise ? await raw : raw

    assert.strictEqual(result.success, false)
    assert.strictEqual(result.error, 'ambiguous_target')
    assert.strictEqual(result.match_count, 2)
    assert.strictEqual(result.match_strategy, 'ambiguous_selector')
    assert.ok(Array.isArray(result.candidates), 'candidates should be provided for disambiguation')
    assert.ok(result.candidates.length >= 2, 'expected at least two candidates')
    assert.ok(
      (result.message || '').includes('element_id'),
      'ambiguous hint should include element_id recovery guidance'
    )
    assert.strictEqual(clickCount, 0, 'no click should be executed on ambiguous target')
  })

  test('read-only actions remain backward-compatible on duplicate matches', () => {
    const first = new MockHTMLElement('DIV', { textContent: 'alpha' })
    first.innerText = 'alpha'
    Object.setPrototypeOf(first, MockHTMLElement.prototype)
    first.getBoundingClientRect = () => ({ width: 100, height: 20 })
    first.getRootNode = () => globalThis.document
    first.getAttribute = () => null

    const second = new MockHTMLElement('DIV', { textContent: 'beta' })
    second.innerText = 'beta'
    Object.setPrototypeOf(second, MockHTMLElement.prototype)
    second.getBoundingClientRect = () => ({ width: 100, height: 20 })
    second.getRootNode = () => globalThis.document
    second.getAttribute = () => null

    globalThis.document = {
      querySelector: (sel) => (sel === '.dup-text' ? first : null),
      querySelectorAll: (sel) => (sel === '.dup-text' ? [first, second] : []),
      getElementById: () => null,
      body: { querySelectorAll: () => [], appendChild: () => {}, children: { length: 0 } },
      documentElement: { children: { length: 0 } },
      createTreeWalker: () => ({ nextNode: () => null }),
      getSelection: () => null,
      execCommand: () => {}
    }

    const result = domPrimitive('get_text', '.dup-text', {})
    assert.strictEqual(result.success, true)
    assert.strictEqual(result.value, 'alpha')
  })

  test('scope_selector narrows mutating actions to a scoped container', async () => {
    let outsideClicks = 0
    let scopedClicks = 0

    const outside = new MockHTMLElement('BUTTON', { textContent: 'Post outside' })
    Object.setPrototypeOf(outside, MockHTMLElement.prototype)
    outside.getBoundingClientRect = () => ({ width: 100, height: 30 })
    outside.getRootNode = () => globalThis.document
    outside.getAttribute = () => null
    outside.click = () => { outsideClicks++ }

    const inside = new MockHTMLElement('BUTTON', { textContent: 'Post in scope' })
    Object.setPrototypeOf(inside, MockHTMLElement.prototype)
    inside.getBoundingClientRect = () => ({ width: 120, height: 36 })
    inside.getRootNode = () => globalThis.document
    inside.getAttribute = () => null
    inside.click = () => { scopedClicks++ }

    const scope = new MockHTMLElement('DIV', { id: 'composer' })
    Object.setPrototypeOf(scope, MockHTMLElement.prototype)
    scope.getAttribute = () => null
    scope.querySelector = (sel) => (sel === '.dup-post' ? inside : null)
    scope.querySelectorAll = (sel) => (sel === '.dup-post' ? [inside] : [])
    scope.children = { length: 0 }

    globalThis.document = {
      querySelector: (sel) => {
        if (sel === '#composer') return scope
        if (sel === '.dup-post') return outside
        return null
      },
      querySelectorAll: (sel) => {
        if (sel === '#composer') return [scope]
        if (sel === '.dup-post') return [outside, inside]
        return []
      },
      getElementById: () => null,
      body: { querySelectorAll: () => [], appendChild: () => {}, children: { length: 0 } },
      documentElement: { children: { length: 0 } },
      createTreeWalker: () => ({ nextNode: () => null }),
      getSelection: () => null,
      execCommand: () => {}
    }

    const raw = domPrimitive('click', '.dup-post', { scope_selector: '#composer' })
    const result = raw instanceof Promise ? await raw : raw

    assert.strictEqual(result.success, true)
    assert.strictEqual(outsideClicks, 0, 'outside element should not be clicked when scope is provided')
    assert.strictEqual(scopedClicks, 1, 'scoped element should be clicked exactly once')
    assert.strictEqual(result.match_count, 1)
    assert.strictEqual(result.match_strategy, 'scoped_selector')
    assert.strictEqual(result.matched.scope_selector_used, '#composer')
  })

  test('scope_selector applies to read-only actions', () => {
    const outside = new MockHTMLElement('DIV', { textContent: 'outside text' })
    outside.innerText = 'outside text'
    Object.setPrototypeOf(outside, MockHTMLElement.prototype)
    outside.getBoundingClientRect = () => ({ width: 100, height: 20 })
    outside.getRootNode = () => globalThis.document
    outside.getAttribute = () => null

    const inside = new MockHTMLElement('DIV', { textContent: 'inside text' })
    inside.innerText = 'inside text'
    Object.setPrototypeOf(inside, MockHTMLElement.prototype)
    inside.getBoundingClientRect = () => ({ width: 120, height: 24 })
    inside.getRootNode = () => globalThis.document
    inside.getAttribute = () => null

    const scope = new MockHTMLElement('DIV', { id: 'composer' })
    Object.setPrototypeOf(scope, MockHTMLElement.prototype)
    scope.getAttribute = () => null
    scope.querySelector = (sel) => (sel === '.dup-text' ? inside : null)
    scope.querySelectorAll = (sel) => (sel === '.dup-text' ? [inside] : [])
    scope.children = { length: 0 }

    globalThis.document = {
      querySelector: (sel) => {
        if (sel === '#composer') return scope
        if (sel === '.dup-text') return outside
        return null
      },
      querySelectorAll: (sel) => {
        if (sel === '#composer') return [scope]
        if (sel === '.dup-text') return [outside, inside]
        return []
      },
      getElementById: () => null,
      body: { querySelectorAll: () => [], appendChild: () => {}, children: { length: 0 } },
      documentElement: { children: { length: 0 } },
      createTreeWalker: () => ({ nextNode: () => null }),
      getSelection: () => null,
      execCommand: () => {}
    }

    const result = domPrimitive('get_text', '.dup-text', { scope_selector: '#composer' })
    assert.strictEqual(result.success, true)
    assert.strictEqual(result.value, 'inside text')
  })

  test('scope_selector returns scope_not_found when container is missing', () => {
    setupDocument()

    const result = domPrimitive('get_text', '#test-btn', { scope_selector: '#missing-scope' })
    assert.strictEqual(result.success, false)
    assert.strictEqual(result.error, 'scope_not_found')
  })

  test('click with scope_rect returns ambiguous_target when multiple candidates are in-region', async () => {
    let clickCount = 0
    const btn1 = new MockHTMLElement('BUTTON', { textContent: 'Post 1' })
    Object.setPrototypeOf(btn1, MockHTMLElement.prototype)
    btn1.getBoundingClientRect = () => ({
      x: 410, y: 320, left: 410, top: 320, width: 110, height: 32, right: 520, bottom: 352
    })
    btn1.getRootNode = () => globalThis.document
    btn1.getAttribute = () => null
    btn1.click = () => { clickCount++ }

    const btn2 = new MockHTMLElement('BUTTON', { textContent: 'Post 2' })
    Object.setPrototypeOf(btn2, MockHTMLElement.prototype)
    btn2.getBoundingClientRect = () => ({
      x: 430, y: 360, left: 430, top: 360, width: 110, height: 32, right: 540, bottom: 392
    })
    btn2.getRootNode = () => globalThis.document
    btn2.getAttribute = () => null
    btn2.click = () => { clickCount++ }

    globalThis.document = {
      querySelector: (sel) => (sel === '.dup-post' ? btn1 : null),
      querySelectorAll: (sel) => (sel === '.dup-post' ? [btn1, btn2] : []),
      getElementById: () => null,
      body: { querySelectorAll: () => [], appendChild: () => {}, children: { length: 0 } },
      documentElement: { children: { length: 0 } },
      createTreeWalker: () => ({ nextNode: () => null }),
      getSelection: () => null,
      execCommand: () => {}
    }

    const raw = domPrimitive('click', '.dup-post', {
      scope_rect: { x: 400, y: 300, width: 220, height: 180 }
    })
    const result = raw instanceof Promise ? await raw : raw

    assert.strictEqual(result.success, false)
    assert.strictEqual(result.error, 'ambiguous_target')
    assert.strictEqual(result.match_count, 2)
    assert.strictEqual(clickCount, 0, 'no click should be executed on ambiguous in-rect targets')
  })

  test('click with scope_rect ignores out-of-region duplicates and targets in-region element', async () => {
    let outsideClicks = 0
    let insideClicks = 0

    const outside = new MockHTMLElement('BUTTON', { textContent: 'Outside' })
    Object.setPrototypeOf(outside, MockHTMLElement.prototype)
    outside.getBoundingClientRect = () => ({
      x: 40, y: 50, left: 40, top: 50, width: 100, height: 30, right: 140, bottom: 80
    })
    outside.getRootNode = () => globalThis.document
    outside.getAttribute = () => null
    outside.click = () => { outsideClicks++ }

    const inside = new MockHTMLElement('BUTTON', { textContent: 'Inside' })
    Object.setPrototypeOf(inside, MockHTMLElement.prototype)
    inside.getBoundingClientRect = () => ({
      x: 430, y: 340, left: 430, top: 340, width: 120, height: 36, right: 550, bottom: 376
    })
    inside.getRootNode = () => globalThis.document
    inside.getAttribute = () => null
    inside.click = () => { insideClicks++ }

    globalThis.document = {
      querySelector: (sel) => (sel === '.dup-post' ? outside : null),
      querySelectorAll: (sel) => (sel === '.dup-post' ? [outside, inside] : []),
      getElementById: () => null,
      body: { querySelectorAll: () => [], appendChild: () => {}, children: { length: 0 } },
      documentElement: { children: { length: 0 } },
      createTreeWalker: () => ({ nextNode: () => null }),
      getSelection: () => null,
      execCommand: () => {}
    }

    const raw = domPrimitive('click', '.dup-post', {
      scope_rect: { x: 400, y: 300, width: 220, height: 180 }
    })
    const result = raw instanceof Promise ? await raw : raw

    assert.strictEqual(result.success, true)
    assert.strictEqual(insideClicks, 1, 'in-rect element should be clicked exactly once')
    assert.strictEqual(outsideClicks, 0, 'out-of-rect duplicate should not be clicked')
    assert.strictEqual(result.match_count, 1)
  })
})

describe('element handles', () => {
  beforeEach(() => {
    perfNowValue = 0
    globalThis.MutationObserver = MockMutationObserver
    globalThis.requestAnimationFrame = (cb) => cb()
  })

  test('list_interactive includes element_id handles', () => {
    const btn = new MockHTMLElement('BUTTON', { textContent: 'Submit' })
    Object.setPrototypeOf(btn, MockHTMLElement.prototype)
    btn.getBoundingClientRect = () => ({ width: 100, height: 30 })
    btn.getRootNode = () => globalThis.document
    btn.getAttribute = () => null

    globalThis.document = {
      querySelector: () => null,
      querySelectorAll: (sel) => (sel === 'button' ? [btn] : []),
      getElementById: () => null,
      body: { querySelectorAll: () => [], appendChild: () => {}, children: { length: 0 } },
      documentElement: { children: { length: 0 } },
      createTreeWalker: () => ({ nextNode: () => null }),
      getSelection: () => null,
      execCommand: () => {}
    }

    const result = domPrimitive('list_interactive', '', {})
    assert.strictEqual(result.success, true)
    assert.ok(result.elements.length > 0)
    assert.strictEqual(typeof result.elements[0].element_id, 'string')
    assert.ok(result.elements[0].element_id.startsWith('el_'))
  })

  test('element_id remains stable across list refresh even when order changes', () => {
    const alpha = new MockHTMLElement('BUTTON', { textContent: 'Alpha' })
    Object.setPrototypeOf(alpha, MockHTMLElement.prototype)
    alpha.getBoundingClientRect = () => ({ width: 100, height: 30 })
    alpha.getRootNode = () => globalThis.document
    alpha.getAttribute = () => null

    const beta = new MockHTMLElement('BUTTON', { textContent: 'Beta' })
    Object.setPrototypeOf(beta, MockHTMLElement.prototype)
    beta.getBoundingClientRect = () => ({ width: 100, height: 30 })
    beta.getRootNode = () => globalThis.document
    beta.getAttribute = () => null

    let pass = 0
    globalThis.document = {
      querySelector: () => null,
      querySelectorAll: (sel) => {
        if (sel === 'button') {
          pass++
          return pass > 1 ? [beta, alpha] : [alpha, beta]
        }
        return []
      },
      getElementById: () => null,
      body: { querySelectorAll: () => [], appendChild: () => {}, children: { length: 0 } },
      documentElement: { children: { length: 0 } },
      createTreeWalker: () => ({ nextNode: () => null }),
      getSelection: () => null,
      execCommand: () => {}
    }

    const first = domPrimitive('list_interactive', '', {})
    const second = domPrimitive('list_interactive', '', {})

    const firstByLabel = new Map(first.elements.map((e) => [e.label, e.element_id]))
    const secondByLabel = new Map(second.elements.map((e) => [e.label, e.element_id]))
    assert.strictEqual(firstByLabel.get('Alpha'), secondByLabel.get('Alpha'))
    assert.strictEqual(firstByLabel.get('Beta'), secondByLabel.get('Beta'))
  })

  test('click resolves by element_id even when selector no longer matches', async () => {
    let clickCount = 0
    const btn = new MockHTMLElement('BUTTON', { textContent: 'Post' })
    Object.setPrototypeOf(btn, MockHTMLElement.prototype)
    btn.getBoundingClientRect = () => ({ width: 120, height: 36 })
    btn.getRootNode = () => globalThis.document
    btn.getAttribute = () => null
    btn.click = () => { clickCount++ }

    globalThis.document = {
      querySelector: () => null,
      querySelectorAll: (sel) => (sel === 'button' ? [btn] : []),
      getElementById: () => null,
      body: { querySelectorAll: () => [], appendChild: () => {}, children: { length: 0 } },
      documentElement: { children: { length: 0 } },
      createTreeWalker: () => ({ nextNode: () => null }),
      getSelection: () => null,
      execCommand: () => {}
    }

    const listed = domPrimitive('list_interactive', '', {})
    const elementID = listed.elements[0].element_id

    const raw = domPrimitive('click', '.missing-selector', { element_id: elementID })
    const result = raw instanceof Promise ? await raw : raw

    assert.strictEqual(result.success, true)
    assert.strictEqual(clickCount, 1)
    assert.strictEqual(result.matched.element_id, elementID)
    assert.strictEqual(result.match_count, 1)
    assert.strictEqual(result.match_strategy, 'element_id')
  })

  test('returns stale_element_id when element handle is unknown', () => {
    setupDocument()
    const result = domPrimitive('click', '#test-btn', { element_id: 'el_missing' })
    assert.strictEqual(result.success, false)
    assert.strictEqual(result.error, 'stale_element_id')
  })

  test('returns element_id_scope_mismatch when handle is outside requested scope', async () => {
    const outside = new MockHTMLElement('BUTTON', { textContent: 'Outside' })
    Object.setPrototypeOf(outside, MockHTMLElement.prototype)
    outside.getBoundingClientRect = () => ({ width: 100, height: 30 })
    outside.getRootNode = () => globalThis.document
    outside.getAttribute = () => null

    const inside = new MockHTMLElement('BUTTON', { textContent: 'Inside' })
    Object.setPrototypeOf(inside, MockHTMLElement.prototype)
    inside.getBoundingClientRect = () => ({ width: 120, height: 36 })
    inside.getRootNode = () => globalThis.document
    inside.getAttribute = () => null

    const scope = new MockHTMLElement('DIV', { id: 'composer' })
    Object.setPrototypeOf(scope, MockHTMLElement.prototype)
    scope.getAttribute = () => null
    scope.querySelector = (sel) => (sel === '.btn' ? inside : null)
    scope.querySelectorAll = (sel) => (sel === '.btn' ? [inside] : [])
    scope.contains = (node) => node === inside
    scope.children = { length: 0 }

    globalThis.document = {
      querySelector: (sel) => {
        if (sel === '#composer') return scope
        if (sel === '.btn') return outside
        return null
      },
      querySelectorAll: (sel) => {
        if (sel === '#composer') return [scope]
        if (sel === '.btn' || sel === 'button') return [outside, inside]
        return []
      },
      getElementById: () => null,
      body: { querySelectorAll: () => [], appendChild: () => {}, children: { length: 0 } },
      documentElement: { children: { length: 0 } },
      createTreeWalker: () => ({ nextNode: () => null }),
      getSelection: () => null,
      execCommand: () => {}
    }

    const listed = domPrimitive('list_interactive', '', {})
    const outsideHandle = listed.elements.find((e) => e.label === 'Outside').element_id
    const raw = domPrimitive('click', '.btn', { element_id: outsideHandle, scope_selector: '#composer' })
    const result = raw instanceof Promise ? await raw : raw

    assert.strictEqual(result.success, false)
    assert.strictEqual(result.error, 'element_id_scope_mismatch')
  })
})

describe('intent-level composer and dialog primitives', () => {
  beforeEach(() => {
    perfNowValue = 0
    globalThis.MutationObserver = MockMutationObserver
    globalThis.requestAnimationFrame = (cb) => cb()
    globalThis.getComputedStyle = (el) => ({
      visibility: 'visible',
      display: 'block',
      position: (el && el.style && el.style.position) || '',
      zIndex: (el && el.style && el.style.zIndex) || '0'
    })
  })

  test('open_composer selects and clicks the best composer trigger', async () => {
    let startPostClicks = 0
    let otherClicks = 0

    const startPost = new MockHTMLElement('BUTTON', { textContent: 'Start a post' })
    Object.setPrototypeOf(startPost, MockHTMLElement.prototype)
    startPost.getBoundingClientRect = () => ({ width: 180, height: 40 })
    startPost.getRootNode = () => globalThis.document
    startPost.getAttribute = () => null
    startPost.click = () => {
      startPostClicks++
    }

    const other = new MockHTMLElement('BUTTON', { textContent: 'View insights' })
    Object.setPrototypeOf(other, MockHTMLElement.prototype)
    other.getBoundingClientRect = () => ({ width: 140, height: 36 })
    other.getRootNode = () => globalThis.document
    other.getAttribute = () => null
    other.click = () => {
      otherClicks++
    }

    globalThis.document = {
      querySelector: () => null,
      querySelectorAll: (sel) => {
        if (sel === 'button') return [other, startPost]
        return []
      },
      getElementById: () => null,
      body: { querySelectorAll: () => [], appendChild: () => {}, children: { length: 0 } },
      documentElement: { children: { length: 0 } },
      createTreeWalker: () => ({ nextNode: () => null }),
      getSelection: () => null,
      execCommand: () => {}
    }

    const raw = domPrimitive('open_composer', '', {})
    const result = raw instanceof Promise ? await raw : raw

    assert.strictEqual(result.success, true)
    assert.strictEqual(startPostClicks, 1)
    assert.strictEqual(otherClicks, 0)
    assert.strictEqual(result.match_strategy, 'intent_open_composer')
    assert.strictEqual(result.match_count, 1)
    assert.ok(result.matched && result.matched.element_id, 'matched evidence should include element_id')
  })

  test('submit_active_composer prefers submit action within active composer scope', async () => {
    let postClicks = 0
    let cancelClicks = 0

    const textbox = new MockHTMLElement('DIV', { textContent: 'Draft content' })
    Object.setPrototypeOf(textbox, MockHTMLElement.prototype)
    textbox.getBoundingClientRect = () => ({ width: 420, height: 180 })
    textbox.getRootNode = () => globalThis.document
    textbox.getAttribute = (name) => {
      if (name === 'role') return 'textbox'
      if (name === 'contenteditable') return 'true'
      return null
    }

    const postBtn = new MockHTMLElement('BUTTON', { textContent: 'Post' })
    Object.setPrototypeOf(postBtn, MockHTMLElement.prototype)
    postBtn.getBoundingClientRect = () => ({ width: 100, height: 32 })
    postBtn.getRootNode = () => globalThis.document
    postBtn.getAttribute = () => null
    postBtn.click = () => {
      postClicks++
    }

    const cancelBtn = new MockHTMLElement('BUTTON', { textContent: 'Cancel' })
    Object.setPrototypeOf(cancelBtn, MockHTMLElement.prototype)
    cancelBtn.getBoundingClientRect = () => ({ width: 100, height: 32 })
    cancelBtn.getRootNode = () => globalThis.document
    cancelBtn.getAttribute = () => null
    cancelBtn.click = () => {
      cancelClicks++
    }

    const composerDialog = new MockHTMLElement('DIV', { id: 'composer-dialog' })
    Object.setPrototypeOf(composerDialog, MockHTMLElement.prototype)
    composerDialog.style.zIndex = '200'
    composerDialog.getBoundingClientRect = () => ({ width: 640, height: 520 })
    composerDialog.getRootNode = () => globalThis.document
    composerDialog.getAttribute = (name) => (name === 'role' ? 'dialog' : null)
    composerDialog.querySelectorAll = (sel) => {
      if (sel === '[role="textbox"], textarea, [contenteditable="true"]') return [textbox]
      if (sel === 'button, [role="button"], input[type="submit"]') return [cancelBtn, postBtn]
      if (sel === 'button') return [cancelBtn, postBtn]
      return []
    }
    composerDialog.children = { length: 0 }

    globalThis.document = {
      querySelector: () => null,
      querySelectorAll: (sel) => {
        if (sel === '[role="dialog"]') return [composerDialog]
        return []
      },
      getElementById: () => null,
      body: {
        querySelectorAll: (sel) => {
          if (sel === '[role="dialog"]') return [composerDialog]
          return []
        },
        appendChild: () => {},
        children: { length: 0 }
      },
      documentElement: { children: { length: 0 } },
      createTreeWalker: () => ({ nextNode: () => null }),
      getSelection: () => null,
      execCommand: () => {}
    }

    const raw = domPrimitive('submit_active_composer', '', {})
    const result = raw instanceof Promise ? await raw : raw

    assert.strictEqual(result.success, true)
    assert.strictEqual(postClicks, 1)
    assert.strictEqual(cancelClicks, 0)
    assert.strictEqual(result.match_strategy, 'intent_submit_active_composer')
    assert.ok(result.matched && result.matched.element_id, 'matched evidence should include element_id')
  })

  test('confirm_top_dialog targets the top-most dialog confirm action', async () => {
    let topClicks = 0
    let lowerClicks = 0

    const lowerConfirm = new MockHTMLElement('BUTTON', { textContent: 'Confirm' })
    Object.setPrototypeOf(lowerConfirm, MockHTMLElement.prototype)
    lowerConfirm.getBoundingClientRect = () => ({ width: 110, height: 30 })
    lowerConfirm.getRootNode = () => globalThis.document
    lowerConfirm.getAttribute = () => null
    lowerConfirm.click = () => {
      lowerClicks++
    }

    const topConfirm = new MockHTMLElement('BUTTON', { textContent: 'Confirm' })
    Object.setPrototypeOf(topConfirm, MockHTMLElement.prototype)
    topConfirm.getBoundingClientRect = () => ({ width: 110, height: 30 })
    topConfirm.getRootNode = () => globalThis.document
    topConfirm.getAttribute = () => null
    topConfirm.click = () => {
      topClicks++
    }

    const lowerDialog = new MockHTMLElement('DIV', { id: 'lower' })
    Object.setPrototypeOf(lowerDialog, MockHTMLElement.prototype)
    lowerDialog.style.zIndex = '10'
    lowerDialog.getBoundingClientRect = () => ({ width: 400, height: 300 })
    lowerDialog.getRootNode = () => globalThis.document
    lowerDialog.getAttribute = (name) => (name === 'role' ? 'dialog' : null)
    lowerDialog.querySelectorAll = (sel) => (sel.includes('button') ? [lowerConfirm] : [])
    lowerDialog.children = { length: 0 }

    const topDialog = new MockHTMLElement('DIV', { id: 'top' })
    Object.setPrototypeOf(topDialog, MockHTMLElement.prototype)
    topDialog.style.zIndex = '300'
    topDialog.getBoundingClientRect = () => ({ width: 400, height: 300 })
    topDialog.getRootNode = () => globalThis.document
    topDialog.getAttribute = (name) => (name === 'role' ? 'dialog' : null)
    topDialog.querySelectorAll = (sel) => (sel.includes('button') ? [topConfirm] : [])
    topDialog.children = { length: 0 }

    globalThis.document = {
      querySelector: () => null,
      querySelectorAll: (sel) => {
        if (sel === '[role="dialog"]') return [lowerDialog, topDialog]
        return []
      },
      getElementById: () => null,
      body: {
        querySelectorAll: (sel) => {
          if (sel === '[role="dialog"]') return [lowerDialog, topDialog]
          return []
        },
        appendChild: () => {},
        children: { length: 0 }
      },
      documentElement: { children: { length: 0 } },
      createTreeWalker: () => ({ nextNode: () => null }),
      getSelection: () => null,
      execCommand: () => {}
    }

    const raw = domPrimitive('confirm_top_dialog', '', {})
    const result = raw instanceof Promise ? await raw : raw

    assert.strictEqual(result.success, true)
    assert.strictEqual(topClicks, 1)
    assert.strictEqual(lowerClicks, 0)
    assert.strictEqual(result.match_strategy, 'intent_confirm_top_dialog')
  })

  test('dismiss_top_overlay targets close controls in top-most overlay', async () => {
    let topCloseClicks = 0
    let lowCloseClicks = 0

    const lowClose = new MockHTMLElement('BUTTON', { textContent: 'Close' })
    Object.setPrototypeOf(lowClose, MockHTMLElement.prototype)
    lowClose.getBoundingClientRect = () => ({ width: 96, height: 30 })
    lowClose.getRootNode = () => globalThis.document
    lowClose.getAttribute = (name) => (name === 'aria-label' ? 'Close' : null)
    lowClose.click = () => {
      lowCloseClicks++
    }

    const topClose = new MockHTMLElement('BUTTON', { textContent: 'Close' })
    Object.setPrototypeOf(topClose, MockHTMLElement.prototype)
    topClose.getBoundingClientRect = () => ({ width: 96, height: 30 })
    topClose.getRootNode = () => globalThis.document
    topClose.getAttribute = (name) => (name === 'aria-label' ? 'Close' : null)
    topClose.click = () => {
      topCloseClicks++
    }

    const lowDialog = new MockHTMLElement('DIV', { id: 'low-overlay' })
    Object.setPrototypeOf(lowDialog, MockHTMLElement.prototype)
    lowDialog.style.zIndex = '20'
    lowDialog.getBoundingClientRect = () => ({ width: 320, height: 220 })
    lowDialog.getRootNode = () => globalThis.document
    lowDialog.getAttribute = (name) => (name === 'role' ? 'dialog' : null)
    lowDialog.querySelectorAll = (sel) => {
      if (sel === 'button, [role="button"], [aria-label], [data-testid], [title]') return [lowClose]
      return []
    }
    lowDialog.children = { length: 0 }

    const topDialog = new MockHTMLElement('DIV', { id: 'top-overlay' })
    Object.setPrototypeOf(topDialog, MockHTMLElement.prototype)
    topDialog.style.zIndex = '900'
    topDialog.getBoundingClientRect = () => ({ width: 420, height: 280 })
    topDialog.getRootNode = () => globalThis.document
    topDialog.getAttribute = (name) => (name === 'role' ? 'dialog' : null)
    topDialog.querySelectorAll = (sel) => {
      if (sel === 'button, [role="button"], [aria-label], [data-testid], [title]') return [topClose]
      return []
    }
    topDialog.children = { length: 0 }

    globalThis.document = {
      querySelector: () => null,
      querySelectorAll: (sel) => {
        if (sel === '[role="dialog"]') return [lowDialog, topDialog]
        return []
      },
      getElementById: () => null,
      body: {
        querySelectorAll: (sel) => {
          if (sel === '[role="dialog"]') return [lowDialog, topDialog]
          return []
        },
        appendChild: () => {},
        children: { length: 0 }
      },
      documentElement: { children: { length: 0 } },
      createTreeWalker: () => ({ nextNode: () => null }),
      getSelection: () => null,
      execCommand: () => {}
    }

    const raw = domPrimitive('dismiss_top_overlay', '', {})
    const result = raw instanceof Promise ? await raw : raw

    assert.strictEqual(result.success, true)
    assert.strictEqual(topCloseClicks, 1)
    assert.strictEqual(lowCloseClicks, 0)
    assert.strictEqual(result.match_strategy, 'intent_dismiss_top_overlay')
  })

  test('submit_active_composer returns ambiguous_target with candidate summary when tied', () => {
    const postA = new MockHTMLElement('BUTTON', { textContent: 'Post' })
    Object.setPrototypeOf(postA, MockHTMLElement.prototype)
    postA.getBoundingClientRect = () => ({ width: 100, height: 30 })
    postA.getRootNode = () => globalThis.document
    postA.getAttribute = () => null

    const postB = new MockHTMLElement('BUTTON', { textContent: 'Post' })
    Object.setPrototypeOf(postB, MockHTMLElement.prototype)
    postB.getBoundingClientRect = () => ({ width: 100, height: 30 })
    postB.getRootNode = () => globalThis.document
    postB.getAttribute = () => null

    const textbox = new MockHTMLElement('DIV', { textContent: 'Draft content' })
    Object.setPrototypeOf(textbox, MockHTMLElement.prototype)
    textbox.getBoundingClientRect = () => ({ width: 420, height: 180 })
    textbox.getRootNode = () => globalThis.document
    textbox.getAttribute = (name) => {
      if (name === 'role') return 'textbox'
      if (name === 'contenteditable') return 'true'
      return null
    }

    const dialog = new MockHTMLElement('DIV', { id: 'composer-ambiguous' })
    Object.setPrototypeOf(dialog, MockHTMLElement.prototype)
    dialog.style.zIndex = '220'
    dialog.getBoundingClientRect = () => ({ width: 620, height: 520 })
    dialog.getRootNode = () => globalThis.document
    dialog.getAttribute = (name) => (name === 'role' ? 'dialog' : null)
    dialog.querySelectorAll = (sel) => {
      if (sel === '[role="textbox"], textarea, [contenteditable="true"]') return [textbox]
      if (sel === 'button, [role="button"], input[type="submit"]') return [postA, postB]
      return []
    }
    dialog.children = { length: 0 }

    globalThis.document = {
      querySelector: () => null,
      querySelectorAll: (sel) => {
        if (sel === '[role="dialog"]') return [dialog]
        return []
      },
      getElementById: () => null,
      body: {
        querySelectorAll: (sel) => {
          if (sel === '[role="dialog"]') return [dialog]
          return []
        },
        appendChild: () => {},
        children: { length: 0 }
      },
      documentElement: { children: { length: 0 } },
      createTreeWalker: () => ({ nextNode: () => null }),
      getSelection: () => null,
      execCommand: () => {}
    }

    const result = domPrimitive('submit_active_composer', '', {})
    assert.strictEqual(result.success, false)
    assert.strictEqual(result.error, 'ambiguous_target')
    assert.strictEqual(result.match_strategy, 'intent_submit_active_composer')
    assert.ok(Array.isArray(result.candidates), 'ambiguous response should include candidates')
    assert.ok(result.candidates.length >= 2, 'expected candidate summary for ambiguity')
  })
})
