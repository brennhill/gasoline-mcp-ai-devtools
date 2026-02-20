/**
 * Purpose: Handles extension background coordination and message routing.
 * Docs: docs/features/feature/analyze-tool/index.md
 * Docs: docs/features/feature/interact-explore/index.md
 * Docs: docs/features/feature/observe/index.md
 */

/**
 * @fileoverview Extension Initialization
 * Handles startup logic: loading settings, installing listeners, and initial connection setup.
 * Uses async/await for cleaner control flow (replaces callback nesting).
 */

import * as index from './index'
import {
  setDebugMode,
  setServerUrl,
  setCurrentLogLevel,
  setScreenshotOnError,
  setAiWebPilotEnabledCache,
  setAiWebPilotCacheInitialized,
  setPilotInitCallback,
  resetSyncClientConnection,
  markInitComplete
} from './index'
import * as stateManager from './state-manager'
import * as eventListeners from './event-listeners'
import { handlePendingQuery } from './pending-queries'
import type { MessageHandlerDependencies } from './message-handlers'
import { installMessageListener, broadcastTrackingState } from './message-handlers'
import * as communication from './communication'
import * as storageUtils from './storage-utils'

// =============================================================================
// STATE SETTERS (imported from index.ts)
// =============================================================================
// These are now exported from index.ts to properly mutate module state

/**
 * Initialize the extension on startup
 * Handles state recovery after service worker restart, loads settings, installs listeners.
 * Uses async/await for readable, linear control flow.
 */
export function initializeExtension(): void {
  if (typeof chrome === 'undefined' || !chrome.runtime) {
    return
  }

  // Fire async initialization without awaiting at top level
  // (Service worker will remain alive as long as event handlers are installed)
  initializeExtensionAsync().catch((err) => {
    console.error('[Gasoline] Failed to initialize extension:', err)
  })
}

/**
 * Async initialization sequence
 * Reads settings, installs listeners, sets up connection checking.
 */
