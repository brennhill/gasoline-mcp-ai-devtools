// @ts-nocheck
/**
 * @fileoverview tracked-hover-launcher.test.js — Unit tests for tracked-tab quick actions launcher UI.
 */

import { beforeEach, describe, mock, test } from 'node:test'
import assert from 'node:assert'

let elementsById
let appendedToBody
let storageData
let storageChangeListener
let runtimeSendMessage
let setTrackedHoverLauncherEnabled

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

function findButtonByTitle(element, title) {
  if (!element) return null
  if (element.tag === 'button' && element.title === title) return element
  for (const child of element.children || []) {
    const found = findButtonByTitle(child, title)
    if (found) return found
  }
  return null
}

function resetGlobals() {
  elementsById = {}
  appendedToBody = []
  storageData = { gasoline_recording: { active: false } }
  storageChangeListener = null

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
      getURL: mock.fn((path) => `chrome-extension://test/${path}`),
      sendMessage: runtimeSendMessage
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
        appendedToBody.push(el)
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
    if (!setTrackedHoverLauncherEnabled) {
      ;({ setTrackedHoverLauncherEnabled } = await import('../../extension/content/ui/tracked-hover-launcher.js'))
    }
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
    const screenshotButton = findButtonByTitle(root, 'Capture screenshot')
    assert.ok(screenshotButton, 'expected screenshot button')

    screenshotButton.dispatch('click')

    const sentTypes = runtimeSendMessage.mock.calls.map((call) => call.arguments[0]?.type)
    assert.ok(sentTypes.includes('captureScreenshot'))
  })

  test('record action toggles between record_start and record_stop', () => {
    setTrackedHoverLauncherEnabled(true)

    const root = elementsById['gasoline-tracked-hover-launcher']
    const recordButton = findButtonByTitle(root, 'Start recording')
    assert.ok(recordButton, 'expected record button')

    recordButton.dispatch('click')
    assert.strictEqual(recordButton.textContent, 'Stop', 'record button should switch to Stop after start')

    recordButton.dispatch('click')
    assert.strictEqual(recordButton.textContent, 'Rec', 'record button should switch back to Rec after stop')

    const sentTypes = runtimeSendMessage.mock.calls.map((call) => call.arguments[0]?.type)
    assert.ok(sentTypes.includes('record_start'))
    assert.ok(sentTypes.includes('record_stop'))
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
