// @ts-nocheck
/**
 * @fileoverview dom-primitives-richtext.test.js — Tests for rich text support:
 * 1. type action: multiline text in contenteditable (split on \n, insertParagraph)
 * 2. get_text: use innerText instead of textContent to preserve line breaks
 * 3. paste action: synthetic ClipboardEvent on contenteditable elements
 *
 * Run: node --test extension/background/dom-primitives-richtext.test.js
 */

import { test, describe, beforeEach } from 'node:test'
import assert from 'node:assert'

// ---------------------------------------------------------------------------
// Minimal DOM mocks
// ---------------------------------------------------------------------------
class MockHTMLElement {
  constructor(tag, props = {}) {
    this.tagName = tag
    this.id = props.id || ''
    this.textContent = props.textContent || ''
    this.innerText = props.innerText || ''
    this.isContentEditable = props.isContentEditable || false
    this.offsetParent = {}
    this.style = { position: '' }
  }
  click() {}
  focus() {}
  getAttribute(name) {
    if (name === 'contenteditable' && this.isContentEditable) return 'true'
    return null
  }
  closest() {
    return null
  }
  querySelector() {
    return null
  }
  querySelectorAll(sel) {
    if (sel === '*') return [this]
    return []
  }
  scrollIntoView() {}
  setAttribute() {}
  dispatchEvent() {
    return true
  }
  getBoundingClientRect() {
    return { width: 100, height: 20, top: 0, left: 0, right: 100, bottom: 20 }
  }
}

globalThis.HTMLElement = MockHTMLElement
globalThis.HTMLInputElement = class extends MockHTMLElement {}
globalThis.HTMLTextAreaElement = class extends MockHTMLElement {}
globalThis.HTMLSelectElement = class extends MockHTMLElement {}
globalThis.CSS = { escape: (s) => s }
globalThis.NodeFilter = { SHOW_TEXT: 4 }
globalThis.InputEvent = class extends Event {}
globalThis.KeyboardEvent = class extends Event {}
globalThis.getComputedStyle = () => ({ visibility: 'visible', display: 'block' })

class MockMutationObserver {
  constructor(cb) {
    this._cb = cb
  }
  observe() {}
  disconnect() {}
}
globalThis.MutationObserver = MockMutationObserver

let perfNowValue = 0
globalThis.performance = { now: () => perfNowValue++ }

// Mock ClipboardEvent and DataTransfer for paste tests
class MockDataTransfer {
  constructor() {
    this._data = {}
  }
  setData(type, value) {
    this._data[type] = value
  }
  getData(type) {
    return this._data[type] || ''
  }
}
globalThis.DataTransfer = MockDataTransfer

class MockClipboardEvent extends Event {
  constructor(type, init = {}) {
    super(type, init)
    this.clipboardData = init.clipboardData || null
  }
}
globalThis.ClipboardEvent = MockClipboardEvent

// ---------------------------------------------------------------------------
// Import domPrimitive AFTER globals are set up
// ---------------------------------------------------------------------------
const { domPrimitive } = await import('./dom-primitives.js')

// ---------------------------------------------------------------------------
// Helper: create a mock document with a contenteditable element
// ---------------------------------------------------------------------------
function setupContentEditable(id = '#editor') {
  const el = new MockHTMLElement('DIV', {
    id: id.replace('#', ''),
    isContentEditable: true,
    textContent: '',
    innerText: ''
  })
  Object.setPrototypeOf(el, MockHTMLElement.prototype)

  // Track execCommand calls
  const commands = []
  globalThis.document = {
    querySelector: (sel) => (sel === id ? el : null),
    querySelectorAll: (sel) => (sel === id || sel === '*' ? [el] : []),
    body: {
      querySelectorAll: (sel) => (sel === id || sel === '*' ? [el] : []),
      appendChild: () => {}
    },
    documentElement: {},
    createTreeWalker: () => ({ nextNode: () => null }),
    getSelection: () => ({
      selectAllChildren: () => {},
      deleteFromDocument: () => {}
    }),
    execCommand: (cmd, _showUI, value) => {
      commands.push({ cmd, value })
      return true
    }
  }

  return { el, commands }
}

// ---------------------------------------------------------------------------
// 1. type action: multiline text in contenteditable
// ---------------------------------------------------------------------------

