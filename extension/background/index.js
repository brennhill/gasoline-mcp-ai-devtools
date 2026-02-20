/**
 * Purpose: Handles extension background coordination and message routing.
 * Docs: docs/features/feature/analyze-tool/index.md
 * Docs: docs/features/feature/interact-explore/index.md
 * Docs: docs/features/feature/observe/index.md
 */
import { debugMode, connectionStatus, extensionLogQueue, currentLogLevel, screenshotOnError, _setDebugModeRaw, serverUrl, setConnectionStatus, _connectionCheckRunning, setConnectionCheckRunning, EXTENSION_SESSION_ID, aiControlled, __aiWebPilotEnabledCache, applyCaptureOverrides } from './state.js';
import { addDebugLogEntry, getDebugLog as getDebugLogEntries, clearDebugLog as clearDebugLogEntries, isSourceMapEnabled, resolveStackTrace, processErrorGroup, canTakeScreenshot, recordScreenshot } from './state-manager.js';
import { createCircuitBreaker, RATE_LIMIT_CONFIG, shouldCaptureLog, formatLogEntry, captureScreenshot, updateBadge, checkServerHealth, sendStatusPing } from './communication.js';
import { getTrackedTabInfo } from './event-listeners.js';
import { DebugCategory } from './debug.js';
import { getRequestHeaders } from './server.js';
import { saveStateSnapshot, loadStateSnapshot, listStateSnapshots, deleteStateSnapshot } from './message-handlers.js';
import { handlePendingQuery as handlePendingQueryImpl, handlePilotCommand as handlePilotCommandImpl } from './pending-queries.js';
import { updateVersionFromHealth } from './version-check.js';
import { createBatcherInstances } from './batcher-instances.js';
import { startSyncClient as startSyncClientImpl, resetSyncClientConnection as resetSyncClientConnectionImpl } from './sync-manager.js';
// Re-export for consumers that already import from here
export { DEFAULT_SERVER_URL } from '../lib/constants.js';
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
        ...(data !== null ? { data } : {})
    };
    addDebugLogEntry(entry);
    if (connectionStatus.connected) {
        extensionLogQueue.push({
            timestamp,
            level: 'debug',
            message,
            source: 'background',
            category,
            ...(data !== null ? { data } : {})
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
            console.log(prefix, message, data); // nosemgrep: javascript.lang.security.audit.unsafe-formatstring.unsafe-formatstring -- console.log with internal error message, not user-controlled
        }
        else {
            console.log(prefix, message); // nosemgrep: javascript.lang.security.audit.unsafe-formatstring.unsafe-formatstring -- console.log with internal error message, not user-controlled
        }
    }
}
/**
 * Get all debug log entries
 */
export function getDebugLog() {
    return getDebugLogEntries();
}
/**
 * Clear debug log buffer
 */
export function clearDebugLog() {
    clearDebugLogEntries();
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
            sourceMapEnabled: isSourceMapEnabled()
        },
        entries: getDebugLogEntries()
    }, null, 2);
}
/**
 * Set debug mode enabled/disabled
 */
