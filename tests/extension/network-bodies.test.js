// @ts-nocheck
/**
 * @fileoverview network-bodies.test.js — Tests for network response body capture.
 * Covers fetch response cloning, body size truncation, header sanitization
 * (stripping auth/cookie headers), content-type detection, and the
 * GASOLINE_NETWORK_BODY message format posted to the content script.
 */

import { test, describe, mock, beforeEach, afterEach } from 'node:test'
import assert from 'node:assert'
import { createMockWindow } from './helpers.js'

const createMockResponse = (options = {}) => ({
  ok: options.ok !== undefined ? options.ok : true,
  status: options.status || 200,
  statusText: options.statusText || 'OK',
  headers: new Map([['content-type', options.contentType || 'application/json'], ...(options.headers || [])]),
  clone: function () {
    return {
      ...this,
      text: () => Promise.resolve(options.body || '{}'),
      blob: () =>
        Promise.resolve({ size: (options.body || '{}').length, type: options.contentType || 'application/json' })
    }
  }
})

// Minimal Headers polyfill for testing
class MockHeaders {
  constructor(init) {
    this._map = new Map(Object.entries(init || {}))
  }
  get(name) {
    return this._map.get(name.toLowerCase()) || null
  }
  entries() {
    return this._map.entries()
  }
  forEach(fn) {
    this._map.forEach((v, k) => fn(v, k))
  }
}

let originalWindow

