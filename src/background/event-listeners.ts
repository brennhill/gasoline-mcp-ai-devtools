/**
 * Purpose: Handles extension background coordination and message routing.
 * Docs: docs/features/feature/analyze-tool/index.md
 * Docs: docs/features/feature/interact-explore/index.md
 * Docs: docs/features/feature/observe/index.md
 */

/**
 * @fileoverview Event Listeners - Handles Chrome alarms, tab listeners,
 * storage change listeners, and other Chrome extension events.
 */

import type { StorageChange } from '../types'
import { scaleTimeout } from '../lib/timeouts'
import { StorageKey } from '../lib/constants'

// =============================================================================
// CONSTANTS - Rate Limiting & DoS Protection
// =============================================================================

/**
 * Reconnect interval: 5 seconds
 * DoS Protection: If MCP server is down, we check every 5s (circuit breaker
 * will back off exponentially if failures continue).
 * Ensures connection restored quickly when server comes back up.
 */
const RECONNECT_INTERVAL_MINUTES = 5 / 60 // 5 seconds in minutes

/**
 * Error group flush interval: 30 seconds
 * DoS Protection: Deduplicates identical errors within a 5-second window
 * before sending to server. Reduces network traffic and API quota usage.
 * Flushed every 30 seconds to keep errors reasonably fresh.
 */
const ERROR_GROUP_FLUSH_INTERVAL_MINUTES = 0.5 // 30 seconds

/**
 * Memory check interval: 30 seconds
 * DoS Protection: Monitors estimated buffer memory and triggers circuit breaker
 * if soft limit (20MB) or hard limit (50MB) is exceeded.
 * Prevents memory exhaustion from unbounded capture buffer growth.
 */
const MEMORY_CHECK_INTERVAL_MINUTES = 0.5 // 30 seconds

/**
 * Error group cleanup interval: 10 minutes
 * DoS Protection: Removes stale error group deduplication state that is >5min old.
 * Prevents unbounded growth of error group metadata.
 */
const ERROR_GROUP_CLEANUP_INTERVAL_MINUTES = 10

// =============================================================================
// ALARM NAMES
// =============================================================================

export const ALARM_NAMES = {
  RECONNECT: 'reconnect',
  ERROR_GROUP_FLUSH: 'errorGroupFlush',
  MEMORY_CHECK: 'memoryCheck',
  ERROR_GROUP_CLEANUP: 'errorGroupCleanup'
} as const

export type AlarmName = (typeof ALARM_NAMES)[keyof typeof ALARM_NAMES]

// =============================================================================
// CHROME ALARMS
// =============================================================================

/**
 * Setup Chrome alarms for periodic tasks
 *
 * RATE LIMITING & DoS PROTECTION:
 * 1. RECONNECT (5s): Maintains MCP connection with exponential backoff
 * 2. ERROR_GROUP_FLUSH (30s): Deduplicates errors, reduces server load
 * 3. MEMORY_CHECK (30s): Monitors buffer memory, prevents exhaustion
 * 4. ERROR_GROUP_CLEANUP (10min): Removes stale deduplication state
 *
 * Note: Alarms are re-created on service worker startup (not persistent)
 * If service worker restarts, alarms must be recreated by this function
 */
export function setupChromeAlarms(): void {
  if (typeof chrome === 'undefined' || !chrome.alarms) return

  chrome.alarms.create(ALARM_NAMES.RECONNECT, { periodInMinutes: RECONNECT_INTERVAL_MINUTES })
  chrome.alarms.create(ALARM_NAMES.ERROR_GROUP_FLUSH, { periodInMinutes: ERROR_GROUP_FLUSH_INTERVAL_MINUTES })
  chrome.alarms.create(ALARM_NAMES.MEMORY_CHECK, { periodInMinutes: MEMORY_CHECK_INTERVAL_MINUTES })
  chrome.alarms.create(ALARM_NAMES.ERROR_GROUP_CLEANUP, { periodInMinutes: ERROR_GROUP_CLEANUP_INTERVAL_MINUTES })
}

/**
 * Install Chrome alarm listener.
 * Handlers may be async — the listener awaits them to keep the SW alive
 * until the work completes (prevents badge updates from being lost).
 */
