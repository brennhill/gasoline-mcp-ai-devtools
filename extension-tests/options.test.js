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
    onMessage: { addListener: mock.fn() },
  },
  storage: {
    local: {
      get: mock.fn((keys, cb) => cb({})),
      set: mock.fn((data, cb) => cb && cb()),
    },
  },
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
            },
          },
          style: {},
          addEventListener: mock.fn(),
        }
      }
      return elements[id]
    },
    addEventListener: mock.fn(),
    readyState: 'complete',
    _elements: elements,
  }
}

globalThis.document = createMockDocument()

const { loadOptions, saveOptions, toggleDeferral } = await import('../extension/options.js')

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
        (c) => c.arguments[0].type === 'setDeferralEnabled' && c.arguments[0].enabled === true,
      ),
    )
  })

  test('should send setDeferralEnabled=false when disabled', () => {
    mockChrome.storage.local.get = mock.fn((keys, cb) => cb({ deferralEnabled: false }))
    loadOptions()

    saveOptions()

    assert.ok(
      mockChrome.runtime.sendMessage.mock.calls.some(
        (c) => c.arguments[0].type === 'setDeferralEnabled' && c.arguments[0].enabled === false,
      ),
    )
  })
})
