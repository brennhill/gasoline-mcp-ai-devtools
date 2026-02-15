// @ts-nocheck
/**
 * @fileoverview message-handlers.test.js — Tests for extension/background/message-handlers.js.
 * Covers sender validation, message routing, state snapshots.
 */

import { test, describe, mock, beforeEach } from 'node:test'
import assert from 'node:assert'
import { MANIFEST_VERSION as _MANIFEST_VERSION } from './helpers.js'

const {
  installMessageListener,
  saveStateSnapshot,
  loadStateSnapshot,
  listStateSnapshots,
  deleteStateSnapshot
} = await import('../../extension/background/message-handlers.js')

/**
 * Helper: installs the message listener and returns the raw handler function.
 */
function getInstalledHandler(depsOverrides = {}) {
  const addListenerFn = mock.fn()
  const origOnMessage = chrome.runtime.onMessage
  chrome.runtime.onMessage = { addListener: addListenerFn }

  const defaultDeps = {
    debugLog: mock.fn(),
    addToWsBatcher: mock.fn(),
    addToEnhancedActionBatcher: mock.fn(),
    addToNetworkBodyBatcher: mock.fn(),
    addToPerfBatcher: mock.fn(),
    addToLogBatcher: mock.fn(),
    handleLogMessage: mock.fn(() => Promise.resolve()),
    handleClearLogs: mock.fn(() => Promise.resolve({ success: true })),
    isNetworkBodyCaptureDisabled: mock.fn(() => false),
    getConnectionStatus: mock.fn(() => ({ connected: true })),
    getServerUrl: mock.fn(() => 'http://localhost:9222'),
    getScreenshotOnError: mock.fn(() => false),
    getSourceMapEnabled: mock.fn(() => false),
    getDebugMode: mock.fn(() => false),
    getContextWarning: mock.fn(() => null),
    getCircuitBreakerState: mock.fn(() => 'closed'),
    getMemoryPressureState: mock.fn(() => 'normal'),
    setCurrentLogLevel: mock.fn(),
    setScreenshotOnError: mock.fn(),
    setSourceMapEnabled: mock.fn(),
    setDebugMode: mock.fn(),
    setServerUrl: mock.fn(),
    setAiWebPilotEnabled: mock.fn((val, cb) => cb && cb()),
    getAiWebPilotEnabled: mock.fn(() => false),
    saveSetting: mock.fn(),
    exportDebugLog: mock.fn(() => []),
    clearDebugLog: mock.fn(),
    clearSourceMapCache: mock.fn(),
    captureScreenshot: mock.fn(() => Promise.resolve({ success: true })),
    forwardToAllContentScripts: mock.fn(),
    checkConnectionAndUpdate: mock.fn(),
    ...depsOverrides
  }

  installMessageListener(defaultDeps)
  chrome.runtime.onMessage = origOnMessage

  const handler = addListenerFn.mock.calls[0].arguments[0]
  return { handler, deps: defaultDeps }
}

// Trusted sender: content script with tab context
const contentScriptSender = { tab: { id: 1, url: 'http://localhost:3000' } }
// Trusted sender: extension internal
const extensionSender = { id: chrome.runtime.id }
// Untrusted sender: web page
const webPageSender = { id: 'other-extension-id', url: 'http://evil.com' }

// ============================================
// isValidMessageSender (tested via handler)
// ============================================

describe('sender validation', () => {
  test('content script sender is valid', () => {
    const { handler } = getInstalledHandler()
    const sendResponse = mock.fn()
    handler({ type: 'GET_TAB_ID' }, contentScriptSender, sendResponse)
    assert.strictEqual(sendResponse.mock.calls.length, 1)
    assert.strictEqual(sendResponse.mock.calls[0].arguments[0].tabId, 1)
  })

  test('extension page sender is valid', () => {
    const { handler } = getInstalledHandler()
    const sendResponse = mock.fn()
    handler({ type: 'getStatus' }, extensionSender, sendResponse)
    assert.strictEqual(sendResponse.mock.calls.length, 1)
    assert.ok(sendResponse.mock.calls[0].arguments[0].connected !== undefined)
  })

  test('web page sender is rejected', () => {
    const { handler, deps } = getInstalledHandler()
    const sendResponse = mock.fn()
    const result = handler({ type: 'getStatus' }, webPageSender, sendResponse)
    assert.strictEqual(result, false)
    assert.strictEqual(sendResponse.mock.calls.length, 0)
    // debugLog should be called with rejection
    assert.ok(deps.debugLog.mock.calls.some(
      c => c.arguments[0] === 'error' && c.arguments[1].includes('untrusted')
    ))
  })
})

