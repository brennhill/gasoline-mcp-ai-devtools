/**
 * Purpose: Tab-state accessors, settings persistence, and content-script helpers.
 * Split from event-listeners.ts to keep files under 800 LOC.
 */

import { scaleTimeout } from '../lib/timeouts.js'
import { delay } from '../lib/timeout-utils.js'
import { StorageKey } from '../lib/constants.js'

// =============================================================================
// CONTENT SCRIPT HELPERS
// =============================================================================

/**
 * Ping content script to check if it's loaded
 */
export async function pingContentScript(tabId: number, timeoutMs = scaleTimeout(500)): Promise<boolean> {
  try {
    const response = (await Promise.race([
      chrome.tabs.sendMessage(tabId, { type: 'GASOLINE_PING' }),
      new Promise<never>((_, reject) => {
        setTimeout(
          () => reject(new Error(`Content script ping timeout after ${timeoutMs}ms on tab ${tabId}`)),
          timeoutMs
        )
      })
    ])) as { status?: string }
    return response?.status === 'alive'
  } catch {
    return false
  }
}

/**
 * Wait for tab to finish loading
 */
export async function waitForTabLoad(tabId: number, timeoutMs = scaleTimeout(5000)): Promise<boolean> {
  const startTime = Date.now()
  while (Date.now() - startTime < timeoutMs) {
    try {
      const tab = await chrome.tabs.get(tabId)
      if (tab.status === 'complete') return true
    } catch {
      return false
    }
    await delay(scaleTimeout(100))
  }
  return false
}

/**
 * Forward a message to all content scripts
 */
export async function forwardToAllContentScripts(
  message: { type: string; [key: string]: unknown },
  debugLogFn?: (category: string, message: string, data?: unknown) => void
): Promise<void> {
  if (typeof chrome === 'undefined' || !chrome.tabs) return

  const tabs = await chrome.tabs.query({})
  for (const tab of tabs) {
    if (tab.id) {
      chrome.tabs.sendMessage(tab.id, message).catch((err: Error) => {
        if (
          !err.message?.includes('Receiving end does not exist') &&
          !err.message?.includes('Could not establish connection')
        ) {
          if (debugLogFn) {
            debugLogFn('error', 'Unexpected error forwarding setting to tab', {
              tabId: tab.id,
              error: err.message
            })
          }
        }
      })
    }
  }
}

// =============================================================================
// SETTINGS LOADING
// =============================================================================

/** Settings returned by loadSavedSettings */
export interface SavedSettings {
  serverUrl?: string
  logLevel?: string
  screenshotOnError?: boolean
  sourceMapEnabled?: boolean
  debugMode?: boolean
}

/**
 * Load saved settings from chrome.storage.local
 */
export async function loadSavedSettings(): Promise<SavedSettings> {
  if (typeof chrome === 'undefined' || !chrome.storage) {
    return {}
  }

  try {
    const result = (await chrome.storage.local.get([
      StorageKey.SERVER_URL,
      StorageKey.LOG_LEVEL,
      StorageKey.SCREENSHOT_ON_ERROR,
      StorageKey.SOURCE_MAP_ENABLED,
      StorageKey.DEBUG_MODE
    ])) as SavedSettings
    return result
  } catch {
    console.warn('[Gasoline] Could not load saved settings - using defaults')
    return {}
  }
}

/**
 * Load AI Web Pilot enabled state from storage
 */
export async function loadAiWebPilotState(logFn?: (message: string) => void): Promise<boolean> {
  if (typeof chrome === 'undefined' || !chrome.storage) {
    return false
  }

  const startTime = performance.now()
  const result = (await chrome.storage.local.get([StorageKey.AI_WEB_PILOT_ENABLED])) as { aiWebPilotEnabled?: boolean }
  const wasLoaded = result.aiWebPilotEnabled !== false
  const loadTime = performance.now() - startTime
  if (logFn) {
    logFn(`[Gasoline] AI Web Pilot loaded on startup: ${wasLoaded} (took ${loadTime.toFixed(1)}ms)`)
  }
  return wasLoaded
}

/**
 * Load debug mode state from storage
 */
export async function loadDebugModeState(): Promise<boolean> {
  if (typeof chrome === 'undefined' || !chrome.storage) {
    return false
  }

  const result = (await chrome.storage.local.get([StorageKey.DEBUG_MODE])) as { debugMode?: boolean }
  return result.debugMode === true
}

