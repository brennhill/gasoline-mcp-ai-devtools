// @ts-nocheck
/**
 * @fileoverview content-ui.test.js — Unit tests for content/ui/toast.js and content/ui/subtitle.js.
 * Tests DOM-manipulating overlay modules with lightweight mock document/chrome globals.
 */

import { test, describe, mock, beforeEach } from 'node:test'
import assert from 'node:assert'

// ---------------------------------------------------------------------------
// DOM + Chrome mocks
// ---------------------------------------------------------------------------

/** Registry of elements created via createElement or returned by getElementById */
let elements
let appendedToBody
let appendedToHead
let eventListeners

function createMockElement(tag) {
  const el = {
    tag,
    id: '',
    textContent: '',
    className: '',
    src: '',
    style: {},
    offsetHeight: 0,
    children: [],
    appendChild: mock.fn((child) => el.children.push(child)),
    remove: mock.fn(),
    addEventListener: mock.fn((type, handler) => {
      if (!eventListeners[type]) eventListeners[type] = []
      eventListeners[type].push(handler)
    }),
    setAttribute: mock.fn()
  }
  return el
}

function resetMocks() {
  elements = {}
  appendedToBody = []
  appendedToHead = []
  eventListeners = {}

  globalThis.document = {
    getElementById: mock.fn((id) => elements[id] || null),
    createElement: mock.fn((tag) => {
      const el = createMockElement(tag)
      return el
    }),
    head: { appendChild: mock.fn((el) => appendedToHead.push(el)) },
    body: { appendChild: mock.fn((el) => appendedToBody.push(el)) },
    documentElement: { appendChild: mock.fn() },
    addEventListener: mock.fn((type, handler) => {
      if (!eventListeners[type]) eventListeners[type] = []
      eventListeners[type].push(handler)
    }),
    removeEventListener: mock.fn()
  }

  globalThis.chrome = {
    runtime: {
      getURL: mock.fn((path) => `chrome-extension://test-id/${path}`)
    }
  }

  globalThis.requestAnimationFrame = (cb) => cb()
}

// ---------------------------------------------------------------------------
// Toast tests
// ---------------------------------------------------------------------------

