// @ts-nocheck
/**
 * @fileoverview tracked-hover-launcher.test.js — Unit tests for tracked-tab quick actions launcher UI.
 */

import { beforeEach, describe, mock, test } from 'node:test'
import assert from 'node:assert'

let elementsById
let storageData
let storageChangeListener
let runtimeSendMessage
let runtimeOnMessageListeners
let setTrackedHoverLauncherEnabled
let importCounter = 0

function registerElement(el) {
  if (el && el.id) {
    elementsById[el.id] = el
  }
}

function createMockElement(tag) {
  const listeners = {}
  const el = {
    tag,
    id: '',
    type: '',
    title: '',
    textContent: '',
    className: '',
    disabled: false,
    href: '',
    target: '',
    rel: '',
    dataset: {},
    style: {},
    children: [],
    appendChild: mock.fn((child) => {
      el.children.push(child)
      registerElement(child)
      return child
    }),
    remove: mock.fn(() => {
      if (el.id) delete elementsById[el.id]
    }),
    addEventListener: mock.fn((type, handler) => {
      listeners[type] = handler
    }),
    dispatch(type) {
      const handler = listeners[type]
      if (handler) {
        handler({
          preventDefault() {},
          stopPropagation() {}
        })
      }
    }
  }
  return el
}

function findElementByTitle(element, title) {
  if (!element) return null
  if (element.title === title) return element
  for (const child of element.children || []) {
    const found = findElementByTitle(child, title)
    if (found) return found
  }
  return null
}

function findLinkByText(element, text) {
  if (!element) return null
  if (element.tag === 'a' && element.textContent === text) return element
  for (const child of element.children || []) {
    const found = findLinkByText(child, text)
    if (found) return found
  }
  return null
}

function dispatchRuntimeMessage(message) {
  for (const listener of runtimeOnMessageListeners) {
    listener(message, { id: 'test-extension-id' }, () => {})
  }
}

