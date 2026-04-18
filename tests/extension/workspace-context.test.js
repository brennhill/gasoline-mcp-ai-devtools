// @ts-nocheck
/**
 * @fileoverview workspace-context.test.js — Regression coverage for workspace context refresh and injection behavior.
 */

import { beforeEach, describe, mock, test } from 'node:test'
import assert from 'node:assert'

let tabUpdatedListeners = []

function makeSnapshot(url, summary = 'Updated summary') {
  return {
    mode: 'live',
    seo: { label: 'SEO', score: 64, state: 'needs_attention', source: 'heuristic' },
    accessibility: { label: 'Accessibility', score: 72, state: 'needs_attention', source: 'heuristic' },
    performance: { verdict: 'mixed', source: 'heuristic' },
    session: { recording_active: false, screenshot_count: 0, note_count: 0 },
    audit: { updated_at: null, state: 'idle' },
    page: {
      title: 'Tracked page',
      url,
      summary
    },
    recommendation: 'Run an audit to confirm page health.'
  }
}

function sleep(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms))
}

function emitTabUpdated(tabId, url) {
  for (const listener of tabUpdatedListeners) {
    listener(tabId, { url, status: 'complete' }, { id: tabId, url })
  }
}

describe('workspace context controller', () => {
  beforeEach(() => {
    mock.reset()
    tabUpdatedListeners = []
    globalThis.chrome = {
      tabs: {
        onUpdated: {
          addListener: mock.fn((listener) => {
            tabUpdatedListeners.push(listener)
          }),
          removeListener: mock.fn((listener) => {
            tabUpdatedListeners = tabUpdatedListeners.filter((item) => item !== listener)
          })
        }
      }
    }
  })

  test('retries route refresh when the first navigation snapshot is unavailable', async () => {
    const { createWorkspaceContextController } = await import(`../../extension/sidepanel/workspace-context.js?v=${Date.now()}`)
    const writes = []
    const uiMessages = []
    let refreshAttempts = 0
    const refreshWorkspaceStatus = mock.fn(() => {
      refreshAttempts += 1
      if (refreshAttempts === 1) return Promise.resolve(undefined)
      return Promise.resolve(makeSnapshot('https://tracked.example/new-route', 'New route summary'))
    })

    const controller = createWorkspaceContextController({
      hostTabId: 7,
      writeToTerminal: (text) => {
        writes.push(text)
      },
      shouldDeferWrite: () => false,
      onUiStateChange: (state) => {
        uiMessages.push(state.message)
      },
      refreshWorkspaceStatus
    })

    controller.setSnapshot(makeSnapshot('https://tracked.example/original', 'Original route summary'))
    emitTabUpdated(7, 'https://tracked.example/new-route')

    await sleep(200)

    assert.strictEqual(refreshWorkspaceStatus.mock.calls.length, 2)
    assert.ok(writes.some((text) => /new-route/i.test(text)))
    assert.ok(uiMessages.includes('Context injection queued for the new route.'))

    controller.dispose()
  })
})
