/**
 * Purpose: Handles content-script message relay between background and inject contexts.
 * Docs: docs/features/feature/interact-explore/index.md
 * Docs: docs/features/feature/query-dom/index.md
 */
import { registerHighlightRequest, hasHighlightRequest, deleteHighlightRequest, registerExecuteRequest, registerA11yRequest, registerDomRequest } from './request-tracking.js';
import { createDeferredPromise, promiseRaceWithCleanup } from './timeout-utils.js';
import { isInjectScriptLoaded, getPageNonce } from './script-injection.js';
import { ASYNC_COMMAND_TIMEOUT_MS } from '../lib/constants.js';
/** Send a nonce-authenticated message to inject.js (MAIN world) */
function postToInject(data) {
    window.postMessage({ ...data, _nonce: getPageNonce() }, window.location.origin);
}
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
    'setServerUrl'
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
    postToInject({
        type: 'GASOLINE_HIGHLIGHT_REQUEST',
        requestId,
        params: message.params
    });
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
    postToInject({
        type: 'GASOLINE_STATE_COMMAND',
        messageId,
        action,
        name,
        state,
        include_url
    });
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
    window.postMessage({ ...payload, _nonce: getPageNonce() }, window.location.origin);
}
/**
 * Execute JS in the MAIN world via inject script, with safety timeout.
 */
function executeInMainWorld(params, sendResponse) {
    const timeoutMs = params.timeout_ms || 5000;
    const requestId = registerExecuteRequest(sendResponse);
    // Safety timeout: user's timeout + 2s buffer (NOT fixed 30s)
    // If inject script responds, its own timeout handles slow scripts.
    // This only fires if inject script never responds at all.
    const safetyTimeoutMs = timeoutMs + 2000;
    setTimeout(createRequestTimeoutCleanup(requestId, new Map([[requestId, sendResponse]]), {
        success: false,
        error: 'inject_not_responding',
        message: `Inject script did not respond within ${safetyTimeoutMs}ms. The tab may not be tracked or the inject script failed to load.`
    }), safetyTimeoutMs);
    postToInject({
        type: 'GASOLINE_EXECUTE_JS',
        requestId,
        script: params.script || '',
        timeoutMs
    });
}
/**
 * Handle GASOLINE_EXECUTE_JS message.
 * Always executes in MAIN world via inject script.
 * Returns inject_not_loaded error if inject script isn't available,
 * so background can fallback to chrome.scripting API.
 */
export function handleExecuteJs(params, sendResponse) {
    if (!isInjectScriptLoaded()) {
        sendResponse({
            success: false,
            error: 'inject_not_loaded',
            message: 'Inject script not loaded in page context. Tab may not be tracked.'
        });
        return true;
    }
    executeInMainWorld(params, sendResponse);
    return true;
}
/**
 * Handle GASOLINE_EXECUTE_QUERY message (async command path)
 */
export function handleExecuteQuery(params, sendResponse) {
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
    // Timeout fallback: respond with error and cleanup after async command timeout
    setTimeout(createRequestTimeoutCleanup(requestId, new Map([[requestId, sendResponse]]), {
        error: 'Accessibility audit timeout'
    }), ASYNC_COMMAND_TIMEOUT_MS);
    // Forward to inject.js via postMessage
    postToInject({
        type: 'GASOLINE_A11Y_QUERY',
        requestId,
        params: parsedParams
    });
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
    // Timeout fallback: respond with error and cleanup after async command timeout
    setTimeout(createRequestTimeoutCleanup(requestId, new Map([[requestId, sendResponse]]), { error: 'DOM query timeout' }), ASYNC_COMMAND_TIMEOUT_MS);
    // Forward to inject.js via postMessage
    postToInject({
        type: 'GASOLINE_DOM_QUERY',
        requestId,
        params: parsedParams
    });
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
    postToInject({
        type: 'GASOLINE_GET_WATERFALL',
        requestId
    });
    // Timeout fallback: respond with empty array after 5 seconds
    promiseRaceWithCleanup(deferred.promise, 5000, { entries: [] }, () => {
        window.removeEventListener('message', responseHandler);
    }).then((result) => {
        sendResponse(result);
    }, () => {
        sendResponse({ entries: [] });
    });
    return true;
}
/**
 * Handle LINK_HEALTH_QUERY message
 */
export function handleLinkHealthQuery(params, sendResponse) {
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
    const requestId = Date.now();
    const deferred = createDeferredPromise();
    // Set up a one-time listener for the response
    const responseHandler = (event) => {
        if (event.source !== window)
            return;
        if (event.data?.type === 'GASOLINE_LINK_HEALTH_RESPONSE') {
            window.removeEventListener('message', responseHandler);
            deferred.resolve(event.data.result || { error: 'No result from link health check' });
        }
    };
    window.addEventListener('message', responseHandler);
    // Forward to inject.js via postMessage
    postToInject({
        type: 'GASOLINE_LINK_HEALTH_QUERY',
        requestId,
        params: parsedParams
    });
    // Timeout fallback: respond with error after async command timeout window
    promiseRaceWithCleanup(deferred.promise, ASYNC_COMMAND_TIMEOUT_MS, { error: 'Link health check timeout' }, () => {
        window.removeEventListener('message', responseHandler);
    }).then((result) => {
        sendResponse(result);
    }, () => {
        sendResponse({ error: 'Link health check failed' });
    });
    return true;
}
//# sourceMappingURL=message-handlers.js.map