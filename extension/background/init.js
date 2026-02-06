/**
 * @fileoverview Extension Initialization
 * Handles startup logic: loading settings, installing listeners, and initial connection setup.
 * Uses async/await for cleaner control flow (replaces callback nesting).
 */
import * as index from './index.js';
import { setDebugMode, setServerUrl, setCurrentLogLevel, setScreenshotOnError, setAiWebPilotEnabledCache, setAiWebPilotCacheInitialized, setPilotInitCallback, resetSyncClientConnection, } from './index.js';
import * as stateManager from './state-manager.js';
import * as eventListeners from './event-listeners.js';
import { installMessageListener, broadcastTrackingState } from './message-handlers.js';
import * as communication from './communication.js';
import * as storageUtils from './storage-utils.js';
// =============================================================================
// PROMISE WRAPPERS FOR CHROME STORAGE APIs
// =============================================================================
/**
 * Promisified version of loadSavedSettings
 */
function loadSavedSettingsAsync() {
    return new Promise((resolve) => {
        eventListeners.loadSavedSettings((settings) => resolve(settings));
    });
}
/**
 * Promisified version of loadAiWebPilotState
 */
function loadAiWebPilotStateAsync() {
    return new Promise((resolve) => {
        eventListeners.loadAiWebPilotState((enabled) => resolve(enabled));
    });
}
/**
 * Promisified version of loadDebugModeState
 */
function loadDebugModeStateAsync() {
    return new Promise((resolve) => {
        eventListeners.loadDebugModeState((enabled) => resolve(enabled));
    });
}
/**
 * Promisified version of wasServiceWorkerRestarted
 */
function wasServiceWorkerRestartedAsync() {
    return new Promise((resolve) => {
        storageUtils.wasServiceWorkerRestarted((wasRestarted) => resolve(wasRestarted));
    });
}
/**
 * Promisified version of markStateVersion
 */
function markStateVersionAsync() {
    return new Promise((resolve) => {
        storageUtils.markStateVersion(() => resolve());
    });
}
// =============================================================================
// STATE SETTERS (imported from index.ts)
// =============================================================================
// These are now exported from index.ts to properly mutate module state
/**
 * Initialize the extension on startup
 * Handles state recovery after service worker restart, loads settings, installs listeners.
 * Uses async/await for readable, linear control flow.
 */
export function initializeExtension() {
    if (typeof chrome === 'undefined' || !chrome.runtime) {
        return;
    }
    // Fire async initialization without awaiting at top level
    // (Service worker will remain alive as long as event handlers are installed)
    initializeExtensionAsync().catch((err) => {
        console.error('[Gasoline] Failed to initialize extension:', err);
    });
}
/**
 * Async initialization sequence
 * Reads settings, installs listeners, sets up connection checking.
 */
