/**
 * @fileoverview Main Background Service Worker
 * Manages server communication, batchers, log handling, and pending query processing.
 * Receives captured events from content scripts, batches them with debouncing,
 * and posts to the Go server. Handles error deduplication, connection status,
 * badge updates, and on-demand query polling.
 */
import * as stateManager from './state-manager.js';
import * as communication from './communication.js';
import * as eventListeners from './event-listeners.js';
import { DebugCategory } from './debug.js';
import { saveStateSnapshot, loadStateSnapshot, listStateSnapshots, deleteStateSnapshot, } from './message-handlers.js';
import { handlePendingQuery as handlePendingQueryImpl, handlePilotCommand as handlePilotCommandImpl, } from './pending-queries.js';
import { createSyncClient } from './sync-client.js';
// =============================================================================
// CONSTANTS
// =============================================================================
export const DEFAULT_SERVER_URL = 'http://localhost:7890';
// =============================================================================
// MODULE STATE
// =============================================================================
/** Session ID for detecting extension reloads */
export const EXTENSION_SESSION_ID = `ext_${Date.now()}_${Math.random().toString(36).slice(2, 8)}`;
// All communication now uses unified /sync endpoint
/** Sync client instance (initialized lazily) */
let syncClient = null;
/** Server URL */
export let serverUrl = DEFAULT_SERVER_URL;
/** Debug mode flag */
export let debugMode = false;
export let connectionStatus = {
    connected: false,
    entries: 0,
    maxEntries: 1000,
    errorCount: 0,
    logFile: '',
};
/** Log level filter */
export let currentLogLevel = 'all';
/** Screenshot settings */
export let screenshotOnError = false;
/** AI capture control state */
export let _captureOverrides = {};
export let aiControlled = false;
/** Connection check mutex */
export let _connectionCheckRunning = false;
/** AI Web Pilot state */
export let __aiWebPilotEnabledCache = false;
export let __aiWebPilotCacheInitialized = false;
export let __pilotInitCallback = null;
/** Extension log queue for server posting */
export const extensionLogQueue = [];
// =============================================================================
// STATE SETTERS (for init.ts)
// =============================================================================
// Note: setDebugMode is defined later in the file
export function setServerUrl(url) {
    serverUrl = url;
}
export function setCurrentLogLevel(level) {
    currentLogLevel = level;
}
export function setScreenshotOnError(enabled) {
    screenshotOnError = enabled;
}
export function setAiWebPilotEnabledCache(enabled) {
    __aiWebPilotEnabledCache = enabled;
}
export function setAiWebPilotCacheInitialized(initialized) {
    __aiWebPilotCacheInitialized = initialized;
}
export function setPilotInitCallback(callback) {
    __pilotInitCallback = callback;
}
// =============================================================================
// DEBUG LOGGING
// =============================================================================
// Re-export DebugCategory from debug module (to avoid circular dependencies)
export { DebugCategory } from './debug.js';
/**
 * Log a diagnostic message only when debug mode is enabled
 */
export function diagnosticLog(message) {
    if (debugMode) {
        console.log(message);
    }
}
/**
 * Log a debug message (only when debug mode is enabled)
 */
export function debugLog(category, message, data = null) {
    const timestamp = new Date().toISOString();
    // Cast category to DebugCategory - callers use DebugCategory constants
    const entry = {
        ts: timestamp,
        category: category,
        message,
        ...(data !== null ? { data } : {}),
    };
    stateManager.addDebugLogEntry(entry);
    if (connectionStatus.connected) {
        extensionLogQueue.push({
            timestamp,
            level: 'debug',
            message,
            source: 'background',
            category,
            ...(data !== null ? { data } : {}),
        });
        // Cap queue size to prevent memory leak if server is unreachable
        const MAX_EXTENSION_LOGS = 2000;
        if (extensionLogQueue.length > MAX_EXTENSION_LOGS) {
            extensionLogQueue.splice(0, extensionLogQueue.length - MAX_EXTENSION_LOGS);
        }
    }
    if (debugMode) {
        const prefix = `[Gasoline:${category}]`;
        if (data !== null) {
            console.log(prefix, message, data);
        }
        else {
            console.log(prefix, message);
        }
    }
}
/**
 * Get all debug log entries
 */
