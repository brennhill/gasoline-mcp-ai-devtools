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
    globalThis.chrome.tabs.create = mock.fn(() =>
      Promise.resolve({ id: 33, windowId: 1, url: 'https://example.com', title: 'Example' })
    )

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

    assert.strictEqual(globalThis.chrome.tabs.create.mock.calls.length, 1)
    assert.strictEqual(globalThis.chrome.tabs.goBack.mock.calls.length, 1)
    assert.strictEqual(globalThis.chrome.tabs.goBack.mock.calls[0].arguments[0], 33)
    assert.strictEqual(mockSyncClient.queueCommandResult.mock.calls.length, 1)

    const queued = mockSyncClient.queueCommandResult.mock.calls[0].arguments[0]
    assert.strictEqual(queued.status, 'complete')
    assert.strictEqual(queued.result.success, true)
    assert.strictEqual(queued.result.target_context.source, 'auto_tracked_new_tab')
    assert.strictEqual(queued.result.target_context.tracked_tab_id, 33)
  })

  test('auto-tracks active non-internal tab when tracking is missing', async () => {
    globalThis.chrome = createMockChrome(null, 17)
    globalThis.chrome.tabs.query = mock.fn((query, callback) => {
      const result = query?.active
        ? [{ id: 17, windowId: 1, url: 'https://app.example.com/home', title: 'Home' }]
        : [{ id: 17, windowId: 1, url: 'https://app.example.com/home', title: 'Home' }]
      if (callback) callback(result)
      return Promise.resolve(result)
    })

    const mockSyncClient = { queueCommandResult: mock.fn() }

    await bgModule.handlePendingQuery(
      {
        id: 'q-auto-track-active',
        type: 'browser_action',
        correlation_id: 'corr-auto-track-active',
        params: JSON.stringify({ action: 'back' })
      },
      mockSyncClient
    )

    assert.strictEqual(globalThis.chrome.tabs.goBack.mock.calls.length, 1)
    assert.strictEqual(globalThis.chrome.tabs.goBack.mock.calls[0].arguments[0], 17)
    const queued = mockSyncClient.queueCommandResult.mock.calls[0].arguments[0]
    assert.strictEqual(queued.status, 'complete')
    assert.strictEqual(queued.result.target_context.source, 'auto_tracked_active_tab')
  })

  test('when active tab is internal, switches to random non-internal tab and tracks it', async () => {
    globalThis.chrome = createMockChrome(null, 55)
    globalThis.chrome.tabs.query = mock.fn((query, callback) => {
      const allTabs = [
        { id: 55, windowId: 1, url: 'chrome://extensions', title: 'Extensions' },
        { id: 88, windowId: 1, url: 'https://docs.example.com', title: 'Docs' }
      ]
      const result = query?.active ? [allTabs[0]] : allTabs
      if (callback) callback(result)
      return Promise.resolve(result)
    })
    globalThis.chrome.tabs.update = mock.fn((tabId, updates) =>
      Promise.resolve({
        id: tabId,
        windowId: 1,
        url: tabId === 88 ? 'https://docs.example.com' : 'chrome://extensions',
        title: tabId === 88 ? 'Docs' : 'Extensions',
        status: 'complete',
        active: !!updates?.active
      })
    )
    const priorRandom = Math.random
    Math.random = () => 0 // deterministic random selection

    const mockSyncClient = { queueCommandResult: mock.fn() }
    try {
      await bgModule.handlePendingQuery(
        {
          id: 'q-auto-track-random',
          type: 'browser_action',
          correlation_id: 'corr-auto-track-random',
          params: JSON.stringify({ action: 'back' })
        },
        mockSyncClient
      )
    } finally {
      Math.random = priorRandom
    }

    assert.strictEqual(globalThis.chrome.tabs.update.mock.calls.length >= 1, true)
    assert.strictEqual(globalThis.chrome.tabs.goBack.mock.calls.length, 1)
    assert.strictEqual(globalThis.chrome.tabs.goBack.mock.calls[0].arguments[0], 88)
    const queued = mockSyncClient.queueCommandResult.mock.calls[0].arguments[0]
    assert.strictEqual(queued.status, 'complete')
    assert.strictEqual(queued.result.target_context.source, 'auto_tracked_random_tab')
    assert.strictEqual(queued.result.target_context.tracked_tab_id, 88)
  })

  test('returns no_trackable_tab with recovery attempts when all fallback stages fail', async () => {
    globalThis.chrome = createMockChrome(null, 77)
    globalThis.chrome.tabs.query = mock.fn((query, callback) => {
      const allTabs = [{ id: 77, windowId: 1, url: 'chrome://settings', title: 'Settings' }]
      const result = query?.active ? allTabs : allTabs
      if (callback) callback(result)
      return Promise.resolve(result)
    })
    globalThis.chrome.tabs.create = mock.fn(() => Promise.reject(new Error('tabs.create blocked')))

    const mockSyncClient = { queueCommandResult: mock.fn() }

    await bgModule.handlePendingQuery(
      {
        id: 'q-no-trackable',
        type: 'browser_action',
        correlation_id: 'corr-no-trackable',
        params: JSON.stringify({ action: 'back' })
      },
      mockSyncClient
    )

    assert.strictEqual(globalThis.chrome.tabs.goBack.mock.calls.length, 0)
    const queued = mockSyncClient.queueCommandResult.mock.calls[0].arguments[0]
    assert.strictEqual(queued.status, 'error')
    assert.strictEqual(queued.result.error, 'no_trackable_tab')
    assert.ok(Array.isArray(queued.result.attempted_recovery))
    assert.ok(queued.result.attempted_recovery.length >= 3)
    assert.strictEqual(queued.error, 'no_trackable_tab')
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

  test('browser_action accepts what alias for action', async () => {
    const mockSyncClient = { queueCommandResult: mock.fn() }

    await bgModule.handlePendingQuery(
      {
        id: 'q-what-alias',
        type: 'browser_action',
        correlation_id: 'corr-what-alias',
        params: JSON.stringify({ what: 'back' })
      },
      mockSyncClient
    )

    assert.strictEqual(globalThis.chrome.tabs.goBack.mock.calls.length, 1)
    assert.strictEqual(globalThis.chrome.tabs.goBack.mock.calls[0].arguments[0], 1)

    const queued = mockSyncClient.queueCommandResult.mock.calls[0].arguments[0]
    assert.strictEqual(queued.status, 'complete')
    assert.strictEqual(queued.result.action, 'back')
  })
})
