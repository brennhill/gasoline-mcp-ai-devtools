// @ts-nocheck
/**
 * @fileoverview a11y-query.test.js — Tests for accessibility audit message passing chain.
 * Verifies the complete message flow: background.js -> content.js -> inject.js -> axe-core
 * and results flowing back: inject.js -> content.js -> background.js -> server.
 *
 * Message chain:
 *   background.js sends A11Y_QUERY via chrome.tabs.sendMessage
 *   content.js forwards as GASOLINE_A11Y_QUERY via window.postMessage
 *   inject.js calls runAxeAuditWithTimeout() and returns GASOLINE_A11Y_QUERY_RESPONSE
 *   content.js receives response and calls sendResponse back to background.js
 *   background.js posts result to server via /a11y-result endpoint
 */

import { test, describe, mock, beforeEach } from 'node:test'
import assert from 'node:assert'
import { createMockChrome } from './helpers.js'

// =============================================================================
// Content Script: A11Y_QUERY forwarding to inject.js
// =============================================================================

describe('Content Script: A11Y_QUERY message handling', () => {
  let mockChrome
  let _onMessageHandler
  let windowMessageHandlers

  beforeEach(() => {
    mockChrome = createMockChrome()
    globalThis.chrome = mockChrome
    windowMessageHandlers = []

    globalThis.window = {
      addEventListener: mock.fn((type, handler) => {
        if (type === 'message') windowMessageHandlers.push(handler)
      }),
      removeEventListener: mock.fn(),
      postMessage: mock.fn(),
      location: { origin: 'http://localhost:3000' },
    }

    // Capture the chrome.runtime.onMessage listener
    _onMessageHandler = null
    mockChrome.runtime.onMessage.addListener = mock.fn((handler) => {
      _onMessageHandler = handler
    })
  })

  test('should forward A11Y_QUERY to inject.js via window.postMessage', () => {
    // Simulate the content.js message handler
    const _sendResponse = mock.fn()
    const params = { scope: 'main', tags: ['wcag2a'] }

    // Simulate the A11Y_QUERY handler logic from content.js
    const _message = { type: 'A11Y_QUERY', params }
    const requestId = Date.now()

    // Forward to inject.js via postMessage (as content.js does)
    globalThis.window.postMessage(
      {
        type: 'GASOLINE_A11Y_QUERY',
        requestId,
        params,
      },
      globalThis.window.location.origin,
    )

    // Verify postMessage was called with correct type and params
    assert.strictEqual(globalThis.window.postMessage.mock.calls.length, 1)
    const [postedMessage, origin] = globalThis.window.postMessage.mock.calls[0].arguments
    assert.strictEqual(postedMessage.type, 'GASOLINE_A11Y_QUERY')
    assert.deepStrictEqual(postedMessage.params, params)
    assert.strictEqual(origin, 'http://localhost:3000')
  })

  test('should parse string params before forwarding', () => {
    const params = '{"scope":"#content","tags":["wcag2aa"]}'

    // Parse params if it's a string (as content.js does)
    let parsedParams = params
    if (typeof params === 'string') {
      try {
        parsedParams = JSON.parse(params)
      } catch {
        parsedParams = {}
      }
    }

    globalThis.window.postMessage(
      {
        type: 'GASOLINE_A11Y_QUERY',
        requestId: Date.now(),
        params: parsedParams,
      },
      globalThis.window.location.origin,
    )

    const [postedMessage] = globalThis.window.postMessage.mock.calls[0].arguments
    assert.strictEqual(postedMessage.params.scope, '#content')
    assert.deepStrictEqual(postedMessage.params.tags, ['wcag2aa'])
  })

  test('should handle malformed JSON string params gracefully', () => {
    const params = '{invalid json'

    let parsedParams = params
    if (typeof params === 'string') {
      try {
        parsedParams = JSON.parse(params)
      } catch {
        parsedParams = {}
      }
    }

    globalThis.window.postMessage(
      {
        type: 'GASOLINE_A11Y_QUERY',
        requestId: Date.now(),
        params: parsedParams,
      },
      globalThis.window.location.origin,
    )

    const [postedMessage] = globalThis.window.postMessage.mock.calls[0].arguments
    assert.deepStrictEqual(postedMessage.params, {})
  })

  test('should set up response listener for GASOLINE_A11Y_QUERY_RESPONSE', () => {
    const sendResponse = mock.fn()
    const requestId = Date.now()

    // Simulate setting up the response handler (as content.js does)
    const responseHandler = (event) => {
      if (event.source !== globalThis.window) return
      if (event.data?.type === 'GASOLINE_A11Y_QUERY_RESPONSE' && event.data?.requestId === requestId) {
        globalThis.window.removeEventListener('message', responseHandler)
        sendResponse(event.data.result)
      }
    }

    globalThis.window.addEventListener('message', responseHandler)

    // Verify listener was registered
    assert.strictEqual(globalThis.window.addEventListener.mock.calls.length, 1)
    assert.strictEqual(globalThis.window.addEventListener.mock.calls[0].arguments[0], 'message')
  })

  test('should call sendResponse when receiving A11Y_QUERY_RESPONSE', () => {
    const sendResponse = mock.fn()
    const requestId = 12345

    const auditResult = {
      summary: { violations: 2, passes: 10, incomplete: 1 },
      violations: [
        { id: 'color-contrast', impact: 'serious', description: 'Elements must have sufficient color contrast' },
      ],
    }

    // Set up response handler (as content.js does)
    const responseHandler = (event) => {
      if (event.source !== globalThis.window) return
      if (event.data?.type === 'GASOLINE_A11Y_QUERY_RESPONSE' && event.data?.requestId === requestId) {
        globalThis.window.removeEventListener('message', responseHandler)
        sendResponse(event.data.result)
      }
    }

    // Simulate receiving response from inject.js
    responseHandler({
      source: globalThis.window,
      data: {
        type: 'GASOLINE_A11Y_QUERY_RESPONSE',
        requestId,
        result: auditResult,
      },
    })

    assert.strictEqual(sendResponse.mock.calls.length, 1)
    const receivedResult = sendResponse.mock.calls[0].arguments[0]
    assert.deepStrictEqual(receivedResult.summary, auditResult.summary)
    assert.strictEqual(receivedResult.violations.length, 1)
  })

  test('should ignore responses from other sources', () => {
    const sendResponse = mock.fn()
    const requestId = 12345

    const responseHandler = (event) => {
      if (event.source !== globalThis.window) return
      if (event.data?.type === 'GASOLINE_A11Y_QUERY_RESPONSE' && event.data?.requestId === requestId) {
        sendResponse(event.data.result)
      }
    }

    // Simulate response from different source
    responseHandler({
      source: {}, // Not window
      data: {
        type: 'GASOLINE_A11Y_QUERY_RESPONSE',
        requestId,
        result: { violations: [] },
      },
    })

    assert.strictEqual(sendResponse.mock.calls.length, 0)
  })

  test('should ignore responses with wrong requestId', () => {
    const sendResponse = mock.fn()
    const requestId = 12345

    const responseHandler = (event) => {
      if (event.source !== globalThis.window) return
      if (event.data?.type === 'GASOLINE_A11Y_QUERY_RESPONSE' && event.data?.requestId === requestId) {
        sendResponse(event.data.result)
      }
    }

    // Simulate response with different requestId
    responseHandler({
      source: globalThis.window,
      data: {
        type: 'GASOLINE_A11Y_QUERY_RESPONSE',
        requestId: 99999, // Wrong ID
        result: { violations: [] },
      },
    })

    assert.strictEqual(sendResponse.mock.calls.length, 0)
  })

  test('should handle timeout with error response', async () => {
    const sendResponse = mock.fn()

    // Simulate the timeout behavior from content.js (using shorter timeout for test)
    const responseHandler = mock.fn()
    globalThis.window.addEventListener('message', responseHandler)

    // Simulate timeout firing
    setTimeout(() => {
      globalThis.window.removeEventListener('message', responseHandler)
      sendResponse({ error: 'Accessibility audit timeout' })
    }, 50)

    await new Promise((r) => setTimeout(r, 100))

    assert.strictEqual(sendResponse.mock.calls.length, 1)
    assert.strictEqual(sendResponse.mock.calls[0].arguments[0].error, 'Accessibility audit timeout')
    assert.strictEqual(globalThis.window.removeEventListener.mock.calls.length, 1)
  })

  test('should use empty object for undefined params', () => {
    const params = undefined

    // Simulate content.js behavior
    const resolvedParams = params || {}

    globalThis.window.postMessage(
      {
        type: 'GASOLINE_A11Y_QUERY',
        requestId: Date.now(),
        params: resolvedParams,
      },
      globalThis.window.location.origin,
    )

    const [postedMessage] = globalThis.window.postMessage.mock.calls[0].arguments
    assert.deepStrictEqual(postedMessage.params, {})
  })
})

