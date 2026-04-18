// @ts-nocheck
/**
 * @fileoverview workspace-sidebar.test.js — Regression coverage for the workspace sidebar shell.
 */

import { afterEach, beforeEach, describe, mock, test } from 'node:test'
import assert from 'node:assert'
import { StorageKey } from '../../extension/lib/constants.js'

let importCounter = 0
let localStorageData = {}
let sessionStorageData = {}
let fetchHandler = null
let roots = []
let storageChangeListener = null
let runtimeMessageListeners = []
let windowListeners = {}
let workspaceStatusSnapshot = null
let activeTabId = 7
let activeSidepanelModule = null

function makeResponse(status, body) {
  return {
    ok: status >= 200 && status < 300,
    status,
    json: async () => body
  }
}

function walkTree(node, visit) {
  if (!node) return null
  for (const child of node.children || []) {
    if (visit(child)) return child
    const found = walkTree(child, visit)
    if (found) return found
  }
  return null
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
    dispatch: (type, event = {}) => {
      const handler = listeners[type]
      if (!handler) return
      handler({
        preventDefault() {},
        stopPropagation() {},
        ...event
      })
    }
  }

  if (tag === 'iframe') {
    el.contentWindow = { postMessage: mock.fn() }
  }

  return el
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

function findButtonByText(root, text) {
  return walkTree(root, (node) => node.tagName === 'BUTTON' && node.textContent === text)
}

function getPostMessagePayloads(iframe, startAt = 0) {
  const calls = iframe?.contentWindow?.postMessage?.mock?.calls || []
  return calls.slice(startAt).map((call) => call.arguments[0])
}

function dispatchRuntimeMessage(message) {
  for (const listener of runtimeMessageListeners) {
    listener(message, { id: 'test-extension-id' }, () => {})
  }
}

function dispatchWindowEvent(type, event = {}) {
  const handlers = windowListeners[type] || []
  for (const handler of handlers) handler(event)
}