function resetGlobals() {
  elementsById = {}
  storageData = { gasoline_recording: { active: false } }
  storageChangeListener = null
  runtimeOnMessageListeners = []

  runtimeSendMessage = mock.fn((message, callback) => {
    if (message?.type === 'captureScreenshot') {
      callback?.({ success: true })
      return Promise.resolve({ success: true })
    }
    if (message?.type === 'record_start') {
      storageData.gasoline_recording = { active: true }
      callback?.({ status: 'recording' })
      return Promise.resolve({ status: 'recording' })
    }
    if (message?.type === 'record_stop') {
      storageData.gasoline_recording = { active: false }
      callback?.({ status: 'saved' })
      return Promise.resolve({ status: 'saved' })
    }
    callback?.({})
    return Promise.resolve({})
  })

  globalThis.chrome = {
    runtime: {
      id: 'test-extension-id',
      getURL: mock.fn((path) => `chrome-extension://test/${path}`),
      sendMessage: runtimeSendMessage,
      onMessage: {
        addListener: mock.fn((listener) => {
          runtimeOnMessageListeners.push(listener)
        }),
        removeListener: mock.fn((listener) => {
          runtimeOnMessageListeners = runtimeOnMessageListeners.filter((item) => item !== listener)
        })
      }
    },
    storage: {
      local: {
        get: mock.fn((_keys, callback) => {
          callback?.(storageData)
          return Promise.resolve(storageData)
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

  globalThis.document = {
    getElementById: mock.fn((id) => elementsById[id] || null),
    createElement: mock.fn((tag) => createMockElement(tag)),
    body: {
      appendChild: mock.fn((el) => {
        registerElement(el)
        return el
      })
    },
    documentElement: {
      appendChild: mock.fn((el) => {
        registerElement(el)
        return el
      })
    }
  }
}

describe('tracked hover launcher', () => {
  beforeEach(async () => {
    mock.reset()
    resetGlobals()
    ;({ setTrackedHoverLauncherEnabled } = await import(
      `../../extension/content/ui/tracked-hover-launcher.js?v=${++importCounter}`
    ))
    setTrackedHoverLauncherEnabled(false)
  })

  test('mounts only when tracked is enabled', () => {
    setTrackedHoverLauncherEnabled(true)

    assert.ok(elementsById['gasoline-tracked-hover-launcher'], 'launcher root should be mounted')
    assert.ok(elementsById['gasoline-tracked-hover-toggle'], 'launcher toggle should exist')
    assert.ok(elementsById['gasoline-tracked-hover-panel'], 'launcher panel should exist')
  })

  test('screenshot action sends captureScreenshot runtime message', () => {
    setTrackedHoverLauncherEnabled(true)

    const root = elementsById['gasoline-tracked-hover-launcher']
    const screenshotButton = findElementByTitle(root, 'Capture screenshot')
    assert.ok(screenshotButton, 'expected screenshot button')

    screenshotButton.dispatch('click')

    const sentTypes = runtimeSendMessage.mock.calls.map((call) => call.arguments[0]?.type)
    assert.ok(sentTypes.includes('captureScreenshot'))
  })

  test('record action toggles between record_start and record_stop', () => {
    setTrackedHoverLauncherEnabled(true)

    const root = elementsById['gasoline-tracked-hover-launcher']
    const recordButton = findElementByTitle(root, 'Start recording')
    assert.ok(recordButton, 'expected record button')

    recordButton.dispatch('click')
    assert.strictEqual(recordButton.textContent, 'Stop', 'record button should switch to Stop after start')

    recordButton.dispatch('click')
    assert.strictEqual(recordButton.textContent, 'Rec', 'record button should switch back to Rec after stop')

    const sentTypes = runtimeSendMessage.mock.calls.map((call) => call.arguments[0]?.type)
    assert.ok(sentTypes.includes('record_start'))
    assert.ok(sentTypes.includes('record_stop'))
  })

  test('settings menu exposes docs and github links', () => {
    setTrackedHoverLauncherEnabled(true)

    const root = elementsById['gasoline-tracked-hover-launcher']
    const settingsButton = findElementByTitle(root, 'Launcher settings')
    assert.ok(settingsButton, 'expected settings button')
    settingsButton.dispatch('click')

    const docsLink = findLinkByText(root, 'Docs')
    const repoLink = findLinkByText(root, 'GitHub Repository')

    assert.ok(docsLink, 'expected docs link')
    assert.ok(repoLink, 'expected repo link')
    assert.strictEqual(docsLink.href, 'https://cookwithgasoline.com/docs')
    assert.strictEqual(repoLink.href, 'https://github.com/brennhill/gasoline-agentic-browser-devtools-mcp')
  })

  test('hide action removes launcher until popup show message arrives', () => {
    setTrackedHoverLauncherEnabled(true)

    const root = elementsById['gasoline-tracked-hover-launcher']
    const settingsButton = findElementByTitle(root, 'Launcher settings')
    assert.ok(settingsButton)
    settingsButton.dispatch('click')

    const hideButton = findElementByTitle(root, 'Hide launcher until popup is opened again')
    assert.ok(hideButton, 'expected hide button')
    hideButton.dispatch('click')

    assert.strictEqual(elementsById['gasoline-tracked-hover-launcher'], undefined)

    dispatchRuntimeMessage({ type: 'GASOLINE_SHOW_TRACKED_HOVER_LAUNCHER' })
    assert.ok(elementsById['gasoline-tracked-hover-launcher'], 'launcher should remount after popup signal')
  })

  test('unmount removes launcher and storage listener', () => {
    setTrackedHoverLauncherEnabled(true)
    assert.ok(elementsById['gasoline-tracked-hover-launcher'])
    assert.ok(storageChangeListener, 'storage listener should be installed while mounted')

    setTrackedHoverLauncherEnabled(false)

    assert.strictEqual(elementsById['gasoline-tracked-hover-launcher'], undefined)
    assert.strictEqual(storageChangeListener, null, 'storage listener should be removed on unmount')
  })
})
