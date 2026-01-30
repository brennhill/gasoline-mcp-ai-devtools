/**
 * @fileoverview Message Handlers Module
 * Handles messages from background script
 */
import { registerHighlightRequest, hasHighlightRequest, deleteHighlightRequest, registerExecuteRequest, registerA11yRequest, registerDomRequest, } from './request-tracking.js';
import { createDeferredPromise, promiseRaceWithCleanup, } from './timeout-utils.js';
// Feature toggle message types forwarded from background to inject.js
export const TOGGLE_MESSAGES = new Set([
    'setNetworkWaterfallEnabled',
    'setPerformanceMarksEnabled',
    'setActionReplayEnabled',
    'setWebSocketCaptureEnabled',
    'setWebSocketCaptureMode',
    'setPerformanceSnapshotEnabled',
    'setDeferralEnabled',
    'setNetworkBodyCaptureEnabled',
    'setServerUrl',
]);
/**
 * Security: Validate sender is from the extension background script
 * Prevents content script from trusting messages from compromised page context
 */
export function isValidBackgroundSender(sender) {
    // Messages from background should NOT have a tab (or have tab with chrome-extension:// url)
    // Messages from content scripts have tab.id
    // We only want messages from the background service worker
    return typeof sender.id === 'string' && sender.id === chrome.runtime.id;
}
/**
 * Create a timeout handler that cleans up a pending request from a Map
 */
function createRequestTimeoutCleanup(requestId, pendingMap, errorResponse) {
    return () => {
        if (pendingMap.has(requestId)) {
            const cb = pendingMap.get(requestId);
            pendingMap.delete(requestId);
            if (cb) {
                cb(errorResponse);
            }
        }
    };
}
/**
 * Forward a highlight message from background to inject.js
 */
export function forwardHighlightMessage(message) {
    const requestId = registerHighlightRequest((result) => deferred.resolve(result));
    const deferred = createDeferredPromise();
    // Post message to page context (inject.js)
    window.postMessage({
        type: 'GASOLINE_HIGHLIGHT_REQUEST',
        requestId,
        params: message.params,
    }, window.location.origin);
    // Timeout fallback + cleanup stale entries after 30 seconds
    return promiseRaceWithCleanup(deferred.promise, 30000, { success: false, error: 'timeout' }, () => {
        if (hasHighlightRequest(requestId)) {
            deleteHighlightRequest(requestId);
        }
    });
}
/**
 * Handle state capture/restore commands
 */
export async function handleStateCommand(params) {
    const { action, name, state, include_url } = params || {};
    // Create a promise to receive response from inject.js
    const messageId = `state_${Date.now()}_${Math.random().toString(36).slice(2)}`;
    const deferred = createDeferredPromise();
    // Set up listener for response from inject.js
    const responseHandler = (event) => {
        if (event.source !== window)
            return;
        if (event.data?.type === 'GASOLINE_STATE_RESPONSE' && event.data?.messageId === messageId) {
            window.removeEventListener('message', responseHandler);
            deferred.resolve(event.data.result || { error: 'No result from state command' });
        }
    };
    window.addEventListener('message', responseHandler);
    // Send command to inject.js (include state for restore action)
    window.postMessage({
        type: 'GASOLINE_STATE_COMMAND',
        messageId,
        action,
        name,
        state,
        include_url,
    }, window.location.origin);
    // Timeout after 5 seconds with cleanup
    return promiseRaceWithCleanup(deferred.promise, 5000, { error: 'State command timeout' }, () => window.removeEventListener('message', responseHandler));
}
/**
 * Handle GASOLINE_PING message
 */
export function handlePing(sendResponse) {
    sendResponse({ status: 'alive', timestamp: Date.now() });
    return true;
}
/**
 * Handle toggle messages
 */
export function handleToggleMessage(message) {
    if (!TOGGLE_MESSAGES.has(message.type))
        return;
    const payload = { type: 'GASOLINE_SETTING', setting: message.type };
    if (message.type === 'setWebSocketCaptureMode') {
        payload.mode = message.mode;
    }
    else if (message.type === 'setServerUrl') {
        payload.url = message.url;
    }
    else {
        payload.enabled = message.enabled;
    }
    // SECURITY: Use explicit targetOrigin (window.location.origin) not "*"
    window.postMessage(payload, window.location.origin);
}
/**
 * Handle GASOLINE_EXECUTE_JS message
 */