// ============================================
// Message routing
// ============================================

describe('message routing', () => {
  test('GET_TAB_ID returns sender tab id', () => {
    const { handler } = getInstalledHandler()
    const sendResponse = mock.fn()
    handler({ type: 'GET_TAB_ID' }, contentScriptSender, sendResponse)
    assert.strictEqual(sendResponse.mock.calls[0].arguments[0].tabId, 1)
  })

  test('ws_event routes to WS batcher', () => {
    const { handler, deps } = getInstalledHandler()
    const payload = { event: 'message', data: 'hello' }
    handler({ type: 'ws_event', payload }, contentScriptSender, mock.fn())
    assert.strictEqual(deps.addToWsBatcher.mock.calls.length, 1)
    assert.deepStrictEqual(deps.addToWsBatcher.mock.calls[0].arguments[0], payload)
  })

  test('enhanced_action routes to action batcher', () => {
    const { handler, deps } = getInstalledHandler()
    const payload = { type: 'click', timestamp: 1000 }
    handler({ type: 'enhanced_action', payload }, contentScriptSender, mock.fn())
    assert.strictEqual(deps.addToEnhancedActionBatcher.mock.calls.length, 1)
  })

  test('network_body routes to body batcher with tabId', () => {
    const { handler, deps } = getInstalledHandler()
    handler({
      type: 'network_body',
      payload: { url: 'https://api.example.com', status: 200 },
      tabId: 42
    }, contentScriptSender, mock.fn())
    assert.strictEqual(deps.addToNetworkBodyBatcher.mock.calls.length, 1)
    assert.strictEqual(deps.addToNetworkBodyBatcher.mock.calls[0].arguments[0].tabId, 42)
  })

  test('network_body dropped when capture disabled', () => {
    const { handler, deps } = getInstalledHandler({
      isNetworkBodyCaptureDisabled: mock.fn(() => true)
    })
    handler({
      type: 'network_body',
      payload: { url: 'https://api.example.com' }
    }, contentScriptSender, mock.fn())
    assert.strictEqual(deps.addToNetworkBodyBatcher.mock.calls.length, 0)
    assert.ok(deps.debugLog.mock.calls.some(
      c => c.arguments[1].includes('capture disabled')
    ))
  })

  test('getStatus returns connection info', () => {
    const { handler } = getInstalledHandler({
      getConnectionStatus: mock.fn(() => ({ connected: true, latency: 5 })),
      getServerUrl: mock.fn(() => 'http://localhost:9222')
    })
    const sendResponse = mock.fn()
    handler({ type: 'getStatus' }, extensionSender, sendResponse)
    const resp = sendResponse.mock.calls[0].arguments[0]
    assert.strictEqual(resp.connected, true)
    assert.strictEqual(resp.serverUrl, 'http://localhost:9222')
  })

  test('clearLogs calls handler and responds', async () => {
    const { handler } = getInstalledHandler({
      handleClearLogs: mock.fn(() => Promise.resolve({ cleared: 50 }))
    })
    const sendResponse = mock.fn()
    handler({ type: 'clearLogs' }, extensionSender, sendResponse)
    // Wait for async handler
    await new Promise(r => setTimeout(r, 10))
    assert.strictEqual(sendResponse.mock.calls.length, 1)
    assert.strictEqual(sendResponse.mock.calls[0].arguments[0].cleared, 50)
  })

  test('setLogLevel updates level and persists', () => {
    const { handler, deps } = getInstalledHandler()
    handler({ type: 'setLogLevel', level: 'warn' }, extensionSender, mock.fn())
    assert.strictEqual(deps.setCurrentLogLevel.mock.calls[0].arguments[0], 'warn')
    assert.ok(deps.saveSetting.mock.calls.some(
      c => c.arguments[0] === 'logLevel' && c.arguments[1] === 'warn'
    ))
  })

  test('unknown message type returns false', () => {
    const { handler } = getInstalledHandler()
    const result = handler({ type: 'UNKNOWN_TYPE_XYZ' }, contentScriptSender, mock.fn())
    assert.strictEqual(result, false)
  })

  test('performance_snapshot routes to perf batcher', () => {
    const { handler, deps } = getInstalledHandler()
    handler({ type: 'performance_snapshot', payload: { metrics: {} } }, contentScriptSender, mock.fn())
    assert.strictEqual(deps.addToPerfBatcher.mock.calls.length, 1)
  })

  test('setDebugMode updates and persists', () => {
    const { handler, deps } = getInstalledHandler()
    const sendResponse = mock.fn()
    handler({ type: 'setDebugMode', enabled: true }, extensionSender, sendResponse)
    assert.strictEqual(deps.setDebugMode.mock.calls[0].arguments[0], true)
    assert.ok(deps.saveSetting.mock.calls.some(c => c.arguments[0] === 'debugMode'))
    assert.strictEqual(sendResponse.mock.calls[0].arguments[0].success, true)
  })

  test('getDebugLog returns log array', () => {
    const { handler } = getInstalledHandler({
      exportDebugLog: mock.fn(() => ['line1', 'line2'])
    })
    const sendResponse = mock.fn()
    handler({ type: 'getDebugLog' }, extensionSender, sendResponse)
    assert.deepStrictEqual(sendResponse.mock.calls[0].arguments[0].log, ['line1', 'line2'])
  })

  test('clearDebugLog clears and responds', () => {
    const { handler, deps } = getInstalledHandler()
    const sendResponse = mock.fn()
    handler({ type: 'clearDebugLog' }, extensionSender, sendResponse)
    assert.strictEqual(deps.clearDebugLog.mock.calls.length, 1)
    assert.strictEqual(sendResponse.mock.calls[0].arguments[0].success, true)
  })
})

