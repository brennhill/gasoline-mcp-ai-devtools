/**
 * @fileoverview Polling - Handles all polling loops including query polling,
 * settings heartbeat, waterfall posting, extension logs, and status pings.
 */
// =============================================================================
// CONSTANTS
// =============================================================================
/** Query polling interval in milliseconds */
const QUERY_POLLING_INTERVAL_MS = 1000;
/** Settings heartbeat interval in milliseconds */
const SETTINGS_HEARTBEAT_INTERVAL_MS = 2000;
/** Network waterfall posting interval in milliseconds */
const WATERFALL_POSTING_INTERVAL_MS = 10000;
/** Extension logs posting interval in milliseconds */
const EXTENSION_LOGS_INTERVAL_MS = 5000;
/** Status ping interval in milliseconds */
const STATUS_PING_INTERVAL_MS = 30000;
// =============================================================================
// STATE
// =============================================================================
let queryPollingInterval = null;
let settingsHeartbeatInterval = null;
let waterfallPostingInterval = null;
let extensionLogsInterval = null;
let statusPingInterval = null;
// =============================================================================
// QUERY POLLING
// =============================================================================
/**
 * Start polling for pending queries at 1-second intervals
 */
export function startQueryPolling(pollFn, debugLogFn) {
    stopQueryPolling();
    if (debugLogFn)
        debugLogFn('connection', 'Starting query polling');
    queryPollingInterval = setInterval(pollFn, QUERY_POLLING_INTERVAL_MS);
}
/**
 * Stop polling for pending queries
 */
export function stopQueryPolling() {
    if (queryPollingInterval) {
        clearInterval(queryPollingInterval);
        queryPollingInterval = null;
    }
}
/**
 * Check if query polling is active
 */
export function isQueryPollingActive() {
    return queryPollingInterval !== null;
}
// =============================================================================
// SETTINGS HEARTBEAT
// =============================================================================
/**
 * Start settings heartbeat: POST /settings every 2 seconds
 */
export function startSettingsHeartbeat(postSettingsFn, debugLogFn) {
    stopSettingsHeartbeat();
    if (debugLogFn)
        debugLogFn('connection', 'Starting settings heartbeat');
    // Post immediately, then every 2 seconds
    postSettingsFn();
    settingsHeartbeatInterval = setInterval(postSettingsFn, SETTINGS_HEARTBEAT_INTERVAL_MS);
}
/**
 * Stop settings heartbeat
 */
export function stopSettingsHeartbeat(debugLogFn) {
    if (settingsHeartbeatInterval) {
        clearInterval(settingsHeartbeatInterval);
        settingsHeartbeatInterval = null;
        if (debugLogFn)
            debugLogFn('connection', 'Stopped settings heartbeat');
    }
}
/**
 * Check if settings heartbeat is active
 */
export function isSettingsHeartbeatActive() {
    return settingsHeartbeatInterval !== null;
}
// =============================================================================
// NETWORK WATERFALL POSTING
// =============================================================================
/**
 * Start network waterfall posting: POST /network-waterfall every 10 seconds
 */
export function startWaterfallPosting(postWaterfallFn, debugLogFn) {
    stopWaterfallPosting();
    if (debugLogFn)
        debugLogFn('connection', 'Starting waterfall posting');
    // Post immediately, then every 10 seconds
    postWaterfallFn();
    waterfallPostingInterval = setInterval(postWaterfallFn, WATERFALL_POSTING_INTERVAL_MS);
}
/**
 * Stop network waterfall posting
 */
export function stopWaterfallPosting(debugLogFn) {
    if (waterfallPostingInterval) {
        clearInterval(waterfallPostingInterval);
        waterfallPostingInterval = null;
        if (debugLogFn)
            debugLogFn('connection', 'Stopped waterfall posting');
    }
}
/**
 * Check if waterfall posting is active
 */
export function isWaterfallPostingActive() {
    return waterfallPostingInterval !== null;
}
// =============================================================================
// EXTENSION LOGS POSTING
// =============================================================================
/**
 * Start extension logs posting: POST /extension-logs every 5 seconds
 */
export function startExtensionLogsPosting(postLogsFn) {
    stopExtensionLogsPosting();
    // Post every 5 seconds (batch logs to reduce overhead)
    extensionLogsInterval = setInterval(postLogsFn, EXTENSION_LOGS_INTERVAL_MS);
}
/**
 * Stop extension logs posting
 */
export function stopExtensionLogsPosting() {
    if (extensionLogsInterval) {
        clearInterval(extensionLogsInterval);
        extensionLogsInterval = null;
    }
}
/**
 * Check if extension logs posting is active
 */
export function isExtensionLogsPostingActive() {
    return extensionLogsInterval !== null;
}
// =============================================================================
// STATUS PING
// =============================================================================
/**
 * Start status ping: POST /api/extension-status every 30 seconds
 */
export function startStatusPing(sendPingFn) {
    stopStatusPing();
    sendPingFn(); // Send immediately on start
    statusPingInterval = setInterval(sendPingFn, STATUS_PING_INTERVAL_MS);
}
/**
 * Stop status ping
 */
export function stopStatusPing() {
    if (statusPingInterval) {
        clearInterval(statusPingInterval);
        statusPingInterval = null;
    }
}
/**
 * Check if status ping is active
 */
export function isStatusPingActive() {
    return statusPingInterval !== null;
}
// =============================================================================
// CLEANUP
// =============================================================================
/**
 * Stop all polling loops
 */
export function stopAllPolling() {
    stopQueryPolling();
    stopSettingsHeartbeat();
    stopWaterfallPosting();
    stopExtensionLogsPosting();
    stopStatusPing();
}
//# sourceMappingURL=polling.js.map