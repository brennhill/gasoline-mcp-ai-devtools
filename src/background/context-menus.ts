/**
 * Purpose: Chrome context menu installation and click handlers for Gasoline actions.
 * Split from event-listeners.ts to keep files under 800 LOC.
 */

import { StorageKey } from '../lib/constants.js'
import type { ScreenRecordingHandlers, RecordingShortcutHandlers } from './keyboard-shortcuts.js'
import { toggleScreenRecording, buildActionSequenceRecordingName } from './keyboard-shortcuts.js'

// =============================================================================
// CONTEXT MENU IDS
// =============================================================================

const MENU_ID_CONTROL = 'gasoline-control-page'
const MENU_ID_SCREENSHOT = 'gasoline-screenshot'
const MENU_ID_ANNOTATE = 'gasoline-annotate-page'
const MENU_ID_RECORD = 'gasoline-record-screen'
const MENU_ID_ACTION_RECORD = 'gasoline-action-record'

// =============================================================================
// CONTEXT MENU INSTALLATION
// =============================================================================

/**
 * Create context menu items for Gasoline actions.
 * Chrome auto-groups multiple items under a parent with the extension icon.
 */
export function installContextMenus(
  recordingHandlers: ScreenRecordingHandlers,
  actionRecordingHandlers: RecordingShortcutHandlers,
  logFn?: (message: string) => void
): void {
  if (typeof chrome === 'undefined' || !chrome.contextMenus) return

  chrome.contextMenus.removeAll(() => {
    const ctx: ['page'] = ['page']
    chrome.contextMenus.create({ id: MENU_ID_CONTROL, title: 'Control Page', contexts: ctx })
    chrome.contextMenus.create({ id: MENU_ID_SCREENSHOT, title: 'Take Screenshot', contexts: ctx })
    chrome.contextMenus.create({ id: MENU_ID_ANNOTATE, title: 'Annotate Page', contexts: ctx })
    chrome.contextMenus.create({ id: MENU_ID_RECORD, title: 'Record Screen', contexts: ctx })
    chrome.contextMenus.create({ id: MENU_ID_ACTION_RECORD, title: 'Record User Actions', contexts: ctx })
  })

  chrome.contextMenus.onClicked.addListener(async (info, tab) => {
    if (!tab?.id) return

    if (info.menuItemId === MENU_ID_CONTROL) {
      try {
        await chrome.storage.local.set({
          [StorageKey.TRACKED_TAB_ID]: tab.id,
          [StorageKey.TRACKED_TAB_URL]: tab.url ?? '',
          [StorageKey.TRACKED_TAB_TITLE]: tab.title ?? ''
        })
        if (logFn) logFn(`Now controlling tab ${tab.id}: ${tab.url}`)
      } catch (err) {
        if (logFn) logFn(`Control page error: ${(err as Error).message}`)
      }
    } else if (info.menuItemId === MENU_ID_SCREENSHOT) {
      try {
        chrome.tabs.sendMessage(tab.id, { type: 'captureScreenshot' })
      } catch {
        if (logFn) logFn('Cannot reach content script for screenshot via context menu')
      }
    } else if (info.menuItemId === MENU_ID_RECORD) {
      try {
        await toggleScreenRecording(recordingHandlers, tab, logFn)
      } catch (err) {
        if (logFn) logFn(`Context menu recording error: ${(err as Error).message}`)
      }
    } else if (info.menuItemId === MENU_ID_ACTION_RECORD) {
      try {
        if (actionRecordingHandlers.isRecording()) {
          await actionRecordingHandlers.stopRecording(false)
        } else {
          const name = buildActionSequenceRecordingName()
          await actionRecordingHandlers.startRecording(name, 15, '', '', true, tab.id)
        }
      } catch (err) {
        if (logFn) logFn(`Context menu action recording error: ${(err as Error).message}`)
      }
    } else if (info.menuItemId === MENU_ID_ANNOTATE) {
      try {
        const result = (await chrome.tabs.sendMessage(tab.id, {
          type: 'GASOLINE_GET_ANNOTATIONS'
        })) as { draw_mode_active?: boolean }

        if (result?.draw_mode_active) {
          await chrome.tabs.sendMessage(tab.id, { type: 'GASOLINE_DRAW_MODE_STOP' })
        } else {
          await chrome.tabs.sendMessage(tab.id, {
            type: 'GASOLINE_DRAW_MODE_START',
            started_by: 'user'
          })
        }
      } catch {
        try {
          await chrome.tabs.sendMessage(tab.id, {
            type: 'GASOLINE_DRAW_MODE_START',
            started_by: 'user'
          })
        } catch {
          if (logFn) logFn('Cannot reach content script for annotation via context menu')
        }
      }
    }
  })
}