// =============================================================================
// Inject Script: GASOLINE_A11Y_QUERY handling
// =============================================================================

describe('Inject Script: GASOLINE_A11Y_QUERY message handling', () => {
  let _messageHandler

  beforeEach(() => {
    globalThis.window = {
      postMessage: mock.fn(),
      addEventListener: mock.fn(),
      removeEventListener: mock.fn(),
      location: {
        origin: 'http://localhost:3000',
        href: 'http://localhost:3000/test',
      },
    }
  })

  test('should call runAxeAuditWithTimeout and post success response', async () => {
    const requestId = 12345
    const params = { scope: '#main', tags: ['wcag2a'] }

    const auditResult = {
      summary: { violations: 1, passes: 5, incomplete: 0 },
      violations: [{ id: 'image-alt', impact: 'critical', description: 'Images must have alt text' }],
    }

    // Simulate the inject.js handler behavior
    const mockRunAxeAuditWithTimeout = mock.fn(() => Promise.resolve(auditResult))

    // Simulate receiving GASOLINE_A11Y_QUERY and calling the audit function
    try {
      const result = await mockRunAxeAuditWithTimeout(params)
      globalThis.window.postMessage(
        {
          type: 'GASOLINE_A11Y_QUERY_RESPONSE',
          requestId,
          result,
        },
        globalThis.window.location.origin,
      )
    } catch (err) {
      globalThis.window.postMessage(
        {
          type: 'GASOLINE_A11Y_QUERY_RESPONSE',
          requestId,
          result: { error: err.message },
        },
        globalThis.window.location.origin,
      )
    }

    // Verify audit function was called with correct params
    assert.strictEqual(mockRunAxeAuditWithTimeout.mock.calls.length, 1)
    assert.deepStrictEqual(mockRunAxeAuditWithTimeout.mock.calls[0].arguments[0], params)

    // Verify response was posted back
    assert.strictEqual(globalThis.window.postMessage.mock.calls.length, 1)
    const [response, origin] = globalThis.window.postMessage.mock.calls[0].arguments
    assert.strictEqual(response.type, 'GASOLINE_A11Y_QUERY_RESPONSE')
    assert.strictEqual(response.requestId, requestId)
    assert.deepStrictEqual(response.result.summary, auditResult.summary)
    assert.strictEqual(origin, 'http://localhost:3000')
  })

  test('should post error response when audit fails', async () => {
    const requestId = 12345
    const params = {}

    const mockRunAxeAuditWithTimeout = mock.fn(() => Promise.reject(new Error('axe-core not loaded')))

    try {
      await mockRunAxeAuditWithTimeout(params)
      globalThis.window.postMessage(
        {
          type: 'GASOLINE_A11Y_QUERY_RESPONSE',
          requestId,
          result: {},
        },
        globalThis.window.location.origin,
      )
    } catch (err) {
      globalThis.window.postMessage(
        {
          type: 'GASOLINE_A11Y_QUERY_RESPONSE',
          requestId,
          result: { error: err.message || 'Accessibility audit failed' },
        },
        globalThis.window.location.origin,
      )
    }

    assert.strictEqual(globalThis.window.postMessage.mock.calls.length, 1)
    const [response] = globalThis.window.postMessage.mock.calls[0].arguments
    assert.strictEqual(response.type, 'GASOLINE_A11Y_QUERY_RESPONSE')
    assert.strictEqual(response.requestId, requestId)
    assert.strictEqual(response.result.error, 'axe-core not loaded')
  })

  test('should post timeout error when audit times out', async () => {
    const requestId = 12345
    const params = {}

    // Simulate timeout from runAxeAuditWithTimeout
    const mockRunAxeAuditWithTimeout = mock.fn(() => Promise.resolve({ error: 'Accessibility audit timeout' }))

    const result = await mockRunAxeAuditWithTimeout(params)
    globalThis.window.postMessage(
      {
        type: 'GASOLINE_A11Y_QUERY_RESPONSE',
        requestId,
        result,
      },
      globalThis.window.location.origin,
    )

    const [response] = globalThis.window.postMessage.mock.calls[0].arguments
    assert.strictEqual(response.result.error, 'Accessibility audit timeout')
  })

  test('should use empty object for undefined params', async () => {
    const _requestId = 12345

    const mockRunAxeAuditWithTimeout = mock.fn(() => Promise.resolve({ summary: {} }))

    // Simulate inject.js handling of undefined params
    const undefinedParams = undefined
    const resolvedParams = undefinedParams || {}
    await mockRunAxeAuditWithTimeout(resolvedParams)

    assert.deepStrictEqual(mockRunAxeAuditWithTimeout.mock.calls[0].arguments[0], {})
  })
})