export function getDebugLog() {
    return stateManager.getDebugLog();
}
/**
 * Clear debug log buffer
 */
export function clearDebugLog() {
    stateManager.clearDebugLog();
}
/**
 * Export debug log as JSON string
 */
export function exportDebugLog() {
    return JSON.stringify({
        exportedAt: new Date().toISOString(),
        version: typeof chrome !== 'undefined' ? chrome.runtime.getManifest().version : 'test',
        debugMode,
        connectionStatus,
        settings: {
            logLevel: currentLogLevel,
            screenshotOnError,
            sourceMapEnabled: stateManager.isSourceMapEnabled(),
        },
        entries: stateManager.getDebugLog(),
    }, null, 2);
}
/**
 * Set debug mode enabled/disabled
 */
export function setDebugMode(enabled) {
    debugMode = enabled;
    debugLog(DebugCategory.SETTINGS, `Debug mode ${enabled ? 'enabled' : 'disabled'}`);
}
// =============================================================================
// SHARED CIRCUIT BREAKER
// =============================================================================
export const sharedServerCircuitBreaker = communication.createCircuitBreaker(() => Promise.reject(new Error('shared circuit breaker')), {
    maxFailures: communication.RATE_LIMIT_CONFIG.maxFailures,
    resetTimeout: communication.RATE_LIMIT_CONFIG.resetTimeout,
    initialBackoff: 0,
    maxBackoff: 0,
});
// =============================================================================
// BATCHERS
// =============================================================================
function withConnectionStatus(sendFn, onSuccess) {
    return async (entries) => {
        try {
            const result = await sendFn(entries);
            connectionStatus.connected = true;
            if (onSuccess)
                onSuccess(entries, result);
            communication.updateBadge(connectionStatus);
            return result;
        }
        catch (err) {
            connectionStatus.connected = false;
            communication.updateBadge(connectionStatus);
            throw err;
        }
    };
}
export const logBatcherWithCB = communication.createBatcherWithCircuitBreaker(withConnectionStatus((entries) => {
    stateManager.checkContextAnnotations(entries);
    return communication.sendLogsToServer(serverUrl, entries, debugLog);
}, (entries, result) => {
    const typedResult = result;
    connectionStatus.entries = typedResult.entries || connectionStatus.entries + entries.length;
    connectionStatus.errorCount += entries.filter((e) => e.level === 'error').length;
}), { sharedCircuitBreaker: sharedServerCircuitBreaker });
export const logBatcher = logBatcherWithCB.batcher;
export const wsBatcherWithCB = communication.createBatcherWithCircuitBreaker(withConnectionStatus((events) => communication.sendWSEventsToServer(serverUrl, events, debugLog)), { debounceMs: 200, maxBatchSize: 100, sharedCircuitBreaker: sharedServerCircuitBreaker });
export const wsBatcher = wsBatcherWithCB.batcher;
export const enhancedActionBatcherWithCB = communication.createBatcherWithCircuitBreaker(withConnectionStatus((actions) => communication.sendEnhancedActionsToServer(serverUrl, actions, debugLog)), { debounceMs: 200, maxBatchSize: 50, sharedCircuitBreaker: sharedServerCircuitBreaker });
export const enhancedActionBatcher = enhancedActionBatcherWithCB.batcher;
export const networkBodyBatcherWithCB = communication.createBatcherWithCircuitBreaker(withConnectionStatus((bodies) => communication.sendNetworkBodiesToServer(serverUrl, bodies, debugLog)), { debounceMs: 200, maxBatchSize: 50, sharedCircuitBreaker: sharedServerCircuitBreaker });
export const networkBodyBatcher = networkBodyBatcherWithCB.batcher;
export const perfBatcherWithCB = communication.createBatcherWithCircuitBreaker(withConnectionStatus((snapshots) => communication.sendPerformanceSnapshotsToServer(serverUrl, snapshots, debugLog)), { debounceMs: 500, maxBatchSize: 10, sharedCircuitBreaker: sharedServerCircuitBreaker });
export const perfBatcher = perfBatcherWithCB.batcher;
// =============================================================================
// LOG HANDLING
// =============================================================================
export async function handleLogMessage(payload, sender, tabId) {
    if (!communication.shouldCaptureLog(payload.level, currentLogLevel, payload.type)) {
        debugLog(DebugCategory.CAPTURE, `Log filtered out: level=${payload.level}, type=${payload.type}`);
        return;
    }
    let entry = communication.formatLogEntry(payload);
    const resolvedTabId = tabId ?? sender?.tab?.id;
    if (resolvedTabId !== null && resolvedTabId !== undefined) {
        entry = { ...entry, tabId: resolvedTabId };
    }
    debugLog(DebugCategory.CAPTURE, `Log received: type=${entry.type}, level=${entry.level}`, {
        url: entry.url,
        enrichments: entry._enrichments,
    });
    if (stateManager.isSourceMapEnabled() && entry.stack) {
        try {
            const resolvedStack = await stateManager.resolveStackTrace(entry.stack, debugLog);
            const existingEnrichments = entry._enrichments;
            const enrichments = existingEnrichments ? [...existingEnrichments] : [];
            if (!enrichments.includes('sourceMap')) {
                enrichments.push('sourceMap');
            }
            entry = {
                ...entry,
                stack: resolvedStack,
                _sourceMapResolved: true,
                _enrichments: enrichments,
            };
            debugLog(DebugCategory.CAPTURE, 'Stack trace resolved via source map');
        }
        catch (err) {
            debugLog(DebugCategory.ERROR, 'Source map resolution failed', { error: err.message });
        }
    }
    const { shouldSend, entry: processedEntry } = stateManager.processErrorGroup(entry);
    if (shouldSend && processedEntry) {
        logBatcher.add(processedEntry);
        debugLog(DebugCategory.CAPTURE, `Log queued for server: type=${processedEntry.type}`, {
            aggregatedCount: processedEntry._aggregatedCount,
        });
        maybeAutoScreenshot(processedEntry, sender);
    }
    else {
        debugLog(DebugCategory.CAPTURE, 'Log deduplicated (error grouping)');
    }
}
async function maybeAutoScreenshot(errorEntry, sender) {
    if (!screenshotOnError)
        return;
    if (!sender?.tab?.id)
        return;
    if (errorEntry.level !== 'error')
        return;
    const entryType = errorEntry.type;
    if (entryType !== 'exception' && entryType !== 'network')
        return;
    const errorId = `err_${Date.now()}_${Math.random().toString(36).slice(2, 8)}`;
    errorEntry._errorId = errorId;
    const result = await communication.captureScreenshot(sender.tab.id, serverUrl, errorId, entryType || null, stateManager.canTakeScreenshot, stateManager.recordScreenshot, debugLog);
    if (result.success && result.entry) {
        logBatcher.add(result.entry);
    }
}
export async function handleClearLogs() {
    try {
        await fetch(`${serverUrl}/logs`, { method: 'DELETE' });
        connectionStatus.entries = 0;
        connectionStatus.errorCount = 0;
        communication.updateBadge(connectionStatus);
        return { success: true };
    }
    catch (error) {
        return { success: false, error: error.message };
    }
}
// =============================================================================
// CONNECTION MANAGEMENT
// =============================================================================
/**
 * Check if a connection check is currently running (for testing)
 */
