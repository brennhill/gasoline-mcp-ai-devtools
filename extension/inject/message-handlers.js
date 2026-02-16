/**
 * @fileoverview Message Handlers - Handles messages from content script including
 * settings, state management, JavaScript execution, and DOM/accessibility queries.
 */
import { createDeferredPromise } from '../lib/timeout-utils.js';
import { executeDOMQuery, runAxeAuditWithTimeout } from '../lib/dom-queries.js';
import { checkLinkHealth } from '../lib/link-health.js';
import { getNetworkWaterfall, setNetworkWaterfallEnabled, setNetworkBodyCaptureEnabled, setServerUrl } from '../lib/network.js';
import { setPerformanceMarksEnabled, installPerformanceCapture, uninstallPerformanceCapture } from '../lib/performance.js';
import { setActionCaptureEnabled } from '../lib/actions.js';
import { setWebSocketCaptureEnabled, setWebSocketCaptureMode, installWebSocketCapture, uninstallWebSocketCapture } from '../lib/websocket.js';
import { setPerformanceSnapshotEnabled } from '../lib/perf-snapshot.js';
import { setDeferralEnabled } from './observers.js';
/** Read the page nonce set by the content script on the inject script element */
let pageNonce = '';
if (typeof document !== 'undefined' && typeof document.querySelector === 'function') {
    const nonceEl = document.querySelector('script[data-gasoline-nonce]');
    if (nonceEl) {
        pageNonce = nonceEl.getAttribute('data-gasoline-nonce') || '';
    }
}
/**
 * Valid setting names from content script
 */