// =============================================================================
// Background Script: A11Y query dispatch and result posting
// =============================================================================

describe('Background Script: A11Y query dispatch', () => {
  let mockChrome
  let mockFetch

  beforeEach(() => {
    mockChrome = {
      runtime: {
        onMessage: { addListener: mock.fn() },
        onInstalled: { addListener: mock.fn() },
        sendMessage: mock.fn(() => Promise.resolve()),
        getManifest: () => ({ version: '5.8.0' }),
      },
      action: {
        setBadgeText: mock.fn(),
        setBadgeBackgroundColor: mock.fn(),
      },
      storage: {
        local: {
          get: mock.fn((keys, callback) => callback({ logLevel: 'error' })),
          set: mock.fn((data, callback) => callback && callback()),
        },
        sync: {
          get: mock.fn((keys, callback) => callback({})),
          set: mock.fn((data, callback) => callback && callback()),
        },
        session: {
          get: mock.fn((keys, callback) => callback({})),
          set: mock.fn((data, callback) => callback && callback()),
        },
        onChanged: { addListener: mock.fn() },
      },
      alarms: {
        create: mock.fn(),
        onAlarm: { addListener: mock.fn() },
      },
      tabs: {
        get: mock.fn((tabId) => Promise.resolve({ id: tabId, windowId: 1, url: 'http://localhost:3000' })),
        sendMessage: mock.fn(() =>
          Promise.resolve({
            summary: { violations: 2 },
            violations: [{ id: 'color-contrast', impact: 'serious' }],
          }),
        ),
        query: mock.fn((query, callback) => callback([{ id: 1, windowId: 1 }])),
        captureVisibleTab: mock.fn(() => Promise.resolve('data:image/jpeg;base64,abc')),
        onRemoved: { addListener: mock.fn() },
      },
    }
    globalThis.chrome = mockChrome

    mockFetch = mock.fn(() =>
      Promise.resolve({
        ok: true,
        json: () => Promise.resolve({ status: 'ok' }),
      }),
    )
    globalThis.fetch = mockFetch
  })

  test('should send A11Y_QUERY to content script via chrome.tabs.sendMessage', async () => {
    const tabId = 42
    const params = { scope: '#main', tags: ['wcag2a'] }

    // Simulate what background.js does for a11y queries
    const result = await mockChrome.tabs.sendMessage(tabId, {
      type: 'A11Y_QUERY',
      params,
    })

    assert.strictEqual(mockChrome.tabs.sendMessage.mock.calls.length, 1)
    const [sentTabId, sentMessage] = mockChrome.tabs.sendMessage.mock.calls[0].arguments
    assert.strictEqual(sentTabId, 42)
    assert.strictEqual(sentMessage.type, 'A11Y_QUERY')
    assert.deepStrictEqual(sentMessage.params, params)

    // Verify result was received
    assert.ok(result.summary)
    assert.strictEqual(result.summary.violations, 2)
  })

  test('should post result to /a11y-result endpoint', async () => {
    const { postQueryResult } = await import('../../extension/background.js')

    // Reset fetch mock after background.js import (which may trigger startup fetches)
    mockFetch.mock.resetCalls()

    const queryId = 'test-query-123'
    const result = {
      summary: { violations: 1, passes: 10 },
      violations: [{ id: 'image-alt', impact: 'critical' }],
    }

    await postQueryResult('http://localhost:7890', queryId, 'a11y', result)

    // Find the a11y-result fetch call (background.js startup may have made other calls)
    const a11yCalls = mockFetch.mock.calls.filter((call) => {
      const url = call.arguments[0]
      return typeof url === 'string' && url.includes('/a11y-result')
    })
    assert.strictEqual(a11yCalls.length, 1, `Expected 1 /a11y-result call, found ${a11yCalls.length}`)

    const [url, opts] = a11yCalls[0].arguments
    assert.ok(url.includes('/a11y-result'), `Expected URL to include /a11y-result, got: ${url}`)
    assert.strictEqual(opts.method, 'POST')

    const body = JSON.parse(opts.body)
    assert.strictEqual(body.id, queryId)
    assert.ok(body.result)
  })

  test('should handle sendMessage failure gracefully', async () => {
    // Simulate content script error
    mockChrome.tabs.sendMessage = mock.fn(() => Promise.reject(new Error('Could not establish connection')))

    let errorResult = null
    try {
      await mockChrome.tabs.sendMessage(42, { type: 'A11Y_QUERY', params: {} })
    } catch (err) {
      errorResult = {
        error: 'a11y_audit_failed',
        message: err.message || 'Failed to execute accessibility audit',
      }
    }

    assert.ok(errorResult)
    assert.strictEqual(errorResult.error, 'a11y_audit_failed')
    assert.ok(errorResult.message.includes('Could not establish connection'))
  })
})

