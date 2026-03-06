// @ts-nocheck
/**
 * @fileoverview options.test.js — Tests for the extension options/settings page.
 * Covers server URL persistence, domain filter management, toggle states
 * (screenshot, source maps, deferral), and chrome.storage.local integration.
 */

import { test, describe, mock, beforeEach } from 'node:test'
import assert from 'node:assert'

// Mock Chrome APIs
const mockChrome = {
  runtime: {
    sendMessage: mock.fn(),
    onMessage: { addListener: mock.fn() }
  },
  storage: {
    local: {
      get: mock.fn((keys, cb) => cb({})),
      set: mock.fn((data, cb) => cb && cb())
    }
  }
}

globalThis.chrome = mockChrome

// Mock DOM
function createMockDocument() {
  const elements = {}

  return {
    getElementById: (id) => {
      if (!elements[id]) {
        elements[id] = {
          id,
          value: '',
          textContent: '',
          classList: {
            _classes: new Set(),
            add(c) {
              this._classes.add(c)
            },
            remove(c) {
              this._classes.delete(c)
            },
            contains(c) {
              return this._classes.has(c)
            },
            toggle(c) {
              if (this._classes.has(c)) {
                this._classes.delete(c)
              } else {
                this._classes.add(c)
              }
            }
          },
          style: {},
          addEventListener: mock.fn()
        }
      }
      return elements[id]
    },
    addEventListener: mock.fn(),
    querySelector: mock.fn(() => null),
    querySelectorAll: mock.fn(() => []),
    readyState: 'complete',
    _elements: elements
  }
}

globalThis.document = createMockDocument()

const { loadOptions, saveOptions, toggleDeferral, toggleDebugMode } = await import('../../extension/options.js')

describe('Options Deferral Toggle', () => {
  beforeEach(() => {
    globalThis.document = createMockDocument()
    mockChrome.runtime.sendMessage = mock.fn()
    mockChrome.storage.local.get = mock.fn((keys, cb) => cb({}))
    mockChrome.storage.local.set = mock.fn((data, cb) => cb && cb())
  })

  test('should load deferral toggle state from storage (default: true/active)', () => {
    mockChrome.storage.local.get = mock.fn((keys, cb) => cb({}))

    loadOptions()

    const toggle = document.getElementById('deferral-toggle')
    assert.ok(toggle.classList.contains('active'))
  })

  test('should load saved deferral state from storage (disabled)', () => {
    mockChrome.storage.local.get = mock.fn((keys, cb) => {
      cb({ deferralEnabled: false })
    })

    loadOptions()

    const toggle = document.getElementById('deferral-toggle')
    assert.ok(!toggle.classList.contains('active'))
  })

  test('should load saved deferral state from storage (enabled)', () => {
    mockChrome.storage.local.get = mock.fn((keys, cb) => {
      cb({ deferralEnabled: true })
    })

    loadOptions()

    const toggle = document.getElementById('deferral-toggle')
    assert.ok(toggle.classList.contains('active'))
  })

  test('should toggle deferral state on click', () => {
    // Start with active state
    mockChrome.storage.local.get = mock.fn((keys, cb) => cb({ deferralEnabled: true }))
    loadOptions()

    const toggle = document.getElementById('deferral-toggle')
    assert.ok(toggle.classList.contains('active'))

    // Toggle (simulates click handler)
    toggleDeferral()
    assert.ok(!toggle.classList.contains('active'))

    // Toggle again
    toggleDeferral()
    assert.ok(toggle.classList.contains('active'))
  })

  test('should include deferralEnabled in save', () => {
    mockChrome.storage.local.get = mock.fn((keys, cb) => cb({}))
    loadOptions()

    // Toggle is active by default
    saveOptions()

    assert.ok(mockChrome.storage.local.set.mock.calls.some((c) => c.arguments[0].deferralEnabled === true))
  })

  test('should save deferralEnabled=false when toggle is inactive', () => {
    mockChrome.storage.local.get = mock.fn((keys, cb) => cb({ deferralEnabled: false }))
    loadOptions()

    saveOptions()

    assert.ok(mockChrome.storage.local.set.mock.calls.some((c) => c.arguments[0].deferralEnabled === false))
  })

  test('should send setDeferralEnabled message on save', () => {
    mockChrome.storage.local.get = mock.fn((keys, cb) => cb({}))
    loadOptions()

    saveOptions()

    assert.ok(
      mockChrome.runtime.sendMessage.mock.calls.some(
        (c) => c.arguments[0].type === 'setDeferralEnabled' && c.arguments[0].enabled === true
      )
    )
  })

  test('should send setDeferralEnabled=false when disabled', () => {
    mockChrome.storage.local.get = mock.fn((keys, cb) => cb({ deferralEnabled: false }))
    loadOptions()

    saveOptions()

    assert.ok(
      mockChrome.runtime.sendMessage.mock.calls.some(
        (c) => c.arguments[0].type === 'setDeferralEnabled' && c.arguments[0].enabled === false
      )
    )
  })
})

