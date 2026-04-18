// @ts-nocheck
/**
 * @fileoverview workspace-actions.test.js — Tests for shared workspace action helpers.
 */

import { beforeEach, describe, mock, test } from 'node:test'
import assert from 'node:assert'

describe('workspace actions', () => {
  beforeEach(() => {
    mock.reset()
    globalThis.chrome = {
      runtime: {
        sendMessage: mock.fn((message) => Promise.resolve({ success: true, type: message?.type }))
      },
      tabs: {
        sendMessage: mock.fn((_tabId, message) => Promise.resolve({ success: true, type: message?.type }))
      }
    }
  })

  test('requests screenshots through the shared runtime action helper', async () => {
    const { requestWorkspaceScreenshot } = await import(`../../extension/lib/workspace-actions.js?screenshot=1`)

    await requestWorkspaceScreenshot()

    const sentTypes = chrome.runtime.sendMessage.mock.calls.map((call) => call.arguments[0]?.type)
    assert.deepStrictEqual(sentTypes, ['capture_screenshot'])
  })

  test('requests audit through the shared workspace action path', async () => {
    const { requestWorkspaceAudit } = await import(`../../extension/lib/workspace-actions.js?audit=1`)

    await requestWorkspaceAudit('https://tracked.example/')

    const sentTypes = chrome.runtime.sendMessage.mock.calls.map((call) => call.arguments[0]?.type)
    assert.deepStrictEqual(sentTypes, ['open_terminal_panel', 'qa_scan_requested'])
    assert.strictEqual(chrome.runtime.sendMessage.mock.calls[1].arguments[0].page_url, 'https://tracked.example/')
  })

  test('requests note mode through the shared draw-mode action helper', async () => {
    const { requestWorkspaceNoteMode } = await import(`../../extension/lib/workspace-actions.js?note=1`)

    await requestWorkspaceNoteMode(42)

    assert.deepStrictEqual(chrome.tabs.sendMessage.mock.calls[0].arguments, [
      42,
      { type: 'kaboom_draw_mode_start', started_by: 'user' }
    ])
  })

  test('toggles recording through the shared runtime action helper', async () => {
    const { toggleWorkspaceRecording } = await import(`../../extension/lib/workspace-actions.js?record=1`)

    await toggleWorkspaceRecording(false)
    await toggleWorkspaceRecording(true)

    const sentMessages = chrome.runtime.sendMessage.mock.calls.map((call) => call.arguments[0])
    assert.deepStrictEqual(sentMessages, [
      { type: 'screen_recording_start', audio: '' },
      { type: 'screen_recording_stop' }
    ])
  })
})
