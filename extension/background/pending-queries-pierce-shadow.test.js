// @ts-nocheck
import { test, describe, beforeEach } from 'node:test'
import assert from 'node:assert'

const { handlePendingQuery } = await import('./pending-queries.js')
const index = await import('./index.js')

let queuedResults = []
let sendMessageCalls = []
let trackedState = { trackedTabId: null, trackedTabUrl: null, trackedTabTitle: null }
let tabRegistry = new Map()

function makeSyncClient() {
  return {
    queueCommandResult(result) {
      queuedResults.push(result)
    }
  }
}

function resetChromeMock() {
  globalThis.chrome = {
    tabs: {
      get: async (tabId) => {
        const tab = tabRegistry.get(tabId)
        if (!tab) throw new Error(`No tab with id ${tabId}`)
        return tab
      },
      query: async () => [{ id: 1, windowId: 1, url: 'https://active.example.com/home' }],
      sendMessage: async (tabId, message, options) => {
        sendMessageCalls.push({ tabId, message, options })
        if (message.type === 'DOM_QUERY') {
          return {
            url: `https://result.example.com/tab-${tabId}`,
            title: 'Mock Result',
            matchCount: 0,
            returnedCount: 0,
            matches: []
          }
        }
        return {}
      }
    },
    scripting: {
      executeScript: async () => [{ frameId: 0, result: { matches: true } }]
    },
    storage: {
      local: {
        get: (_keys, cb) => cb(trackedState)
      }
    },
    runtime: {
      sendMessage: async () => ({}),
      getManifest: () => ({ version: '0.0.0-test' }),
      onMessage: { addListener: () => {} }
    },
    action: {
      setBadgeText: () => {},
      setBadgeBackgroundColor: () => {}
    },
    alarms: { create: () => {}, onAlarm: { addListener: () => {} } },
    commands: { onCommand: { addListener: () => {} } }
  }
}

function setupTrackedTarget(tabId, tabUrl) {
  trackedState = {
    trackedTabId: tabId,
    trackedTabUrl: tabUrl,
    trackedTabTitle: 'Tracked'
  }
  tabRegistry = new Map([[tabId, { id: tabId, url: tabUrl, windowId: 1 }]])
}

describe('pending dom query pierce_shadow auto heuristic', () => {
  beforeEach(() => {
    queuedResults = []
    sendMessageCalls = []
    trackedState = { trackedTabId: null, trackedTabUrl: null, trackedTabTitle: null }
    tabRegistry = new Map([[1, { id: 1, url: 'https://active.example.com/home', windowId: 1 }]])
    resetChromeMock()
    index.markInitComplete()
  })

  test('auto resolves to true with active debug intent (pilot enabled + tracked target origin)', async () => {
    setupTrackedTarget(12, 'https://app.example.com/dashboard')
    index._resetPilotCacheForTesting(true)

    await handlePendingQuery(
      {
        id: 'q-dom-auto-on',
        type: 'dom',
        params: { selector: 'button', pierce_shadow: 'auto' }
      },
      makeSyncClient()
    )

    assert.strictEqual(sendMessageCalls.length, 1)
    assert.strictEqual(sendMessageCalls[0].message.type, 'DOM_QUERY')
    assert.strictEqual(sendMessageCalls[0].message.params.pierce_shadow, true)
  })

  test('auto resolves to false when AI Web Pilot is disabled', async () => {
    setupTrackedTarget(15, 'https://app.example.com/settings')
    index._resetPilotCacheForTesting(false)

    await handlePendingQuery(
      {
        id: 'q-dom-auto-off-pilot',
        type: 'dom',
        params: { selector: 'button', pierce_shadow: 'auto' }
      },
      makeSyncClient()
    )

    assert.strictEqual(sendMessageCalls.length, 1)
    assert.strictEqual(sendMessageCalls[0].message.params.pierce_shadow, false)
  })

  test('auto resolves to false when tracked origin does not match target URL origin', async () => {
    trackedState = {
      trackedTabId: 20,
      trackedTabUrl: 'https://app.example.com/home',
      trackedTabTitle: 'Tracked'
    }
    tabRegistry = new Map([[20, { id: 20, url: 'https://other.example.org/home', windowId: 1 }]])
    index._resetPilotCacheForTesting(true)

    await handlePendingQuery(
      {
        id: 'q-dom-auto-origin-mismatch',
        type: 'dom',
        params: { selector: '#root', pierce_shadow: 'auto' }
      },
      makeSyncClient()
    )

    assert.strictEqual(sendMessageCalls.length, 1)
    assert.strictEqual(sendMessageCalls[0].message.params.pierce_shadow, false)
  })

  test('explicit true bypasses auto heuristic', async () => {
    setupTrackedTarget(33, 'https://app.example.com/home')
    index._resetPilotCacheForTesting(false)

    await handlePendingQuery(
      {
        id: 'q-dom-explicit-true',
        type: 'dom',
        params: { selector: '.x', pierce_shadow: true }
      },
      makeSyncClient()
    )

    assert.strictEqual(sendMessageCalls.length, 1)
    assert.strictEqual(sendMessageCalls[0].message.params.pierce_shadow, true)
  })

  test('explicit false bypasses auto heuristic', async () => {
    setupTrackedTarget(34, 'https://app.example.com/home')
    index._resetPilotCacheForTesting(true)

    await handlePendingQuery(
      {
        id: 'q-dom-explicit-false',
        type: 'dom',
        params: { selector: '.x', pierce_shadow: false }
      },
      makeSyncClient()
    )

    assert.strictEqual(sendMessageCalls.length, 1)
    assert.strictEqual(sendMessageCalls[0].message.params.pierce_shadow, false)
  })

  test('invalid pierce_shadow value returns invalid_param without dispatching DOM_QUERY', async () => {
    setupTrackedTarget(35, 'https://app.example.com/home')
    index._resetPilotCacheForTesting(true)

    await handlePendingQuery(
      {
        id: 'q-dom-invalid',
        type: 'dom',
        params: { selector: '.x', pierce_shadow: 'sometimes' }
      },
      makeSyncClient()
    )

    assert.strictEqual(sendMessageCalls.length, 0)
    assert.strictEqual(queuedResults.length, 1)
    assert.strictEqual(queuedResults[0].status, 'complete')
    assert.strictEqual(queuedResults[0].result.error, 'invalid_param')
    assert.ok(String(queuedResults[0].result.message).includes('pierce_shadow'))
  })

  test('auto matching is case-insensitive', async () => {
    setupTrackedTarget(36, 'https://app.example.com/home')
    index._resetPilotCacheForTesting(true)

    await handlePendingQuery(
      {
        id: 'q-dom-auto-uppercase',
        type: 'dom',
        params: { selector: '.x', pierce_shadow: 'AUTO' }
      },
      makeSyncClient()
    )

    assert.strictEqual(sendMessageCalls.length, 1)
    assert.strictEqual(sendMessageCalls[0].message.params.pierce_shadow, true)
  })
})
