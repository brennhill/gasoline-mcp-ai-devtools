// @ts-nocheck
/**
 * @fileoverview popup-update-button.test.js — Tests for the "Update now" banner
 * in the extension popup. Covers banner visibility based on availableVersion,
 * the nonce→install fetch sequence, and the reload-extension CTA.
 */

import { beforeEach, describe, mock, test } from 'node:test'
import assert from 'node:assert'

let importCounter = 0
let fetchMock

function createMockElement(id) {
  const style = { display: id === 'update-available' ? 'none' : '' }
  const dataset = {}
  let onclick = null
  const listeners = new Map()
  return {
    id,
    textContent: '',
    innerHTML: '',
    style,
    dataset,
    disabled: false,
    addEventListener: mock.fn((event, handler) => {
      listeners.set(event, handler)
    }),
    dispatch: (event) => {
      const handler = listeners.get(event)
      return handler ? handler() : undefined
    },
    set onclick(fn) {
      onclick = fn
    },
    get onclick() {
      return onclick
    }
  }
}

function createMockDocument() {
  const elements = {}
  return {
    getElementById: mock.fn((id) => {
      if (!elements[id]) elements[id] = createMockElement(id)
      return elements[id]
    }),
    _elements: elements
  }
}

async function importUpdateButton() {
  const mod = await import(`../../extension/popup/update-button.js?v=${++importCounter}`)
  return mod
}

function jsonResponse(body, status = 200) {
  return {
    ok: status >= 200 && status < 300,
    status,
    json: async () => body
  }
}

