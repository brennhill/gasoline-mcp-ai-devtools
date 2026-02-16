// @ts-nocheck
import { test, describe, mock, beforeEach } from 'node:test'
import assert from 'node:assert'
import { MANIFEST_VERSION } from './helpers.js'

function createMockChrome() {
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
      reload: mock.fn(() => Promise.resolve()),
      update: mock.fn(() => Promise.resolve()),
      goBack: mock.fn(() => Promise.resolve()),
      goForward: mock.fn(() => Promise.resolve()),
      create: mock.fn(() => Promise.resolve({ id: 99 })),
      get: mock.fn((tabId) =>
        Promise.resolve({
          id: tabId,
          status: 'complete',
          url: 'https://example.com/docs'
        })
      ),
      sendMessage: mock.fn((_tabId, _msg) => Promise.resolve({ status: 'alive' })),
      query: mock.fn((queryInfo, callback) => {
        if (typeof callback === 'function') callback([{ id: 1, windowId: 1, url: 'https://example.com' }])
        return Promise.resolve([{ id: 1, windowId: 1, url: 'https://example.com' }])
      }),
      onRemoved: { addListener: mock.fn() }
    },
    storage: {
      local: {
        get: mock.fn((keys, callback) => {
          const data = {
            serverUrl: 'http://localhost:7890',
            aiWebPilotEnabled: true,
            trackedTabId: 1
          }
          if (typeof callback === 'function') callback(data)
          return Promise.resolve(data)
        }),
        set: mock.fn((data, callback) => {
          if (typeof callback === 'function') callback()
          return Promise.resolve()
        }),
        remove: mock.fn((keys, callback) => {
          if (typeof callback === 'function') callback()
          return Promise.resolve()
        })
      },
      sync: {
        get: mock.fn((keys, callback) => {
          if (typeof callback === 'function') callback({})
          return Promise.resolve({})
        }),
        set: mock.fn((data, callback) => {
          if (typeof callback === 'function') callback()
          return Promise.resolve()
        })
      },
      session: {
        get: mock.fn((keys, callback) => {
          if (typeof callback === 'function') callback({})
          return Promise.resolve({})
        }),
        set: mock.fn((data, callback) => {
          if (typeof callback === 'function') callback()
          return Promise.resolve()
        })
      },
      onChanged: { addListener: mock.fn() }
    },
    alarms: {
      create: mock.fn(),
      onAlarm: { addListener: mock.fn() }
    },
    scripting: {
      executeScript: mock.fn(() =>
        Promise.resolve([
          {
            result: {
              success: true,
              result: {
                type: 'article',
                title: 'Mock Summary'
              }
            }
          }
        ])
      )
    }
  }
}

describe('Navigation Summary Limitations', () => {
  let browserActions
  let indexModule

  beforeEach(async () => {
    mock.reset()
    globalThis.chrome = createMockChrome()

    browserActions = await import('../../extension/background/browser-actions.js')
    indexModule = await import('../../extension/background/index.js')
    indexModule._resetPilotCacheForTesting(true)
  })

  test('refresh with summary_script should execute summary and include summary in response', async () => {
    const actionToast = mock.fn()

    const result = await browserActions.handleBrowserAction(
      1,
      {
        action: 'refresh',
        summary_script: '(() => ({ type: "article", title: "Mock Summary" }))()'
      },
      actionToast
    )

    assert.strictEqual(globalThis.chrome.scripting.executeScript.mock.calls.length, 1)
    assert.strictEqual(result.success, true)
    assert.ok(result.summary, 'refresh should include summary when summary_script is provided')
  })
})
