/**
 * Purpose: HTTP functions for sending telemetry data (logs, WebSocket events, network bodies, actions, performance) to the Gasoline MCP server.
 * Docs: docs/features/feature/backend-log-streaming/index.md
 */
import { getExtensionVersion } from './version-check.js';
/**
 * Get standard headers for API requests including version header
 */
export function getRequestHeaders(additionalHeaders = {}) {
    return {
        'Content-Type': 'application/json',
        'X-Gasoline-Client': `gasoline-extension/${getExtensionVersion()}`,
        'X-Gasoline-Extension-Version': getExtensionVersion(),
        ...additionalHeaders
    };
}
/**
 * Send log entries to the server
 */
export async function sendLogsToServer(serverUrl, entries, debugLogFn) {
    if (debugLogFn)
        debugLogFn('connection', `Sending ${entries.length} entries to server`);
    const response = await fetch(`${serverUrl}/logs`, {
        method: 'POST',
        headers: getRequestHeaders(),
        body: JSON.stringify({ entries })
    });
    if (!response.ok) {
        const error = `Server error: ${response.status} ${response.statusText}`;
        if (debugLogFn)
            debugLogFn('error', error);
        throw new Error(error);
    }
    const result = (await response.json());
    if (debugLogFn)
        debugLogFn('connection', `Server accepted entries, total: ${result.entries}`);
    return result;
}
/**
 * Send WebSocket events to the server
 */
export async function sendWSEventsToServer(serverUrl, events, debugLogFn) {
    if (debugLogFn)
        debugLogFn('connection', `Sending ${events.length} WS events to server`);
    const response = await fetch(`${serverUrl}/websocket-events`, {
        method: 'POST',
        headers: getRequestHeaders(),
        body: JSON.stringify({ events })
    });
    if (!response.ok) {
        const error = `Server error (WS): ${response.status} ${response.statusText}`;
        if (debugLogFn)
            debugLogFn('error', error);
        throw new Error(error);
    }
    if (debugLogFn)
        debugLogFn('connection', `Server accepted ${events.length} WS events`);
}
/**
 * Send network bodies to the server
 */
export async function sendNetworkBodiesToServer(serverUrl, bodies, debugLogFn) {
    if (debugLogFn)
        debugLogFn('connection', `Sending ${bodies.length} network bodies to server`);
    const response = await fetch(`${serverUrl}/network-bodies`, {
        method: 'POST',
        headers: getRequestHeaders(),
        body: JSON.stringify({ bodies })
    });
    if (!response.ok) {
        const error = `Server error (network bodies): ${response.status} ${response.statusText}`;
        if (debugLogFn)
            debugLogFn('error', error);
        throw new Error(error);
    }
    if (debugLogFn)
        debugLogFn('connection', `Server accepted ${bodies.length} network bodies`);
}
/**
 * Send enhanced actions to server
 */
export async function sendEnhancedActionsToServer(serverUrl, actions, debugLogFn) {
    if (debugLogFn)
        debugLogFn('connection', `Sending ${actions.length} enhanced actions to server`);
    const response = await fetch(`${serverUrl}/enhanced-actions`, {
        method: 'POST',
        headers: getRequestHeaders(),
        body: JSON.stringify({ actions })
    });
    if (!response.ok) {
        const error = `Server error (enhanced actions): ${response.status} ${response.statusText}`;
        if (debugLogFn)
            debugLogFn('error', error);
        throw new Error(error);
    }
    if (debugLogFn)
        debugLogFn('connection', `Server accepted ${actions.length} enhanced actions`);
}
/**
 * Send performance snapshots to server
 */
export async function sendPerformanceSnapshotsToServer(serverUrl, snapshots, debugLogFn) {
    if (debugLogFn)
        debugLogFn('connection', `Sending ${snapshots.length} performance snapshots to server`);
    const response = await fetch(`${serverUrl}/performance-snapshots`, {
        method: 'POST',
        headers: getRequestHeaders(),
        body: JSON.stringify({ snapshots })
    });
    if (!response.ok) {
        const error = `Server error (performance snapshots): ${response.status} ${response.statusText}`;
        if (debugLogFn)
            debugLogFn('error', error);
        throw new Error(error);
    }
    if (debugLogFn)
        debugLogFn('connection', `Server accepted ${snapshots.length} performance snapshots`);
}
/**
 * Check server health
 */
export async function checkServerHealth(serverUrl) {
    try {
        const response = await fetch(`${serverUrl}/health`);
        if (!response.ok) {
            return { connected: false, error: `HTTP ${response.status}` };
        }
        let data;
        try {
            data = (await response.json());
        }
        catch {
            return {
                connected: false,
                error: 'Server returned invalid response - check Server URL in options'
            };
        }
        return {
            ...data,
            connected: true
        };
    }
    catch (error) {
        return {
            connected: false,
            error: error.message
        };
    }
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
 * Send status ping to server
 */
export async function sendStatusPing(serverUrl, statusMessage, diagnosticLogFn) {
    try {
        const response = await fetch(`${serverUrl}/api/extension-status`, {
            method: 'POST',
            headers: getRequestHeaders(),
            body: JSON.stringify(statusMessage)
        });
        if (!response.ok) {
            console.error(`[Gasoline] Failed to send status ping: HTTP ${response.status}`, { type: statusMessage.type }); // nosemgrep: javascript.lang.security.audit.unsafe-formatstring.unsafe-formatstring -- console.log with internal server state, not user-controlled format string
        }
    }
    catch (err) {
        console.error('[Gasoline] Error sending status ping:', { type: statusMessage.type, error: err.message });
        if (diagnosticLogFn) {
            diagnosticLogFn('[Gasoline] Status ping error: ' + err.message);
        }
    }
}
//# sourceMappingURL=server.js.map