/**
 * Purpose: Manages popup tab-tracking UI state and track/untrack transitions for the active browser tab.
 * Why: Keeps the tracked-tab lifecycle explicit so content-script injection and status UX stay synchronized.
 * Docs: docs/features/feature/tab-tracking-ux/index.md
 */

/**
 * @fileoverview Tab Tracking Module for Popup
 * Manages the "Track This Tab" button and tracking status
 */

import { isInternalUrl } from './ui-utils.js'
import { StorageKey } from '../lib/constants.js'
import { getLocal, getLocals, setLocals, removeLocals, onStorageChanged } from '../lib/storage-utils.js' // async API only

let trackingStorageSyncInstalled = false

/**
 * Initialize the Track This Tab button.
 * Shows current tracking status and handles track/untrack.
 * Disables the button on internal Chrome pages where tracking is impossible.
 */
function showInternalPageState(btn: HTMLButtonElement): void {
  const trackingBar = document.getElementById('tracking-bar')
  if (trackingBar) trackingBar.style.display = 'none'
  btn.disabled = true
  btn.textContent = 'Cannot Track Internal Pages'
  btn.title = 'Chrome blocks extensions on internal pages like chrome:// and about:'
  Object.assign(btn.style, { opacity: '0.5', background: '#252525', color: '#888', borderColor: '#333' })
}

function showTrackingState(
  btn: HTMLButtonElement,
  trackedTabUrl: string | undefined,
  trackedTabId: number | undefined
): void {
  // Hide the hero button area
  const heroEl = document.getElementById('track-hero')
  if (heroEl) heroEl.style.display = 'none'
  const noTrackEl = document.getElementById('no-tracking-warning')
  if (noTrackEl) noTrackEl.style.display = 'none'

  // Show the compact tracking bar
  const trackingBar = document.getElementById('tracking-bar')
  const trackingBarUrl = document.getElementById('tracking-bar-url')
  const trackingBarStop = document.getElementById('tracking-bar-stop')

  if (trackingBar) trackingBar.style.display = 'flex'
  if (trackingBarUrl && trackedTabUrl) {
    trackingBarUrl.textContent = trackedTabUrl
    trackingBarUrl.onclick = () => {
      void handleUrlClick(trackedTabId)
    }
  }
  if (trackingBarStop) {
    trackingBarStop.onclick = (e: Event) => {
      e.stopPropagation()
      handleStopTracking()
    }
  }
}

function showIdleState(btn: HTMLButtonElement): void {
  // Show the hero button area
  const heroEl = document.getElementById('track-hero')
  if (heroEl) heroEl.style.display = ''
  btn.textContent = 'Track This Tab'
  Object.assign(btn.style, {
    background: '#1a3a5c',
    color: '#58a6ff',
    borderColor: '#58a6ff',
    fontSize: '16px',
    fontWeight: '600',
    padding: '14px 16px',
    borderWidth: '2px'
  })
  const heroDesc = document.getElementById('track-hero-desc')
  if (heroDesc) heroDesc.style.display = ''

  // Hide the tracking bar
  const trackingBar = document.getElementById('tracking-bar')
  if (trackingBar) trackingBar.style.display = 'none'

  // Show "no tracking" warning
  const noTrackEl = document.getElementById('no-tracking-warning')
  if (noTrackEl) noTrackEl.style.display = 'block'
}

function syncTrackButtonState(btn: HTMLButtonElement): void {
  void getLocals([StorageKey.TRACKED_TAB_ID, StorageKey.TRACKED_TAB_URL]).then(
    (result: Record<string, unknown>) => {
      const trackedTabId = result[StorageKey.TRACKED_TAB_ID] as number | undefined
      const trackedTabUrl = result[StorageKey.TRACKED_TAB_URL] as string | undefined
      chrome.tabs.query({ active: true, currentWindow: true }, (tabs: chrome.tabs.Tab[]) => {
        const currentUrl = tabs?.[0]?.url

        if (trackedTabId) {
          showTrackingState(btn, trackedTabUrl, trackedTabId)
        } else if (isInternalUrl(currentUrl)) {
          showInternalPageState(btn)
        } else {
          showIdleState(btn)
        }
      })
    }
  )
}

function installTrackingStorageSync(btn: HTMLButtonElement): void {
  if (trackingStorageSyncInstalled) return
  trackingStorageSyncInstalled = true

  onStorageChanged((changes, areaName) => {
    if (areaName !== 'local') return
    if (!changes[StorageKey.TRACKED_TAB_ID] && !changes[StorageKey.TRACKED_TAB_URL]) return
    syncTrackButtonState(btn)
  })
}

/**
 * Handle stop tracking from the compact tracking bar stop button.
 */
async function handleStopTracking(): Promise<void> {
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

export function initTrackPageButton(): void {
  const btn = document.getElementById('track-page-btn') as HTMLButtonElement | null
  if (!btn) return

  syncTrackButtonState(btn)
  installTrackingStorageSync(btn)
  btn.addEventListener('click', handleTrackPageClick)
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
export async function handleTrackPageClick(): Promise<void> {
  const btn = document.getElementById('track-page-btn') as HTMLButtonElement | null

  // Check if we're currently tracking
  const trackedTabId = await getLocal(StorageKey.TRACKED_TAB_ID) as number | undefined
  if (trackedTabId) {
    // Untrack — delegate to the shared stop handler
    await handleStopTracking()
    return
  }

  // Track current tab
  const [tab] = await chrome.tabs.query({ active: true, currentWindow: true })
  if (!tab) return

  // Block tracking on internal Chrome pages
  if (isInternalUrl(tab.url)) {
    if (btn) {
      btn.disabled = true
      btn.textContent = 'Cannot Track Internal Pages'
      btn.style.opacity = '0.5'
    }
    return
  }

  await setLocals({
    trackedTabId: tab.id,
    trackedTabUrl: tab.url,
    trackedTabTitle: tab.title || ''
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