export function isConnectionCheckRunning() {
    return _connectionCheckRunning;
}
export async function checkConnectionAndUpdate() {
    if (_connectionCheckRunning) {
        debugLog(DebugCategory.CONNECTION, 'Skipping connection check - already running');
        return;
    }
    _connectionCheckRunning = true;
    try {
        const health = await communication.checkServerHealth(serverUrl);
        // Update version information from health response
        if (health.connected) {
            import('./version-check')
                .then((vc) => {
                vc.updateVersionFromHealth({
                    version: health.version,
                    availableVersion: health.availableVersion,
                }, debugLog);
            })
                .catch((err) => {
                debugLog(DebugCategory.CONNECTION, 'Failed to update version info', { error: err.message });
            });
        }
        const wasConnected = connectionStatus.connected;
        connectionStatus = {
            ...connectionStatus,
            ...health,
            connected: health.connected,
        };
        if (health.logs) {
            connectionStatus.logFile = health.logs.logFile || connectionStatus.logFile;
            connectionStatus.logFileSize = health.logs.logFileSize;
            connectionStatus.entries = health.logs.entries ?? connectionStatus.entries;
            connectionStatus.maxEntries = health.logs.maxEntries ?? connectionStatus.maxEntries;
        }
        if (health.connected && health.version && typeof chrome !== 'undefined') {
            const extVersion = chrome.runtime.getManifest().version;
            const serverMajor = health.version.split('.')[0];
            const extMajor = extVersion.split('.')[0];
            connectionStatus.serverVersion = health.version;
            connectionStatus.extensionVersion = extVersion;
            connectionStatus.versionMismatch = serverMajor !== extMajor;
        }
        communication.updateBadge(connectionStatus);
        if (wasConnected !== health.connected) {
            debugLog(DebugCategory.CONNECTION, health.connected ? 'Connected to server' : 'Disconnected from server', {
                entries: connectionStatus.entries,
                error: health.error || null,
                serverVersion: health.version || null,
            });
        }
        // Always start sync client - it handles failures gracefully with 1s retry
        // Don't gate on health check - sync client IS the connection mechanism
        startSyncClient();
        if (typeof chrome !== 'undefined' && chrome.runtime) {
            chrome.runtime
                .sendMessage({
                type: 'statusUpdate',
                status: { ...connectionStatus, aiControlled },
            })
                .catch((err) => console.error('[Gasoline] Error sending status update:', err));
        }
    }
    finally {
        _connectionCheckRunning = false;
    }
}
export function applyCaptureOverrides(overrides) {
    _captureOverrides = overrides;
    aiControlled = Object.keys(overrides).length > 0;
    if (overrides.log_level !== undefined) {
        currentLogLevel = overrides.log_level;
    }
    if (overrides.screenshot_on_error !== undefined) {
        screenshotOnError = overrides.screenshot_on_error === 'true';
    }
}
// =============================================================================
// STATUS PING (still used for tracked tab change notifications)
// =============================================================================
export async function sendStatusPingWrapper() {
    const trackingInfo = await eventListeners.getTrackedTabInfo();
    const statusMessage = {
        type: 'status',
        tracking_enabled: !!trackingInfo.trackedTabId,
        tracked_tab_id: trackingInfo.trackedTabId,
        tracked_tab_url: trackingInfo.trackedTabUrl,
        message: trackingInfo.trackedTabId ? 'tracking enabled' : 'no tab tracking enabled',
        extension_connected: true,
        timestamp: new Date().toISOString(),
    };
    await communication.sendStatusPing(serverUrl, statusMessage, diagnosticLog);
}
// =============================================================================
// SYNC CLIENT
// =============================================================================
/**
 * Get extension version safely
 */
