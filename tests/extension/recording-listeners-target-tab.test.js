// @ts-nocheck
import { beforeEach, describe, test, mock } from 'node:test'
import assert from 'node:assert'

let runtimeListeners = []

function createChromeMock({ trackedTabId = 99 } = {}) {
  runtimeListeners = []
  return {
    runtime: {
      id: 'test-extension-id',
      onMessage: {
        addListener: mock.fn((listener) => runtimeListeners.push(listener))
      }
    },
    action: {
      setBadgeBackgroundColor: mock.fn(),
      setBadgeText: mock.fn()
    },
    storage: {
      local: {
        get: mock.fn((key, cb) => {
          const result = key === 'trackedTabId' ? { trackedTabId } : {}
          if (typeof cb === 'function') {
            cb(result)
            return
          }
          return Promise.resolve(result)
        }),
        set: mock.fn(() => Promise.resolve()),
        remove: mock.fn(() => Promise.resolve())
      }
    },
    tabs: {
      get: mock.fn((tabId) =>
        Promise.resolve({
          id: tabId,
          url: `https://tracked.example.com/path/${tabId}`,
          title: `Tracked ${tabId}`
        })
      ),
      query: mock.fn(() => Promise.resolve([{ id: 42, url: 'https://active.example.com', title: 'Active' }])),
      sendMessage: mock.fn(() => Promise.resolve({ draw_mode_active: false })),
      update: mock.fn(() => Promise.resolve())
    }
  }
}

function dispatchPopupMessage(message) {
  return new Promise((resolve) => {
    const sender = { id: globalThis.chrome.runtime.id }
    const sendResponse = (response) => resolve(response)
    for (const listener of runtimeListeners) {
      const handledAsync = listener(message, sender, sendResponse)
      if (handledAsync === true) return
    }
    resolve(undefined)
  })
}

describe('recording listeners popup target selection', () => {
  beforeEach(() => {
    globalThis.chrome = createChromeMock()
  })

  test('popup screen_recording_start targets tracked tab when available', async () => {
    const { installRecordingListeners } = await import('../../extension/background/recording/listeners.js')

    const deps = {
      startRecording: mock.fn(async () => ({ status: 'recording', name: 'tracked', startTime: Date.now() })),
      stopRecording: mock.fn(async () => ({ status: 'saved', name: 'tracked' })),
      isActive: () => false,
      getTabId: () => 0,
      setInactive: () => {},
      clearRecordingState: async () => {},
      getServerUrl: () => 'http://localhost:7890'
    }

    installRecordingListeners(deps)
    const response = await dispatchPopupMessage({ type: 'screen_recording_start', audio: '' })

    assert.strictEqual(response?.status, 'recording')
    assert.strictEqual(deps.startRecording.mock.calls.length, 1)

    const startCall = deps.startRecording.mock.calls[0].arguments
    assert.strictEqual(startCall[5], 99, 'Expected tracked tab ID as targetTabId')
    assert.ok(String(startCall[0]).includes('tracked-example-com'), 'Expected slug derived from tracked tab URL')
  })
})