describe('type action: multiline contenteditable support', () => {
  beforeEach(() => {
    perfNowValue = 0
    globalThis.MutationObserver = MockMutationObserver
    globalThis.requestAnimationFrame = (cb) => cb()
  })

  test('single-line text uses insertText only (no insertParagraph)', async () => {
    const { commands } = setupContentEditable()

    const result = await domPrimitive('type', '#editor', { text: 'hello world' })

    assert.strictEqual(result.success, true)
    assert.deepStrictEqual(
      commands.map((c) => c.cmd),
      ['insertText'],
      'Single-line text should use one insertText command'
    )
    assert.strictEqual(commands[0].value, 'hello world')
  })

  test('two-line text uses insertText + insertParagraph + insertText', async () => {
    const { commands } = setupContentEditable()

    const result = await domPrimitive('type', '#editor', { text: 'line one\nline two' })

    assert.strictEqual(result.success, true)
    assert.deepStrictEqual(
      commands.map((c) => c.cmd),
      ['insertText', 'insertParagraph', 'insertText'],
      'Two-line text should split on \\n with insertParagraph between'
    )
    assert.strictEqual(commands[0].value, 'line one')
    assert.strictEqual(commands[2].value, 'line two')
  })

  test('three-line text produces two insertParagraph commands', async () => {
    const { commands } = setupContentEditable()

    await domPrimitive('type', '#editor', { text: 'a\nb\nc' })

    assert.deepStrictEqual(
      commands.map((c) => c.cmd),
      ['insertText', 'insertParagraph', 'insertText', 'insertParagraph', 'insertText'],
      'Three lines should produce text-para-text-para-text'
    )
  })

  test('consecutive newlines produce empty paragraphs', async () => {
    const { commands } = setupContentEditable()

    await domPrimitive('type', '#editor', { text: 'before\n\nafter' })

    // 'before' -> insertText, '\n' -> insertParagraph, '' (empty line) -> skip insertText,
    // '\n' -> insertParagraph, 'after' -> insertText
    assert.deepStrictEqual(
      commands.map((c) => c.cmd),
      ['insertText', 'insertParagraph', 'insertParagraph', 'insertText'],
      'Consecutive \\n\\n should produce two insertParagraph commands (empty paragraph)'
    )
  })

  test('trailing newline produces insertParagraph at end', async () => {
    const { commands } = setupContentEditable()

    await domPrimitive('type', '#editor', { text: 'hello\n' })

    assert.deepStrictEqual(
      commands.map((c) => c.cmd),
      ['insertText', 'insertParagraph'],
      'Trailing \\n should produce insertParagraph at end'
    )
  })

  test('leading newline produces insertParagraph at start', async () => {
    const { commands } = setupContentEditable()

    await domPrimitive('type', '#editor', { text: '\nhello' })

    assert.deepStrictEqual(
      commands.map((c) => c.cmd),
      ['insertParagraph', 'insertText'],
      'Leading \\n should produce insertParagraph at start'
    )
  })

  test('clear option works with multiline text', async () => {
    const { commands } = setupContentEditable()

    const result = await domPrimitive('type', '#editor', { text: 'a\nb', clear: true })

    assert.strictEqual(result.success, true)
    // Should have insertText + insertParagraph + insertText
    const cmdNames = commands.map((c) => c.cmd)
    assert.ok(cmdNames.includes('insertParagraph'), 'Should include insertParagraph even with clear')
  })
})

// ---------------------------------------------------------------------------
// 2. get_text: innerText preserves line breaks
// ---------------------------------------------------------------------------

