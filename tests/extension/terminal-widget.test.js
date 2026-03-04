// @ts-nocheck
/**
 * @fileoverview terminal-widget.test.js — Regression coverage for terminal header controls.
 */

import { beforeEach, describe, mock, test } from 'node:test'
import assert from 'node:assert'
import { StorageKey } from '../../extension/lib/constants.js'

let importCounter = 0
let localStorageData = {}
let sessionStorageData = {}
let fetchCalls = []
let fetchHandler = null
let roots = []
let windowListeners = {}

function makeResponse(status, body) {
  return {
    ok: status >= 200 && status < 300,
    status,
    json: async () => body
  }
}

function walkTree(node, visit) {
  for (const child of node.children || []) {
    if (visit(child)) return child
    const found = walkTree(child, visit)
    if (found) return found
  }
  return null
}

function matchSelector(el, selector) {
  if (selector.startsWith('#')) return el.id === selector.slice(1)
  if (selector.startsWith('.')) {
    const cls = selector.slice(1)
    return String(el.className || '')
      .split(/\s+/)
      .filter(Boolean)
      .includes(cls)
  }
  return String(el.tagName || '').toLowerCase() === selector.toLowerCase()
}

function querySelectorWithin(node, selector) {
  return walkTree(node, (child) => matchSelector(child, selector))
}

function getElementById(id) {
  for (const root of roots) {
    if (root.id === id) return root
    const found = walkTree(root, (child) => child.id === id)
    if (found) return found
  }
  return null
}

function createElement(tag) {
  const listeners = {}
  const el = {
    tagName: String(tag).toUpperCase(),
    id: '',
    className: '',
    textContent: '',
    title: '',
    type: '',
    style: {},
    children: [],
    parentElement: null,
    attributes: {},
    dataset: {},
    offsetWidth: 800,
    offsetHeight: 400,
    appendChild: mock.fn((child) => {
      child.parentElement = el
      el.children.push(child)
      return child
    }),
    remove: mock.fn(() => {
      if (!el.parentElement) return
      const siblings = el.parentElement.children || []
      const idx = siblings.indexOf(el)
      if (idx >= 0) siblings.splice(idx, 1)
      el.parentElement = null
    }),
    addEventListener: mock.fn((type, handler) => {
      listeners[type] = handler
    }),
    setAttribute: mock.fn((name, value) => {
      el.attributes[name] = value
    }),
    querySelector: mock.fn((selector) => querySelectorWithin(el, selector)),
    dispatch: (type, event = {}) => {
      const handler = listeners[type]
      if (!handler) return
      handler({
        preventDefault() {},
        stopPropagation() {},
        clientX: 0,
        clientY: 0,
        ...event
      })
    }
  }

  if (tag === 'iframe') {
    el.contentWindow = { postMessage: mock.fn() }
  }

  return el
}

function dispatchWindowEvent(type, event = {}) {
  const handlers = windowListeners[type] || []
  for (const handler of handlers) handler(event)
}

function getPostMessagePayloads(iframe, startAt = 0) {
  const calls = iframe?.contentWindow?.postMessage?.mock?.calls || []
  return calls.slice(startAt).map((call) => call.arguments[0])
}

function sleep(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms))
}