describe('showActionToast', () => {
  let showActionToast

  beforeEach(async () => {
    mock.reset()
    resetMocks()
    ;({ showActionToast } = await import('../../extension/content/ui/toast.js'))
  })

  test('creates a toast element with correct ID', () => {
    showActionToast('Hello')

    const toast = appendedToBody.find((el) => el.id === 'gasoline-action-toast')
    assert.ok(toast, 'toast element should be appended to body')
    assert.strictEqual(toast.id, 'gasoline-action-toast')
  })

  test('applies trying theme by default', () => {
    showActionToast('Clicking')

    const toast = appendedToBody.find((el) => el.id === 'gasoline-action-toast')
    assert.ok(toast)
    assert.ok(toast.style.background.includes('#3b82f6'), 'should use trying gradient')
  })

  test('applies error theme when state=error', () => {
    showActionToast('Failed', undefined, 'error')

    const toast = appendedToBody.find((el) => el.id === 'gasoline-action-toast')
    assert.ok(toast)
    assert.ok(toast.style.background.includes('#ef4444'), 'should use error gradient')
  })

  test('applies success theme when state=success', () => {
    showActionToast('Done', undefined, 'success')

    const toast = appendedToBody.find((el) => el.id === 'gasoline-action-toast')
    assert.ok(toast)
    assert.ok(toast.style.background.includes('#22c55e'), 'should use success gradient')
  })

  test('truncates text longer than 30 characters', () => {
    const longText = 'A'.repeat(35)
    showActionToast(longText)

    // The label span is the first child appended to the toast
    const toast = appendedToBody.find((el) => el.id === 'gasoline-action-toast')
    assert.ok(toast)
    // Find the label span among children appended to toast
    const label = toast.appendChild.mock.calls
      .map((c) => c.arguments[0])
      .find((child) => child.tag === 'span' && child.style.fontWeight === '700')
    assert.ok(label, 'should have a label span')
    assert.ok(label.textContent.length <= 30, `label should be truncated, got ${label.textContent.length} chars`)
    assert.ok(label.textContent.endsWith('\u2026'), 'should end with ellipsis')
  })

  test('truncates detail longer than 50 characters', () => {
    const longDetail = 'B'.repeat(55)
    showActionToast('Short', longDetail)

    const toast = appendedToBody.find((el) => el.id === 'gasoline-action-toast')
    assert.ok(toast)
    // Detail span has fontWeight '400' and opacity '0.9'
    const detail = toast.appendChild.mock.calls
      .map((c) => c.arguments[0])
      .find((child) => child.tag === 'span' && child.style.opacity === '0.9')
    assert.ok(detail, 'should have a detail span')
    assert.ok(detail.textContent.length <= 50, `detail should be truncated, got ${detail.textContent.length} chars`)
    assert.ok(detail.textContent.endsWith('\u2026'), 'should end with ellipsis')
  })

  test('does not truncate short text', () => {
    showActionToast('Click', 'Submit')

    const toast = appendedToBody.find((el) => el.id === 'gasoline-action-toast')
    const label = toast.appendChild.mock.calls
      .map((c) => c.arguments[0])
      .find((child) => child.tag === 'span' && child.style.fontWeight === '700')
    assert.strictEqual(label.textContent, 'Click')
  })

  test('removes existing toast before creating new one', () => {
    const existingToast = createMockElement('div')
    existingToast.id = 'gasoline-action-toast'
    elements['gasoline-action-toast'] = existingToast

    showActionToast('New toast')

    assert.strictEqual(existingToast.remove.mock.calls.length, 1, 'existing toast should be removed')
  })

  test('injects animation styles only once', () => {
    showActionToast('First')

    // After first call, the style element should be appended to head
    const styleEl = appendedToHead.find((el) => el.id === 'gasoline-toast-animations')
    assert.ok(styleEl, 'animation style should be injected')

    // Now simulate that the style element exists
    elements['gasoline-toast-animations'] = styleEl
    const headCallsBefore = appendedToHead.length

    showActionToast('Second')

    assert.strictEqual(appendedToHead.length, headCallsBefore, 'should not inject styles again')
  })

  test('audio state adds pulse class and icon', () => {
    showActionToast('Recording', undefined, 'audio')

    const toast = appendedToBody.find((el) => el.id === 'gasoline-action-toast')
    assert.ok(toast)
    assert.strictEqual(toast.className, 'gasoline-toast-pulse')

    // Should have created an img element for the icon
    const imgChild = toast.appendChild.mock.calls
      .map((c) => c.arguments[0])
      .find((child) => child.tag === 'img')
    assert.ok(imgChild, 'audio toast should have an icon image')
    assert.ok(imgChild.src.includes('icon-48.png'), 'icon should reference icon-48.png')
  })

  test('non-audio state does not add pulse class', () => {
    showActionToast('Click', undefined, 'trying')

    const toast = appendedToBody.find((el) => el.id === 'gasoline-action-toast')
    assert.strictEqual(toast.className, '', 'non-audio toast should not have pulse class')
  })

  test('auto-removes after duration via setTimeout', () => {
    const origSetTimeout = globalThis.setTimeout
    const timeoutCalls = []
    globalThis.setTimeout = mock.fn((cb, ms) => {
      timeoutCalls.push({ cb, ms })
      return 1
    })

    try {
      showActionToast('Quick', undefined, 'trying', 2000)

      // Should schedule a timeout for the given duration
      const fadeOut = timeoutCalls.find((t) => t.ms === 2000)
      assert.ok(fadeOut, 'should schedule fade-out at durationMs')
    } finally {
      globalThis.setTimeout = origSetTimeout
    }
  })
})

// ---------------------------------------------------------------------------
// Subtitle tests
// ---------------------------------------------------------------------------

