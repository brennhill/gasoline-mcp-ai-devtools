// @ts-nocheck
/**
 * @fileoverview pending-queries-iframe.test.js — Frame routing tests for analyze(dom/a11y).
 */

import { test, describe, beforeEach } from 'node:test'
import assert from 'node:assert'

const { handlePendingQuery } = await import('./pending-queries.js')
const { markInitComplete } = await import('./state.js')

let executeScriptCalls = []
let executeScriptReturn = []
let sendMessageCalls = []
let queuedResults = []

function makeSyncClient() {
  return {
    queueCommandResult(result) {
      queuedResults.push(result)
    }
  }
}

beforeEach(() => {
  executeScriptCalls = []
  executeScriptReturn = []
  sendMessageCalls = []
  queuedResults = []

  globalThis.chrome = {
    tabs: {
      get: async (tabId) => ({ id: tabId, url: `https://example.com/tab-${tabId}` }),
      query: async () => [{ id: 1, windowId: 1 }],
      sendMessage: async (tabId, message, options) => {
        sendMessageCalls.push({ tabId, message, options })

        const frameId = options?.frameId
        if (message.type === 'DOM_QUERY') {
          return {
            url: `https://example.com/frame-${frameId}`,
            title: `frame-${frameId}`,
            matchCount: 1,
            returnedCount: 1,
            matches: [{ tag: 'button', selector: '#submit' }]
          }
        }

        if (message.type === 'A11Y_QUERY') {
          if (frameId === 0) {
            return {
              violations: [{ id: 'main-v' }],
              passes: [{ id: 'main-p' }],
              incomplete: [],
              inapplicable: [],
              summary: { violations: 1, passes: 1, incomplete: 0, inapplicable: 0 }
            }
          }
          return {
            violations: [{ id: 'child-v' }],
            passes: [],
            incomplete: [{ id: 'child-i' }],
            inapplicable: [],
            summary: { violations: 1, passes: 0, incomplete: 1, inapplicable: 0 }
          }
        }

        return {}
      }
    },
    scripting: {
      executeScript: async (opts) => {
        executeScriptCalls.push(opts)
        return executeScriptReturn.shift() || []
      }
    },
    storage: {
      local: {
        get: async () => ({})
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
    }
  }

  // Skip initReady delay in handlePendingQuery.
  markInitComplete()
})

describe('analyze frame routing', () => {
  test('dom query routes to a matched frame_id when frame index is provided', async () => {
    executeScriptReturn.push([
      { frameId: 0, result: { matches: false } },
      { frameId: 2, result: { matches: true } }
    ])

    const query = {
      id: 'q-dom-frame',
      type: 'dom',
      tab_id: 5,
      params: { selector: '#submit', frame: 0 }
    }

    await handlePendingQuery(query, makeSyncClient())

    assert.strictEqual(executeScriptCalls.length, 1, 'should probe frames once')
    assert.strictEqual(sendMessageCalls.length, 1, 'should send DOM_QUERY to one matched frame')
    assert.strictEqual(sendMessageCalls[0].options.frameId, 2)

    assert.strictEqual(queuedResults.length, 1)
    const cmd = queuedResults[0]
    assert.strictEqual(cmd.status, 'complete')
    assert.strictEqual(cmd.result.frame_id, 2)
    assert.strictEqual(cmd.result.matchCount, 1)
    assert.strictEqual(cmd.result.resolved_tab_id, 5)
  })

  test('a11y query aggregates when frame is "all"', async () => {
    executeScriptReturn.push([
      { frameId: 0, result: { matches: true } },
      { frameId: 1, result: { matches: true } }
    ])

    const query = {
      id: 'q-a11y-all',
      type: 'a11y',
      tab_id: 9,
      params: { frame: 'all' }
    }

    await handlePendingQuery(query, makeSyncClient())

    assert.strictEqual(executeScriptCalls.length, 1, 'should probe frames once')
    assert.strictEqual(sendMessageCalls.length, 2, 'should send A11Y_QUERY to each matched frame')
    assert.strictEqual(sendMessageCalls[0].options.frameId, 0)
    assert.strictEqual(sendMessageCalls[1].options.frameId, 1)

    assert.strictEqual(queuedResults.length, 1)
    const cmd = queuedResults[0]
    assert.strictEqual(cmd.status, 'complete')
    assert.strictEqual(cmd.result.summary.violations, 2)
    assert.strictEqual(cmd.result.summary.passes, 1)
    assert.strictEqual(cmd.result.summary.incomplete, 1)
    assert.ok(Array.isArray(cmd.result.frames), 'expected per-frame aggregation metadata')
    assert.strictEqual(cmd.result.frames.length, 2)
    assert.strictEqual(cmd.result.resolved_tab_id, 9)
  })
})
