/**
 * Purpose: Handles extension background coordination and message routing.
 * Why: Centralizes extension coordination to reduce race conditions and split-brain state.
 * Docs: docs/features/feature/analyze-tool/index.md
 * Docs: docs/features/feature/interact-explore/index.md
 * Docs: docs/features/feature/observe/index.md
 */
import { getOrCreateTransportProvider, getRequestHeaders } from './transport-provider.js';
/**
 * Get standard headers for API requests including version header
 */
export { getRequestHeaders };
/**
 * Send log entries to the server
 */
export async function sendLogsToServer(serverUrl, entries, debugLogFn) {
    return getOrCreateTransportProvider(serverUrl).postLogs(entries, undefined, debugLogFn);
}
/**
 * Send WebSocket events to the server
 */
export async function sendWSEventsToServer(serverUrl, events, debugLogFn) {
    return getOrCreateTransportProvider(serverUrl).postWebSocketEvents(events, undefined, debugLogFn);
}
/**
 * Send network bodies to the server
 */
export async function sendNetworkBodiesToServer(serverUrl, bodies, debugLogFn) {
    return getOrCreateTransportProvider(serverUrl).postNetworkBodies(bodies, undefined, debugLogFn);
}
/**
 * Send network waterfall data to server
 */
export async function sendNetworkWaterfallToServer(serverUrl, payload, debugLogFn) {
    return getOrCreateTransportProvider(serverUrl).postNetworkWaterfall(payload, undefined, debugLogFn);
}
/**
 * Send enhanced actions to server
 */
export async function sendEnhancedActionsToServer(serverUrl, actions, debugLogFn) {
    return getOrCreateTransportProvider(serverUrl).postEnhancedActions(actions, undefined, debugLogFn);
}
/**
 * Send performance snapshots to server
 */
export async function sendPerformanceSnapshotsToServer(serverUrl, snapshots, debugLogFn) {
    return getOrCreateTransportProvider(serverUrl).postPerformanceSnapshots(snapshots, undefined, debugLogFn);
}
/**
 * Check server health
 */
export async function checkServerHealth(serverUrl) {
    return getOrCreateTransportProvider(serverUrl).checkHealth();
}
/**
 * Update extension badge.
 * Uses Promise.all to ensure both text and color are applied atomically
 * before the MV3 service worker can be suspended.
 */
export function updateBadge(status) {
    if (typeof chrome === 'undefined' || !chrome.action)
        return;
    if (status.connected) {
        const errorCount = status.errorCount || 0;
        Promise.all([
            chrome.action.setBadgeText({
                text: errorCount === 0 ? '' : errorCount > 99 ? '99+' : String(errorCount)
            }),
            chrome.action.setBadgeBackgroundColor({
                color: '#3fb950'
            })
        ]).catch(() => {
            /* badge update failed — SW may be shutting down */
        });
    }
    else {
        Promise.all([
            chrome.action.setBadgeText({ text: '!' }),
            chrome.action.setBadgeBackgroundColor({
                color: '#f85149'
            })
        ]).catch(() => {
            /* badge update failed — SW may be shutting down */
        });
    }
}
/**
 * Post query results back to the server
 */
export async function postQueryResult(serverUrl, queryId, type, result, debugLogFn) {
    return getOrCreateTransportProvider(serverUrl).postQueryResult(queryId, type, result, undefined, debugLogFn);
}
/**
 * POST async command result to server using correlation_id
 */
export async function postAsyncCommandResult(serverUrl, correlationId, status, result = null, error = null, debugLogFn) {
    return getOrCreateTransportProvider(serverUrl).postAsyncCommandResult(correlationId, status, result, error, undefined, debugLogFn);
}
// NOTE: postSettings and pollCaptureSettings removed - use /sync for all communication
/**
 * Post extension logs to server
 */
export async function postExtensionLogs(serverUrl, logs) {
    return getOrCreateTransportProvider(serverUrl).postExtensionLogs(logs);
}
/**
 * Send status ping to server
 */
export async function sendStatusPing(serverUrl, statusMessage, diagnosticLogFn) {
    return getOrCreateTransportProvider(serverUrl).postStatusPing(statusMessage, undefined, diagnosticLogFn);
}
/**
 * Poll server for pending queries
 */
export async function pollPendingQueries(serverUrl, extSessionId, pilotState, diagnosticLogFn, debugLogFn) {
    return getOrCreateTransportProvider(serverUrl).pollPendingQueries(extSessionId, pilotState, undefined, diagnosticLogFn, debugLogFn);
}
//# sourceMappingURL=server.js.map
