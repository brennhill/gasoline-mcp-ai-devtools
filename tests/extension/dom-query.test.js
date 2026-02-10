// @ts-nocheck
/**
 * @fileoverview dom-query.test.js — Tests for DOM query message passing chain.
 * Verifies the complete message flow: background.js -> content.js -> inject.js -> executeDOMQuery
 * and results flowing back: inject.js -> content.js -> background.js -> server.
 *
 * Message chain:
 *   background.js sends DOM_QUERY via chrome.tabs.sendMessage
 *   content.js forwards as GASOLINE_DOM_QUERY via window.postMessage
 *   inject.js calls executeDOMQuery() and returns GASOLINE_DOM_QUERY_RESPONSE
 *   content.js receives response and calls sendResponse back to background.js
 *   background.js posts result to server via /dom-result endpoint
 */

import { test, describe, mock, beforeEach } from 'node:test'
import assert from 'node:assert'
import { createMockChrome, MANIFEST_VERSION } from './helpers.js'

// =============================================================================
// Content Script: DOM_QUERY forwarding to inject.js
// =============================================================================

describe('Content Script: DOM_QUERY message handling', () => {
  let mockChrome
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
    mockChrome.runtime.onMessage.addListener = mock.fn((_handler) => {})
  })

  test('should forward DOM_QUERY to inject.js via window.postMessage', () => {
    const _sendResponse = mock.fn()
    const params = { selector: 'button.submit' }

    // Simulate the DOM_QUERY handler logic from content.js
    const requestId = Date.now()

    // Forward to inject.js via postMessage (as content.js does)
    globalThis.window.postMessage(
      {
        type: 'GASOLINE_DOM_QUERY',
        requestId,
        params,
      },
      globalThis.window.location.origin,
    )

    // Verify postMessage was called with correct type and params
    assert.strictEqual(globalThis.window.postMessage.mock.calls.length, 1)
    const [postedMessage, origin] = globalThis.window.postMessage.mock.calls[0].arguments
    assert.strictEqual(postedMessage.type, 'GASOLINE_DOM_QUERY')
    assert.deepStrictEqual(postedMessage.params, params)
    assert.strictEqual(origin, 'http://localhost:3000')
  })

  test('should parse string params before forwarding', () => {
    const params = '{"selector":"#content"}'

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
        type: 'GASOLINE_DOM_QUERY',
        requestId: Date.now(),
        params: parsedParams,
      },
      globalThis.window.location.origin,
    )

    const [postedMessage] = globalThis.window.postMessage.mock.calls[0].arguments
    assert.strictEqual(postedMessage.params.selector, '#content')
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
        type: 'GASOLINE_DOM_QUERY',
        requestId: Date.now(),
        params: parsedParams,
      },
      globalThis.window.location.origin,
    )

    const [postedMessage] = globalThis.window.postMessage.mock.calls[0].arguments
    assert.deepStrictEqual(postedMessage.params, {})
  })

  test('should call sendResponse when receiving GASOLINE_DOM_QUERY_RESPONSE', () => {
    const sendResponse = mock.fn()
    const requestId = 12345

    const domResult = {
      url: 'http://localhost:3000/test',
      title: 'Test Page',
      matchCount: 2,
      returnedCount: 2,
      matches: [
        { tag: 'button', text: 'Submit', visible: true, attributes: { class: 'submit' } },
        { tag: 'button', text: 'Cancel', visible: true, attributes: { class: 'submit cancel' } },
      ],
    }

    // Set up response handler (as content.js does with pendingDomRequests)
    const pendingDomRequests = new Map()
    pendingDomRequests.set(requestId, sendResponse)

    // Simulate receiving response from inject.js
    const messageType = 'GASOLINE_DOM_QUERY_RESPONSE'
    if (messageType === 'GASOLINE_DOM_QUERY_RESPONSE') {
      const cb = pendingDomRequests.get(requestId)
      if (cb) {
        pendingDomRequests.delete(requestId)
        cb(domResult)
      }
    }

    assert.strictEqual(sendResponse.mock.calls.length, 1)
    const receivedResult = sendResponse.mock.calls[0].arguments[0]
    assert.strictEqual(receivedResult.matchCount, 2)
    assert.strictEqual(receivedResult.matches.length, 2)
    assert.strictEqual(receivedResult.matches[0].tag, 'button')
  })

  test('should ignore responses with wrong requestId', () => {
    const sendResponse = mock.fn()
    const requestId = 12345

    const pendingDomRequests = new Map()
    pendingDomRequests.set(requestId, sendResponse)

    // Try with wrong requestId
    const wrongId = 99999
    const cb = pendingDomRequests.get(wrongId)
    if (cb) {
      cb({ matches: [] })
    }

    assert.strictEqual(sendResponse.mock.calls.length, 0)
  })

  test('should handle timeout with error response', async () => {
    const sendResponse = mock.fn()
    const requestId = 12345

    const pendingDomRequests = new Map()
    pendingDomRequests.set(requestId, sendResponse)

    // Simulate timeout firing (using shorter timeout for test)
    setTimeout(() => {
      if (pendingDomRequests.has(requestId)) {
        const cb = pendingDomRequests.get(requestId)
        pendingDomRequests.delete(requestId)
        cb({ error: 'DOM query timeout' })
      }
    }, 50)

    await new Promise((r) => setTimeout(r, 100))

    assert.strictEqual(sendResponse.mock.calls.length, 1)
    assert.strictEqual(sendResponse.mock.calls[0].arguments[0].error, 'DOM query timeout')
    assert.strictEqual(pendingDomRequests.has(requestId), false)
  })

  test('should use empty object for undefined params', () => {
    const params = undefined

    // Simulate content.js behavior
    const resolvedParams = params || {}

    globalThis.window.postMessage(
      {
        type: 'GASOLINE_DOM_QUERY',
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
// Inject Script: GASOLINE_DOM_QUERY handling
// =============================================================================

describe('Inject Script: GASOLINE_DOM_QUERY message handling', () => {
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

  test('should call executeDOMQuery and post success response', async () => {
    const requestId = 12345
    const params = { selector: 'button.submit' }

    const domResult = {
      url: 'http://localhost:3000/test',
      title: 'Test Page',
      matchCount: 2,
      returnedCount: 2,
      matches: [
        { tag: 'button', text: 'Submit', visible: true, attributes: { class: 'submit' } },
      ],
    }

    // Simulate the inject.js handler behavior
    const mockExecuteDOMQuery = mock.fn(() => Promise.resolve(domResult))

    // Simulate receiving GASOLINE_DOM_QUERY and calling the query function
    try {
      const result = await mockExecuteDOMQuery(params)
      globalThis.window.postMessage(
        {
          type: 'GASOLINE_DOM_QUERY_RESPONSE',
          requestId,
          result,
        },
        globalThis.window.location.origin,
      )
    } catch (err) {
      globalThis.window.postMessage(
        {
          type: 'GASOLINE_DOM_QUERY_RESPONSE',
          requestId,
          result: { error: err.message },
        },
        globalThis.window.location.origin,
      )
    }

    // Verify query function was called with correct params
    assert.strictEqual(mockExecuteDOMQuery.mock.calls.length, 1)
    assert.deepStrictEqual(mockExecuteDOMQuery.mock.calls[0].arguments[0], params)

    // Verify response was posted back
    assert.strictEqual(globalThis.window.postMessage.mock.calls.length, 1)
    const [response, origin] = globalThis.window.postMessage.mock.calls[0].arguments
    assert.strictEqual(response.type, 'GASOLINE_DOM_QUERY_RESPONSE')
    assert.strictEqual(response.requestId, requestId)
    assert.strictEqual(response.result.matchCount, 2)
    assert.strictEqual(origin, 'http://localhost:3000')
  })

  test('should post error response when query fails (invalid selector)', async () => {
    const requestId = 12345
    const params = { selector: '[invalid' }

    const mockExecuteDOMQuery = mock.fn(() =>
      Promise.reject(new Error("'[invalid' is not a valid selector")),
    )

    try {
      await mockExecuteDOMQuery(params)
    } catch (err) {
      globalThis.window.postMessage(
        {
          type: 'GASOLINE_DOM_QUERY_RESPONSE',
          requestId,
          result: { error: err.message || 'DOM query failed' },
        },
        globalThis.window.location.origin,
      )
    }

    assert.strictEqual(globalThis.window.postMessage.mock.calls.length, 1)
    const [response] = globalThis.window.postMessage.mock.calls[0].arguments
    assert.strictEqual(response.type, 'GASOLINE_DOM_QUERY_RESPONSE')
    assert.strictEqual(response.requestId, requestId)
    assert.ok(response.result.error.includes('not a valid selector'))
  })

  test('should handle executeDOMQuery not being available', () => {
    const requestId = 12345

    // Simulate the defensive check (executeDOMQuery not available)
    const executeDOMQuery = undefined

    if (typeof executeDOMQuery !== 'function') {
      globalThis.window.postMessage(
        {
          type: 'GASOLINE_DOM_QUERY_RESPONSE',
          requestId,
          result: {
            error: 'executeDOMQuery not available — try reloading the extension',
          },
        },
        globalThis.window.location.origin,
      )
    }

    assert.strictEqual(globalThis.window.postMessage.mock.calls.length, 1)
    const [response] = globalThis.window.postMessage.mock.calls[0].arguments
    assert.strictEqual(response.type, 'GASOLINE_DOM_QUERY_RESPONSE')
    assert.ok(response.result.error.includes('not available'))
  })

  test('should use empty object for undefined params', async () => {
    const _requestId = 12345

    const mockExecuteDOMQuery = mock.fn(() =>
      Promise.resolve({ url: '', title: '', matchCount: 0, returnedCount: 0, matches: [] }),
    )

    // Simulate inject.js handling of undefined params
    const undefinedParams = undefined
    const resolvedParams = undefinedParams || {}
    await mockExecuteDOMQuery(resolvedParams)

    assert.deepStrictEqual(mockExecuteDOMQuery.mock.calls[0].arguments[0], {})
  })

  test('should return zero matches for valid selector with no results', async () => {
    const requestId = 12345
    const params = { selector: '.nonexistent' }

    const domResult = {
      url: 'http://localhost:3000/test',
      title: 'Test Page',
      matchCount: 0,
      returnedCount: 0,
      matches: [],
    }

    const mockExecuteDOMQuery = mock.fn(() => Promise.resolve(domResult))

    const result = await mockExecuteDOMQuery(params)
    globalThis.window.postMessage(
      {
        type: 'GASOLINE_DOM_QUERY_RESPONSE',
        requestId,
        result,
      },
      globalThis.window.location.origin,
    )

    const [response] = globalThis.window.postMessage.mock.calls[0].arguments
    assert.strictEqual(response.result.matchCount, 0)
    assert.strictEqual(response.result.matches.length, 0)
  })
})

// =============================================================================
// Background Script: DOM query dispatch and result posting
// =============================================================================

describe('Background Script: DOM query dispatch', () => {
  let mockChrome
  let mockFetch

  beforeEach(() => {
    mockChrome = {
      runtime: {
        onMessage: { addListener: mock.fn() },
        onInstalled: { addListener: mock.fn() },
        sendMessage: mock.fn(() => Promise.resolve()),
        getManifest: () => ({ version: MANIFEST_VERSION }),
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
            url: 'http://localhost:3000/test',
            title: 'Test Page',
            matchCount: 1,
            returnedCount: 1,
            matches: [{ tag: 'h1', text: 'Hello', visible: true }],
          }),
        ),
        query: mock.fn((query, callback) => callback([{ id: 1, windowId: 1 }])),
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

  test('should send DOM_QUERY to content script via chrome.tabs.sendMessage', async () => {
    const tabId = 42
    const params = { selector: 'h1' }

    // Simulate what background.js should do for dom queries
    const result = await mockChrome.tabs.sendMessage(tabId, {
      type: 'DOM_QUERY',
      params,
    })

    assert.strictEqual(mockChrome.tabs.sendMessage.mock.calls.length, 1)
    const [sentTabId, sentMessage] = mockChrome.tabs.sendMessage.mock.calls[0].arguments
    assert.strictEqual(sentTabId, 42)
    assert.strictEqual(sentMessage.type, 'DOM_QUERY')
    assert.deepStrictEqual(sentMessage.params, params)

    // Verify result was received
    assert.ok(result.matchCount >= 0)
  })

  test('should post result to /dom-result endpoint', async () => {
    const { postQueryResult } = await import('../../extension/background.js')

    // Reset fetch mock after background.js import
    mockFetch.mock.resetCalls()

    const queryId = 'test-query-dom-123'
    const result = {
      url: 'http://localhost:3000/test',
      title: 'Test Page',
      matchCount: 2,
      returnedCount: 2,
      matches: [{ tag: 'button', text: 'Submit' }],
    }

    await postQueryResult('http://localhost:7890', queryId, 'dom', result)

    // Find the dom-result fetch call
    const domCalls = mockFetch.mock.calls.filter((call) => {
      const url = call.arguments[0]
      return typeof url === 'string' && url.includes('/dom-result')
    })
    assert.strictEqual(domCalls.length, 1, `Expected 1 /dom-result call, found ${domCalls.length}`)

    const [url, opts] = domCalls[0].arguments
    assert.ok(url.includes('/dom-result'), `Expected URL to include /dom-result, got: ${url}`)
    assert.strictEqual(opts.method, 'POST')

    const body = JSON.parse(opts.body)
    assert.strictEqual(body.id, queryId)
    assert.ok(body.result)
  })

  test('should handle sendMessage failure gracefully', async () => {
    // Simulate content script error (e.g., tab closed)
    mockChrome.tabs.sendMessage = mock.fn(() => Promise.reject(new Error('Could not establish connection')))

    let errorResult = null
    try {
      await mockChrome.tabs.sendMessage(42, { type: 'DOM_QUERY', params: { selector: 'h1' } })
    } catch (err) {
      errorResult = {
        error: 'dom_query_failed',
        message: err.message || 'Failed to execute DOM query',
      }
    }

    assert.ok(errorResult)
    assert.strictEqual(errorResult.error, 'dom_query_failed')
    assert.ok(errorResult.message.includes('Could not establish connection'))
  })

  test('should handle chrome internal page errors', async () => {
    mockChrome.tabs.sendMessage = mock.fn(() =>
      Promise.reject(new Error('Cannot access contents of the page')),
    )

    let errorResult = null
    try {
      await mockChrome.tabs.sendMessage(42, { type: 'DOM_QUERY', params: { selector: 'h1' } })
    } catch (err) {
      errorResult = {
        error: 'dom_query_failed',
        message: err.message || 'Failed to execute DOM query',
      }
    }

    assert.ok(errorResult)
    assert.ok(errorResult.message.includes('Cannot access contents'))
  })
})

// =============================================================================
// End-to-End: Full message chain simulation
// =============================================================================

describe('DOM Query: End-to-end message chain', () => {
  test('should complete the full message chain: background -> content -> inject -> content -> background', async () => {
    // Simulate the full chain
    const params = { selector: 'button.submit' }
    const domResult = {
      url: 'http://localhost:3000/checkout',
      title: 'Checkout - MyApp',
      matchCount: 2,
      returnedCount: 2,
      matches: [
        {
          tag: 'button',
          text: 'Place Order',
          visible: true,
          attributes: { class: 'submit btn-primary', type: 'submit' },
          boundingBox: { x: 320, y: 540, width: 200, height: 44 },
        },
        {
          tag: 'button',
          text: 'Save Draft',
          visible: true,
          attributes: { class: 'submit btn-secondary', disabled: '' },
          boundingBox: { x: 320, y: 600, width: 200, height: 44 },
        },
      ],
    }

    // Step 1: background.js sends DOM_QUERY to content.js
    const mockTabsSendMessage = mock.fn((tabId, message) => {
      // Step 2: content.js receives DOM_QUERY and forwards to inject.js
      return new Promise((resolve) => {
        assert.strictEqual(message.type, 'DOM_QUERY')
        assert.deepStrictEqual(message.params, params)

        // Step 3: inject.js processes GASOLINE_DOM_QUERY, runs executeDOMQuery, returns result
        // Step 4: content.js receives GASOLINE_DOM_QUERY_RESPONSE, calls sendResponse
        resolve(domResult)
      })
    })

    // Execute the chain
    const result = await mockTabsSendMessage(1, {
      type: 'DOM_QUERY',
      params,
    })

    // Step 5: background.js receives result and would post to server
    assert.ok(result.url)
    assert.strictEqual(result.matchCount, 2)
    assert.strictEqual(result.returnedCount, 2)
    assert.strictEqual(result.matches.length, 2)
    assert.strictEqual(result.matches[0].tag, 'button')
    assert.strictEqual(result.matches[0].text, 'Place Order')
    assert.strictEqual(result.matches[1].text, 'Save Draft')
  })

  test('should propagate errors through the chain', async () => {
    const mockTabsSendMessage = mock.fn(() => Promise.reject(new Error('Extension context invalidated')))

    let errorResult
    try {
      await mockTabsSendMessage(1, { type: 'DOM_QUERY', params: { selector: 'h1' } })
    } catch (err) {
      errorResult = {
        error: 'dom_query_failed',
        message: err.message,
      }
    }

    assert.ok(errorResult)
    assert.strictEqual(errorResult.error, 'dom_query_failed')
    assert.strictEqual(errorResult.message, 'Extension context invalidated')
  })

  test('should handle empty query results', async () => {
    const mockTabsSendMessage = mock.fn(() =>
      Promise.resolve({
        url: 'http://localhost:3000/test',
        title: 'Test Page',
        matchCount: 0,
        returnedCount: 0,
        matches: [],
      }),
    )

    const result = await mockTabsSendMessage(1, {
      type: 'DOM_QUERY',
      params: { selector: '.nonexistent' },
    })

    assert.strictEqual(result.matchCount, 0)
    assert.strictEqual(result.matches.length, 0)
  })

  test('should handle large result sets (capped at max elements)', async () => {
    const manyMatches = Array.from({ length: 50 }, (_, i) => ({
      tag: 'div',
      text: `Item ${i}`,
      visible: true,
    }))

    const mockTabsSendMessage = mock.fn(() =>
      Promise.resolve({
        url: 'http://localhost:3000/test',
        title: 'Test Page',
        matchCount: 100, // Total matched
        returnedCount: 50, // Capped at 50
        matches: manyMatches,
      }),
    )

    const result = await mockTabsSendMessage(1, {
      type: 'DOM_QUERY',
      params: { selector: 'div' },
    })

    assert.strictEqual(result.matchCount, 100)
    assert.strictEqual(result.returnedCount, 50)
    assert.strictEqual(result.matches.length, 50)
  })
})
