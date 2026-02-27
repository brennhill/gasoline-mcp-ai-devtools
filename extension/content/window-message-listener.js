/**
 * Purpose: Handles content-script message relay between background and inject contexts.
 * Why: Keeps content-script bridging predictable between extension and page contexts.
 * Docs: docs/features/feature/interact-explore/index.md
 * Docs: docs/features/feature/query-dom/index.md
 */
import { resolveHighlightRequest, resolveExecuteRequest, resolveA11yRequest, resolveDomRequest } from './request-tracking';
import { MESSAGE_MAP, safeSendMessage } from './message-forwarding';
import { getIsTrackedTab, getCurrentTabId } from './tab-tracking';
import { getPageNonce } from './script-injection';
const RESPONSE_HANDLERS = {
    GASOLINE_HIGHLIGHT_RESPONSE: (id, result) => resolveHighlightRequest(id, result),
    GASOLINE_EXECUTE_JS_RESULT: (id, result) => resolveExecuteRequest(id, result),
    GASOLINE_A11Y_QUERY_RESPONSE: (id, result) => resolveA11yRequest(id, result),
    GASOLINE_DOM_QUERY_RESPONSE: (id, result) => resolveDomRequest(id, result)
};
export function initWindowMessageListener() {
    window.addEventListener('message', (event) => {
        if (event.source !== window || event.origin !== window.location.origin)
            return;
        const { type: messageType, requestId, result, payload } = event.data || {};
        const responseHandler = messageType ? RESPONSE_HANDLERS[messageType] : undefined;
        if (responseHandler) {
            // Validate nonce on response messages (spoofing prevention).
            // Accept responses with no nonce for backwards compat during migration.
            const nonce = event.data?._nonce;
            if (nonce && nonce !== getPageNonce())
                return;
            if (requestId !== undefined)
                responseHandler(requestId, result);
            return;
        }
        // Tab isolation filter: only forward captured data from the tracked tab.
        // Response messages (highlight, execute JS, a11y) are NOT filtered because
        // they are responses to explicit commands from the background script.
        if (!getIsTrackedTab())
            return;
        if (messageType && messageType in MESSAGE_MAP && payload && typeof payload === 'object') {
            const mappedType = MESSAGE_MAP[messageType];
            if (mappedType) {
                safeSendMessage({
                    type: mappedType,
                    payload,
                    tabId: getCurrentTabId()
                });
            }
        }
    });
}
//# sourceMappingURL=window-message-listener.js.map