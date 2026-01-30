/**
 * @fileoverview Event Listeners - Handles Chrome alarms, tab listeners,
 * storage change listeners, and other Chrome extension events.
 */

import type { StorageChange } from '../types';

// =============================================================================
// CONSTANTS - Rate Limiting & DoS Protection
// =============================================================================

/**
 * Reconnect interval: 5 seconds
 * DoS Protection: If MCP server is down, we check every 5s (circuit breaker
 * will back off exponentially if failures continue).
 * Ensures connection restored quickly when server comes back up.
 */
const RECONNECT_INTERVAL_MINUTES = 5 / 60; // 5 seconds in minutes

/**
 * Error group flush interval: 30 seconds
 * DoS Protection: Deduplicates identical errors within a 5-second window
 * before sending to server. Reduces network traffic and API quota usage.
 * Flushed every 30 seconds to keep errors reasonably fresh.
 */
const ERROR_GROUP_FLUSH_INTERVAL_MINUTES = 0.5; // 30 seconds

/**
 * Memory check interval: 30 seconds
 * DoS Protection: Monitors estimated buffer memory and triggers circuit breaker
 * if soft limit (20MB) or hard limit (50MB) is exceeded.
 * Prevents memory exhaustion from unbounded capture buffer growth.
 */
const MEMORY_CHECK_INTERVAL_MINUTES = 0.5; // 30 seconds

/**
 * Error group cleanup interval: 10 minutes
 * DoS Protection: Removes stale error group deduplication state that is >5min old.
 * Prevents unbounded growth of error group metadata.
 */
const ERROR_GROUP_CLEANUP_INTERVAL_MINUTES = 10;

// =============================================================================
// ALARM NAMES
// =============================================================================

export const ALARM_NAMES = {
  RECONNECT: 'reconnect',
  ERROR_GROUP_FLUSH: 'errorGroupFlush',
  MEMORY_CHECK: 'memoryCheck',
  ERROR_GROUP_CLEANUP: 'errorGroupCleanup',
} as const;

export type AlarmName = (typeof ALARM_NAMES)[keyof typeof ALARM_NAMES];

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
  if (typeof chrome === 'undefined' || !chrome.alarms) return;

  chrome.alarms.create(ALARM_NAMES.RECONNECT, { periodInMinutes: RECONNECT_INTERVAL_MINUTES });
  chrome.alarms.create(ALARM_NAMES.ERROR_GROUP_FLUSH, { periodInMinutes: ERROR_GROUP_FLUSH_INTERVAL_MINUTES });
  chrome.alarms.create(ALARM_NAMES.MEMORY_CHECK, { periodInMinutes: MEMORY_CHECK_INTERVAL_MINUTES });
  chrome.alarms.create(ALARM_NAMES.ERROR_GROUP_CLEANUP, { periodInMinutes: ERROR_GROUP_CLEANUP_INTERVAL_MINUTES });
}

/**
 * Install Chrome alarm listener
 */
export function installAlarmListener(handlers: {
  onReconnect: () => void;
  onErrorGroupFlush: () => void;
  onMemoryCheck: () => void;
  onErrorGroupCleanup: () => void;
}): void {
  if (typeof chrome === 'undefined' || !chrome.alarms) return;

  chrome.alarms.onAlarm.addListener((alarm) => {
    switch (alarm.name) {
      case ALARM_NAMES.RECONNECT:
        handlers.onReconnect();
        break;
      case ALARM_NAMES.ERROR_GROUP_FLUSH:
        handlers.onErrorGroupFlush();
        break;
      case ALARM_NAMES.MEMORY_CHECK:
        handlers.onMemoryCheck();
        break;
      case ALARM_NAMES.ERROR_GROUP_CLEANUP:
        handlers.onErrorGroupCleanup();
        break;
    }
  });
}

// =============================================================================
// TAB LISTENERS
// =============================================================================

/**
 * Install tab removed listener
 */
export function installTabRemovedListener(
  onTabRemoved: (tabId: number) => void
): void {
  if (typeof chrome === 'undefined' || !chrome.tabs || !chrome.tabs.onRemoved) return;

  chrome.tabs.onRemoved.addListener((tabId) => {
    onTabRemoved(tabId);
  });
}

/**
 * Handle tracked tab being closed
 * SECURITY: Clears ephemeral tracking state when tab closes
 * Uses session storage for ephemeral tab tracking data
 */