export function installAlarmListener(handlers: {
  onReconnect: () => void | Promise<void>
  onErrorGroupFlush: () => void
  onMemoryCheck: () => void
  onErrorGroupCleanup: () => void
}): void {
  if (typeof chrome === 'undefined' || !chrome.alarms) return

  chrome.alarms.onAlarm.addListener(async (alarm) => {
    switch (alarm.name) {
      case ALARM_NAMES.RECONNECT:
        await handlers.onReconnect()
        break
      case ALARM_NAMES.ERROR_GROUP_FLUSH:
        handlers.onErrorGroupFlush()
        break
      case ALARM_NAMES.MEMORY_CHECK:
        handlers.onMemoryCheck()
        break
      case ALARM_NAMES.ERROR_GROUP_CLEANUP:
        handlers.onErrorGroupCleanup()
        break
    }
  })
}

// =============================================================================
// TAB LISTENERS
// =============================================================================

/**
 * Install tab removed listener
 */
export function installTabRemovedListener(onTabRemoved: (tabId: number) => void): void {
  if (typeof chrome === 'undefined' || !chrome.tabs || !chrome.tabs.onRemoved) return

  chrome.tabs.onRemoved.addListener((tabId) => {
    onTabRemoved(tabId)
  })
}

/**
 * Install tab updated listener to track URL changes
 */
export function installTabUpdatedListener(onTabUpdated: (tabId: number, newUrl: string) => void): void {
  if (typeof chrome === 'undefined' || !chrome.tabs || !chrome.tabs.onUpdated) return

  chrome.tabs.onUpdated.addListener((tabId, changeInfo) => {
    // Only care about URL changes
    if (changeInfo.url) {
      onTabUpdated(tabId, changeInfo.url)
    }
  })
}

/**
 * Handle tracked tab URL change
 * Updates the stored URL and title when the tracked tab navigates
 */
export async function handleTrackedTabUrlChange(
  updatedTabId: number,
  newUrl: string,
  logFn?: (message: string) => void
): Promise<void> {
  if (typeof chrome === 'undefined' || !chrome.storage) return

  const result = (await chrome.storage.local.get([StorageKey.TRACKED_TAB_ID])) as { trackedTabId?: number }
  if (result.trackedTabId === updatedTabId) {
    // Update URL immediately, then refresh title from the tab
    try {
      const tab = await chrome.tabs.get(updatedTabId)
      const updates: Record<string, string> = { [StorageKey.TRACKED_TAB_URL]: newUrl }
      if (tab?.title) updates[StorageKey.TRACKED_TAB_TITLE] = tab.title
      await chrome.storage.local.set(updates)
      if (logFn) {
        logFn('[Gasoline] Tracked tab updated: ' + newUrl)
      }
    } catch {
      // Tab may have been closed — update URL only
      chrome.storage.local.set({ [StorageKey.TRACKED_TAB_URL]: newUrl })
    }
  }
}

/**
 * Handle tracked tab being closed
 * SECURITY: Clears ephemeral tracking state when tab closes
 * Uses session storage for ephemeral tab tracking data
 */
export async function handleTrackedTabClosed(
  closedTabId: number,
  logFn?: (message: string, data?: unknown) => void
): Promise<void> {
  if (typeof chrome === 'undefined' || !chrome.storage) return

  const result = (await chrome.storage.local.get([StorageKey.TRACKED_TAB_ID])) as { trackedTabId?: number }
  if (result.trackedTabId === closedTabId) {
    if (logFn) logFn('[Gasoline] Tracked tab closed (id:', closedTabId)
    chrome.storage.local.remove([StorageKey.TRACKED_TAB_ID, StorageKey.TRACKED_TAB_URL, StorageKey.TRACKED_TAB_TITLE])
  }
}

// =============================================================================
// STORAGE LISTENERS
// =============================================================================

/**
 * Install storage change listener
 */
export function installStorageChangeListener(handlers: {
  onAiWebPilotChanged?: (newValue: boolean) => void
  onTrackedTabChanged?: (newTabId: number | null, oldTabId: number | null) => void
}): void {
  if (typeof chrome === 'undefined' || !chrome.storage) return

  chrome.storage.onChanged.addListener((changes: { [key: string]: StorageChange<unknown> }, areaName: string) => {
    if (areaName === 'local') {
      if (changes[StorageKey.AI_WEB_PILOT_ENABLED] && handlers.onAiWebPilotChanged) {
        handlers.onAiWebPilotChanged(changes[StorageKey.AI_WEB_PILOT_ENABLED]!.newValue === true)
      }
      if (changes[StorageKey.TRACKED_TAB_ID] && handlers.onTrackedTabChanged) {
        const newTabId = (changes[StorageKey.TRACKED_TAB_ID]!.newValue as number) ?? null
        const oldTabId = (changes[StorageKey.TRACKED_TAB_ID]!.oldValue as number) ?? null
        handlers.onTrackedTabChanged(newTabId, oldTabId)
      }
    }
  })
}