describe('Network Body Capture - Fetch Wrapper', () => {
  beforeEach(() => {
    originalWindow = globalThis.window
    globalThis.window = createMockWindow({ withFetch: true })
    globalThis.Headers = MockHeaders
  })

  afterEach(() => {
    globalThis.window = originalWindow
  })

  test('network body event has spec-compliant shape', async () => {
    const { wrapFetchWithBodies } = await import('../../extension/inject.js')

    const mockResponse = createMockResponse({ body: '{"id":1}', contentType: 'application/json', status: 201 })
    const originalFetch = mock.fn(() => Promise.resolve(mockResponse))

    const wrappedFetch = wrapFetchWithBodies(originalFetch)
    await wrappedFetch('/api/users', { method: 'POST', body: '{"name":"Alice"}' })
    await new Promise((r) => setTimeout(r, 10))

    const calls = globalThis.window.postMessage.mock.calls
    const bodyEvent = calls.find((c) => c.arguments[0].type === 'GASOLINE_NETWORK_BODY')
    assert.ok(bodyEvent, 'Expected network body event')
    const payload = bodyEvent.arguments[0].payload

    // Shape from spec: method, url, status, requestBody, responseBody, contentType, duration
    assert.ok('method' in payload, 'missing: method')
    assert.ok('url' in payload, 'missing: url')
    assert.ok('status' in payload, 'missing: status')
    assert.ok('requestBody' in payload, 'missing: requestBody')
    assert.ok('responseBody' in payload, 'missing: responseBody')
    assert.ok('contentType' in payload, 'missing: contentType')
    assert.ok('duration' in payload, 'missing: duration')
  })

  test('should capture response body for JSON responses', async () => {
    const { wrapFetchWithBodies } = await import('../../extension/inject.js')

    const responseBody = '{"id":1,"name":"Alice"}'
    const mockResponse = createMockResponse({ body: responseBody, contentType: 'application/json' })
    const originalFetch = mock.fn(() => Promise.resolve(mockResponse))

    const wrappedFetch = wrapFetchWithBodies(originalFetch)
    await wrappedFetch('/api/users/1')

    // Wait for async body capture
    await new Promise((r) => setTimeout(r, 10))

    const calls = globalThis.window.postMessage.mock.calls
    const bodyEvent = calls.find((c) => {
      const msg = c.arguments[0]
      return msg.type === 'GASOLINE_NETWORK_BODY'
    })

    assert.ok(bodyEvent, 'Expected network body event')
    assert.strictEqual(bodyEvent.arguments[0].payload.responseBody, responseBody)
    assert.strictEqual(bodyEvent.arguments[0].payload.status, 200)
  })

  test('should capture request body for POST requests', async () => {
    const { wrapFetchWithBodies } = await import('../../extension/inject.js')

    const requestBody = '{"name":"Alice","email":"alice@example.com"}'
    const mockResponse = createMockResponse({ status: 201, body: '{"id":1}' })
    const originalFetch = mock.fn(() => Promise.resolve(mockResponse))

    const wrappedFetch = wrapFetchWithBodies(originalFetch)
    await wrappedFetch('/api/users', {
      method: 'POST',
      body: requestBody,
      headers: { 'Content-Type': 'application/json' }
    })

    await new Promise((r) => setTimeout(r, 10))

    const calls = globalThis.window.postMessage.mock.calls
    const bodyEvent = calls.find((c) => c.arguments[0].type === 'GASOLINE_NETWORK_BODY')

    assert.ok(bodyEvent, 'Expected network body event')
    assert.strictEqual(bodyEvent.arguments[0].payload.requestBody, requestBody)
    assert.strictEqual(bodyEvent.arguments[0].payload.method, 'POST')
  })

  test('should return response immediately without blocking', async () => {
    const { wrapFetchWithBodies } = await import('../../extension/inject.js')

    const mockResponse = createMockResponse({ body: '{"data":"test"}' })
    const originalFetch = mock.fn(() => Promise.resolve(mockResponse))

    const wrappedFetch = wrapFetchWithBodies(originalFetch)
    const response = await wrappedFetch('/api/test')

    // Response should be returned immediately
    assert.strictEqual(response.status, 200)
    assert.strictEqual(response.ok, true)
  })

  test('should not block on slow body reads', async () => {
    const { wrapFetchWithBodies } = await import('../../extension/inject.js')

    const slowClone = {
      ok: true,
      status: 200,
      headers: new Map([['content-type', 'application/json']]),
      clone: () => ({
        text: () => new Promise((resolve) => setTimeout(() => resolve('slow'), 100))
      })
    }

    const originalFetch = mock.fn(() => Promise.resolve(slowClone))
    const wrappedFetch = wrapFetchWithBodies(originalFetch)

    const start = Date.now()
    await wrappedFetch('/api/slow')
    const elapsed = Date.now() - start

    // Should return almost immediately (not wait for body read)
    assert.ok(elapsed < 20, `Expected fast return, took ${elapsed}ms`)
  })

  test('should capture content-type', async () => {
    const { wrapFetchWithBodies } = await import('../../extension/inject.js')

    const mockResponse = createMockResponse({ contentType: 'text/html' })
    const originalFetch = mock.fn(() => Promise.resolve(mockResponse))

    const wrappedFetch = wrapFetchWithBodies(originalFetch)
    await wrappedFetch('/page')

    await new Promise((r) => setTimeout(r, 10))

    const calls = globalThis.window.postMessage.mock.calls
    const bodyEvent = calls.find((c) => c.arguments[0].type === 'GASOLINE_NETWORK_BODY')

    assert.strictEqual(bodyEvent.arguments[0].payload.contentType, 'text/html')
  })

  test('should capture request method', async () => {
    const { wrapFetchWithBodies } = await import('../../extension/inject.js')

    const mockResponse = createMockResponse({})
    const originalFetch = mock.fn(() => Promise.resolve(mockResponse))

    const wrappedFetch = wrapFetchWithBodies(originalFetch)
    await wrappedFetch('/api/test', { method: 'PUT', body: '{}' })

    await new Promise((r) => setTimeout(r, 10))

    const calls = globalThis.window.postMessage.mock.calls
    const bodyEvent = calls.find((c) => c.arguments[0].type === 'GASOLINE_NETWORK_BODY')

    assert.strictEqual(bodyEvent.arguments[0].payload.method, 'PUT')
  })

  test('should default method to GET', async () => {
    const { wrapFetchWithBodies } = await import('../../extension/inject.js')

    const mockResponse = createMockResponse({})
    const originalFetch = mock.fn(() => Promise.resolve(mockResponse))

    const wrappedFetch = wrapFetchWithBodies(originalFetch)
    await wrappedFetch('/api/test')

    await new Promise((r) => setTimeout(r, 10))

    const calls = globalThis.window.postMessage.mock.calls
    const bodyEvent = calls.find((c) => c.arguments[0].type === 'GASOLINE_NETWORK_BODY')

    assert.strictEqual(bodyEvent.arguments[0].payload.method, 'GET')
  })

  test('should handle Request object as input', async () => {
    const { wrapFetchWithBodies } = await import('../../extension/inject.js')

    const mockResponse = createMockResponse({})
    const originalFetch = mock.fn(() => Promise.resolve(mockResponse))

    const wrappedFetch = wrapFetchWithBodies(originalFetch)
    const request = { url: '/api/data', method: 'PATCH' }
    await wrappedFetch(request)

    await new Promise((r) => setTimeout(r, 10))

    const calls = globalThis.window.postMessage.mock.calls
    const bodyEvent = calls.find((c) => c.arguments[0].type === 'GASOLINE_NETWORK_BODY')

    assert.strictEqual(bodyEvent.arguments[0].payload.url, '/api/data')
  })

  test('should not capture binary content types', async () => {
    const { wrapFetchWithBodies } = await import('../../extension/inject.js')

    const binaryTypes = ['image/png', 'video/mp4', 'audio/mpeg', 'font/woff2', 'application/wasm']

    for (const type of binaryTypes) {
      globalThis.window.postMessage.mock.resetCalls()

      const mockResponse = createMockResponse({ contentType: type, body: 'binary data' })
      const originalFetch = mock.fn(() => Promise.resolve(mockResponse))

      const wrappedFetch = wrapFetchWithBodies(originalFetch)
      await wrappedFetch('/asset')

      await new Promise((r) => setTimeout(r, 10))

      const calls = globalThis.window.postMessage.mock.calls
      const bodyEvent = calls.find((c) => c.arguments[0].type === 'GASOLINE_NETWORK_BODY')

      if (bodyEvent) {
        assert.ok(
          bodyEvent.arguments[0].payload.responseBody.includes('[Binary:'),
          `Expected binary placeholder for ${type}, got: ${bodyEvent.arguments[0].payload.responseBody}`
        )
      }
    }
  })

  test('should not capture requests to gasoline server', async () => {
    const { wrapFetchWithBodies } = await import('../../extension/inject.js')

    const mockResponse = createMockResponse({})
    const originalFetch = mock.fn(() => Promise.resolve(mockResponse))

    const wrappedFetch = wrapFetchWithBodies(originalFetch)
    await wrappedFetch('http://localhost:7890/logs')

    await new Promise((r) => setTimeout(r, 10))

    const calls = globalThis.window.postMessage.mock.calls
    const bodyEvent = calls.find((c) => c.arguments[0].type === 'GASOLINE_NETWORK_BODY')

    assert.ok(!bodyEvent, 'Should not capture requests to gasoline server')
  })

  test('should capture duration', async () => {
    const { wrapFetchWithBodies } = await import('../../extension/inject.js')

    const mockResponse = createMockResponse({ body: '{}' })
    const originalFetch = mock.fn(
      () =>
        new Promise((resolve) => {
          setTimeout(() => resolve(mockResponse), 20)
        })
    )

    const wrappedFetch = wrapFetchWithBodies(originalFetch)
    await wrappedFetch('/api/slow')

    await new Promise((r) => setTimeout(r, 30))

    const calls = globalThis.window.postMessage.mock.calls
    const bodyEvent = calls.find((c) => c.arguments[0].type === 'GASOLINE_NETWORK_BODY')

    assert.ok(bodyEvent.arguments[0].payload.duration >= 15, 'Expected duration >= 15ms')
  })
})

