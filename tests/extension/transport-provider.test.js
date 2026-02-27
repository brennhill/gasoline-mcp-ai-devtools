// @ts-nocheck
/**
 * @fileoverview transport-provider.test.js — Contract tests for provider-based
 * extension transport.
 *
 * These are intentionally API-contract-focused tests for the new
 * HTTPExtensionTransportProvider abstraction.
 */

import { test, describe, beforeEach, mock } from 'node:test'
import assert from 'node:assert'

let mockFetch

beforeEach(() => {
  mock.restoreAll()
  mockFetch = mock.fn()
})

const {
  createHTTPExtensionTransportProvider
} = await import('../../extension/background/transport-provider.js')

describe('HTTPExtensionTransportProvider — identity and endpoint management', () => {
  test('returns canonical provider id', () => {
    const provider = createHTTPExtensionTransportProvider('http://localhost:7777', mockFetch)
    assert.strictEqual(provider.id(), 'http')
  })

  test('setEndpoint updates target endpoint', async () => {
    mockFetch.mock.mockImplementation(() =>
      Promise.resolve({
        ok: true,
        json: () => Promise.resolve({ connected: true })
      })
    )
    const provider = createHTTPExtensionTransportProvider('http://localhost:7777', mockFetch)
    provider.setEndpoint('http://localhost:9999')
    await provider.checkHealth()

    const firstCall = mockFetch.mock.calls[0]
    assert.strictEqual(firstCall.arguments[0], 'http://localhost:9999/health')
  })
})

describe('HTTPExtensionTransportProvider — sync transport', () => {
  test('sendSync posts to /sync and returns parsed response', async () => {
    const expected = { ack: true, commands: [], next_poll_ms: 1000, server_time: '2026-02-27T00:00:00Z' }
    mockFetch.mock.mockImplementation(() =>
      Promise.resolve({
        ok: true,
        json: () => Promise.resolve(expected)
      })
    )

    const provider = createHTTPExtensionTransportProvider('http://localhost:7777', mockFetch)
    const request = { ext_session_id: 'ext-1', settings: { pilot_enabled: true } }
    const result = await provider.sendSync(request, '0.7.9')

    const call = mockFetch.mock.calls[0]
    assert.strictEqual(call.arguments[0], 'http://localhost:7777/sync')
    assert.strictEqual(call.arguments[1].method, 'POST')
    assert.strictEqual(JSON.parse(call.arguments[1].body).ext_session_id, 'ext-1')
    assert.strictEqual(result.ack, true)
  })
})

describe('HTTPExtensionTransportProvider — telemetry and command endpoints', () => {
  test('posts logs via provider', async () => {
    mockFetch.mock.mockImplementation(() =>
      Promise.resolve({
        ok: true,
        json: () => Promise.resolve({ entries: 1 })
      })
    )
    const provider = createHTTPExtensionTransportProvider('http://localhost:7777', mockFetch)
    const result = await provider.postLogs([{ level: 'error', message: 'boom' }], '0.7.9')

    const call = mockFetch.mock.calls[0]
    assert.strictEqual(call.arguments[0], 'http://localhost:7777/logs')
    assert.strictEqual(result.entries, 1)
  })

  test('posts query result via provider', async () => {
    mockFetch.mock.mockImplementation(() =>
      Promise.resolve({ ok: true })
    )
    const provider = createHTTPExtensionTransportProvider('http://localhost:7777', mockFetch)
    await provider.postQueryResult('q-1', { ok: true }, '0.7.9')

    const call = mockFetch.mock.calls[0]
    assert.strictEqual(call.arguments[0], 'http://localhost:7777/query-result')
    assert.strictEqual(JSON.parse(call.arguments[1].body).id, 'q-1')
  })

  test('uploads screenshot via provider', async () => {
    mockFetch.mock.mockImplementation(() =>
      Promise.resolve({
        ok: true,
        json: () => Promise.resolve({ filename: 'shot-1.jpg' })
      })
    )
    const provider = createHTTPExtensionTransportProvider('http://localhost:7777', mockFetch)
    const result = await provider.postScreenshot({
      dataUrl: 'data:image/jpeg;base64,abc',
      url: 'https://example.com',
      errorId: '',
      errorType: ''
    }, '0.7.9')

    const call = mockFetch.mock.calls[0]
    assert.strictEqual(call.arguments[0], 'http://localhost:7777/screenshots')
    assert.strictEqual(result.filename, 'shot-1.jpg')
  })

  test('posts draw mode completion via provider', async () => {
    mockFetch.mock.mockImplementation(() =>
      Promise.resolve({ ok: true, status: 200, text: () => Promise.resolve('OK') })
    )
    const provider = createHTTPExtensionTransportProvider('http://localhost:7777', mockFetch)
    await provider.postDrawModeCompletion({
      screenshot_data_url: 'data:image/png;base64,abc',
      annotations: [],
      element_details: {},
      page_url: 'https://example.com',
      tab_id: 1
    }, '0.7.9')

    const call = mockFetch.mock.calls[0]
    assert.strictEqual(call.arguments[0], 'http://localhost:7777/draw-mode/complete')
    assert.strictEqual(call.arguments[1].method, 'POST')
  })

  test('reads file bytes via provider', async () => {
    mockFetch.mock.mockImplementation(() =>
      Promise.resolve({
        ok: true,
        status: 200,
        json: () => Promise.resolve({ success: true, data_base64: 'YWJj', file_name: 'a.txt' })
      })
    )
    const provider = createHTTPExtensionTransportProvider('http://localhost:7777', mockFetch)
    const result = await provider.readFile({ file_path: '/tmp/a.txt' }, '0.7.9')

    const call = mockFetch.mock.calls[0]
    assert.strictEqual(call.arguments[0], 'http://localhost:7777/api/file/read')
    assert.strictEqual(result.success, true)
  })

  test('calls OS automation endpoints via provider', async () => {
    mockFetch.mock.mockImplementation(() =>
      Promise.resolve({
        ok: true,
        status: 200,
        json: () => Promise.resolve({ success: true })
      })
    )
    const provider = createHTTPExtensionTransportProvider('http://localhost:7777', mockFetch)
    await provider.osAutomationInject({ file_path: '/tmp/a.txt', browser_pid: 0 }, '0.7.9')
    await provider.osAutomationDismiss('0.7.9')

    assert.strictEqual(mockFetch.mock.calls[0].arguments[0], 'http://localhost:7777/api/os-automation/inject')
    assert.strictEqual(mockFetch.mock.calls[1].arguments[0], 'http://localhost:7777/api/os-automation/dismiss')
  })
})
