// message-handlers.ts — Message dispatch from content script to inject-context handlers.
import { executeDOMQuery, runAxeAuditWithTimeout } from '../lib/dom-queries.js';
import { checkLinkHealth } from '../lib/link-health.js';
import { getNetworkWaterfall } from '../lib/network.js';
import { executeJavaScript } from './execute-js.js';
import { isValidSettingPayload, handleSetting, handleStateCommand } from './settings.js';
// Re-export for barrel (src/inject/index.ts)
export { executeJavaScript, safeSerializeForExecute } from './execute-js.js';
/** Read the page nonce set by the content script on the inject script element */
let pageNonce = '';
if (typeof document !== 'undefined' && typeof document.querySelector === 'function') {
    const nonceEl = document.querySelector('script[data-gasoline-nonce]');
    if (nonceEl) {
        pageNonce = nonceEl.getAttribute('data-gasoline-nonce') || '';
    }
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
        window.postMessage({
            type: 'GASOLINE_WATERFALL_RESPONSE',
            requestId,
            entries: entries || [],
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