function getExtensionVersion() {
    if (typeof chrome !== 'undefined' && chrome.runtime?.getManifest) {
        return chrome.runtime.getManifest().version;
    }
    return '';
}
/**
 * Start the sync client (unified /sync endpoint)
 */
function startSyncClient() {
    if (syncClient) {
        // Already running, nothing to do
        return;
    }
    syncClient = createSyncClient(serverUrl, EXTENSION_SESSION_ID, {
        // Handle commands from server
        onCommand: async (command) => {
            debugLog(DebugCategory.CONNECTION, 'Processing sync command', { type: command.type, id: command.id });
            if (stateManager.isQueryProcessing(command.id)) {
                debugLog(DebugCategory.CONNECTION, 'Skipping already processing command', { id: command.id });
                return;
            }
            stateManager.addProcessingQuery(command.id);
            try {
                await handlePendingQueryImpl(command);
            }
            catch (err) {
                debugLog(DebugCategory.CONNECTION, 'Error processing sync command', {
                    type: command.type,
                    error: err.message,
                });
            }
            finally {
                stateManager.removeProcessingQuery(command.id);
            }
        },
        // Handle connection state changes
        onConnectionChange: (connected) => {
            connectionStatus.connected = connected;
            communication.updateBadge(connectionStatus);
            debugLog(DebugCategory.CONNECTION, connected ? 'Sync connected' : 'Sync disconnected');
            // Notify popup
            if (typeof chrome !== 'undefined' && chrome.runtime) {
                chrome.runtime
                    .sendMessage({
                    type: 'statusUpdate',
                    status: { ...connectionStatus, aiControlled },
                })
                    .catch(() => {
                    /* popup may not be open */
                });
            }
        },
        // Handle capture overrides from server
        onCaptureOverrides: (overrides) => {
            applyCaptureOverrides(overrides);
        },
        // Get current settings to send to server
        getSettings: async () => {
            const trackingInfo = await eventListeners.getTrackedTabInfo();
            return {
                pilot_enabled: __aiWebPilotEnabledCache,
                tracking_enabled: !!trackingInfo.trackedTabId,
                tracked_tab_id: trackingInfo.trackedTabId || 0,
                tracked_tab_url: trackingInfo.trackedTabUrl || '',
                tracked_tab_title: trackingInfo.trackedTabTitle || '',
                capture_logs: true,
                capture_network: true,
                capture_websocket: true,
                capture_actions: true,
            };
        },
        // Get pending extension logs
        getExtensionLogs: () => {
            return extensionLogQueue.map((log) => ({
                timestamp: log.timestamp,
                level: log.level,
                message: log.message,
                source: log.source,
                category: log.category,
                data: log.data,
            }));
        },
        // Clear extension logs after sending
        clearExtensionLogs: () => {
            extensionLogQueue.length = 0;
        },
        // Debug logging
        debugLog: (category, message, data) => {
            debugLog(DebugCategory.CONNECTION, `[Sync] ${message}`, data);
        },
    }, getExtensionVersion());
    syncClient.start();
    debugLog(DebugCategory.CONNECTION, 'Sync client started');
}
/**
 * Stop the sync client
 */
function stopSyncClient() {
    if (syncClient) {
        syncClient.stop();
        debugLog(DebugCategory.CONNECTION, 'Sync client stopped');
    }
}
/**
 * Reset sync client connection (call when user enables pilot/tracking)
 */
export function resetSyncClientConnection() {
    if (syncClient) {
        syncClient.resetConnection();
        debugLog(DebugCategory.CONNECTION, 'Sync client connection reset');
    }
}
// =============================================================================
// AI WEB PILOT UTILITIES
// =============================================================================
/**
 * Reset pilot cache for testing
 */
export function _resetPilotCacheForTesting(value) {
    __aiWebPilotEnabledCache = value !== undefined ? value : false;
}
/**
 * Check if AI Web Pilot is enabled
 */
export function isAiWebPilotEnabled() {
    return __aiWebPilotEnabledCache === true;
}
// Re-export statically imported functions (Service Workers don't support dynamic import())
export const handlePendingQuery = handlePendingQueryImpl;
export const handlePilotCommand = handlePilotCommandImpl;
// Export snapshot/state management for backward compatibility
export { saveStateSnapshot, loadStateSnapshot, listStateSnapshots, deleteStateSnapshot };
//# sourceMappingURL=index.js.map