describe('Options Screenshot Toggle', () => {
  beforeEach(() => {
    globalThis.document = createMockDocument()
    mockChrome.runtime.sendMessage = mock.fn()
    mockChrome.storage.local.get = mock.fn((keys, cb) => cb({}))
    mockChrome.storage.local.set = mock.fn((data, cb) => cb && cb())
  })

  test('should not activate screenshot toggle when no saved value (default: off)', () => {
    mockChrome.storage.local.get = mock.fn((keys, cb) => cb({}))

    loadOptions()

    const toggle = document.getElementById('screenshot-toggle')
    assert.ok(!toggle.classList.contains('active'))
  })

  test('should activate screenshot toggle when saved as true', () => {
    mockChrome.storage.local.get = mock.fn((keys, cb) => {
      cb({ screenshotOnError: true })
    })

    loadOptions()

    const toggle = document.getElementById('screenshot-toggle')
    assert.ok(toggle.classList.contains('active'))
  })

  test('should not activate screenshot toggle when saved as false', () => {
    mockChrome.storage.local.get = mock.fn((keys, cb) => {
      cb({ screenshotOnError: false })
    })

    loadOptions()

    const toggle = document.getElementById('screenshot-toggle')
    assert.ok(!toggle.classList.contains('active'))
  })

  test('should include screenshotOnError=false in save when inactive', () => {
    mockChrome.storage.local.get = mock.fn((keys, cb) => cb({}))
    loadOptions()

    saveOptions()

    assert.ok(mockChrome.storage.local.set.mock.calls.some((c) => c.arguments[0].screenshotOnError === false))
  })

  test('should include screenshotOnError=true in save when active', () => {
    mockChrome.storage.local.get = mock.fn((keys, cb) => cb({ screenshotOnError: true }))
    loadOptions()

    saveOptions()

    assert.ok(mockChrome.storage.local.set.mock.calls.some((c) => c.arguments[0].screenshotOnError === true))
  })

  test('should send setScreenshotOnError message on save', () => {
    mockChrome.storage.local.get = mock.fn((keys, cb) => cb({ screenshotOnError: true }))
    loadOptions()

    saveOptions()

    assert.ok(
      mockChrome.runtime.sendMessage.mock.calls.some(
        (c) => c.arguments[0].type === 'setScreenshotOnError' && c.arguments[0].enabled === true
      )
    )
  })
})

describe('Options Source Map Toggle', () => {
  beforeEach(() => {
    globalThis.document = createMockDocument()
    mockChrome.runtime.sendMessage = mock.fn()
    mockChrome.storage.local.get = mock.fn((keys, cb) => cb({}))
    mockChrome.storage.local.set = mock.fn((data, cb) => cb && cb())
  })

  test('should not activate source map toggle when no saved value (default: off)', () => {
    mockChrome.storage.local.get = mock.fn((keys, cb) => cb({}))

    loadOptions()

    const toggle = document.getElementById('sourcemap-toggle')
    assert.ok(!toggle.classList.contains('active'))
  })

  test('should activate source map toggle when saved as true', () => {
    mockChrome.storage.local.get = mock.fn((keys, cb) => {
      cb({ sourceMapEnabled: true })
    })

    loadOptions()

    const toggle = document.getElementById('sourcemap-toggle')
    assert.ok(toggle.classList.contains('active'))
  })

  test('should not activate source map toggle when saved as false', () => {
    mockChrome.storage.local.get = mock.fn((keys, cb) => {
      cb({ sourceMapEnabled: false })
    })

    loadOptions()

    const toggle = document.getElementById('sourcemap-toggle')
    assert.ok(!toggle.classList.contains('active'))
  })

  test('should include sourceMapEnabled=false in save when inactive', () => {
    mockChrome.storage.local.get = mock.fn((keys, cb) => cb({}))
    loadOptions()

    saveOptions()

    assert.ok(mockChrome.storage.local.set.mock.calls.some((c) => c.arguments[0].sourceMapEnabled === false))
  })

  test('should include sourceMapEnabled=true in save when active', () => {
    mockChrome.storage.local.get = mock.fn((keys, cb) => cb({ sourceMapEnabled: true }))
    loadOptions()

    saveOptions()

    assert.ok(mockChrome.storage.local.set.mock.calls.some((c) => c.arguments[0].sourceMapEnabled === true))
  })

  test('should send setSourceMapEnabled message on save', () => {
    mockChrome.storage.local.get = mock.fn((keys, cb) => cb({ sourceMapEnabled: true }))
    loadOptions()

    saveOptions()

    assert.ok(
      mockChrome.runtime.sendMessage.mock.calls.some(
        (c) => c.arguments[0].type === 'setSourceMapEnabled' && c.arguments[0].enabled === true
      )
    )
  })
})