function setupEnvironment() {
  roots = []
  fetchCalls = []
  fetchHandler = null
  windowListeners = {}

  const body = createElement('body')
  const head = createElement('head')
  const documentElement = createElement('html')
  roots.push(body, head, documentElement)

  globalThis.document = {
    body,
    head,
    documentElement,
    createElement: mock.fn((tag) => createElement(tag)),
    getElementById: mock.fn((id) => getElementById(id)),
    addEventListener: mock.fn(),
    removeEventListener: mock.fn()
  }

  globalThis.window = {
    addEventListener: mock.fn((type, handler) => {
      if (!windowListeners[type]) windowListeners[type] = []
      windowListeners[type].push(handler)
    }),
    removeEventListener: mock.fn((type, handler) => {
      if (!windowListeners[type]) return
      windowListeners[type] = windowListeners[type].filter((item) => item !== handler)
    }),
    innerWidth: 1600,
    innerHeight: 900
  }

  const clipboard = { writeText: mock.fn(() => Promise.resolve()) }
  if (!globalThis.navigator) {
    Object.defineProperty(globalThis, 'navigator', {
      value: { clipboard },
      configurable: true
    })
  } else {
    Object.defineProperty(globalThis.navigator, 'clipboard', {
      value: clipboard,
      configurable: true
    })
  }

  globalThis.requestAnimationFrame = (cb) => cb()

  globalThis.fetch = mock.fn(async (url, options = {}) => {
    const call = { url: String(url), options }
    fetchCalls.push(call)
    if (!fetchHandler) throw new Error('fetchHandler is not configured')
    return fetchHandler(call)
  })

  globalThis.chrome = {
    runtime: {
      lastError: null
    },
    storage: {
      local: {
        get: mock.fn((keys, callback) => {
          const keyList = Array.isArray(keys) ? keys : [keys]
          const result = {}
          for (const key of keyList) result[key] = localStorageData[key]
          callback(result)
        }),
        set: mock.fn((payload, callback) => {
          localStorageData = { ...localStorageData, ...(payload || {}) }
          callback?.()
        }),
        remove: mock.fn((keys, callback) => {
          const keyList = Array.isArray(keys) ? keys : [keys]
          for (const key of keyList) delete localStorageData[key]
          callback?.()
        })
      },
      session: {
        get: mock.fn((keys, callback) => {
          const keyList = Array.isArray(keys) ? keys : [keys]
          const result = {}
          for (const key of keyList) result[key] = sessionStorageData[key]
          callback(result)
        }),
        set: mock.fn((payload, callback) => {
          sessionStorageData = { ...sessionStorageData, ...(payload || {}) }
          callback?.()
        }),
        remove: mock.fn((keys, callback) => {
          const keyList = Array.isArray(keys) ? keys : [keys]
          for (const key of keyList) delete sessionStorageData[key]
          callback?.()
        })
      }
    }
  }
}

function findButton(root, predicate) {
  if (!root) return null
  return walkTree(root, (node) => node.tagName === 'BUTTON' && predicate(node))
}

