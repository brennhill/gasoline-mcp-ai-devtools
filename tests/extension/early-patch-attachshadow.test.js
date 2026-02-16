// @ts-nocheck
// early-patch-attachshadow.test.js — Verifies attachShadow capture survives page overrides.

import { test, describe, beforeEach, afterEach, mock } from 'node:test'
import assert from 'node:assert'
import { pathToFileURL } from 'node:url'
import path from 'node:path'

class MockShadowRoot {
  constructor(mode = 'open') {
    this.mode = mode
  }
}

function nativeAttachShadow(init) {
  const mode = init?.mode || 'open'
  const root = new MockShadowRoot(mode)
  if (mode === 'open') {
    this.shadowRoot = root
  }
  return root
}

class MockElement {
  constructor() {
    this.shadowRoot = null
  }
}

async function importEarlyPatchFresh() {
  const earlyPatchFile = pathToFileURL(path.resolve(process.cwd(), 'extension/early-patch.js'))
  earlyPatchFile.search = `?t=${Date.now()}_${Math.random().toString(16).slice(2)}`
  await import(earlyPatchFile.href)
}

describe('early-patch attachShadow hardening', () => {
  let originalWindow
  let originalElement
  let originalShadowRoot

  beforeEach(() => {
    originalWindow = globalThis.window
    originalElement = globalThis.Element
    originalShadowRoot = globalThis.ShadowRoot

    globalThis.window = {
      location: { href: 'http://localhost:3000/', origin: 'http://localhost:3000' },
      postMessage: mock.fn()
    }
    globalThis.ShadowRoot = MockShadowRoot
    globalThis.Element = MockElement
    Object.defineProperty(globalThis.Element.prototype, 'attachShadow', {
      configurable: true,
      enumerable: false,
      writable: true,
      value: nativeAttachShadow
    })
  })

  afterEach(() => {
    globalThis.window = originalWindow
    globalThis.Element = originalElement
    globalThis.ShadowRoot = originalShadowRoot
  })

  test('captures closed roots before and after page-level override', async () => {
    await importEarlyPatchFresh()

    const firstHost = new globalThis.Element()
    firstHost.attachShadow({ mode: 'closed' })
    assert.ok(globalThis.window.__GASOLINE_CLOSED_SHADOWS__.has(firstHost), 'Expected first closed root to be captured')

    let replacementCalls = 0
    const replacement = function (init) {
      replacementCalls += 1
      const root = globalThis.window.__GASOLINE_ORIGINAL_ATTACH_SHADOW__.call(this, init)
      root.replacement_called = true
      return root
    }
    replacement.__gasolineMarker = 'ATTACH_SHADOW_SMOKE_MARKER'

    globalThis.window.__GASOLINE_INJECT_READY__ = true

    // Simulate hostile page overwrite.
    globalThis.Element.prototype.attachShadow = replacement

    const secondHost = new globalThis.Element()
    const secondRoot = secondHost.attachShadow({ mode: 'closed' })

    assert.strictEqual(replacementCalls, 1, 'Page replacement should still execute')
    assert.strictEqual(secondRoot.replacement_called, true, 'Replacement return path should be preserved')
    assert.ok(globalThis.window.__GASOLINE_CLOSED_SHADOWS__.has(secondHost), 'Capture should survive page overwrite')

    assert.ok(globalThis.window.postMessage.mock.calls.length > 0, 'Expected best-effort immediate postMessage telemetry')
    const [message] = globalThis.window.postMessage.mock.calls[0].arguments
    assert.strictEqual(message.type, 'GASOLINE_LOG')
    assert.strictEqual(message.payload.message, 'attachShadow overwrite intercepted')
    assert.strictEqual(message.payload.data.marker, 'ATTACH_SHADOW_SMOKE_MARKER')
  })

  test('ignores non-function overwrite attempts and keeps capture intact', async () => {
    await importEarlyPatchFresh()

    globalThis.Element.prototype.attachShadow = 42

    const host = new globalThis.Element()
    host.attachShadow({ mode: 'closed' })
    assert.ok(globalThis.window.__GASOLINE_CLOSED_SHADOWS__.has(host))

    const queued = globalThis.window.__GASOLINE_EARLY_LOGS__ || []
    const ignoredLog = queued.find((e) => e.message === 'attachShadow overwrite ignored (non-function)')
    assert.ok(ignoredLog, 'Expected telemetry log for ignored non-function overwrite')
  })
})