const VALID_SETTINGS = new Set([
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
const VALID_STATE_ACTIONS = new Set(['capture', 'restore']);
/**
 * Safe serialization for complex objects returned from executeJavaScript.
 */
// #lizard forgives
function serializeObject(obj, depth, seen) {
    if (seen.has(obj))
        return '[Circular]';
    seen.add(obj);
    if (Array.isArray(obj))
        return obj.slice(0, 100).map((v) => safeSerializeForExecute(v, depth + 1, seen));
    if (obj instanceof Error)
        return { error: obj.message, stack: obj.stack };
    if (obj instanceof Date)
        return obj.toISOString();
    if (obj instanceof RegExp)
        return obj.toString();
    if (typeof Node !== 'undefined' && obj instanceof Node) {
        const node = obj;
        return `[${node.nodeName}${node.id ? '#' + node.id : ''}]`;
    }
    const result = {};
    const keys = Object.keys(obj).slice(0, 50);
    for (const key of keys) {
        try {
            result[key] = safeSerializeForExecute(obj[key], depth + 1, seen);
        }
        catch {
            result[key] = '[unserializable]';
        }
    }
    if (Object.keys(obj).length > 50) {
        result['...'] = `[${Object.keys(obj).length - 50} more keys]`;
    }
    return result;
}
export function safeSerializeForExecute(value, depth = 0, seen = new WeakSet()) {
    if (depth > 10)
        return '[max depth exceeded]';
    if (value === null || value === undefined)
        return value;
    const type = typeof value;
    if (type === 'string' || type === 'number' || type === 'boolean')
        return value;
    if (type === 'function')
        return `[Function: ${value.name || 'anonymous'}]`;
    if (type === 'symbol')
        return value.toString();
    if (type === 'object')
        return serializeObject(value, depth, seen);
    return String(value);
}
/**
 * Execute arbitrary JavaScript in the page context with timeout handling.
 */
export function executeJavaScript(script, timeoutMs = 5000) {
    const deferred = createDeferredPromise();
    // #lizard forgives
    const executeWithTimeoutProtection = async () => {
        const timeoutHandle = setTimeout(() => {
            deferred.resolve({
                success: false,
                error: 'execution_timeout',
                message: `Script exceeded ${timeoutMs}ms timeout. RECOMMENDED ACTIONS:

1. Check for infinite loops or blocking operations in your script
2. Break the task into smaller pieces (< 2s execution time works best)
3. Verify the script logic - test with simpler operations first

Tip: Run small test scripts to isolate the issue, then build up complexity.`
            });
        }, timeoutMs);
        try {
            const cleanScript = script.trim();
            // Try expression form first (captures return values from IIFEs, expressions).
            // If it throws SyntaxError (statements like try/catch, if/else), fall back to statement form.
            let fn;
            try {
                // eslint-disable-next-line no-new-func
                fn = new Function(`"use strict"; return (${cleanScript});`); // nosemgrep: javascript.lang.security.eval.rule-eval-with-expression -- Function() constructor for controlled sandbox execution
            }
            catch {
                // eslint-disable-next-line no-new-func
                fn = new Function(`"use strict"; ${cleanScript}`); // nosemgrep: javascript.lang.security.eval.rule-eval-with-expression -- Function() constructor for controlled sandbox execution
            }
            const result = fn();
            // Handle promises
            if (result && typeof result.then === 'function') {
                ;
                result
                    .then((value) => {
                    clearTimeout(timeoutHandle);
                    deferred.resolve({ success: true, result: safeSerializeForExecute(value) });
                })
                    .catch((err) => {
                    clearTimeout(timeoutHandle);
                    deferred.resolve({
                        success: false,
                        error: 'promise_rejected',
                        message: err.message,
                        stack: err.stack
                    });
                });
            }
            else {
                clearTimeout(timeoutHandle);
                deferred.resolve({ success: true, result: safeSerializeForExecute(result) });
            }
        }
        catch (err) {
            clearTimeout(timeoutHandle);
            const error = err;
            if (error.message &&
                (error.message.includes('Content Security Policy') ||
                    error.message.includes('unsafe-eval') ||
                    error.message.includes('Trusted Type'))) {
                deferred.resolve({
                    success: false,
                    error: 'csp_blocked',
                    message: 'This page has a Content Security Policy that blocks script execution in the MAIN world. ' +
                        'Use world: "isolated" to bypass CSP (DOM access only, no page JS globals). ' +
                        'With world: "auto" (default), this fallback happens automatically.'
                });
            }
            else {
                deferred.resolve({
                    success: false,
                    error: 'execution_error',
                    message: error.message,
                    stack: error.stack
                });
            }
        }
    };
    executeWithTimeoutProtection().catch((err) => {
        console.error('[Gasoline] Unexpected error in executeJavaScript:', err);
        deferred.resolve({
            success: false,
            error: 'execution_error',
            message: 'Unexpected error during script execution'
        });
    });
    return deferred.promise;
}
/**
 * Handle link health check request from content script
 */
export async function handleLinkHealthQuery(data) {
    try {
        const params = data.params || {};
        const result = await checkLinkHealth(params);
        return result;
    }
    catch (err) {
        return {
            error: 'link_health_error',
            message: err.message || 'Failed to check link health'
        };
    }
}
/**
 * Install message listener for handling content script messages
 */
function isValidSettingPayload(data) {
    if (!VALID_SETTINGS.has(data.setting)) {
        console.warn('[Gasoline] Invalid setting:', data.setting);
        return false;
    }
    if (data.setting === 'setWebSocketCaptureMode')
        return typeof data.mode === 'string';
    if (data.setting === 'setServerUrl')
        return typeof data.url === 'string';
    // Boolean settings
    if (typeof data.enabled !== 'boolean') {
        console.warn('[Gasoline] Invalid enabled value type');
        return false;
    }
    return true;
}
function handleLinkHealthMessage(data) {
    handleLinkHealthQuery(data)
        .then((result) => {
        window.postMessage({ type: 'GASOLINE_LINK_HEALTH_RESPONSE', requestId: data.requestId, result }, window.location.origin);
    })
        .catch((err) => {
        window.postMessage({
            type: 'GASOLINE_LINK_HEALTH_RESPONSE',
            requestId: data.requestId,
            result: { error: 'link_health_error', message: err.message || 'Failed to check link health' }
        }, window.location.origin);
    });
}
export function installMessageListener(captureStateFn, restoreStateFn) {
    if (typeof window === 'undefined')
        return;
    const messageHandlers = {
        GASOLINE_SETTING: (data) => {
            const settingData = data;
            if (isValidSettingPayload(settingData))
                handleSetting(settingData);
        },
        GASOLINE_STATE_COMMAND: (data) => handleStateCommand(data, captureStateFn, restoreStateFn),
        GASOLINE_EXECUTE_JS: (data) => handleExecuteJs(data),
        GASOLINE_A11Y_QUERY: (data) => handleA11yQuery(data),
        GASOLINE_DOM_QUERY: (data) => handleDomQuery(data),
        GASOLINE_GET_WATERFALL: (data) => handleGetWaterfall(data),
        GASOLINE_LINK_HEALTH_QUERY: (data) => handleLinkHealthMessage(data)
    };
    window.addEventListener('message', (event) => {
        if (event.source !== window || event.origin !== window.location.origin)
            return;
        if (pageNonce && event.data?._nonce !== pageNonce)
            return;
        const msgType = event.data?.type;
        if (!msgType)
            return;
        const handler = messageHandlers[msgType]; // nosemgrep: unsafe-dynamic-method
        if (handler)
            handler(event.data);
    });
}
const SETTING_HANDLERS = {
    setNetworkWaterfallEnabled: (data) => setNetworkWaterfallEnabled(data.enabled),
    setPerformanceMarksEnabled: (data) => {
        setPerformanceMarksEnabled(data.enabled);
        if (data.enabled)
            installPerformanceCapture();
        else
            uninstallPerformanceCapture();
    },
    setActionReplayEnabled: (data) => setActionCaptureEnabled(data.enabled),
    setWebSocketCaptureEnabled: (data) => {
        setWebSocketCaptureEnabled(data.enabled);
        if (data.enabled)
            installWebSocketCapture();
        else
            uninstallWebSocketCapture();
    },
    setWebSocketCaptureMode: (data) => setWebSocketCaptureMode((data.mode || 'medium')),
    setPerformanceSnapshotEnabled: (data) => setPerformanceSnapshotEnabled(data.enabled),
    setDeferralEnabled: (data) => setDeferralEnabled(data.enabled),
    setNetworkBodyCaptureEnabled: (data) => setNetworkBodyCaptureEnabled(data.enabled),
    setServerUrl: (data) => setServerUrl(data.url)
};
function handleSetting(data) {
    const handler = SETTING_HANDLERS[data.setting];
    if (handler)
        handler(data);
}
function handleStateCommand(data, captureStateFn, restoreStateFn) {
    const { messageId, action, state } = data;
    // Validate action
    if (!VALID_STATE_ACTIONS.has(action)) {
        console.warn('[Gasoline] Invalid state action:', action);
        window.postMessage({
            type: 'GASOLINE_STATE_RESPONSE',
            messageId,
            result: { error: `Invalid action: ${action}` }
        }, window.location.origin);
        return;
    }
    // Validate state object for restore action
    if (action === 'restore' && (!state || typeof state !== 'object')) {
        console.warn('[Gasoline] Invalid state object for restore');
        window.postMessage({
            type: 'GASOLINE_STATE_RESPONSE',
            messageId,
            result: { error: 'Invalid state object' }
        }, window.location.origin);
        return;
    }
    let result;
    try {
        if (action === 'capture') {
            result = captureStateFn();
        }
        else if (action === 'restore') {
            const includeUrl = data.include_url !== false;
            result = restoreStateFn(state, includeUrl);
        }
        else {
            result = { error: `Unknown action: ${action}` };
        }
    }
    catch (err) {
        result = { error: err.message };
    }
    // Send response back to content script
    window.postMessage({
        type: 'GASOLINE_STATE_RESPONSE',
        messageId,
        result
    }, window.location.origin);
}
function handleExecuteJs(data) {
    const { requestId, script, timeoutMs } = data;
    // Validate parameters
    if (typeof script !== 'string') {
        console.warn('[Gasoline] Script must be a string');
        window.postMessage({
            type: 'GASOLINE_EXECUTE_JS_RESULT',
            requestId,
            result: { success: false, error: 'invalid_script', message: 'Script must be a string' }
        }, window.location.origin);
        return;
    }
    if (typeof requestId !== 'number' && typeof requestId !== 'string') {
        console.warn('[Gasoline] Invalid requestId type');
        return;
    }
    executeJavaScript(script, timeoutMs)
        .then((result) => {
        window.postMessage({
            type: 'GASOLINE_EXECUTE_JS_RESULT',
            requestId,
            result
        }, window.location.origin);
    })
        .catch((err) => {
        console.error('[Gasoline] Failed to execute JS:', err);
        window.postMessage({
            type: 'GASOLINE_EXECUTE_JS_RESULT',
            requestId,
            result: { success: false, error: 'execution_failed', message: err.message }
        }, window.location.origin);
    });
}
function handleA11yQuery(data) {
    const { requestId, params } = data;
    if (typeof runAxeAuditWithTimeout !== 'function') {
        window.postMessage({
            type: 'GASOLINE_A11Y_QUERY_RESPONSE',
            requestId,
            result: {
                error: 'runAxeAuditWithTimeout not available - try reloading the extension'
            }
        }, window.location.origin);
        return;
    }
    try {
        runAxeAuditWithTimeout(params || {})
            .then((result) => {
            window.postMessage({
                type: 'GASOLINE_A11Y_QUERY_RESPONSE',
                requestId,
                result
            }, window.location.origin);
        })
            .catch((err) => {
            console.error('[Gasoline] Accessibility audit error:', err);
            window.postMessage({
                type: 'GASOLINE_A11Y_QUERY_RESPONSE',
                requestId,
                result: { error: err.message || 'Accessibility audit failed' }
            }, window.location.origin);
        });
    }
    catch (err) {
        console.error('[Gasoline] Failed to run accessibility audit:', err);
        window.postMessage({
            type: 'GASOLINE_A11Y_QUERY_RESPONSE',
            requestId,
            result: { error: err.message || 'Failed to run accessibility audit' }
        }, window.location.origin);
    }
}
function handleDomQuery(data) {
    const { requestId, params } = data;
    if (typeof executeDOMQuery !== 'function') {
        window.postMessage({
            type: 'GASOLINE_DOM_QUERY_RESPONSE',
            requestId,
            result: {
                error: 'executeDOMQuery not available - try reloading the extension'
            }
        }, window.location.origin);
        return;
    }
    try {
        executeDOMQuery((params || {}))
            .then((result) => {
            window.postMessage({
                type: 'GASOLINE_DOM_QUERY_RESPONSE',
                requestId,
                result
            }, window.location.origin);
        })
            .catch((err) => {
            console.error('[Gasoline] DOM query error:', err);
            window.postMessage({
                type: 'GASOLINE_DOM_QUERY_RESPONSE',
                requestId,
                result: { error: err.message || 'DOM query failed' }
            }, window.location.origin);
        });
    }
    catch (err) {
        console.error('[Gasoline] Failed to run DOM query:', err);
        window.postMessage({
            type: 'GASOLINE_DOM_QUERY_RESPONSE',
            requestId,
            result: { error: err.message || 'Failed to run DOM query' }
        }, window.location.origin);
    }
}
function handleGetWaterfall(data) {
    const { requestId } = data;
    try {
        const entries = getNetworkWaterfall({});
        // Convert camelCase WaterfallEntry fields to snake_case for Go daemon
        const snakeEntries = (entries || []).map((e) => ({
            url: e.url,
            name: e.url,
            initiator_type: e.initiatorType,
            start_time: e.startTime,
            duration: e.duration,
            transfer_size: e.transferSize,
            encoded_body_size: e.encodedBodySize,
            decoded_body_size: e.decodedBodySize
        }));
        window.postMessage({
            type: 'GASOLINE_WATERFALL_RESPONSE',
            requestId,
            entries: snakeEntries,
            page_url: window.location.href
        }, window.location.origin);
    }
    catch (err) {
        console.error('[Gasoline] Failed to get network waterfall:', err);
        window.postMessage({
            type: 'GASOLINE_WATERFALL_RESPONSE',
            requestId,
            entries: []
        }, window.location.origin);
    }
}
//# sourceMappingURL=message-handlers.js.map