function setupEnvironment() {
  roots = []
  fetchHandler = null
  storageChangeListener = null
  runtimeMessageListeners = []
  windowListeners = {}

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
    })
  }

  globalThis.fetch = mock.fn(async (url, options = {}) => {
    const call = { url: String(url), options }
    if (!fetchHandler) throw new Error('fetchHandler is not configured')
    return fetchHandler(call)
  })

  globalThis.requestAnimationFrame = (cb) => cb()

  globalThis.chrome = {
    runtime: {
      id: 'test-extension-id',
      lastError: null,
      sendMessage: mock.fn((message) => {
        if (message?.type === 'get_workspace_status') {
          return Promise.resolve(workspaceStatusSnapshot)
        }
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
      query: mock.fn(() => Promise.resolve([{ id: activeTabId }])),
      sendMessage: mock.fn((_tabId, message) => Promise.resolve({ success: true, type: message?.type }))
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

function getTerminalFallbackText() {
  return walkTree(getElementById('kaboom-workspace-terminal-region'), (node) =>
    typeof node.textContent === 'string' && /Terminal unavailable/i.test(node.textContent)
  )?.textContent
}

describe('workspace sidebar shell', () => {
  beforeEach(() => {
    mock.reset()
    localStorageData = { [StorageKey.SERVER_URL]: 'http://localhost:7890' }
    sessionStorageData = {}
    workspaceStatusSnapshot = {
      mode: 'live',
      seo: { label: 'SEO', score: 64, state: 'needs_attention', source: 'heuristic' },
      accessibility: { label: 'Accessibility', score: 71, state: 'needs_attention', source: 'heuristic' },
      performance: { verdict: 'mixed', source: 'heuristic' },
      session: { recording_active: true, screenshot_count: 2, note_count: 1 },
      audit: { updated_at: null, state: 'idle' },
      page: {
        title: 'Checkout',
        url: 'https://tracked.example/checkout',
        summary: 'Checkout flow with a single-step form.'
      },
      recommendation: 'Run an audit to confirm labels and metadata.'
    }
    activeTabId = 7
    setupEnvironment()
  })

  afterEach(async () => {
    const exitTerminalSession = activeSidepanelModule?._terminalPanelForTests?.exitTerminalSession
    activeSidepanelModule = null
    if (typeof exitTerminalSession !== 'function') return
    try {
      await exitTerminalSession()
    } catch {
      // Best effort. Cleanup only needs to stop timers/session state.
    }
  })

  test('boots the workspace shell around the terminal host', async () => {
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
    activeSidepanelModule = module
    await module._terminalPanelForTests.bootTerminalPanel(true)

    assert.ok(getElementById('kaboom-workspace-summary-strip'))
    assert.ok(getElementById('kaboom-workspace-action-row'))
    assert.ok(getElementById('kaboom-workspace-terminal-region'))
    assert.ok(getElementById('kaboom-workspace-status-area'))
    assert.match(textOf('kaboom-workspace-summary-strip'), /SEO/i)
    assert.match(textOf('kaboom-workspace-summary-strip'), /Accessibility/i)
    assert.match(textOf('kaboom-workspace-summary-strip'), /Performance/i)
    assert.match(textOf('kaboom-workspace-status-area'), /recording/i)
  })

  test('fallback keeps the workspace shell mounted when the terminal is unavailable', async () => {
    fetchHandler = ({ url }) => {
      if (url.endsWith('/terminal/start')) {
        return Promise.resolve(makeResponse(503, { error: 'offline' }))
      }
      throw new Error(`Unexpected fetch call: ${url}`)
    }

    const module = await import(`../../extension/sidepanel.js?v=${++importCounter}`)
    activeSidepanelModule = module
    await module._terminalPanelForTests.bootTerminalPanel(true)

    assert.match(getTerminalFallbackText(), /Terminal unavailable/i)
    assert.ok(getElementById('kaboom-workspace-summary-strip'))
  })

  test('workspace summary strip replaces live heuristic values with audit updates', async () => {
    fetchHandler = ({ url }) => {
      if (url.endsWith('/terminal/start')) {
        return Promise.resolve(makeResponse(200, {
          session_id: 'session-2',
          token: 'token-2',
          pid: 999
        }))
      }
      throw new Error(`Unexpected fetch call: ${url}`)
    }

    const module = await import(`../../extension/sidepanel.js?v=${++importCounter}`)
    activeSidepanelModule = module
    await module._terminalPanelForTests.bootTerminalPanel(true)

    assert.match(textOf('kaboom-workspace-summary-strip'), /64/)

    dispatchRuntimeMessage({
      type: 'workspace_status_updated',
      host_tab_id: 7,
      snapshot: {
        ...workspaceStatusSnapshot,
        mode: 'audit',
        seo: { label: 'SEO', score: 88, state: 'healthy', source: 'audit' },
        accessibility: { label: 'Accessibility', score: 90, state: 'healthy', source: 'audit' },
        performance: { verdict: 'good', source: 'audit' },
        audit: { updated_at: '2026-04-18T10:15:00.000Z', state: 'available' }
      }
    })

    assert.match(textOf('kaboom-workspace-summary-strip'), /88/)
    assert.match(textOf('kaboom-workspace-status-area'), /2026-04-18/)
  })

  test('action row wires the planned QA actions through shared helpers and manual context injection', async () => {
    chrome.runtime.sendMessage = mock.fn((message) => {
      if (message?.type === 'get_workspace_status') {
        if (message.mode === 'audit') {
          return Promise.resolve({
            ...workspaceStatusSnapshot,
            mode: 'audit',
            audit: { updated_at: '2026-04-18T12:45:00.000Z', state: 'available' }
          })
        }
        return Promise.resolve(workspaceStatusSnapshot)
      }
      return Promise.resolve({})
    })

    fetchHandler = ({ url }) => {
      if (url.endsWith('/terminal/start')) {
        return Promise.resolve(makeResponse(200, {
          session_id: 'session-actions',
          token: 'token-actions',
          pid: 999
        }))
      }
      throw new Error(`Unexpected fetch call: ${url}`)
    }

    const module = await import(`../../extension/sidepanel.js?v=${++importCounter}`)
    activeSidepanelModule = module
    await module._terminalPanelForTests.bootTerminalPanel(true)

    const root = getElementById('kaboom-terminal-widget')
    const iframe = getElementById('kaboom-terminal-iframe')

    dispatchWindowEvent('message', {
      origin: 'http://localhost:7891',
      data: { source: 'kaboom-terminal', event: 'connected' }
    })
    await new Promise((resolve) => setTimeout(resolve, 400))

    const baselineRuntimeCalls = chrome.runtime.sendMessage.mock.calls.length
    const baselineIframeCalls = iframe.contentWindow.postMessage.mock.calls.length

    findButtonByText(root, 'Record')?.dispatch('click')
    findButtonByText(root, 'Screenshot')?.dispatch('click')
    findButtonByText(root, 'Run audit')?.dispatch('click')
    findButtonByText(root, 'Add note')?.dispatch('click')
    findButtonByText(root, 'Inject context')?.dispatch('click')
    findButtonByText(root, 'Reset workspace')?.dispatch('click')

    await new Promise((resolve) => setTimeout(resolve, 400))

    const runtimeMessages = chrome.runtime.sendMessage.mock.calls
      .slice(baselineRuntimeCalls)
      .map((call) => call.arguments[0])
    const runtimeTypes = runtimeMessages.map((message) => message?.type)
    const injectedWrites = getPostMessagePayloads(iframe, baselineIframeCalls)
      .filter((payload) => payload?.command === 'write')
      .map((payload) => payload.text)

    assert.ok(runtimeTypes.includes('screen_recording_stop'))
    assert.ok(runtimeTypes.includes('capture_screenshot'))
    assert.ok(runtimeTypes.includes('qa_scan_requested'))
    assert.strictEqual(runtimeMessages.find((message) => message?.type === 'qa_scan_requested')?.page_url, 'https://tracked.example/checkout')
    assert.deepStrictEqual(chrome.tabs.sendMessage.mock.calls[0].arguments, [
      7,
      { type: 'kaboom_draw_mode_start', started_by: 'user' }
    ])
    assert.ok(injectedWrites.some((text) => /Page context/i.test(text)))
    assert.ok(injectedWrites.every((text) => !/Audit summary/i.test(text)))
    assert.match(textOf('kaboom-workspace-status-area'), /Latest audit: not yet run/)
    assert.match(textOf('kaboom-workspace-status-area'), /reset/i)
  })

  test('action row shows a workspace error when screenshot capture fails', async () => {
    chrome.runtime.sendMessage = mock.fn((message) => {
      if (message?.type === 'get_workspace_status') {
        return Promise.resolve(workspaceStatusSnapshot)
      }
      if (message?.type === 'capture_screenshot') {
        return Promise.reject(new Error('capture failed'))
      }
      return Promise.resolve({})
    })

    fetchHandler = ({ url }) => {
      if (url.endsWith('/terminal/start')) {
        return Promise.resolve(makeResponse(200, {
          session_id: 'session-screenshot-error',
          token: 'token-screenshot-error',
          pid: 999
        }))
      }
      throw new Error(`Unexpected fetch call: ${url}`)
    }

    const module = await import(`../../extension/sidepanel.js?v=${++importCounter}`)
    activeSidepanelModule = module
    await module._terminalPanelForTests.bootTerminalPanel(true)

    const root = getElementById('kaboom-terminal-widget')
    findButtonByText(root, 'Screenshot')?.dispatch('click')

    await new Promise((resolve) => setTimeout(resolve, 20))

    assert.match(textOf('kaboom-workspace-status-area'), /Screenshot capture failed\./)
    assert.match(textOf('kaboom-workspace-status-area'), /capture failed/)
  })
})
