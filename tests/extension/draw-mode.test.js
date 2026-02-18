// @ts-nocheck
/**
 * @fileoverview draw-mode.test.js — Tests for draw mode overlay, annotations, and export.
 * Covers activation/deactivation, annotation CRUD, DOM capture, persistence, and export.
 */

import { test, describe, mock, beforeEach, afterEach } from 'node:test'
import assert from 'node:assert'
import { MANIFEST_VERSION } from './helpers.js'

// =============================================================================
// DOM + Chrome mocks
// =============================================================================

/** Minimal mock element that supports style, events, and child management. */
function createMockElement(tag = 'div') {
  const el = {
    tagName: tag.toUpperCase(),
    id: '',
    className: '',
    classList: {
      _items: [],
      add(c) {
        this._items.push(c)
      },
      contains(c) {
        return this._items.includes(c)
      },
      [Symbol.iterator]() {
        return this._items[Symbol.iterator]()
      }
    },
    style: {},
    dataset: {},
    textContent: '',
    children: [],
    parentElement: null,
    _listeners: {},
    addEventListener(type, fn) {
      if (!this._listeners[type]) this._listeners[type] = []
      this._listeners[type].push(fn)
    },
    removeEventListener(type, fn) {
      if (this._listeners[type]) {
        this._listeners[type] = this._listeners[type].filter((f) => f !== fn)
      }
    },
    appendChild(child) {
      this.children.push(child)
      child.parentElement = this
      return child
    },
    remove() {
      if (this.parentElement) {
        this.parentElement.children = this.parentElement.children.filter((c) => c !== this)
      }
    },
    focus: mock.fn(),
    getBoundingClientRect() {
      return { x: 10, y: 20, width: 100, height: 50 }
    },
    getContext(type) {
      if (type === '2d') return createMockCanvasContext()
      return null
    },
    // For canvas
    width: 1024,
    height: 768,
    toDataURL: mock.fn(() => 'data:image/png;base64,mockdata'),
    // Dispatch for tests
    _dispatch(type, eventData = {}) {
      if (this._listeners[type]) {
        for (const fn of this._listeners[type]) fn(eventData)
      }
    }
  }
  return el
}

function createMockCanvasContext() {
  return {
    clearRect: mock.fn(),
    fillRect: mock.fn(),
    strokeRect: mock.fn(),
    fillText: mock.fn(),
    measureText: mock.fn(() => ({ width: 50 })),
    beginPath: mock.fn(),
    arc: mock.fn(),
    fill: mock.fn(),
    setLineDash: mock.fn(),
    drawImage: mock.fn(),
    fillStyle: '',
    strokeStyle: '',
    lineWidth: 1,
    font: '',
    textAlign: '',
    textBaseline: ''
  }
}

let documentBody
let createdElements
let styleEl
let storageData