export function handleTrackedTabClosed(
  closedTabId: number,
  logFn?: (message: string, data?: unknown) => void
): void {
  if (typeof chrome === 'undefined' || !chrome.storage) return;

  // Check both session and local storage for backward compatibility
  const checkStorageArea = (area: 'local' | 'session'): void => {
    chrome.storage[area].get(['trackedTabId'], (result: { trackedTabId?: number }) => {
      if (result.trackedTabId === closedTabId) {
        if (logFn) logFn('[Gasoline] Tracked tab closed (id:', closedTabId);
        chrome.storage[area].remove(['trackedTabId', 'trackedTabUrl']);
      }
    });
  };

  // Check local storage (legacy)
  checkStorageArea('local');

  // Check session storage (modern)
  if ((chrome.storage as any).session) {
    checkStorageArea('session' as any);
  }
}

// =============================================================================
// STORAGE LISTENERS
// =============================================================================

/**
 * Install storage change listener
 */
export function installStorageChangeListener(handlers: {
  onAiWebPilotChanged?: (newValue: boolean) => void;
  onTrackedTabChanged?: () => void;
}): void {
  if (typeof chrome === 'undefined' || !chrome.storage) return;

  chrome.storage.onChanged.addListener((changes: { [key: string]: StorageChange<unknown> }, areaName: string) => {
    if (areaName === 'local') {
      if (changes.aiWebPilotEnabled && handlers.onAiWebPilotChanged) {
        handlers.onAiWebPilotChanged(changes.aiWebPilotEnabled.newValue === true);
      }
      if (changes.trackedTabId && handlers.onTrackedTabChanged) {
        handlers.onTrackedTabChanged();
      }
    }
  });
}

// =============================================================================
// RUNTIME LISTENERS
// =============================================================================

/**
 * Install browser startup listener (clears tracking state)
 */
export function installStartupListener(
  logFn?: (message: string) => void
): void {
  if (typeof chrome === 'undefined' || !chrome.runtime || !chrome.runtime.onStartup) return;

  chrome.runtime.onStartup.addListener(() => {
    if (logFn) logFn('[Gasoline] Browser restarted - clearing tracking state');
    chrome.storage.local.remove(['trackedTabId', 'trackedTabUrl']);
  });
}

// =============================================================================
// CONTENT SCRIPT HELPERS
// =============================================================================

/**
 * Ping content script to check if it's loaded
 */
export async function pingContentScript(tabId: number, timeoutMs = 500): Promise<boolean> {
  try {
    const response = await Promise.race([
      chrome.tabs.sendMessage(tabId, { type: 'GASOLINE_PING' }),
      new Promise<never>((_, reject) => {
        setTimeout(() => reject(new Error('timeout')), timeoutMs);
      }),
    ]) as { status?: string };
    return response?.status === 'alive';
  } catch {
    return false;
  }
}

/**
 * Wait for tab to finish loading
 */
export async function waitForTabLoad(tabId: number, timeoutMs = 5000): Promise<boolean> {
  const startTime = Date.now();
  while (Date.now() - startTime < timeoutMs) {
    try {
      const tab = await chrome.tabs.get(tabId);
      if (tab.status === 'complete') return true;
    } catch {
      return false;
    }
    await new Promise((r) => {
      setTimeout(r, 100);
    });
  }
  return false;
}

/**
 * Forward a message to all content scripts
 */
export function forwardToAllContentScripts(
  message: { type: string; [key: string]: unknown },
  debugLogFn?: (category: string, message: string, data?: unknown) => void
): void {
  if (typeof chrome === 'undefined' || !chrome.tabs) return;

  chrome.tabs.query({}, (tabs) => {
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
                error: err.message,
              });
            }
          }
        });
      }
    }
  });
}

// =============================================================================
// SETTINGS LOADING
// =============================================================================

/**
 * Load saved settings from chrome.storage.local
 */
export function loadSavedSettings(
  callback: (settings: {
    serverUrl?: string;
    logLevel?: string;
    screenshotOnError?: boolean;
    sourceMapEnabled?: boolean;
    debugMode?: boolean;
  }) => void
): void {
  if (typeof chrome === 'undefined' || !chrome.storage) {
    callback({});
    return;
  }

  chrome.storage.local.get(
    ['serverUrl', 'logLevel', 'screenshotOnError', 'sourceMapEnabled', 'debugMode'],
    (result: {
      serverUrl?: string;
      logLevel?: string;
      screenshotOnError?: boolean;
      sourceMapEnabled?: boolean;
      debugMode?: boolean;
    }) => {
      if (chrome.runtime.lastError) {
        console.warn('[Gasoline] Could not load saved settings:', chrome.runtime.lastError.message, '- using defaults');
        callback({});
        return;
      }
      callback(result);
    }
  );
}