async function initializeExtensionAsync() {
    try {
        // ============= STEP 1: Check service worker restart =============
        const wasRestarted = await wasServiceWorkerRestartedAsync();
        if (wasRestarted) {
            console.warn('[Gasoline] Service worker restarted - ephemeral state cleared. ' +
                'User preferences restored from persistent storage.');
            index.debugLog(index.DebugCategory.LIFECYCLE, 'Service worker restarted, ephemeral state recovered');
        }
        // Mark the current state version
        await markStateVersionAsync();
        // ============= STEP 2: Load debug mode =============
        const debugEnabled = await loadDebugModeStateAsync();
        setDebugMode(debugEnabled);
        if (debugEnabled) {
            console.log('[Gasoline] Debug mode enabled on startup');
        }
        // ============= STEP 3: Install startup listener =============
        eventListeners.installStartupListener((msg) => console.log(msg));
        // ============= STEP 4: Load AI Web Pilot state =============
        const aiPilotEnabled = await loadAiWebPilotStateAsync();
        setAiWebPilotEnabledCache(aiPilotEnabled);
        setAiWebPilotCacheInitialized(true);
        console.log('[Gasoline] Storage value:', aiPilotEnabled, '| Cache value:', index.__aiWebPilotEnabledCache);
        // Execute any pending pilot init callback
        if (index.__pilotInitCallback) {
            index.__pilotInitCallback();
            setPilotInitCallback(null);
        }
        // ============= STEP 5: Load saved settings =============
        const settings = await loadSavedSettingsAsync();
        setServerUrl(settings.serverUrl || 'http://localhost:7890');
        setCurrentLogLevel('all');
        setScreenshotOnError(settings.screenshotOnError !== false);
        stateManager.setSourceMapEnabled(settings.sourceMapEnabled !== false);
        setDebugMode(settings.debugMode || false);
        // ============= STEP 6: Install storage change listener =============
        eventListeners.installStorageChangeListener({
            onAiWebPilotChanged: (newValue) => {
                setAiWebPilotEnabledCache(newValue);
                console.log('[Gasoline] AI Web Pilot cache updated from storage:', newValue);
                // Reset connection when AI Web Pilot is enabled to allow immediate reconnection
                if (newValue) {
                    resetSyncClientConnection();
                    console.log('[Gasoline] Sync client reset due to AI Web Pilot enabled');
                }
                // Broadcast to tracked tab for favicon flicker
                broadcastTrackingState().catch((err) => console.error('[Gasoline] Error broadcasting tracking state:', err));
            },
            onTrackedTabChanged: (newTabId, oldTabId) => {
                index.sendStatusPingWrapper();
                // Reset connection when tracking is enabled to allow immediate reconnection
                if (newTabId !== null) {
                    resetSyncClientConnection();
                    console.log('[Gasoline] Sync client reset due to tracking enabled');
                }
                // Broadcast to tracked tab for favicon flicker (pass old tab to notify it to stop flicker)
                broadcastTrackingState(oldTabId).catch((err) => console.error('[Gasoline] Error broadcasting tracking state:', err));
            },
        });
        // ============= STEP 7: Install message handler =============
        const deps = {
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
                setServerUrl(url || 'http://localhost:7890');
            },
            setCurrentLogLevel: (level) => {
                setCurrentLogLevel(level);
            },
            setScreenshotOnError: (enabled) => {
                setScreenshotOnError(enabled);
            },
            setSourceMapEnabled: (enabled) => {
                stateManager.setSourceMapEnabled(enabled);
            },
            setDebugMode: (enabled) => {
                setDebugMode(enabled);
            },
            setAiWebPilotEnabled: (enabled, callback) => {
                chrome.storage.local.set({ aiWebPilotEnabled: enabled }, () => {
                    setAiWebPilotEnabledCache(enabled);
                    // Reset connection when enabling to allow immediate reconnection
                    if (enabled) {
                        resetSyncClientConnection();
                        console.log('[Gasoline] Sync client reset due to AI Web Pilot enabled (direct)');
                    }
                    if (callback)
                        callback();
                });
            },
            addToLogBatcher: (entry) => index.logBatcher.add(entry),
            addToWsBatcher: (event) => index.wsBatcher.add(event),
            addToEnhancedActionBatcher: (action) => index.enhancedActionBatcher.add(action),
            addToNetworkBodyBatcher: (body) => index.networkBodyBatcher.add(body),
            addToPerfBatcher: (snapshot) => index.perfBatcher.add(snapshot),
            handleLogMessage: index.handleLogMessage,
            handleClearLogs: index.handleClearLogs,
            captureScreenshot: (tabId, relatedErrorId) => communication.captureScreenshot(tabId, index.serverUrl, relatedErrorId, null, stateManager.canTakeScreenshot, stateManager.recordScreenshot, index.debugLog),
            checkConnectionAndUpdate: index.checkConnectionAndUpdate,
            clearSourceMapCache: stateManager.clearSourceMapCache,
            debugLog: index.debugLog,
            exportDebugLog: index.exportDebugLog,
            clearDebugLog: index.clearDebugLog,
            saveSetting: eventListeners.saveSetting,
            forwardToAllContentScripts: (msg) => eventListeners.forwardToAllContentScripts(msg, index.debugLog),
        };
        installMessageListener(deps);
        // ============= STEP 8: Setup Chrome alarms =============
        eventListeners.setupChromeAlarms();
        eventListeners.installAlarmListener({
            onReconnect: index.checkConnectionAndUpdate,
            onErrorGroupFlush: () => {
                const aggregatedEntries = stateManager.flushErrorGroups();
                if (aggregatedEntries.length > 0) {
                    aggregatedEntries.forEach((entry) => index.logBatcher.add(entry));
                }
            },
            onMemoryCheck: () => {
                index.debugLog(index.DebugCategory.LIFECYCLE, 'Memory check alarm fired');
            },
            onErrorGroupCleanup: () => stateManager.cleanupStaleErrorGroups(index.debugLog),
        });
        // ============= STEP 9: Install tab removed listener =============
        eventListeners.installTabRemovedListener((tabId) => {
            stateManager.clearScreenshotTimestamps(tabId);
            eventListeners.handleTrackedTabClosed(tabId, (msg) => console.log(msg));
        });
        // ============= STEP 9.5: Install tab updated listener =============
        eventListeners.installTabUpdatedListener((tabId, newUrl) => {
            eventListeners.handleTrackedTabUrlChange(tabId, newUrl, (msg) => console.log(msg));
        });
        // ============= STEP 10: Initial connection check =============
        if (index.__aiWebPilotCacheInitialized) {
            index.checkConnectionAndUpdate();
        }
        else {
            setPilotInitCallback(index.checkConnectionAndUpdate);
        }
        // ============= INITIALIZATION COMPLETE =============
        index.debugLog(index.DebugCategory.LIFECYCLE, 'Extension initialized', {
            serverUrl: index.serverUrl,
            logLevel: index.currentLogLevel,
            screenshotOnError: index.screenshotOnError,
            sourceMapEnabled: stateManager.isSourceMapEnabled(),
            debugMode: index.debugMode,
        });
    }
    catch (error) {
        console.error('[Gasoline] Error during extension initialization:', error);
        index.debugLog(index.DebugCategory.LIFECYCLE, 'Extension initialization failed', { error: String(error) });
    }
}
//# sourceMappingURL=init.js.map