/**
 * Save setting to chrome.storage.local
 */
export function saveSetting(key: string, value: unknown): void {
  if (typeof chrome === 'undefined' || !chrome.storage) return
  chrome.storage.local.set({ [key]: value })
}

// =============================================================================
// TRACKED TAB STATE
// =============================================================================

/** Tracked tab info type */
export interface TrackedTabInfo {
  trackedTabId: number | null
  trackedTabUrl: string | null
  trackedTabTitle: string | null
  tabStatus: 'loading' | 'complete' | null
  trackedTabActive: boolean | null
}

/**
 * Get tracked tab information, including Chrome tab status.
 */
export async function getTrackedTabInfo(): Promise<TrackedTabInfo> {
  if (typeof chrome === 'undefined' || !chrome.storage) {
    return { trackedTabId: null, trackedTabUrl: null, trackedTabTitle: null, tabStatus: null, trackedTabActive: null }
  }

  const result = (await chrome.storage.local.get([
    StorageKey.TRACKED_TAB_ID,
    StorageKey.TRACKED_TAB_URL,
    StorageKey.TRACKED_TAB_TITLE
  ])) as { trackedTabId?: number; trackedTabUrl?: string; trackedTabTitle?: string }

  const tabId = result.trackedTabId || null
  let tabStatus: 'loading' | 'complete' | null = null
  let trackedTabActive: boolean | null = null

  // Query Chrome tab API for live tab status if we have a tracked tab
  if (tabId && typeof chrome !== 'undefined' && chrome.tabs) {
    try {
      const tab = await chrome.tabs.get(tabId)
      if (tab.status === 'loading' || tab.status === 'complete') {
        tabStatus = tab.status
      }
      trackedTabActive = !!tab.active
    } catch {
      // Tab may have been closed -- tabStatus stays null
    }
  }

  return {
    trackedTabId: tabId,
    trackedTabUrl: result.trackedTabUrl || null,
    trackedTabTitle: result.trackedTabTitle || null,
    tabStatus,
    trackedTabActive
  }
}

/**
 * Clear tracked tab state
 */
export function clearTrackedTab(): void {
  if (typeof chrome === 'undefined' || !chrome.storage) return
  chrome.storage.local.remove([StorageKey.TRACKED_TAB_ID, StorageKey.TRACKED_TAB_URL, StorageKey.TRACKED_TAB_TITLE])
}

/**
 * Get all extension config settings.
 */
export async function getAllConfigSettings(): Promise<Record<string, boolean | string | undefined>> {
  if (typeof chrome === 'undefined' || !chrome.storage) {
    return {}
  }

  const result = (await chrome.storage.local.get([
    StorageKey.AI_WEB_PILOT_ENABLED,
    StorageKey.WEBSOCKET_CAPTURE_ENABLED,
    StorageKey.NETWORK_WATERFALL_ENABLED,
    StorageKey.PERFORMANCE_MARKS_ENABLED,
    StorageKey.ACTION_REPLAY_ENABLED,
    StorageKey.SCREENSHOT_ON_ERROR,
    StorageKey.SOURCE_MAP_ENABLED,
    StorageKey.NETWORK_BODY_CAPTURE_ENABLED
  ])) as Record<string, boolean | string | undefined>

  return result
}

// =============================================================================
// ACTIVE TAB LOOKUP
// =============================================================================

/**
 * Query for the currently active tab in the current window.
 * Returns null if no active tab or no tab id.
 */
export async function getActiveTab(): Promise<chrome.tabs.Tab | null> {
  const activeTabs = await chrome.tabs.query({ active: true, currentWindow: true })
  const tab = activeTabs[0]
  if (!tab?.id) {
    return null
  }
  return tab
}

// =============================================================================
// TAB TOAST
// =============================================================================

/**
 * Send a GASOLINE_ACTION_TOAST message to a tab.
 * Silently ignores errors (content script may not be loaded).
 */
export function sendTabToast(
  tabId: number,
  text: string,
  detail = '',
  state: 'trying' | 'success' | 'warning' | 'error' | 'audio' = 'success',
  duration_ms = 3000
): void {
  chrome.tabs
    .sendMessage(tabId, {
      type: 'GASOLINE_ACTION_TOAST' as const,
      text,
      detail,
      state,
      duration_ms
    })
    .catch(() => {
      /* content script may not be loaded */
    })
}