/**
 * Load AI Web Pilot enabled state from storage
 */
export function loadAiWebPilotState(
  callback: (enabled: boolean) => void,
  logFn?: (message: string) => void
): void {
  if (typeof chrome === 'undefined' || !chrome.storage) {
    callback(false);
    return;
  }

  const startTime = performance.now();
  chrome.storage.local.get(['aiWebPilotEnabled'], (result: { aiWebPilotEnabled?: boolean }) => {
    const wasLoaded = result.aiWebPilotEnabled === true;
    const loadTime = performance.now() - startTime;
    if (logFn) {
      logFn(`[Gasoline] AI Web Pilot loaded on startup: ${wasLoaded} (took ${loadTime.toFixed(1)}ms)`);
    }
    callback(wasLoaded);
  });
}

/**
 * Load debug mode state from storage
 */
export function loadDebugModeState(
  callback: (enabled: boolean) => void
): void {
  if (typeof chrome === 'undefined' || !chrome.storage) {
    callback(false);
    return;
  }

  chrome.storage.local.get(['debugMode'], (result: { debugMode?: boolean }) => {
    callback(result.debugMode === true);
  });
}

/**
 * Save setting to chrome.storage.local
 */
export function saveSetting(key: string, value: unknown): void {
  if (typeof chrome === 'undefined' || !chrome.storage) return;
  chrome.storage.local.set({ [key]: value });
}

/**
 * Get tracked tab information (callback-based for compatibility with pre-async event listeners)
 */
// Overload: Promise-based (for await usage)
export function getTrackedTabInfo(): Promise<{ trackedTabId: number | null; trackedTabUrl: string | null }>;
// Overload: Callback-based (for backward compatibility)
export function getTrackedTabInfo(
  callback: (info: { trackedTabId: number | null; trackedTabUrl: string | null }) => void
): void;
// Implementation
export function getTrackedTabInfo(
  callback?: (info: { trackedTabId: number | null; trackedTabUrl: string | null }) => void
): void | Promise<{ trackedTabId: number | null; trackedTabUrl: string | null }> {
  if (!callback) {
    // Promise-based version
    return new Promise((resolve) => {
      getTrackedTabInfo((info) => resolve(info));
    });
  }

  // Callback-based version
  if (typeof chrome === 'undefined' || !chrome.storage) {
    callback({ trackedTabId: null, trackedTabUrl: null });
    return;
  }

  chrome.storage.local.get(['trackedTabId', 'trackedTabUrl'], (result: {
    trackedTabId?: number;
    trackedTabUrl?: string;
  }) => {
    callback({
      trackedTabId: result.trackedTabId || null,
      trackedTabUrl: result.trackedTabUrl || null,
    });
  });
}

/**
 * Clear tracked tab state
 */
export function clearTrackedTab(): void {
  if (typeof chrome === 'undefined' || !chrome.storage) return;
  chrome.storage.local.remove(['trackedTabId', 'trackedTabUrl']);
}

/**
 * Get all extension config settings
 */
// Overload: Promise-based (for await usage)
export function getAllConfigSettings(): Promise<Record<string, boolean | string | undefined>>;
// Overload: Callback-based (for backward compatibility)
export function getAllConfigSettings(
  callback: (settings: Record<string, boolean | string | undefined>) => void
): void;
// Implementation
export function getAllConfigSettings(
  callback?: (settings: Record<string, boolean | string | undefined>) => void
): void | Promise<Record<string, boolean | string | undefined>> {
  if (!callback) {
    // Promise-based version
    return new Promise((resolve) => {
      getAllConfigSettings((settings) => resolve(settings));
    });
  }

  // Callback-based version
  if (typeof chrome === 'undefined' || !chrome.storage) {
    callback({});
    return;
  }

  chrome.storage.local.get([
    'aiWebPilotEnabled',
    'webSocketCaptureEnabled',
    'networkWaterfallEnabled',
    'performanceMarksEnabled',
    'actionReplayEnabled',
    'screenshotOnError',
    'sourceMapEnabled',
    'networkBodyCaptureEnabled',
  ], (result: Record<string, boolean | string | undefined>) => {
    callback(result);
  });
}