describe('showSubtitle', () => {
  let showSubtitle, clearSubtitle // eslint-disable-line no-unused-vars

  beforeEach(async () => {
    mock.reset()
    resetMocks()
    ;({ showSubtitle, clearSubtitle } = await import('../../extension/content/ui/subtitle.js'))
  })

  test('creates subtitle element and appends to body', () => {
    showSubtitle('Opening page')

    const bar = appendedToBody.find((el) => el.id === 'gasoline-subtitle')
    assert.ok(bar, 'subtitle bar should be appended to body')
  })

  test('sets text content on the subtitle bar', () => {
    showSubtitle('Navigating to settings')

    const bar = appendedToBody.find((el) => el.id === 'gasoline-subtitle')
    assert.ok(bar)
    assert.strictEqual(bar.textContent, 'Navigating to settings')
  })

  test('reuses existing subtitle element', () => {
    const existingBar = createMockElement('div')
    existingBar.id = 'gasoline-subtitle'
    existingBar.style = { opacity: '0' }
    elements['gasoline-subtitle'] = existingBar

    showSubtitle('Updated text')

    // Should not create a new element appended to body
    assert.strictEqual(appendedToBody.length, 0, 'should reuse existing bar, not create new one')
    assert.strictEqual(existingBar.textContent, 'Updated text')
  })

  test('empty text clears subtitle', () => {
    // Set up an existing subtitle element
    const bar = createMockElement('div')
    bar.id = 'gasoline-subtitle'
    bar.style = { opacity: '1' }
    elements['gasoline-subtitle'] = bar

    showSubtitle('')

    assert.strictEqual(bar.style.opacity, '0', 'should fade out subtitle on empty text')
  })

  test('registers keydown listener for Escape dismissal', () => {
    showSubtitle('Press Escape')

    const keydownListeners = globalThis.document.addEventListener.mock.calls
      .filter((c) => c.arguments[0] === 'keydown')
    assert.ok(keydownListeners.length > 0, 'should register keydown listener')
  })

  test('fades in by setting opacity to 1', () => {
    showSubtitle('Hello')

    const bar = appendedToBody.find((el) => el.id === 'gasoline-subtitle')
    assert.ok(bar)
    assert.strictEqual(bar.style.opacity, '1', 'should set opacity to 1 for fade-in')
  })
})

describe('clearSubtitle', () => {
  let clearSubtitle, showSubtitle

  beforeEach(async () => {
    mock.reset()
    resetMocks()
    ;({ clearSubtitle, showSubtitle } = await import('../../extension/content/ui/subtitle.js'))
  })

  test('sets opacity to 0 on existing subtitle element', () => {
    const bar = createMockElement('div')
    bar.id = 'gasoline-subtitle'
    bar.style = { opacity: '1' }
    elements['gasoline-subtitle'] = bar

    clearSubtitle()

    assert.strictEqual(bar.style.opacity, '0', 'should fade out the subtitle')
  })

  test('removes keydown listener on clear', () => {
    // First show a subtitle to register the escape handler
    showSubtitle('Temporary')

    clearSubtitle()

    const removeCalls = globalThis.document.removeEventListener.mock.calls
      .filter((c) => c.arguments[0] === 'keydown')
    assert.ok(removeCalls.length > 0, 'should remove keydown listener on clear')
  })

  test('handles missing subtitle element gracefully', () => {
    // No subtitle element exists
    assert.doesNotThrow(() => clearSubtitle(), 'should not throw when no subtitle exists')
  })
})

describe('toggleRecordingWatermark', () => {
  let toggleRecordingWatermark

  beforeEach(async () => {
    mock.reset()
    resetMocks()
    ;({ toggleRecordingWatermark } = await import('../../extension/content/ui/subtitle.js'))
  })

  test('creates watermark element when visible=true', () => {
    toggleRecordingWatermark(true)

    const watermark = appendedToBody.find((el) => el.id === 'gasoline-recording-watermark')
    assert.ok(watermark, 'watermark should be appended to body')
  })

  test('watermark contains an img with icon.svg', () => {
    toggleRecordingWatermark(true)

    const watermark = appendedToBody.find((el) => el.id === 'gasoline-recording-watermark')
    assert.ok(watermark)
    const img = watermark.appendChild.mock.calls
      .map((c) => c.arguments[0])
      .find((child) => child.tag === 'img')
    assert.ok(img, 'watermark should contain an img element')
    assert.ok(img.src.includes('icon.svg'), 'img should reference icon.svg')
  })

  test('does not create duplicate watermark', () => {
    const existing = createMockElement('div')
    existing.id = 'gasoline-recording-watermark'
    elements['gasoline-recording-watermark'] = existing

    toggleRecordingWatermark(true)

    assert.strictEqual(appendedToBody.length, 0, 'should not create duplicate watermark')
  })

  test('fades out and removes watermark when visible=false', () => {
    const existing = createMockElement('div')
    existing.id = 'gasoline-recording-watermark'
    existing.style = { opacity: '1' }
    elements['gasoline-recording-watermark'] = existing

    toggleRecordingWatermark(false)

    assert.strictEqual(existing.style.opacity, '0', 'should fade out watermark')
  })

  test('handles missing watermark gracefully on hide', () => {
    assert.doesNotThrow(() => toggleRecordingWatermark(false), 'should not throw when no watermark exists')
  })
})
