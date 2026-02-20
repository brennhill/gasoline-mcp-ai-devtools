// @ts-nocheck
import { beforeEach, describe, mock, test } from 'node:test'
import assert from 'node:assert'
import { MANIFEST_VERSION } from './helpers.js'

function createMockChrome(trackedTabId = 1, activeTabId = 1) {
  return {
    runtime: {
      onMessage: { addListener: mock.fn() },
      onInstalled: { addListener: mock.fn() },
      sendMessage: mock.fn(() => Promise.resolve()),
      getManifest: () => ({ version: MANIFEST_VERSION })
    },
    action: {
      setBadgeText: mock.fn(),
      setBadgeBackgroundColor: mock.fn()
    },
    tabs: {
      query: mock.fn((query, callback) => {
        const result = query?.active ? [{ id: activeTabId, windowId: 1, url: `https://active/${activeTabId}` }] : []
        if (callback) {
          callback(result)
        }
        return Promise.resolve(result)
      }),
      sendMessage: mock.fn(() => Promise.resolve({ success: true })),
      get: mock.fn((tabId) =>
        Promise.resolve({
          id: tabId,
          windowId: 1,
          url: `https://tab/${tabId}`,
          status: 'complete'
        })
      ),
      goBack: mock.fn(() => Promise.resolve()),
      goForward: mock.fn(() => Promise.resolve()),
      reload: mock.fn(() => Promise.resolve()),
      update: mock.fn(() => Promise.resolve()),
      create: mock.fn(() => Promise.resolve({ id: 2 })),
      onRemoved: { addListener: mock.fn() }
    },
    storage: {
      local: {
        get: mock.fn((keys, callback) => {
          const data = {
            serverUrl: 'http://localhost:7890',
            aiWebPilotEnabled: true,
            trackedTabId
          }
          if (callback) callback(data)
          return Promise.resolve(data)
        }),
        set: mock.fn((data, callback) => {
          if (callback) callback()
          return Promise.resolve()
        }),
        remove: mock.fn((keys, callback) => {
          if (callback) callback()
          return Promise.resolve()
        })
      },
      sync: {
        get: mock.fn((keys, callback) => {
          if (callback) callback({})
          return Promise.resolve({})
        }),
        set: mock.fn((data, callback) => {
          if (callback) callback()
          return Promise.resolve()
        })
      },
      session: {
        get: mock.fn((keys, callback) => {
          if (callback) callback({})
          return Promise.resolve({})
        }),
        set: mock.fn((data, callback) => {
          if (callback) callback()
          return Promise.resolve()
        })
      },
      onChanged: { addListener: mock.fn() }
    },
    alarms: {
      create: mock.fn(),
      onAlarm: { addListener: mock.fn() }
    }
  }
}

describe('pending query targeting', () => {
  let bgModule

  beforeEach(async () => {
    mock.reset()
    globalThis.chrome = createMockChrome(1, 1)
    globalThis.fetch = mock.fn(() =>
      Promise.resolve({
        ok: true,
        json: () => Promise.resolve({ queries: [] })
      })
    )

    bgModule = await import('../../extension/background.js')
    bgModule.markInitComplete()
    bgModule._resetPilotCacheForTesting(true)
  })

  test('uses explicit tab_id over tracked tab and returns resolved target metadata', async () => {
    const mockSyncClient = { queueCommandResult: mock.fn() }

    await bgModule.handlePendingQuery(
      {
        id: 'q-explicit',
        type: 'browser_action',
        correlation_id: 'corr-explicit',
        tab_id: 99,
        params: JSON.stringify({ action: 'back' })
      },
      mockSyncClient
    )

    assert.strictEqual(globalThis.chrome.tabs.goBack.mock.calls.length, 1)
    assert.strictEqual(globalThis.chrome.tabs.goBack.mock.calls[0].arguments[0], 99)

    const queued = mockSyncClient.queueCommandResult.mock.calls[0].arguments[0]
    assert.strictEqual(queued.result.resolved_tab_id, 99)
    assert.strictEqual(queued.result.target_context.source, 'explicit_tab')
  })

  test('use_active_tab=true overrides tracked tab fallback', async () => {
    globalThis.chrome = createMockChrome(1, 7)
    const mockSyncClient = { queueCommandResult: mock.fn() }

    await bgModule.handlePendingQuery(
      {
        id: 'q-active',
        type: 'browser_action',
        correlation_id: 'corr-active',
        params: JSON.stringify({ action: 'back', use_active_tab: true })
      },
      mockSyncClient
    )

    assert.strictEqual(globalThis.chrome.tabs.goBack.mock.calls.length, 1)
    assert.strictEqual(globalThis.chrome.tabs.goBack.mock.calls[0].arguments[0], 7)

    const queued = mockSyncClient.queueCommandResult.mock.calls[0].arguments[0]
    assert.strictEqual(queued.result.resolved_tab_id, 7)
    assert.strictEqual(queued.result.target_context.source, 'active_tab')
  })

  test('returns deterministic missing_target error when no tab is targetable', async () => {
    globalThis.chrome = createMockChrome(null, 0)
    globalThis.chrome.tabs.query = mock.fn((query, callback) => {
      const result = []
      if (callback) callback(result)
      return Promise.resolve(result)
    })

    const mockSyncClient = { queueCommandResult: mock.fn() }

    await bgModule.handlePendingQuery(
      {
        id: 'q-missing',
        type: 'browser_action',
        correlation_id: 'corr-missing',
        params: JSON.stringify({ action: 'back' })
      },
      mockSyncClient
    )

    assert.strictEqual(globalThis.chrome.tabs.goBack.mock.calls.length, 0)
    assert.strictEqual(mockSyncClient.queueCommandResult.mock.calls.length, 1)

    const queued = mockSyncClient.queueCommandResult.mock.calls[0].arguments[0]
    assert.strictEqual(queued.status, 'error')
    assert.strictEqual(queued.result.error, 'missing_target')
    assert.ok(String(queued.error).includes('No target tab resolved'))
  })

  test('new_tab includes created tab_id in queued result', async () => {
    globalThis.chrome.tabs.create = mock.fn(() => Promise.resolve({ id: 222, url: 'https://new.example' }))
    const mockSyncClient = { queueCommandResult: mock.fn() }

    await bgModule.handlePendingQuery(
      {
        id: 'q-new-tab',
        type: 'browser_action',
        correlation_id: 'corr-new-tab',
        tab_id: 99,
        params: JSON.stringify({ action: 'new_tab', url: 'https://new.example' })
      },
      mockSyncClient
    )

    assert.strictEqual(globalThis.chrome.tabs.create.mock.calls.length, 1)
    assert.deepStrictEqual(globalThis.chrome.tabs.create.mock.calls[0].arguments[0], {
      url: 'https://new.example',
      active: false
    })

    const queued = mockSyncClient.queueCommandResult.mock.calls[0].arguments[0]
    assert.strictEqual(queued.status, 'complete')
    assert.strictEqual(queued.result.action, 'new_tab')
    assert.strictEqual(queued.result.tab_id, 222)
    assert.strictEqual(queued.result.resolved_tab_id, 99)
  })
})