export function setDebugMode(enabled) {
    _setDebugModeRaw(enabled);
    debugLog(DebugCategory.SETTINGS, `Debug mode ${enabled ? 'enabled' : 'disabled'}`);
}
// =============================================================================
// SHARED CIRCUIT BREAKER
// =============================================================================
export const sharedServerCircuitBreaker = createCircuitBreaker(() => Promise.reject(new Error('shared circuit breaker')), {
    maxFailures: RATE_LIMIT_CONFIG.maxFailures,
    resetTimeout: RATE_LIMIT_CONFIG.resetTimeout,
    initialBackoff: 0,
    maxBackoff: 0
});
// =============================================================================
// BATCHERS (delegated to batcher-instances.ts)
// =============================================================================
const _batchers = createBatcherInstances({
    getServerUrl: () => serverUrl,
    getConnectionStatus: () => connectionStatus,
    setConnectionStatus: (patch) => {
        setConnectionStatus(patch);
    },
    debugLog
}, sharedServerCircuitBreaker);
export const logBatcherWithCB = _batchers.logBatcherWithCB;
export const logBatcher = _batchers.logBatcher;
export const wsBatcherWithCB = _batchers.wsBatcherWithCB;
export const wsBatcher = _batchers.wsBatcher;
export const enhancedActionBatcherWithCB = _batchers.enhancedActionBatcherWithCB;
export const enhancedActionBatcher = _batchers.enhancedActionBatcher;
export const networkBodyBatcherWithCB = _batchers.networkBodyBatcherWithCB;
export const networkBodyBatcher = _batchers.networkBodyBatcher;
export const perfBatcherWithCB = _batchers.perfBatcherWithCB;
export const perfBatcher = _batchers.perfBatcher;
// =============================================================================
// LOG HANDLING
// =============================================================================
async function tryResolveSourceMap(entry) {
    if (!isSourceMapEnabled())
        return entry;
    if (!entry.stack)
        return entry;
    try {
        const resolvedStack = await resolveStackTrace(entry.stack, debugLog);
        const existingEnrichments = entry._enrichments;
        const enrichments = existingEnrichments ? [...existingEnrichments] : [];
        if (!enrichments.includes('sourceMap')) {
            enrichments.push('sourceMap');
        }
        debugLog(DebugCategory.CAPTURE, 'Stack trace resolved via source map');
        return {
            ...entry,
            stack: resolvedStack,
            _sourceMapResolved: true,
            _enrichments: enrichments
        };
    }
    catch (err) {
        debugLog(DebugCategory.ERROR, 'Source map resolution failed', { error: err.message });
        return entry;
    }
}
export async function handleLogMessage(payload, sender, tabId) {
    if (!shouldCaptureLog(payload.level, currentLogLevel, payload.type)) {
        debugLog(DebugCategory.CAPTURE, `Log filtered out: level=${payload.level}, type=${payload.type}` // nosemgrep: missing-template-string-indicator
        );
        return;
    }
    let entry = formatLogEntry(payload);
    const resolvedTabId = tabId ?? sender?.tab?.id;
    if (resolvedTabId !== null && resolvedTabId !== undefined) {
        entry = { ...entry, tabId: resolvedTabId };
    }
    // nosemgrep: missing-template-string-indicator
    debugLog(DebugCategory.CAPTURE, `Log received: type=${entry.type}, level=${entry.level}`, {
        url: entry.url,
        enrichments: entry._enrichments
    });
    entry = await tryResolveSourceMap(entry);
    const { shouldSend, entry: processedEntry } = processErrorGroup(entry);
    if (shouldSend && processedEntry) {
        logBatcher.add(processedEntry);
        // nosemgrep: missing-template-string-indicator
        debugLog(DebugCategory.CAPTURE, `Log queued for server: type=${processedEntry.type}`, {
            aggregatedCount: processedEntry._aggregatedCount
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
    const result = await captureScreenshot(sender.tab.id, serverUrl, errorId, entryType || null, canTakeScreenshot, recordScreenshot, debugLog);
    if (result.success && result.entry) {
        logBatcher.add(result.entry);
    }
}
export async function handleClearLogs() {
    try {
        await fetch(`${serverUrl}/logs`, { method: 'DELETE', headers: getRequestHeaders() });
        setConnectionStatus({ entries: 0, errorCount: 0 });
        updateBadge(connectionStatus);
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
// #lizard forgives
function updateVersionFromHealthSafe(health) {
    try {
        updateVersionFromHealth({ version: health.version, availableVersion: health.availableVersion }, debugLog);
    }
    catch (err) {
        debugLog(DebugCategory.CONNECTION, 'Failed to update version info', { error: err.message });
    }
}
function applyHealthLogs(health) {
    if (!health.logs)
        return;
    setConnectionStatus({
        logFile: health.logs.logFile || connectionStatus.logFile,
        logFileSize: health.logs.logFileSize,
        entries: health.logs.entries ?? connectionStatus.entries,
        maxEntries: health.logs.maxEntries ?? connectionStatus.maxEntries
    });
}
function applyVersionMismatchCheck(health) {
    if (!health.connected || !health.version || typeof chrome === 'undefined')
        return;
    const extVersion = chrome.runtime.getManifest().version;
    setConnectionStatus({
        serverVersion: health.version,
        extensionVersion: extVersion,
        versionMismatch: health.version.split('.')[0] !== extVersion.split('.')[0]
    });
}
function logConnectionChange(wasConnected, health) {
    if (wasConnected === health.connected)
        return;
    debugLog(DebugCategory.CONNECTION, health.connected ? 'Connected to server' : 'Disconnected from server', {
        entries: connectionStatus.entries,
        error: health.error || null,
        serverVersion: health.version || null
    });
}
function broadcastStatusUpdate() {
    if (typeof chrome === 'undefined' || !chrome.runtime)
        return;
    chrome.runtime
        .sendMessage({ type: 'statusUpdate', status: { ...connectionStatus, aiControlled } })
        .catch((err) => console.error('[Gasoline] Error sending status update:', err));
}
// eslint-disable-next-line security-node/detect-unhandled-async-errors
export async function checkConnectionAndUpdate() {
    if (_connectionCheckRunning) {
        debugLog(DebugCategory.CONNECTION, 'Skipping connection check - already running');
        return;
    }
    setConnectionCheckRunning(true);
    try {
        const health = await checkServerHealth(serverUrl);
        const wasConnected = connectionStatus.connected;
        if (health.connected) {
            updateVersionFromHealthSafe(health);
        }
        setConnectionStatus({ ...health, connected: health.connected });
        applyHealthLogs(health);
        applyVersionMismatchCheck(health);
        updateBadge(connectionStatus);
        logConnectionChange(wasConnected, health);
        // Always start sync client - it handles failures gracefully with 1s retry
        startSyncClientImpl(syncManagerDeps);
        broadcastStatusUpdate();
    }
    finally {
        setConnectionCheckRunning(false);
    }
}
// =============================================================================
// STATUS PING (still used for tracked tab change notifications)
// =============================================================================
export async function sendStatusPingWrapper() {
    const trackingInfo = await getTrackedTabInfo();
    const statusMessage = {
        type: 'status',
        tracking_enabled: !!trackingInfo.trackedTabId,
        tracked_tab_id: trackingInfo.trackedTabId,
        tracked_tab_url: trackingInfo.trackedTabUrl,
        message: trackingInfo.trackedTabId ? 'tracking enabled' : 'no tab tracking enabled',
        extension_connected: true,
        timestamp: new Date().toISOString()
    };
    await sendStatusPing(serverUrl, statusMessage, diagnosticLog);
}
// =============================================================================
// SYNC CLIENT (delegated to sync-manager.ts)
// =============================================================================
/** Shared deps object for sync-manager — created once, closures read live state */
const syncManagerDeps = {
    getServerUrl: () => serverUrl,
    getExtSessionId: () => EXTENSION_SESSION_ID,
    getConnectionStatus: () => connectionStatus,
    setConnectionStatus: (patch) => {
        setConnectionStatus(patch);
    },
    getAiControlled: () => aiControlled,
    getAiWebPilotEnabledCache: () => __aiWebPilotEnabledCache,
    getExtensionLogQueue: () => extensionLogQueue,
    clearExtensionLogQueue: () => {
        extensionLogQueue.length = 0;
    },
    applyCaptureOverrides,
    debugLog
};
/**
 * Reset sync client connection (call when user enables pilot/tracking)
 */
export function resetSyncClientConnection() {
    resetSyncClientConnectionImpl(debugLog);
}
// Re-export statically imported functions (Service Workers don't support dynamic import())
export const handlePendingQuery = handlePendingQueryImpl;
export const handlePilotCommand = handlePilotCommandImpl;
// Export snapshot/state management for backward compatibility
export { saveStateSnapshot, loadStateSnapshot, listStateSnapshots, deleteStateSnapshot };
//# sourceMappingURL=index.js.map