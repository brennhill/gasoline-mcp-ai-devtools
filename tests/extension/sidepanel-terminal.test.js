// @ts-nocheck
/**
 * @fileoverview sidepanel-terminal.test.js — Regression coverage for the terminal side panel host.
 */

import { beforeEach, describe, mock, test } from 'node:test'
import assert from 'node:assert'
import { StorageKey } from '../../extension/lib/constants.js'

let importCounter = 0
let localStorageData = {}
let sessionStorageData = {}
let fetchHandler = null
let roots = []
let windowListeners = {}
let runtimeMessageListeners = []
let storageChangeListener = null
let activeTabId = 1
let tabUpdatedListeners = []
let workspaceStatusSnapshot = null

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
    src: '',
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
    replaceChildren: mock.fn((...children) => {
      el.children = []
      for (const child of children) {
        child.parentElement = el
        el.children.push(child)
      }
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

function textOf(id) {
  const node = getElementById(id)
  const texts = []
  walkTree(node, (child) => {
    if (typeof child.textContent === 'string' && child.textContent) {
      texts.push(child.textContent)
    }
    return false
  })
  return texts.join(' ')
}

function sleep(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms))
}

function emitStorageChange(areaName, key, oldValue, newValue) {
  if (!storageChangeListener) return
  storageChangeListener({ [key]: { oldValue, newValue } }, areaName)
}

function dispatchRuntimeMessage(message, sender = { id: 'test-extension-id' }) {
  for (const listener of runtimeMessageListeners) {
    listener(message, sender, () => {})
  }
}

function emitTabUpdated(tabId, url) {
  if (workspaceStatusSnapshot) {
    workspaceStatusSnapshot = {
      ...workspaceStatusSnapshot,
      page: {
        title: 'New Route',
        url,
        summary: 'New route summary'
      }
    }
  }
  for (const listener of tabUpdatedListeners) {
    listener(tabId, { url, status: 'complete' }, { id: tabId, url })
  }
}

function setupEnvironment() {
  roots = []
  fetchHandler = null
  windowListeners = {}
  runtimeMessageListeners = []
  storageChangeListener = null
  activeTabId = 1
  tabUpdatedListeners = []
  workspaceStatusSnapshot = {
    mode: 'live',
    seo: { label: 'SEO', score: 64, state: 'needs_attention', source: 'heuristic' },
    accessibility: { label: 'Accessibility', score: 72, state: 'needs_attention', source: 'heuristic' },
    performance: { verdict: 'mixed', source: 'heuristic' },
    session: { recording_active: false, screenshot_count: 0, note_count: 0 },
    audit: { updated_at: null, state: 'idle' },
    page: {
      title: 'Tracked Checkout',
      url: 'https://tracked.example/checkout',
      summary: 'Tracked checkout summary'
    },
    recommendation: 'Run an audit to confirm page health.'
  }

  const body = createElement('body')
  const head = createElement('head')
  const documentElement = createElement('html')
  roots.push(body, head, documentElement)

  globalThis.document = {
    body,
    head,
    documentElement,
    readyState: 'complete',
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
    if (!fetchHandler) throw new Error('fetchHandler is not configured')
    return fetchHandler(call)
  })

  globalThis.chrome = {
    runtime: {
      id: 'test-extension-id',
      lastError: null,
      sendMessage: mock.fn((message, callback) => {
        if (message?.type === 'get_workspace_status') {
          callback?.(workspaceStatusSnapshot)
          return Promise.resolve(workspaceStatusSnapshot)
        }
        if (message?.type === 'terminal_panel_write') {
          callback?.({ success: true })
          return Promise.resolve({ success: true })
        }
        if (message?.type === 'open_terminal_panel') {
          callback?.({ success: true })
          return Promise.resolve({ success: true })
        }
        callback?.({})
        return Promise.resolve({})
      }),
      onMessage: {
        addListener: mock.fn((listener) => {
          runtimeMessageListeners.push(listener)
        }),
        removeListener: mock.fn((listener) => {
          runtimeMessageListeners = runtimeMessageListeners.filter((item) => item !== listener)
        })
      }
    },
    sidePanel: {
      close: mock.fn(() => Promise.resolve())
    },
    tabs: {
      query: mock.fn((_queryInfo) => Promise.resolve([{ id: activeTabId }])),
      onUpdated: {
        addListener: mock.fn((listener) => {
          tabUpdatedListeners.push(listener)
        }),
        removeListener: mock.fn((listener) => {
          tabUpdatedListeners = tabUpdatedListeners.filter((item) => item !== listener)
        })
      }
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
          const prev = { ...sessionStorageData }
          sessionStorageData = { ...sessionStorageData, ...(payload || {}) }
          for (const [key, value] of Object.entries(payload || {})) {
            emitStorageChange('session', key, prev[key], value)
          }
          callback?.()
        }),
        remove: mock.fn((keys, callback) => {
          const keyList = Array.isArray(keys) ? keys : [keys]
          const prev = { ...sessionStorageData }
          for (const key of keyList) {
            delete sessionStorageData[key]
            emitStorageChange('session', key, prev[key], undefined)
          }
          callback?.()
        })
      },
      onChanged: {
        addListener: mock.fn((listener) => {
          storageChangeListener = listener
        }),
        removeListener: mock.fn((listener) => {
          if (storageChangeListener === listener) storageChangeListener = null
        })
      }
    }
  }
}

