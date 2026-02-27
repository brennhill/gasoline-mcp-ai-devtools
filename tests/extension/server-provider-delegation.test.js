// @ts-nocheck
/**
 * @fileoverview server-provider-delegation.test.js — Ensures legacy server helper
 * functions delegate to ExtensionTransportProvider instead of using fetch directly.
 */

import { test, describe, mock, beforeEach } from 'node:test'
import assert from 'node:assert'

const provider = {
  postLogs: mock.fn(() => Promise.resolve({ entries: 1 })),
  postWebSocketEvents: mock.fn(() => Promise.resolve()),
  postNetworkBodies: mock.fn(() => Promise.resolve()),
  postEnhancedActions: mock.fn(() => Promise.resolve()),
  postPerformanceSnapshots: mock.fn(() => Promise.resolve()),
  checkHealth: mock.fn(() => Promise.resolve({ connected: true })),
  postQueryResult: mock.fn(() => Promise.resolve()),
  postAsyncCommandResult: mock.fn(() => Promise.resolve()),
  postExtensionLogs: mock.fn(() => Promise.resolve()),
  postStatusPing: mock.fn(() => Promise.resolve()),
  pollPendingQueries: mock.fn(() => Promise.resolve([]))
}

const mockGetOrCreateTransportProvider = mock.fn(() => provider)

mock.module('../../extension/background/transport-provider.js', {
  namedExports: {
    getOrCreateTransportProvider: mockGetOrCreateTransportProvider,
    getRequestHeaders: () => ({ 'Content-Type': 'application/json' })
  }
})

const server = await import('../../extension/background/server.js')

describe('server.js provider delegation', () => {
  beforeEach(() => {
    mock.restoreAll()
    globalThis.fetch = mock.fn((url) => {
      if (typeof url === 'string' && url.endsWith('/health')) {
        return Promise.resolve({ ok: true, json: () => Promise.resolve({ connected: true }) })
      }
      return Promise.resolve({ ok: true, json: () => Promise.resolve({ entries: 1 }) })
    })
    mockGetOrCreateTransportProvider.mock.resetCalls()
    for (const fn of Object.values(provider)) {
      fn.mock.resetCalls()
    }
  })

  test('sendLogsToServer delegates to provider.postLogs', async () => {
    const result = await server.sendLogsToServer('http://localhost:7777', [{ level: 'error' }])
    assert.strictEqual(result.entries, 1)
    assert.strictEqual(mockGetOrCreateTransportProvider.mock.calls.length, 1)
    assert.strictEqual(provider.postLogs.mock.calls.length, 1)
  })

  test('sendWSEventsToServer delegates to provider.postWebSocketEvents', async () => {
    await server.sendWSEventsToServer('http://localhost:7777', [{ event: 'open' }])
    assert.strictEqual(provider.postWebSocketEvents.mock.calls.length, 1)
  })

  test('sendNetworkBodiesToServer delegates to provider.postNetworkBodies', async () => {
    await server.sendNetworkBodiesToServer('http://localhost:7777', [{ url: 'https://example.com' }])
    assert.strictEqual(provider.postNetworkBodies.mock.calls.length, 1)
  })

  test('checkServerHealth delegates to provider.checkHealth', async () => {
    const health = await server.checkServerHealth('http://localhost:7777')
    assert.strictEqual(health.connected, true)
    assert.strictEqual(provider.checkHealth.mock.calls.length, 1)
  })

  test('postQueryResult delegates to provider.postQueryResult', async () => {
    await server.postQueryResult('http://localhost:7777', 'q1', 'dom', { ok: true })
    assert.strictEqual(provider.postQueryResult.mock.calls.length, 1)
  })
})