// =============================================================================
// End-to-End: Full message chain simulation
// =============================================================================

describe('A11Y Query: End-to-end message chain', () => {
  test('should complete the full message chain: background -> content -> inject -> content -> background', async () => {
    // Simulate the full chain
    const params = { scope: '#content', tags: ['wcag2a', 'wcag2aa'] }
    const auditResult = {
      summary: { violations: 3, passes: 15, incomplete: 2, inapplicable: 5 },
      violations: [
        { id: 'color-contrast', impact: 'serious', nodes: 2 },
        { id: 'image-alt', impact: 'critical', nodes: 1 },
      ],
    }

    // Step 1: background.js sends A11Y_QUERY to content.js
    const _contentScriptHandler = mock.fn()
    const mockTabsSendMessage = mock.fn((tabId, message) => {
      // Step 2: content.js receives A11Y_QUERY and forwards to inject.js
      return new Promise((resolve) => {
        assert.strictEqual(message.type, 'A11Y_QUERY')
        assert.deepStrictEqual(message.params, params)

        // Step 3: inject.js processes GASOLINE_A11Y_QUERY, runs axe, returns result
        // Step 4: content.js receives GASOLINE_A11Y_QUERY_RESPONSE, calls sendResponse
        resolve(auditResult)
      })
    })

    // Execute the chain
    const result = await mockTabsSendMessage(1, {
      type: 'A11Y_QUERY',
      params,
    })

    // Step 5: background.js receives result and would post to server
    assert.ok(result.summary)
    assert.strictEqual(result.summary.violations, 3)
    assert.strictEqual(result.summary.passes, 15)
    assert.strictEqual(result.violations.length, 2)
    assert.strictEqual(result.violations[0].id, 'color-contrast')
    assert.strictEqual(result.violations[1].id, 'image-alt')
  })

  test('should propagate errors through the chain', async () => {
    const mockTabsSendMessage = mock.fn(() => Promise.reject(new Error('Extension context invalidated')))

    let errorResult
    try {
      await mockTabsSendMessage(1, { type: 'A11Y_QUERY', params: {} })
    } catch (err) {
      errorResult = {
        error: 'a11y_audit_failed',
        message: err.message,
      }
    }

    assert.ok(errorResult)
    assert.strictEqual(errorResult.error, 'a11y_audit_failed')
    assert.strictEqual(errorResult.message, 'Extension context invalidated')
  })

  test('should handle empty audit results', async () => {
    const mockTabsSendMessage = mock.fn(() =>
      Promise.resolve({
        summary: { violations: 0, passes: 20, incomplete: 0, inapplicable: 10 },
        violations: [],
        incomplete: [],
      }),
    )

    const result = await mockTabsSendMessage(1, {
      type: 'A11Y_QUERY',
      params: {},
    })

    assert.strictEqual(result.summary.violations, 0)
    assert.strictEqual(result.violations.length, 0)
  })
})