describe('terminal widget header controls', () => {
  beforeEach(() => {
    mock.reset()
    localStorageData = { [StorageKey.SERVER_URL]: 'http://localhost:7890' }
    sessionStorageData = {}
    setupEnvironment()
  })

  test('restoring minimized session preserves power button icon and tooltip', async () => {
    sessionStorageData = {
      [StorageKey.TERMINAL_SESSION]: { sessionId: 'session-1', token: 'token-1' },
      [StorageKey.TERMINAL_UI_STATE]: 'minimized'
    }

    fetchHandler = ({ url }) => {
      if (url.includes('/terminal/validate?token=')) {
        return Promise.resolve(makeResponse(200, { valid: true }))
      }
      throw new Error(`Unexpected fetch call: ${url}`)
    }

    const module = await import(`../../extension/content/ui/terminal-widget.js?v=${++importCounter}`)
    await module.restoreTerminalIfNeeded()

    const header = getElementById('gasoline-terminal-header')
    assert.ok(header, 'terminal header should be mounted')

    const powerButton = findButton(header, (node) => node.title === 'disconnect terminal & and end session')
    const minimizeButton = findButton(header, (node) => node.title === 'Restore terminal')

    assert.ok(powerButton, 'power button should exist')
    assert.strictEqual(powerButton.textContent, '\u23FB', 'power icon should remain visible')
    assert.strictEqual(powerButton.title, 'disconnect terminal & and end session')

    assert.ok(minimizeButton, 'minimize button should exist')
    assert.strictEqual(minimizeButton.textContent, '\u25A1', 'minimize button should become restore icon when minimized')
    assert.strictEqual(minimizeButton.title, 'Restore terminal')
  })

  test('power button click ends current session and next toggle starts a fresh CLI session', async () => {
    let startCount = 0
    const stopBodies = []

    fetchHandler = ({ url, options }) => {
      if (url.endsWith('/terminal/start')) {
        startCount += 1
        return Promise.resolve(makeResponse(200, {
          session_id: `session-${startCount}`,
          token: `token-${startCount}`,
          pid: 999
        }))
      }
      if (url.endsWith('/terminal/stop')) {
        stopBodies.push(JSON.parse(String(options.body || '{}')))
        return Promise.resolve(makeResponse(200, { ok: true }))
      }
      if (url.includes('/terminal/validate?token=')) {
        return Promise.resolve(makeResponse(200, { valid: false }))
      }
      throw new Error(`Unexpected fetch call: ${url}`)
    }

    const module = await import(`../../extension/content/ui/terminal-widget.js?v=${++importCounter}`)

    await module.toggleTerminal()
    assert.strictEqual(startCount, 1, 'first toggle should start one CLI session')

    const header = getElementById('gasoline-terminal-header')
    const powerButton = findButton(header, (node) => node.title === 'disconnect terminal & and end session')
    assert.ok(powerButton, 'power button should be present after opening terminal')
    assert.strictEqual(powerButton.title, 'disconnect terminal & and end session')

    powerButton.dispatch('click')
    await new Promise((resolve) => setTimeout(resolve, 0))

    assert.strictEqual(stopBodies.length, 1, 'power click should stop the active session')
    assert.deepStrictEqual(stopBodies[0], { id: 'session-1' }, 'stop call should target current session id')

    await module.toggleTerminal()
    assert.strictEqual(startCount, 2, 'reopening after exit should start a brand-new CLI session')
  })

  test('redraw button resets geometry and reloads iframe without restarting the session', async () => {
    let startCount = 0

    fetchHandler = ({ url }) => {
      if (url.endsWith('/terminal/start')) {
        startCount += 1
        return Promise.resolve(makeResponse(200, {
          session_id: `session-${startCount}`,
          token: `token-${startCount}`,
          pid: 999
        }))
      }
      if (url.includes('/terminal/validate?token=')) {
        return Promise.resolve(makeResponse(200, { valid: true }))
      }
      throw new Error(`Unexpected fetch call: ${url}`)
    }

    const module = await import(`../../extension/content/ui/terminal-widget.js?v=${++importCounter}`)
    await module.toggleTerminal()

    const widget = getElementById('gasoline-terminal-widget')
    const header = getElementById('gasoline-terminal-header')
    const iframe = getElementById('gasoline-terminal-iframe')
    const redrawButton = findButton(header, (node) => node.title === 'Redraw terminal graphics')
    const minimizeButton = findButton(header, (node) => node.title === 'Minimize terminal')

    assert.ok(widget, 'terminal widget should exist')
    assert.ok(header, 'terminal header should exist')
    assert.ok(iframe, 'terminal iframe should exist')
    assert.ok(redrawButton, 'redraw button should exist')
    assert.ok(minimizeButton, 'minimize button should exist')

    widget.style.width = '92vw'
    widget.style.height = '74vh'
    widget.style.minHeight = '360px'
    widget.style.transform = 'translateY(20px) scale(0.98)'
    widget.style.pointerEvents = 'none'
    const priorSrc = iframe.src

    redrawButton.dispatch('click')

    assert.strictEqual(widget.style.width, '50vw')
    assert.strictEqual(widget.style.height, '40vh')
    assert.strictEqual(widget.style.minHeight, '250px')
    assert.strictEqual(widget.style.transform, 'translateY(0) scale(1)')
    assert.strictEqual(widget.style.pointerEvents, 'auto')
    assert.strictEqual(iframe.src, priorSrc, 'redraw should reload same iframe URL for the active token')
    assert.strictEqual(minimizeButton.title, 'Minimize terminal')
    assert.strictEqual(sessionStorageData[StorageKey.TERMINAL_UI_STATE], 'open')
    assert.strictEqual(startCount, 1, 'redraw should not start a new terminal session')
  })

  test('writeToTerminal waits while user is typing and flushes after focus clears', async () => {
    fetchHandler = ({ url }) => {
      if (url.endsWith('/terminal/start')) {
        return Promise.resolve(makeResponse(200, {
          session_id: 'session-typing-guard',
          token: 'token-typing-guard',
          pid: 999
        }))
      }
      throw new Error(`Unexpected fetch call: ${url}`)
    }

    const module = await import(`../../extension/content/ui/terminal-widget.js?v=${++importCounter}`)
    await module.toggleTerminal()

    const iframe = getElementById('gasoline-terminal-iframe')
    assert.ok(iframe, 'terminal iframe should exist')

    dispatchWindowEvent('message', {
      origin: 'http://localhost:7891',
      data: { source: 'gasoline-terminal', event: 'connected' }
    })
    const statusDot = getElementById('gasoline-terminal-widget')?.querySelector('.gasoline-terminal-status-dot')
    assert.strictEqual(statusDot?.style?.background, '#9ece6a', 'connected message should update terminal status dot')
    dispatchWindowEvent('message', {
      origin: 'http://localhost:7891',
      data: { source: 'gasoline-terminal', event: 'focus', data: { focused: true } }
    })
    dispatchWindowEvent('message', {
      origin: 'http://localhost:7891',
      data: { source: 'gasoline-terminal', event: 'typing', data: { at: Date.now() } }
    })

    const callStart = iframe.contentWindow.postMessage.mock.calls.length
    module.writeToTerminal('queued command')

    await sleep(80)
    const whileTypingPayloads = getPostMessagePayloads(iframe, callStart)
    const whileTypingWrites = whileTypingPayloads.filter((payload) => payload?.command === 'write')
    assert.strictEqual(whileTypingWrites.length, 0, 'terminal write should stay queued while user is actively typing')

    dispatchWindowEvent('message', {
      origin: 'http://localhost:7891',
      data: { source: 'gasoline-terminal', event: 'focus', data: { focused: false } }
    })

    await sleep(800)
    const flushedPayloads = getPostMessagePayloads(iframe, callStart)
    const flushedWrites = flushedPayloads
      .filter((payload) => payload?.command === 'write')
      .map((payload) => payload.text)

    assert.deepStrictEqual(flushedWrites, ['queued command', '\r'])
    assert.ok(
      flushedPayloads.some((payload) => payload?.command === 'focus'),
      'focus should return to xterm after queued write submission'
    )
  })

  test('terminal submit re-guards if user retakes focus before auto-enter', async () => {
    fetchHandler = ({ url }) => {
      if (url.endsWith('/terminal/start')) {
        return Promise.resolve(makeResponse(200, {
          session_id: 'session-re-guard',
          token: 'token-re-guard',
          pid: 999
        }))
      }
      throw new Error(`Unexpected fetch call: ${url}`)
    }

    const module = await import(`../../extension/content/ui/terminal-widget.js?v=${++importCounter}`)
    await module.toggleTerminal()

    const iframe = getElementById('gasoline-terminal-iframe')
    assert.ok(iframe, 'terminal iframe should exist')

    dispatchWindowEvent('message', {
      origin: 'http://localhost:7891',
      data: { source: 'gasoline-terminal', event: 'connected' }
    })
    const statusDot = getElementById('gasoline-terminal-widget')?.querySelector('.gasoline-terminal-status-dot')
    assert.strictEqual(statusDot?.style?.background, '#9ece6a', 'connected message should update terminal status dot')
    dispatchWindowEvent('message', {
      origin: 'http://localhost:7891',
      data: { source: 'gasoline-terminal', event: 'focus', data: { focused: false } }
    })

    const callStart = iframe.contentWindow.postMessage.mock.calls.length
    module.writeToTerminal('submit guard command')

    await sleep(80)
    dispatchWindowEvent('message', {
      origin: 'http://localhost:7891',
      data: { source: 'gasoline-terminal', event: 'focus', data: { focused: true } }
    })
    dispatchWindowEvent('message', {
      origin: 'http://localhost:7891',
      data: { source: 'gasoline-terminal', event: 'typing', data: { at: Date.now() } }
    })

    await sleep(680)
    const blockedPayloads = getPostMessagePayloads(iframe, callStart)
    const blockedWrites = blockedPayloads
      .filter((payload) => payload?.command === 'write')
      .map((payload) => payload.text)
    assert.deepStrictEqual(blockedWrites, ['submit guard command'], 'auto-enter should pause after focus returns')

    dispatchWindowEvent('message', {
      origin: 'http://localhost:7891',
      data: { source: 'gasoline-terminal', event: 'focus', data: { focused: false } }
    })

    await sleep(320)
    const releasedPayloads = getPostMessagePayloads(iframe, callStart)
    const releasedWrites = releasedPayloads
      .filter((payload) => payload?.command === 'write')
      .map((payload) => payload.text)
    assert.deepStrictEqual(releasedWrites, ['submit guard command', '\r'])
  })
})
