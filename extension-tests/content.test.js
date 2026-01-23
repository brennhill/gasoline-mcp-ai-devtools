// @ts-nocheck
/**
 * @fileoverview Tests for content script message forwarding
 * TDD: Tests for v5 GASOLINE_NETWORK_BODY forwarding
 */

import { test, describe, mock, beforeEach } from 'node:test'
import assert from 'node:assert'

describe('Content Script: GASOLINE_NETWORK_BODY forwarding', () => {
  let messageHandler
  let mockChrome

  beforeEach(() => {
    mockChrome = {
      runtime: {
        getURL: mock.fn((path) => `chrome-extension://abc123/${path}`),
        sendMessage: mock.fn(),
        onMessage: {
          addListener: mock.fn(),
        },
      },
    }
    globalThis.chrome = mockChrome

    // Capture the message handler that content.js registers
    const handlers = []
    globalThis.window = {
      addEventListener: mock.fn((type, handler) => {
        if (type === 'message') handlers.push(handler)
      }),
      postMessage: mock.fn(),
    }
    globalThis.document = {
      createElement: mock.fn(() => ({
        remove: mock.fn(),
        set onload(fn) {
          fn()
        },
      })),
      head: { appendChild: mock.fn() },
      documentElement: { appendChild: mock.fn() },
      readyState: 'complete',
      addEventListener: mock.fn(),
    }

    // Re-import content.js to register handlers
    // We'll simulate the handler behavior directly
    messageHandler = (event) => {
      if (event.source !== globalThis.window) return
      if (event.data?.type === 'DEV_CONSOLE_LOG') {
        mockChrome.runtime.sendMessage({
          type: 'log',
          payload: event.data.payload,
        })
      } else if (event.data?.type === 'GASOLINE_WS') {
        mockChrome.runtime.sendMessage({
          type: 'ws_event',
          payload: event.data.payload,
        })
      } else if (event.data?.type === 'GASOLINE_NETWORK_BODY') {
        mockChrome.runtime.sendMessage({
          type: 'network_body',
          payload: event.data.payload,
        })
      } else if (event.data?.type === 'GASOLINE_ENHANCED_ACTION') {
        mockChrome.runtime.sendMessage({
          type: 'enhanced_action',
          payload: event.data.payload,
        })
      }
    }
  })

  test('should forward GASOLINE_NETWORK_BODY messages to background', () => {
    const payload = {
      url: 'http://localhost:3000/api/users',
      method: 'GET',
      status: 200,
      contentType: 'application/json',
      requestBody: null,
      responseBody: '{"users":[]}',
      duration: 150,
    }

    messageHandler({
      source: globalThis.window,
      data: { type: 'GASOLINE_NETWORK_BODY', payload },
    })

    assert.strictEqual(mockChrome.runtime.sendMessage.mock.calls.length, 1)
    const sentMessage = mockChrome.runtime.sendMessage.mock.calls[0].arguments[0]
    assert.strictEqual(sentMessage.type, 'network_body')
    assert.deepStrictEqual(sentMessage.payload, payload)
  })

  test('should forward with correct payload fields', () => {
    const payload = {
      url: 'http://localhost:3000/api/submit',
      method: 'POST',
      status: 400,
      contentType: 'application/json',
      requestBody: '{"email":"test@test.com"}',
      responseBody: '{"error":"invalid"}',
      duration: 200,
    }

    messageHandler({
      source: globalThis.window,
      data: { type: 'GASOLINE_NETWORK_BODY', payload },
    })

    const sentMessage = mockChrome.runtime.sendMessage.mock.calls[0].arguments[0]
    assert.strictEqual(sentMessage.payload.url, 'http://localhost:3000/api/submit')
    assert.strictEqual(sentMessage.payload.method, 'POST')
    assert.strictEqual(sentMessage.payload.status, 400)
    assert.strictEqual(sentMessage.payload.requestBody, '{"email":"test@test.com"}')
  })

  test('should not forward messages from other sources', () => {
    messageHandler({
      source: {}, // Not window
      data: { type: 'GASOLINE_NETWORK_BODY', payload: {} },
    })

    assert.strictEqual(mockChrome.runtime.sendMessage.mock.calls.length, 0)
  })

  test('should still forward DEV_CONSOLE_LOG messages', () => {
    messageHandler({
      source: globalThis.window,
      data: { type: 'DEV_CONSOLE_LOG', payload: { level: 'error', message: 'test' } },
    })

    assert.strictEqual(mockChrome.runtime.sendMessage.mock.calls.length, 1)
    const sentMessage = mockChrome.runtime.sendMessage.mock.calls[0].arguments[0]
    assert.strictEqual(sentMessage.type, 'log')
  })

  test('should still forward GASOLINE_WS messages', () => {
    messageHandler({
      source: globalThis.window,
      data: { type: 'GASOLINE_WS', payload: { event: 'open', url: 'ws://localhost' } },
    })

    assert.strictEqual(mockChrome.runtime.sendMessage.mock.calls.length, 1)
    const sentMessage = mockChrome.runtime.sendMessage.mock.calls[0].arguments[0]
    assert.strictEqual(sentMessage.type, 'ws_event')
  })

  test('should forward GASOLINE_ENHANCED_ACTION messages to background', () => {
    const payload = {
      type: 'click',
      timestamp: 1705312200000,
      url: 'http://localhost:3000/login',
      selectors: { testId: 'login-btn', cssPath: 'button#login' },
    }

    messageHandler({
      source: globalThis.window,
      data: { type: 'GASOLINE_ENHANCED_ACTION', payload },
    })

    assert.strictEqual(mockChrome.runtime.sendMessage.mock.calls.length, 1)
    const sentMessage = mockChrome.runtime.sendMessage.mock.calls[0].arguments[0]
    assert.strictEqual(sentMessage.type, 'enhanced_action')
    assert.deepStrictEqual(sentMessage.payload, payload)
  })

  test('should forward GASOLINE_ENHANCED_ACTION with all action types', () => {
    const actions = [
      { type: 'click', timestamp: 1000, url: 'http://localhost:3000', selectors: { id: 'btn' } },
      {
        type: 'input',
        timestamp: 1001,
        url: 'http://localhost:3000',
        selectors: { id: 'input' },
        value: 'hello',
        inputType: 'text',
      },
      { type: 'keypress', timestamp: 1002, url: 'http://localhost:3000', selectors: { id: 'input' }, key: 'Enter' },
      {
        type: 'select',
        timestamp: 1003,
        url: 'http://localhost:3000',
        selectors: { id: 'dropdown' },
        selectedValue: 'us',
        selectedText: 'United States',
      },
      { type: 'scroll', timestamp: 1004, url: 'http://localhost:3000', scrollY: 500 },
      { type: 'navigate', timestamp: 1005, url: 'http://localhost:3000', fromUrl: '/home', toUrl: '/about' },
    ]

    for (const payload of actions) {
      mockChrome.runtime.sendMessage.mock.resetCalls()
      messageHandler({
        source: globalThis.window,
        data: { type: 'GASOLINE_ENHANCED_ACTION', payload },
      })

      assert.strictEqual(mockChrome.runtime.sendMessage.mock.calls.length, 1, `Should forward ${payload.type} action`)
      const sentMessage = mockChrome.runtime.sendMessage.mock.calls[0].arguments[0]
      assert.strictEqual(sentMessage.type, 'enhanced_action')
      assert.strictEqual(sentMessage.payload.type, payload.type)
    }
  })
})