describe('get_text: innerText for HTMLElement', () => {
  beforeEach(() => {
    perfNowValue = 0
  })

  test('get_text returns innerText for HTMLElement (preserves line breaks)', () => {
    const el = new MockHTMLElement('DIV', { id: 'content' })
    el.textContent = 'line oneline two'
    el.innerText = 'line one\nline two'
    Object.setPrototypeOf(el, MockHTMLElement.prototype)

    globalThis.document = {
      querySelector: (sel) => (sel === '#content' ? el : null),
      querySelectorAll: (sel) => (sel === '#content' || sel === '*' ? [el] : []),
      body: { querySelectorAll: (sel) => (sel === '#content' || sel === '*' ? [el] : []) },
      documentElement: {},
      createTreeWalker: () => ({ nextNode: () => null })
    }

    const result = domPrimitive('get_text', '#content', {})

    assert.strictEqual(result.success, true)
    assert.strictEqual(
      result.value,
      'line one\nline two',
      'get_text MUST use innerText (preserves \\n at block boundaries), not textContent'
    )
  })

  test('get_text uses textContent for non-HTMLElement', () => {
    // Simulate an SVG element (not an HTMLElement)
    const el = {
      tagName: 'SVG',
      textContent: 'svg text',
      getAttribute: () => null,
      closest: () => null,
      querySelector: () => null
    }
    // Don't set prototype to HTMLElement — it should NOT be instanceof HTMLElement

    globalThis.document = {
      querySelector: (sel) => (sel === '#svg' ? el : null),
      querySelectorAll: (sel) => (sel === '#svg' || sel === '*' ? [el] : []),
      body: { querySelectorAll: (sel) => (sel === '#svg' || sel === '*' ? [el] : []) },
      documentElement: {},
      createTreeWalker: () => ({ nextNode: () => null })
    }

    const result = domPrimitive('get_text', '#svg', {})

    assert.strictEqual(result.success, true)
    assert.strictEqual(result.value, 'svg text', 'Non-HTMLElement should fall back to textContent')
  })

  test('get_text returns reason when resolved text is null', () => {
    const el = new MockHTMLElement('DIV', { id: 'content' })
    el.innerText = null
    el.textContent = null
    Object.setPrototypeOf(el, MockHTMLElement.prototype)

    globalThis.document = {
      querySelector: (sel) => (sel === '#content' ? el : null),
      querySelectorAll: (sel) => (sel === '#content' || sel === '*' ? [el] : []),
      body: { querySelectorAll: (sel) => (sel === '#content' || sel === '*' ? [el] : []) },
      documentElement: {},
      createTreeWalker: () => ({ nextNode: () => null })
    }

    const result = domPrimitive('get_text', '#content', {})

    assert.strictEqual(result.success, true)
    assert.strictEqual(result.value, null)
    assert.strictEqual(result.reason, 'no_text_content')
    assert.ok(result.message.includes('null'))
  })
})

describe('null-value read actions include reason payload', () => {
  beforeEach(() => {
    perfNowValue = 0
  })

  test('get_attribute includes attribute_not_found reason when attribute is missing', () => {
    const el = new MockHTMLElement('DIV', { id: 'target' })
    Object.setPrototypeOf(el, MockHTMLElement.prototype)

    globalThis.document = {
      querySelector: (sel) => (sel === '#target' ? el : null),
      querySelectorAll: (sel) => (sel === '#target' || sel === '*' ? [el] : []),
      body: { querySelectorAll: (sel) => (sel === '#target' || sel === '*' ? [el] : []) },
      documentElement: {},
      createTreeWalker: () => ({ nextNode: () => null })
    }

    const result = domPrimitive('get_attribute', '#target', { name: 'aria-label' })

    assert.strictEqual(result.success, true)
    assert.strictEqual(result.value, null)
    assert.strictEqual(result.reason, 'attribute_not_found')
    assert.ok(result.message.includes('aria-label'))
  })

  test('get_value includes no_value reason when value is null', () => {
    const input = new globalThis.HTMLInputElement('INPUT', { id: 'field' })
    input.value = null
    Object.setPrototypeOf(input, globalThis.HTMLInputElement.prototype)

    globalThis.document = {
      querySelector: (sel) => (sel === '#field' ? input : null),
      querySelectorAll: (sel) => (sel === '#field' || sel === '*' ? [input] : []),
      body: { querySelectorAll: (sel) => (sel === '#field' || sel === '*' ? [input] : []) },
      documentElement: {},
      createTreeWalker: () => ({ nextNode: () => null })
    }

    const result = domPrimitive('get_value', '#field', {})

    assert.strictEqual(result.success, true)
    assert.strictEqual(result.value, null)
    assert.strictEqual(result.reason, 'no_value')
    assert.ok(result.message.includes('null'))
  })
})

// ---------------------------------------------------------------------------
// 3. paste action: synthetic ClipboardEvent
// ---------------------------------------------------------------------------

