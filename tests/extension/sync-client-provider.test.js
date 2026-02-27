// @ts-nocheck
/**
 * @fileoverview sync-client-provider.test.js — Contract tests for provider-driven
 * sync-client behavior.
 */

import { test, describe, mock, beforeEach, afterEach } from 'node:test'
import assert from 'node:assert'

import { createSyncClientWithProvider } from '../../extension/background/sync-client.js'

function createMockCallbacks(overrides = {}) {
  return {
    onCommand: mock.fn(() => Promise.resolve()),
    onConnectionChange: mock.fn(),
    onCaptureOverrides: mock.fn(),
    onVersionMismatch: mock.fn(),
    getSettings: mock.fn(() =>
      Promise.resolve({
        pilot_enabled: false,
        tracking_enabled: false,
        tracked_tab_id: 0,
        tracked_tab_url: '',
        tracked_tab_title: '',
        capture_logs: true,
        capture_network: true,
        capture_websocket: false,
        capture_actions: true
      })
    ),
    getExtensionLogs: mock.fn(() => []),
    clearExtensionLogs: mock.fn(),
    debugLog: mock.fn(),
    ...overrides
  }
}

function createMockProvider(overrides = {}) {
  return {
    id: mock.fn(() => 'http'),
    setEndpoint: mock.fn(),
    sendSync: mock.fn(() =>
      Promise.resolve({
        ack: true,
        commands: [],
        next_poll_ms: 1000,
        server_time: new Date().toISOString()
      })
    ),
    ...overrides
  }
}

describe('SyncClient provider integration', () => {
  let client

  afterEach(() => {
    if (client) {
      client.stop()
    }
  })

  beforeEach(() => {
    mock.restoreAll()
  })

  test('routes heartbeat sync via provider.sendSync (no direct fetch dependency)', async () => {
    const provider = createMockProvider()
    const callbacks = createMockCallbacks()
    client = createSyncClientWithProvider(provider, 'ext-1', callbacks, '0.7.9')
    client.start()

    await new Promise((r) => setTimeout(r, 30))

    assert.ok(provider.sendSync.mock.calls.length >= 1, 'expected provider.sendSync to be called')
  })

  test('setServerUrl delegates endpoint updates to provider', () => {
    const provider = createMockProvider()
    const callbacks = createMockCallbacks()
    client = createSyncClientWithProvider(provider, 'ext-1', callbacks, '0.7.9')

    client.setServerUrl('http://localhost:9999')
    assert.strictEqual(provider.setEndpoint.mock.calls.length, 1)
    assert.strictEqual(provider.setEndpoint.mock.calls[0].arguments[0], 'http://localhost:9999')
  })
})