async function initializeExtensionAsync(): Promise<void> {
  try {
    // ============= STEP 1: Check service worker restart =============
    const wasRestarted = await storageUtils.wasServiceWorkerRestarted()
    if (wasRestarted) {
      console.warn(
        '[Gasoline] Service worker restarted - ephemeral state cleared. ' +
          'User preferences restored from persistent storage.'
      )
      index.debugLog(index.DebugCategory.LIFECYCLE, 'Service worker restarted, ephemeral state recovered')
    }
    // Mark the current state version
    await storageUtils.markStateVersion()

    // ============= STEP 2: Load debug mode =============
    const debugEnabled = await eventListeners.loadDebugModeState()
    setDebugMode(debugEnabled)
    if (debugEnabled) {
      console.log('[Gasoline] Debug mode enabled on startup')
    }

    // ============= STEP 3: Install startup listener =============
    eventListeners.installStartupListener((msg) => console.log(msg))

    // ============= STEP 4: Load AI Web Pilot state =============
    const aiPilotEnabled = await eventListeners.loadAiWebPilotState()
    setAiWebPilotEnabledCache(aiPilotEnabled)
    setAiWebPilotCacheInitialized(true)
    console.log('[Gasoline] Storage value:', aiPilotEnabled, '| Cache value:', index.__aiWebPilotEnabledCache)

    // Execute any pending pilot init callback
    const pilotCb = index.__pilotInitCallback
    if (pilotCb) {
      pilotCb()
      setPilotInitCallback(null)
    }

    // ============= STEP 5: Load saved settings =============
    const settings = await eventListeners.loadSavedSettings()
    setServerUrl(settings.serverUrl || index.DEFAULT_SERVER_URL)
    setCurrentLogLevel('all')
    setScreenshotOnError(settings.screenshotOnError !== false)
    stateManager.setSourceMapEnabled(settings.sourceMapEnabled !== false)
    setDebugMode(settings.debugMode || false)

    // ============= STEP 6: Install storage change listener =============
    eventListeners.installStorageChangeListener({
      onAiWebPilotChanged: (newValue) => {
        setAiWebPilotEnabledCache(newValue)
        console.log('[Gasoline] AI Web Pilot cache updated from storage:', newValue)
        // Reset connection when AI Web Pilot is enabled to allow immediate reconnection
        if (newValue) {
          resetSyncClientConnection()
          console.log('[Gasoline] Sync client reset due to AI Web Pilot enabled')
        }
        // Broadcast to tracked tab for favicon flicker
        broadcastTrackingState().catch((err) => console.error('[Gasoline] Error broadcasting tracking state:', err))
      },
      onTrackedTabChanged: (newTabId, oldTabId) => {
        index.sendStatusPingWrapper()
        if (newTabId !== null) {
          resetSyncClientConnection()
          console.log('[Gasoline] Sync client reset due to tracking enabled')
        } else if (oldTabId !== null) {
          // Tracking was lost — notify user on active tab
          console.log('[Gasoline] Tracking lost for tab', oldTabId)
          chrome.tabs
            .query({ active: true, currentWindow: true })
            .then((tabs) => {
              if (tabs[0]?.id) {
                chrome.tabs
                  .sendMessage(tabs[0].id, {
                    type: 'GASOLINE_ACTION_TOAST',
                    text: 'Tab tracking lost',
                    detail: 'Re-enable in Gasoline popup',
                    state: 'warning',
                    duration_ms: 5000
                  })
                  .catch(() => {})
              }
            })
            .catch(() => {})
        }
        broadcastTrackingState(oldTabId).catch((err) =>
          console.error('[Gasoline] Error broadcasting tracking state:', err)
        )
      }
    })

    // ============= STEP 7: Install message handler =============
    // #lizard forgives
    const deps: MessageHandlerDependencies = {
      getServerUrl: () => index.serverUrl,
      getConnectionStatus: () => index.connectionStatus,
      getDebugMode: () => index.debugMode,
      getScreenshotOnError: () => index.screenshotOnError,
      getSourceMapEnabled: () => stateManager.isSourceMapEnabled(),
      getCurrentLogLevel: () => index.currentLogLevel,
      getContextWarning: stateManager.getContextWarning,
      getCircuitBreakerState: () => index.sharedServerCircuitBreaker.getState(),
      getMemoryPressureState: stateManager.getMemoryPressureState,
      getAiWebPilotEnabled: () => index.__aiWebPilotEnabledCache,
      isNetworkBodyCaptureDisabled: stateManager.isNetworkBodyCaptureDisabled,

      setServerUrl: (url) => {
        setServerUrl(url || index.DEFAULT_SERVER_URL)
      },
      setCurrentLogLevel: (level) => {
        setCurrentLogLevel(level)
      },
      setScreenshotOnError: (enabled) => {
        setScreenshotOnError(enabled)
      },
      setSourceMapEnabled: (enabled) => {
        stateManager.setSourceMapEnabled(enabled)
      },
      setDebugMode: (enabled) => {
        setDebugMode(enabled)
      },
      setAiWebPilotEnabled: (enabled, callback) => {
        chrome.storage.local.set({ aiWebPilotEnabled: enabled }, () => {
          setAiWebPilotEnabledCache(enabled)
          // Reset connection when enabling to allow immediate reconnection
          if (enabled) {
            resetSyncClientConnection()
            console.log('[Gasoline] Sync client reset due to AI Web Pilot enabled (direct)')
          }
          if (callback) callback()
        })
      },

      addToLogBatcher: (entry) => index.logBatcher.add(entry),
      addToWsBatcher: (event) => index.wsBatcher.add(event),
      addToEnhancedActionBatcher: (action) => index.enhancedActionBatcher.add(action),
      addToNetworkBodyBatcher: (body) => index.networkBodyBatcher.add(body),
      addToPerfBatcher: (snapshot) => index.perfBatcher.add(snapshot),

      handleLogMessage: index.handleLogMessage,
      handleClearLogs: index.handleClearLogs,
      captureScreenshot: (tabId, relatedErrorId) =>
        communication.captureScreenshot(
          tabId,
          index.serverUrl,
          relatedErrorId,
          null,
          stateManager.canTakeScreenshot,
          stateManager.recordScreenshot,
          index.debugLog
        ),
      checkConnectionAndUpdate: index.checkConnectionAndUpdate,
      clearSourceMapCache: stateManager.clearSourceMapCache,

      debugLog: index.debugLog,
      exportDebugLog: index.exportDebugLog,
      clearDebugLog: index.clearDebugLog,

      saveSetting: eventListeners.saveSetting,
      forwardToAllContentScripts: (msg) => eventListeners.forwardToAllContentScripts(msg, index.debugLog)
    }

    installMessageListener(deps)

    // ============= STEP 8: Setup Chrome alarms =============
    eventListeners.setupChromeAlarms()
    eventListeners.installAlarmListener({
      onReconnect: index.checkConnectionAndUpdate,
      onErrorGroupFlush: () => {
        const aggregatedEntries = stateManager.flushErrorGroups()
        if (aggregatedEntries.length > 0) {
          aggregatedEntries.forEach((entry) => index.logBatcher.add(entry))
        }
      },
      onMemoryCheck: () => {
        index.debugLog(index.DebugCategory.LIFECYCLE, 'Memory check alarm fired')
      },
      onErrorGroupCleanup: () => stateManager.cleanupStaleErrorGroups(index.debugLog)
    })

    // ============= STEP 9: Install tab removed listener =============
    eventListeners.installTabRemovedListener((tabId) => {
      stateManager.clearScreenshotTimestamps(tabId)
      eventListeners.handleTrackedTabClosed(tabId, (msg) => console.log(msg))
    })

    // ============= STEP 9.5: Install tab updated listener =============
    eventListeners.installTabUpdatedListener((tabId, newUrl) => {
      eventListeners.handleTrackedTabUrlChange(tabId, newUrl, (msg) => console.log(msg))
    })

    // ============= STEP 9.6: Install draw mode keyboard shortcut listener =============
    eventListeners.installDrawModeCommandListener((msg) => console.log(`[Gasoline] ${msg}`))

    // ============= STEP 10: Set disconnected badge immediately =============
    // Badge must reflect disconnected state BEFORE the async health check.
    // Without this, a stale "connected" badge persists from a previous SW session
    // until the health check completes (could be seconds if server is slow to refuse).
    communication.updateBadge(index.connectionStatus)

    // ============= STEP 11: Initial connection check =============
    // Await the connection check to keep the SW alive until the badge is updated.
    // Without await, Chrome may suspend the SW before the fetch completes.
    if (index.__aiWebPilotCacheInitialized) {
      await index.checkConnectionAndUpdate()
    } else {
      setPilotInitCallback(index.checkConnectionAndUpdate)
    }

    // ============= INITIALIZATION COMPLETE =============
    markInitComplete()
    index.debugLog(index.DebugCategory.LIFECYCLE, 'Extension initialized', {
      serverUrl: index.serverUrl,
      logLevel: index.currentLogLevel,
      screenshotOnError: index.screenshotOnError,
      sourceMapEnabled: stateManager.isSourceMapEnabled(),
      debugMode: index.debugMode
    })
  } catch (error) {
    console.error('[Gasoline] Error during extension initialization:', error)
    index.debugLog(index.DebugCategory.LIFECYCLE, 'Extension initialization failed', { error: String(error) })
  }
}
