// @ts-nocheck
/**
 * @fileoverview sync-manager.test.js — Tests for sync client lifecycle management.
 * Covers startSyncClient, stopSyncClient, resetSyncClientConnection, and
 * idempotent start behavior.
 *
 * Run: node --experimental-test-module-mocks --test tests/extension/sync-manager.test.js
 */

import { test, describe, mock, beforeEach } from 'node:test'
import assert from 'node:assert'

// ---------------------------------------------------------------------------
// Mock sibling modules before importing the unit under test
// ---------------------------------------------------------------------------

const mockSyncClientInstance = {
  start: mock.fn(),
  stop: mock.fn(),
  resetConnection: mock.fn()
}

const mockCreateSyncClient = mock.fn(() => mockSyncClientInstance)

mock.module('../../extension/background/sync-client.js', {
  namedExports: {
    createSyncClient: mockCreateSyncClient,
    SyncClient: class {}
  }
})

mock.module('../../extension/background/debug.js', {
  namedExports: {
    DebugCategory: {
      CONNECTION: 'connection', CAPTURE: 'capture', ERROR: 'error',
      LIFECYCLE: 'lifecycle', SETTINGS: 'settings', SOURCEMAP: 'sourcemap', QUERY: 'query'
    }
  }
})

mock.module('../../extension/background/communication.js', {
  namedExports: { updateBadge: mock.fn() }
})

mock.module('../../extension/background/state-manager.js', {
  namedExports: {
    isQueryProcessing: mock.fn(() => false),
    addProcessingQuery: mock.fn(),
    removeProcessingQuery: mock.fn()
  }
})

mock.module('../../extension/background/event-listeners.js', {
  namedExports: {
    getTrackedTabInfo: mock.fn(() => Promise.resolve({
      trackedTabId: 0, trackedTabUrl: '', trackedTabTitle: ''
    }))
  }
})

mock.module('../../extension/background/pending-queries.js', {
  namedExports: { handlePendingQuery: mock.fn(() => Promise.resolve()) }
})

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function createMockDeps(overrides = {}) {
  return {
    getServerUrl: mock.fn(() => 'http://localhost:7777'),
    getExtSessionId: mock.fn(() => 'test-session-1'),
    getConnectionStatus: mock.fn(() => ({
      connected: false, entries: 0, maxEntries: 1000, errorCount: 0, logFile: ''
    })),
    setConnectionStatus: mock.fn(),
    getAiControlled: mock.fn(() => false),
    getAiWebPilotEnabledCache: mock.fn(() => false),
    getExtensionLogQueue: mock.fn(() => []),
    clearExtensionLogQueue: mock.fn(),
    applyCaptureOverrides: mock.fn(),
    debugLog: mock.fn(),
    ...overrides
  }
}

// ---------------------------------------------------------------------------
// Tests — ordered to account for module-level syncClient state.
// The module holds a single syncClient variable. Once created, stopSyncClient
// does NOT null it out, so startSyncClient remains idempotent.
// We import a fresh module per describe block using dynamic import + unique
// cache-busting query strings.
// ---------------------------------------------------------------------------

// Fresh import helper: each call gets a fresh module with its own syncClient state
let importCounter = 0
async function freshImport() {
  importCounter++
  // mock.module applies to all imports of the same specifier, so
  // we use the same path — but we need a fresh module instance.
  // Node caches modules by URL. Query params bust the cache.
  const mod = await import(
    `../../extension/background/sync-manager.js?v=${importCounter}`
  )
  return mod
}

describe('startSyncClient', () => {
  beforeEach(() => {
    mockCreateSyncClient.mock.resetCalls()
    mockSyncClientInstance.start.mock.resetCalls()
    mockSyncClientInstance.stop.mock.resetCalls()
    mockSyncClientInstance.resetConnection.mock.resetCalls()
  })

  test('creates and starts a sync client', async () => {
    const { startSyncClient } = await freshImport()
    const deps = createMockDeps()
    startSyncClient(deps)

    assert.strictEqual(mockCreateSyncClient.mock.calls.length, 1, 'Should call createSyncClient once')
    assert.strictEqual(mockSyncClientInstance.start.mock.calls.length, 1, 'Should call start()')

    const startLog = deps.debugLog.mock.calls.find(c =>
      typeof c.arguments[1] === 'string' && c.arguments[1].includes('Sync client started')
    )
    assert.ok(startLog, 'Should log sync client started')
  })

  test('passes server URL and session ID to createSyncClient', async () => {
    const { startSyncClient } = await freshImport()
    const deps = createMockDeps()
    startSyncClient(deps)

    const [url, sessionId] = mockCreateSyncClient.mock.calls[0].arguments
    assert.strictEqual(url, 'http://localhost:7777')
    assert.strictEqual(sessionId, 'test-session-1')
  })

  test('is idempotent — second call is a no-op', async () => {
    const { startSyncClient } = await freshImport()
    const deps = createMockDeps()
    startSyncClient(deps)
    startSyncClient(deps)

    assert.strictEqual(mockCreateSyncClient.mock.calls.length, 1, 'Should only create once')
    assert.strictEqual(mockSyncClientInstance.start.mock.calls.length, 1, 'Should only start once')
  })
})

describe('stopSyncClient', () => {
  beforeEach(() => {
    mockCreateSyncClient.mock.resetCalls()
    mockSyncClientInstance.start.mock.resetCalls()
    mockSyncClientInstance.stop.mock.resetCalls()
  })

  test('stops the running sync client and logs', async () => {
    const { startSyncClient, stopSyncClient } = await freshImport()
    const deps = createMockDeps()
    startSyncClient(deps)

    mockSyncClientInstance.stop.mock.resetCalls()

    const debugLog = mock.fn()
    stopSyncClient(debugLog)

    assert.strictEqual(mockSyncClientInstance.stop.mock.calls.length, 1, 'Should call stop()')
    const stopLog = debugLog.mock.calls.find(c =>
      typeof c.arguments[1] === 'string' && c.arguments[1].includes('Sync client stopped')
    )
    assert.ok(stopLog, 'Should log sync client stopped')
  })

  test('is a no-op when no client has been created', async () => {
    const { stopSyncClient } = await freshImport()
    const debugLog = mock.fn()
    stopSyncClient(debugLog)

    assert.strictEqual(debugLog.mock.calls.length, 0, 'Should not log when no client exists')
  })
})

describe('resetSyncClientConnection', () => {
  beforeEach(() => {
    mockCreateSyncClient.mock.resetCalls()
    mockSyncClientInstance.start.mock.resetCalls()
    mockSyncClientInstance.stop.mock.resetCalls()
    mockSyncClientInstance.resetConnection.mock.resetCalls()
  })

  test('resets connection on running client', async () => {
    const { startSyncClient, resetSyncClientConnection } = await freshImport()
    const deps = createMockDeps()
    startSyncClient(deps)

    const debugLog = mock.fn()
    resetSyncClientConnection(debugLog)

    assert.strictEqual(mockSyncClientInstance.resetConnection.mock.calls.length, 1)
    const resetLog = debugLog.mock.calls.find(c =>
      typeof c.arguments[1] === 'string' && c.arguments[1].includes('Sync client connection reset')
    )
    assert.ok(resetLog, 'Should log connection reset')
  })

  test('is a no-op when no client exists', async () => {
    const { resetSyncClientConnection } = await freshImport()
    const debugLog = mock.fn()
    resetSyncClientConnection(debugLog)

    assert.strictEqual(mockSyncClientInstance.resetConnection.mock.calls.length, 0)
    assert.strictEqual(debugLog.mock.calls.length, 0)
  })
})