// ============================================
// State snapshots
// ============================================

describe('state snapshots', () => {
  beforeEach(() => {
    // Reset storage mock to return empty snapshots
    chrome.storage.local.get = mock.fn((keys, callback) => {
      callback({ gasoline_state_snapshots: {} })
    })
    chrome.storage.local.set = mock.fn((data, callback) => {
      if (callback) callback()
    })
  })

  test('save and load roundtrip', async () => {
    const stored = {}
    chrome.storage.local.get = mock.fn((keys, callback) => {
      callback({ gasoline_state_snapshots: stored })
    })
    chrome.storage.local.set = mock.fn((data, callback) => {
      Object.assign(stored, data.gasoline_state_snapshots)
      callback()
    })

    const state = { url: 'https://example.com', timestamp: '2024-01-01T00:00:00Z' }
    const result = await saveStateSnapshot('test-snap', state)
    assert.strictEqual(result.success, true)
    assert.strictEqual(result.snapshot_name, 'test-snap')

    chrome.storage.local.get = mock.fn((keys, callback) => {
      callback({ gasoline_state_snapshots: stored })
    })

    const loaded = await loadStateSnapshot('test-snap')
    assert.strictEqual(loaded.url, 'https://example.com')
    assert.strictEqual(loaded.name, 'test-snap')
  })

  test('load nonexistent snapshot returns null', async () => {
    const loaded = await loadStateSnapshot('nonexistent')
    assert.strictEqual(loaded, null)
  })

  test('list returns array of snapshot metadata', async () => {
    chrome.storage.local.get = mock.fn((keys, callback) => {
      callback({
        gasoline_state_snapshots: {
          snap1: { name: 'snap1', url: 'https://a.com', timestamp: '2024-01-01T00:00:00Z', size_bytes: 100 },
          snap2: { name: 'snap2', url: 'https://b.com', timestamp: '2024-01-02T00:00:00Z', size_bytes: 200 }
        }
      })
    })

    const list = await listStateSnapshots()
    assert.strictEqual(list.length, 2)
    assert.ok(list.some(s => s.name === 'snap1'))
    assert.ok(list.some(s => s.name === 'snap2'))
  })

  test('delete removes snapshot', async () => {
    const stored = {
      snap1: { name: 'snap1', url: 'https://a.com' }
    }
    chrome.storage.local.get = mock.fn((keys, callback) => {
      callback({ gasoline_state_snapshots: { ...stored } })
    })
    chrome.storage.local.set = mock.fn((data, callback) => {
      Object.assign(stored, data.gasoline_state_snapshots)
      callback()
    })

    const result = await deleteStateSnapshot('snap1')
    assert.strictEqual(result.success, true)
    assert.strictEqual(result.deleted, 'snap1')
  })
})
