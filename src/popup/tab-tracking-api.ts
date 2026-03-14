/**
 * Purpose: Chrome API and storage operations for tab tracking — track/untrack lifecycle, tab switching.
 * Why: Separates browser API side-effects from DOM UI state rendering in tab-tracking.
 * Docs: docs/features/feature/tab-tracking-ux/index.md
 */

import { isInternalUrl } from './ui-utils.js'
import { StorageKey } from '../lib/constants.js'
import { getLocal, setLocals, removeLocals } from '../lib/storage-utils.js'
import { isDomainCloaked } from '../lib/cloaked-domains.js'

export type ShowStateFn = (btn: HTMLButtonElement) => void
export type ShowTrackingStateFn = (btn: HTMLButtonElement, url: string | undefined, tabId: number | undefined) => void

/**
 * Handle stop tracking from the compact tracking bar stop button.
 */
export async function handleStopTracking(showIdleState: ShowStateFn): Promise<void> {
  const prevTabId = await getLocal(StorageKey.TRACKED_TAB_ID) as number | undefined
  if (!prevTabId) return

  await removeLocals([StorageKey.TRACKED_TAB_ID, StorageKey.TRACKED_TAB_URL])
  const btn = document.getElementById('track-page-btn') as HTMLButtonElement | null
  if (btn) showIdleState(btn)

  // Stop recording if active
  chrome.runtime.sendMessage({ type: 'screen_recording_stop' }, () => {
    if (chrome.runtime.lastError) {
      /* no recording active — expected */
    }
  })
  // Notify content script so favicon restores without reload
  chrome.tabs
    .sendMessage(prevTabId, {
      type: 'tracking_state_changed',
      state: { isTracked: false, aiPilotEnabled: false }
    })
    .catch(() => {
      /* tab may be closed */
    })
  console.log('[Gasoline] Stopped tracking via bar stop button')
}

/**
 * Handle clicking on the tracked URL.
 * Switches to the tracked tab.
 */
export async function handleUrlClick(tabId: number | undefined): Promise<void> {
  if (!tabId) return

  try {
    // Switch to the tracked tab and bring its window to focus
    await chrome.tabs.update(tabId, { active: true })
    const tab = await chrome.tabs.get(tabId)
    if (tab.windowId) {
      await chrome.windows.update(tab.windowId, { focused: true })
    }
    console.log('[Gasoline] Switched to tracked tab:', tabId)
  } catch (err) {
    console.error('[Gasoline] Failed to switch to tracked tab:', err)
    // Tab might have been closed - clear tracking
    void removeLocals([StorageKey.TRACKED_TAB_ID, StorageKey.TRACKED_TAB_URL])
  }
}

/**
 * Handle Track This Tab button click.
 * Toggles tracking on/off for the current tab.
 * Blocks tracking on internal Chrome pages.
 */
// #lizard forgives
export async function handleTrackPageClick(
  showInternalPageState: ShowStateFn,
  showCloakedState: ShowStateFn,
  showTrackingState: ShowTrackingStateFn,
  showIdleState: ShowStateFn
): Promise<void> {
  const btn = document.getElementById('track-page-btn') as HTMLButtonElement | null

  // Check if we're currently tracking
  const trackedTabId = await getLocal(StorageKey.TRACKED_TAB_ID) as number | undefined
  if (trackedTabId) {
    // Untrack — delegate to the shared stop handler
    await handleStopTracking(showIdleState)
    return
  }

  // Track current tab
  const [tab] = await chrome.tabs.query({ active: true, currentWindow: true })
  if (!tab) return

  // Block tracking on internal Chrome pages
  if (isInternalUrl(tab.url)) {
    if (btn) showInternalPageState(btn)
    return
  }

  // Block tracking on cloaked domains
  let hostname = ''
  try { hostname = tab.url ? new URL(tab.url).hostname : '' } catch { /* malformed URL */ }
  if (await isDomainCloaked(hostname)) {
    if (btn) showCloakedState(btn)
    return
  }

  await setLocals({
    [StorageKey.TRACKED_TAB_ID]: tab.id,
    [StorageKey.TRACKED_TAB_URL]: tab.url,
    [StorageKey.TRACKED_TAB_TITLE]: tab.title || ''
  })
  if (btn) showTrackingState(btn, tab.url, tab.id)

  console.log('[Gasoline] Now tracking tab:', tab.id, tab.url)
  // Only reload if content script is not already injected
  if (tab.id) {
    const tabId = tab.id
    chrome.tabs.sendMessage(tabId, { type: 'gasoline_ping' }, (response) => {
      if (chrome.runtime.lastError || !response?.status) {
        // Content script not loaded — reload to inject it
        console.log('[Gasoline] Content script not found, reloading tab', tabId)
        chrome.tabs.reload(tabId)
      } else {
        // Content script already running — notify it of tracking change
        console.log('[Gasoline] Content script already loaded, skipping reload')
        chrome.tabs.sendMessage(tabId, {
          type: 'tracking_state_changed',
          state: { isTracked: true, aiPilotEnabled: false }
        })
      }
    })
  }
}
