// @ts-nocheck
/**
 * @fileoverview server-edge-cases.test.js — Edge case and negative path tests
 * for extension/background/server.js functions.
 */

import { test, describe, mock, beforeEach } from 'node:test'
import assert from 'node:assert'
import { MANIFEST_VERSION as _MANIFEST_VERSION } from './helpers.js'

let mockFetch

beforeEach(() => {
  mock.restoreAll()
  mockFetch = mock.fn()
  globalThis.fetch = mockFetch
})

const {
  sendLogsToServer,
  sendNetworkBodiesToServer,
  sendWSEventsToServer: _sendWSEventsToServer,
  sendEnhancedActionsToServer: _sendEnhancedActionsToServer,
  sendPerformanceSnapshotsToServer,
  postQueryResult,
  postAsyncCommandResult,
  postExtensionLogs,
  sendStatusPing
} = await import('../../extension/background/server.js')

// ============================================
// sendLogsToServer edge cases
// ============================================

describe('sendLogsToServer edge cases', () => {
  test('calls debugLogFn on success', async () => {
    mockFetch.mock.mockImplementation(() =>
      Promise.resolve({ ok: true, json: () => Promise.resolve({ entries: 3 }) })
    )
    const debugLog = mock.fn()
    await sendLogsToServer('http://localhost:9222', [{}, {}, {}], debugLog)
    assert.ok(debugLog.mock.calls.length >= 2, 'debugLog should be called for send + accept')
  })

  test('calls debugLogFn on error', async () => {
    mockFetch.mock.mockImplementation(() =>
      Promise.resolve({ ok: false, status: 502, statusText: 'Bad Gateway' })
    )
    const debugLog = mock.fn()
    await assert.rejects(
      () => sendLogsToServer('http://localhost:9222', [{}], debugLog)
    )
    assert.ok(debugLog.mock.calls.some(c => c.arguments[0] === 'error'))
  })

  test('empty entries array succeeds', async () => {
    mockFetch.mock.mockImplementation(() =>
      Promise.resolve({ ok: true, json: () => Promise.resolve({ entries: 0 }) })
    )
    const result = await sendLogsToServer('http://localhost:9222', [])
    assert.strictEqual(result.entries, 0)
  })
})

// ============================================
// sendNetworkBodiesToServer edge cases
// ============================================

describe('sendNetworkBodiesToServer edge cases', () => {
  test('handles body with all optional fields missing', async () => {
    mockFetch.mock.mockImplementation(() => Promise.resolve({ ok: true }))

    await sendNetworkBodiesToServer('http://localhost:9222', [{
      url: 'https://api.example.com',
      method: 'GET',
      status: 200
    }])

    const call = mockFetch.mock.calls[0]
    const body = JSON.parse(call.arguments[1].body)
    const entry = body.bodies[0]
    assert.strictEqual(entry.url, 'https://api.example.com')
    assert.strictEqual(entry.content_type, undefined)
    assert.strictEqual('tab_id' in entry, false)
    assert.strictEqual('response_truncated' in entry, false)
  })

  test('multiple bodies in single batch', async () => {
    mockFetch.mock.mockImplementation(() => Promise.resolve({ ok: true }))

    await sendNetworkBodiesToServer('http://localhost:9222', [
      { url: 'https://a.com', method: 'GET', status: 200, content_type: 'text/html' },
      { url: 'https://b.com', method: 'POST', status: 201, content_type: 'application/json' },
      { url: 'https://c.com', method: 'DELETE', status: 204 }
    ])

    const body = JSON.parse(mockFetch.mock.calls[0].arguments[1].body)
    assert.strictEqual(body.bodies.length, 3)
  })

  test('throws on server error', async () => {
    mockFetch.mock.mockImplementation(() =>
      Promise.resolve({ ok: false, status: 500, statusText: 'Internal Server Error' })
    )
    await assert.rejects(
      () => sendNetworkBodiesToServer('http://localhost:9222', [{ url: 'x', method: 'GET', status: 200 }])
    )
  })
})

// ============================================
// postQueryResult edge cases
// ============================================

describe('postQueryResult edge cases', () => {
  test('sends query result to correct endpoint', async () => {
    mockFetch.mock.mockImplementation(() => Promise.resolve({ ok: true }))

    await postQueryResult('http://localhost:9222', 'q-1', 'dom', { html: '<div>' })
    const call = mockFetch.mock.calls[0]
    assert.strictEqual(call.arguments[0], 'http://localhost:9222/query-result')
    const body = JSON.parse(call.arguments[1].body)
    assert.strictEqual(body.id, 'q-1')
    assert.deepStrictEqual(body.result, { html: '<div>' })
  })

  test('logs error on non-ok response', async () => {
    mockFetch.mock.mockImplementation(() =>
      Promise.resolve({ ok: false, status: 404 })
    )
    const debugLog = mock.fn()
    // Should not throw — postQueryResult catches errors
    await postQueryResult('http://localhost:9222', 'q-1', 'dom', {}, debugLog)
    assert.ok(debugLog.mock.calls.some(c => c.arguments[1].includes('Failed')))
  })

  test('handles fetch error gracefully', async () => {
    mockFetch.mock.mockImplementation(() => Promise.reject(new Error('timeout')))
    const debugLog = mock.fn()
    await postQueryResult('http://localhost:9222', 'q-1', 'dom', {}, debugLog)
    assert.ok(debugLog.mock.calls.some(c => c.arguments[1].includes('error')))
  })
})