describe('paste action: synthetic ClipboardEvent', () => {
  beforeEach(() => {
    perfNowValue = 0
    globalThis.MutationObserver = MockMutationObserver
    globalThis.requestAnimationFrame = (cb) => cb()
  })

  test('paste dispatches ClipboardEvent with text/plain data', async () => {
    const el = new MockHTMLElement('DIV', { id: 'editor', isContentEditable: true })
    el.innerText = 'pasted content'
    Object.setPrototypeOf(el, MockHTMLElement.prototype)

    let dispatchedEvent = null
    el.dispatchEvent = (event) => {
      dispatchedEvent = event
      return true
    }

    globalThis.document = {
      querySelector: (sel) => (sel === '#editor' ? el : null),
      querySelectorAll: (sel) => (sel === '#editor' || sel === '*' ? [el] : []),
      body: { querySelectorAll: (sel) => (sel === '#editor' || sel === '*' ? [el] : []), appendChild: () => {} },
      documentElement: {},
      createTreeWalker: () => ({ nextNode: () => null }),
      getSelection: () => ({
        selectAllChildren: () => {},
        deleteFromDocument: () => {}
      })
    }

    const result = await domPrimitive('paste', '#editor', { text: 'hello\nworld' })

    assert.strictEqual(result.success, true)
    assert.ok(dispatchedEvent, 'paste should dispatch an event')
    assert.strictEqual(dispatchedEvent.type, 'paste', 'Event type should be paste')
    assert.ok(dispatchedEvent.clipboardData, 'Event should have clipboardData')
    assert.strictEqual(
      dispatchedEvent.clipboardData.getData('text/plain'),
      'hello\nworld',
      'clipboardData should contain the text'
    )
  })

  test('paste with clear selects all and deletes before pasting', async () => {
    const el = new MockHTMLElement('DIV', { id: 'editor', isContentEditable: true })
    el.innerText = 'new text'
    Object.setPrototypeOf(el, MockHTMLElement.prototype)

    let selectAllCalled = false
    let deleteCalled = false
    el.dispatchEvent = () => true

    globalThis.document = {
      querySelector: (sel) => (sel === '#editor' ? el : null),
      querySelectorAll: (sel) => (sel === '#editor' || sel === '*' ? [el] : []),
      body: { querySelectorAll: (sel) => (sel === '#editor' || sel === '*' ? [el] : []), appendChild: () => {} },
      documentElement: {},
      createTreeWalker: () => ({ nextNode: () => null }),
      getSelection: () => ({
        selectAllChildren: () => {
          selectAllCalled = true
        },
        deleteFromDocument: () => {
          deleteCalled = true
        }
      })
    }

    const result = await domPrimitive('paste', '#editor', { text: 'replacement', clear: true })

    assert.strictEqual(result.success, true)
    assert.strictEqual(selectAllCalled, true, 'clear should call selectAllChildren')
    assert.strictEqual(deleteCalled, true, 'clear should call deleteFromDocument')
  })

  test('paste fails on non-HTMLElement', async () => {
    const el = {
      tagName: 'SVG',
      textContent: 'svg',
      getAttribute: () => null,
      closest: () => null,
      querySelector: () => null
    }

    globalThis.document = {
      querySelector: (sel) => (sel === '#svg' ? el : null),
      querySelectorAll: (sel) => (sel === '#svg' || sel === '*' ? [el] : []),
      body: { querySelectorAll: (sel) => (sel === '#svg' || sel === '*' ? [el] : []), appendChild: () => {} },
      documentElement: {},
      createTreeWalker: () => ({ nextNode: () => null })
    }

    const result = await domPrimitive('paste', '#svg', { text: 'test' })

    assert.strictEqual(result.success, false)
    assert.strictEqual(result.error, 'not_interactive')
  })

  test('paste returns innerText as value', async () => {
    const el = new MockHTMLElement('DIV', { id: 'editor', isContentEditable: true })
    el.innerText = 'result text\nwith lines'
    Object.setPrototypeOf(el, MockHTMLElement.prototype)
    el.dispatchEvent = () => true

    globalThis.document = {
      querySelector: (sel) => (sel === '#editor' ? el : null),
      querySelectorAll: (sel) => (sel === '#editor' || sel === '*' ? [el] : []),
      body: { querySelectorAll: (sel) => (sel === '#editor' || sel === '*' ? [el] : []), appendChild: () => {} },
      documentElement: {},
      createTreeWalker: () => ({ nextNode: () => null }),
      getSelection: () => null
    }

    const result = await domPrimitive('paste', '#editor', { text: 'test' })

    assert.strictEqual(result.success, true)
    assert.strictEqual(result.value, 'result text\nwith lines', 'paste should return innerText')
  })
})