function setupGlobals() {
  createdElements = []
  documentBody = createMockElement('body')
  styleEl = null
  storageData = {}

  globalThis.window = {
    innerWidth: 1024,
    innerHeight: 768,
    location: { href: 'https://example.com/page' },
    addEventListener: mock.fn(),
    removeEventListener: mock.fn(),
    getComputedStyle: mock.fn(() => ({
      getPropertyValue: mock.fn((prop) => {
        const defaults = {
          'background-color': 'rgb(59, 130, 246)',
          color: 'rgb(255, 255, 255)',
          'font-size': '14px'
        }
        return defaults[prop] || ''
      })
    }))
  }

  globalThis.document = {
    createElement: mock.fn((tag) => {
      const el = createMockElement(tag)
      createdElements.push(el)
      return el
    }),
    body: documentBody,
    documentElement: createMockElement('html'),
    head: createMockElement('head'),
    addEventListener: mock.fn(),
    removeEventListener: mock.fn(),
    createTextNode: mock.fn((text) => {
      const node = { nodeType: 3, textContent: text }
      return node
    }),
    getElementById: mock.fn((id) => {
      if (id === 'gasoline-draw-styles') return styleEl
      return null
    }),
    querySelector: mock.fn(() => null),
    elementsFromPoint: mock.fn((_x, _y) => {
      // Return a mock button element
      const btn = createMockElement('button')
      btn.classList._items = ['btn-primary']
      btn.textContent = 'Submit'
      btn.id = 'submit-btn'
      btn.parentElement = createMockElement('div')
      btn.parentElement.classList._items = ['actions']
      return [btn]
    })
  }

  // When head.appendChild is called for style element, track it
  const origAppendChild = globalThis.document.head.appendChild.bind(globalThis.document.head)
  globalThis.document.head.appendChild = (child) => {
    if (child.id === 'gasoline-draw-styles') styleEl = child
    return origAppendChild(child)
  }

  globalThis.chrome = {
    runtime: {
      sendMessage: mock.fn(() => Promise.resolve()),
      onMessage: { addListener: mock.fn() },
      getManifest: () => ({ version: MANIFEST_VERSION })
    },
    storage: {
      session: {
        get: mock.fn((keys, callback) => {
          const result = {}
          if (Array.isArray(keys)) {
            for (const k of keys) {
              if (storageData[k]) result[k] = storageData[k]
            }
          }
          if (typeof callback === 'function') callback(result)
          else return Promise.resolve(result)
        }),
        set: mock.fn((data, callback) => {
          Object.assign(storageData, data)
          if (typeof callback === 'function') callback()
          else return Promise.resolve()
        }),
        remove: mock.fn((keys) => {
          const keyList = Array.isArray(keys) ? keys : [keys]
          for (const k of keyList) delete storageData[k]
          return Promise.resolve()
        })
      }
    }
  }

  // CSS.escape mock for buildCSSSelector
  globalThis.CSS = { escape: (s) => s.replace(/([#.,:[\]()>+~'"!@])/g, '\\$1') }

  globalThis.requestAnimationFrame = mock.fn((cb) => {
    cb()
    return 1
  })
  globalThis.cancelAnimationFrame = mock.fn()
  globalThis.Image = class MockImage {
    set src(val) {
      this.width = 1024
      this.height = 768
      setTimeout(() => this.onload?.(), 0)
    }
  }
}

// =============================================================================
// Module import — fresh each test via dynamic import
// =============================================================================

/**
 * Dynamically import draw-mode.js. Each test group re-imports to get a fresh module.
 * We use a cache-busting query string so Node doesn't return the cached module.
 */
let importCounter = 0
async function importDrawMode() {
  importCounter++
  const mod = await import(`../../extension/content/draw-mode.js?v=${importCounter}`)
  return mod
}

// =============================================================================
// Tests
// =============================================================================

describe('Draw Mode — Activation/Deactivation', () => {
  let dm

  beforeEach(async () => {
    setupGlobals()
    dm = await importDrawMode()
  })

  test('activateDrawMode returns active status', () => {
    const result = dm.activateDrawMode('user')
    assert.strictEqual(result.status, 'active')
    assert.strictEqual(result.started_by, 'user')
  })

  test('isDrawModeActive returns true after activation', () => {
    dm.activateDrawMode('user')
    assert.strictEqual(dm.isDrawModeActive(), true)
  })

  test('activateDrawMode from LLM sets started_by', () => {
    const result = dm.activateDrawMode('llm')
    assert.strictEqual(result.started_by, 'llm')
  })

  test('double activation returns already_active', () => {
    dm.activateDrawMode('user')
    const result = dm.activateDrawMode('user')
    assert.strictEqual(result.status, 'already_active')
    assert.strictEqual(typeof result.annotation_count, 'number')
  })

  test('deactivateDrawMode returns results', () => {
    dm.activateDrawMode('user')
    const result = dm.deactivateDrawMode()
    assert.ok(Array.isArray(result.annotations))
    assert.ok(result.elementDetails !== undefined)
  })

  test('deactivateDrawMode when not active returns empty', () => {
    const result = dm.deactivateDrawMode()
    assert.deepStrictEqual(result.annotations, [])
  })

  test('isDrawModeActive returns false after deactivation', () => {
    dm.activateDrawMode('user')
    dm.deactivateDrawMode()
    assert.strictEqual(dm.isDrawModeActive(), false)
  })

  test('overlay is appended to document.body on activation', () => {
    dm.activateDrawMode('user')
    assert.ok(documentBody.children.length > 0, 'expected overlay to be appended')
  })

  test('overlay is removed on deactivation', () => {
    dm.activateDrawMode('user')
    assert.ok(documentBody.children.length > 0, 'overlay should exist before deactivation')
    dm.deactivateDrawMode()
    assert.strictEqual(dm.isDrawModeActive(), false, 'draw mode should be inactive after deactivation')
  })
})

describe('Draw Mode — Annotations CRUD', () => {
  let dm

  beforeEach(async () => {
    setupGlobals()
    dm = await importDrawMode()
  })

  test('getAnnotations returns empty array initially', () => {
    dm.activateDrawMode('user')
    const anns = dm.getAnnotations()
    assert.deepStrictEqual(anns, [])
  })

  test('clearAnnotations empties the list', () => {
    dm.activateDrawMode('user')
    dm.clearAnnotations()
    assert.deepStrictEqual(dm.getAnnotations(), [])
  })

  test('getElementDetail returns null for unknown correlationId', () => {
    const detail = dm.getElementDetail('nonexistent')
    assert.strictEqual(detail, null)
  })

  test('getAnnotations returns copies (not references)', () => {
    dm.activateDrawMode('user')
    const a = dm.getAnnotations()
    const b = dm.getAnnotations()
    assert.notStrictEqual(a, b) // Different array instances
  })
})

describe('Draw Mode — Export', () => {
  let compositeAnnotations

  beforeEach(async () => {
    setupGlobals()
    const mod = await import(`../../extension/content/draw-mode-export.js?v=${++importCounter}`)
    compositeAnnotations = mod.compositeAnnotations
  })

  test('returns original screenshot for empty annotations', async () => {
    const dataUrl = 'data:image/png;base64,abc'
    const result = await compositeAnnotations(dataUrl, [])
    assert.strictEqual(result, dataUrl)
  })

  test('returns original screenshot for null annotations', async () => {
    const dataUrl = 'data:image/png;base64,abc'
    const result = await compositeAnnotations(dataUrl, null)
    assert.strictEqual(result, dataUrl)
  })

  test('composites annotations onto screenshot', async () => {
    const dataUrl = 'data:image/png;base64,abc'
    const annotations = [{ rect: { x: 100, y: 200, width: 150, height: 50 }, text: 'make darker', id: 'ann_1' }]
    const result = await compositeAnnotations(dataUrl, annotations)
    // Should return a data URL (from our mock canvas.toDataURL)
    assert.ok(result.startsWith('data:image/png'))
  })
})

describe('Draw Mode — Event Handling Basics', () => {
  let dm

  beforeEach(async () => {
    setupGlobals()
    dm = await importDrawMode()
  })

  test('ESC keydown triggers deactivation and sends messages', async () => {
    dm.activateDrawMode('user')

    // Track all sendMessage calls in order
    const sentMessages = []
    globalThis.chrome.runtime.sendMessage = mock.fn((msg, callback) => {
      sentMessages.push(msg)
      if (msg.type === 'GASOLINE_CAPTURE_SCREENSHOT' && typeof callback === 'function') {
        // Simulate background returning a screenshot synchronously
        callback({ dataUrl: 'data:image/png;base64,mockscreenshot' })
      }
      return undefined
    })

    // Find the keydown listener registered on document
    const keydownCalls = globalThis.document.addEventListener.mock.calls
    const keydownHandler = keydownCalls.find((c) => c.arguments[0] === 'keydown')?.arguments[1]

    if (keydownHandler) {
      const event = {
        key: 'Escape',
        preventDefault: mock.fn(),
        stopPropagation: mock.fn()
      }
      keydownHandler(event)

      // Wait for the 300ms fade-out delay before deactivation completes
      await new Promise((r) => setTimeout(r, 350))

      // After ESC + fade, draw mode should be deactivated
      assert.strictEqual(dm.isDrawModeActive(), false, 'draw mode should be deactivated after ESC')

      // Should have sent messages (toast + capture screenshot + completed)
      assert.ok(sentMessages.length > 0, 'expected at least one sendMessage call')

      // Find the DRAW_MODE_COMPLETED message
      const completed = sentMessages.find((m) => m.type === 'DRAW_MODE_COMPLETED')
      assert.ok(completed, 'expected DRAW_MODE_COMPLETED message')
      assert.ok(Array.isArray(completed.annotations), 'expected annotations array')

      // Verify toast was sent
      const toast = sentMessages.find((m) => m.type === 'GASOLINE_ACTION_TOAST')
      assert.ok(toast, 'expected GASOLINE_ACTION_TOAST message')
      assert.strictEqual(toast.text, 'Annotations submitted')
      assert.strictEqual(toast.state, 'success')
    }
  })

  test('draw mode creates canvas element', () => {
    dm.activateDrawMode('user')
    const canvasEls = createdElements.filter((el) => el.tagName === 'CANVAS')
    assert.ok(canvasEls.length > 0, 'expected canvas to be created')
  })

  test('draw mode creates badge element', () => {
    dm.activateDrawMode('user')
    // Badge div is created as part of overlay
    assert.ok(createdElements.length >= 3, 'expected overlay, canvas, badge, and style elements')
  })
})

describe('Draw Mode — Persistence', () => {
  let dm

  beforeEach(async () => {
    setupGlobals()
    dm = await importDrawMode()
  })

  test('activateDrawMode loads annotations from storage', () => {
    dm.activateDrawMode('user')
    // chrome.storage.session.get should have been called
    const getCalls = globalThis.chrome.storage.session.get.mock.calls
    assert.ok(getCalls.length > 0, 'expected storage.session.get to be called')
  })

  test('clearAnnotations triggers persistence', (_t) => {
    dm.activateDrawMode('user')

    // Reset mock call count
    globalThis.chrome.storage.session.set.mock.resetCalls()

    dm.clearAnnotations()

    // Verify annotations were actually cleared
    const anns = dm.getAnnotations()
    assert.deepStrictEqual(anns, [], 'annotations should be empty after clearAnnotations')
  })
})

// =============================================================================
// Gap 2: Drawing Mechanics — mousedown → mousemove → mouseup → text → Enter
// =============================================================================

describe('Draw Mode — Drawing Mechanics', () => {
  let dm

  beforeEach(async () => {
    setupGlobals()
    dm = await importDrawMode()
  })

  test('mousedown + mousemove + mouseup creates text input for annotation', () => {
    dm.activateDrawMode('user')

    const overlay = documentBody.children[0]
    assert.ok(overlay, 'expected overlay element')

    overlay._dispatch('mousedown', { button: 0, clientX: 100, clientY: 100 })
    overlay._dispatch('mousemove', { clientX: 250, clientY: 200 })
    overlay._dispatch('mouseup', { clientX: 250, clientY: 200 })

    const inputEls = createdElements.filter((el) => el.tagName === 'INPUT')
    assert.ok(inputEls.length > 0, 'expected text input after drawing rectangle')
  })

  test('tiny rectangle (< 5px) does not create text input', () => {
    dm.activateDrawMode('user')
    const overlay = documentBody.children[0]

    const inputsBefore = createdElements.filter((el) => el.tagName === 'INPUT').length

    overlay._dispatch('mousedown', { button: 0, clientX: 100, clientY: 100 })
    overlay._dispatch('mousemove', { clientX: 102, clientY: 102 })
    overlay._dispatch('mouseup', { clientX: 102, clientY: 102 })

    const inputsAfter = createdElements.filter((el) => el.tagName === 'INPUT').length
    assert.strictEqual(inputsAfter, inputsBefore, 'no text input for tiny rectangle')
  })

  test('completing text input with Enter creates annotation', () => {
    dm.activateDrawMode('user')
    const overlay = documentBody.children[0]

    overlay._dispatch('mousedown', { button: 0, clientX: 100, clientY: 100 })
    overlay._dispatch('mousemove', { clientX: 250, clientY: 200 })
    overlay._dispatch('mouseup', { clientX: 250, clientY: 200 })

    const inputEl = createdElements.find((el) => el.tagName === 'INPUT')
    assert.ok(inputEl, 'expected text input element')

    inputEl.value = 'make this darker'

    const enterHandler = inputEl._listeners['keydown']?.[0]
    if (enterHandler) {
      enterHandler({
        key: 'Enter',
        preventDefault: mock.fn(),
        stopPropagation: mock.fn()
      })
    }

    const annotations = dm.getAnnotations()
    assert.strictEqual(annotations.length, 1, 'expected 1 annotation')
    assert.strictEqual(annotations[0].text, 'make this darker')
    assert.ok(annotations[0].id, 'expected annotation id')
    assert.ok(annotations[0].correlation_id, 'expected correlation_id')
    assert.ok(annotations[0].rect, 'expected rect')
    assert.strictEqual(annotations[0].rect.x, 100)
    assert.strictEqual(annotations[0].rect.y, 100)
    assert.strictEqual(annotations[0].rect.width, 150)
    assert.strictEqual(annotations[0].rect.height, 100)
  })

  test('empty text on Enter discards annotation', () => {
    dm.activateDrawMode('user')
    const overlay = documentBody.children[0]

    overlay._dispatch('mousedown', { button: 0, clientX: 100, clientY: 100 })
    overlay._dispatch('mousemove', { clientX: 250, clientY: 200 })
    overlay._dispatch('mouseup', { clientX: 250, clientY: 200 })

    const inputEl = createdElements.find((el) => el.tagName === 'INPUT')
    inputEl.value = ''
    const enterHandler = inputEl._listeners['keydown']?.[0]
    if (enterHandler) {
      enterHandler({ key: 'Enter', preventDefault: mock.fn(), stopPropagation: mock.fn() })
    }

    assert.deepStrictEqual(dm.getAnnotations(), [], 'empty text should discard annotation')
  })

  test('blur with text auto-confirms annotation', () => {
    dm.activateDrawMode('user')
    const overlay = documentBody.children[0]

    overlay._dispatch('mousedown', { button: 0, clientX: 100, clientY: 100 })
    overlay._dispatch('mousemove', { clientX: 250, clientY: 200 })
    overlay._dispatch('mouseup', { clientX: 250, clientY: 200 })

    const inputEl = createdElements.find((el) => el.tagName === 'INPUT')
    inputEl.value = 'increase padding'

    const blurHandler = inputEl._listeners['blur']?.[0]
    if (blurHandler) {
      blurHandler({})
    }

    const annotations = dm.getAnnotations()
    assert.strictEqual(annotations.length, 1, 'blur with text should auto-confirm')
    assert.strictEqual(annotations[0].text, 'increase padding')
  })

  test('right-click does not start drawing', () => {
    dm.activateDrawMode('user')
    const overlay = documentBody.children[0]

    overlay._dispatch('mousedown', { button: 2, clientX: 100, clientY: 100 })
    overlay._dispatch('mousemove', { clientX: 250, clientY: 200 })
    overlay._dispatch('mouseup', { clientX: 250, clientY: 200 })

    const inputEls = createdElements.filter((el) => el.tagName === 'INPUT')
    assert.strictEqual(inputEls.length, 0, 'right-click should not create text input')
  })

  test('reverse-direction drawing normalizes rect coordinates', () => {
    dm.activateDrawMode('user')
    const overlay = documentBody.children[0]

    // Draw from bottom-right to top-left
    overlay._dispatch('mousedown', { button: 0, clientX: 300, clientY: 300 })
    overlay._dispatch('mousemove', { clientX: 100, clientY: 100 })
    overlay._dispatch('mouseup', { clientX: 100, clientY: 100 })

    const inputEl = createdElements.find((el) => el.tagName === 'INPUT')
    assert.ok(inputEl, 'expected text input for reverse-drawn rectangle')

    inputEl.value = 'test reverse'
    const enterHandler = inputEl._listeners['keydown']?.[0]
    if (enterHandler) {
      enterHandler({ key: 'Enter', preventDefault: mock.fn(), stopPropagation: mock.fn() })
    }

    const annotations = dm.getAnnotations()
    assert.strictEqual(annotations.length, 1)
    assert.strictEqual(annotations[0].rect.x, 100, 'rect.x should be min')
    assert.strictEqual(annotations[0].rect.y, 100, 'rect.y should be min')
    assert.strictEqual(annotations[0].rect.width, 200)
    assert.strictEqual(annotations[0].rect.height, 200)
  })

  test('multiple annotations accumulate in order', () => {
    dm.activateDrawMode('user')
    const overlay = documentBody.children[0]

    // Draw first annotation
    overlay._dispatch('mousedown', { button: 0, clientX: 10, clientY: 10 })
    overlay._dispatch('mouseup', { clientX: 110, clientY: 60 })
    let inputEl = createdElements.filter((el) => el.tagName === 'INPUT').pop()
    inputEl.value = 'first'
    inputEl._listeners['keydown']?.[0]?.({ key: 'Enter', preventDefault: mock.fn(), stopPropagation: mock.fn() })

    // Draw second annotation
    overlay._dispatch('mousedown', { button: 0, clientX: 200, clientY: 200 })
    overlay._dispatch('mouseup', { clientX: 350, clientY: 300 })
    inputEl = createdElements.filter((el) => el.tagName === 'INPUT').pop()
    inputEl.value = 'second'
    inputEl._listeners['keydown']?.[0]?.({ key: 'Enter', preventDefault: mock.fn(), stopPropagation: mock.fn() })

    const annotations = dm.getAnnotations()
    assert.strictEqual(annotations.length, 2)
    assert.strictEqual(annotations[0].text, 'first')
    assert.strictEqual(annotations[1].text, 'second')
  })

  test('DOM element capture populates element_summary', () => {
    dm.activateDrawMode('user')
    const overlay = documentBody.children[0]

    overlay._dispatch('mousedown', { button: 0, clientX: 100, clientY: 100 })
    overlay._dispatch('mouseup', { clientX: 250, clientY: 200 })

    const inputEl = createdElements.find((el) => el.tagName === 'INPUT')
    inputEl.value = 'test capture'
    inputEl._listeners['keydown']?.[0]?.({ key: 'Enter', preventDefault: mock.fn(), stopPropagation: mock.fn() })

    const annotations = dm.getAnnotations()
    assert.strictEqual(annotations.length, 1)
    // elementsFromPoint mock returns a button.btn-primary 'Submit'
    assert.ok(annotations[0].element_summary.includes('button'), 'element_summary should contain tag')
  })

  test('DOM element capture stores retrievable detail via correlation_id', () => {
    dm.activateDrawMode('user')
    const overlay = documentBody.children[0]

    overlay._dispatch('mousedown', { button: 0, clientX: 100, clientY: 100 })
    overlay._dispatch('mouseup', { clientX: 250, clientY: 200 })

    const inputEl = createdElements.find((el) => el.tagName === 'INPUT')
    inputEl.value = 'test detail'
    inputEl._listeners['keydown']?.[0]?.({ key: 'Enter', preventDefault: mock.fn(), stopPropagation: mock.fn() })

    const annotations = dm.getAnnotations()
    const correlationId = annotations[0].correlation_id
    assert.ok(correlationId, 'should have correlation_id')

    const detail = dm.getElementDetail(correlationId)
    assert.ok(detail, 'should retrieve detail by correlation_id')
    assert.ok(detail.selector, 'detail should have selector')
    assert.ok(detail.tag, 'detail should have tag')
  })
})

// =============================================================================
// Gap 5: Content Script Message Routing
// =============================================================================

describe('Draw Mode — Content Script Message Routing', () => {
  let dm

  beforeEach(async () => {
    setupGlobals()
    dm = await importDrawMode()
  })

  test('GASOLINE_DRAW_MODE_START activates with correct source', () => {
    const result = dm.activateDrawMode('llm')
    assert.strictEqual(result.status, 'active')
    assert.strictEqual(result.started_by, 'llm')
    assert.strictEqual(dm.isDrawModeActive(), true)
  })

  test('GASOLINE_DRAW_MODE_STOP deactivates and returns annotations', () => {
    dm.activateDrawMode('user')
    assert.strictEqual(dm.isDrawModeActive(), true)

    const result = dm.deactivateDrawMode()
    assert.ok(Array.isArray(result.annotations))
    assert.strictEqual(dm.isDrawModeActive(), false)
  })

  test('GASOLINE_GET_ANNOTATIONS returns annotations and viewport', () => {
    dm.activateDrawMode('user')

    const response = {
      annotations: dm.getAnnotations(),
      draw_mode_active: dm.isDrawModeActive(),
      viewport: { width: globalThis.window.innerWidth, height: globalThis.window.innerHeight }
    }

    assert.ok(Array.isArray(response.annotations))
    assert.strictEqual(response.draw_mode_active, true)
    assert.strictEqual(response.viewport.width, 1024)
    assert.strictEqual(response.viewport.height, 768)
  })

  test('GASOLINE_GET_ANNOTATION_DETAIL returns null for unknown id', () => {
    assert.strictEqual(dm.getElementDetail('nonexistent'), null)
  })

  test('GASOLINE_CLEAR_ANNOTATIONS empties annotation list', () => {
    dm.activateDrawMode('user')
    dm.clearAnnotations()
    assert.deepStrictEqual(dm.getAnnotations(), [])
  })

  test('draw mode start while active returns already_active', () => {
    dm.activateDrawMode('llm')
    const second = dm.activateDrawMode('llm')
    assert.strictEqual(second.status, 'already_active')
    assert.strictEqual(typeof second.annotation_count, 'number')
  })
})

// =============================================================================
// State leak prevention (#9)
// =============================================================================
describe('Draw Mode — State Leak Prevention', () => {
  let dm

  beforeEach(async () => {
    setupGlobals()
    dm = await importDrawMode()
  })

  afterEach(() => {
    if (dm?.isDrawModeActive()) dm.deactivateDrawMode()
  })

  test('deactivateDrawMode clears annotations for next session', () => {
    dm.activateDrawMode('user')

    // Simulate adding annotation data via the public API
    // First session
    dm.deactivateDrawMode()

    // Second activation should start clean
    dm.activateDrawMode('user')
    const anns = dm.getAnnotations()
    assert.strictEqual(anns.length, 0, 'Annotations should be empty after re-activation')
  })

  test('deactivateDrawMode clears elementDetails for next session', () => {
    dm.activateDrawMode('user')
    dm.deactivateDrawMode()

    // After deactivation, getElementDetail for any old ID should return null
    dm.activateDrawMode('user')
    assert.strictEqual(dm.getElementDetail('old_correlation_id'), null)
  })
})

// =============================================================================
// Deactivation failure paths (#22)
// =============================================================================
describe('Draw Mode — Deactivation Failure Paths', () => {
  let dm

  beforeEach(async () => {
    setupGlobals()
    dm = await importDrawMode()
  })

  afterEach(() => {
    if (dm?.isDrawModeActive()) dm.deactivateDrawMode()
  })

  test('deactivateDrawMode succeeds even when chrome.runtime.sendMessage throws', () => {
    dm.activateDrawMode('user')
    assert.strictEqual(dm.isDrawModeActive(), true)

    // Make sendMessage throw
    globalThis.chrome.runtime.sendMessage = mock.fn(() => {
      throw new Error('Extension context invalidated')
    })

    // Direct deactivation should still work
    const result = dm.deactivateDrawMode()
    assert.strictEqual(dm.isDrawModeActive(), false)
    assert.ok(Array.isArray(result.annotations))
  })

  test('deactivateDrawMode returns results even when not active', () => {
    // Not active → should return empty result without error
    const result = dm.deactivateDrawMode()
    assert.deepStrictEqual(result.annotations, [])
    assert.deepStrictEqual(result.elementDetails, {})
  })

  test('clearAnnotations works when draw mode is active', () => {
    dm.activateDrawMode('user')
    dm.clearAnnotations()
    assert.deepStrictEqual(dm.getAnnotations(), [])
    assert.strictEqual(dm.isDrawModeActive(), true, 'Should still be active after clear')
  })
})

// =============================================================================
// Session name (#18 multi-page sessions)
// =============================================================================
describe('Draw Mode — Session Name', () => {
  let dm

  beforeEach(async () => {
    setupGlobals()
    dm = await importDrawMode()
  })

  afterEach(() => {
    if (dm?.isDrawModeActive()) dm.deactivateDrawMode()
  })

  test('activateDrawMode accepts session name', () => {
    const result = dm.activateDrawMode('llm', 'qa-review')
    assert.strictEqual(result.status, 'active')
    assert.strictEqual(dm.isDrawModeActive(), true)
  })

  test('session name cleared on deactivation', () => {
    dm.activateDrawMode('llm', 'qa-review')
    dm.deactivateDrawMode()
    // Reactivate without session name — should not carry over
    dm.activateDrawMode('user')
    assert.strictEqual(dm.isDrawModeActive(), true)
  })
})

// =============================================================================
// A11y checks (#19 accessibility auto-enrichment)
// =============================================================================
describe('Draw Mode — A11y Auto-Enrichment', () => {
  let dm

  beforeEach(async () => {
    setupGlobals()
    dm = await importDrawMode()
  })

  afterEach(() => {
    if (dm?.isDrawModeActive()) dm.deactivateDrawMode()
  })

  test('DOM capture produces element detail with a11y_flags field', () => {
    dm.activateDrawMode('user')
    const overlay = documentBody.children[0]
    assert.ok(overlay, 'expected overlay element')

    // Draw a rectangle to trigger DOM capture + a11y enrichment
    overlay._dispatch('mousedown', { button: 0, clientX: 100, clientY: 100 })
    overlay._dispatch('mouseup', { clientX: 250, clientY: 200 })

    const inputEl = createdElements.find((el) => el.tagName === 'INPUT')
    inputEl.value = 'a11y test'
    inputEl._listeners['keydown']?.[0]?.({ key: 'Enter', preventDefault: mock.fn(), stopPropagation: mock.fn() })

    const annotations = dm.getAnnotations()
    assert.strictEqual(annotations.length, 1, 'expected 1 annotation')

    const detail = dm.getElementDetail(annotations[0].correlation_id)
    assert.ok(detail, 'should retrieve detail by correlation_id')
    assert.ok(Array.isArray(detail.a11y_flags), 'a11y_flags should be an array')
  })
})

// =============================================================================
// Re-entry guard and timeout fallback (#36)
// =============================================================================
describe('Draw Mode — deactivateAndSendResults re-entry guard', () => {
  let dm

  beforeEach(async () => {
    setupGlobals()
    dm = await importDrawMode()
  })

  afterEach(() => {
    if (dm?.isDrawModeActive()) dm.deactivateDrawMode()
  })

  test('deactivateAndSendResults is exported', () => {
    assert.strictEqual(typeof dm.deactivateAndSendResults, 'function')
  })

  test('deactivateAndSendResults does nothing when not active', () => {
    // Should not throw
    dm.deactivateAndSendResults()
    assert.strictEqual(dm.isDrawModeActive(), false)
  })

  test('double call does not throw (re-entry guard)', async () => {
    dm.activateDrawMode('user')

    // Track sendMessage calls
    const sendCalls = []
    globalThis.chrome.runtime.sendMessage = mock.fn((...args) => {
      sendCalls.push(args)
      // Simulate async callback for GASOLINE_CAPTURE_SCREENSHOT
      const callback = args[1]
      if (typeof callback === 'function') {
        callback({ dataUrl: '' })
      }
    })

    dm.deactivateAndSendResults()
    // Second call should be a no-op (re-entry guard)
    dm.deactivateAndSendResults()

    // Wait for the 300ms fade-out delay before deactivation completes
    await new Promise((r) => setTimeout(r, 350))
    assert.strictEqual(dm.isDrawModeActive(), false)
  })
})