describe('Options Debug Mode Toggle', () => {
  beforeEach(() => {
    globalThis.document = createMockDocument()
    mockChrome.runtime.sendMessage = mock.fn()
    mockChrome.storage.local.get = mock.fn((keys, cb) => cb({}))
    mockChrome.storage.local.set = mock.fn((data, cb) => cb && cb())
  })

  test('should not activate debug mode toggle when no saved value (default: off)', () => {
    mockChrome.storage.local.get = mock.fn((keys, cb) => cb({}))

    loadOptions()

    const toggle = document.getElementById('debug-mode-toggle')
    assert.ok(!toggle.classList.contains('active'))
  })

  test('should activate debug mode toggle when saved as true', () => {
    mockChrome.storage.local.get = mock.fn((keys, cb) => {
      cb({ debugMode: true })
    })

    loadOptions()

    const toggle = document.getElementById('debug-mode-toggle')
    assert.ok(toggle.classList.contains('active'))
  })

  test('should not activate debug mode toggle when saved as false', () => {
    mockChrome.storage.local.get = mock.fn((keys, cb) => {
      cb({ debugMode: false })
    })

    loadOptions()

    const toggle = document.getElementById('debug-mode-toggle')
    assert.ok(!toggle.classList.contains('active'))
  })

  test('should toggle debug mode state on click', () => {
    mockChrome.storage.local.get = mock.fn((keys, cb) => cb({ debugMode: true }))
    loadOptions()

    const toggle = document.getElementById('debug-mode-toggle')
    assert.ok(toggle.classList.contains('active'))

    toggleDebugMode()
    assert.ok(!toggle.classList.contains('active'))

    toggleDebugMode()
    assert.ok(toggle.classList.contains('active'))
  })

  test('should include debugMode=false in save when inactive', () => {
    mockChrome.storage.local.get = mock.fn((keys, cb) => cb({}))
    loadOptions()

    saveOptions()

    assert.ok(mockChrome.storage.local.set.mock.calls.some((c) => c.arguments[0].debugMode === false))
  })

  test('should include debugMode=true in save when active', () => {
    mockChrome.storage.local.get = mock.fn((keys, cb) => cb({ debugMode: true }))
    loadOptions()

    saveOptions()

    assert.ok(mockChrome.storage.local.set.mock.calls.some((c) => c.arguments[0].debugMode === true))
  })

  test('should send setDebugMode message on save', () => {
    mockChrome.storage.local.get = mock.fn((keys, cb) => cb({ debugMode: true }))
    loadOptions()

    saveOptions()

    assert.ok(
      mockChrome.runtime.sendMessage.mock.calls.some(
        (c) => c.arguments[0].type === 'setDebugMode' && c.arguments[0].enabled === true
      )
    )
  })

  test('should send setDebugMode=false when disabled', () => {
    mockChrome.storage.local.get = mock.fn((keys, cb) => cb({ debugMode: false }))
    loadOptions()

    saveOptions()

    assert.ok(
      mockChrome.runtime.sendMessage.mock.calls.some(
        (c) => c.arguments[0].type === 'setDebugMode' && c.arguments[0].enabled === false
      )
    )
  })
})