describe('popup update button', () => {
  beforeEach(() => {
    mock.reset()
    fetchMock = mock.fn()
    globalThis.fetch = fetchMock
    globalThis.document = createMockDocument()
    globalThis.chrome = {
      runtime: { id: 'test-ext-id', getManifest: () => ({ version: '0.8.1' }) },
      storage: {
        local: {
          get: mock.fn((_keys, cb) => {
            cb?.({})
            return Promise.resolve({})
          })
        }
      },
      tabs: {
        create: mock.fn(() => Promise.resolve({}))
      }
    }
  })

  test('hides banner when availableVersion is missing', async () => {
    const { renderUpdateAvailableBanner } = await importUpdateButton()
    await renderUpdateAvailableBanner({ version: '0.8.2' })
    assert.strictEqual(document.getElementById('update-available').style.display, 'none')
  })

  test('hides banner when availableVersion equals current version', async () => {
    const { renderUpdateAvailableBanner } = await importUpdateButton()
    await renderUpdateAvailableBanner({ version: '0.8.2', availableVersion: '0.8.2' })
    assert.strictEqual(document.getElementById('update-available').style.display, 'none')
  })

  test('shows banner with version delta in detail when update is available', async () => {
    const { renderUpdateAvailableBanner } = await importUpdateButton()
    await renderUpdateAvailableBanner({ version: '0.8.2', availableVersion: '0.9.0' })

    const container = document.getElementById('update-available')
    assert.strictEqual(container.style.display, '')

    const detail = document.getElementById('update-available-detail')
    assert.match(detail.textContent, /v0\.8\.2.*v0\.9\.0/)

    // Idle state visible; running/reload/error hidden
    assert.strictEqual(document.getElementById('update-action-idle').style.display, '')
    assert.strictEqual(document.getElementById('update-action-running').style.display, 'none')
    assert.strictEqual(document.getElementById('update-action-reload').style.display, 'none')
    assert.strictEqual(document.getElementById('update-action-error').style.display, 'none')
  })

  test('click fetches nonce, POSTs install, then reaches reload state on version match', async () => {
    mock.timers.enable({ apis: ['setTimeout'] })

    fetchMock.mock.mockImplementation((url) => {
      if (String(url).endsWith('/upgrade/nonce')) {
        return Promise.resolve(jsonResponse({ nonce: 'a'.repeat(64) }))
      }
      if (String(url).endsWith('/upgrade/install')) {
        return Promise.resolve(jsonResponse({ status: 'installing' }, 202))
      }
      if (String(url).endsWith('/health')) {
        // New daemon has restarted and reports target version.
        return Promise.resolve(jsonResponse({ version: '0.9.0' }))
      }
      return Promise.reject(new Error(`unexpected fetch: ${url}`))
    })

    const { renderUpdateAvailableBanner } = await importUpdateButton()
    await renderUpdateAvailableBanner({ version: '0.8.2', availableVersion: '0.9.0' })

    const btn = document.getElementById('update-now-btn')
    const clickHandler = btn.addEventListener.mock.calls.find((c) => c.arguments[0] === 'click').arguments[1]
    clickHandler()

    // Drain microtasks so nonce + install fetches settle and the flow reaches
    // the first poll-tick setTimeout.
    for (let i = 0; i < 5; i++) await new Promise((resolve) => setImmediate(resolve))

    // Fire the first poll tick; the mocked /health immediately reports the
    // target version, so runUpgradeFlow transitions to the reload state.
    mock.timers.tick(2000)
    for (let i = 0; i < 5; i++) await new Promise((resolve) => setImmediate(resolve))

    // Terminal state: reload banner visible, running hidden.
    assert.strictEqual(document.getElementById('update-action-reload').style.display, '')
    assert.strictEqual(document.getElementById('update-action-running').style.display, 'none')
    assert.strictEqual(document.getElementById('update-action-error').style.display, 'none')

    // Assert the fetch sequence.
    const urls = fetchMock.mock.calls.map((c) => String(c.arguments[0]))
    assert.ok(urls.some((u) => u.endsWith('/upgrade/nonce')), 'should fetch nonce')
    assert.ok(urls.some((u) => u.endsWith('/upgrade/install')), 'should POST install')
    assert.ok(urls.some((u) => u.endsWith('/health')), 'should poll health')

    // Install POST body contains the nonce from the nonce endpoint.
    const installCall = fetchMock.mock.calls.find((c) => String(c.arguments[0]).endsWith('/upgrade/install'))
    const installBody = JSON.parse(installCall.arguments[1].body)
    assert.strictEqual(installBody.nonce, 'a'.repeat(64))

    mock.timers.reset()
  })

  test('install 429 surfaces rate-limit error', async () => {
    fetchMock.mock.mockImplementation((url) => {
      if (String(url).endsWith('/upgrade/nonce')) return Promise.resolve(jsonResponse({ nonce: 'a'.repeat(64) }))
      if (String(url).endsWith('/upgrade/install')) return Promise.resolve(jsonResponse({ error: 'rl' }, 429))
      return Promise.reject(new Error('unexpected'))
    })

    const { renderUpdateAvailableBanner } = await importUpdateButton()
    await renderUpdateAvailableBanner({ version: '0.8.2', availableVersion: '0.9.0' })

    const btn = document.getElementById('update-now-btn')
    const clickHandler = btn.addEventListener.mock.calls.find((c) => c.arguments[0] === 'click').arguments[1]
    clickHandler()
    // Click handler fire-and-forgets runUpgradeFlow; drain microtasks so the
    // rejected fetch propagates to the error branch.
    for (let i = 0; i < 5; i++) await new Promise((resolve) => setImmediate(resolve))

    const errorEl = document.getElementById('update-action-error')
    assert.strictEqual(errorEl.style.display, '')
    assert.match(errorEl.textContent, /recently|minute/i)
  })

  test('install 501 surfaces unsupported-platform error', async () => {
    fetchMock.mock.mockImplementation((url) => {
      if (String(url).endsWith('/upgrade/nonce')) return Promise.resolve(jsonResponse({ nonce: 'a'.repeat(64) }))
      if (String(url).endsWith('/upgrade/install')) return Promise.resolve(jsonResponse({ error: 'nope' }, 501))
      return Promise.reject(new Error('unexpected'))
    })

    const { renderUpdateAvailableBanner } = await importUpdateButton()
    await renderUpdateAvailableBanner({ version: '0.8.2', availableVersion: '0.9.0' })

    const btn = document.getElementById('update-now-btn')
    const clickHandler = btn.addEventListener.mock.calls.find((c) => c.arguments[0] === 'click').arguments[1]
    clickHandler()
    for (let i = 0; i < 5; i++) await new Promise((resolve) => setImmediate(resolve))

    const errorEl = document.getElementById('update-action-error')
    assert.strictEqual(errorEl.style.display, '')
    assert.match(errorEl.textContent, /not supported|re-run the installer/i)
  })

  test('reload button opens chrome extensions page with runtime.id', async () => {
    const { renderUpdateAvailableBanner } = await importUpdateButton()
    await renderUpdateAvailableBanner({ version: '0.8.2', availableVersion: '0.9.0' })

    const reloadBtn = document.getElementById('update-reload-ext-btn')
    const clickHandler = reloadBtn.addEventListener.mock.calls.find((c) => c.arguments[0] === 'click').arguments[1]
    clickHandler()

    assert.strictEqual(chrome.tabs.create.mock.calls.length, 1)
    assert.deepStrictEqual(chrome.tabs.create.mock.calls[0].arguments[0], {
      url: 'chrome://extensions/?id=test-ext-id'
    })
  })

  test('handlers wire once per render — repeat render does not double-attach', async () => {
    const { renderUpdateAvailableBanner } = await importUpdateButton()
    await renderUpdateAvailableBanner({ version: '0.8.2', availableVersion: '0.9.0' })
    await renderUpdateAvailableBanner({ version: '0.8.2', availableVersion: '0.9.0' })

    const btn = document.getElementById('update-now-btn')
    const clickCount = btn.addEventListener.mock.calls.filter((c) => c.arguments[0] === 'click').length
    assert.strictEqual(clickCount, 1, 'click handler should be wired exactly once')
  })
})
