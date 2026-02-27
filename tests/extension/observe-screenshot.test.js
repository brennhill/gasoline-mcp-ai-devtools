// @ts-nocheck
/**
 * @fileoverview observe-screenshot.test.js — Regression tests for screenshot observe command.
 * Ensures explicit MCP screenshot requests are not blocked by local extension rate-limiting.
 */

import { describe, test, mock, beforeEach } from 'node:test'
import assert from 'node:assert'

const registered = new Map()
const mockRegisterCommand = mock.fn((name, handler) => {
  registered.set(name, handler)
})

const mockCanTakeScreenshot = mock.fn(() => ({
  allowed: false,
  reason: 'rate_limit',
  nextAllowedIn: 5000
}))
const mockRecordScreenshot = mock.fn()
const mockDebugLog = mock.fn()

mock.module('../../extension/background/commands/registry.js', {
  namedExports: {
    registerCommand: mockRegisterCommand
  }
})

mock.module('../../extension/background/state-manager.js', {
  namedExports: {
    canTakeScreenshot: mockCanTakeScreenshot,
    recordScreenshot: mockRecordScreenshot
  }
})

mock.module('../../extension/background/index.js', {
  namedExports: {
    debugLog: mockDebugLog
  }
})

mock.module('../../extension/background/state.js', {
  namedExports: {
    getServerUrl: () => 'http://localhost:7890'
  }
})

mock.module('../../extension/background/debug.js', {
  namedExports: {
    DebugCategory: { CAPTURE: 'capture' }
  }
})

globalThis.chrome = {
  windows: {
    update: mock.fn(async (windowId, updates) => ({ id: windowId, ...updates }))
  },
  tabs: {
    get: mock.fn(async () => ({ windowId: 11, url: 'https://www.linkedin.com/feed/' })),
    update: mock.fn(async (tabId, updates) => ({ id: tabId, windowId: 11, url: 'https://www.linkedin.com/feed/', ...updates })),
    query: mock.fn(async () => [{ id: 123, windowId: 11, active: true, url: 'https://www.linkedin.com/feed/' }]),
    captureVisibleTab: mock.fn(async () => 'data:image/jpeg;base64,Zm9v')
  }
}

globalThis.fetch = mock.fn(async () => ({
  ok: true,
  status: 200
}))

await import('../../extension/background/commands/observe.js')

describe('observe screenshot command', () => {
  beforeEach(() => {
    mockCanTakeScreenshot.mock.resetCalls()
    mockRecordScreenshot.mock.resetCalls()
    mockDebugLog.mock.resetCalls()
    globalThis.chrome.windows.update.mock.resetCalls()
    globalThis.chrome.tabs.get.mock.resetCalls()
    globalThis.chrome.tabs.update.mock.resetCalls()
    globalThis.chrome.tabs.query.mock.resetCalls()
    globalThis.chrome.tabs.captureVisibleTab.mock.resetCalls()
    globalThis.fetch.mock.resetCalls()
  })

  test('bypasses local screenshot limiter for explicit observe(screenshot)', async () => {
    const handler = registered.get('screenshot')
    assert.ok(handler, 'screenshot handler should be registered')

    const sendResult = mock.fn()
    await handler({
      tabId: 123,
      query: { id: 'q-1' },
      sendResult
    })

    assert.strictEqual(sendResult.mock.calls.length, 0, 'success path should resolve via server/query_id')
    assert.strictEqual(globalThis.chrome.tabs.get.mock.calls.length, 1)
    assert.strictEqual(globalThis.chrome.windows.update.mock.calls.length, 1)
    assert.deepStrictEqual(globalThis.chrome.windows.update.mock.calls[0].arguments, [11, { focused: true }])
    assert.strictEqual(globalThis.chrome.tabs.update.mock.calls.length, 1)
    assert.deepStrictEqual(globalThis.chrome.tabs.update.mock.calls[0].arguments, [123, { active: true }])
    assert.strictEqual(globalThis.chrome.tabs.query.mock.calls.length, 1)
    assert.strictEqual(globalThis.chrome.tabs.captureVisibleTab.mock.calls.length, 1)
    assert.deepStrictEqual(globalThis.chrome.tabs.captureVisibleTab.mock.calls[0].arguments, [
      11,
      { format: 'jpeg', quality: 80 }
    ])
    assert.strictEqual(mockRecordScreenshot.mock.calls.length, 1)
    assert.strictEqual(globalThis.fetch.mock.calls.length, 1)
    assert.strictEqual(mockCanTakeScreenshot.mock.calls.length, 0, 'local limiter should not gate explicit screenshot')
  })

  test('returns screenshot_failed when target tab is not active in target window', async () => {
    const handler = registered.get('screenshot')
    assert.ok(handler, 'screenshot handler should be registered')

    globalThis.chrome.tabs.query.mock.mockImplementationOnce(async () => [
      { id: 999, windowId: 11, active: true, url: 'https://example.com/other' }
    ])

    const sendResult = mock.fn()
    await handler({
      tabId: 123,
      query: { id: 'q-2' },
      sendResult
    })

    assert.strictEqual(sendResult.mock.calls.length, 1)
    const payload = sendResult.mock.calls[0].arguments[0]
    assert.strictEqual(payload.error, 'screenshot_failed')
    assert.match(payload.message, /Failed to activate target tab/)
  })
})
