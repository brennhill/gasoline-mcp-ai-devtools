// @ts-nocheck
// early-patch-logs.test.js — Verifies early-patch telemetry is flushed into GASOLINE_LOG events.

import { test, describe, beforeEach, afterEach, mock } from 'node:test'
import assert from 'node:assert'

describe('early-patch log bridge', () => {
  let originalWindow

  beforeEach(() => {
    originalWindow = globalThis.window
    globalThis.window = {
      location: { href: 'http://localhost:3000/test', origin: 'http://localhost:3000' },
      postMessage: mock.fn()
    }
  })

  afterEach(() => {
    globalThis.window = originalWindow
  })

  test('flushEarlyPatchLogs forwards queued entries and clears the queue', async () => {
    const marker = `EARLY_PATCH_${Date.now()}`
    globalThis.window.__GASOLINE_EARLY_LOGS__ = [
      {
        ts: '2026-02-16T00:00:00.000Z',
        level: 'warn',
        message: 'attachShadow overwrite intercepted',
        source: 'early-patch',
        category: 'shadow_dom',
        data: { marker }
      }
    ]

    const { flushEarlyPatchLogs } = await import('../../extension/inject/early-patch-logs.js')
    const flushed = flushEarlyPatchLogs()

    assert.strictEqual(flushed, 1, 'Expected one early log to be forwarded')
    assert.strictEqual(globalThis.window.__GASOLINE_EARLY_LOGS__.length, 0, 'Queue should be emptied after flush')
    assert.strictEqual(globalThis.window.postMessage.mock.calls.length, 1, 'Expected one GASOLINE_LOG postMessage call')

    const [message] = globalThis.window.postMessage.mock.calls[0].arguments
    assert.strictEqual(message.type, 'GASOLINE_LOG')
    assert.strictEqual(message.payload.type, 'early_patch')
    assert.strictEqual(message.payload.category, 'shadow_dom')
    assert.strictEqual(message.payload.data.marker, marker)
  })

  test('flushEarlyPatchLogs is a no-op when queue is absent', async () => {
    delete globalThis.window.__GASOLINE_EARLY_LOGS__

    const { flushEarlyPatchLogs } = await import('../../extension/inject/early-patch-logs.js')
    const flushed = flushEarlyPatchLogs()

    assert.strictEqual(flushed, 0)
    assert.strictEqual(globalThis.window.postMessage.mock.calls.length, 0)
  })
})