// ============================================
// postAsyncCommandResult edge cases
// ============================================

describe('postAsyncCommandResult edge cases', () => {
  test('sends correlation_id and status', async () => {
    mockFetch.mock.mockImplementation(() => Promise.resolve({ ok: true }))

    await postAsyncCommandResult('http://localhost:9222', 'corr-123', 'completed', { data: 'ok' })
    const body = JSON.parse(mockFetch.mock.calls[0].arguments[1].body)
    assert.strictEqual(body.correlation_id, 'corr-123')
    assert.strictEqual(body.status, 'completed')
    assert.deepStrictEqual(body.result, { data: 'ok' })
  })

  test('sends error field when provided', async () => {
    mockFetch.mock.mockImplementation(() => Promise.resolve({ ok: true }))

    await postAsyncCommandResult('http://localhost:9222', 'corr-456', 'failed', null, 'something broke')
    const body = JSON.parse(mockFetch.mock.calls[0].arguments[1].body)
    assert.strictEqual(body.error, 'something broke')
    assert.strictEqual(body.result, undefined)
  })

  test('omits result and error when null', async () => {
    mockFetch.mock.mockImplementation(() => Promise.resolve({ ok: true }))

    await postAsyncCommandResult('http://localhost:9222', 'corr-789', 'completed')
    const body = JSON.parse(mockFetch.mock.calls[0].arguments[1].body)
    assert.strictEqual('result' in body, false)
    assert.strictEqual('error' in body, false)
  })

  test('handles fetch error gracefully', async () => {
    mockFetch.mock.mockImplementation(() => Promise.reject(new Error('network')))
    // Should not throw
    await postAsyncCommandResult('http://localhost:9222', 'corr-1', 'failed')
  })
})

// ============================================
// postExtensionLogs edge cases
// ============================================

describe('postExtensionLogs edge cases', () => {
  test('skips fetch for empty logs', async () => {
    await postExtensionLogs('http://localhost:9222', [])
    assert.strictEqual(mockFetch.mock.calls.length, 0)
  })

  test('sends logs to correct endpoint', async () => {
    mockFetch.mock.mockImplementation(() => Promise.resolve({ ok: true }))
    await postExtensionLogs('http://localhost:9222', [{ level: 'info', message: 'test' }])
    assert.strictEqual(mockFetch.mock.calls[0].arguments[0], 'http://localhost:9222/extension-logs')
  })

  test('handles fetch error gracefully', async () => {
    mockFetch.mock.mockImplementation(() => Promise.reject(new Error('down')))
    // Should not throw
    await postExtensionLogs('http://localhost:9222', [{ level: 'error' }])
  })
})

// ============================================
// sendStatusPing edge cases
// ============================================

describe('sendStatusPing edge cases', () => {
  test('sends to correct endpoint', async () => {
    mockFetch.mock.mockImplementation(() => Promise.resolve({ ok: true }))
    await sendStatusPing('http://localhost:9222', { type: 'heartbeat' })
    assert.strictEqual(mockFetch.mock.calls[0].arguments[0], 'http://localhost:9222/api/extension-status')
  })

  test('handles fetch error with diagnosticLogFn', async () => {
    mockFetch.mock.mockImplementation(() => Promise.reject(new Error('refused')))
    const diagLog = mock.fn()
    await sendStatusPing('http://localhost:9222', { type: 'heartbeat' }, diagLog)
    assert.ok(diagLog.mock.calls.length >= 1)
    assert.ok(diagLog.mock.calls[0].arguments[0].includes('refused'))
  })
})

// ============================================
// sendPerformanceSnapshotsToServer
// ============================================

describe('sendPerformanceSnapshotsToServer edge cases', () => {
  test('sends to correct endpoint', async () => {
    mockFetch.mock.mockImplementation(() => Promise.resolve({ ok: true }))
    await sendPerformanceSnapshotsToServer('http://localhost:9222', [{ metrics: {} }])
    assert.strictEqual(mockFetch.mock.calls[0].arguments[0], 'http://localhost:9222/performance-snapshots')
  })

  test('throws on error response', async () => {
    mockFetch.mock.mockImplementation(() =>
      Promise.resolve({ ok: false, status: 503, statusText: 'Unavailable' })
    )
    await assert.rejects(
      () => sendPerformanceSnapshotsToServer('http://localhost:9222', [{}])
    )
  })
})
