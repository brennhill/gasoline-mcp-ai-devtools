/**
 * @fileoverview Extension Initialization
 * Handles startup logic: loading settings, installing listeners, and initial connection setup.
 */

import * as index from './index';
import * as stateManager from './state-manager';
import * as eventListeners from './event-listeners';
import { handlePendingQuery } from './pending-queries';
import type { MessageHandlerDependencies } from './message-handlers';
import { installMessageListener } from './message-handlers';
import * as communication from './communication';
import * as storageUtils from './storage-utils';

// Setter helper functions
function setDebugMode(enabled: boolean): void {
  (index as any).debugMode = enabled;
}

function setServerUrl(url: string): void {
  (index as any).serverUrl = url;
}

function setCurrentLogLevel(level: string): void {
  (index as any).currentLogLevel = level;
}

function setScreenshotOnError(enabled: boolean): void {
  (index as any).screenshotOnError = enabled;
}

function setAiWebPilotEnabledCache(enabled: boolean): void {
  (index as any).__aiWebPilotEnabledCache = enabled;
}

function setAiWebPilotCacheInitialized(initialized: boolean): void {
  (index as any).__aiWebPilotCacheInitialized = initialized;
}

function setPilotInitCallback(callback: (() => void) | null): void {
  (index as any).__pilotInitCallback = callback;
}

/**
 * Initialize the extension on startup
 * Handles state recovery after service worker restart
 */
export function initializeExtension(): void {
  if (typeof chrome === 'undefined' || !chrome.runtime) {
    return;
  }

  // SECURITY: Check if service worker was restarted and state was lost
  storageUtils.wasServiceWorkerRestarted((wasRestarted) => {
    if (wasRestarted) {
      console.warn(
        '[Gasoline] Service worker restarted - ephemeral state cleared. ' +
          'User preferences restored from persistent storage.'
      );
      index.debugLog(index.DebugCategory.LIFECYCLE, 'Service worker restarted, ephemeral state recovered');
    }
    // Mark current state version
    storageUtils.markStateVersion();
  });

  // Load debug mode setting
  eventListeners.loadDebugModeState((enabled) => {
    setDebugMode(enabled);
    if (enabled) {
      console.log('[Gasoline] Debug mode enabled on startup');
    }
  });

  // Install browser startup listener
  eventListeners.installStartupListener((msg) => console.log(msg));

  // Load AI Web Pilot state
  eventListeners.loadAiWebPilotState(
    (enabled) => {
      setAiWebPilotEnabledCache(enabled);
      setAiWebPilotCacheInitialized(true);
      console.log('[Gasoline] Storage value:', enabled, '| Cache value:', index.__aiWebPilotEnabledCache);

      if (index.__pilotInitCallback) {
        index.__pilotInitCallback();
        setPilotInitCallback(null);
      }

      eventListeners.loadSavedSettings((result) => {
        if (result.serverUrl) {
          index.postSettingsWrapper();
        }
      });
    },
    (msg) => console.log(msg)
  );

  // Install storage change listener
  eventListeners.installStorageChangeListener({
    onAiWebPilotChanged: (newValue) => {
      setAiWebPilotEnabledCache(newValue);
      console.log('[Gasoline] AI Web Pilot cache updated from storage:', newValue);
    },
    onTrackedTabChanged: () => {
      index.sendStatusPingWrapper();
    },
  });

  // Install message handler
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

    setServerUrl: (url) => { setServerUrl(url || 'http://localhost:7890'); },
    setCurrentLogLevel: (level) => { setCurrentLogLevel(level); },
    setScreenshotOnError: (enabled) => { setScreenshotOnError(enabled); },
    setSourceMapEnabled: (enabled) => {
      stateManager.setSourceMapEnabled(enabled);
    },
    setDebugMode: (enabled) => { setDebugMode(enabled); },
    setAiWebPilotEnabled: (enabled, callback) => {
      chrome.storage.local.set({ aiWebPilotEnabled: enabled }, () => {
        setAiWebPilotEnabledCache(enabled);
        if (callback) callback();
      });
    },

    addToLogBatcher: (entry) => index.logBatcher.add(entry),
    addToWsBatcher: (event) => index.wsBatcher.add(event as any),
    addToEnhancedActionBatcher: (action) => index.enhancedActionBatcher.add(action as any),
    addToNetworkBodyBatcher: (body) => index.networkBodyBatcher.add(body as any),
    addToPerfBatcher: (snapshot) => index.perfBatcher.add(snapshot as any),

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
    postSettings: index.postSettingsWrapper,

    debugLog: index.debugLog,
    exportDebugLog: index.exportDebugLog,
    clearDebugLog: index.clearDebugLog,

    saveSetting: eventListeners.saveSetting,
    forwardToAllContentScripts: (msg) => eventListeners.forwardToAllContentScripts(msg, index.debugLog),
  };

  installMessageListener(deps);

  // Setup Chrome alarms
  eventListeners.setupChromeAlarms();
  eventListeners.installAlarmListener({
    onReconnect: index.checkConnectionAndUpdate,
    onErrorGroupFlush: () => {
      const aggregatedEntries = stateManager.flushErrorGroups();
      if (aggregatedEntries.length > 0) {
        aggregatedEntries.forEach((entry) => index.logBatcher.add(entry as any));
      }
    },
    onMemoryCheck: () => {
      index.debugLog(index.DebugCategory.LIFECYCLE, 'Memory check alarm fired');
    },
    onErrorGroupCleanup: () => stateManager.cleanupStaleErrorGroups(index.debugLog),
  });

  // Install tab removed listener
  eventListeners.installTabRemovedListener((tabId) => {
    stateManager.clearScreenshotTimestamps(tabId);
    eventListeners.handleTrackedTabClosed(tabId, (msg) => console.log(msg));
  });

  // Initial connection check
  if (index.__aiWebPilotCacheInitialized) {
    index.checkConnectionAndUpdate();
  } else {
    index.__pilotInitCallback = index.checkConnectionAndUpdate;
  }

  // Load saved settings
  eventListeners.loadSavedSettings((result) => {
    setServerUrl(result.serverUrl || 'http://localhost:7890');
    setCurrentLogLevel(result.logLevel || 'error');
    setScreenshotOnError(result.screenshotOnError || false);
    stateManager.setSourceMapEnabled(result.sourceMapEnabled || false);
    setDebugMode(result.debugMode || false);
    index.debugLog(index.DebugCategory.LIFECYCLE, 'Extension initialized', {
      serverUrl: index.serverUrl,
      logLevel: index.currentLogLevel,
      screenshotOnError: index.screenshotOnError,
      sourceMapEnabled: stateManager.isSourceMapEnabled(),
      debugMode: index.debugMode,
    });
  });
}