function findButton(root, predicate) {
  if (!root) return null
  return walkTree(root, (node) => node.tagName === 'BUTTON' && predicate(node))
}

describe('terminal side panel host', () => {
  beforeEach(() => {
    mock.reset()
    localStorageData = { [StorageKey.SERVER_URL]: 'http://localhost:7890' }
    sessionStorageData = {}
    setupEnvironment()
  })

  test('boots a panel with terminal iframe and persists open state', async () => {
    fetchHandler = ({ url }) => {
      if (url.endsWith('/terminal/start')) {
        return Promise.resolve(makeResponse(200, {
          session_id: 'session-1',
          token: 'token-1',
          pid: 999
        }))
      }
      throw new Error(`Unexpected fetch call: ${url}`)
    }

    const module = await import(`../../extension/sidepanel.js?v=${++importCounter}`)
    await module._terminalPanelForTests.bootTerminalPanel(true)

    const terminalRegion = getElementById('kaboom-workspace-terminal-region')
    const header = getElementById('kaboom-terminal-header')
    const iframe = getElementById('kaboom-terminal-iframe')
    assert.ok(terminalRegion, 'workspace terminal region should be mounted')
    assert.ok(header, 'terminal header should be mounted')
    assert.ok(iframe, 'terminal iframe should be mounted')
    assert.strictEqual(sessionStorageData[StorageKey.TERMINAL_UI_STATE], 'open')

    const minimizeButton = findButton(header, (node) => node.title === 'Minimize terminal')
    assert.ok(minimizeButton, 'minimize button should exist')
    assert.strictEqual(minimizeButton.textContent, '\u2581')
  })

  test('disconnect button ends the current session and closes the side panel', async () => {
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

    const module = await import(`../../extension/sidepanel.js?v=${++importCounter}`)
    await module._terminalPanelForTests.bootTerminalPanel(true)

    const header = getElementById('kaboom-terminal-header')
    const powerButton = findButton(header, (node) => node.title === 'Disconnect terminal & end session')
    assert.ok(powerButton, 'power button should be present')
    assert.strictEqual(startCount, 1)

    powerButton.dispatch('click')
    await sleep(0)

    assert.strictEqual(stopBodies.length, 1, 'disconnect should stop the current session')
    assert.deepStrictEqual(stopBodies[0], { id: 'session-1' })
    assert.strictEqual(startCount, 1, 'disconnect should not boot a fresh session')
    assert.strictEqual(chrome.sidePanel.close.mock.calls.length, 1, 'disconnect should close the side panel')
    assert.strictEqual(chrome.sidePanel.close.mock.calls[0].arguments[0].tabId, 1)
    assert.strictEqual(sessionStorageData[StorageKey.TERMINAL_SESSION], undefined, 'disconnect should clear persisted session')
    assert.strictEqual(sessionStorageData[StorageKey.TERMINAL_UI_STATE], undefined, 'disconnect should clear persisted UI state')
    assert.strictEqual(getElementById('kaboom-terminal-widget'), null, 'disconnect should unmount the side panel shell')
  })

  test('minimize button hides the side panel and keeps the current session alive', async () => {
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
      throw new Error(`Unexpected fetch call: ${url}`)
    }

    const module = await import(`../../extension/sidepanel.js?v=${++importCounter}`)
    await module._terminalPanelForTests.bootTerminalPanel(true)

    const header = getElementById('kaboom-terminal-header')
    const minimizeButton = findButton(header, (node) => node.title === 'Minimize terminal')
    assert.ok(minimizeButton, 'minimize button should be present')
    assert.strictEqual(startCount, 1)

    minimizeButton.dispatch('click')
    await sleep(0)

    assert.strictEqual(stopBodies.length, 0, 'minimize should not stop the current session')
    assert.strictEqual(startCount, 1, 'minimize should not boot a fresh session')
    assert.strictEqual(chrome.sidePanel.close.mock.calls.length, 1, 'minimize should close the side panel')
    assert.strictEqual(chrome.sidePanel.close.mock.calls[0].arguments[0].tabId, 1)
    assert.deepStrictEqual(
      sessionStorageData[StorageKey.TERMINAL_SESSION],
      { sessionId: 'session-1', token: 'token-1' },
      'minimize should keep the persisted session'
    )
    assert.strictEqual(sessionStorageData[StorageKey.TERMINAL_UI_STATE], 'minimized', 'minimize should persist hidden-session state')
    assert.strictEqual(getElementById('kaboom-terminal-widget'), null, 'minimize should unmount the side panel shell')
  })

  test('redraw button reloads iframe without starting a new session', async () => {
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

    const module = await import(`../../extension/sidepanel.js?v=${++importCounter}`)
    await module._terminalPanelForTests.bootTerminalPanel(true)

    const header = getElementById('kaboom-terminal-header')
    const iframe = getElementById('kaboom-terminal-iframe')
    const redrawButton = findButton(header, (node) => node.title === 'Redraw terminal graphics')
    assert.ok(iframe, 'terminal iframe should exist')
    assert.ok(redrawButton, 'redraw button should exist')

    const priorSrc = iframe.src
    redrawButton.dispatch('click')

    assert.strictEqual(iframe.src, priorSrc, 'redraw should keep the same token URL')
    assert.strictEqual(startCount, 1, 'redraw should not start a new session')
  })

  test('write guard waits while user is typing and flushes after blur', async () => {
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

    const module = await import(`../../extension/sidepanel.js?v=${++importCounter}`)
    await module._terminalPanelForTests.bootTerminalPanel(true)

    const iframe = getElementById('kaboom-terminal-iframe')
    assert.ok(iframe, 'terminal iframe should exist')

    dispatchWindowEvent('message', {
      origin: 'http://localhost:7891',
      data: { source: 'kaboom-terminal', event: 'connected' }
    })
    await sleep(720)
    dispatchWindowEvent('message', {
      origin: 'http://localhost:7891',
      data: { source: 'kaboom-terminal', event: 'focus', data: { focused: true } }
    })
    dispatchWindowEvent('message', {
      origin: 'http://localhost:7891',
      data: { source: 'kaboom-terminal', event: 'typing', data: { at: Date.now() } }
    })

    const callStart = iframe.contentWindow.postMessage.mock.calls.length
    module._terminalPanelForTests.writeToTerminal('queued command')

    await sleep(80)
    const whileTypingPayloads = getPostMessagePayloads(iframe, callStart)
    assert.strictEqual(whileTypingPayloads.filter((payload) => payload?.command === 'write').length, 0)

    dispatchWindowEvent('message', {
      origin: 'http://localhost:7891',
      data: { source: 'kaboom-terminal', event: 'focus', data: { focused: false } }
    })

    await sleep(800)
    const flushedPayloads = getPostMessagePayloads(iframe, callStart)
    const flushedWrites = flushedPayloads
      .filter((payload) => payload?.command === 'write')
      .map((payload) => payload.text)

    assert.ok(flushedPayloads.some((payload) => payload?.command === 'write'), 'queued write should reach the iframe')
    assert.deepStrictEqual(flushedWrites, ['queued command', '\r'])
  })

  test('terminal submit re-guards if focus returns before auto-enter', async () => {
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

    const module = await import(`../../extension/sidepanel.js?v=${++importCounter}`)
    await module._terminalPanelForTests.bootTerminalPanel(true)

    const iframe = getElementById('kaboom-terminal-iframe')
    assert.ok(iframe, 'terminal iframe should exist')

    dispatchWindowEvent('message', {
      origin: 'http://localhost:7891',
      data: { source: 'kaboom-terminal', event: 'connected' }
    })
    dispatchWindowEvent('message', {
      origin: 'http://localhost:7891',
      data: { source: 'kaboom-terminal', event: 'focus', data: { focused: false } }
    })
    await sleep(720)

    const callStart = iframe.contentWindow.postMessage.mock.calls.length
    module._terminalPanelForTests.writeToTerminal('submit guard command')

    await sleep(80)
    dispatchWindowEvent('message', {
      origin: 'http://localhost:7891',
      data: { source: 'kaboom-terminal', event: 'focus', data: { focused: true } }
    })
    dispatchWindowEvent('message', {
      origin: 'http://localhost:7891',
      data: { source: 'kaboom-terminal', event: 'typing', data: { at: Date.now() } }
    })

    await sleep(680)
    const blockedPayloads = getPostMessagePayloads(iframe, callStart)
    const blockedWrites = blockedPayloads
      .filter((payload) => payload?.command === 'write')
      .map((payload) => payload.text)
    assert.deepStrictEqual(blockedWrites, ['submit guard command'])

    dispatchWindowEvent('message', {
      origin: 'http://localhost:7891',
      data: { source: 'kaboom-terminal', event: 'focus', data: { focused: false } }
    })

    await sleep(320)
    const releasedPayloads = getPostMessagePayloads(iframe, callStart)
    const releasedWrites = releasedPayloads
      .filter((payload) => payload?.command === 'write')
      .map((payload) => payload.text)
    assert.deepStrictEqual(releasedWrites, ['submit guard command', '\r'])
  })

  test('reopening a minimized session restores the full panel without starting a new session', async () => {
    sessionStorageData[StorageKey.TERMINAL_SESSION] = { sessionId: 'session-min', token: 'token-min' }
    sessionStorageData[StorageKey.TERMINAL_UI_STATE] = 'minimized'

    fetchHandler = ({ url }) => {
      if (url.includes('/terminal/validate?token=')) {
        return Promise.resolve(makeResponse(200, { valid: true }))
      }
      throw new Error(`Unexpected fetch call: ${url}`)
    }

    const module = await import(`../../extension/sidepanel.js?v=${++importCounter}`)
    await module._terminalPanelForTests.bootTerminalPanel(true)

    const header = getElementById('kaboom-terminal-header')
    const minimizeButton = findButton(header, (node) => node.title === 'Minimize terminal')
    const terminalBody = header?.parentElement?.children?.[1] || null

    assert.ok(minimizeButton, 'minimize button should be present after restore')
    assert.ok(terminalBody, 'terminal body should exist after restore')
    assert.strictEqual(terminalBody.style.display, 'block', 'reopened minimized session should restore the full panel')
    assert.strictEqual(sessionStorageData[StorageKey.TERMINAL_UI_STATE], 'open', 'reopen should promote minimized session back to open')
  })

  test('panel mounts the workspace shell around the terminal pane', async () => {
    fetchHandler = ({ url }) => {
      if (url.endsWith('/terminal/start')) {
        return Promise.resolve(makeResponse(200, {
          session_id: 'session-full-height',
          token: 'token-full-height',
          pid: 999
        }))
      }
      throw new Error(`Unexpected fetch call: ${url}`)
    }

    const module = await import(`../../extension/sidepanel.js?v=${++importCounter}`)
    await module._terminalPanelForTests.bootTerminalPanel(true)

    const root = getElementById('kaboom-terminal-widget')
    const summaryStrip = getElementById('kaboom-workspace-summary-strip')
    const actionRow = getElementById('kaboom-workspace-action-row')
    const statusArea = getElementById('kaboom-workspace-status-area')
    const terminalRegion = getElementById('kaboom-workspace-terminal-region')
    const header = getElementById('kaboom-terminal-header')
    const iframe = getElementById('kaboom-terminal-iframe')
    const terminalShell = header?.parentElement || null
    const newProjectButton = findButton(root, (node) => node.textContent === 'New Project')
    const titleNode = walkTree(header, (child) => child.textContent === 'KaBOOM! Workspace')

    assert.ok(root, 'panel root should exist')
    assert.ok(summaryStrip, 'workspace summary strip should exist')
    assert.ok(actionRow, 'workspace action row should exist')
    assert.ok(statusArea, 'workspace status area should exist')
    assert.ok(terminalRegion, 'workspace terminal region should exist')
    assert.ok(header, 'terminal header should exist')
    assert.ok(iframe, 'terminal iframe should exist')
    assert.ok(terminalShell, 'terminal shell should wrap the header and iframe')
    assert.ok(titleNode, 'terminal header should show KaBOOM! Workspace')
    assert.strictEqual(newProjectButton, null, 'placeholder palette action should not be rendered')
    assert.strictEqual(root.children.length, 4, 'workspace shell should render summary, actions, terminal, and status regions')
  })

  test('daemon-unavailable fallback keeps workspace copy and shell chrome', async () => {
    fetchHandler = ({ url }) => {
      if (url.endsWith('/terminal/start')) {
        return Promise.resolve(makeResponse(500, { error: 'daemon_unavailable' }))
      }
      throw new Error(`Unexpected fetch call: ${url}`)
    }

    const module = await import(`../../extension/sidepanel.js?v=${++importCounter}`)
    await module._terminalPanelForTests.bootTerminalPanel(true)

    const summaryStrip = getElementById('kaboom-workspace-summary-strip')
    const header = getElementById('kaboom-terminal-header')
    const terminalBody = header?.parentElement?.children?.[1] || null
    const titleNode = walkTree(header, (child) => child.textContent === 'KaBOOM! Workspace')
    const fallbackNode = walkTree(terminalBody, (child) =>
      child.textContent === 'Terminal unavailable. Start the KaBOOM! daemon and reopen the panel.'
    )

    assert.ok(summaryStrip, 'workspace summary strip should remain visible when terminal boot fails')
    assert.ok(header, 'terminal header should exist')
    assert.ok(titleNode, 'terminal header should show KaBOOM! Workspace')
    assert.ok(terminalBody, 'terminal body should exist')
    assert.ok(fallbackNode, 'fallback should mention the KaBOOM! daemon')
  })

  test('workspace context injects on open, queues route refresh while typing, and sends audit summaries', async () => {
    fetchHandler = ({ url }) => {
      if (url.endsWith('/terminal/start')) {
        return Promise.resolve(makeResponse(200, {
          session_id: 'session-context',
          token: 'token-context',
          pid: 999
        }))
      }
      throw new Error(`Unexpected fetch call: ${url}`)
    }

    const module = await import(`../../extension/sidepanel.js?v=${++importCounter}`)
    await module._terminalPanelForTests.bootTerminalPanel(true)

    const iframe = getElementById('kaboom-terminal-iframe')
    assert.ok(iframe, 'terminal iframe should exist')

    const initialWrites = getPostMessagePayloads(iframe)
      .filter((payload) => payload?.command === 'write')
      .map((payload) => payload.text)
    assert.ok(initialWrites.some((text) => /Page context/i.test(text)), 'workspace open should inject page context')

    dispatchWindowEvent('message', {
      origin: 'http://localhost:7891',
      data: { source: 'kaboom-terminal', event: 'connected' }
    })
    await sleep(720)
    dispatchWindowEvent('message', {
      origin: 'http://localhost:7891',
      data: { source: 'kaboom-terminal', event: 'focus', data: { focused: true } }
    })
    dispatchWindowEvent('message', {
      origin: 'http://localhost:7891',
      data: { source: 'kaboom-terminal', event: 'typing', data: { at: Date.now() } }
    })

    const queuedStart = iframe.contentWindow.postMessage.mock.calls.length
    emitTabUpdated(1, 'https://tracked.example/new-route')

    const queuedWrites = getPostMessagePayloads(iframe, queuedStart).filter((payload) => payload?.command === 'write')
    assert.strictEqual(queuedWrites.length, 0, 'route refresh should defer while the user is typing')
    assert.match(textOf('kaboom-workspace-status-area'), /queued/i)

    dispatchWindowEvent('message', {
      origin: 'http://localhost:7891',
      data: { source: 'kaboom-terminal', event: 'focus', data: { focused: false } }
    })

    await sleep(1000)
    const flushedWrites = getPostMessagePayloads(iframe, queuedStart)
      .filter((payload) => payload?.command === 'write')
      .map((payload) => payload.text)
    assert.ok(flushedWrites.some((text) => /new-route/i.test(text)), 'queued route refresh should flush after focus clears')

    const auditStart = iframe.contentWindow.postMessage.mock.calls.length
    dispatchRuntimeMessage({
      type: 'workspace_status_updated',
      host_tab_id: 1,
      snapshot: {
        mode: 'audit',
        seo: { label: 'SEO', score: 91, state: 'healthy', source: 'audit' },
        accessibility: { label: 'Accessibility', score: 93, state: 'healthy', source: 'audit' },
        performance: { verdict: 'good', source: 'audit' },
        session: { recording_active: false, screenshot_count: 3, note_count: 1 },
        audit: { updated_at: '2026-04-18T12:00:00.000Z', state: 'available' },
        page: {
          title: 'New Route',
          url: 'https://tracked.example/new-route',
          summary: 'New route summary'
        },
        recommendation: 'Audit summary is ready for terminal follow-up.'
      }
    })

    await sleep(720)

    const auditWrites = getPostMessagePayloads(iframe, auditStart)
      .filter((payload) => payload?.command === 'write')
      .map((payload) => payload.text)
    assert.ok(auditWrites.some((text) => /audit summary/i.test(text)), 'audit updates should inject a terminal summary')
  })

  test('workspace status updates require an extension-page sender and matching host tab id', async () => {
    fetchHandler = ({ url }) => {
      if (url.endsWith('/terminal/start')) {
        return Promise.resolve(makeResponse(200, {
          session_id: 'session-routing',
          token: 'token-routing',
          pid: 999
        }))
      }
      throw new Error(`Unexpected fetch call: ${url}`)
    }

    const module = await import(`../../extension/sidepanel.js?v=${++importCounter}`)
    await module._terminalPanelForTests.bootTerminalPanel(true)

    assert.match(textOf('kaboom-workspace-summary-strip'), /64/)

    const ignoredSnapshot = {
      mode: 'audit',
      seo: { label: 'SEO', score: 12, state: 'needs_attention', source: 'audit' },
      accessibility: { label: 'Accessibility', score: 20, state: 'needs_attention', source: 'audit' },
      performance: { verdict: 'poor', source: 'audit' },
      session: { recording_active: false, screenshot_count: 0, note_count: 0 },
      audit: { updated_at: '2026-04-18T12:30:00.000Z', state: 'available' },
      page: {
        title: 'Wrong Tab',
        url: 'https://tracked.example/wrong',
        summary: 'Wrong tab summary'
      },
      recommendation: 'Wrong tab recommendation.'
    }

    dispatchRuntimeMessage(
      { type: 'workspace_status_updated', host_tab_id: 1, snapshot: ignoredSnapshot },
      { id: 'test-extension-id', tab: { id: 99 } }
    )
    assert.match(textOf('kaboom-workspace-summary-strip'), /64/)

    dispatchRuntimeMessage({ type: 'workspace_status_updated', host_tab_id: 99, snapshot: ignoredSnapshot })
    assert.match(textOf('kaboom-workspace-summary-strip'), /64/)

    dispatchRuntimeMessage({
      type: 'workspace_status_updated',
      host_tab_id: 1,
      snapshot: {
        ...ignoredSnapshot,
        seo: { label: 'SEO', score: 92, state: 'healthy', source: 'audit' },
        page: {
          title: 'Tracked Checkout',
          url: 'https://tracked.example/checkout',
          summary: 'Tracked checkout summary'
        }
      }
    })
    assert.match(textOf('kaboom-workspace-summary-strip'), /92/)
  })

  test('closing during a slow workspace-status refresh does not apply stale boot state', async () => {
    let resolveWorkspaceStatus
    chrome.runtime.sendMessage = mock.fn((message, callback) => {
      if (message?.type === 'get_workspace_status') {
        return new Promise((resolve) => {
          resolveWorkspaceStatus = resolve
        })
      }
      callback?.({})
      return Promise.resolve({})
    })

    fetchHandler = ({ url }) => {
      if (url.endsWith('/terminal/start')) {
        return Promise.resolve(makeResponse(200, {
          session_id: 'session-slow-status',
          token: 'token-slow-status',
          pid: 999
        }))
      }
      if (url.endsWith('/terminal/stop')) {
        return Promise.resolve(makeResponse(200, { success: true }))
      }
      throw new Error(`Unexpected fetch call: ${url}`)
    }

    const module = await import(`../../extension/sidepanel.js?v=${++importCounter}`)
    const bootPromise = module._terminalPanelForTests.bootTerminalPanel(true)

    await Promise.resolve()
    await Promise.resolve()

    const exitPromise = module._terminalPanelForTests.exitTerminalSession()
    resolveWorkspaceStatus?.(workspaceStatusSnapshot)

    await exitPromise
    await bootPromise

    assert.strictEqual(getElementById('kaboom-terminal-widget'), null)
  })
})
