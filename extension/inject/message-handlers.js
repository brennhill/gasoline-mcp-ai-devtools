/**
 * Purpose: Dispatches window.postMessage commands from the content script to specialized inject-context handlers (settings, state, JS execution, DOM/a11y queries).
 * Docs: docs/features/feature/interact-explore/index.md
 */
import { executeDOMQuery, runAxeAuditWithTimeout } from '../lib/dom-queries.js';
import { checkLinkHealth } from '../lib/link-health.js';
import { queryComputedStyles } from './computed-styles.js';
import { discoverForms } from './form-discovery.js';
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
/** Send a nonce-authenticated response back to the content script */
function postResponse(data) {
    window.postMessage({ ...data, _nonce: pageNonce }, window.location.origin);
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
        postResponse({ type: 'GASOLINE_LINK_HEALTH_RESPONSE', requestId: data.requestId, result });
    })
        .catch((err) => {
        postResponse({
            type: 'GASOLINE_LINK_HEALTH_RESPONSE',
            requestId: data.requestId,
            result: { error: 'link_health_error', message: err.message || 'Failed to check link health' }
        });
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
        GASOLINE_LINK_HEALTH_QUERY: (data) => handleLinkHealthMessage(data),
        GASOLINE_COMPUTED_STYLES_QUERY: (data) => handleComputedStylesMessage(data),
        GASOLINE_FORM_DISCOVERY_QUERY: (data) => handleFormDiscoveryMessage(data),
        GASOLINE_INJECT_BRIDGE_PING: (data) => handleBridgePingMessage(data)
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
function handleBridgePingMessage(data) {
    postResponse({
        type: 'GASOLINE_INJECT_BRIDGE_PONG',
        requestId: data.requestId
    });
}
function handleComputedStylesMessage(data) {
    try {
        const params = (data.params || {});
        const result = queryComputedStyles({
            selector: params.selector || '*',
            properties: params.properties
        });
        postResponse({
            type: 'GASOLINE_COMPUTED_STYLES_RESPONSE',
            requestId: data.requestId,
            result: { elements: result, count: result.length }
        });
    }
    catch (err) {
        postResponse({
            type: 'GASOLINE_COMPUTED_STYLES_RESPONSE',
            requestId: data.requestId,
            result: { error: 'computed_styles_error', message: err.message || 'Failed to query computed styles' }
        });
    }
}
function handleFormDiscoveryMessage(data) {
    try {
        const params = (data.params || {});
        const result = discoverForms({
            selector: params.selector,
            mode: params.mode === 'validate' ? 'validate' : 'discover'
        });
        postResponse({
            type: 'GASOLINE_FORM_DISCOVERY_RESPONSE',
            requestId: data.requestId,
            result: { forms: result, count: result.length }
        });
    }
    catch (err) {
        postResponse({
            type: 'GASOLINE_FORM_DISCOVERY_RESPONSE',
            requestId: data.requestId,
            result: { error: 'form_discovery_error', message: err.message || 'Failed to discover forms' }
        });
    }
}
function handleExecuteJs(data) {
    const { requestId, script, timeoutMs } = data;
    // Validate parameters
    if (typeof script !== 'string') {
        console.warn('[Gasoline] Script must be a string');
        postResponse({
            type: 'GASOLINE_EXECUTE_JS_RESULT',
            requestId,
            result: { success: false, error: 'invalid_script', message: 'Script must be a string' }
        });
        return;
    }
    if (typeof requestId !== 'number' && typeof requestId !== 'string') {
        console.warn('[Gasoline] Invalid requestId type');
        return;
    }
    executeJavaScript(script, timeoutMs)
        .then((result) => {
        postResponse({
            type: 'GASOLINE_EXECUTE_JS_RESULT',
            requestId,
            result
        });
    })
        .catch((err) => {
        console.error('[Gasoline] Failed to execute JS:', err);
        postResponse({
            type: 'GASOLINE_EXECUTE_JS_RESULT',
            requestId,
            result: { success: false, error: 'execution_failed', message: err.message }
        });
    });
}
function handleA11yQuery(data) {
    const { requestId, params } = data;
    if (typeof runAxeAuditWithTimeout !== 'function') {
        postResponse({
            type: 'GASOLINE_A11Y_QUERY_RESPONSE',
            requestId,
            result: {
                error: 'runAxeAuditWithTimeout not available - try reloading the extension'
            }
        });
        return;
    }
    try {
        runAxeAuditWithTimeout(params || {})
            .then((result) => {
            postResponse({
                type: 'GASOLINE_A11Y_QUERY_RESPONSE',
                requestId,
                result
            });
        })
            .catch((err) => {
            console.error('[Gasoline] Accessibility audit error:', err);
            postResponse({
                type: 'GASOLINE_A11Y_QUERY_RESPONSE',
                requestId,
                result: { error: err.message || 'Accessibility audit failed' }
            });
        });
    }
    catch (err) {
        console.error('[Gasoline] Failed to run accessibility audit:', err);
        postResponse({
            type: 'GASOLINE_A11Y_QUERY_RESPONSE',
            requestId,
            result: { error: err.message || 'Failed to run accessibility audit' }
        });
    }
}
function handleDomQuery(data) {
    const { requestId, params } = data;
    if (typeof executeDOMQuery !== 'function') {
        postResponse({
            type: 'GASOLINE_DOM_QUERY_RESPONSE',
            requestId,
            result: {
                error: 'executeDOMQuery not available - try reloading the extension'
            }
        });
        return;
    }
    try {
        executeDOMQuery((params || {}))
            .then((result) => {
            postResponse({
                type: 'GASOLINE_DOM_QUERY_RESPONSE',
                requestId,
                result
            });
        })
            .catch((err) => {
            console.error('[Gasoline] DOM query error:', err);
            postResponse({
                type: 'GASOLINE_DOM_QUERY_RESPONSE',
                requestId,
                result: { error: err.message || 'DOM query failed' }
            });
        });
    }
    catch (err) {
        console.error('[Gasoline] Failed to run DOM query:', err);
        postResponse({
            type: 'GASOLINE_DOM_QUERY_RESPONSE',
            requestId,
            result: { error: err.message || 'Failed to run DOM query' }
        });
    }
}
function handleGetWaterfall(data) {
    const { requestId } = data;
    try {
        const entries = getNetworkWaterfall({});
        postResponse({
            type: 'GASOLINE_WATERFALL_RESPONSE',
            requestId,
            entries: entries || [],
            page_url: window.location.href
        });
    }
    catch (err) {
        console.error('[Gasoline] Failed to get network waterfall:', err);
        postResponse({
            type: 'GASOLINE_WATERFALL_RESPONSE',
            requestId,
            entries: []
        });
    }
}
//# sourceMappingURL=message-handlers.js.map