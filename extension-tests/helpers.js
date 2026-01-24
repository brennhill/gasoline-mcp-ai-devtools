// @ts-nocheck
/**
 * @fileoverview Shared test infrastructure for extension tests.
 * Provides configurable mock factories and query helpers.
 *
 * Usage: Import only what each test file needs.
 *   import { createMockWindow, createMockCrypto } from './helpers.js'
 */

import { mock } from 'node:test'

/**
 * Configurable mock window factory.
 * Options allow each test file to get exactly the shape it needs.
 *
 * @param {Object} options
 * @param {string} [options.pathname] - window.location.pathname (default: '/')
 * @param {string} [options.hostname] - window.location.hostname (default: 'localhost')
 * @param {string} [options.href] - window.location.href (default: 'http://localhost/')
 * @param {boolean} [options.withWebSocket] - Include MockWebSocket class on the window
 * @param {boolean} [options.withFetch] - Include mock fetch function on the window
 * @param {boolean} [options.withOnerror] - Include onerror/onunhandledrejection fields
 * @param {Object} [options.overrides] - Additional properties to spread onto the window
 * @returns {Object} Mock window object
 */
export function createMockWindow(options = {}) {
  const base = {
    postMessage: mock.fn(),
    addEventListener: mock.fn(),
    removeEventListener: mock.fn(),
    location: {
      pathname: options.pathname || '/',
      hostname: options.hostname || 'localhost',
      href: options.href || 'http://localhost/',
    },
  }

  if (options.withOnerror) {
    base.onerror = null
    base.onunhandledrejection = null
  }

  if (options.withWebSocket) {
    class MockWebSocket {
      constructor(url, protocols) {
        this.url = url
        this.protocols = protocols
        this.readyState = 0
        this._listeners = {}
      }
      addEventListener(event, handler) {
        if (!this._listeners[event]) this._listeners[event] = []
        this._listeners[event].push(handler)
      }
      send(_data) {}
      close(_code, _reason) {}
      // Helper for tests to simulate events
      _emit(event, data) {
        if (this._listeners[event]) {
          this._listeners[event].forEach((h) => h(data))
        }
      }
    }
    MockWebSocket.CONNECTING = 0
    MockWebSocket.OPEN = 1
    MockWebSocket.CLOSING = 2
    MockWebSocket.CLOSED = 3

    base.WebSocket = MockWebSocket
  }

  if (options.withFetch) {
    base.fetch = mock.fn()
  }

  return { ...base, ...options.overrides }
}

/**
 * Mock crypto for UUID generation.
 * Returns a consistent mock that generates predictable-format UUIDs for testing.
 *
 * @returns {Object} Mock crypto object with randomUUID
 */
export function createMockCrypto() {
  return {
    randomUUID: mock.fn(() => 'test-uuid-' + Math.random().toString(36).slice(2)),
  }
}

/**
 * Mock console factory with all standard methods as mock functions.
 *
 * @returns {Object} Mock console object
 */
export function createMockConsole() {
  return {
    log: mock.fn(),
    warn: mock.fn(),
    error: mock.fn(),
    info: mock.fn(),
    debug: mock.fn(),
  }
}

/**
 * Mock Chrome API factory for content script tests.
 *
 * @param {Object} [overrides] - Override specific Chrome API methods
 * @returns {Object} Mock chrome object
 */
export function createMockChrome(overrides = {}) {
  return {
    runtime: {
      getURL: mock.fn((path) => `chrome-extension://abc123/${path}`),
      sendMessage: mock.fn(),
      onMessage: {
        addListener: mock.fn(),
      },
      ...overrides.runtime,
    },
    ...overrides,
  }
}

/**
 * Mock document factory for tests that need DOM interactions.
 *
 * @param {Object} [overrides] - Additional properties to merge
 * @returns {Object} Mock document object
 */
export function createMockDocument(overrides = {}) {
  return {
    addEventListener: mock.fn(),
    removeEventListener: mock.fn(),
    ...overrides,
  }
}