export function handleExecuteJs(params, sendResponse) {
    const requestId = registerExecuteRequest(sendResponse);
    // Timeout fallback: respond with error and cleanup after 30 seconds
    setTimeout(createRequestTimeoutCleanup(requestId, new Map([[requestId, sendResponse]]), { success: false, error: 'timeout', message: 'Execute request timed out after 30s' }), 30000);
    // Forward to inject.js via postMessage
    window.postMessage({
        type: 'GASOLINE_EXECUTE_JS',
        requestId,
        script: params.script || '',
        timeoutMs: params.timeout_ms || 5000,
    }, window.location.origin);
    return true;
}
/**
 * Handle GASOLINE_EXECUTE_QUERY message
 */
export function handleExecuteQuery(params, sendResponse) {
    // Parse params if it's a string (from JSON)
    let parsedParams = {};
    if (typeof params === 'string') {
        try {
            parsedParams = JSON.parse(params);
        }
        catch {
            parsedParams = {};
        }
    }
    else if (typeof params === 'object') {
        parsedParams = params;
    }
    return handleExecuteJs(parsedParams, sendResponse);
}
/**
 * Handle A11Y_QUERY message
 */
export function handleA11yQuery(params, sendResponse) {
    // Parse params if it's a string (from JSON)
    let parsedParams = {};
    if (typeof params === 'string') {
        try {
            parsedParams = JSON.parse(params);
        }
        catch {
            parsedParams = {};
        }
    }
    else if (typeof params === 'object') {
        parsedParams = params;
    }
    const requestId = registerA11yRequest(sendResponse);
    // Timeout fallback: respond with error and cleanup after 30 seconds
    setTimeout(createRequestTimeoutCleanup(requestId, new Map([[requestId, sendResponse]]), { error: 'Accessibility audit timeout' }), 30000);
    // Forward to inject.js via postMessage
    window.postMessage({
        type: 'GASOLINE_A11Y_QUERY',
        requestId,
        params: parsedParams,
    }, window.location.origin);
    return true;
}
/**
 * Handle DOM_QUERY message
 */
export function handleDomQuery(params, sendResponse) {
    // Parse params if it's a string (from JSON)
    let parsedParams = {};
    if (typeof params === 'string') {
        try {
            parsedParams = JSON.parse(params);
        }
        catch {
            parsedParams = {};
        }
    }
    else if (typeof params === 'object') {
        parsedParams = params;
    }
    const requestId = registerDomRequest(sendResponse);
    // Timeout fallback: respond with error and cleanup after 30 seconds
    setTimeout(createRequestTimeoutCleanup(requestId, new Map([[requestId, sendResponse]]), { error: 'DOM query timeout' }), 30000);
    // Forward to inject.js via postMessage
    window.postMessage({
        type: 'GASOLINE_DOM_QUERY',
        requestId,
        params: parsedParams,
    }, window.location.origin);
    return true;
}
/**
 * Handle GET_NETWORK_WATERFALL message
 */
export function handleGetNetworkWaterfall(sendResponse) {
    const requestId = Date.now();
    const deferred = createDeferredPromise();
    // Set up a one-time listener for the response
    const responseHandler = (event) => {
        if (event.source !== window)
            return;
        if (event.data?.type === 'GASOLINE_WATERFALL_RESPONSE') {
            window.removeEventListener('message', responseHandler);
            deferred.resolve({ entries: event.data.entries || [] });
        }
    };
    window.addEventListener('message', responseHandler);
    // Post message to page context
    window.postMessage({
        type: 'GASOLINE_GET_WATERFALL',
        requestId,
    }, window.location.origin);
    // Timeout fallback: respond with empty array after 5 seconds
    promiseRaceWithCleanup(deferred.promise, 5000, { entries: [] }, () => {
        window.removeEventListener('message', responseHandler);
    }).then((result) => {
        sendResponse(result);
    });
    return true;
}
//# sourceMappingURL=message-handlers.js.map