// =============================================================================
// RUNTIME LISTENERS
// =============================================================================

/**
 * Install browser startup listener (clears tracking state)
 */
export function installStartupListener(logFn?: (message: string) => void): void {
  if (typeof chrome === 'undefined' || !chrome.runtime || !chrome.runtime.onStartup) return

  chrome.runtime.onStartup.addListener(async () => {
    try {
      const result = await chrome.storage.local.get([StorageKey.TRACKED_TAB_ID])
      const trackedTabId = result[StorageKey.TRACKED_TAB_ID] as number | undefined
      if (trackedTabId) {
        try {
          await chrome.tabs.get(trackedTabId)
          if (logFn) logFn('[Gasoline] Browser restarted - tracked tab still exists, keeping tracking')
        } catch {
          if (logFn) logFn('[Gasoline] Browser restarted - tracked tab gone, clearing tracking state')
          chrome.storage.local.remove([StorageKey.TRACKED_TAB_ID, StorageKey.TRACKED_TAB_URL, StorageKey.TRACKED_TAB_TITLE])
        }
      }
    } catch {
      // Safety fallback: clear if we can't check
      chrome.storage.local.remove([StorageKey.TRACKED_TAB_ID, StorageKey.TRACKED_TAB_URL, StorageKey.TRACKED_TAB_TITLE])
    }
  })
}

// =============================================================================
// KEYBOARD SHORTCUT LISTENER
// =============================================================================

/**
 * Install keyboard shortcut listener for draw mode toggle (Ctrl+Shift+D / Cmd+Shift+D).
 * Sends GASOLINE_DRAW_MODE_START or GASOLINE_DRAW_MODE_STOP to the active tab's content script.
 */
export function installDrawModeCommandListener(logFn?: (message: string) => void): void {
  if (typeof chrome === 'undefined' || !chrome.commands) return

  chrome.commands.onCommand.addListener(async (command: string) => {
    if (command !== 'toggle_draw_mode') return

    try {
      const tabs = await chrome.tabs.query({ active: true, currentWindow: true })
      const tab = tabs[0]
      if (!tab?.id) return

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
        // Content script not loaded — try activating anyway
        try {
          await chrome.tabs.sendMessage(tab.id, {
            type: 'GASOLINE_DRAW_MODE_START',
            started_by: 'user'
          })
        } catch {
          if (logFn) logFn('Cannot reach content script for draw mode toggle')
          try {
            await chrome.tabs.sendMessage(tab.id, {
              type: 'GASOLINE_ACTION_TOAST',
              text: 'Draw mode unavailable',
              detail: 'Refresh the page and try again',
              state: 'error',
              duration_ms: 3000
            })
          } catch {
            // Tab truly unreachable
          }
        }
      }
    } catch (err) {
      if (logFn) logFn(`Draw mode keyboard shortcut error: ${(err as Error).message}`)
    }
  })
}

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
    await new Promise((r) => {
      setTimeout(r, scaleTimeout(100))
    })
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

/** Tracked tab info type */
export interface TrackedTabInfo {
  trackedTabId: number | null
  trackedTabUrl: string | null
  trackedTabTitle: string | null
}

/**
 * Get tracked tab information.
 */
export async function getTrackedTabInfo(): Promise<TrackedTabInfo> {
  if (typeof chrome === 'undefined' || !chrome.storage) {
    return { trackedTabId: null, trackedTabUrl: null, trackedTabTitle: null }
  }

  const result = (await chrome.storage.local.get([
    StorageKey.TRACKED_TAB_ID,
    StorageKey.TRACKED_TAB_URL,
    StorageKey.TRACKED_TAB_TITLE
  ])) as { trackedTabId?: number; trackedTabUrl?: string; trackedTabTitle?: string }

  return {
    trackedTabId: result.trackedTabId || null,
    trackedTabUrl: result.trackedTabUrl || null,
    trackedTabTitle: result.trackedTabTitle || null
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