describe('Header Sanitization', () => {
  test('should strip Authorization header', async () => {
    const { sanitizeHeaders } = await import('../../extension/inject.js')

    const headers = {
      Authorization: 'Bearer secret-token-123',
      'Content-Type': 'application/json',
      Accept: 'application/json'
    }

    const sanitized = sanitizeHeaders(headers)

    assert.strictEqual(sanitized['Authorization'], undefined)
    assert.strictEqual(sanitized['Content-Type'], 'application/json')
  })

  test('should strip Cookie header', async () => {
    const { sanitizeHeaders } = await import('../../extension/inject.js')

    const headers = {
      Cookie: 'session=abc123; token=xyz',
      'Content-Type': 'text/html'
    }

    const sanitized = sanitizeHeaders(headers)
    assert.strictEqual(sanitized['Cookie'], undefined)
  })

  test('should strip Set-Cookie header', async () => {
    const { sanitizeHeaders } = await import('../../extension/inject.js')

    const headers = {
      'Set-Cookie': 'session=abc123; Path=/',
      'Content-Type': 'text/html'
    }

    const sanitized = sanitizeHeaders(headers)
    assert.strictEqual(sanitized['Set-Cookie'], undefined)
  })

  test('should strip X-API-Key header', async () => {
    const { sanitizeHeaders } = await import('../../extension/inject.js')

    const headers = {
      'X-API-Key': 'sk_live_abc123',
      'Content-Type': 'application/json'
    }

    const sanitized = sanitizeHeaders(headers)
    assert.strictEqual(sanitized['X-API-Key'], undefined)
  })

  test('should strip headers matching token/secret/key/password pattern', async () => {
    const { sanitizeHeaders } = await import('../../extension/inject.js')

    const headers = {
      'X-Auth-Token': 'abc123',
      'X-Secret-Key': 'xyz789',
      'X-API-Secret': 'secret-value',
      'X-Password': 'hunter2',
      'X-Custom-Header': 'safe-value'
    }

    const sanitized = sanitizeHeaders(headers)

    assert.strictEqual(sanitized['X-Auth-Token'], undefined)
    assert.strictEqual(sanitized['X-Secret-Key'], undefined)
    assert.strictEqual(sanitized['X-API-Secret'], undefined)
    assert.strictEqual(sanitized['X-Password'], undefined)
    assert.strictEqual(sanitized['X-Custom-Header'], 'safe-value')
  })

  test('should be case-insensitive for sensitive header patterns', async () => {
    const { sanitizeHeaders } = await import('../../extension/inject.js')

    const headers = {
      'x-auth-TOKEN': 'value',
      'X-SECRET-key': 'value',
      authorization: 'Bearer xyz'
    }

    const sanitized = sanitizeHeaders(headers)

    assert.strictEqual(sanitized['x-auth-TOKEN'], undefined)
    assert.strictEqual(sanitized['X-SECRET-key'], undefined)
    assert.strictEqual(sanitized['authorization'], undefined)
  })

  test('should handle null/undefined headers', async () => {
    const { sanitizeHeaders } = await import('../../extension/inject.js')

    assert.deepStrictEqual(sanitizeHeaders(null), {})
    assert.deepStrictEqual(sanitizeHeaders(undefined), {})
  })

  test('should handle Headers object', async () => {
    const { sanitizeHeaders } = await import('../../extension/inject.js')

    const headers = new MockHeaders({
      authorization: 'Bearer token',
      'content-type': 'application/json'
    })

    const sanitized = sanitizeHeaders(headers)
    assert.strictEqual(sanitized['authorization'], undefined)
    assert.strictEqual(sanitized['content-type'], 'application/json')
  })
})

