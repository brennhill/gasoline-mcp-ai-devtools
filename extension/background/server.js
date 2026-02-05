/**
 * @fileoverview Server Communication - HTTP functions for sending data to
 * the Gasoline server.
 */
import { getExtensionVersion } from './version-check.js';
/**
 * Get standard headers for API requests including version header
 */
function getRequestHeaders(additionalHeaders = {}) {
    return {
        'Content-Type': 'application/json',
        'X-Gasoline-Extension-Version': getExtensionVersion(),
        ...additionalHeaders,
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
        body: JSON.stringify({ entries }),
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
        body: JSON.stringify({ events }),
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
        body: JSON.stringify({ bodies }),
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
 * Send network waterfall data to server
 */
export async function sendNetworkWaterfallToServer(serverUrl, payload, debugLogFn) {
    if (debugLogFn)
        debugLogFn('connection', `Sending ${payload.entries.length} waterfall entries to server`);
    const response = await fetch(`${serverUrl}/network-waterfall`, {
        method: 'POST',
        headers: getRequestHeaders(),
        body: JSON.stringify(payload),
    });
    if (!response.ok) {
        const error = `Server error (network waterfall): ${response.status} ${response.statusText}`;
        if (debugLogFn)
            debugLogFn('error', error);
        throw new Error(error);
    }
    if (debugLogFn)
        debugLogFn('connection', `Server accepted ${payload.entries.length} waterfall entries`);
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
        body: JSON.stringify({ actions }),
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
        body: JSON.stringify({ snapshots }),
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
                error: 'Server returned invalid response - check Server URL in options',
            };
        }
        return {
            ...data,
            connected: true,
        };
    }
    catch (error) {
        return {
            connected: false,
            error: error.message,
        };
    }
}
/**
 * Update extension badge
 */
export function updateBadge(status) {
    if (typeof chrome === 'undefined' || !chrome.action)
        return;
    if (status.connected) {
        const errorCount = status.errorCount || 0;
        chrome.action.setBadgeText({
            text: errorCount === 0 ? '' : errorCount > 99 ? '99+' : String(errorCount),
        });
        chrome.action.setBadgeBackgroundColor({
            color: '#3fb950',
        });
    }
    else {
        chrome.action.setBadgeText({ text: '!' });
        chrome.action.setBadgeBackgroundColor({
            color: '#f85149',
        });
    }
}
/**
 * Post query results back to the server
 */
export async function postQueryResult(serverUrl, queryId, type, result, debugLogFn) {
    let endpoint;
    if (type === 'a11y') {
        endpoint = '/a11y-result';
    }
    else if (type === 'state') {
        endpoint = '/state-result';
    }
    else if (type === 'highlight') {
        endpoint = '/highlight-result';
    }
    else if (type === 'execute' || type === 'browser_action') {
        endpoint = '/execute-result';
    }
    else {
        endpoint = '/dom-result';
    }
    const logData = { queryId, type, endpoint, resultSize: JSON.stringify(result).length };
    if (debugLogFn)
        debugLogFn('api', `POST ${endpoint}`, logData);
    console.log(`[Gasoline API] POST ${endpoint}`, logData);
    try {
        const response = await fetch(`${serverUrl}${endpoint}`, {
            method: 'POST',
            headers: getRequestHeaders(),
            body: JSON.stringify({ id: queryId, result }),
        });
        if (!response.ok) {
            const errMsg = `Failed to post query result: HTTP ${response.status}`;
            if (debugLogFn)
                debugLogFn('api', errMsg, { queryId, type, endpoint });
            console.error(`[Gasoline API] ${errMsg}`, { queryId, type, endpoint });
        }
        else {
            if (debugLogFn)
                debugLogFn('api', `POST ${endpoint} success`, { queryId });
            console.log(`[Gasoline API] POST ${endpoint} success`, { queryId });
        }
    }
    catch (err) {
        const errMsg = err.message;
        if (debugLogFn)
            debugLogFn('api', `POST ${endpoint} error: ${errMsg}`, { queryId, type });
        console.error('[Gasoline API] Error posting query result:', { queryId, type, endpoint, error: errMsg });
    }
}
/**
 * POST async command result to server using correlation_id
 */
export async function postAsyncCommandResult(serverUrl, correlationId, status, result = null, error = null, debugLogFn) {
    const payload = {
        correlation_id: correlationId,
        status: status,
    };
    if (result !== null) {
        payload.result = result;
    }
    if (error !== null) {
        payload.error = error;
    }
    try {
        const response = await fetch(`${serverUrl}/execute-result`, {
            method: 'POST',
            headers: getRequestHeaders(),
            body: JSON.stringify(payload),
        });
        if (!response.ok) {
            console.error(`[Gasoline] Failed to post async command result: HTTP ${response.status}`, {
                correlationId,
                status,
            });
        }
    }
    catch (err) {
        console.error('[Gasoline] Error posting async command result:', {
            correlationId,
            status,
            error: err.message,
        });
        if (debugLogFn) {
            debugLogFn('connection', 'Failed to post async command result', {
                correlationId,
                status,
                error: err.message,
            });
        }
    }
}
// NOTE: postSettings and pollCaptureSettings removed - use /sync for all communication
/**
 * Post extension logs to server
 */
export async function postExtensionLogs(serverUrl, logs) {
    if (logs.length === 0)
        return;
    try {
        const response = await fetch(`${serverUrl}/extension-logs`, {
            method: 'POST',
            headers: getRequestHeaders(),
            body: JSON.stringify({ logs }),
        });
        if (!response.ok) {
            console.error(`[Gasoline] Failed to post extension logs: HTTP ${response.status}`, { count: logs.length });
        }
    }
    catch (err) {
        console.error('[Gasoline] Error posting extension logs:', { count: logs.length, error: err.message });
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
            body: JSON.stringify(statusMessage),
        });
        if (!response.ok) {
            console.error(`[Gasoline] Failed to send status ping: HTTP ${response.status}`, { type: statusMessage.type });
        }
    }
    catch (err) {
        console.error('[Gasoline] Error sending status ping:', { type: statusMessage.type, error: err.message });
        if (diagnosticLogFn) {
            diagnosticLogFn('[Gasoline] Status ping error: ' + err.message);
        }
    }
}
/**
 * Poll server for pending queries
 */
export async function pollPendingQueries(serverUrl, sessionId, pilotState, diagnosticLogFn, debugLogFn) {
    try {
        if (diagnosticLogFn) {
            diagnosticLogFn(`[Diagnostic] Poll request: header=${pilotState}`);
        }
        const response = await fetch(`${serverUrl}/pending-queries`, {
            headers: {
                ...getRequestHeaders({ 'X-Gasoline-Session': sessionId, 'X-Gasoline-Pilot': pilotState }),
            },
        });
        if (!response.ok) {
            if (debugLogFn)
                debugLogFn('connection', 'Poll pending-queries failed', { status: response.status });
            return [];
        }
        const data = (await response.json());
        if (!data.queries || data.queries.length === 0)
            return [];
        if (debugLogFn)
            debugLogFn('connection', 'Got pending queries', { count: data.queries.length });
        return data.queries;
    }
    catch (err) {
        if (debugLogFn)
            debugLogFn('connection', 'Poll pending-queries error', { error: err.message });
        return [];
    }
}
//# sourceMappingURL=server.js.map