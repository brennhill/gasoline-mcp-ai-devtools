/**
 * @fileoverview Tab Tracking Module for Popup
 * Manages the "Track This Tab" button and tracking status
 */
import { isInternalUrl } from './ui-utils.js'
/**
 * Initialize the Track This Tab button.
 * Shows current tracking status and handles track/untrack.
 * Disables the button on internal Chrome pages where tracking is impossible.
 */
export async function initTrackPageButton() {
  const btn = document.getElementById('track-page-btn')
  const info = document.getElementById('tracked-page-info')
  const urlEl = document.getElementById('tracked-url')
  if (!btn) return
  return new Promise((resolve) => {
    chrome.storage.local.get(['trackedTabId', 'trackedTabUrl'], async (result) => {
      // Check if current tab is an internal page
      chrome.tabs.query({ active: true, currentWindow: true }, (tabs) => {
        const currentTab = tabs && tabs[0]
        const currentUrl = currentTab?.url
        if (isInternalUrl(currentUrl)) {
          // Disable button on internal pages
          btn.disabled = true
          btn.textContent = 'Cannot Track Internal Pages'
          btn.title = 'Chrome blocks extensions on internal pages like chrome:// and about:'
          btn.style.opacity = '0.5'
          btn.style.background = '#252525'
          btn.style.color = '#888'
          btn.style.borderColor = '#333'
          if (info) {
            info.style.display = 'block'
            info.textContent = 'Internal browser pages cannot be tracked'
          }
          resolve()
          return
        }
        // Update UI based on whether we're tracking a tab
        if (result.trackedTabId) {
          // Show tracking info
          btn.textContent = 'Stop Tracking'
          btn.style.background = '#f85149'
          btn.style.color = 'white'
          btn.style.borderColor = '#f85149'
          if (info) info.style.display = 'block'
          if (urlEl && result.trackedTabUrl) {
            urlEl.textContent = result.trackedTabUrl
            // Make URL clickable - switch to tracked tab on click
            urlEl.style.cursor = 'pointer'
            urlEl.style.textDecoration = 'underline'
            urlEl.title = 'Click to switch to this tab'
            urlEl.addEventListener('click', () => handleUrlClick(result.trackedTabId))
          }
        } else {
          // Show track button - renamed from "Track This Page" to "Track This Tab"
          btn.textContent = 'Track This Tab'
          btn.style.background = '#252525'
          btn.style.color = '#58a6ff'
          btn.style.borderColor = '#58a6ff'
          if (info) {
            info.style.display = 'block'
          }
          // Show "no tracking" status
          const noTrackEl = document.getElementById('no-tracking-warning')
          if (noTrackEl) {
            noTrackEl.style.display = 'block'
          }
        }
        // Set up click handler
        btn.addEventListener('click', handleTrackPageClick)
        resolve()
      })
    })
  })
}
/**
 * Handle clicking on the tracked URL.
 * Switches to the tracked tab.
 */
export async function handleUrlClick(tabId) {
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
    chrome.storage.local.remove(['trackedTabId', 'trackedTabUrl'])
  }
}
/**
 * Handle Track This Tab button click.
 * Toggles tracking on/off for the current tab.
 * Blocks tracking on internal Chrome pages.
 */
export async function handleTrackPageClick() {
  const btn = document.getElementById('track-page-btn')
  const info = document.getElementById('tracked-page-info')
  const urlEl = document.getElementById('tracked-url')
  // Check if we're currently tracking
  chrome.storage.local.get(['trackedTabId'], async (result) => {
    if (result.trackedTabId) {
      // Untrack
      chrome.storage.local.remove(['trackedTabId', 'trackedTabUrl'], () => {
        if (btn) {
          btn.textContent = 'Track This Tab'
          btn.style.background = '#252525'
          btn.style.color = '#58a6ff'
          btn.style.borderColor = '#58a6ff'
        }
        if (info) info.style.display = 'none'
        // Show "no tracking" warning
        const noTrackEl = document.getElementById('no-tracking-warning')
        if (noTrackEl) noTrackEl.style.display = 'block'
        console.log('[Gasoline] Stopped tracking')
      })
    } else {
      // Track current tab
      chrome.tabs.query({ active: true, currentWindow: true }, (tabs) => {
        if (tabs[0]) {
          const tab = tabs[0]
          // Block tracking on internal Chrome pages
          if (isInternalUrl(tab.url)) {
            if (btn) {
              btn.disabled = true
              btn.textContent = 'Cannot Track Internal Pages'
              btn.style.opacity = '0.5'
            }
            return
          }
          chrome.storage.local.set({ trackedTabId: tab.id, trackedTabUrl: tab.url }, () => {
            if (btn) {
              btn.textContent = 'Stop Tracking'
              btn.style.background = '#f85149'
              btn.style.color = 'white'
              btn.style.borderColor = '#f85149'
            }
            if (info) info.style.display = 'block'
            if (urlEl) {
              urlEl.textContent = tab.url || ''
              // Make URL clickable
              urlEl.style.cursor = 'pointer'
              urlEl.style.textDecoration = 'underline'
              urlEl.title = 'Click to switch to this tab'
              urlEl.addEventListener('click', () => handleUrlClick(tab.id))
            }
            // Hide "no tracking" warning
            const noTrackEl = document.getElementById('no-tracking-warning')
            if (noTrackEl) noTrackEl.style.display = 'none'
            console.log('[Gasoline] Now tracking tab:', tab.id, tab.url)
          })
        }
      })
    }
  })
}
//# sourceMappingURL=tab-tracking.js.map