describe('Body Truncation', () => {
  test('should truncate request body at 8KB', async () => {
    const { truncateRequestBody } = await import('../../extension/inject.js')

    const largeBody = 'x'.repeat(10000)
    const result = truncateRequestBody(largeBody)

    assert.ok(result.body.length <= 8192)
    assert.strictEqual(result.truncated, true)
  })

  test('should not truncate request body under 8KB', async () => {
    const { truncateRequestBody } = await import('../../extension/inject.js')

    const smallBody = '{"name":"Alice"}'
    const result = truncateRequestBody(smallBody)

    assert.strictEqual(result.body, smallBody)
    assert.strictEqual(result.truncated, false)
  })

  test('should truncate response body at 16KB', async () => {
    const { truncateResponseBody } = await import('../../extension/inject.js')

    const largeBody = 'y'.repeat(20000)
    const result = truncateResponseBody(largeBody)

    assert.ok(result.body.length <= 16384)
    assert.strictEqual(result.truncated, true)
  })

  test('should not truncate response body under 16KB', async () => {
    const { truncateResponseBody } = await import('../../extension/inject.js')

    const smallBody = '{"items":[1,2,3]}'
    const result = truncateResponseBody(smallBody)

    assert.strictEqual(result.body, smallBody)
    assert.strictEqual(result.truncated, false)
  })

  test('should handle null body', async () => {
    const { truncateRequestBody, truncateResponseBody } = await import('../../extension/inject.js')

    assert.deepStrictEqual(truncateRequestBody(null), { body: null, truncated: false })
    assert.deepStrictEqual(truncateResponseBody(null), { body: null, truncated: false })
  })
})

describe('Body Reading', () => {
  test('should read JSON response as text', async () => {
    const { readResponseBody } = await import('../../extension/inject.js')

    const response = {
      headers: new Map([['content-type', 'application/json']]),
      text: () => Promise.resolve('{"id":1}')
    }

    const body = await readResponseBody(response)
    assert.strictEqual(body, '{"id":1}')
  })

  test('should read text response as text', async () => {
    const { readResponseBody } = await import('../../extension/inject.js')

    const response = {
      headers: new Map([['content-type', 'text/html']]),
      text: () => Promise.resolve('<html></html>')
    }

    const body = await readResponseBody(response)
    assert.strictEqual(body, '<html></html>')
  })

  test('should report binary size for non-text content', async () => {
    const { readResponseBody } = await import('../../extension/inject.js')

    const response = {
      headers: new Map([['content-type', 'image/png']]),
      blob: () => Promise.resolve({ size: 4096 })
    }

    const body = await readResponseBody(response)
    assert.ok(body.includes('[Binary:'))
    assert.ok(body.includes('4096'))
    assert.ok(body.includes('image/png'))
  })

  test('should handle missing content-type', async () => {
    const { readResponseBody } = await import('../../extension/inject.js')

    const response = {
      headers: new Map(),
      text: () => Promise.resolve('raw data')
    }

    const body = await readResponseBody(response)
    // Should try to read as text when content-type is missing
    assert.strictEqual(body, 'raw data')
  })

  test('should timeout body read after 5ms', async () => {
    const { readResponseBodyWithTimeout } = await import('../../extension/inject.js')

    const response = {
      headers: new Map([['content-type', 'application/json']]),
      text: () => new Promise((resolve) => setTimeout(() => resolve('{}'), 50))
    }

    const body = await readResponseBodyWithTimeout(response, 5)
    assert.ok(body.includes('[Skipped:'), `Expected timeout message, got: ${body}`)
  })
})

describe('URL Filtering', () => {
  test('should exclude gasoline server URLs', async () => {
    const { shouldCaptureUrl } = await import('../../extension/inject.js')

    assert.strictEqual(shouldCaptureUrl('http://localhost:7890/logs'), false)
    assert.strictEqual(shouldCaptureUrl('http://127.0.0.1:7890/health'), false)
  })

  test('should include normal URLs', async () => {
    const { shouldCaptureUrl } = await import('../../extension/inject.js')

    assert.strictEqual(shouldCaptureUrl('/api/users'), true)
    assert.strictEqual(shouldCaptureUrl('https://api.example.com/data'), true)
    assert.strictEqual(shouldCaptureUrl('http://localhost:3000/api/test'), true)
  })

  test('should exclude extension resource URLs', async () => {
    const { shouldCaptureUrl } = await import('../../extension/inject.js')

    assert.strictEqual(shouldCaptureUrl('chrome-extension://abc123/lib/axe.min.js'), false)
  })
})
